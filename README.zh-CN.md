# Salt Agent CLI

## Register Remote Server

Register a remote server, configure passwordless SSH, and sync the built-in `scripts/` directory:

```bash
./salt-agent register \
  --ssh-dir /data/salt-agent/.ssh \
  --host 10.0.0.1 \
  --port 22 \
  --user root \
  --remote-path /shell/salt-agent \
  --scripts-dir /shell/salt-agent
```

The register command uses native Go SSH/SFTP with password authentication first. It creates `--ssh-dir`, generates `id_rsa` and `id_rsa.pub` when they do not exist, uploads the public key to the remote server's `~/.ssh/authorized_keys`, verifies password SSH, then verifies passwordless SSH with the private key. Existing keys are preserved by default; pass `--regenerate-key` to replace them.

After SSH bootstrap succeeds, register creates the remote salt-agent directory, configures `SALT_AGENT_HOME` and `PATH` through `/etc/profile.d/salt-agent.sh`, uploads every file under local `scripts/` to `<remote-path>/scripts/`, and grants executable permissions to the synced scripts. When `--scripts-dir` is explicitly provided, register uploads that directory's contents directly to `<remote-path>/` instead of adding another `scripts/` level.

Use `--scripts-dir` when scripts live somewhere other than `scripts/`. Use `--dry-run` to preview the generated register actions without connecting.

语言：[English](README.md) | [中文](README.zh-CN.md)

`salt-agent` 是一个面向 Salt 风格 Java 发布流程的一次性 Go CLI。它会拉取配置好的代码仓库，切换到目标环境分支，按 Maven 缓存判断需要构建的模块，准备 staging 目录，通过 `rsync --delete` 同步到远端主机，重启远端服务，并按配置查看远端日志。

## 构建

```bash
go mod tidy
go test ./...
go build -o salt-agent ./cmd/salt-agent
```

在 Windows 上通过 Docker 构建 CentOS 兼容的 Linux amd64 产物：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-centos.ps1 -Version dev
```

生成文件会写入 `build/centos/`：

- `salt-agent`
- `salt-agent.sha256`

## 部署

```bash
./salt-agent deploy \
  --config configs/agent.yaml \
  --env test \
  --modules example-frank
```

部署多个主模块：

```bash
./salt-agent deploy \
  --config configs/agent.yaml \
  --env test \
  --modules example-frank,example-user \
  --concurrency 2
```

预览命令但不实际执行：

```bash
./salt-agent deploy --config configs/agent.yaml --env test --modules example-frank --dry-run
```

## 配置

复制 `configs/agent.example.yaml` 为 `configs/agent.yaml` 并按实际环境调整：

- `workspace`, `buildRoot`, `stagingDir`, `cacheFile`, `lockDir`
- `jdk.javaHome`
- `maven.executable`, `maven.settings`, `maven.localRepo`
- `ssh.user`, `ssh.host`, `ssh.port`, `ssh.keyFile`, `ssh.connectTimeout`
- `rsync.options`, `rsync.retries`, `rsync.retryDelay`
- `environments.<name>.branch`, `environments.<name>.mavenProfile`
- `modules.<name>.repo`, `dependencies`, `remotePath`, `container`, `remoteScript`

`buildRoot` 应该指向包含 Maven 聚合 `pom.xml` 的目录。模块仓库会被 clone 到 `buildRoot/<module>`，匹配常见的 Maven `<module>example-frank</module>` 目录结构。

当配置了 `environments.<name>.mavenProfile` 时，对应环境的 Maven install 命令会追加 `-P <mavenProfile>`。

只作为依赖的模块只需要配置 `repo` 和 `packaging`。被请求发布的模块必须配置 `remotePath`，并且至少配置 `container` 或 `remoteScript` 之一。

网络不稳定时，可以配置 SSH 连接超时和 rsync 重试：

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

`ssh.connectTimeout` 会转换为 SSH 的 `ConnectTimeout`。`rsync.retries` 表示首次失败后的额外重试次数。

## 行为

- 构建缓存：在配置的 JSON 缓存文件中保存 `{module, branch, commit}`。未变化的依赖模块会跳过 `mvn install`；只有所有依赖模块和主模块都命中缓存时，主模块才会跳过 `mvn install`。
- 调试输出：普通部署默认不输出详细命令和 POM 维护日志。使用 `--debug` 输出 `[cmd]`、`[pom]` 和缓存跳过等细节。`--dry-run` 仍会输出命令，因为它是命令预览模式。
- Maven 安全性：所有 Maven install 步骤都会使用跨进程目录锁，避免并发写入同一个本地仓库。
- 同模块抢占：同一个模块启动新的部署时，会抢占旧的部署任务。旧任务会在下一个阶段边界退出。
- 聚合 POM 维护：每次仓库 checkout 后，agent 会确保 `buildRoot/pom.xml` 中包含 `<module>module-name</module>`，缺失时追加到 `<modules>`。
- 远端同步：WAR 文件会先在本地解压，再通过 SSH 上的 `rsync --delete` 同步，确保远端已删除的 class 和文件也会被清理。SSH 可按配置使用连接超时，rsync 可对临时失败进行重试。
