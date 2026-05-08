# Rollback Procedure — TaskFlow API

## When to Use
Production incident: `/api/v1/stats` returning wrong data, application crash, or any critical failure after deployment.

## Prerequisites
- Docker installed
- Access to GHCR (GitHub Container Registry)
- `DATABASE_URL` environment variable configured
- Known good image tag (e.g., previous `sha-xxxxx` or `stable`)

## Steps

### 1. Detect Problem
- Check `/health` endpoint: `curl http://localhost:8080/health`
- Check `/api/v1/stats` endpoint: `curl http://localhost:8080/api/v1/stats`
- Review application logs: `docker logs taskflow-api`

### 2. Identify Target Version
- List available images: `docker images | grep taskflow-api`
- Or check GHCR: https://github.com/Trenttzzz/taskflow-cicd-devops/pkgs/container/taskflow-api
- Note the last known good SHA tag (e.g., `sha-a3f2c1d`)

### 3. Execute Rollback
```bash
export DATABASE_URL=postgres://taskflow:taskflow_secret@localhost:5432/taskflow?sslmode=disable
make rollback ROLLBACK_TAG=sha-a3f2c1d
```

### 4. Verify Rollback
- Health check: `curl http://localhost:8080/health` → should return 200
- Stats check: `curl http://localhost:8080/api/v1/stats` → should return correct data
- Application logs: `docker logs taskflow-api` → no errors

### 5. Post-Rollback
- Notify team via Slack/Telegram
- Create incident report
- Fix bug in development branch
- Deploy fix through normal CI/CD pipeline

## Emergency: Rollback to Stable
If unsure which version is good:
```bash
make rollback ROLLBACK_TAG=stable
```
Note: `stable` tag only updates when full pipeline passes (CI + CD + smoke test all green).
