# AI Project Memory

## Project Identity

- Repo: `frank-remote-repo-deploy-agent`
- Binary: `salt-agent`
- Language: Go
- Type: one-shot deployment CLI, not a daemon or HTTP service
- Main purpose: deploy Java modules to remote Linux hosts in a Salt-style flow

## What The Tool Does

`salt-agent` has 2 commands:

- `deploy`: clone/update repos, checkout env branch, build changed Maven modules, prepare staging, `rsync --delete` to remote host, then restart service remotely
- `register`: bootstrap a remote server for `salt-agent` by setting up SSH keys, installing shell profile env, and uploading local scripts

## Entry Points

- CLI entry: [cmd/salt-agent/main.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/cmd/salt-agent/main.go)
- Deploy orchestrator: [internal/deploy/deploy.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/deploy/deploy.go)
- Register orchestrator: [internal/register/register.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/register/register.go)
- Config model: [internal/config/config.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/config/config.go)

## Mental Model

This repo is an orchestration wrapper around external tools, not a business-logic-heavy application.

Core dependencies are shell tools and remote infra:

- `git` for source sync
- `mvn` for Java builds
- `rsync` for remote file sync
- `ssh` for restart and verification
- native Go SSH/SFTP for `register`

The most important code is command planning, config validation, caching, and concurrency control.

## Deploy Flow

For each requested main module:

1. Parse CLI flags and load YAML config.
2. Resolve environment -> branch.
3. Acquire same-module lease via `internal/lock`.
4. Build module set = dependencies + main module.
5. Ensure each repo exists locally and checkout the target branch.
6. Ensure aggregator `buildRoot/pom.xml` contains `<module>...</module>`.
7. For dependency modules, compare `(module, branch, HEAD)` with JSON cache.
8. Rebuild only changed dependencies under a global Maven lock.
9. Rebuild main module unless both:
   - all dependencies were cache hits
   - main module HEAD was also a cache hit
10. Find artifact from module `target/`.
11. Prepare staging:
   - `war`: unzip into staging dir
   - `jar`: copy artifact into staging dir
12. `rsync --delete` staging to remote path, with retry support.
13. Restart remote app by:
   - `remoteScript` if configured
   - otherwise `docker restart <container>`

## Register Flow

1. Parse register flags.
2. Ensure local SSH keypair exists in `--ssh-dir`.
3. Connect with password auth using native Go SSH client.
4. Upload public key to remote `~/.ssh/authorized_keys`.
5. Verify password SSH still works.
6. Install `/etc/profile.d/salt-agent.sh` with:
   - `SALT_AGENT_HOME`
   - PATH extension for `<remote-path>` and `<remote-path>/scripts`
7. Upload local scripts directory to remote target.
8. `chmod -R a+rx` remote scripts.
9. Reconnect using private key and verify passwordless SSH.
10. Also verify passwordless login through local OpenSSH `ssh`.

## Key Packages

- `internal/config`: YAML structs, defaults, validation, env/module resolution
- `internal/deploy`: end-to-end deployment orchestration
- `internal/register`: remote bootstrap and script sync
- `internal/gitops`: clone/fetch/checkout and HEAD lookup
- `internal/maven`: Maven command construction
- `internal/packagex`: artifact discovery and staging preparation
- `internal/rsync`: `rsync` command construction
- `internal/remote`: remote `ssh` command construction
- `internal/cache`: JSON build cache keyed by module + branch + commit
- `internal/lock`: module preemption locks + global exclusive locks
- `internal/runner`: external command execution with live output and dry-run support
- `internal/pomxml`: ensure aggregator POM contains expected module entries
- `internal/output`: structured console output and debug logging

## Important Invariants

- Requested deploy modules must define `remotePath`.
- Requested deploy modules must define either `container` or `remoteScript`.
- Dependency-only modules may omit remote deployment fields.
- Supported packaging is only `war` or `jar`.
- `buildRoot` is expected to be the Maven aggregator root.
- Local clones live under `buildRoot/<module>`.
- Environment selection only changes branch and optional Maven profile.
- `deploy` is safe for multiple main modules concurrently, but Maven installs are serialized.

## Concurrency And Safety

- Multiple different main modules can deploy concurrently through `--concurrency`.
- Same-module deployments are preemptive:
  - newer deployment supersedes older one
  - older run exits at next stage boundary via lease checks
- Maven install steps are globally locked to protect shared local `.m2`.
- Aggregator POM mutation is also globally locked.

## Cache Rules

- Cache file stores build state by module + branch + commit.
- Dependency cache hit => skip `mvn install` for that dependency.
- Main module cache hit only skips install when:
  - main HEAD unchanged
  - all dependencies also cache-hit
- Cache is local JSON, not remote or shared service state.

## Remote Execution Rules

- File deployment uses `rsync --delete`, so remote stale files are intentionally removed.
- Restart priority is:
  - custom `remoteScript`
  - fallback `docker restart`
- `remoteScript` can receive `--user <operator>` from CLI `deploy --user`.

## Config Shape

See example: [configs/agent.example.yaml](/E:/01-Code/Github/frank-remote-repo-deploy-agent/configs/agent.example.yaml)

Top-level fields:

- `workspace`
- `buildRoot`
- `stagingDir`
- `cacheFile`
- `lockDir`
- `jdk`
- `maven`
- `ssh`
- `rsync`
- `environments`
- `modules`

Per-environment:

- `branch`
- optional `mavenProfile`

Per-module:

- `repo`
- `packaging`
- `dependencies`
- `remotePath`
- `container`
- `remoteScript`

## Behavior That Is Easy To Miss

- `gitops.Checkout()` does `fetch`, `checkout -B`, `reset --hard`, and `clean -ffd` inside local cloned module dirs.
- Those destructive git operations target the deployment workspace clones under `buildRoot`, not the developer's current repo checkout.
- `register` uploads local `scripts/` under `<remote-path>/scripts/` by default.
- If `--scripts-dir` is explicitly provided, files are uploaded directly under `<remote-path>/`, without an extra `scripts/` level.
- `register` preserves existing SSH keys unless `--regenerate-key` is set.
- `connectTimeout` affects SSH command building.
- `rsync.retries` means extra retries after the first failure.

## File Map For Fast Navigation

- Command dispatch: [cmd/salt-agent/main.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/cmd/salt-agent/main.go)
- Deploy pipeline: [internal/deploy/deploy.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/deploy/deploy.go)
- Register pipeline: [internal/register/register.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/register/register.go)
- Native SSH client: [internal/register/ssh_client.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/register/ssh_client.go)
- Git wrapper: [internal/gitops/gitops.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/gitops/gitops.go)
- Maven command builder: [internal/maven/maven.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/maven/maven.go)
- Packaging/staging: [internal/packagex/package.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/packagex/package.go)
- Rsync command builder: [internal/rsync/rsync.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/rsync/rsync.go)
- Remote SSH command builder: [internal/remote/remote.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/remote/remote.go)
- Locking: [internal/lock/lock.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/lock/lock.go)
- Build cache: [internal/cache/cache.go](/E:/01-Code/Github/frank-remote-repo-deploy-agent/internal/cache/cache.go)

## Current Evolution Signals

Recent commit themes indicate the repo is actively evolving around:

- deployment flow simplification
- remote restart plugin support
- Salt-Agent container restart support
- register flow refinement
- build cache optimization

When changing behavior, inspect recent commits first because deployment assumptions may still be moving.

## Recommended AI Working Strategy

When asked to change this repo:

1. Read `AGENTS.md` first.
2. Identify whether the task belongs to `deploy`, `register`, or shared infra.
3. Read only the relevant internal package plus its tests.
4. Check config and README only if CLI or behavior contracts are involved.
5. Run targeted tests first, then `go test ./...` if the change is broad.

## Useful Commands

```powershell
go test ./...
go build -o salt-agent ./cmd/salt-agent
go test ./internal/deploy ./internal/register ./internal/config
```

CentOS build from Windows:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-centos.ps1 -Version dev
```

## Do Not Assume

- There is no long-running service state.
- There is no database.
- There is no web UI.
- `deploy` success depends heavily on external environment correctness: Git access, Maven/JDK, SSH, rsync, remote scripts, remote Docker.
- Changes in command builders or config validation can have real deployment impact; verify carefully.
