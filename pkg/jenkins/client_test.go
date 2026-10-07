package jenkins

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResolveJobPath(t *testing.T) {
	c := &Client{}
	tests := []struct {
		input    string
		expected string
	}{
		{"my-job", "/job/my-job"},
		{"folder/sub/job", "/job/folder/job/sub/job/job"},
		{"/leading/trailing/", "/job/leading/job/trailing"},
		{"", ""},
	}

	for _, tt := range tests {
		got := c.ResolveJobPath(tt.input)
		if got != tt.expected {
			t.Errorf("ResolveJobPath(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestMockJenkinsClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "testuser" || p != "testtoken" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/api/json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"nodeName": "master"}`))
		case "/whoAmI/api/json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name": "testuser", "authenticated": true}`))
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"crumbRequestField": "Jenkins-Crumb", "crumb": "test-crumb-value"}`))
		case "/job/my-job/config.xml":
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`<project><name>my-job</name></project>`))
			} else if r.Method == http.MethodPost {
				if r.Header.Get("Jenkins-Crumb") != "test-crumb-value" {
					http.Error(w, "Missing crumb", http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusOK)
			}
		case "/job/my-job/lastBuild/consoleText":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Started by user\nBuilding...\nFinished: SUCCESS\n"))
		case "/job/my-job/lastBuild/wfapi/describe":
			w.WriteHeader(http.StatusOK)
			data := map[string]interface{}{
				"stages": []map[string]interface{}{
					{"id": "1", "name": "Build", "status": "SUCCESS", "durationMillis": 1000},
				},
			}
			_ = json.NewEncoder(w).Encode(data)
		case "/job/my-job/api/json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"builds": [
					{"number": 40, "url": "http://jenkins/job/my-job/40/", "result": "SUCCESS", "timestamp": 1600000000000, "duration": 84000},
					{"number": 39, "url": "http://jenkins/job/my-job/39/", "result": "SUCCESS", "timestamp": 1599990000000, "duration": 78000}
				]
			}`))
		case "/job/my-job/39/api/json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"number": 39,
				"result": "SUCCESS",
				"timestamp": 1599990000000,
				"duration": 78000,
				"building": false,
				"url": "http://jenkins/job/my-job/39/",
				"actions": [
					{
						"causes": [
							{"shortDescription": "Started by remote host 10.165.17.210"}
						]
					},
					{
						"parameters": [
							{"name": "BRANCH", "value": "QLVB_TRAVINH_core"},
							{"name": "COMMIT_ID", "value": "feb1b47c185e0c37ce21ef8c36d0637419b6b1ac"},
							{"name": "ITKV", "value": "ITKV5"},
							{"name": "TASKDEPLOY", "value": "IT360-1639280"}
						]
					},
					{
						"_class": "hudson.plugins.git.util.BuildData",
						"lastBuiltRevision": {
							"SHA1": "feb1b47c185e0c37ce21ef8c36d0637419b6b1ac",
							"branch": [{"name": "origin/QLVB_TRAVINH_core"}]
						}
					}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client, err := NewClient(ts.URL, "testuser", "testtoken", 5*time.Second, 10.0, 10)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	// 1. Check
	res, err := client.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if !res.Authenticated || res.User != "testuser" || !res.CSRFEnabled {
		t.Errorf("Unexpected check result: %+v", res)
	}

	// 2. GetJobConfig
	cfg, err := client.GetJobConfig(ctx, "my-job")
	if err != nil {
		t.Fatalf("GetJobConfig failed: %v", err)
	}
	if cfg != "<project><name>my-job</name></project>" {
		t.Errorf("Unexpected config: %s", cfg)
	}

	// 3. SetJobConfig (tests crumb injection)
	err = client.SetJobConfig(ctx, "my-job", []byte("<project><name>updated</name></project>"))
	if err != nil {
		t.Fatalf("SetJobConfig with crumb failed: %v", err)
	}

	// 4. StreamLog
	var buf bytes.Buffer
	err = client.StreamLog(ctx, "my-job", "", false, &buf)
	if err != nil {
		t.Fatalf("StreamLog failed: %v", err)
	}
	if buf.String() != "Started by user\nBuilding...\nFinished: SUCCESS\n" {
		t.Errorf("Unexpected log: %s", buf.String())
	}

	// 5. GetStages
	stages, err := client.GetStages(ctx, "my-job", "")
	if err != nil {
		t.Fatalf("GetStages failed: %v", err)
	}
	if len(stages) != 1 || stages[0].Name != "Build" {
		t.Errorf("Unexpected stages: %+v", stages)
	}

	// 6. GetBuilds
	builds, err := client.GetBuilds(ctx, "my-job", 10)
	if err != nil {
		t.Fatalf("GetBuilds failed: %v", err)
	}
	if len(builds) != 2 || builds[0].Number != 40 || builds[0].Result != "SUCCESS" {
		t.Errorf("Unexpected builds: %+v", builds)
	}

	// 7. GetBuildInfo
	info, err := client.GetBuildInfo(ctx, "my-job", "39")
	if err != nil {
		t.Fatalf("GetBuildInfo failed: %v", err)
	}
	if info.Number != 39 || info.Result != "SUCCESS" || info.Parameters["ITKV"] != "ITKV5" || info.Parameters["COMMIT_ID"] != "feb1b47c185e0c37ce21ef8c36d0637419b6b1ac" {
		t.Errorf("Unexpected build info: %+v", info)
	}
	if len(info.Causes) != 1 || info.Causes[0] != "Started by remote host 10.165.17.210" {
		t.Errorf("Unexpected causes: %+v", info.Causes)
	}
	if info.GitCommit != "feb1b47c185e0c37ce21ef8c36d0637419b6b1ac" || info.GitBranch != "origin/QLVB_TRAVINH_core" {
		t.Errorf("Unexpected git info: commit=%s branch=%s", info.GitCommit, info.GitBranch)
	}
}
