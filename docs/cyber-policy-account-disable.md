# 上游 Cyber Policy 被动计数与账号停用

网关在消费现有上游响应时旁路检查错误，不发起检测请求、不调用外部检测服务，也不需要额外 API key。HTTP 错误、HTTP 200 中的失败响应、SSE 失败事件，以及 OpenAI WebSocket 的失败事件和握手错误均参与检测。上游原始状态码、响应内容、错误转换与重试逻辑仍由原有处理器决定。

## 配置

在现有 `config.yaml` 的 `gateway` 下设置：

```yaml
gateway:
  cyber_policy_disable_threshold: 10
```

也可使用环境变量 `GATEWAY_CYBER_POLICY_DISABLE_THRESHOLD=10`。修改后重启网关。默认值为 10（此次需求确认的阈值）；正整数表示停用阈值，0 表示只累计计数、不自动停用，负数导致配置校验失败。

## 识别与计数

- 检查错误对象的 `code` 或 `type` 是否精确为 `cyber_policy`（忽略大小写及首尾空白），支持 `error`、`response.error` 和兼容上游的裸错误对象。
- 检查错误消息中的明确提示 `This content was flagged for possible cybersecurity risk`（也支持 `This request ...`）。有效 JSON 只检查错误字段及兼容错误的 `message` / 字符串 `detail`，不扫描回显请求或成功输出。纯文本 HTTP 错误支持同一完整提示。
- 普通错误中出现 `cyber`、`cybersecurity`，或普通内容策略错误，不计数。
- 流事件先确认失败类型或 `status=failed`，再检查错误字段，因此也支持 `response.done` 携带的失败。缺失 SSE 响应头时按已读取正文的 SSE 前缀识别；HTTP 错误即使误带 SSE 响应头，也可识别实际返回的 JSON 或纯文本错误。
- 每次上游请求最多加一；同一次请求中的 `error` / `response.failed` 配对事件只计一次。实际重试或后续 WebSocket turn 是新的请求，独立计数。
- 计数为账号维度的累计次数，保存为 PostgreSQL `accounts.extra.cyber_policy_error_count`，不会因成功请求、进程重启或多个网关实例而重置。普通账号编辑不能回写过期计数；复制出的新账号不继承源账号计数。
- HTTP 观察缓冲每个 JSON 响应或 SSE 事件最多 1 MiB，超出的响应/事件跳过识别；SSE 后续事件仍可检测。不会为观察而额外读取或消耗上游响应。
- 只检查现有处理器实际读取的字节。被提前关闭或截断的 JSON 不退回全文提示搜索，以免把请求回显误认成错误；因此未读到或不完整的错误可能无法计数。

## 停用和恢复

计数与停用在同一数据库语句中更新。达到阈值时使用现有账号错误机制：`status=error`、`schedulable=false`、记录停用原因，并写入调度 outbox、同步账号缓存。后续调度将跳过该账号；已经发送的请求按现有流程结束。

可在现有账号管理界面查看 `extra` 中的计数，并使用原有账号恢复/启用操作。恢复操作不清零累计计数；若累计次数已达到阈值，下一次命中会再次停用。此功能没有新增计数重置接口。

账号编辑在行锁内保留数据库的最新计数。如果编辑读取账号后又触发自动停用，保存该旧编辑不会覆盖停用状态。重新读取停用后的账号，再使用原有恢复操作即可启用。

数据库写入失败会记服务端日志，不替换客户端错误或制造新的响应。此时该次计数/停用无法保证成功，应按现有数据库故障流程处理。

## 测试

配置测试覆盖默认 10 次、环境变量覆盖、0 仅计数和负值校验。服务与传输测试覆盖精确错误特征、普通错误/成功内容/截断回显不计数、按账号分别累计、每次请求去重、阈值停用、SSE 分块和响应头不匹配、WebSocket 多轮及失败终止事件，以及响应内容和读取/关闭错误保持原样。仓储单元测试覆盖计数写入接口、存储故障、旧编辑保护和读取最新状态后的恢复。

2026-11-10 本地验证：后端 `internal/config`、`internal/service`、`internal/repository`、`internal/handler/...` 和 `cmd/server` 整包测试通过；`unit` 标签的 `TestAccountFirstOutput` 首输出超时/自动轮转专项通过；前端 `EditAccountModal`、`BulkEditAccountModal` 共 133 项测试通过。`gofmt` 和 `git diff --check` 通过。本机未配置 CGO，`-race` 检查未能执行。

数据库集成用例位于 `backend/internal/repository/account_repo_cyber_policy_integration_test.go`，验证真实 PostgreSQL 持久化、阈值停用、账号隔离、并发累计和旧编辑保护。需在具备 Docker 的环境运行：

```sh
cd backend
go test -tags integration ./internal/repository -run Cyber -v -count=1
```

现有集成测试 harness 在本地无 Docker 时会跳过整包；跳过不代表数据库行为已验证。此次本地运行已完成集成测试编译，但因缺少 Docker 跳过，真实 PostgreSQL 集成尚未执行。
