# QQ 认证准入、付费充值与福利码门槛

本功能沿用现有 QQ 机器人认证、积分、福利发奖和管理员权限机制。管理员充值和余额兑换码是本次支持的付费来源；没有新增在线支付入口或调用第三方检测服务。

## 管理方式

- **QQ 机器人 → 群认证与连接**：新增“使用 API 前必须完成 QQ 认证”。默认关闭，开启后 API Key 所属用户必须完成当前配置群的 QQ 认证。标准 API、Google/Gemini API、简易模式和 Responses WebSocket 均执行检查。
- 此开关独立于“启用福利中心”：关闭福利后仍可生成认证码、通过机器人认证和使用已获准的 API；签到、领取和积分兑换继续要求福利中心启用。
- HTTP 每次请求直接读取当前持久化策略。WebSocket 读取首帧后及后续 `response.create` 前重新检查，管理员切换认证开关或群后，旧连接下一次请求也会重新验证。查询失败不会放行。
- 用户在个人资料查看认证要求、生成认证码，并在配置群内按原流程认证。登录和个人资料接口不受 API Key 准入限制。

## 付费与赠送口径

后台“用户 → 充值”和“兑换码 → 生成余额兑换码”新增来源选项，默认 **赠送**，可明确选择 **付费**。列表及充值历史展示来源。

`balance_source` 可持久化为 `paid`、`gift` 或 `unknown`。历史凭据均为 `unknown`，不猜测历史收入来源。后台生成和管理员充值默认 `gift`；内部未声明来源的旧入口仍为 `unknown`。普通用户不能通过兑换请求指定来源。

只有成功入账的正数付费余额累计为福利资格：

- 管理员明确标记为付费的 `add` 充值；`set` 和扣款不允许标为付费。
- 明确标记为付费的余额兑换码成功核销后的到账金额。生成未使用码不累计。
- 赠送、福利、签到、积分兑换、历史未分类金额均不累计。

累计付费是以 USD 余额计量的**历史成功入账额**，不是当前剩余余额。正常消费、管理员后续扣款或删除兑换码凭据不会减少累计额；本次没有实现退款冲销、在线支付本金识别或历史付费补录流程。已有在线支付等未声明来源的内部入口不会自动获得付费资格。

管理员 `add/subtract` 凭据使用请求增量，避免从大额浮点余额相减推导小额充值。数据库余额、凭据和累计资格使用现有 `numeric(20,8)` 精度。

## 福利兑换码

每个共享福利码可以设置 `min_paid_balance`，单位 USD，默认 `0`（不限制）。累计付费达到门槛即可领取，刚好等于门槛也可领取；仍遵守每账号限领一次、有效期和总数量限制。门槛最多八位小数。

资格在原发奖事务中、锁定网站用户之后检查，未达门槛不消耗名额、积分，也不生成奖励或修改余额。同一请求编号的重试先返回既有订单；提高门槛不会让已经成功的请求重放失败。

管理员在共享码列表点击“领取详情”，可按该码分页查看用户名、邮箱、网站用户 ID、到账金额、领取时间及订单。普通用户订单和机器人积分结果不包含其他用户的身份资料。

## 存储与迁移

迁移 `254_qq_access_paid_balance.sql` 新增认证开关、福利码门槛、充值来源及 `paid_balance_credits`。部署时沿用项目正常迁移流程。

付费凭据核销或创建为已使用时，由 PostgreSQL 触发器在**同一事务**中写入付费入账记录，按兑换码 ID 去重，并锁定对应用户以与福利领取串行化。管理员余额变更与凭据创建也在同一事务中；凭据写入失败时余额回滚，核销后的余额写入失败时凭据和付费累计同时回滚。

入账记录不外键依赖可删除的兑换码。已核销余额凭据的来源、金额、类型和归属不可修改为另一笔付费；原用户删除的 `ON DELETE SET NULL` 语义保留。

## 验证

```text
go test ./internal/config ./internal/service ./internal/repository ./internal/handler/... ./internal/server/middleware ./internal/server/routes ./cmd/server -count=1 -timeout=5m
go test -tags unit ./internal/service ./internal/server/middleware ./internal/handler/... ./internal/server/routes -run 'QQ|BalanceSource|UpdateUserBalance|AccountFirstOutput|FirstOutput.*Failover|Failover.*FirstOutput' -count=1 -timeout=5m
go test -tags welfareintegration ./internal/repository -run 'QQPaid|QQAPIGate|QQWelfare' -count=1 -timeout=5m
```

第三条命令需要 `WELFARE_TEST_DSN` 指向一次性本地 PostgreSQL，测试会执行迁移和数据写入。覆盖付费与赠送、历史来源排除、按账号累计、并发核销去重、临界门槛、重试、事务回滚、大余额小额充值、充值与领取交错、认证开关与群变更、删除凭据及用户兼容性。

前端验证包含类型检查、ESLint、生产构建、中英文语言键一致性及管理员设置、来源选择、福利领取和详情分页测试。另有真实 WebSocket 连接测试覆盖握手后的首帧及连接内后续轮次的策略变化。

当前变更仅在本地完成。生产合并、迁移和部署需待用户决定。
