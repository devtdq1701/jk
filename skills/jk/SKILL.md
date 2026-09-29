---
name: jk
description: "USE when: interacting with Jenkins controller via jk CLI: checking controller health, managing contexts, listing jobs, reading or streaming logs (including stage logs), inspecting or patching config.xml, listing credentials, and triggering builds. SKIP when: editing application source code or Kubernetes manifests directly."
triggers:
  use_when:
    - inspecting Jenkins jobs or builds
    - checking Jenkins console or stage logs
    - reading or patching Jenkins config.xml
    - listing Jenkins credentials
    - triggering Jenkins pipeline builds
    - diagnosing Jenkins controller connectivity
  skip_when:
    - editing Kubernetes manifests
    - modifying application code directly
    - inspecting Harbor registry or Git repositories
  keywords:
    - jk
    - jenkins
    - jenkins-cli
    - console log
    - stage log
    - config.xml
    - jenkins build
    - jenkins credential
---

# Jenkins CLI (`jk`) Skill

Use `jk` to interact with Jenkins controllers safely, fast, and without raw curl commands.

## Core Rules for AI Agents

1. **Verify Context First**: Before performing any mutation or reading logs, verify the active controller with `jk context current` or run `jk check`.
2. **Never Expose Tokens**: Tokens are stored safely in `~/.config/jk/config.json`. Do not log, print, or leak token values.
3. **Backup Before Patching**: Before updating a job's configuration with `jk job set-config`, always export the existing configuration:
   ```bash
   jk job get-config <job> > /tmp/backup-<job>.xml
   ```
4. **Targeted Stage Investigation**: When diagnosing a failing pipeline, run `jk stages <job>` first to locate the failed stage ID/name, then fetch only that stage's log via `jk log <job> --stage <stage>`.

---

## Command Reference

### 1. Context & Health Check
```bash
# Verify controller reachability, authentication, and CSRF crumb readiness
jk check

# List saved contexts (active context marked with *)
jk context ls

# Show active context name
jk context current

# Switch active context
jk context use <name>

# Add or update a context
jk context add <name> --url <url> --user <user> --token <token>
```

### 2. Job Discovery & Credential Inspection
```bash
# List jobs in root
jk job ls

# List jobs inside a folder
jk job ls <folder_path>

# List system credentials (ID, type, description)
jk cred ls
```

### 3. Log & Stage Inspection
```bash
# View console log of the latest build
jk log <job>

# View console log of a specific build number
jk log <job> 128

# Follow/stream live build log until completion
jk log <job> -f
jk log <job> 128 -f

# List stages of a pipeline build with status and duration
jk stages <job>
jk stages <job> 128

# View log for a specific stage (by name or ID)
jk log <job> --stage "Build & Test"
jk log <job> 128 --stage 12
```

### 4. Configuration Management (`config.xml`)
```bash
# Export raw config.xml to stdout or file
jk job get-config <job>
jk job get-config <job> > config.xml

# Update job config from file (automatically handles CSRF crumb)
jk job set-config <job> -f config.xml

# Update job config from stdin
cat config.xml | jk job set-config <job> --stdin
```

### 5. Triggering Builds
```bash
# Trigger standard build
jk build <job>

# Trigger parameterized build
jk build <job> -p SERVICE=integration-service -p ENVIRONMENT=production
```
