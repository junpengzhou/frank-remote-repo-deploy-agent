# Salt Agent CLI

Languages: [English](README.md) | [中文](README.zh-CN.md)

`salt-agent` is a one-shot Go CLI for Salt-style Java deployments. It checks out configured repositories, switches them to the environment branch, builds changed modules with Maven cache awareness, prepares a staging directory, syncs it to the remote host with `rsync --delete`, restarts the remote service, and prints remote logs.

## Build

```bash
go mod tidy
go test ./...
go build -o salt-agent ./cmd/salt-agent
```

Build a CentOS-compatible Linux amd64 artifact from Windows with Docker:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-centos.ps1 -Version dev
```

The generated files are written to `build/centos/`:

- `salt-agent`
- `salt-agent.sha256`

## Deploy

```bash
./salt-agent deploy \
  --config configs/agent.yaml \
  --env test \
  --modules example-frank
```

Deploy multiple main modules:

```bash
./salt-agent deploy \
  --config configs/agent.yaml \
  --env test \
  --modules example-frank,example-user \
  --concurrency 2
```

Preview commands without running them:

```bash
./salt-agent deploy --config configs/agent.yaml --env test --modules example-frank --dry-run
```

## Register Remote Server

Register a remote server, configure passwordless SSH, and sync the built-in `scripts/` directory:

```bash
./salt-agent register \
  --ssh-dir /data/salt-agent/.ssh \
  --host 10.0.0.1 \
  --port 22 \
  --user root \
  --password 'secret' \
  --remote-path /shell/salt-agent \
  --scripts-dir /shell/salt-agent
```

The register command uses native Go SSH/SFTP with password authentication first. It creates `--ssh-dir`, generates `id_rsa` and `id_rsa.pub` when they do not exist, uploads the public key to the remote server's `~/.ssh/authorized_keys`, verifies password SSH, then verifies passwordless SSH with the private key. Existing keys are preserved by default; pass `--regenerate-key` to replace them.

After SSH bootstrap succeeds, register creates the remote salt-agent directory, configures `SALT_AGENT_HOME` and `PATH` through `/etc/profile.d/salt-agent.sh`, uploads every file under local `scripts/` to `<remote-path>/scripts/`, and grants executable permissions to the synced scripts. When `--scripts-dir` is explicitly provided, register uploads that directory's contents directly to `<remote-path>/` instead of adding another `scripts/` level.

Use `--scripts-dir` when scripts live somewhere other than `scripts/`. Use `--dry-run` to preview the generated register actions without connecting.

## Configuration

Copy `configs/agent.example.yaml` to `configs/agent.yaml` and adjust:

- `workspace`, `buildRoot`, `stagingDir`, `cacheFile`, `lockDir`
- `jdk.javaHome`
- `maven.executable`, `maven.settings`, `maven.localRepo`
- `ssh.user`, `ssh.host`, `ssh.port`, `ssh.keyFile`, `ssh.connectTimeout`
- `rsync.options`, `rsync.retries`, `rsync.retryDelay`
- `environments.<name>.branch`, `environments.<name>.mavenProfile`
- `modules.<name>.repo`, `dependencies`, `remotePath`, `container`, `remoteScript`

`buildRoot` should be the directory containing the Maven aggregator `pom.xml`. Module repositories are cloned into `buildRoot/<module>`, matching normal Maven `<module>example-frank</module>` layout.

When `environments.<name>.mavenProfile` is configured, Maven install commands for that environment include `-P <mavenProfile>`.

Dependency-only modules only need `repo` and `packaging`. Requested deployment modules must define `remotePath` and either `container` or `remoteScript`.

For unstable networks, configure the SSH connection timeout and rsync retries:

```yaml
ssh:
  connectTimeout: 10s

rsync:
  executable: rsync
  options:
    - -az
    - --delete
    - --partial
  retries: 3
  retryDelay: 5s
```

`ssh.connectTimeout` is passed to SSH as `ConnectTimeout`. `rsync.retries` is the number of extra rsync attempts after the first failure.

## Behavior

- Build cache: stores `{module, branch, commit}` in the configured JSON cache. Unchanged dependencies skip `mvn install`; the main module skips `mvn install` only when every dependency and the main module are cache hits.
- Build metadata: each staged deployment includes [`salt-agent-metadata.json`](docs/salt-agent-metadata.md) with build timing and up to three recent commits for the main module and every configured dependency.
- Debug output: normal deploys keep verbose command and POM maintenance logs quiet. Use `--debug` to print `[cmd]`, `[pom]`, and cache skip details. `--dry-run` still prints commands because it is a command preview mode.
- Maven safety: all Maven install steps use a cross-process directory lock to avoid concurrent writes to the same local repository.
- Same-module preemption: starting a new deployment for the same module supersedes the older run. The older run exits at the next stage boundary.
- Aggregator POM maintenance: after each repository checkout, the agent ensures `buildRoot/pom.xml` contains `<module>module-name</module>` and appends it to `<modules>` when missing.
- Remote sync: WAR files are extracted locally, then synchronized over SSH with `rsync --delete` so removed classes and files are also removed remotely. SSH can use a configured connection timeout, and rsync can retry transient failures.
