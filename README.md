# Salt Agent CLI

Languages: [English](README.md) | [中文](README.zh-CN.md)

`salt-agent` is a one-shot Go CLI for Salt-style Java deployments. It checks out configured repositories, switches them to the environment branch, builds changed dependency modules with Maven cache awareness, builds the requested main module, prepares a staging directory, syncs it to the remote host with `rsync --delete`, restarts the remote service, and prints remote logs.

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

Register a remote server and sync the built-in `scripts/` directory:

```bash
./salt-agent register \
  --ssh-dir ~/.ssh \
  --host 10.0.0.1 \
  --port 22 \
  --user root \
  --password 'secret' \
  --remote-path /data/salt-agent
```

The register command uses native Go SSH/SFTP with password authentication. It creates the remote salt-agent directory, configures `SALT_AGENT_HOME` and `PATH` through `/etc/profile.d/salt-agent.sh`, uploads every file under local `scripts/` to `<remote-path>/scripts/`, and grants executable permissions to the synced scripts.

Use `--scripts-dir` when scripts live somewhere other than `scripts/`. Use `--dry-run` to preview the generated register actions without connecting.

## Configuration

Copy `configs/agent.example.yaml` to `configs/agent.yaml` and adjust:

- `workspace`, `buildRoot`, `stagingDir`, `cacheFile`, `lockDir`
- `jdk.javaHome`
- `maven.executable`, `maven.settings`, `maven.localRepo`
- `ssh.user`, `ssh.host`, `ssh.port`, `ssh.keyFile`
- `rsync.options`
- `environments.<name>.branch`, `environments.<name>.mavenProfile`
- `modules.<name>.repo`, `dependencies`, `remotePath`, `container`, `logFile`, `healthUrl`, `healthTimeout`, `remoteScript`

`buildRoot` should be the directory containing the Maven aggregator `pom.xml`. Module repositories are cloned into `buildRoot/<module>`, matching normal Maven `<module>example-frank</module>` layout.

When `environments.<name>.mavenProfile` is configured, Maven install commands for that environment include `-P <mavenProfile>`.

Dependency-only modules only need `repo` and `packaging`. Requested deployment modules must define `remotePath` and either `container` or `remoteScript`.

`modules.<name>.healthTimeout` defaults to `2m` and accepts Go duration values such as `30s`, `2m`, or `5m`.

## Behavior

- Dependency cache: stores `{module, branch, commit}` in the configured JSON cache. If the commit did not change, dependency `mvn install` is skipped.
- Debug output: normal deploys keep verbose command and POM maintenance logs quiet. Use `--debug` to print `[cmd]`, `[pom]`, and cache skip details. `--dry-run` still prints commands because it is a command preview mode.
- Maven safety: all Maven install steps use a cross-process directory lock to avoid concurrent writes to the same local repository.
- Same-module preemption: starting a new deployment for the same module supersedes the older run. The older run exits at the next stage boundary.
- Aggregator POM maintenance: after each repository checkout, the agent ensures `buildRoot/pom.xml` contains `<module>module-name</module>` and appends it to `<modules>` when missing.
- Remote sync: WAR files are extracted locally, then synchronized with `rsync --delete` so removed classes and files are also removed remotely.
- Logs: when `healthUrl` is not configured, remote logs are not tailed automatically. The agent prints an English suggestion so operators can log in to the server and check startup status manually.
- Health checks: when `healthUrl` is configured, the agent prints an English startup wait message, polls until HTTP 200 or `healthTimeout`, then prints remote logs once with `tail -fn` if `logFile` is configured. If `healthUrl` is configured without `logFile`, success prints an English message indicating the app started and that logs are not configured; timeout prints an English unknown-status message.
