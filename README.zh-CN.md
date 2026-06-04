# Salt Agent CLI

语言：[English](README.md) | [中文](README.zh-CN.md)

`salt-agent` 是一个面向 Salt 风格 Java 发布流程的一次性 Go CLI。它会拉取配置好的代码仓库，切换到目标环境分支，按 Maven 缓存判断构建发生变化的依赖模块，构建本次请求发布的主模块，准备 staging 目录，通过 `rsync --delete` 同步到远端主机，重启远端服务，并按配置查看远端日志。

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

输出详细部署信息：

```bash
./salt-agent deploy --config configs/agent.yaml --env test --modules example-frank --debug
```

## 配置

复制 `configs/agent.example.yaml` 为 `configs/agent.yaml` 并按实际环境调整：

- `workspace`, `buildRoot`, `stagingDir`, `cacheFile`, `lockDir`
- `jdk.javaHome`
- `maven.executable`, `maven.settings`, `maven.localRepo`
- `ssh.user`, `ssh.host`, `ssh.port`, `ssh.keyFile`
- `rsync.options`
- `environments.<name>.branch`, `environments.<name>.mavenProfile`
- `modules.<name>.repo`, `dependencies`, `remotePath`, `container`, `logFile`, `healthUrl`, `healthTimeout`, `remoteScript`

`buildRoot` 应该指向包含 Maven 聚合 `pom.xml` 的目录。模块仓库会被 clone 到 `buildRoot/<module>`，匹配常见的 Maven `<module>example-frank</module>` 目录结构。

当配置了 `environments.<name>.mavenProfile` 时，对应环境的 Maven install 命令会追加 `-P <mavenProfile>`。

只作为依赖的模块只需要配置 `repo` 和 `packaging`。被请求发布的模块必须配置 `remotePath`，并且至少配置 `container` 或 `remoteScript` 之一。

`modules.<name>.healthTimeout` 默认值为 `2m`，支持 Go duration 格式，例如 `30s`、`2m`、`5m`。

## 行为

- 依赖缓存：在配置的 JSON 缓存文件中保存 `{module, branch, commit}`。如果 commit 没有变化，会跳过依赖模块的 `mvn install`。
- 调试输出：普通部署默认不输出详细命令和 POM 维护日志。使用 `--debug` 输出 `[cmd]`、`[pom]` 和缓存跳过等细节。`--dry-run` 仍会输出命令，因为它是命令预览模式。
- Maven 安全性：所有 Maven install 步骤都会使用跨进程目录锁，避免并发写入同一个本地仓库。
- 同模块抢占：同一个模块启动新的部署时，会抢占旧的部署任务。旧任务会在下一个阶段边界退出。
- 聚合 POM 维护：每次仓库 checkout 后，agent 会确保 `buildRoot/pom.xml` 中包含 `<module>module-name</module>`，缺失时追加到 `<modules>`。
- 远端同步：WAR 文件会先在本地解压，再通过 `rsync --delete` 同步，确保远端已删除的 class 和文件也会被清理。
- 日志：未配置 `healthUrl` 时不会自动 tail 远端日志。agent 会输出英文建议，提示运维人员登录服务器手动检查启动状态。
- 健康检查：配置 `healthUrl` 时，agent 会先输出英文等待启动提示，并轮询该地址直到 HTTP 200 或达到 `healthTimeout`；如果同时配置了 `logFile`，会在健康检查成功或超时后使用 `tail -fn` 打印一次远端日志。如果只配置 `healthUrl` 没有配置 `logFile`，成功时会输出英文提示，说明应用已启动但未配置日志；超时时会输出英文未知状态提示。
