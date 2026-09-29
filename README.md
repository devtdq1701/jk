# jk — Lightweight Jenkins CLI for AI Agents & DevOps Engineers

`jk` is a fast, dependency-light CLI for interacting with Jenkins controllers. Designed for both terminal use and autonomous AI coding agents (Antigravity, Claude Code, Cursor, OpenCode).

## Features

- **Multi-Context**: Seamlessly switch between multiple Jenkins instances (`~/.config/jk/config.json`).
- **CSRF Crumb Aware**: Automatically fetches and handles Jenkins crumbs for state-mutating requests.
- **Rate-Limited**: Built-in token-bucket rate limiter (`golang.org/x/time/rate`) to prevent hammering controllers.
- **Safe Config Patching**: Inspect (`get-config`) and patch (`set-config`) job `config.xml` via stdin or file.
- **Log & Stage Streaming**: View full console logs, follow live build output (`-f`), or inspect specific pipeline stages (`--stage`).
- **Zero Heavy Dependencies**: Pure Go with Cobra CLI and standard library `net/http`.

## Installation

### Pre-built Binaries (Linux & Windows)

Download the latest pre-compiled binary from [GitHub Releases](https://github.com/devtdq1701/jk/releases/latest):

- **Linux (x86_64)**: [`jk-linux-amd64`](https://github.com/devtdq1701/jk/releases/download/latest/jk-linux-amd64)
- **Linux (ARM64)**: [`jk-linux-arm64`](https://github.com/devtdq1701/jk/releases/download/latest/jk-linux-arm64)
- **Windows (x86_64)**: [`jk-windows-amd64.exe`](https://github.com/devtdq1701/jk/releases/download/latest/jk-windows-amd64.exe)

#### Linux Quick Install:
```bash
curl -sSL -o ~/.local/bin/jk https://github.com/devtdq1701/jk/releases/download/latest/jk-linux-amd64 && chmod +x ~/.local/bin/jk
```

### With Go (1.22+)

```bash
go install github.com/devtdq1701/jk@latest
```

Ensure `$(go env GOPATH)/bin` is in your `PATH`.

### From Source

```bash
git clone https://github.com/devtdq1701/jk.git
cd jk
go build -o ~/.local/bin/jk .
```

## Quick Start

### 1. Configure Context

Add one or more Jenkins controller profiles:

```bash
# Add a context
jk context add prod --url https://jenkins.example.com --user alice --token 1102...

# List configured contexts
jk context ls

# Switch active context
jk context use prod
```

### 2. Verify Connectivity & Auth

```bash
jk check
```

Output:
```text
[OK] Connect:   https://jenkins.example.com (45ms)
[OK] Auth:      Logged in as 'alice'
[OK] CSRF:      Crumb issuer available (Jenkins-Crumb)
Context is healthy and ready.
```

### 3. Job Operations & Config Management

```bash
# List jobs in root or folder
jk job ls
jk job ls my-folder

# Export job config.xml
jk job get-config my-job > config.xml

# Update job config.xml from file or pipe
jk job set-config my-job -f config.xml
cat config.xml | jk job set-config my-job --stdin
```

### 4. Build & Stage Logs

```bash
# Trigger a build (with optional parameters)
jk build my-pipeline -p BRANCH=main -p ENV=staging

# View console log (or follow live)
jk log my-pipeline
jk log my-pipeline -f

# Inspect pipeline stages
jk stages my-pipeline

# View specific stage log
jk log my-pipeline --stage "Build & Test"
```

### 5. Inspect System Credentials

```bash
jk cred ls
```

## AI Agent Integration

`jk` comes with a pre-configured Agent Skill definition under [`skills/jk/SKILL.md`](skills/jk/SKILL.md).

To equip your AI assistant (e.g., Claude Code, Antigravity, or Cursor):
- Copy or link `skills/jk/SKILL.md` to your agent's skills directory (e.g., `~/.gemini/config/skills/jk/SKILL.md` or `.agents/skills/jk/SKILL.md`).

## License

[MIT](LICENSE) © 2026 Quang Tran (devtdq1701)
