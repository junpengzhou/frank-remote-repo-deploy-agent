# `salt-agent-metadata.json` 下游接入协议

## 1. 用途

`salt-agent` 在每个主模块完成 Maven 判断/构建后，收集主模块及其配置依赖的 Git 提交信息，生成 `salt-agent-metadata.json`。该文件与应用产物一起进入 staging，并通过现有 `rsync` 流程同步到目标服务器。

下游重启插件、运维页面或应用程序可以读取该文件，用来确认：

- 当前同步的是哪个环境和分支；
- 本次构建是否因为缓存命中而跳过 Maven；
- 构建阶段的开始时间、结束时间和耗时；
- 主模块和依赖模块对应的最近提交；
- 每条提交的完整 hash、提交者和提交时间。

该文件只描述“已同步产物的来源”。它不证明远程重启成功，也不替代健康检查。

机器可读定义见 [JSON Schema](schemas/salt-agent-metadata.schema.json)。

## 2. 文件位置与编码

文件固定命名为 `salt-agent-metadata.json`，使用 UTF-8 编码并以换行符结尾。

- WAR：位于解压后应用目录的根目录。
- JAR：与同步后的 JAR 文件同级。
- 远端：位于模块配置的 `remotePath` 根目录。

示例：当 `remotePath` 是 `/data/apps/example-app/` 时，元数据路径是：

```text
/data/apps/example-app/salt-agent-metadata.json
```

## 3. 完整示例

```json
{
  "schemaVersion": "1.0",
  "generatedAt": "2026-07-17T14:35:13+08:00",
  "environment": "test",
  "branch": "test",
  "mainModule": "example-app",
  "build": {
    "startedAt": "2026-07-17T14:34:01+08:00",
    "finishedAt": "2026-07-17T14:35:12+08:00",
    "durationMs": 71000,
    "buildSkipped": false
  },
  "modules": [
    {
      "name": "example-app",
      "role": "main",
      "commits": [
        {
          "hash": "0123456789abcdef0123456789abcdef01234567",
          "committer": {
            "name": "Frank Zhou",
            "email": "frank@example.com"
          },
          "committedAt": "2026-07-17T13:20:30+08:00",
          "description": "Add build metadata"
        },
        {
          "hash": "89abcdef0123456789abcdef0123456789abcdef",
          "committer": {
            "name": "Developer",
            "email": "developer@example.com"
          },
          "committedAt": "2026-07-16T18:10:00+08:00",
          "description": "Update deployment flow"
        }
      ]
    },
    {
      "name": "example-common",
      "role": "dependency",
      "commits": [
        {
          "hash": "fedcba9876543210fedcba9876543210fedcba98",
          "committer": {
            "name": "Common Maintainer",
            "email": "common@example.com"
          },
          "committedAt": "2026-07-15T09:08:07+08:00",
          "description": "Adjust shared API"
        }
      ]
    }
  ]
}
```

## 4. 字段定义

所有表格中标记为必填的字段都会存在。下游仍应忽略无法识别的新增字段，以便兼容同一主版本内的协议扩展。

### 4.1 顶层字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `schemaVersion` | string | 是 | 协议版本，当前为 `1.0`。 |
| `generatedAt` | RFC 3339 string | 是 | 本次元数据模型生成时间，保留时区偏移。 |
| `environment` | string | 是 | `deploy --env` 传入的环境名称。 |
| `branch` | string | 是 | 由环境配置解析出的 Git 分支。 |
| `mainModule` | string | 是 | 当前文件对应的主模块名称。 |
| `build` | object | 是 | 本次 Maven 判断/构建阶段信息。 |
| `modules` | array | 是 | 主模块及配置依赖的提交信息。至少包含主模块。 |

### 4.2 `build`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `startedAt` | RFC 3339 string | 是 | 进入 Maven 缓存判断/构建阶段的时间。 |
| `finishedAt` | RFC 3339 string | 是 | Maven 阶段成功完成的时间。 |
| `durationMs` | integer | 是 | `finishedAt - startedAt` 的非负毫秒数。 |
| `buildSkipped` | boolean | 是 | 所有依赖和主模块都命中缓存、没有执行 Maven 命令时为 `true`。只要任一模块执行 Maven 即为 `false`。 |

`buildSkipped: true` 时仍会生成新的元数据。缓存判断本身需要时间，因此 `durationMs` 可以大于 `0`。

### 4.3 `modules[]`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 配置中的模块名称。 |
| `role` | string | 是 | 主模块为 `main`，依赖模块为 `dependency`。 |
| `commits` | array | 是 | 从该模块本地 `HEAD` 开始的最近零至三条非 merge 提交。 |

模块顺序固定：

1. 主模块始终是第一项。
2. 依赖模块按照主模块配置中的 `dependencies` 顺序排列。

### 4.4 `commits[]`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `hash` | string | 是 | 完整 Git 对象 ID，不进行缩写。当前常见 SHA-1 为 40 位，SHA-256 仓库可为 64 位。 |
| `committer.name` | string | 是 | Git committer 姓名。 |
| `committer.email` | string | 是 | Git committer 邮箱。 |
| `committedAt` | RFC 3339 string | 是 | Git committer 时间，保留提交记录中的时区偏移。 |
| `description` | string | 是 | Git 提交标题，即 subject 第一行，不包含提交正文。 |

提交已过滤 merge commit，并按从新到旧排列。仓库只有一条或两条非 merge 提交时，数组只包含实际存在的记录，不会补齐到三条。

## 5. 描述截断规则

`description` 按 Unicode 字符数量截断，不按 UTF-8 字节数截断：

- 不超过 100 个字符：原样输出。
- 超过 100 个字符：保留前 100 个字符，再追加三个 ASCII 点 `...`。
- `...` 不计入 100 个字符上限，因此截断后的最大长度是 103 个 Unicode 字符。

该规则可以避免在中文字符的 UTF-8 字节中间截断。

## 6. 特殊场景示例

### 6.1 全部命中构建缓存

```json
{
  "startedAt": "2026-07-17T14:40:00+08:00",
  "finishedAt": "2026-07-17T14:40:00.023+08:00",
  "durationMs": 23,
  "buildSkipped": true
}
```

### 6.2 仓库不足三条提交

```json
{
  "name": "new-module",
  "role": "dependency",
  "commits": [
    {
      "hash": "0123456789abcdef0123456789abcdef01234567",
      "committer": {
        "name": "Initial Developer",
        "email": "initial@example.com"
      },
      "committedAt": "2026-07-17T09:00:00+08:00",
      "description": "Initial commit"
    }
  ]
}
```

### 6.3 无法获取提交记录

```json
{
  "name": "example-common",
  "role": "dependency",
  "commits": []
}
```

`commits` 不会缺失，也不会写为 `null`。下游必须把空数组当作“本次未取得提交信息”，不能据此判断模块没有代码。

## 7. 生成、同步与失败语义

每个主模块的处理顺序如下：

1. checkout 主模块及依赖模块。
2. 执行现有 Maven 缓存判断和必要构建。
3. Maven 阶段成功后读取每个模块最近最多三条本地非 merge 提交。
4. 准备 WAR/JAR staging。
5. 在 staging 根目录写入元数据。
6. 通过现有 `rsync --delete` 同步应用和元数据。
7. 执行远程脚本或容器重启。

提交查询使用 Git 1.8.3.1 已验证支持的 `%ci` 和 `--no-merges`；`%ci` 时间由 `salt-agent` 转换为 RFC 3339 后再写入 JSON。

降级规则：

- 某个模块执行 `git log` 或解析输出失败：输出 warning，该模块写入 `"commits": []`，部署继续。
- JSON 序列化、临时文件创建、写入或重命名失败：输出 warning，跳过元数据写入，应用 rsync 和重启继续。
- 元数据写入失败时，staging 中没有该文件；现有 `rsync --delete` 会删除远端旧元数据，避免把旧文件误认为当前发布信息。
- Maven 构建失败：保持原流程，不执行本次 staging、rsync 或重启。
- rsync 成功但远程重启失败：新元数据可能已经在远端，文件存在不能作为服务启动成功的证明。

元数据读取来自 checkout 后的本地仓库，不会为收集提交信息再次访问远程 Git 服务。

## 8. 协议兼容策略

`schemaVersion` 使用 `<major>.<minor>`：

- 增加可选字段或不改变现有字段语义的兼容扩展：提升 minor。
- 删除字段、改变字段类型或语义、改变必填性：提升 major。
- 消费者支持 `1.x` 时，应忽略未知字段。
- 消费者遇到不支持的 major 时，应明确拒绝解析或降级为只展示原始文件，不能静默按旧语义解释。

## 9. 下游解析建议

推荐的解析步骤：

1. 按 UTF-8 读取完整文件并解析为 JSON 对象。
2. 检查 `schemaVersion` 主版本。
3. 将 `generatedAt`、构建时间和 `committedAt` 解析为带时区的时间类型。
4. 按 `role == "main"` 定位主模块，不依赖数组下标作为唯一判断依据。
5. 允许 `commits` 包含零至三项；展示前检查数组长度。
6. 把 `hash` 当作不透明字符串，不强制只接受 40 位。
7. 忽略未知字段，为未来兼容字段预留空间。
8. 需要确认实际运行状态时，同时结合远程重启结果或独立健康检查。

使用 Jackson 的 Java 插件可以采用类似配置：

```java
ObjectMapper mapper = new ObjectMapper()
    .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);

SaltAgentMetadata metadata = mapper.readValue(
    metadataPath.toFile(),
    SaltAgentMetadata.class
);
```

时间字段建议映射为 `java.time.OffsetDateTime`，`durationMs` 映射为 `long`，`commits` 映射为始终非空的 `List<CommitInfo>`。若使用强类型模型，仍应在业务层检查 `schemaVersion`，不能仅依赖 JSON 反序列化成功。

## 10. 安全与展示建议

- 文件包含提交者邮箱，下游展示时应遵守组织内的个人信息访问规则。
- 提交描述属于不可信文本；写入 HTML、Shell 或日志模板时必须进行对应上下文的转义。
- 不要根据提交描述执行命令或决定权限。
- 需要对比版本时优先使用完整 `hash`，不要仅依赖描述或提交时间。
