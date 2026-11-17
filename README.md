# pr-review-go

A high-performance, single-binary AI pull request reviewer and assistant built in Go.

`pr-review-go` is designed as a lightweight, drop-in alternative to heavy Python-based review tools (such as `pr-agent`). It compiles to a single **12MB static binary** with **sub-20ms cold start**, making it equally suited for local CLI development, CI pipelines, and a long-running webhook daemon.

---

## Key Features

- **Single Static Binary**: Zero Python dependencies, virtual environments, or multi-gigabyte Docker containers. Runs in `scratch` or `distroless` containers.
- **Multi-Provider Support**:
  - **GitHub SaaS** (`api.github.com`) & **GitHub Enterprise Server (GHES)** via Personal Access Token or GitHub App (App ID + Private Key RSA PEM).
  - **Bitbucket Cloud** (`api.bitbucket.org/2.0`) via App Passwords.
  - **Bitbucket Data Center / Server** on-premises via Personal Access Tokens.
  - **Local Git / Stdin**: Review local git diffs without committing or pushing.
- **Core Commands**:
  - `review`: In-depth review with severity-rated findings, actionable inline suggestions, risk assessment, and provider commit status.
  - `describe`: Automatically generates PR title, type (`feat`, `fix`, etc.), summary, and walkthrough.
  - `ask`: Interactive Q&A regarding pull request diffs.
- **Production Webhook Daemon (`serve`)**:
  - Immediate `202 Accepted` response to avoid webhook timeouts.
  - Bounded in-memory worker pool for concurrency control.
  - **Debounce cancellation**: Automatically cancels in-flight reviews for a PR if a newer commit is pushed to the same branch.
  - **Comment triggers**: Supports interactive `/review`, `/describe`, and `/ask <question>` commands posted by reviewers directly in PR comment threads.
- **Universal LLM Compatibility**:
  - Connects to any OpenAI-compatible endpoint: OpenAI (GPT-4o, etc.), Azure OpenAI, Groq, Ollama, DeepSeek, OpenRouter, and local vLLM instances.

---

## Quick Start

### 1. Build the Binary

```bash
make build
# Binary created at bin/pr-review-go (~12MB)
```

To run all unit tests:
```bash
make test
```

---

## CLI Usage

### A. Local Git Diff (Developer Mode)

Review code locally before pushing:

```bash
export OPENAI_API_KEY="sk-..."

# Review current branch diff against main
git diff main...HEAD | ./bin/pr-review-go review --stdin

# Ask questions about local changes
git diff main...HEAD | ./bin/pr-review-go ask "Are there any unclosed resources or race conditions?" --stdin

# Generate PR description from local diff
git diff main...HEAD | ./bin/pr-review-go describe --stdin
```

### B. Remote Pull Request Review

```bash
export OPENAI_API_KEY="sk-..."

# 1. GitHub PR (SaaS or GHES)
export GITHUB_TOKEN="ghp_..."
./bin/pr-review-go review --pr-url="https://github.com/owner/repo/pull/42"

# 2. Bitbucket Cloud PR
export BITBUCKET_USERNAME="your-user"
export BITBUCKET_APP_PASSWORD="your-app-password"
./bin/pr-review-go review --pr-url="https://bitbucket.org/workspace/repo/pull-requests/10"

# 3. Bitbucket Data Center (Server) PR
export BITBUCKET_SERVER_TOKEN="your-pat"
./bin/pr-review-go review --pr-url="https://bitbucket.internal.corp/projects/PROJ/repos/repo/pull-requests/5"
```

---

## Webhook Daemon Mode (`serve`)

Deploy `pr-review-go` as a lightweight webhook receiver for your Git server.

```bash
export OPENAI_API_KEY="sk-..."
export GITHUB_TOKEN="ghp_..."
export GITHUB_WEBHOOK_SECRET="my-webhook-secret"

./bin/pr-review-go serve --port=8080 --host=0.0.0.0
```

### Endpoints

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/webhooks/github` | `POST` | GitHub SaaS / GHES webhooks (Events: `Pull requests`, `Issue comments`) |
| `/webhooks/bitbucket` | `POST` | Bitbucket Cloud & Data Center webhooks (Events: `created`, `updated`, `comment`) |
| `/healthz` | `GET` | Health check probe (returns `200 OK`) |

---

## Configuration Reference

`pr-review-go` loads settings from an optional YAML config file (`-c config.yaml`), environment variables, and repository-level `.pr-agent.yaml` files.

### Key Environment Variables

| Variable | Description | Default |
| :--- | :--- | :--- |
| `OPENAI_API_KEY` | API Key for LLM provider | - |
| `OPENAI_BASE_URL` | Custom endpoint for Ollama, Azure, vLLM, Groq | `https://api.openai.com/v1` |
| `LLM_MODEL` | Model name | `gpt-4o` |
| `GITHUB_TOKEN` | GitHub Personal Access Token | - |
| `GITHUB_BASE_URL` | GitHub Enterprise Server URL (blank for SaaS) | `""` |
| `GITHUB_APP_ID` | GitHub App ID (for App auth) | - |
| `GITHUB_APP_PRIVATE_KEY_PATH` | Path to GitHub App RSA private key | - |
| `GITHUB_WEBHOOK_SECRET` | HMAC secret for GitHub webhook verification | - |
| `BITBUCKET_USERNAME` | Bitbucket Cloud username | - |
| `BITBUCKET_APP_PASSWORD` | Bitbucket Cloud App Password | - |
| `BITBUCKET_SERVER_BASE_URL` | Bitbucket Data Center instance URL | - |
| `BITBUCKET_SERVER_TOKEN` | Bitbucket Data Center Personal Access Token (PAT) | - |
| `PORT` | HTTP daemon listening port | `8080` |

---

## In-Repository Custom Instructions (`.pr-agent.yaml`)

Repositories can define project-specific review instructions by placing a `.pr-agent.yaml` in the repo root:

```yaml
review:
  extra_instructions: |
    - Ensure all database queries specify a context timeout.
    - Validate that exported functions have godoc comments.
    - Check that error wrapping always uses %w.
  max_findings: 5
  publish_inline: true

ignore:
  glob_patterns:
    - "legacy/**"
    - "generated/**"
```

---

## Architecture & Design Decisions

To understand domain terminology and design choices, see:
- [Domain Glossary](GLOSSARY.md)
- [ADR 0001: Single Binary Architecture](docs/adr/0001-single-binary-architecture.md)
- [ADR 0002: Embedded Prompt Templates & Structured JSON](docs/adr/0002-structured-json-and-embedded-templates.md)
- [ADR 0003: Git Provider Abstraction & Separation](docs/adr/0003-git-provider-architecture.md)
- [ADR 0004: Worker Pool & Debounce Cancellation](docs/adr/0004-worker-pool-and-debounce-cancellation.md)
- [ADR 0005: Three-Tier Diff Processing](docs/adr/0005-three-tier-diff-processing.md)
- [ADR 0006: Dual-Auth & Explicit Credentials](docs/adr/0006-dual-auth-and-bitbucket-credentials.md)

---

## License

Apache 2.0
