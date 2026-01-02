# GitHub Pull Request AI Teammate

An AI-powered system that reviews GitHub pull requests by combining rule-based checks, static analysis, and LLM reasoning. It posts inline comments plus a summary review, and adapts to team conventions over time.

## What This System Does (Concrete Scope)
When a pull request is opened or updated:

1. GitHub sends a webhook to the backend.
2. The service fetches PR diffs and metadata.
3. It analyzes code changes with:
   - Rule-based checks
   - Static analysis (language-aware)
   - AI-based reasoning
4. It posts review comments back on the PR:
   - Inline comments on specific lines
   - A summary comment covering design, performance, and security
5. It learns over time from accepted/ignored feedback to align with team conventions.

## High-Level Architecture
```
GitHub PR
   ↓ (Webhook)
Webhook API
   ↓
PR Analysis Orchestrator
   ↓
┌───────────────┬────────────────┬────────────────┐
│ Rule Engine   │ Static Analysis │ AI Reviewer    │
└───────────────┴────────────────┴────────────────┘
   ↓
Comment Generator
   ↓
GitHub Review API
```

## GitHub Integration (First Thing to Build)
Use a **GitHub App** (not OAuth) for:

- Webhooks
- Repo-level permissions
- Automated PR reviews

### Required Permissions
- Pull Requests: **Read & Write**
- Contents: **Read**
- Metadata: **Read**

### Webhooks to Subscribe To
- `pull_request.opened`
- `pull_request.synchronize`
- `pull_request.reopened`

## Backend Design
Suggested stack:

| Layer | Tech |
| --- | --- |
| API | Go |
| Async Jobs | Redis + worker |
| AI | OpenAI-compatible API |
| Storage | PostgreSQL |
| Queue | Redis / SQS |
| Hosting | Docker + cloud |

### API Endpoints
- `POST /webhook/github`
- `GET  /health`
- `POST /analyze/pr`

## PR Analysis Orchestrator (The Brain)
**Inputs**
- PR number
- Repo
- Commit SHA

**Steps**
1. Fetch PR diff
2. Split diff by file
3. Classify files (test, prod, config)
4. Dispatch analysis jobs
5. Aggregate results
6. Generate review comments

## Rule-Based Engine (Non-AI, Very Important)
Examples of rules:

- Functions > 50 lines
- Missing error handling
- TODOs in production code
- Logging secrets
- SQL without parameterization
- Missing tests

## Static Code Analysis (Language-Aware)
Use real parsers, not regex.

| Language | Tool |
| --- | --- |
| JS/TS | Tree-sitter / ESLint AST |
| Python | `ast` module |
| Go | `go/parser` |
| Java | JavaParser |

**Output format example**
```json
{
  "file": "auth.go",
  "issues": [
    {
      "line": 42,
      "type": "error-handling",
      "detail": "Returned error is ignored"
    }
  ]
}
```

## AI Reviewer
**Inputs**
- PR title + description
- Diff (chunked)
- Rule findings
- Repo context (README, conventions)

### Prompt Strategy (Critical)
Never ask: “Review this code.”

Always ask structured questions:

> You are a senior software engineer.
>
> Review this PR for:
> 1. Architectural concerns
> 2. Performance risks
> 3. Security issues
> 4. Maintainability
> 5. API design
>
> For each issue:
> - Explain why it matters
> - Suggest a concrete improvement
> - Reference specific lines

## Comment Generator (Human-Like Output)
**Inline comment example**

```
⚠️ Potential Performance Issue
This loop performs a DB query per iteration, which may not scale. Consider batching or caching results.
```

**Summary comment example**
```
## AI Review Summary

👍 Strengths:
- Clear separation of concerns
- Good test coverage

⚠️ Concerns:
- Error handling in auth flow
- N+1 query pattern

💡 Suggestions:
- Add integration tests
- Introduce request-level caching
```

## Learning Team Conventions (Advanced Feature)
Store:
- Approved PRs
- Ignored warnings
- Review comments accepted/rejected

Use this data to:
- Reduce noise
- Adapt tone
- Match repo standards

**Data model**
```
review_feedback (
  repo,
  rule_id,
  accepted BOOLEAN
)
```

## Database Schema (Minimal but Real)
```
pull_requests (
  id,
  repo,
  pr_number,
  status,
  created_at
)

analysis_results (
  pr_id,
  file,
  issue_type,
  severity,
  message
)
```

## CI/CD + DevOps
- Dockerize everything
- GitHub Actions for deploy
- Health checks

### Metrics to Track
- PRs reviewed
- Avg analysis time
- AI token usage

## Getting Started (Local)
Run the HTTP API locally:

```bash
go run ./cmd/server
```

Example requests:

```bash
curl -s http://localhost:8080/health

curl -s -X POST http://localhost:8080/analyze/pr \\
  -H 'Content-Type: application/json' \\
  -d '{\"repository\":\"acme/repo\",\"pull_number\":42,\"commit_sha\":\"abc123\"}'
```
