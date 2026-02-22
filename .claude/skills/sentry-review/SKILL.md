---
name: sentry-review
description: Review and triage Sentry issues for terrty.net infrastructure. Use when user asks to check Sentry errors, review error reports, triage issues, or mentions Sentry. Pulls unresolved issues from last 14 days, classifies them by category, and provides actionable recommendations.
allowed-tools: Bash(curl:*), Bash(python3:*)
---

# Sentry Issue Review

## Configuration

- **Org:** `ksenia-gulyaeva`
- **Project:** `terrty`
- **Auth:** `~/.sentry/terrty.token`
- **API base:** `https://sentry.io/api/0/projects/ksenia-gulyaeva/terrty`

## API Notes

- **Use org-level endpoints** for issue events and mutations — project-level event endpoints return 404 for this token
- All mutation endpoints (archive, resolve) also use org-level path

## Workflow

### 1. Fetch unresolved issues (last 14 days)

```bash
TOKEN=$(cat ~/.sentry/terrty.token | tr -d '\n')
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://sentry.io/api/0/projects/ksenia-gulyaeva/terrty/issues/?query=is:unresolved&statsPeriod=14d&limit=100"
```

### 2. For each issue, fetch latest event details

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://sentry.io/api/0/organizations/ksenia-gulyaeva/issues/{ISSUE_ID}/events/?limit=1"
```

Key fields in event response:
- `tags[]` — blocked-uri, effective-directive, url, browser, client_os
- `user.ip_address`, `user.geo` — origin of the event
- `metadata` — directive, message, uri for CSP issues

### 3. Classify issues into categories

**Categories:**

| Category | Description | Action |
|----------|-------------|--------|
| INFRA | Docker, networking, DNS, TLS, resource exhaustion | Investigate — infrastructure issue |
| SERVICE_DOWN | Container crash, health check failure, OOM | Investigate — service availability |
| APP_BUG | Application errors, unhandled exceptions in our code | Investigate — real bugs |
| NETWORK_TRANSIENT | Timeouts, connection resets, DNS failures (low count) | Archive if low frequency, investigate if persistent |
| THIRD_PARTY | Errors in external services or libraries we don't control | Archive if confirmed third-party |
| PERFORMANCE | Slow queries, high latency, resource bottlenecks | Investigate if impacting users |
| UNKNOWN | No stack trace or insufficient data | Fetch event details to classify |

**Classification rules (apply in order):**

1. Title contains container/Docker/OOM/health check references → `SERVICE_DOWN`
2. Title contains TLS/certificate/DNS/nginx/proxy errors → `INFRA`
3. Title contains `timeout`, `connection reset`, `connection refused` (low count) → `NETWORK_TRANSIENT`
4. Title contains unhandled exception with stack trace in our code → `APP_BUG`
5. Title references external API failures (e.g. third-party webhooks) → `THIRD_PARTY`
6. Title contains slow query, high latency, memory pressure → `PERFORMANCE`
7. Insufficient data → `UNKNOWN`, fetch event details

### 4. Present results

Output a summary table grouped by category, with:
- Issue ID (linked to Sentry if possible)
- Event count and affected user count
- First/last seen dates
- Title (truncated)
- Recommended action

Then list specific action items:
- **Archive candidates** — issues safe to ignore (with user confirmation before archiving)
- **Investigate** — real bugs or infra issues with brief context from stack trace/logs

### 5. Archiving and resolving issues (only with user approval)

**Default: archive unless 1000 users hit it (from now).** This is the standard action for noise — propose it by default for all archive candidates:
```bash
TOKEN=$(cat ~/.sentry/terrty.token | tr -d '\n')
curl -s -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "ignored", "statusDetails": {"ignoreUserCount": 1000}}' \
  "https://sentry.io/api/0/organizations/ksenia-gulyaeva/issues/{ISSUE_ID}/"
```

**Permanently ignore** (only for issues that can never be actionable):
```bash
curl -s -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "ignored", "statusDetails": {}}' \
  "https://sentry.io/api/0/organizations/ksenia-gulyaeva/issues/{ISSUE_ID}/"
```

**Always confirm with user before archiving.** Present the list and ask for approval.

### 6. Resolve fixed issues

After investigating and fixing an issue (config change, code fix, CSP update), **prompt the user to resolve it in Sentry**. Don't wait for a separate request — once a fix is deployed or ready to deploy, ask: "Want me to resolve these fixed issues in Sentry?"

```bash
curl -s -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}' \
  "https://sentry.io/api/0/organizations/ksenia-gulyaeva/issues/{ISSUE_ID}/"
```

Track which issues were addressed during the session and present them as a batch for resolution at the end.

## Sentry Web UI Links

- **Issue detail:** `https://ksenia-gulyaeva.sentry.io/issues/{ISSUE_ID}/`
- **Project issues:** `https://ksenia-gulyaeva.sentry.io/issues/?project=terrty`
