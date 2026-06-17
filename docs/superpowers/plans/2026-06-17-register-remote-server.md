# Register Remote Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `salt-agent register` command that registers a remote server with password-based native SSH/SFTP and syncs built-in scripts to the requested remote path.

**Architecture:** Add an `internal/register` package with a testable `Client` interface and a `Registrar` orchestrator. The CLI parses register flags, builds a native SSH/SFTP client, runs the generated install script remotely, uploads all files from local `scripts/` to `<remote-path>/scripts`, and sets executable permissions.

**Tech Stack:** Go 1.22, `golang.org/x/crypto/ssh`, `github.com/pkg/sftp`, existing `flag` CLI style, Go unit tests with fake clients.

---

### Task 1: Register Orchestrator

**Files:**
- Create: `internal/register/register.go`
- Test: `internal/register/register_test.go`

- [ ] **Step 1: Write failing tests**

Create tests for validation, generated install script content, command order, upload target paths, and executable chmod behavior.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/register`
Expected: FAIL because `internal/register` does not exist.

- [ ] **Step 3: Implement minimal orchestrator**

Define `Options`, `Client`, `Registrar`, `Run`, and install script generation. Keep network operations behind the `Client` interface.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/register`
Expected: PASS.

### Task 2: Native SSH/SFTP Client

**Files:**
- Create: `internal/register/ssh_client.go`
- Modify: `go.mod`
- Test: `internal/register/ssh_client_test.go`

- [ ] **Step 1: Write failing tests**

Test SSH client config defaults, password auth wiring, timeout, and host-key callback behavior without opening a network connection.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/register`
Expected: FAIL because the native client constructor is missing.

- [ ] **Step 3: Implement minimal native client**

Use `golang.org/x/crypto/ssh` and `github.com/pkg/sftp` for `Run`, `UploadFile`, `MkdirAll`, `Chmod`, and `Close`.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/register`
Expected: PASS.

### Task 3: CLI Integration And Docs

**Files:**
- Modify: `cmd/salt-agent/main.go`
- Modify: `README.md`
- Modify: `README.zh-CN.md`

- [ ] **Step 1: Write failing CLI tests if feasible**

If the existing CLI has no test harness, add focused tests for argument validation in a small helper rather than invoking real SSH.

- [ ] **Step 2: Implement command wiring**

Add `register` to command dispatch with flags: `--ssh-dir`, `--host`, `--port`, `--user`, `--password`, `--remote-path`, `--scripts-dir`, `--dry-run`, and `--debug`.

- [ ] **Step 3: Update docs**

Document the new command and parameters in both READMEs.

- [ ] **Step 4: Verify all tests**

Run: `go test ./...`
Expected: PASS.
