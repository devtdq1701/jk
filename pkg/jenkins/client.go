package jenkins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Client struct {
	BaseURL    string
	Username   string
	Token      string
	HTTPClient *http.Client
	Limiter    *rate.Limiter

	mu         sync.Mutex
	crumbField string
	crumbValue string
}

type CheckResult struct {
	URL           string        `json:"url"`
	Latency       time.Duration `json:"latency"`
	Authenticated bool          `json:"authenticated"`
	User          string        `json:"user"`
	CrumbField    string        `json:"crumb_field,omitempty"`
	CSRFEnabled   bool          `json:"csrf_enabled"`
}

type JobItem struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CredItem struct {
	ID          string `json:"id"`
	TypeName    string `json:"typeName"`
	Description string `json:"description"`
}

type StageItem struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Status              string `json:"status"`
	DurationMillis      int64  `json:"durationMillis"`
	PauseDurationMillis int64  `json:"pauseDurationMillis"`
}

func NewClient(rawURL, user, token string, timeout time.Duration, rps float64, burst int) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(rawURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid jenkins url: %w", err)
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
	}

	if rps <= 0 {
		rps = 5.0
	}
	if burst <= 0 {
		burst = 10
	}

	return &Client{
		BaseURL:  parsed.String(),
		Username: user,
		Token:    token,
		HTTPClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		Limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}, nil
}

func (c *Client) ResolveJobPath(input string) string {
	clean := strings.Trim(input, "/")
	if clean == "" {
		return ""
	}
	parts := strings.Split(clean, "/")
	return "/job/" + strings.Join(parts, "/job/")
}

func (c *Client) fetchCrumb(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.crumbField != "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/crumbIssuer/api/json", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Username, c.Token)

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// CSRF disabled on controller
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("crumb issuer returned status %d", resp.StatusCode)
	}

	var data struct {
		CrumbRequestField string `json:"crumbRequestField"`
		Crumb             string `json:"crumb"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("decode crumb: %w", err)
	}

	c.crumbField = data.CrumbRequestField
	c.crumbValue = data.Crumb
	return nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if err := c.Limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return c.HTTPClient.Do(req)
}

func (c *Client) newAuthenticatedRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	fullURL := c.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Username, c.Token)

	if method == http.MethodPost || method == http.MethodPut {
		if err := c.fetchCrumb(ctx); err == nil && c.crumbField != "" {
			req.Header.Set(c.crumbField, c.crumbValue)
		}
	}
	return req, nil
}

func (c *Client) Check(ctx context.Context) (*CheckResult, error) {
	res := &CheckResult{URL: c.BaseURL}

	// 1. Connectivity & Latency
	start := time.Now()
	req, err := c.newAuthenticatedRequest(ctx, http.MethodGet, "/api/json?tree=nodeName", nil)
	if err != nil {
		return nil, fmt.Errorf("build check request: %w", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", c.BaseURL, err)
	}
	resp.Body.Close()
	res.Latency = time.Since(start)

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("authentication failed (HTTP %d): check username and token", resp.StatusCode)
	}

	// 2. WhoAmI
	reqWho, err := c.newAuthenticatedRequest(ctx, http.MethodGet, "/whoAmI/api/json", nil)
	if err == nil {
		if respWho, err := c.Do(reqWho); err == nil {
			defer respWho.Body.Close()
			if respWho.StatusCode == http.StatusOK {
				var who struct {
					Name          string `json:"name"`
					Authenticated bool   `json:"authenticated"`
				}
				if json.NewDecoder(respWho.Body).Decode(&who) == nil {
					res.User = who.Name
					res.Authenticated = who.Authenticated
				}
			}
		}
	}

	// 3. CSRF Crumb
	if err := c.fetchCrumb(ctx); err == nil && c.crumbField != "" {
		res.CSRFEnabled = true
		res.CrumbField = c.crumbField
	}

	return res, nil
}

func (c *Client) ListJobs(ctx context.Context, folder string) ([]JobItem, error) {
	endpoint := "/api/json?tree=jobs[name,color]"
	if folder != "" {
		endpoint = c.ResolveJobPath(folder) + "/api/json?tree=jobs[name,color]"
	}

	req, err := c.newAuthenticatedRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list jobs returned HTTP %d", resp.StatusCode)
	}

	var data struct {
		Jobs []JobItem `json:"jobs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode jobs: %w", err)
	}
	return data.Jobs, nil
}

func (c *Client) ListCredentials(ctx context.Context) ([]CredItem, error) {
	endpoint := "/credentials/store/system/domain/_/api/json?tree=credentials[id,typeName,description]"
	req, err := c.newAuthenticatedRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list credentials returned HTTP %d", resp.StatusCode)
	}

	var data struct {
		Credentials []CredItem `json:"credentials"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode credentials: %w", err)
	}
	return data.Credentials, nil
}

func (c *Client) GetJobConfig(ctx context.Context, jobPath string) (string, error) {
	path := c.ResolveJobPath(jobPath)
	req, err := c.newAuthenticatedRequest(ctx, http.MethodGet, path+"/config.xml", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get config returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read config body: %w", err)
	}
	return string(body), nil
}

func (c *Client) SetJobConfig(ctx context.Context, jobPath string, xmlData []byte) error {
	path := c.ResolveJobPath(jobPath)
	req, err := c.newAuthenticatedRequest(ctx, http.MethodPost, path+"/config.xml", strings.NewReader(string(xmlData)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set config returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *Client) BuildJob(ctx context.Context, jobPath string, params map[string]string) (string, error) {
	path := c.ResolveJobPath(jobPath)
	endpoint := path + "/build"

	var body io.Reader
	var contentType string

	if len(params) > 0 {
		endpoint = path + "/buildWithParameters"
		vals := url.Values{}
		for k, v := range params {
			vals.Set(k, v)
		}
		body = strings.NewReader(vals.Encode())
		contentType = "application/x-www-form-urlencoded"
	}

	req, err := c.newAuthenticatedRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("build returned HTTP %d", resp.StatusCode)
	}

	queueLoc := resp.Header.Get("Location")
	return queueLoc, nil
}

func (c *Client) GetStages(ctx context.Context, jobPath, buildNo string) ([]StageItem, error) {
	if buildNo == "" {
		buildNo = "lastBuild"
	}
	path := c.ResolveJobPath(jobPath)
	endpoint := fmt.Sprintf("%s/%s/wfapi/describe", path, buildNo)

	req, err := c.newAuthenticatedRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("describe workflow returned HTTP %d (is pipeline-stage-view installed?)", resp.StatusCode)
	}

	var data struct {
		Stages []StageItem `json:"stages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode stages: %w", err)
	}
	return data.Stages, nil
}

func (c *Client) GetStageLog(ctx context.Context, jobPath, buildNo, stageNameOrID string) (string, error) {
	stages, err := c.GetStages(ctx, jobPath, buildNo)
	if err != nil {
		return "", err
	}

	var targetNodeID string
	for _, s := range stages {
		if s.ID == stageNameOrID || strings.EqualFold(s.Name, stageNameOrID) {
			targetNodeID = s.ID
			break
		}
	}
	if targetNodeID == "" {
		return "", fmt.Errorf("stage '%s' not found", stageNameOrID)
	}

	if buildNo == "" {
		buildNo = "lastBuild"
	}
	path := c.ResolveJobPath(jobPath)
	endpoint := fmt.Sprintf("%s/%s/execution/node/%s/wfapi/log", path, buildNo, targetNodeID)

	req, err := c.newAuthenticatedRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get stage log returned HTTP %d", resp.StatusCode)
	}

	var logData struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&logData); err != nil {
		return "", fmt.Errorf("decode stage log: %w", err)
	}
	return logData.Text, nil
}

func (c *Client) StreamLog(ctx context.Context, jobPath, buildNo string, follow bool, out io.Writer) error {
	if buildNo == "" {
		buildNo = "lastBuild"
	}
	path := c.ResolveJobPath(jobPath)

	if !follow {
		req, err := c.newAuthenticatedRequest(ctx, http.MethodGet, fmt.Sprintf("%s/%s/consoleText", path, buildNo), nil)
		if err != nil {
			return err
		}
		resp, err := c.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("get console log returned HTTP %d", resp.StatusCode)
		}
		_, err = io.Copy(out, resp.Body)
		return err
	}

	// Follow mode: progressiveText polling
	var offset int64 = 0
	chunkClient := &http.Client{
		Transport: c.HTTPClient.Transport,
		Timeout:   30 * time.Second, // Idle chunk read timeout
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		endpoint := fmt.Sprintf("%s/%s/logText/progressiveText?start=%d", path, buildNo, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+endpoint, nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth(c.Username, c.Token)

		if err := c.Limiter.Wait(ctx); err != nil {
			return err
		}

		resp, err := chunkClient.Do(req)
		if err != nil {
			// Transient network hiccup, retry after short sleep
			time.Sleep(1 * time.Second)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			_, _ = io.Copy(out, resp.Body)
			resp.Body.Close()

			if sizeStr := resp.Header.Get("X-Text-Size"); sizeStr != "" {
				if newOffset, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
					offset = newOffset
				}
			}

			hasMore := resp.Header.Get("X-More-Data")
			if strings.ToLower(hasMore) != "true" {
				// Build finished
				break
			}
		} else if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			// Offset reset if needed
			resp.Body.Close()
		} else {
			resp.Body.Close()
			return fmt.Errorf("progressive log returned HTTP %d", resp.StatusCode)
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}
