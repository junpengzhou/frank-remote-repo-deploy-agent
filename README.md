# Spug Agent CLI

`spug-agent` is a one-shot Go CLI for Spug-style Java deployments. It checks out configured repositories, switches them to the environment branch, builds changed dependency modules with Maven cache awareness, builds the requested main module, prepares a staging directory, syncs it to the remote host with `rsync --delete`, restarts the remote service, and prints remote logs.

## Build

```bash
go mod tidy
go test ./...
go build -o spug-agent ./cmd/spug-agent
```

## Deploy

```bash
./spug-agent deploy \
  --config configs/agent.yaml \
  --env test \
  --modules example-frank \
  --tail
```

Deploy multiple main modules:

```bash
./spug-agent deploy \
  --config configs/agent.yaml \
  --env test \
  --modules example-frank,example-user \
  --concurrency 2
```

Preview commands without running them:

```bash
./spug-agent deploy --config configs/agent.yaml --env test --modules example-frank --dry-run
```

## Configuration

Copy `configs/agent.example.yaml` to `configs/agent.yaml` and adjust:

- `workspace`, `buildRoot`, `stagingDir`, `cacheFile`, `lockDir`
- `jdk.javaHome`
- `maven.executable`, `maven.settings`, `maven.localRepo`
- `ssh.user`, `ssh.host`, `ssh.port`, `ssh.keyFile`
- `rsync.options`
- `environments.<name>.branch`
- `modules.<name>.repo`, `dependencies`, `remotePath`, `container`, `logFile`, `remoteScript`

`buildRoot` should be the directory containing the Maven aggregator `pom.xml`. Module repositories are cloned into `buildRoot/<module>`, matching normal Maven `<module>example-frank</module>` layout.

Dependency-only modules only need `repo` and `packaging`. Requested deployment modules must define `remotePath` and either `container` or `remoteScript`.

## Behavior

- Dependency cache: stores `{module, branch, commit}` in the configured JSON cache. If the commit did not change, dependency `mvn install` is skipped.
- Maven safety: all Maven install steps use a cross-process directory lock to avoid concurrent writes to the same local repository.
- Same-module preemption: starting a new deployment for the same module supersedes the older run. The older run exits at the next stage boundary.
- Aggregator POM maintenance: after each repository checkout, the agent ensures `buildRoot/pom.xml` contains `<module>module-name</module>` and appends it to `<modules>` when missing.
- Remote sync: WAR files are extracted locally, then synchronized with `rsync --delete` so removed classes and files are also removed remotely.
- Logs: use `--tail` to follow remote logs after restart; omit it to print the last configured line count once.
