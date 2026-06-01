# Spug Agent CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a robust Go CLI for one-shot Spug deployments with configurable source checkout, Maven caching, artifact staging, rsync sync, remote restart, and log tail.

**Architecture:** The command entrypoint delegates to small internal packages for config, command execution, cache, locks, Maven, rsync, remote SSH, packaging, and orchestration. External commands are built in focused packages and executed through a cancellable runner.

**Tech Stack:** Go, Cobra-compatible standard flag parsing through `flag`, YAML via `gopkg.in/yaml.v3`, JSON cache files, OS command execution, git, Maven, rsync, SSH.

---

### Task 1: Go Module And Config

**Files:**
- Create: `go.mod`
- Create: `cmd/spug-agent/main.go`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] Write failing tests for valid config loading, missing module validation, and environment branch resolution.
- [ ] Run `go test ./internal/config` and verify the package does not compile before implementation.
- [ ] Implement YAML structs, `LoadFile`, `Validate`, `BranchForEnv`, and `ModuleNames`.
- [ ] Run `go test ./internal/config` and verify it passes.

### Task 2: Cache And Locks

**Files:**
- Create: `internal/cache/cache.go`
- Test: `internal/cache/cache_test.go`
- Create: `internal/lock/lock.go`
- Test: `internal/lock/lock_test.go`

- [ ] Write failing tests for cache hit, cache miss, cache update persistence, lock acquisition, and supersede detection.
- [ ] Run `go test ./internal/cache ./internal/lock` and verify expected failures.
- [ ] Implement JSON cache storage and generation-based module locks.
- [ ] Run `go test ./internal/cache ./internal/lock` and verify it passes.

### Task 3: Command Builders

**Files:**
- Create: `internal/maven/maven.go`
- Test: `internal/maven/maven_test.go`
- Create: `internal/rsync/rsync.go`
- Test: `internal/rsync/rsync_test.go`
- Create: `internal/remote/remote.go`
- Test: `internal/remote/remote_test.go`

- [ ] Write failing tests for Maven build args, rsync args, SSH script args, Docker restart args, and tail args.
- [ ] Run package tests and verify expected failures.
- [ ] Implement command builders without executing commands.
- [ ] Run package tests and verify they pass.

### Task 4: Runner, Git, Packaging, And Deploy Orchestration

**Files:**
- Create: `internal/runner/runner.go`
- Create: `internal/gitops/gitops.go`
- Create: `internal/packagex/package.go`
- Test: `internal/packagex/package_test.go`
- Create: `internal/deploy/deploy.go`

- [ ] Write failing tests for artifact resolution and staging path decisions.
- [ ] Run `go test ./internal/packagex` and verify expected failures.
- [ ] Implement runner, git command wrappers, packaging helpers, and deployment orchestration.
- [ ] Run `go test ./...` and verify it passes.

### Task 5: CLI, Example Config, And Docs

**Files:**
- Modify: `cmd/spug-agent/main.go`
- Create: `configs/agent.example.yaml`
- Create: `README.md`

- [ ] Add CLI flag parsing for `deploy`, `--config`, `--env`, `--modules`, `--concurrency`, `--dry-run`, and `--tail`.
- [ ] Add example config matching the requested Spug deployment flow.
- [ ] Document install, build, config, and common deployment examples.
- [ ] Run `gofmt`, `go test ./...`, and `go build ./cmd/spug-agent`.
