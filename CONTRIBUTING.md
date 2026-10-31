# 本仓库的更新与生产约束

这是 `Wei-Shaw/sub2api` 的公开 fork，包含独立维护的界面和账号首输出超时补丁。

| 分支 | 用途 | 更新方式 |
| --- | --- | --- |
| `main` / `upstream/main` | 上游源码镜像，不含本地定制 | 只从官方 upstream/main 快进同步；main 保留 fork 初始分支 |
| `custom/main` | 本地定制与上游的集成版本，默认开发分支 | feature/*、fix/*、sync/upstream-* 提交 PR |
| `production` | 已验收、允许部署的版本 | 仅接受本仓库 custom/main 的 PR，禁止直接推送、强推和删除 |

## 日常开发

从 `origin/custom/main` 建立 feature/* 或 fix/*；不得直接在上游镜像或生产分支开发。修改后创建到 custom/main 的 PR，通过 Release gate 后合并。生产热修复也先进入 custom/main，再 PR 到 production，避免生产补丁在下次更新时消失。

## 拉取上游并保留本地更新

在干净工作区运行：

```sh
python .github/maintenance/sync_upstream.py --publish
```

命令从最新 origin/custom/main 新建 sync/upstream-日期-SHA 分支，再正常 merge 官方 upstream/main，推送后建立到 custom/main 的 PR，等待 CI 检查及审阅。绝不使用 ours/theirs 全量覆盖。若有冲突会停止，保留现场；逐项解决并提交，执行前后端检查后再以 `git push -u origin HEAD` 和 `gh pr create --base custom/main` 提交 PR。不要在冲突现场重跑脚本。去掉 --publish 可只在本地准备合并，完成后同样使用 git push / gh pr create，不要重新运行 --publish。

上游同步 PR 必须检查：账号优先级、extra 合并、OpenAI CC/Responses 首输出超时、取消行为、超时后轮转、UI 编辑表单、国际化、定制界面、数据库迁移。先把上游与本地定制合并到 custom/main 并验证，再建立 custom/main → production 的 PR。禁止上游镜像直接进入生产。

## 检查和发布

Release gate 会验证分支来源、前端类型/关键测试/构建及首输出补丁 race 单元测试。分支保护同时要求现有 CI 的 test、frontend、golangci-lint、shell、release-helpers 检查通过，涵盖全量后端单元/集成与 lint；审阅生产 PR 时还要检查迁移和依赖变更。不能把构建通过当成业务回归通过。单人维护允许零个额外 reviewer，仍必须创建 PR 并通过必需检查。

生产 PR 合并后，从 production 的确切提交构建，不使用浮动 latest 作为部署凭据。可在 production 分支手动执行 Production image 工作流，镜像标签为 production-完整SHA，带 OCI revision。该工作流只构建镜像，不会连接服务器自动部署。部署必须：

1. 记录当前镜像、配置和回滚标签；保留数据库兼容性与备份。
2. 从 origin/production 的确切 SHA 构建或拉取该 SHA 镜像，核对镜像 revision。
3. 部署后检查健康接口、页面、账号编辑字段及主账号/备用账号路径。
4. 记录 SHA、镜像 digest、部署时间和验证结果；失败即回滚到记录的镜像。部署记录不得包含密钥或用户数据。

production 分支是已验收的发布来源；正在运行的版本以实际部署记录的 SHA 为准，PR 合并本身不代表已部署。部署基线标签 `production/20261101-fallback-baseline` 记录此次整理前的运行源码。旧独立仓库保存在 `SSOrangeCATY/sub2api-legacy-20261101`，孤立定制历史通过显式集成提交保留。

## 首输出超时

管理后台 → 账号管理 → 编辑 OpenAI / API Key 账号 → 首输出超时（秒）。0/留空关闭，1–300 秒启用，可用小数。字段保存在 extra.openai_first_output_timeout_seconds。只约束 CC/Responses 的首个有效输出；输出开始后不再施加此计时。账号级设置对该账号所在的所有分组生效。故障后由现有调度器选择下一账号；备用账号必须有独立可用的上游。

## 公开仓库安全

禁止提交管理 API Key、上游账号凭据、SSH 私钥、.env、数据库备份、用户数据、实际服务器配置及带凭据的日志。运行配置保存在服务器或 GitHub Secrets。提交前检查 staged diff，发现凭据先撤销并清理历史。
