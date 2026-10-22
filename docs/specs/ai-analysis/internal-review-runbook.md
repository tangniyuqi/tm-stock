# AI 内部操作手册（L0 审核与隔离 Agent 执行）

适用：内部授权资料登记、快照导入/采集、L0 抽取与人工核对。默认不调用模型；只有单独配置内部批准门禁后，`extract` 才可能调用付费 Provider。本工具不接积分收费、不开放公众 AI 报告；GVA 界面不参与审核写入。

外部 Agent 使用独立的 `cmd/ai-agent`，不复用 `extract` 的无工具契约；要求与限制见末节。未配置、未批准时不运行真实模型。

## 启动前

1. 在隔离测试库完成迁移验收，再经人类确认安排正式变更。共表须先完成 `20261002_converge_addon_quant_theme_stock.sql`，然后依次执行 `20261010_create_ai_source_snapshots.sql`、`20261010_create_tm_ai_extraction_candidates.sql`、`20261010_create_tm_ai_l0_extraction_runs.sql`、`20261010_create_tm_ai_review_inspections.sql`、`20261010_create_tm_ai_source_audits.sql`。服务不会自动建表；缺来源审计表时写入失败并回滚，不会降级无审计写入。
2. 五份 `20261010` 开发迁移及来源/审核/抽检事务已在全新 MySQL 8.0.46 / 9.7.0 临时实例各连续通过 10 次竞态集成测试；**不能据此直接用于生产**，尚未在生产副本做升级演练。早期开发版本的候选状态列已改为独立审计事件，并补了审核抽样索引和完整窗口编号；若曾手工运行早期版本，`CREATE TABLE IF NOT EXISTS` 不会升级旧表或补索引，不要重复运行后误认为结构已升级，更不能删除真实证据来重建。
3. 由凭据管理系统注入环境变量：`TM_DB_DSN`（含 `parseTime=true`）、`TM_AI_ADMIN_REVIEWERS`。后者是 JSON 数组，每项为 `actor_id` 与独立随机 `token`（32–256 个无空白 ASCII 字符）。审核身份使用实际获准审核人员编号，**不得多人共享同一凭据**；轮换后重启服务。不得把实际值写进仓库、命令行参数或日志。
4. `TM_AI_ADMIN_ADDR` 默认 `127.0.0.1:8081`，只允许回环 IP。远程操作使用经授权的 SSH 隧道，不改为公网/内网监听；不通过浏览器或转发请求头放行。

在 `server/` 启动独立进程：

```bash
go run ./cmd/ai-admin
```

内部路由：

- `GET /internal/v1/ai-review-candidates?after_id=0&limit=100`，单页最大 200 条，按编号递增。
- `POST /internal/v1/ai-review-decisions`，请求仅含 `candidate_id / evidence_digest / decision / reason / request_id`。审核人从凭据获取，传 `actor_id` 会被拒绝。
- `POST /internal/v1/ai-sources`、`GET /internal/v1/ai-sources/{id}`、`POST /internal/v1/ai-sources/{id}/state`，登记默认停用，启用时重新核对授权。
- `GET /internal/v1/ai-sources/{id}/audits?after_id=0&limit=100`，来源操作历史，单页最多 200 条；内部获准人员可读，禁止公开代理此接口。
- `POST /internal/v1/ai-snapshot-imports`、`POST /internal/v1/ai-snapshot-fetches`、`GET /internal/v1/ai-snapshots/{id}`，正文不可变，采集只接受精确 HTTPS 来源域名。
- `POST /internal/v1/ai-extractions`、`GET /internal/v1/ai-extractions/{request_id}`，执行/查询内部抽取。历史查询按请求所属审核身份隔离，关闭模型仍可查询。
- `POST /internal/v1/ai-inspection-batches`、`GET /internal/v1/ai-inspection-batches/{request_id}`、`GET /internal/v1/ai-inspection-items/{id}`、`POST /internal/v1/ai-inspection-decisions`、`GET /internal/v1/ai-inspection-issues`，内部随机抽检、独立复核与问题历史。

`cmd/api` 不注册上述路由。内部接口要求真实 TCP 对端为回环 IP、独立 Bearer 凭据；拒绝浏览器 Origin，不信任 `X-Forwarded-For`，响应禁止缓存。

## 来源、快照与抽取

以下管理命令从标准输入读取单个 UTF-8 JSON 对象，输出 JSON；都使用自己的 `TM_AI_ADMIN_TOKEN`。示例编号须替换为实际返回编号，示例授权引用不构成真实授权。

```bash
go run ./cmd/ai-review source-register < source-registration.json
go run ./cmd/ai-review source-state < source-state.json
go run ./cmd/ai-review source-show < source-id.json
go run ./cmd/ai-review source-audits < source-audits-page.json
go run ./cmd/ai-review snapshot-import < snapshot-import.json
go run ./cmd/ai-review snapshot-fetch < snapshot-fetch.json
go run ./cmd/ai-review snapshot-show < snapshot-id.json
go run ./cmd/ai-review extract < extraction-request.json
go run ./cmd/ai-review extract-show < extraction-lookup.json
```

输入字段：

| 命令 | JSON 字段 |
|---|---|
| `source-register` | `domain`（精确域名，不是 URL）、`category`、`license_tag`、`license_ref`、`license_expires_at`（Unix 毫秒或 `null`）、`min_interval_ms`（0–86400000；至少按 1 秒采集） |
| `source-state` | `source_id`、`enabled`（必须显式布尔值） |
| `source-show` / `snapshot-show` | `id` |
| `source-audits` | `source_id`、`after_id`（非负）、`limit`（1–200） |
| `snapshot-import` | `source_id`、`url`、`media_type`、`body`（**原始文件字节的 Base64，不是直接正文字符串**） |
| `snapshot-fetch` | `source_id`、`url` |
| `extract` | `snapshot_id`、`request_id`（16–64 位字母、数字、`-`、`_`；先保存唯一编号再提交） |
| `extract-show` | `request_id`（与原请求一致） |

`category` 只接受 `announcement / annual_report / prospectus / official_catalog / interactive_qa`。导入和采集只处理 HTML 或纯文本，不直接解析 PDF；原始正文上限 2 MiB，导入 JSON 上限 3 MiB。快照查询返回规范化正文、哈希、采集毫秒时间及 `sentence_count`；不可变句子编号/偏移仍保存在库内。过期来源可以查看和停用，但不能据此继续采集或抽取。

来源登记、启停、导入和采集成功时，数据与不可变 `tm_ai_source_audits` 同事务提交。身份只由凭据取得，各写请求传 `actor_id` 会被拒绝。历史包含操作人、登记/当前授权副本、启停前后副本及快照引用，不返回正文或凭据；数据库时间和授权到期时间经 API 均为 Unix 毫秒。重复内容复用快照，但每次成功操作有自己的审计；返回 `next_after_id` 用于下一页，同来源写操作串行持锁。

操作历史**只代表已成功提交的写入**，不包含拒绝、采集失败或崩溃前未提交的尝试，也不是请求级幂等任务日志。网络结果未知时先查看来源状态和操作历史，不能把“调用失败”当作“数据一定没变”；重复启停会再追加事件。来源停用不会自动删除或暂停历史人审归属，发现相关依据有问题时须使用复核/既有治理流程暂停展示并保留证据。

### 模型调用门禁

`TM_AI_L0_ENABLED` 未设置或为 `false` 时，不加载模型配置、不创建 Provider，`extract` 返回未启用；资料管理和审核可独立使用。取得内部评估用途的来源授权、真实批准文件及模型账号后，方可由运维显式注入：

- `TM_AI_L0_ENABLED=true`、`TM_AI_ENABLED=true`；`TM_AI_PROVIDER / TM_AI_MODEL / TM_AI_BASE_URL / TM_AI_API_KEY / TM_AI_TIMEOUT_MS`。
- `TM_AI_L0_PROVIDER_DOMAIN`：批准的精确模型域名；端点必须 HTTPS、非 IP、无账户/查询/片段，单次时限 1–30000 毫秒。
- `TM_AI_L0_APPROVAL_REF`：内部调用的书面批准引用；`TM_AI_MODEL_REGISTRY`：受控注册表文件路径；`TM_AI_L0_MODEL_ID`：对应注册项编号。厂商和模型名必须精确匹配，备案/上线编号非空，声明 `json_schema` 能力。**这些是配置校验，不核验批准、备案或能力的真实性，不能代替法务与真实接口验收。**
- `TM_AI_L0_MAX_CALLS`：明确设置 1–1000 的单进程调用尝试上限；`TM_AI_L0_MAX_PROMPT_BYTES`：4096–262144 字节的系统提示加正文上限。输入超限拒绝，不静默截断来源。

预算是保守调用计数，可能包含未实际发出的尝试，**不是费用上限或积分账本**；重启会重置，不能通过反复重启规避批准配额。模型输出上限 4096 token，每次抽取最多调用两次，仅允许畸形结构修复，不做网络重试或模型回退；同一进程只执行一份抽取。实际费用由批准账号承担；本轮开发只使用测试传输，未调用真实模型、未产生真实成本评测结果。

### 幂等、审计与不确定结果

- 模型调用前先提交 `tm_ai_l0_extraction_runs`，固化请求、身份、快照、模型与批准引用；每次调用前后各追加不可变 `tm_ai_l0_call_events`，持久化失败时不发出调用或不接受响应。原始审计先脱敏，内部 API 不返回提示词、模型原始响应或密钥；数据库仍须做权限和保留管理。
- 候选与 `succeeded` 终态同事务提交，成功前复核授权；相同证据可以复用原候选，候选保留首次模型追溯，本次运行另记实际模型和用量。候选仍须人工审核，不因抽取成功自动公开。
- 网络超时或进程崩溃后，先用 `extract-show` 查询**原请求编号**。再次提交原编号不重跑已登记任务；更换身份或快照冲突。`running`、409 或未配对的 `start` 可能表示结果不确定，必须由获准人员核对数据库与供应商记录；目前没有自动恢复或人工对账写入接口。**不要换编号重试、清理审计表或直接修改运行终态。**
- `failed` 也是原请求的终态，原编号重放不会再次调用；只有核对失败与成本、重新获准后，才可创建新的独立业务请求。token 的 `usage_estimated=true` 表示估计，不用于收费结算；失败调用的用量留在受控审计中，不能把接口终态里的零用量解释为没有供应商费用。

## 导出、人工审核与回传

以下命令在 `server/` 的 Bash/Git Bash 中运行，以 UTF-8 字节重定向 CSV。使用 PowerShell 时须使用能保持原始 UTF-8 字节的原生程序重定向，避免旧版默认 UTF-16 编码。

1. 审核人从凭据管理系统注入自己的 `TM_AI_ADMIN_TOKEN`；`TM_AI_ADMIN_URL` 默认为 `http://127.0.0.1:8081`，客户端也只接受回环 IP，拒绝代理与重定向。
2. 导出：

   ```bash
   mkdir -p ai-review-output
   go run ./cmd/ai-review export > ai-review-output/review.csv
   ```

   每批最多 2000 条、16 MiB。后续批次按标准错误中的游标设置 `TM_AI_REVIEW_AFTER_ID` 再导出；要重新检查尚未处理的旧候选时清除该变量。CSV 含完整证据与模型追溯字段，只保存在受控位置，不提交 Git；`server/ai-review-output/` 已加入忽略目录。先前审核批次应另存为不同文件名，避免覆盖未回传的决定。
3. 对照来源链接、原文与公司/题材身份逐条核对。**只编辑 `decision` 和 `reason`**：`approve` 的原因必须留空；`reject` 必须填写不超过 250 字的原因；决定留空表示暂不处理。不要改编号、原文、摘要、句子、时点、模型字段或 `request_id`。表格软件须保持 UTF-8 CSV，不能把编号转科学计数法。公式开头的文本会加可逆保护前缀，不要手动删除。
4. 回传：

   ```bash
   go run ./cmd/ai-review apply < ai-review-output/review.csv
   ```

   工具先校验整份文件，检测到依据被改就不发送任何决定。每条决定在服务端独立事务执行，**整批不是一个事务**；中途失败，已确认的记录保留，可用**原 CSV**重试。同请求、同身份、同载荷幂等返回；换身份、改决定或重新审核终态均冲突。不要为重试重新生成请求编号。

## 通过的约束与审计

- 通过前复核当前来源授权、不可变快照逐字内容、实体/题材共现、红线及锁定的主数据身份。来源失效不得通过，但仍允许驳回。
- 节选候选可以在逐字/分段/共现/红线复核通过后审核，`excerpt_trimmed` 随归属保存；C 端显示“原文摘录（节选）”和上下文提示，缺失或类型错误的标记不展示。必须先完成下述增量迁移及所有展示端更新，再恢复审核。“原文转载”特殊人工放行仍未接通，不得靠勾选节选或直接改数据库绕过红线。
- 通过会写带完整四项依据、`audit_status=2` 的归属；已有存活归属返回冲突，不覆盖。归属与不可变审计事件、可见股票数刷新同事务提交；失败全部回滚。**这些人工通过的客观归属可能被现有 C 端题材查询读取，不代表公开 AI 报告开关已启用。**
- 候选完全不可修改/删除。候选没有审核事件即待审，有唯一 `tm_ai_review_audits` 事件即终态；审计记录含候选、证据摘要、身份、决定、原因、归属编号、数据库时点，禁止修改或删除。需要纠错时停止继续通过，按既有归属治理流程处理，不抹掉历史。

## 抽样复核与问题追踪

抽检不调用模型、不消耗积分，也不替代首次全量人工审核。须由另一名实际获准审核人员使用自己的凭据创建并完成，**原审核人不得确认自己的记录**。若来源过期导致启用 L0 的进程启动失败，先经获准运维关闭 `TM_AI_L0_ENABLED`，仍可进入资料/问题治理入口；不要篡改授权引用绕过启动门禁。

```bash
go run ./cmd/ai-review inspection-create < inspection-batch-request.json
go run ./cmd/ai-review inspection-show < inspection-batch-lookup.json
go run ./cmd/ai-review inspection-item < inspection-item-id.json
go run ./cmd/ai-review inspection-decide < inspection-decision.json
go run ./cmd/ai-review inspection-issues < inspection-issues-page.json
```

输入对象：

| 命令 | 字段 |
|---|---|
| `inspection-create` | `request_id`、`after_review_id`（非负）、`window_size`（1–2000）、`sample_size`（1–200 且不大于窗口大小） |
| `inspection-show` | 原批次的 `request_id` |
| `inspection-item` | `id`（批次返回的**条目**编号，不是候选或归属编号） |
| `inspection-decide` | `item_id`、条目详情顶层 `evidence_digest`、`decision`（`confirm` 或 `issue`）、`reason`、独立 `request_id` |
| `inspection-issues` | `after_id`（非负）、`limit`（1–200） |

1. 先保存批次请求编号再提交。服务端在指定游标后读取有限的已通过审核窗口，排除自己的原审核，密码学随机、不放回抽样；实际条数为 `min(sample_size, eligible_count)`，没有可选记录时仍返回空批次。批次保留 `population_review_ids`（完整实际窗口）、内容摘要、计数、末游标和样本分派；不能由客户端指定某几个候选冒充随机样本。相同请求/身份/参数返回原批次，不重抽。
2. 批次只返回编号和口径，逐条用 `inspection-item` 获取原审核事件、完整候选依据、复核用摘要及已有 `result`。**复核摘要不是候选内的首次审核摘要**，不可混用。对照 `snapshot-show` 的完整上下文与来源、主体、否定语义、题材身份核对；仅看摘录不能代替这一步。批次与详情仅允许创建者读取，问题历史供获准内部人员共享追踪。
3. `confirm` 原因必须留空；`issue` 必须填写不超过 250 字、无控制字符的事实问题原因。确认前再次校验授权、逐字引用、主数据和当前归属；已暂停或被其他流程改动的归属不能确认。发现问题时允许授权已失效，但只在当前归属仍精确匹配原证据时暂停其 `status` 展示、追加不可变事件、刷新题材可见股票数，三者同事务；故障全部回滚。原候选、原通过审计及证据保留不变。
4. 网络结果不确定时先查询原条目 `result`，重试原决定编号及载荷。不同身份、不同载荷或终态重新决定冲突；不要换请求编号重审。不同批次允许再次抽到同一原记录，不能借再次 `confirm` 自动恢复已暂停展示的归属。
5. 问题历史按事件编号递增分页，不代表“尚未解决”数量；本轮没有新增自动修复、恢复展示或问题关闭接口。必须按既有归属治理流程核对处理，保留复核历史，不能直接清审计表或修改原审核决定。处理后是否恢复须重新核对授权、依据与合规口径。

后续窗口可使用批次返回的 `next_review_id`，但自增编号不等于提交顺序，不能把一次游标遍历宣称成完整普查；定期从零重新抽查，批次原窗口编号保留首次实际观察范围。**不能用抽样中零问题宣称总体无错误，也不能把人工抽检结论当作真实模型质量评测已完成。** 自动周期调度与度量看板仍未接入。

## 验证

### 节选增量迁移与部署顺序

1. 停止内部审核入口，核对共表收敛和五份初始 AI 迁移已执行；在生产副本演练 `server/migrations/20261011_add_theme_stock_excerpt_trimmed.sql`。此脚本独立重复安全，但不替未知旧结构做破坏性修复；遇到 `tm_migration_abort__excerpt_*` 停止排查，禁止 `--force`。
2. 经人类上线确认后先迁移，再部署 API、GVA 和 H5；刷新/失效旧静态缓存并验证“节选”和“非节选”显示后才恢复内部审核。历史默认 0 不声称完整转载。缺列会使新读写端报错，不得降级丢标记写入。
3. GVA 省略 `excerpt_trimmed` 保留原值；显式变更会打回草稿，不能和同次请求自审通过。复核会拒绝现存标记与原候选不一致的归属。
4. 若已有可见节选，不能仅回滚前端到无标签版。先按既有治理流程暂停相关归属，保留依据/审计，不删列或改历史候选来回滚。

### 自动验证范围

```bash
go test -race ./...
go vet ./...
go test -tags=integration ./internal/repository -run TestIntegrationAISourceRepositoryAndImmutableSnapshots -count=1 -v
```

根目录 `scripts/dev/verify-ai-source-native.ps1 -MySqlBaseDir '<MySQL程序目录>' -RunCount 3` 现在会在同一可丢弃实例中先跑 `server/internal/repository` 全集成套件，再跑 GVA `service/quant` 的全部 `MySQL` 用例（均带 `-race`）。MySQL 8.0.46 / 9.7.0 各连续通过 3 次，包含增量迁移重复执行、既有原文保留、非法 0/1 值、未知列/同名宽松 CHECK 中止、节选候选入库、两条 C 端查询扫描、复核标记比对及 GVA 省略/显式修改语义。成功实例已关闭并清理，不连接本地已有业务实例。

前端可重复验证（PowerShell，在根目录；依赖放临时目录，不改变业务依赖）：

```powershell
$qaRuntime = Join-Path ([System.IO.Path]::GetTempPath()) ('tm-stock-ui-qa-' + [guid]::NewGuid().ToString('N'))
npm.cmd install --prefix $qaRuntime --no-save --package-lock=false --ignore-scripts vue@3.5.31 @vue/compiler-sfc@3.5.31 typescript@5.9.3
node scripts/dev/verify-evidence-ui.cjs $qaRuntime
```

该脚本实际提取页面验证器运行正反例，编译并 SSR 渲染依据浮层，核对节选/非节选提示、原文 HTML 转义，并编译两处 GVA 表单 SFC。已通过；**HBuilderX 整体构建、真实浏览器布局/点击交互尚未验收**，不能以 SSR 代替。

集成测试会清理测试表，**仅允许设置隔离、可丢弃测试库的 `TM_TEST_DSN`**（含 `multiStatements=true`）；绝不使用业务库 DSN。未设置时测试跳过，不代表迁移通过。集成用例覆盖迁移重复执行、候选/审计/抽检不可变、通过入库、重复请求、已有归属冲突、审计故障回滚、并发决定、来源停用后的拒绝通过与允许驳回，以及问题暂停/计数/复核事件的原子性和原审计保留；新增来源登记/启停/快照/操作审计的原子提交、故障回滚、历史分页、非法快照引用及并发状态变更链。

Windows 已有 MySQL 程序时可从仓库根执行原生验证脚本，无需启用现有数据库服务：

```powershell
./scripts/dev/verify-ai-source-native.ps1 -MySqlBaseDir '<MySQL 程序目录>' -BashPath '<Git Bash 的 bash.exe 绝对路径>' -RunCount 3
```

脚本使用 `--no-defaults / --no-monitor`，忽略本机配置及登录凭据文件，创建唯一临时数据目录、动态回环端口和随机测试口令；不连接已有实例，测试凭据不放入命令行。成功后停止并删除本次临时库，失败时停止实例、保留日志和数据路径供排查，不自动清理证据。初版五份 L0 迁移在 MySQL 8.0.46 与 9.7.0 各连续 10 次通过；后续账本用例与客户端迁移验证使用当前脚本重新验收，不能沿用旧的次数。`-BashPath` 在 bash 已加入 PATH 时可省略。编译按 `-p 1` 控制并行度；内存不足时进一步在当前终端设置 `GOMAXPROCS=2`，不要并行启动多组全量编译。仓库总门禁另在根目录执行 `bash scripts/harness-checks.sh`。

整包points触发原20分钟Go看门狗时，原运行按退出1保留，不能把已通过前缀称作整轮成功。原生入口可显式分为两个互补范围：`-NonReportsOnly`只在points命令增加`-skip '^TestIntegrationAIReport'`，仓储、其余points（含纯回环源/进程恢复辅助用例）、三个API启动、Agent、GVA及客户端迁移仍按默认入口运行；`-ReportsOnly`不传自定义`-ReportPattern`，执行全部报告根组及原创建/报告API启动检查。两个模式互斥，默认整包入口、每轮看门狗、单次SQL/业务/租约时限、CI命令与计数下限都不变。

对8/9版本分别以相同源码和RunCount串行运行两个分区，各自须退出0、无断言失败/跳过/竞态且迁移通过，才能讨论覆盖合并。先用`go test -race -tags=integration <包> -list . -json`独立列举应执行根组，再逐包逐根核对分区并集，不只比较总PASS。报告API启动在两分区重复执行，须按包和测试路径识别重复；不能靠重复计数满足下限，也不能将相同源码之前的另一轮日志拼入而冒充同一批验收。CI对应分组表达式和最低执行数须另行完整核对；分区成功不改写默认整包超时或远端CI状态。

2026-10-20，1298文件冻结`84C1882F6DE9C66B2B75010AA4FB703F1D90083F2093C874E5160C7B7C1C35F5`下，8版及9版非报告/完整报告分区均分别431/614条PASS，各自退出0、无失败/跳过/竞态/看门狗、迁移各44/44并清理成功实例。两版分别按包与完整测试路径合并后，168个预期根组逐包相等、无缺失/额外根组；剔除重复API的18条PASS后，各主套件1023条唯一PASS≥931，105个CI分组精确计数全部匹配；GVA四条单列，不用于补主套件下限。报告points包8版1152.675秒、9版1190.811秒，不延长原20分钟上限。原默认整包813条后看门狗失败仍保留，不能改称默认入口已绿或远端CI已通过。

同冻结完整harness含覆盖率退出0（`tm-stock-ai-final-harness-23fb7cb4b0c446cdad585851d3fdde24.log`）：合规38、探测8、扫描器42、全仓扫描、Go编译/vet/竞态、13包覆盖率下限、GVA关键包、分层与pages.json通过，race未降级。总语句覆盖70.5%不代表模型质量。源码交互测试与实际H5报告/取消另有通过证据，具体日志见tasks.md；未宣称物理手机验收。上述结果只验收真实调用关闭的本地开发闭环，生产控制器、模型/来源授权、远端硬预算、质量/发布批准仍须失败关闭，不构成上线许可。未提交/推送/部署，不公开原始扫描/凭据日志。

## 专用账本结构预检（开发底座，尚未启用收费）

账本由 `server/migrations/20261011_create_points_ledger.sql` 单独定义，不需要导入 GVA 旧积分。部署账号在隔离库先用 mysql 客户端执行该文件（保留文件中的 `DELIMITER` 指令），再运行 `verify-migrations.sh`；脚本包含可丢弃库的删除操作，原生模式必须只指向测试实例。迁移及重跑均须停止账务写入，DDL 不是原子事务；重复执行不会升级未知旧表，触发器重建失败或自检失败时不得恢复写入，更不能删除真实账本重建。业务/自检账号只授八张 `tm_points_*` 表的 SELECT、INSERT、UPDATE、DELETE，不授 CREATE、ALTER、TRIGGER 或其他业务表权限。拒绝修改/删除由触发器执行；不要为“看见触发器元数据”授予修改触发器的权限。

凭据管理系统注入 `TM_DB_DSN`（含 `parseTime=true`）后，在 `server/` 执行：

```bash
go run ./cmd/points-check
```

失败即退出非零，不应启用任何账务写入口。命令不运行迁移、不充值、不扣费、不核对历史余额；正例和非法写入探针都在单事务中回滚，可能消耗自增编号。15 秒自检超时与外层 20 秒命令超时均失败关闭。需严格 SQL 模式、启用外键检查和 MySQL ≥8.0.16；未知旧表或被削弱的金额列、外键、唯一键、CHECK、不可变保护都拒绝通过。所有时间字段显式写 UTC，不能把 `UTC_TIMESTAMP` 的到期时间与本地时区默认 `CURRENT_TIMESTAMP` 的创建时间混用。恢复型 worker 已调用包含账本的任务联合自检，其他未来账务/执行路径仍须接线；此命令通过**不代表**任务收费、对账或全量崩溃恢复验收通过。

2026-10-10 当前原生脚本已在 MySQL 8.0.46 与 9.7.0 各连续 3 次通过全部仓储和 GVA 共表竞态测试；两版客户端迁移验证均为 26 项通过、0 项失败，受限账号无 DDL 权限仍能完成真实拒写探针。实例均已关闭，成功实例的临时库已移除；失败尝试的日志/数据仍保留在工具输出的唯一临时目录。以上是新建隔离库验证，不是生产副本升级演练。

## 账务服务核心验证（内部调用，尚未启用收费）

`PointsService` 提供入账、冻结、全额结算/释放、整笔冲正、奖励优先抵扣与单用户对账；上层可通过同一 SQL 事务适配器调用各 `InTx` 方法，成功收据在父事务提交前不得对外返回。提交失败时返回错误与空收据，须以原幂等键核查，不能换键重试。入账数量、来源、优先级与生效时间仅由可信内部业务提供，不接受客户端自行指定。购买批次拒绝有效期且不参与欠款抵扣；赠送/贡献/签到在入账时优先抵扣，生效时间只控制剩余奖励能否消费。收据 `Amount` 记录原额，不代表净到账额。

`(kind,biz_type,biz_id)` 必须全局唯一，例如签到编号包含用户和日期；迁移重跑不会给旧开发表补索引，缺少 `uk_tm_points_txn_business` 的库会被结构自检拒绝。须另行评审增量升级，禁止清空真实流水以通过自检。账面可用余额仍含未生效/未清扫的过期批次，新冻结逐批过滤，不得直接把账面数作为 C 端即时可用余额。

在仅用于测试的 `TM_TEST_DSN` 下，执行 `go test -p 1 -race -tags=integration ./integration/points -count=3 -v`。测试账号需要新建/删除库及建表/触发器权限，**不是线上业务账号权限模板**；每个用例新建随机 `tm_points_it_*` 库，只删除自己成功创建的库。原生验证脚本按仓储、跨层账本、GVA 顺序运行，Docker 入口以 `-p 1` 串行运行，避免共享夹具相互清表。

冲正只接受原入账流水编号，一笔原单仅一次整笔冲正，拒绝任意金额和跨用户原单；其他购买批次不被回收。原批次存在冻结时必须先终结关联任务，不能直接扣走冻结额度。回收剩余批次、已消费部分记欠款；已经写过期分录的部分不重复形成欠款。抵扣按欠款 id 顺序分摊，购买本金保留，奖励正分录和抵扣负分录各有流水。撤销已抵扣奖励会重新记欠款，不改旧抵扣历史；这不是支付退款接口，也不支持部分原单冲正。

真库套件包含 RC/RR 两种隔离级别、200 并发冻结、40 同键入账、40 终态竞争、40 并发冲正/奖励、多个批次与欠款、父事务及真实分录/抵扣分摊故障回滚、整数上限、数据库会话时区/驱动 loc 变化和人为破坏。单测在各数据访问及提交边界注入失败，并单独验证事件语义与欠款分摊破坏。任务创建/失败/取消/到期已接入原子事务，成功报告结算、批次过期清扫与杀进程恢复仍未完成；公开 AI、支付和真实扣费未启用。CI 矩阵已配置两版、账本十三组和任务七组必跑守卫，但不把本地结果当作远端 CI 通过。

共享事务的读取采用当前读，不能混入父事务先前的 RR 快照。回归覆盖建立快照后另一连接提交的入账重放、冻结分摊、过期冲正和对账；修正前已实测出漏读与多记欠款的反例。为控制对账持锁范围，迁移新增 `idx_tm_points_hold_alloc_user` / `idx_tm_points_debt_alloc_user`，并校验七个用户前缀索引可见且列序正确。旧开发表仅重跑 CREATE IF NOT EXISTS 不会补索引，须先评审增量升级；缺失、隐藏或列序不符时自检拒绝启动。对账仍需读取单用户历史并持有账户锁，RR 有范围/间隙锁成本；大历史量延迟、线上容量与线上周期调度尚未验收。

2026-10-10 最终代码在两版原生实例均连续 3 次通过仓储、跨层账本和 GVA 套件，包含冲正/抵扣、父事务旧快照与用户锁范围回归，客户端迁移断言均通过 26 项；成功实例均已关闭并清理。旧快照负例验证失败时的临时实例已停止，日志/数据按原生脚本输出路径保留，不自动销毁排查证据。`go test -p 1 ./... -race -count=1`、`go vet -p 1 ./...`、GVA 关键包与架构检查通过。总门禁仍在既有密钥扫描及扫描器自测处失败，未扩大基线，不能据此发布。定向单测下，冲正 InTx 语句覆盖率 98.2%、抵扣计划 100%、抵扣执行 93.8%、纯对账 96.1%、事件对账 100%；这些是核心函数覆盖率，不是整个 service 包覆盖率，也不替代真库测试。

## 全账户对账与告警出口（内部运维）

从 `server/` 运行，连接仅取 `TM_DB_DSN`，命令不自动迁移、不修改历史余额。需先在获准的测试/演练环境核对实例和权限，不要把测试 DSN 直接替换成生产连接执行。启动会运行上述回滚约束探针，需八表的 SELECT/INSERT/UPDATE/DELETE；不是仅 SELECT 的审计账号，也不需要 DDL 权限。

```bash
# 单次：结构自检 → 固定用户上界 → 主键游标分页 → 单用户真实对账
go run ./cmd/points-audit -page-size 100 -round-timeout 10m

# 显式周期模式：本轮结束后等待 5 分钟再跑，不自动安装为服务
go run ./cmd/points-audit -page-size 100 -round-timeout 10m -interval 5m
```

参数边界：每页 1–1000 人，整轮期限 1 秒–24 小时；周期 0 表示单次，启用时 1 秒–24 小时。每用户事务上下文期限 30 秒，受整轮更早的期限约束；标准输出下游阻塞与纯内存计算不支持强制中断，不能把上下文期限当成操作系统级硬超时。部署需外部进程监督器、终止宽限期及完整轮次新鲜度告警；不能以启动一个周期进程代替这些保护。每轮从头重扫，每个进程串行，不补跑积压时点；多进程并行会重复告警并争用同用户锁，部署方应只配置一个实例。本轮用户上界固定，不追赶无限新增账户；游标之前才新建的低编号账户由下一轮覆盖。结果不是跨用户同一时点快照。

标准输出为 JSON 行，`at` 为毫秒时间戳：异常用户输出 `points_audit_issue`（用户、问题代码及记录编号，不含金额/正文/凭据）；每轮输出 `points_audit_round`，包含 `upper_user_id`、`last_user_id`、`checked`、`unhealthy`、`issues` 和 `complete`。访问/超时失败输出部分统计及 `failed=true, complete=false`，不泄漏驱动细节，不能当成“无异常”；告警输出本身失败则退出非零。结构自检失败仅输出 `points_audit_failure`，不得开始扫描。

编译后的程序退出码：0=单次完整且健康；2=单次完整但检出账本异常；1=配置、自检、扫描或输出失败；130=中断。`go run` 可能把程序的非零退出码统一映射为工具退出码 1，自动化要使用构建好的可执行文件。周期模式在扫描失败/异常后继续等待重扫，故不能只监控进程退出码：运维必须采集异常事件、失败轮次、完整轮次的新鲜度及覆盖范围；该采集平台、保留期、访问权限和通知接收人尚未部署，用户编号日志也须按内部敏感信息保护。反复超时时增加期限或排查阻塞账户与容量，不可跳过账户/扩大基线来假报健康。

异常处理：保留原日志，暂停相关写入口，由获准人员核对流水/分录/任务后走冲正或受控修复；此命令不自动平账、不替代任务锁价之和的审计。冻结批次到期政策仍待产品确认，过期清理没有接自动运行。

`TestIntegrationPointsAuditSweep` 覆盖 RC/RR 真库分页、固定上界后新增账户、下一轮覆盖、人为破坏检出、告警失败、账户锁等待超时及下一轮恢复，并构建真实命令验证健康/异常/结构自检失败的 JSON 与退出码；这是隔离测试，不是线上周期调度验收。

2026-10-11：最终代码在 MySQL 8.0.46 / 9.7.0 各连续 3 次通过仓储、跨层账本十三组及 GVA 真库套件，新增全账户对账与真实命令正反例均通过；两版 mysql 客户端迁移断言各 26 项通过，成功实例已停止并清理。单测连续 20 次竞态通过，扫描核心定向覆盖率 97.7%、周期循环 92.9%、命令包 77.6%；不是整个 service 包覆盖率。全量 Go/race/vet、GVA 关键包、架构、格式和工作流 YAML 验证通过；最新总门禁仍在旧工作树疑似密钥命中及扫描器自测处失败，未扩大基线，远端 CI 未运行。以上不代表真实任务、线上告警平台、生产容量或阶段 3 已验收。

## 持久化任务核（内部服务及有限恢复命令）

迁移顺序为积分账本→`server/migrations/20261011_create_tm_ai_tasks.sql`→后文预算/阶段迁移，部署及真实执行另需获准。任务基础迁移新增 `tm_ai_tasks` / `tm_ai_task_events`，包含三个强制 CHECK、四个不可变触发器、用户隔离外键、任务/冻结单/幂等唯一键，以及历史/候选/期限索引；后续阶段迁移再增加两个任务触发器。执行及重跑时停止任务写入，DDL 不是原子事务；触发器重建失败不得恢复写入。文件里的 `CREATE TABLE IF NOT EXISTS` 不会升级旧开发表，尤其不能把早期仅含 `theme_id` 的试验结构当成当前 `subject_type + subject_id` 结构。须另行评审增量迁移，禁止删除真实任务、冻结单或账本来通过检查。

当前有 `internal/ai/task`、任务 SQL 仓储、`AITaskService`、联合回滚探针及有限恢复命令 `cmd/ai-worker`（见后文）；没有公开提交或成功报告/收费路由，会员读取/取消入口独立默认关闭。`points-check` 本身仍只检查账本。迁移脚本中的计数不是完整结构/拒写自检；当前恢复命令需十四张任务依赖表 DML 及来源三表/题材表只读权限，取消装配需十四表 DML 与会员只读权限，不授 CREATE/ALTER/TRIGGER 或来源写权限，不能套用测试 root。

凭据管理系统注入仅目标环境的 `TM_DB_DSN`（含 `parseTime=true`）后，从 `server/` 手工执行：

```bash
go run ./cmd/ai-task-check
```

命令自检成功退出 0，配置/权限/结构/探针失败退出 1；`go run` 会包装退出码，自动化使用编译后的程序。成功只说明当前运行契约和有限拒写反例通过，不等于任意约束改写均可检出，也不是历史积分/任务对账。随机高位用户和随机任务只 INSERT，所有数据在单个 READ COMMITTED 事务内回滚；可能消耗自增编号。任何非预期错误都拒绝通过，不把权限错误或外键错误冒充 CHECK 拒绝。身份、单调计数、四种终态及事件不可变通过真实写入验证，不依赖 TRIGGER 元数据可见性。

15 秒探针上下文、20 秒命令自检上下文；连接/语句等待受其限制，不是含标准输出阻塞在内的操作系统级硬超时。失败出口隐藏驱动详情、DSN、口令和内部约束名；仅获准运维人员在安全环境排查，禁止为方便调试公开驱动错误。`ai-task-check` 不迁移、不修账、不启动 worker、不收费；恢复型 `ai-worker` 已在接管前调用同一 Verify，其他未来执行/提交路径仍须接线，不能以手工通过替代。版本切换和迁移重跑时仍须停止写入，未知旧结构拒绝启动，不自动放宽保护或清空历史。

除显式负例，探针最后也走真实仓储创建/读取路径，核对省略列的默认状态；旧表把尝试次数或取消标记改成非零时同样拒绝通过。`TestIntegrationAITaskSchemaProbe` / `SchemaCorruption` / `SchemaRestrictedCLI` 包含三种会话时区、外键/严格模式关闭、二十一个结构/保护破坏反例、受限账号和真实命令正反例；每个破坏用例先验证同环境健康基线，失败后核对十表均未遗留探针。旧主体列须由缺列判据拒绝，错误默认值须由真实创建状态判据拒绝，不能用其他基础故障冒充覆盖。

原生脚本可选 `-RepositoryCoveragePath '<语句覆盖率 .out 的绝对路径>'`，仅采集跨层套件对仓储的语句计数，后续 GVA 测试不覆盖该文件；输出不含业务数据，但这是测试覆盖率，不是生产指标。目录须事先存在，选择独立输出文件，不能指向业务文件。各版本分别指定不同路径，以免覆盖前一版结果。

内部调用约束：

1. 创建请求只允许题材或专题编号（`theme|topic`）、分析类型、价格版本与幂等键；价格/预算只能由可信服务端取得，不能把客户端金额或模型参数直接传入。未来接口必须先验证身份、主体主数据和访问权限，内部核心不替代这些核查；不接受个股或自由问题。尚无真实模型成本实测，不将测试价格 7 积分作为报价。
2. 创建、首次进度事件与积分冻结在同一 SQL 事务提交；同用户原键重放返回首次稳定收据，价格簿后来变化不再冻结。提交结果未知时必须查询/重试原键和原载荷，不可换键。复用父事务时，内部返回值必须等父事务提交后才对外返回；幂等读取是当前读，不能依赖父事务旧 RR 快照。
3. 金额相关路径锁顺序为账户→任务→冻结单/批次；候选查询不授予租约。抢占使用数据库 UTC 重验期限/租约/尝试次数，再递增围栏；纯心跳仅锁任务、不反向追加账户锁。上一次 owner/fence 不能在接管后续租或重试。租约延期不得倒退、不得越过任务期限。
4. 排队取消同事务释放；运行取消先记不可逆请求，等待有效 worker 确认或租约到期后接管终结。失败/尝试耗尽/期限到期与释放及终态事件同事务，重试不再次冻结。当前没有运行中的自动清扫器，不能承诺调用取消后立即释放所有运行任务；不得靠直接删表、改冻结单或批次清理处理挂起任务。
5. 进度只存白名单事件、阶段编号和错误码，不保存供应商原文、提示词或正文。终态任务及进度历史不可改删；当前没有“成功”占位操作。后续必须在报告终检/落库与积分结算同事务后才能成功。

验证入口仍是上面的原生脚本，或在 `server/` 使用可丢弃 `TM_TEST_DSN` 执行 `go test -p 1 -race -tags=integration ./integration/points -run TestIntegrationAITask -count=3 -v`。四组任务用例分别覆盖完整流转、真实故障回滚/未知提交结果、同键及租约/取消竞争、200 个独立创建；每组均跑 RC/RR。断言包括同键唯一任务/冻结、40 抢占只有一个 winner、200 次以测试价格 7 从 100 积分中创建恰好 14 个任务（可用 2 / 冻结 98），以及事件 INSERT 故障不得留下已释放但未终结的任务。执行期间不要修改代码或迁移；每个测试进程只加载一次任务迁移副本，改动后必须重启验证，旧二进制通过不能作为新代码的证据。

内部预算预留、隔离阶段存储、确定性抽取策略及恢复型 worker 见后文，不是实际用量、语义蕴含或真实模型质量的验收。尚未验收：其余阶段策略/完整流水线、调用日志及 calls/token/页数实际预算累计、成功报告与结算、全队列调度/自动清扫、任务锁价全局对账、其他启动路径装配、全部事务边界的独立进程恢复、积压队列吞吐/公平性。冻结批次到期政策仍待确认，未借任务到期实现自动批次扣减。以上均是内部底座，不代表阶段 3、公开 AI 或上线关口已通过。

2026-10-12 · 任务核增量：冻结后的最终任务代码/迁移在 MySQL 8.0.46 / 9.7.0 各连续 3 次通过仓储、跨层账本十三组/任务四组及 GVA 真库套件，客户端迁移断言两版各通过 30 项；成功实例均已停止并移除。全量 Go/race/vet、任务核心 20 次定向竞态、架构/格式及工作流 YAML 守卫检查通过；总门禁仍被既有疑似密钥扫描及扫描器自测挡住，未放宽基线，远端 CI 未运行。编译后修改迁移造成的混合版本失败运行不算最终验收，其数据/日志继续保留。该批结果只证明上述隔离场景，不证明生产升级、任务受限账号、真实执行成本或崩溃恢复。

2026-10-12 · 任务联合自检增量：最终代码在上述两个版本各连续 3 次通过仓储、跨层账本十三组/任务七组和 GVA 真库套件，均带 `-race`；客户端迁移断言均为 30/30。新增三组自检用例与二十一个破坏子例，含受限业务账号和实际构建命令的正反例；最终成功实例已停止并清理，无本轮 mysqld 残留。跨层仓储语句计数两版一致：联合 Verify 86.7%、任务元数据检查 80.6%、任务拒写探针 85.8%，不是全仓储包验收。全量 Go/race/vet、定向单测 20 次竞态、架构/格式和工作流守卫通过；总门禁仍仅有既有密钥扫描与其自测失败，未扩大基线、未运行远端 CI。首次事件重复错误判据失败，以及发现默认值盲点后主动终止的中间实例保留日志/数据，不计作最终成功；后者不是产品崩溃恢复测试。受限账号结果仅适用于本节列出的测试权限和隔离环境，生产权限/升级、自动启动接线、worker、真实成本、成功报告/结算和独立进程恢复仍未验收。

## 报告编号转换（内部开发接线，无公开入口）

`finalreport.SelectionStage` 只接受受限的章节/快照/连续句子编号响应，不接受 TradingAgents-CN 的原始研究报告。可信装配方须从当前授权仓储读取任务题材与快照构造策略，再经原预算与阶段服务 Capture/Complete。`AIReportService.Convert` 读取已完成的抽取检查点，两次复核当前用户/租约/授权及题材，自动回取原文并注入出处/公示；它只返回内部候选和下游输入摘要，不调用模型、不冻结/结算、不写报告。候选不能直接作为会员返回值，必须另经 judge 检查点和 Finalize。

开发验证：`go test -race ./internal/ai/finalreport ./internal/service`；原生脚本 `-ReportsOnly` 包含转换四根组及原终检六根组。真库新增用例从空任务走预算→编号 Capture/Complete→Convert→judge→Finalize，示例来源与测试判定器只在测试文件中。覆盖 RC/RR 成功/拒绝、身份/旧围栏、来源撤回/许可变更/题材改名/取消、40并发相同候选及租约接管复用。最终阶段的回滚、提交未知、来源提交锁仍由原报告用例验证，不冒充新增转换自己的杀进程证明。

真实选择器与蕴含/语义适配、质量证明、统一 worker、会员报告读取与页面仍未装配；没有任何默认通过的生产校验器，真实调用与公开报告继续关闭。转换后来源还可能撤回，不能跳过最终事务复核。细则见 design 3.2.7、ADR-0017。

## 模型判定接线（开发专用，缺生产准入控制）

`provider.NewDirect` 构造不调用网络，须显式回环评估才允许HTTP；它不读代理环境、重定向或重试，16KiB响应上限，配置摘要不含密钥。将摘要与模型/备案、批准、质量、远端硬预算引用固定进 `CheckConfig`，再由 `Policy.Inputs` 为蕴含及语义分别生成输入。非空引用不是证明，当前没有可生产放行的控制器。

`AIReportCheckWorker.RunOwned` 只执行已经持有租约的一个判定步骤；先原事务 `ReserveStep` 再持久调用前审计，调用后仍有最多5秒保存审计。未知预留、传输错误、审计失败、未知提交均原键保留，不换键或重试；已捕获响应可以由新围栏 Complete/Load。直接使用普通 Provider、缺控制器、缺模型、关停或配置摘要不同均拒绝。受限响应及真实用量在原隔离表，判定步骤不会结算或提供报告。

从两个已捕获证据构造 `NewCapturedPolicy`，它只纯回放输入匹配的判定；蕴含不通过时可以没有语义调用。此后最终候选 judge 与 Finalize 仍不可省略。模型判定原文不会加到报告正文。`-ReportsOnly` 包含 Checks 四根组及原报告/转换测试；全部模型与审计控制替身均显著标注在测试中，不作为真实质量或授权验收。统一编排、生产控制器、会员读取与页面仍缺。

原生积分整包测试看门狗现为每轮20分钟，CI一致；这不是单次SQL/模型、租约或业务期限的放宽。上一增量曾触及默认10分钟且出现上下文错误，保留失败实例与日志；最终运行仍逐项核查失败/跳过/必跑表达式，不能以扩大测试进程时限掩盖断言失败。

## 外部 Agent：内部隔离执行（不是 L0、公开报告或积分消费）

`cmd/ai-agent` 直接适配 TradingAgents-CN 社区版 V2 异步 API，提供 `version / submit / show / status / result / cancel`。上游版本常量只记录**协议参照**，不证明远端实际运行相同提交。开始真实联调前须核对部署版本与许可、模型/来源批准和独立成本限制；所有引用只用于追溯，不替代批准真实性确认。

### 社区版选择与免激活边界

用户已于 2026-10-16 选择社区版，当前固定提交本身就是社区版，无需另换仓库或切换 Pro。固定版本的 [社区版授权实现](https://github.com/hsliuping/TradingAgents-CN/blob/51060a7682cf51810b4bc3a704829438d18cd8dc/core/licensing/community_stub.py) 不进行软件激活验证。按该版本 LICENSE 允许的个人使用/评估，无须把商业授权码当作启动前置；完整应用层仍是混合授权，选择社区版并不证明网站对外服务的商业用途获准。

在 `server/` 可先运行 `go run ./cmd/ai-agent version`，无需数据库、操作人、标准输入或任何凭据；即使环境中执行开关为 true 或配置损坏，也不创建客户端/访问数据库或网络。输出 `scope=adapter_baseline`、`edition=community`、固定提交、`contract=v2_async_analysis`、`software_activation_required=false`、`service_authentication_required=true`、`remote_version_verified=false`。它不是健康检查，不证明模型可用、服务已部署或远端版本吻合，也不改变执行开关。

三种配置不能混淆：服务 Bearer 是独立服务登录认证；模型 API Key/数据源 Token 由对应供应商授予；许可引用只记录适用 LICENSE、版本、用途或另行批准文件的引用，既不是激活序列号，也不能用虚构文号冒充批准。原阶段不执行真实模型；2026-10-21用户另行批准下述本地V2核心真模型评估，不等于批准完整服务部署、公开原文或收费。

### 环境与配置

上游固定版本的 [部署说明](https://github.com/hsliuping/TradingAgents-CN/blob/51060a7682cf51810b4bc3a704829438d18cd8dc/README.md) 要求 Python 3.11、Node.js 18+、MongoDB 7+ 和 Redis 6+；完整异步任务还需核对 [Worker 入口](https://github.com/hsliuping/TradingAgents-CN/blob/51060a7682cf51810b4bc3a704829438d18cd8dc/app/worker.py)，仅 API 可访问不能证明任务会执行。可以本地部署，不强制 Docker；安装包、数据库、WSL/容器和模型付费操作分别经人类确认，独立于本项目目录、虚拟环境、数据库和凭据，不覆盖既有 MySQL 或 Python。启动仅绑定回环，不公开代理上游管理界面或 API。

本机 2026-10-15 初次只读检查有 Python 3.12/3.13、Node 22、uv；未检测到 Python 3.11、Docker 命令/常见安装路径及 MongoDB/Redis Windows 服务。此后用户明确授权安装依赖，并要求“本地部署不能解决问题就不本地部署”。已完成下节的独立 Python 依赖准备，**没有安装数据库服务、Docker/WSL 或启动完整应用**。本地环境可验证依赖/协议，不会自动建立线上网站到本机的受控连接，也不能解决许可、授权来源、真实成本及公开报告/结算关口。许可范围见 ADR-0010；不能因本地部署就推定允许为商业项目服务。

#### 本机依赖环境（已准备，不是已部署）

- 独立目录：`D:/workspace/services/tradingagents-cn-eval-51060a7`，在本仓库之外；`source/` 固定为上述提交的干净 detached checkout，没有修改上游跟踪文件。
- 解释器：`.venv/Scripts/python.exe`，使用同目录 `runtime/python/` 下的 Python **3.11.17**。只通过隔离工具环境使用 uv **0.13.1**，原系统 Python **3.13.1**、uv **0.7.19** 与 PATH 未替换。
- 依赖权威输入为该提交的 `pyproject.toml`；`requirements.txt` 自述已弃用，`uv.lock` 的项目元数据仍是旧 `tradingagents 0.1.0`，不能用 `--frozen` 假报当前应用依赖完整。全平台 `uv lock --check` 又在未启用的 `qianfan` 可选组解析失败，本环境只安装 Python 3.11 基础依赖，不启用该可选组。
- 实测发现宽松范围会装入 MCP v2，而源码仍导入 `mcp.server.fastmcp`。外部目录的 `constraints-py311.in` 明确约束 `mcp>=1.12,<2`，最终安装 MCP **1.30.0**；不修改上游 API，也不把命名改写当兼容验证。
- 活动清单为 `requirements-py311-v1.lock`，**224 个固定版本，带分发文件哈希**；SHA-256 为 `0ceaa68e80ff34023ee2442a86044409f041c4e8c2a3ab8cbccf0fd9acde738b`。无约束的初始清单改名为 `requirements-py311.mcp2-incompatible.lock` 留作失败依据，**不可用于重装**。只在这个独立环境安装/同步，不操作系统解释器或其他项目虚拟环境。

在核对上述目录与解释器后，可在 PowerShell 复用活动清单：

```powershell
$agentsEvalRoot = 'D:/workspace/services/tradingagents-cn-eval-51060a7'
$agentsEvalPython = Join-Path $agentsEvalRoot '.venv/Scripts/python.exe'
$agentsEvalLock = Join-Path $agentsEvalRoot 'requirements-py311-v1.lock'
uv tool run --isolated --from 'uv==0.13.1' uv pip install --require-hashes -r $agentsEvalLock --python $agentsEvalPython --link-mode copy --default-index 'https://pypi.org/simple'
uv tool run --isolated --from 'uv==0.13.1' uv pip check --python $agentsEvalPython
```

验收范围：224 个安装版本逐项与活动锁定文件一致、全部基础直接依赖满足 `pyproject.toml`、`uv pip check` 通过；15 个主要 SDK 导入与 745 个上游 Python 文件的 AST 语法检查通过；实际 V2 请求结构正常输入可序列化、非法引擎被拒绝，规划模型、LLM 客户端及 MCP v1/记忆接口符号导入通过。**这些不证明完整应用启动、真实 Agent 执行、质量、费用、安全漏洞审计或生产兼容**。

导入 Agent 包会运行配置初始化入口并产生本环境的默认空密钥配置与日志，不能当成无副作用操作。验证明确使用 `USE_MONGODB_STORAGE=false`、指向不存在文件的 `DOTENV_FILE`，JWT/CSRF 仅进程内随机生成且不输出/保存；不创建模型客户端或执行图。Windows 子进程使用 `python.exe -X utf8`，避免上游 Emoji 日志在 GBK 输出中触发编码错误；该修正不改系统区域/编码设置。上游 Pydantic 旧写法、旧配置管理器和 Google 旧 SDK 的弃用警告仍存在，不屏蔽成“全绿”。未创建真实 `.env` 或默认管理员，未安装上游前端 npm 依赖，未启动 API/Worker/同步任务；完成后 18000、27017、6379 端口均未监听。

完整服务暂不部署：本地启动不是线上接入与全部功能完成的替代品。后续若需经批准的真实联调，先确认服务部署位置、适用许可、模型/来源配置和成本限制，再准备相应数据库与服务；不因依赖已备而自动启动。

#### V2核心真模型隔离验收（不是Web/队列部署）

`scripts/dev/tradingcn-live-gateway.mjs`与`verify-tradingcn-live.py`用于固定社区版V2完整核心工作流，不运行已停用的legacy图。调用前重新确认费用授权；维护者通过本地凭据环境提供新Key，不复制聊天中已暴露的Key。`execute:false`默认只做零外部模型请求的构建/计数器预检，`execute:true`才产生真实模型费用。

网关只监听字面回环地址/临时端口，真实Key仅在父进程内存，子进程只持有本次网关会话凭据；固定转发DeepSeek官方HTTPS地址和`deepseek-flash`，禁代理/重定向/自动重试、浏览器Origin、任意路径/模型与远程图片。每次输出上限2048、关闭思考，至多128次请求和15分钟看门狗；这些是测试技术边界，不是人民币硬预算。传输/响应未知后拒绝继续提交，不能以超时认定未收费。上游旧定价目录可能将此模型记录为0元，不能当账单，使用`provider-receipt.json`记录官方实际用量、确认响应和未知状态，金额以供应商账单为准。

仅复制固定提交的Git跟踪源码，要求原跟踪文件无修改，排除.env/日志/缓存/运行数据和符号链接；所有导入副作用、默认配置与报告均留在临时副本。子进程使用最小环境，网络仅允许模型网关，不连接已有MongoDB/Redis或产品数据库；采用原库代码配置模式，资料工具替换为明确标注的`TMEVAL`测试资料，记忆关闭。14个原Agent类、原V2工作流与实际DeepSeek适配器均保留，额外_generate钩子只观测原调用成功，不替换模型响应。固定上游的invoke绕过LangChain标准回调，因此必须先通过直接invoke/工具绑定的本地传输替身计数自测，再核对每个原Agent的真实成功调用，不能根据流程结束或日志文字推导全部实调。

Windows复验示例（Key须已从受控本地凭据环境载入，不写命令行）：

```powershell
node --input-type=module -e "import {startEvaluation} from './scripts/dev/tradingcn-live-gateway.mjs'; const s=await startEvaluation({credential:process.env.DEEPSEEK_API_KEY,source:'D:/workspace/services/tradingagents-cn-eval-51060a7/source',python:'D:/workspace/services/tradingagents-cn-eval-51060a7/.venv/Scripts/python.exe',execute:true}); await s.finished; console.log({state:s.child,requests:s.requests,output:s.output}); if(s.child!=='passed'||s.receipt_error)process.exitCode=1"
```

结束后检查子进程退出0、`evaluation-summary.json`的14个原类及每节点真实成功调用、`missing_agents=[]`，以及独立`provider-receipt.json`的确认数/HTTP成功数/用量/未知状态。`quarantined-result.json`仅为测试资料的内部生成物，不发布、不导入正式资料库，不冒充本项目报告终检/积分结算或真实市场质量验收。网关纯守卫自测`node --test scripts/dev/tradingcn-live-gateway.test.mjs`不需要Key、不访问网络，已接harness/CI；真实调用不能接入常规CI。V2异步HTTP认证、MongoDB/Redis/Worker、真实授权资料、网站整链和跨进程恢复仍须分别验收。

#### 本地网站—真实V2—报告终检与测试结算

`tradingcn-local-chain.mjs`提供项目自有的一次性异步评估桥，并非启动社区版完整Web/MongoDB/Redis队列。先启动官方HBuilderX编译器的实际本地H5源（字面回环地址，例如`http://127.0.0.1:5174`），核对AI页可加载。不要用线上页面、现有数据库或正式会员登录；模型调用有真实费用，积分及资料均为明确测试夹具。

通过受控本地环境将新模型Key载入父Node进程，调用`startLocalChain({credential,source,python})`；不要打印credential、bridge对象或reportEndpoint对象。网关要求固定社区提交与原跟踪文件干净，模型Key仅在父内存，Python及Go/PowerShell子进程均不继承产品配置，只接收临时桥会话凭据。`startChainVerification(bridge,{powershell,mysql,bash,goBin,gccBin,origin})`接受本机程序绝对路径，由既有原生脚本新建独占MySQL实例；读取`<output>/local-chain-go.log`里的`/__test__/`地址。最小Windows子环境保留PATHEXT，否则PowerShell会把.exe误作“文档”而无法在管道中执行；日志明确UTF-8。

验收入口必须同时带`integration,liveagents`构建标签、`TM_TEST_AI_LIVE=local-fixture-only`、回环临时桥地址/凭据和实际H5源。原生脚本的`-LiveAgents`只接受`-ReportsOnly -ReportPattern '^TestIntegrationAIReportLiveAgents$' -RunCount 1`；默认套件和CI不编译该付费测试，也不读取模型Key。不要手工改产品开关来替代该入口。内部测试批准/备案/质量/虚构价格仍不是生产证明。

浏览器选测试会员一，实际页面搜索“示例”、核对7测试积分报价、二次确认创建，进入创建收据对应任务，等待后点击“核对并查看报告”。原Go适配器提交600001仅为格式占位，桥原引擎始终分析非证券TMEVAL；桥固定执行完整原V2图，不声称模拟上游Web的分析师/风险参数裁剪。报告正文必须是测试快照中的四节原句，披露真实`deepseek-flash`及显式test-only编号；原Agent报告只作隔离选句线索。

DeepSeek Chat文档仅保证[JSON对象模式](https://api-docs.deepseek.com/zh-cn/guides/json_mode/)。本地桥显式把json_schema请求译成json_object，并原样附加schema作为系统约束；不改写任何回包，Go严格JSON、枚举/用量/完成原因、原文回取、双独立判定及终检保持不变。三次报告模型调用限额独立，未验证核心前不能使用报告凭据；请求和用量分core/report记录，报告回包不能填补缺失的核心观测。它不是远端strict schema或生产质量证明。

看到报告后回到同一源控制页点击“结束并核对测试”。只有原任务成功、冻结归零、恰好一次原子报告/settle、三个真实报告步骤审计、Agent/模型替身零调用、其他会员404、终态重放无重复结算及积分审计均满足，Go才通过；随后原生脚本仍需启动门禁和48项迁移检查通过。执行`await finishChainVerification(bridge,verification)`，检查`local-chain-receipt.json`的`verified=true`、`go_exit=0`、唯一提交/取结果、真实provider用量齐全且无unknown；函数最后清除父模型Key并关闭模型/异步监听。失败也要执行收尾，不能仅等Node内核销毁；未通过的回执不能升级为成功。

2026-10-21最终通过目录`C:/Users/liufm/AppData/Local/Temp/tm-stock-tradingcn-live-Dpk4sa`：21个HTTP200（18核心＋3报告），输入32994/输出9398 token，异步提交1/轮询104/结果1；原14主Agent、17编译实例及1委派Agent实调，8次显著测试资料工具调用。浏览器实际四节报告截图`local-report.png`，Go测试/启动/迁移日志`local-chain-go.log`。此前`fUftga`轮21次HTTP200但发布测试控制器写死旧模型，报告GET被422拒绝，整链回执false；保留失败证据和用量，不降发布门禁，也不把已有响应回放冒充第二次真实调用。修复测试控制器为创建前固定批准模型/披露字段，并补错模型、披露及政策拒绝反例；第二次完整真调用才计通过。生产资料、完整社区版Web队列和跨进程崩溃恢复仍未验；没有线上部署或真实用户积分变动。

先在隔离库执行 `20261015_create_tm_ai_agent_runs.sql`。普通模式需要：

| 环境变量 | 要求 |
|---|---|
| `TM_DB_DSN` | 已迁移的内部数据库；含 `parseTime=true`；凭据仅经环境注入 |
| `TM_AI_AGENTS_ENABLED` | 明确 `true` 才执行远端操作；默认关闭 |
| `TM_AI_AGENTS_URL` / `TM_AI_AGENTS_APPROVED_HOST` | 精确批准的 HTTPS 根地址/小写域名，不带路径、查询、凭据，端口默认或 443 |
| `TM_AI_AGENTS_TOKEN` | 独立上游服务器 Bearer 凭据，32–8192 字节可打印无空白 ASCII；不是会员 JWT，也不是软件激活码 |
| `TM_AI_AGENTS_APPROVAL_REF` / `TM_AI_AGENTS_LICENSE_REF` | 真实评估批准与适用许可/用途引用，均必填；后者可指向允许该用途的固定版本 LICENSE，不要求购买软件序列号。示例/占位引用不构成批准或商用授权 |
| `TM_AI_AGENTS_QUICK_MODEL` / `TM_AI_AGENTS_DEEP_MODEL` | 已批准的固定模型名称；不由 C 端指定 |
| `TM_AI_AGENTS_TIMEOUT_MS` | 明确 1000–30000；仅单次 HTTP 时限，不是远端执行/费用上限 |
| `TM_AI_AGENTS_MAX_SUBMISSIONS` | 明确 1–1000，同配置跨操作人/进程累计预留，不因未知失败返还 |
| `TM_AI_AGENTS_ACTOR_ID` | 实际获准内部操作人编号；不同操作人不能读取或复用对方请求 |

本地评估另明确设置 `TM_AI_AGENTS_LOCAL_EVALUATION=true`，地址只接受字面回环 IP HTTP 与有效端口，例如批准主机 `127.0.0.1` 和地址 `http://127.0.0.1:18000`。不接受 `localhost` DNS、内网 IP 或公网地址；仍需上表全部批准、模型、令牌和限额。这个开关只改变连接边界，不免除商用许可、不允许真实用户数据或公开输出。同一上游服务账号的令牌轮换不改变配置身份；更换服务账号须重新批准并变更批准引用，不能仅替换令牌。修改模型/地址/许可等形成新配置，必须重新批准，不能改引用来绕过提交限额。

### 操作与恢复

在 `server/` 运行，JSON 从标准输入读取；正文、凭据和身份不能作为命令行参数传入：

```bash
go run ./cmd/ai-agent version
go run ./cmd/ai-agent submit < agent-request.json
go run ./cmd/ai-agent show < agent-lookup.json
go run ./cmd/ai-agent status < agent-lookup.json
go run ./cmd/ai-agent result < agent-lookup.json
go run ./cmd/ai-agent cancel < agent-lookup.json
```

`version` 不读取输入；`submit` 只接受 `request_id` 与六位 `symbol`；其他命令只接受 `request_id`。编号 8–64 个字母、数字、下划线或连字符，首位字母/数字。代码格式校验不证明证券类型、实际存在或上游角色未被自动分流；实际多 Agent/工具执行仍要联调核对。不接受自由提示词、工作流覆盖、会员身份、任意远端地址、重复键或附加 JSON。PowerShell 可用 `Get-Content -Raw .\agent-lookup.json | go run ./cmd/ai-agent show`，文件中不要存凭据。

1. 首次提交预留名额、请求、事件同事务；提交响应只有编号校验通过后才保存映射。任何错误都不补交；`submitting`（可能进程中断）或 `submission_unknown` 须人工到上游按原时间/操作记录核对，保留原请求编号，不换键、不删除记录重置限额。本工具尚无人工绑定未知远端任务的恢复入口。
2. `status` 查询实际远端状态；上游完成记为 `upstream_completed`，并非本项目审核通过。`result` 再获取结果，校验编号与报告对象，已知服务令牌脱敏后隔离保存；不能保证发现所有未知敏感数据，隔离表仍须严格限权。命令只返回状态/摘要，并明确没有可信账单用量和公开报告，不返回正文。
3. `cancel` 先保存请求再发送一次。受理只说明请求被接受；失败记 `cancel_unknown`，进程中断可能保留 `cancel_pending`。重复命令不会再次发送；继续用 `status` 核对，不能声称工具已停止、未收费或已退费。终态不被迟到观察覆盖。
4. 执行开关关闭时，`show` 不需要上游配置，仍可查本地状态；其他命令拒绝。关闭本工具不会停止远端已入队任务，紧急取消需明确授权后按原地址/编号操作，不临时改成公开 HTTP。
5. `submit / status / result / cancel` 在远端操作前做单事务回滚拒写探针。缺关键保护即停止，不自动迁移/修表。`show` 为本地历史读取，`version` 为纯本地协议参照，不执行探针。探针仅验证有限反例，不代表旧结构已升级或任意破坏均能被发现；生产迁移和受限账号仍需另验。

回归使用原生隔离脚本，`-AgentsOnly` 仅用于快速定位该支线，完整回归不带此选项；脚本会保留失败实例的日志/数据并关闭实例，成功实例才安全删除。跨层六组覆盖生命周期/隔离、并发与持久限额、审计故障、保护破坏、取消竞争与事件一致性、编译后命令出口；HTTP 契约用本地模拟服务，**不证明真实上游执行、质量或费用**。独立进程杀死/恢复、生产权限/升级、远端硬预算、受控来源、公开证据转换与成功结算尚未验收。

### 会员任务只读开发入口

用户 2026-10-15 要求先开发、后联调。此增量不启动 TradingAgents-CN、不触发真实模型或收费；`cmd/api` 只在显式 `TM_AI_TASK_READ_ENABLED=true` 时注册 `GET /api/v1/ai/tasks` 和 `GET /api/v1/ai/tasks/{id}`。默认保持关闭，不因开发付费放行自动开启。完整参数/DTO 契约在 design.md 2.4.1；独立默认关闭的取消入口见下一节，报价、提交、报告、反馈和成功结算仍未开放，恢复 worker 不进行真实调用。

后续本地或受控环境验证时，从仓库外凭据提供当前及可选上一把签名密钥；另设 `TM_JWT_ISSUER`、`TM_JWT_AUDIENCE`，必须匹配 GVA 实际签发配置（当前代码受众为 `GVA`，发行者由部署配置决定）。开启只读能力不代表取得公开 AI 上线授权。无需发送密钥到聊天或修改 GVA 配置；未知线上版本/构建与授权关口未解决前不得部署。过渡期只接受新旧两把配置内的 HS256 密钥；旧钥移除后旧令牌失效，怀疑泄露应立即移除，不保留兼容窗口。

请求只用单个 `Authorization: Bearer` 头，会员类型必须为 `client` 且实时查询到启用/未删除会员；不接受后台令牌、x-token、Cookie 或 URL 令牌。401 表示身份不成立，503 表示校验或查询依赖暂不可用；不能为排障放宽签名/类型/时效校验。非本人和不存在的任务都为 404；历史按 `before_id` 倒序分页，未生成报告时保持不可用，不用 Agent 原文或成功状态冒充报告。

复用原生可丢弃 MySQL 脚本，不带 `-AgentsOnly`；新增 `TestIntegrationAITaskMemberReads` 真实走回环 HTTP、签名、会员表查询、任务 SQL 和 DTO。只在测试随机库创建最小会员身份表，不迁移实际 GVA 会员库。覆盖两名会员隔离、全局奖励幂等键独立、分页、后台/停用/删除/未知账号、验签轮换、取消后只读状态、依赖故障及 GET 不产生积分/事件。该测试不证明线上 GVA 登录/网关/前端已经联通，也不替代真实模型联调。

2026-10-15 本地验证：最终代码双版本各完整通过一遍上述脚本，迁移断言两版各 30/30；成功临时实例已停止并清理。会员验签包 96.2%、任务查询 service/handler 新增函数 100%，配置及 DTO 闭集契约也已单测。CI 登记会员必跑组并把总下限增为 112，尚未远端执行。总门禁仍被既有旧工作树疑似密钥扫描和扫描器自测阻挡，不能声称全部完成或可上线；未改基线。失败的奖励幂等键测试夹具实例已停止、日志和数据保留；修正后通过不抹去失败记录。

### 私有会话元数据与历史（2026-10-21，开发增量）

`TM_AI_CONVERSATIONS_ENABLED`默认false；打开须有原任务读取/会员鉴权配置，且`20261021_create_tm_ai_conversations.sql`迁移及启动回滚探针通过。它只注册design 12.2.1的创建/列表/详情/原键核查/改名/归档与历史GET；不注册POST messages，不执行问答或收费任务，不改变模型、报告、创建、取消及账务开关。不要在生产尝试启用或自动执行DDL。

元数据写权限仅新会话表INSERT/UPDATE，历史为SELECT；消息INSERT仅为受控内部服务预留，无公开写历史入口。启动探针会回滚测试新会话/消息，需要相应DML但没有DELETE/TRIGGER/DDL或账本写入需求；不可变删除/更新反例在回滚事务中应被触发器拒绝，缺保护即整体启动失败。独立只读服务可仅授会话两表SELECT用于GET验证，不因读历史申请账本/任务写权限。

创建以单个Idempotency-Key及精确JSON标题返回稳定`id/createdAt`收据；结果未知查询`GET /api/v1/ai/conversation-receipt`并带原键，不能自动换键重建。标题改变或会话归档后仍可核对原收据；同键异体40903。改名/归档用expectedVersion，冲突只查当前元数据，不自动重发。归档不删除历史，不取消任务、不退款。历史按连续序号与固定RR上界读取；只含用户输入/任务引用，不保存模型答案或报告正文，不据任务ID推导报告可读。

复验：`go test -race ./internal/ai/conversation ./internal/service ./internal/handler ./internal/repository ./internal/config ./cmd/api -run '^Test(AIConversation|Conversation)' -count=10`。两版MySQL分别运行`verify-ai-source-native.ps1 -ConversationsOnly -RunCount 1`；专项互斥，使用本次新建回环实例，不读取已有业务DSN；原会话四根组、RC/RR六子组、13类保护破坏及conversation_off/on保留，另增意图四根组、RC/RR六子组/18叶场景、6类保护及conversation_tasks_off/on，最后执行mysql客户端全部迁移。它不是全仓SQL回归，也不验证浏览器/真实问答/付费多Agent。常规CI不调用真实模型；会话关键包覆盖率棘轮90%，不降低原门槛。

会话任务另须`20261021_create_tm_ai_conversations_tasks.sql`及`TM_AI_CONVERSATION_TASKS_ENABLED`（默认false）；原创建开关关闭时仅提供查旧意图/收据与显式POST核查，不注册准备/确认。当前生产准入工厂仍为空，不得为试用改成放行。Prepare正文是原对象/档位/价格版本四字段，金额仅取服务端原报价；Confirm须精确`{"confirmed":true}`，Reconcile须精确`{}`。H5轻度题材现已先保存原意图键、确认已发标志及已知编号；未知时只用该意图核查，不换键、不直接调用原创建API补交。`unconfirmed`且原收据404也不是退款/可重建证明；进程若在权限落库后、原创建前中断，当前实现会保留待核查态，不自动补交。会话有未确认意图时不能归档。关联引用不授予报告正文权限，报告仍逐次走原终检/发布读取。

新本地SQL用例通过真实会员JWT、原准入/冻结、测试worker/终检及测试账本验证成功恰好一次结算与拒绝后释放；模型/Agent端是既有回环测试服务，不是真实付费调用。执行数/账务计数、绑定回滚、两类提交未知、关闭新建后的恢复、同版本金额变动和跨会员拒绝分别断言，不能把HTTP200里的`unconfirmed`算作创建成功。完整会话→真实Agent及实际H5仍按C-8验收，不因本专项通过勾选总目标。

#### 会话面板及H5进度的本地复验

现有AI页已接元数据/连续历史及轻度题材任务面板，不是完整会话优先主界面。新建先保存独立原键，未知仅GET原收据；损坏存储/缺WebCrypto或WebLocks时拒绝POST，不手工清键绕过。标题改名及归档后须显式刷新列表；归档不能删除历史、取消原任务或退款。问题/报告追问、深度及专题仍未开放，原独立任务创建不自动绑定当前会话。

在仓库外隔离依赖目录按CI固定版本安装Vue/compiler-sfc/server-renderer3.5.13、TypeScript5.7.2、Axios1.8.2、vue-router4.4.3；分别执行`node scripts/dev/verify-ai-report-ui.cjs <依赖目录>`及`node scripts/dev/verify-ai-task-stream.cjs <依赖目录>`。前者执行实际组件/封装与状态隔离，后者执行实际UTS解析器/订阅器及可控流/时钟反例；不把VM宿主当作真实H5。实际编译使用已安装HBuilderX内的官方uni-app x编译器，不另装不匹配的uni/Vite依赖、不修改vendor。

新开一次性PowerShell会话，在仓库根按实际安装路径运行（下列仅本机路径示例，不含凭据）：

```powershell
$compileRoot = Join-Path $env:TEMP ('tm-stock-h5-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $compileRoot | Out-Null
$env:HX_APP_ROOT = 'D:/workspace/services/tm-stock-hx-5.26/HBuilderX'
$env:UNI_PLATFORM = 'h5'
$env:UNI_INPUT_DIR = (Get-Location).Path
$env:UNI_APP_X = 'true'
$env:UNI_OUTPUT_DIR = Join-Path $compileRoot 'h5'
$env:UNI_APP_X_CACHE_DIR = Join-Path $compileRoot 'cache'
$env:UNI_APP_X_TSC_DIR = Join-Path $compileRoot 'tsc'
$env:UNI_APP_X_TSC_CACHE_DIR = Join-Path $compileRoot 'tsc-cache'
node "$env:HX_APP_ROOT/plugins/uniapp-cli-vite/node_modules/@dcloudio/vite-plugin-uni/bin/uni.js" --host 127.0.0.1 --port 5174 --strictPort --clearScreen false
```

`HX_APP_ROOT`与`UNI_PLATFORM`必须在Node模块加载前设置，官方Sass来自HBuilderX插件目录；缺设置可能错误扫描backend HTML或找不到现有Sass。同样环境以`build --platform h5`替换开发参数可离线构建，产物留在临时目录，不上传。2026-10-21最终5.26独立H5构建退出0（`tm-stock-assistant-h5-final-20261021.log`）；原其它页面onLoad/onShow、空值及authApi等警告、组件Sass旧语法与cmark模块externalized警告保留，不代表全站无警告/所有浏览器功能验收；本增量AI路径无新诊断。浏览器严格UTF-8必须用WEB内`window.TextDecoder`，UTS同名全局并非该API，详见原技能库兼容性条目。

另开一次性PowerShell会话，PATH中提供Go/GCC/Git Bash，仅针对新建隔离MySQL实例：

```powershell
$env:TM_TEST_AI_H5_ORIGIN = 'http://127.0.0.1:5174'
$env:TM_TEST_AI_H5_ASSISTANT = 'local-fixture-only'
try {
    ./scripts/dev/verify-ai-source-native.ps1 -ReportsOnly -RunCount 1 `
      -ReportPattern '^TestIntegrationAIReportH5Gateway$/^Read_Committed$/^running_cancel$' `
      -MySqlBaseDir 'D:/mysql/mysql-8.0.46-winx64' -BashPath 'D:/Git/bin/bash.exe'
} finally {
    Remove-Item Env:\TM_TEST_AI_H5_ORIGIN -ErrorAction SilentlyContinue
    Remove-Item Env:\TM_TEST_AI_H5_ASSISTANT -ErrorAction SilentlyContinue
}
```

进入日志给出的回环`/__test__/`，用隔离会员在当前AI页新建一个会话、改名并确认归档，显式查询归档列表；切会员查询不能看到他人会话。原任务创建之前完成这些操作，并观察正常15秒流到期后按原游标续读。随后在原30秒worker窗口内连续完成原报价确认→运行态→取消确认→原任务核查；不要在执行窗口内耗时截图或多轮等待。可按既有375px iframe入口检查换行，但不称为真机验收。完成控制页“结束并核对测试”后，还必须等待父脚本退出0：新建/改名/归档各一次、至少两次stream、原创建一次、worker正常退出、CN测试提交1/模型0、远端测试Agent仍Running、本地冻结0、无报告/结算及账本审计全部成立才算通过。正常续读也可在排队取消场景观察；选择叶子时必须两端锚定，`cancel$`同时匹配`running_cancel`，不能用它声称只跑一个用例。首次误匹配超时失败与精确重跑通过的日志均见tasks.md，未提高原worker期限。

无人工介入的原回归须清除以上两个环境变量及下节的`TM_TEST_AI_H5_INTENTS`，再运行`-ReportsOnly -ReportPattern '^TestIntegrationAIReportH5' -RunCount 1`，8/9版实例顺序执行以避免本机内存不足；RC/RR六种原模式、报告/创建启动及客户端55项迁移仍须全绿。该测试网关新增真实会话handler与SSE，只为隔离开发；stream直接使用真实ResponseWriter，不穿会丢失Flusher的Recorder，也不持有测试串行锁等待流结束。它不是生产代理、容量、生产登录或真实付费多Agent验证，所有产品默认关闭开关保持不变。

#### 会话内报价确认、原任务关联与未知恢复

先沿用上述官方HBuilderX 5174开发服务，源码复验同`verify-ai-report-ui.cjs`，无需再装新依赖。新会话选定后会显示共用报价组件的“为当前会话选择轻度题材资料”；原独立创建入口隐藏。保存报价意图不冻结，确认前须重新核对资料范围、版本与金额，再二次确认。已归档会话只读。元数据/意图列表都是查询时快照；创建或关联后当前详情会重新核对，列表需显式刷新，不把列表旧状态当作任务终态。

新开一次性PowerShell会话，PATH提供Go/GCC/Git Bash；复验成功报告及反馈，精确选一项：

```powershell
$env:TM_TEST_AI_H5_ORIGIN = 'http://127.0.0.1:5174'
$env:TM_TEST_AI_H5_INTENTS = 'local-fixture-only'
Remove-Item Env:\TM_TEST_AI_H5_ASSISTANT -ErrorAction SilentlyContinue
try {
    ./scripts/dev/verify-ai-source-native.ps1 -ReportsOnly -RunCount 1 `
      -ReportPattern '^TestIntegrationAIReportH5Gateway$/^Repeatable_Read$/^feedback$' `
      -MySqlBaseDir 'D:/mysql9/mysql-9.7.0-winx64' -BashPath 'D:/Git/bin/bash.exe'
} finally {
    Remove-Item Env:\TM_TEST_AI_H5_ORIGIN -ErrorAction SilentlyContinue
    Remove-Item Env:\TM_TEST_AI_H5_INTENTS -ErrorAction SilentlyContinue
}
```

进入日志中的回环控制页，选隔离会员→新建会话→搜索示例题材→核对报价→保存会话意图。可先重载：应保留原键，旧独立入口提示“有会话任务意图待核查，暂不新建”；重新打开原会话，只GET原意图，不能第二次Prepare。重新核对当前报价/资料范围→确认冻结→由会话引用查看原任务/四节报告→提交一条反馈→控制页“处理测试反馈”→会员页查询已处理。处理控制仍执行原内部鉴权/handler，非GVA登录验收。记录完成按钮先独立GET终态再清同键本地保护，不删除服务器历史；切隔离会员二应无前一会员会话/报告，旧页在令牌监测后移除私有内容。

最后控制页“结束并核对测试”后必须等父脚本退出0：新会话1、Prepare/Confirm/原创建核各1、独立任务创建POST=0、意图linked及task引用各1；原worker终检报告/结算各1、冻结0、反馈受理1/三事件/处理版本3、CN/模型1/3与账本审计通过。这里CN/模型仍是显著测试回环服务，绝不冒充真实付费推理。仅以页面Toast或控制页回执不能通过。本地守卫/产品开关无关，不得在生产设置此测试环境变量。

未知恢复复验使用MySQL8目录、选择器`^TestIntegrationAIReportH5Gateway$/^Read_Committed$/^cancel$`，将`TM_TEST_AI_H5_INTENTS`设为`local-bind-unknown`，其余保持相同。夹具只在关联引用写事务中真实回滚一次，原创建核已提交，不能手工改任务状态。确认后立即刷新→打开原会话→查询本机原意图应为unconfirmed，禁止再次确认。用户显式“核查并恢复原任务关联”只用原收据绑定；若首次仍未确认，可再次显式查询/恢复，绝不重发Prepare/Confirm。恢复原任务后按原取消组件确认并独立查询取消终态，父脚本须核对冻结0、无报告/成功结算、CN/模型0且创建核仍1次。早于HTTP返回的刷新可能中断首次绑定，使下一次恢复才触发那次真实回滚；不把第一次恢复503称为永久失败或退款。

两个入口用共同会员级WebLock执行预约、确认标记及清键；原键分区独立，其他入口的未知/损坏记录一律拦新建。缺安全能力或存储读回失败时零提交，不使用时间戳随机键兜底。确认已发标志或服务器权限提交后中断可能保留未确认态，当前不提供抹掉记录后再试的捷径；须核对原状态，不能以404、未收报告或刷新推定可重新扣分。终态引用/清键都不是积分退款证明。

自动化真库复验先清除`TM_TEST_AI_H5_ORIGIN/TM_TEST_AI_H5_ASSISTANT/TM_TEST_AI_H5_INTENTS`，依次在8/9版运行`-ReportsOnly -ReportPattern '^TestIntegrationAIReportH5' -RunCount 1`。新增`TestIntegrationAIReportH5Conversation`有1根/2隔离级别/4成功与绑定回滚子例，检验受控门面、先准备零冻结、他人404、一次原创建/唯一引用、只读重放零改账及成功结算/取消释放；没有浏览器源时只返回显著代理占位，不算真实H5。CI为新增1/2/4单独守卫，总下限仅升至1023，不删旧守卫；最终本地日志/浏览器截图和一次计数夹具修正见tasks.md，远端CI另验。

### 受限资料问答执行核（公共开关默认关闭，不替代C-4完整验收）

`server/internal/ai/qa`只处理可信计划中的`material_qa`：全部有界快照候选→结构化编号选择→快照原句回取→独立蕴含判定→独立语义判定→固定模板。无证据/越界/禁止内容固定拒答；技术故障不冒充无证据。原问题及资料仅进入JSON data，模型没有自由正文、URL、工具或费用输出权限；显示时再次复核当前授权。身份/序号、范围、快照哈希及模型配置绑定到执行摘要，不能把旧结果用于新问题或对象。

在`server/`执行以下专项（Go/GCC已加入PATH）：

```powershell
go test -race ./internal/ai/qa -count=10
go test ./internal/ai/qa -count=1 -cover
go test ./internal/ai/qa -run '^$' -fuzz '^FuzzClosedDecoders$' -fuzztime=10s -parallel=2
```

使用现有单次直连Provider对回环HTTP供应商实际发请求，核对最多3次、无代理/重定向/修复重试、模型/端点/整数用量、错误已耗用、超时/取消、预约/结果/最终存证失败、20类以上协议拒绝、逐字偏移、跨问题结果复用及每次读取撤权。并发同问题只有一次执行；控制器是显著测试内存记录器，不是持久用户/实例配额或真实授权仓储。普通测试无需真实密钥、不接账务或报告创建。

已有下节SQL持久控制器及默认关闭的会员服务/启动保护，不能将测试控制器或批准夹具装配到产品。执行异常后不自动重发，不将未确认审计记为零消耗/自动释放预算。`report_qa`、真实语义质量、前端完整问答和浏览器纵向验收仍未完成；此专项不能勾选C-4/C-5/C-8或总体目标。原创建/结算链路未改。

#### 私有问题与执行存证（SQL底座，不能单独开放问答）

新增`20261021_create_tm_ai_qa.sql`的五表/五CHECK/十五触发器：问答范围批准、日政策默认关闭；按当前题材/会员、批准的完整策略及快照范围、当前来源授权复核，不凭快照存在放行。专题对象尚未适配，明确不可用。`qa.DurableControl.Enqueue`将问题、会话序号/版本及queued记录同事务写入，原键重放不复制消息。`Run`先持久唯一执行权及UTC日实例/会员两层最坏token预留，再经原执行核逐次预约/存证；错误保留预留，不调用积分或收费任务。`ReadStored`只回放原闭集阶段和最终引用，并每次重验当前授权，不能用一个完成标志或三条用量冒充有据回答。

复验当前增量：先将Go/GCC/Git Bash加入一次性PowerShell会话PATH；两版MySQL顺序执行，不同时启动以免内存不足。

```powershell
./scripts/dev/verify-ai-source-native.ps1 -ConversationsOnly `
  -ConversationPattern '^TestIntegrationAIConversationsQA' -RunCount 1 `
  -MySqlBaseDir 'D:/mysql/mysql-8.0.46-winx64' -BashPath 'D:/Git/bin/bash.exe'
./scripts/dev/verify-ai-source-native.ps1 -ConversationsOnly `
  -ConversationPattern '^TestIntegrationAIConversationsQA' -RunCount 1 `
  -MySqlBaseDir 'D:/mysql9/mysql-9.7.0-winx64' -BashPath 'D:/Git/bin/bash.exe'
```

原`-ConversationsOnly`不传选择器仍跑全部会话/意图/新问答；自定义选择器必须显式该模式并保留会话根前缀，不能用定向通过代替全套。原QA持久四根/八隔离子组/38叶用例保留，新增HTTP/结构三根、四隔离子组、18接口反例及20结构破坏场景；另保留原报告读取启动和新增10问答装配子例、mysql客户端全迁移。测试实际提交后再制造queued/running/调用审计/answered回执未知；未知执行权限不自动补跑，终态未知可只读恢复，后到失败不覆盖已完成。另验20路同键入队、16路执行只发三次回环HTTP、双层预留/单边失败回滚、来源在模型请求中真实撤回、原问题/最终阶段不可变与损坏拒绝、归档读写及零原任务/冻结。迟到用量及时间边界有执行核单测；没有跨午夜真实运行或生产容量验证。

原键重放只恢复本人原问题/状态收据，不授予引文或执行权限；来源撤回/归档不能妨碍确认问题是否已经保存。会员服务沿用原严格Bearer与当前账号服务；内部重放仍严核所有权、原消息/计划一致，执行及答案另核当前会员/范围/来源。

此增量不提供创建批准/提高配额的公共接口，不写生产配置。范围/日政策的启用仅测试随机库夹具，不能复制到线上。受控的真实模型装配与质量批准、用户问答页面及报告追问仍缺；没有新浏览器问答验收，也不沿用此前报告页面截图冒充。任何未确认操作保留原记录/配额和故障现场，不删除后换键绕过。

#### 会员问题接口及无模型历史读取（开发接线，默认关闭）

接口与DTO单一契约见design12.3.3。`TM_AI_QA_READ_ENABLED`、`TM_AI_QA_SUBMIT_ENABLED`沿用原Go布尔解析，默认false；运维统一使用空/false/true，非法布尔值拒启动。读取依赖原任务读取、有效会员JWT和会话开关；提交还依赖读取及可信策略/模型工厂。`cmd/api`的两工厂生产默认均为空，启用提交必须失败启动，不能把测试策略/批准、示例备案或回环模型复制为生产配置。读取状态允许无策略目录；读取答案仍需要按原policy摘要提供无密钥原Config，但不构造Provider、不加载模型凭据或发送模型请求。

启用任一问答能力先执行五表列/默认值/索引/外键/CHECK及15触发器定义检查，再做全部回滚的正反写探针；该启动检查需要新表DML权限，不是只SELECT账号模式，不执行DDL、不产生已提交消息或改账。检查失败前不注册任何问答路由。迁移SQL仍是唯一DDL源，嵌入仅用于比对，不由启动执行。

POST仅题材material_qa，最多25秒请求内执行、3次直接调用；每次模型边界及响应前重验原Bearer/当前账号。登录过期只允许已有调用的有界审计，不再交付答案或继续下一调用。原键重复只查原状态，不构造模型、不再次执行；首次Enqueue回执未知也不补跑。配额不足时仅封存本次尚queued的问题，不终止另一个已经取得执行权的调用、不释放未知预留。HTTP状态200/answered不是当前答案授权；答案GET始终重新校验，来源撤回后拒读而问题收据仍可查。没有问答SSE或新收费任务。

定位接口/结构问题可在上述原生命令中用`-ConversationPattern '^TestIntegrationAIConversationsQA(HTTP|HTTPBoundaries|Schema)$'`，但最终回归应去掉选择器运行完整`-ConversationsOnly`；单测用`go -C server test -race ./internal/ai/qa ./internal/service ./internal/handler ./internal/config ./cmd/api -count=1`。HTTP专项是真实JWT、实际HTTP与MySQL、三次回环供应商请求，不是真实DeepSeek、TradingAgents-CN、H5浏览器或模型语义质量验收。启动测试的示例工厂只提供拒绝型接口，不读取真实模型或授予任何资料批准。

### 会员事件序列（只读轮询，沿用关闭开关）

`GET /api/v1/ai/tasks/{id}/events?after_seq=0&limit=20` 沿用 `TM_AI_TASK_READ_ENABLED` 和同一严格会员 JWT。默认不注册路由，开发放行/Agent 配置均不会开启读取；不启用取消或其他写入，不创建任务、预留、响应或账本流水。读取账号在既有 `addon_member` / `tm_ai_tasks` SELECT 外新增 `tm_ai_task_events` SELECT 即可，实际只读角色已用于验证；不授 DML/DDL/TRIGGER，也不运行迁移。

**2026-10-21 SSE增量（只供隔离开发验收）**：另有`GET /api/v1/ai/tasks/{id}/stream?after_seq=0`，原`/events`仍返回分页JSON。`TM_AI_TASK_STREAM_ENABLED`默认关闭，开启依赖读取开关、有效JWT配置和只SELECT的任务元数据检查；不运行写探针或迁移、不启用模型/报告/扣费。仅标准Bearer，不接Cookie、x-token或查询令牌；可选Last-Event-ID须与显式after_seq相同。身份、归属和首个事件页核验后才开始流，每轮重新验证当前会员与令牌。

`progress`帧的id为原事件seq，data只有`seq/type/status/step/errorCode/at`；保活是注释，不计序号。终态流结束不代表报告当前可读，仍须原任务/报告GET。已完成任务在末断点续读仅建立后立即结束，不重复发历史事件。流中错误为固定`event:error`及公开code，不能展示成模型回答；401须停止并清理身份状态，其他错误/关闭/不支持流时回退原轮询，禁止重发任何写请求。

当前实现每连接最多15秒（服从更短父期限），每次写入最多3秒且不越过连接期限，空页每秒重新核验并查询；单用户最多2条、本实例总计32条，超限42901。此内存限额不覆盖其他实例，不是生产容量承诺。连接结束/取消/写失败释放名额，不开启无界后台goroutine、不长期占用DB事务。H5现已接严格UTF-8、CRLF/分块帧、有界1/2/4秒重连、18秒看门狗及原串行GET降级；完整帧才推进游标，401/403及换账号/离页清理，事件不授报告许可。复验步骤见上节；生产代理缓冲/超时及容量未验，C-3仍不勾完成。

复验：`go test -race ./internal/service ./internal/handler ./internal/config ./internal/repository ./cmd/api -run '^Test(AITaskStream|LoadAITaskStream)' -count=10`；在两版MySQL目录分别运行原生脚本`-TaskEventsOnly -RunCount 1`，新`TestIntegrationAITaskMemberEventsStream`自动进入原选择器。启动装配复用报告启动真库夹具，可用`-ReportsOnly -ReportPattern '^TestIntegrationAIReportSchema' -RunCount 1`，同时执行原报告/创建启动组以及新增stream_off/on；这不是全仓SQL回归。CI另核对SSE根组、RC/RR及启动子例，原守卫不移除。

完整参数/DTO 见 design.md 2.4.3。请求只带单个 Bearer 头及规范编号、游标、页长；显式 `after_seq=0` 有效，缺省游标 0/页长 20，最大页长 100。不传会员编号、URL、模型参数或请求问题。未知/重复/空参数、畸形转义、非规范整数及超当前上界游标为 400；非本人/不存在同为 404。401/503 仍用原读取出口，不泄漏原始驱动信息或令牌。

任务与事件在同一只读 RR 快照读取，单次最多 3 秒并服从更短父期限；按本次 `upperSeq` 限定主键范围，最多 limit+1 条。返回 `items,nextAfterSeq,hasMore,upperSeq`，事件仅 `seq,type,status,step,errorCode,at`。`at` 为真实数据库 UTC 毫秒，先后以序号为准，不假定墙钟永不回拨，不用时间当游标。原文、预算、租约、围栏、供应商异常、报告可用标志与完成百分比不在事件中。

客户端消费成功后才把 `nextAfterSeq` 存为下次游标；空页保持原游标。`hasMore=false` 只代表读完本次上界，运行任务随后仍可产生新事件，不能据此显示完成或停止所有轮询。每次响应固定上界，不承诺多个请求共享一个长期快照。终态和报告是否可见应另查进度/未来报告入口，不能把事件类型 succeeded 当成当前已有可交付报告。

序号缺失、重复/错序、非法类型/状态/错误、步骤倒退、无效时点、末页与任务状态不符或事务失败时整页 503，不返回已读部分。失败保留原游标并有限退避重读，不重置游标掩盖缺失，不手动改/删不可变事件。校验仅覆盖此次有限页，不证明游标前或上界后的全历史健康，也不替代任务/账本审计、真实模型质量或公开报告终检。

原生定位使用 `-TaskEventsOnly`，与其余专项互斥；全套不带专项选项。CI 新增三组、六个 RC/RR 子例与十六个破坏子例必跑守卫，保留所有旧守卫，下限从 243 增至 268。时间/身份测试使用真实 JWT→会员查询→用户隔离 SQL→闭集 DTO，并验证仅 SELECT 角色与驱动/会话 +08:00；快照测试在读任务后由独立事务实际提交取消，既有 RR 页仍只有旧上界，下一轮原游标能读到新事件。损坏反例只在健康基线之后改动随机可丢弃库，不放宽生产保护或扫描基线。生产网关/前端、分页吞吐/限流、SSE、完整 worker、报告/结算仍需独立验收。

2026-10-16 最终验证：冻结代码/测试后的双版本全套通过，新增事件三组/六个隔离子例/十六个损坏反例和原仓储/账本/任务/会员/预算/阶段/抽取/恢复/Agent/GVA 均通过，0 失败/跳过，客户端断言各 38/38；随后双版本各连续三轮通过事件专项，成功隔离实例均停止并移除。事件 service/投影/handler/扫描解码函数单测 100%，二十轮竞态通过，事件 SQL 函数真库定向 84.6%，不等于整个包覆盖率或生产吞吐。有效 IANA 时区与仅 SELECT 角色查询通过，UTC 毫秒无漂移；首轮自定义时区名不能由 DSN 重载的夹具失败已修正，失败证据保留，不计入最终通过。Go/race/vet/build、GVA 关键包与真库、覆盖率棘轮、合规/架构/格式、工作流/脚本语法通过；总门禁仍仅旧工作树疑似密钥扫描与其自测红，基线/用户暂存区未改、远端 CI 未跑。没有真实模型/Agent、真实费用、生产迁移、部署、提交或推送。

### 会员取消开发入口（独立写开关，默认关闭）

契约见 design.md 2.4.2。当前只在获准受控环境显式配置 `TM_AI_TASK_READ_ENABLED=true` 和 `TM_AI_TASK_CANCEL_ENABLED=true` 后注册 `POST /api/v1/ai/tasks/{id}/cancel`；单开读取、付费开发放行或开启 Agent 都不能启用取消。取消要求读取已启用及同一严格 JWT 配置，写开关畸形、身份配置不符或任务/账本联合启动自检失败时 API 退出，不降级开放路由。仍未授权生产部署或开启开关。

新增取消装配所需权限为十四张任务/预算/阶段/账本表 DML 及 `addon_member` SELECT，不需 DDL/TRIGGER，不授来源写权限；这是取消依赖，不替代原 API 其他业务的权限清单。探针全回滚但可能消耗自增编号。共享库迁移由获准流程先行，命令不自动迁移/修复未知旧表。启动后结构漂移仍须运维监测，不宣称一次自检能阻止后续任意约束破坏。

请求只有规范任务编号与单个 Bearer 头，没有用户编号、原因、金额、围栏或幂等键。可无请求体；发 `{}` 时需 `Content-Type: application/json`（可选 UTF-8 charset），最多 64 字节，其他字段、数组/null、尾随 JSON、任意查询参数与压缩编码均拒绝。Cookie、x-token 和查询令牌不作为身份。身份在每次请求实时核对，非本人/不存在同为 404，不通过错误判断他人任务是否存在。

返回 `data` 只有 `id,status,eventSeq,cancelRequested,errorCode,reportAvailable`，不含金额/正文或“远端已停止”承诺。排队任务取消与冻结释放原子提交；运行任务返回 running 与取消请求，冻结保持不变，等待有效 worker 或租约接管终结。终态重放保持既有失败/取消/到期/成功状态，不改写为取消。没有自动清扫器和真实远端取消，因此不能承诺所有运行任务即时释放。

服务事务最多 10 秒且尊重更短父上下文，HTTP 读取仍受既有服务器期限控制，不是进程级硬超时。固定 503 不等于未提交：断连/提交未知时先 GET 原任务，必要时重复同一取消请求受控复核；取消是不可逆请求，重放返回当前状态，不承诺首次响应字节不变。不要新建任务代替取消、换操作键、直接改冻结/预算或让客户端自动无限重试。错误详情不回传/记录到新增取消出口；响应 no-store，不新增跨域放行。公开入口的网关限流、生产容量、令牌/会员撤回竞争、线上前端与授权仍须独立验收。

页面现有独立取消组件与请求封装：仅在原任务排队/运行且未登记取消时提供“申请取消任务”，先就地二次确认，再提交一次。确认/提交暂停同详情轮询，取消响应不覆盖进度DTO、不授予报告许可；“核对原任务状态”仍调用原GET，未知不自动重发。运行回执明确不表示远端停止或积分释放，成功竞争不改写成已取消；切任务/账号/离页后丢弃迟到响应，401清私有显示。取消仍受上述独立服务端开关约束，UI存在不意味着接口已开放。源码回归沿用`verify-ai-report-ui.cjs`，同时执行实际嵌套页面/组件的确认、轮询、未知、401及身份切换用例；不是缺失TMUI依赖时的真实HBuilderX/浏览器验收。

原生脚本 `-TaskCancelOnly` 只跑会员取消三组/六个 RC-RR 子例与 `cmd/api` 启动装配一组/六个子例，和其他专项互斥；全套回归不带专项选项。CI 增加上述细分必跑守卫，原有守卫不减，最低执行数由 227 增至 243。可选 `-APICoveragePath <独立.out绝对路径>` 仅收集取消启动装配所在命令包的语句计数；未运行该套件的专项模式不产生该文件，不能与仓储覆盖输出共用路径。测试只在随机隔离库中操作，真实会员 JWT、回环 HTTP、40 并发/worker 竞争、触发器故障回滚和已提交但回执未知均不调用模型；启动装配测试不宣称已运行完整 API 监听/线上网关。

2026-10-16 最终验证：代码/测试冻结后的双版本全套通过，新增取消三组/六个隔离子例和启动一组/六子例通过，原仓储/账本/任务/会员/预算/阶段/抽取/恢复/Agent/GVA 均通过，0 失败/跳过，客户端断言各 38/38；随后相同代码双版本各连续三轮通过取消专项。成功隔离实例停止并移除，真实已提交但回执未知时 HTTP 返回固定 503，原任务复核不重复释放，故障触发器引发的取消整体回滚。取消 service/handler/开关函数单测 100%，二十轮竞态通过；启动装配函数真库定向 80%，不冒充整个命令包覆盖率。Go/race/vet/build、GVA 关键包、覆盖率棘轮、合规/架构/格式和工作流/脚本语法通过；总门禁仅旧工作树疑似密钥扫描及其自测仍红，基线未改，远端 CI 未跑。初次脚本账户断言与过短到期夹具失败保留证据，不计入最终通过。未运行真实模型/Agent、真实扣费、生产 DDL 或部署；线上访问、网关限流/容量、完整 worker、报告与成功结算仍未验收。

### 任务执行预算预留（不运行模型）

先执行积分、任务迁移，再执行 `20261015_create_tm_ai_task_budgets.sql`，然后更新服务及 `cmd/ai-task-check`。联合自检现在要求预算两表存在，并在同一回滚事务核对元数据及实际拒写；旧版命令的通过结果不能代替新增预算自检。业务账号无需 TRIGGER/DDL 权限，但探针仍需相关表的 SELECT/INSERT/UPDATE/DELETE，并可能消耗账本探针自增编号。迁移不是未知旧开发表的自动修复器；生产更新和权限尚未演练，不自行执行生产迁移。

`AITaskBudgetService.Reserve` 是内部代码能力，没有公开路由或启动命令；不对接 TradingAgents-CN、不读模型密钥、不冻结/释放/扣减积分。完整契约见 design.md 4.2.1 与 ADR-0011。调用前必须由可信执行计划提供输入摘要和可强制的资源上界，且核对有效租约；仅本地预留成功不能证明远端 Agent 已受控。真实用量未知时继续未知，不能用预留冒充实际消耗或追加收费。

收据 `NewlyReserved=false` 表示此前已保守预留，同一操作不得再次执行网络请求；即使租约接管或服务进程重启也不清零。提交未知结果只按原键核查，不自动补交/退款。预算不足只拒绝预留，未来 worker 须显式结束任务并沿既有终态事务释放冻结；这条自动执行链尚未实现，不能只看预留代码便开通收费。不要手工 UPDATE/DELETE 预算、明细或改任务固化限额来“解锁”重试。

原生隔离脚本新增 `-TaskBudgetOnly` 用于快速定位，不能与 `-AgentsOnly` 或 `-TaskStagesOnly` 同用；正式本地回归仍不带任何专项选项。预算专项运行五组：生命周期、同任务 40 并发、40 独立任务连续两轮、真实提交未知/父 RR 旧快照/触发器故障回滚、15 种保护破坏。前三组分别跑 RC/RR，CI 单独要求五组/六个隔离子例/十五个破坏子例齐全；这不是远端 CI 已通过的证明。预算增量时 mysql 客户端迁移断言为 34，额外检查预算两表、两个 CHECK 和七个触发器；阶段增量后的当前断言数见后文。

回归曾测出 RR 初始化时先锁不存在的预算行导致不同任务互相死锁，现用直接 INSERT、只识别明确 1062 后再当前读；新建预算由复合 FK 证明没有历史明细，不锁空明细区间，已存在预算计数为零时仍核对明细。独立任务并发用例已固定为必跑反例，不用自动重试掩盖初始化问题。初轮唯一键探针误用经过转换的业务错误，也已改成取原始 SQL 错误编号；失败实例均停止并保留日志/数据，不计入最终通过记录。

2026-10-15 最终预算验证：双版本各完整通过一遍原生脚本，预算五组/六个隔离级别子例/十五个破坏子例及原账本、任务、会员只读、Agent、GVA 套件通过，0 跳过；客户端迁移断言各 34/34，成功实例已停止并移除。预算领域单测 100%、Reserve 98.4%；SQL 初始化和插预留 100%、预算元数据 90.2%、拒写探针 86%，均为定向函数覆盖率，不声称整个仓储包全量覆盖。Go/race/vet/build、架构/格式/合规及工作流语法通过；总门禁仍被旧工作树疑似密钥扫描和扫描器自测阻挡，未放宽基线。未提交/推送/部署，无真实模型调用、收费或生产迁移；此结果不替代 worker、真实成本、阶段结果/报告/结算及生产容量验收。

2026-10-15 最终专项：双版本六组各连续三遍通过（含实际构建命令对回环模拟 API 的提交、重放、取消、状态、隔离链路）；命令插桩覆盖率两版均 78.7%。此前全套真库回归双版本各一遍通过；客户端迁移断言各 30/30，成功隔离实例均已停止并移除。全量门禁只在既有密钥扫描及其自测失败，未放宽基线，未提交/推送/部署。原始失败证据保留，不计入通过次数；完整限制与待办见 tasks.md 第 10 节。

### 隔离阶段结果与检查点（不运行模型、不生成报告）

迁移顺序为账本→任务→预算→`20261015_create_tm_ai_task_steps.sql`→更新服务/联合自检命令。新增 `tm_ai_task_step_results` 与 `tm_ai_task_steps`，七个新表触发器及两个原任务表触发器；新任务必须从步骤 0 开始，推进必须有绑定的检查点。DDL 重建保护期间仍需停止写入，不能把 `IF NOT EXISTS` 当成未知旧表的自动升级。最新 `ai-task-check` 要求十四张任务/预算/阶段/账本表，并在一个回滚事务中验证元数据与真实拒写；测试受限账号有相关表的 SELECT/INSERT/UPDATE/DELETE，没有 DDL/TRIGGER 权限。生产迁移和生产权限未验，不自行执行。

内部服务 `AITaskStageService` 只接受服务端固定策略（摘要+纯校验函数），没有 HTTP 或启动入口。调用流程见 design.md 4.2.2、ADR-0012：

1. `Capture` 在原预留的有效 owner/fence 下保存脱敏响应；严格 JSON 对象、UTF-8、大小/深度限制与摘要只证明隔离完整性，不证明事实/授权/合规通过。此时不推进进度。
2. `Complete` 事务外执行纯校验，校验函数必须尊重上下文、不能网络调用或产生后果性动作；最终事务重新核对当前租约/取消/期限、输入、响应和策略，检查点/任务步骤/白名单进度事件原子提交。校验细节不会作为错误向外返回。
3. `Load` 仅供当前租约的内部 worker 复用已完成且绑定仍一致的检查点；策略摘要变更拒绝复用。新租约可校验旧租约已捕获的响应，不能伪造补捕获。只有预留却无响应时仍未知，不重发外部调用；提交不明按原操作键核查，不能换键或清除预留。

正文不进入公开 DTO、进度或报告；阶段完成后任务仍 running、报告不可用，积分冻结不变。模板终检、当前来源授权核查和成功结算不能被内部检查点代替。测试策略只针对明确的测试载荷，绝不证明真实事实核验质量，也没有默认放行策略接入生产。

`-TaskStagesOnly` 仅用于定位，与其他专项选项互斥；完整本地回归不带专项选项。检查点存储的 CI 必跑六组、六个 RC/RR 子例、三个恢复/回滚子例、三个校验边界子例及十八个保护破坏子例；抽取纯策略及恢复命令新增守卫见后文，远端 CI 仍需另跑。客户端迁移断言为 38，包含阶段两表、两个 CHECK、七个新表触发器，原任务表触发器现为六个。完整 worker 调度/全部事务边界进程恢复、其余阶段策略、真实多 Agent 硬预算/来源控制、用量审计和报告/结算仍是开放项。

2026-10-16 验证：最终代码双版本各完整通过一遍原生脚本，阶段上述六组及细分子例、原仓储/账本/任务/会员只读/预算/Agent/GVA 全套通过，0 跳过；受限账号及实际构建 `ai-task-check` 正反例通过，迁移断言各 38/38，成功临时实例已停止并清理。阶段服务函数单测 93.6%–100%、任务领域包 97.1%；真实 SQL 插响应/检查点 100%、阶段元数据 82.1%、拒写探针 85.7%，不声称整个包已全量覆盖。Go/race/vet/build、覆盖率棘轮、架构/格式/合规及工作流/脚本语法通过；总门禁仍仅既有旧工作树疑似密钥扫描及扫描器自测阻挡，基线未放宽。无真实模型/Agent 调用、扣积分、生产迁移、部署、提交或推送；本轮没有将数据库事务回滚测试冒充独立进程恢复测试。

### 抽取阶段确定性校验策略（不运行模型）

可信调用方先经 `SnapshotForExtraction` 取得当前启用/授权副本仍匹配的快照，再按任务主体提供至多 128 个题材的词汇，调用 `pipeline.NewExtractionStage(snapshot, vocabulary)`。用其 `InputDigest()` 绑定预算预留和阶段请求，用 `Policy()` 只注册 `extract` 策略；不能把用户自报摘要、任意 URL 或上游研究报告作为这些可信输入。此工厂无 I/O，不自动读取授权登记或主数据；恢复型 worker 已在计划构造及各步前后核查来源/题材，仍不替代未来执行与报告入口的授权核验。

纯策略的容量、来源类别/谓词绑定、整体拒绝条件和摘要细则见 design.md 3.2.6。它比 L0 的逐条筛除严格：空候选、任一条失败、重复或需要节选均不保存完成检查点；保留隔离响应和预留，不清零、不重发、不扣分。其他阶段未配置策略即拒绝，不能补一个“返回 nil”的默认校验器。需要节选的事实仍可按既有 L0 人审路线处理，不能改这个任务的原始响应或借 L0 绕过公开验收。

`guard.AlgorithmVersion` 在确定性实现语义改变时必须递增，词表与句式内容自动参与 `PolicyDigest`。改算法/规则/别名后旧检查点拒绝复用，不改旧记录、不要通过重贴旧摘要放行。工厂复制输入，别名排序去重；相同 UTC 时点、map 或别名顺序不会造成不必要漂移。模型未返回自由引文，引文仍由代码按来源编号取得；共现不是蕴含或主数据身份充分证明。验证通过仍是内部 `extract` 阶段，不改变报告可用性、会员接口或结算。

新增 `TestIntegrationAITaskStageExtractionPolicy/Rejections` 两组，每组跑 RC/RR，拒绝组每种隔离级别十个反例。CI 要求两组、四个隔离子例、二十个拒绝子例齐全；恢复型 worker 增量后的全套下限见后文。`internal/ai/pipeline` 覆盖率下限登记为 80%，守卫包仍为 95%。带违规措辞的固定反例只放 `testdata/*.jsonl`，只由测试二进制/集成测试读取，不随产品嵌入或发布；门禁与词表不为测试放宽。真实模型评测、主数据解析/授权撤回竞争、语义二道防线、多来源裁判、完整 worker 和报告/成功结算仍未验收。

2026-10-16 最终策略验证：代码/测试夹具冻结后，双版本各重新完整通过一遍原生脚本，新增两组及四/二十个细分子例、原仓储/账本/任务/会员只读/预算/阶段/Agent/GVA 全套通过，0 跳过，客户端断言各 38/38；成功隔离实例已停止并移除。相同策略重新装配可复用，别名/规则摘要变化拒绝旧检查点，拒绝保留隔离结果/预留但不推进/结算，JSON 序列化不包含正文。二十轮竞态单测通过；工厂函数 91.7%、词汇复制及规则摘要 100%、流水线包 83.3%，不等于模型准确率。全量 Go/race/vet/build、覆盖率棘轮、架构/格式/合规及工作流语法通过，远端 CI 未跑；总门禁仍只有旧工作树疑似密钥扫描和自测红，未扩大基线。首轮违规反例字面量与测试改动期间的中间运行不计验收，最终已独立重跑；本轮没有真实 Agent/模型调用、扣费、迁移生产、部署、提交或推送，也没有新增生产 DDL。

### 单任务恢复命令（已有隔离响应，不执行模型）

取舍见 ADR-0013 与 design.md 4.2.3。获准内部人员核对已有会员、任务及原操作键后，在 `server/` 执行：

```bash
go run ./cmd/ai-worker recover -user <已有会员编号> -task <已有任务编号> -plan <可信本地计划文件>
```

自动化应使用编译后的二进制以保留退出码，但当前禁止配置成生产周期任务。`TM_DB_DSN` 只从凭据管理注入，不能放在命令行、计划文件或日志。执行账号需十四张账本/任务/预算/阶段表的 SELECT/INSERT/UPDATE/DELETE，以及 `tm_ai_sources`、`tm_ai_source_bodies`、`tm_ai_source_snapshots`、`addon_quant_theme` 的 SELECT；不授予 CREATE/ALTER/TRIGGER 或来源写权限。不要沿用测试 root。联合自检在接管前执行，缺少保护、权限不足或超时均退出；不执行生产迁移。

计划是最多 32 KiB 的严格 JSON 对象，1..64 步；以下仅展示形状，编号与键均为示例，不能直接用于真实任务：

```json
{"steps":[{"snapshot_id":1,"operation_key":"example_existing_operation_key"}]}
```

只接受这两个精确字段，顺序即步骤号；重复键、重复操作键、未知字段/大小写变体、任意 URL/正文/摘要/模型参数全部拒绝。当前只恢复题材的 `extract`，不猜测专题及其他阶段。服务端按任务固化主体读取当前题材与当前授权有效的来源快照，重建策略；输入、策略或已有前缀不一致拒绝，不修改历史记录以适配新规则。每步前后重验来源，不声称已封闭撤回与提交间的全部竞争；公开报告终检仍须独立核查。

命令可接管无活跃租约的已有任务，独立心跳不依赖校验返回。默认租约 60 秒、发起间隔 20 秒，单次心跳 SQL 最多租约三分之一并服从父上下文，整轮恢复最多 5 分钟；本地停止会等待心跳退出。上下文不是包含阻塞标准输出在内的操作系统硬超时。退出后不继续续租，也不自动重跑；再次接管消耗原尝试次数，不能无限循环调用“解锁”。核心对已取消/耗尽任务可跳过来源计划，但 CLI 仍要求格式有效的计划、自检及有效终态事务。

退出与后续处理：

- **0**：`report_pending` 或既有终态。前者仍 running、冻结不变、报告不可用，不是成功结算；后者须看元数据状态，不能把已失败/取消当成功。标准输出只含任务编号、状态、步骤数、封闭 outcome 与 `report_available=false`。
- **2**：`response_unknown`，缺响应或原预留未能定位。保留任务/冻结/已有预留；仅按原任务/操作键受控核查，不重发、不换键、不自动退款。
- **1**：配置、来源、租约、数据库、自检或输出失败，详情隐藏。保存现场并由获准人员核查，不直接改/删预算、响应、检查点或冻结单。提交可能已成功，须在租约允许时按原键复核。
- **130**：中断。停止本地心跳，不等于业务取消或远端取消；未实现实际远端执行，不能借此宣称已撤销模型任务。

已确认用户取消或确定性抽取拒绝沿原任务失败/释放事务终结；来源变更、进程死亡、提交未知或心跳故障不冒充业务失败退款。此命令不创建任务、预留或响应，不采集、不调用 Provider/TradingAgents-CN、不生成报告或成功结算，也不改变 L0 人审路径与公开 API。

本地定位可用原生脚本 `-TaskRecoveryOnly`，与 Agents/预算/阶段专项互斥；全套回归不带专项选项。CI 必跑恢复三组、生命周期两个隔离子例/十六场景、两个杀进程子例、四个实际命令子例，整体下限由 200 增至 227；`cmd/ai-worker` 单测覆盖率棘轮为 85%。进程测试只在测试二进制中阻塞纯校验，超过租约仍由独立心跳维持占有，实际杀死后等待租约过期、新围栏恢复；没有产品故障注入/伪成功开关。

剩余范围：真实远端硬预算/来源工具控制与用量、全队列执行调度/公平性/自动清扫上线、其余阶段、语义核验/第二道防线、报告/成功结算、全部事务边界进程恢复、生产权限/容量及上线确认。有限隔离恢复不等于阶段 3 或 AG-4 至 AG-6 全量完成；独立显式清扫的边界见下文。

### 过期任务显式限量清扫（会修改冻结账务，不执行模型）

依 ADR-0014 和 design.md 4.2.4，获准内部人员确认目标库与任务到期口径后，在 `server/` 执行：

```bash
go run ./cmd/ai-task-expire -after 0 -page-size 100 -max-tasks 1000 -timeout 30s
```

生产尚未授权启用，不配置为周期任务，不使用真实生产 DSN 开发验证。自动化将来应调用编译二进制以保留退出码；DSN 仅由 `TM_DB_DSN` 注入，不打印、不放命令行。账号仅需十四张依赖表的 SELECT/INSERT/UPDATE/DELETE，供联合回滚自检和任务/账务终态；不需 DDL/TRIGGER、来源表或会员表权限。缺保护、自检失败/超时即不开始清扫，无自动迁移。

默认最多1000个任务、每页100行、清扫30秒；可配置边界是 page-size 1..1000、max-tasks 1..100000、timeout 1秒..5分钟、after≥0。联合启动自检另受最多20秒期限；标准输出阻塞不受数据库上下文硬控制，监督器须提供操作系统级退出上限，但当前未配置生产监督器。

JSON `sweep` 含真实数据库 `as_of` 毫秒、upper_id/last_id、visited、expired_observed、skipped、complete；没有 Agent 正文、模型调用或金额统计。扫描所有状态均计入 visited；expired_observed 是本次正常复核到过期终态的数量，包含竞争者先终结，不等于本进程实际写入次数。既有终态和截止时点未到期均跳过；当前数据库时钟回拨导致未到期也不提前释放。

- **0**：本轮可见上界读完，不是全库无过期任务或报告成功的承诺。
- **2**：正常达到数量上限。可用返回的 last_id 续扫；下次重新读取上界/截止时点。
- **1**：配置、自检、数据库、坏页、提交未知、清扫超时或输出失败。此前任务可能已提交，失败项可能也已提交；last_id 只越过已确认项，按原任务重新核查或重扫，不补扣、不换键、不重发。
- **130**：本地中断，不是业务取消或远端停止证明。没有可靠 JSON 时从零重扫；终态幂等保护避免重复释放。

新轮必须从 after=0 开始。分页不是同一个长快照，较小自增编号晚提交、范围后新增、扫描中刚到期、时钟回拨的任务均可能留到下一轮；禁止永久递增游标。任务/截止不可变，但主键游标不意味着严格的提交时间水位。累计历史扫描成本、生产积压吞吐与公平性未验收，不依据此次小库 EXPLAIN 或40并发测试承诺生产容量。

清扫仅走原 Expire 事务，释放的是已到期任务冻结，不执行积分批次到期扣减。未知调用的预算预留、隔离响应与检查点原样保留，既不清预算也不取消远端；旧租约不能晚到推进。没有创建、采集、模型/TradingAgents-CN、报告、成功结算或公开接口接线。

本地专项为原生脚本 `-TaskExpiryOnly`，与其他专项互斥；完整回归不带专项。CI 要求清扫四组、六个 RC/RR 子例、四个原子性子例及四个编译命令子例，下限由268增至286；`cmd/ai-task-expire` 覆盖率棘轮85%。命令的已取消父进程子例仅证明未启动，不冒充运行中杀进程或信号恢复验证。

### 任务-Agent 接线核（无生产启动入口）

本节依 design.md 4.2.5 / ADR-0015。`AITaskAgentWorker.Run` 已把现有任务、独立心跳、单项预算、V2执行登记/轮询/结果隔离和任务回执检查点组合起来；没有新增可直接运行的生产命令或公开 API，也未启动已准备的上游服务。选项默认关闭，缺真实服务/许可/模型/预算配置时保持关闭，不把测试规划器挪到产品，也不能仅切换 `TM_AI_AGENTS_ENABLED` 当作已满足新准入。

将来装配时必须先实现可信 `agenttask.Planner`：按固化主体取得有依据的股票映射，核对当前来源/许可、固定模型及远端可强制成本/工具上界；独立内部操作人不能来自会员请求。Plan/Recheck 接口本身不是这些证明的实现。上游完整 API 仍受原许可边界约束，本地依赖已装不等于获得商业授权；不得启动真实付费调用来替代授权确认。

只支持整个任务的第一步 `agent_call` 与单项预留。固定请求键/预留键无需人工生成；更换主体/股票/操作人/配置/资源会冲突，不能清表、换键或改旧结果。不得将已有抽取/采集任务交给此核：预算事务会拒绝混合阶段。当前不是完整流水线或全队列调度器。

结果处理：`evidence_pending` 只表示上游结果已隔离且四字段回执身份通过内部检查点；没有事实报告，不扣积分。`response_unknown` 保留原键核查，不提交第二次。`capture_pending` 表示远端已隔离但旧租约未捕获任务回执，不补造旧围栏响应；可用原 Agent 工具查元数据，由获准人员核对，不能认为已退款或成功。`upstream_terminal` 不是业务成功/报告可用；原取消、过期清理与独立远端取消按既有手册处理。错误/取消/超时停止本地心跳，不自动终结、远端取消或补扣。

本地测试：原生脚本 `-TaskAgentBridgeOnly`（与其他专项互斥），全套不带专项。测试使用真实 V2 HTTP 适配器连接只在回环监听的明确测试服务，再走真实 MySQL；不是实装 TradingAgents-CN 服务或真实 LLM 联调。CI 要求四组、八个 RC/RR 子例、八个恢复子例、十个边界子例，下限316；包含40并发、慢提交超过初始租约仍由独立心跳维持、网络期间独立任务锁可取得、事件失败回滚及真实已提交回执未知。`internal/ai/agenttask` 覆盖率棘轮95%。

2026-10-16最终冻结版本：双版本各完整回归通过、迁移各38/38、35个实际CI表达式核对通过，成功实例停止并移除；二十轮竞态单测和覆盖率棘轮、Go/race/vet/build及工作流语法通过。总门禁仍仅既有46处疑似密钥/扫描器自测红，未放宽基线；远端CI、真实服务/模型、生产准入与报告/结算未验收。细分证据与早期夹具修正见 tasks.md「任务-Agent 接线开发」，不能将测试规划器、上游默认token数或回执检查点当生产授权/成本/报告证明。

### 报告终检与成功结算（仅内部开发核）

当前实现见 design.md 4.2.6、ADR-0016。`AIReportService` 默认关闭，只有测试显式装配；没有公开路由、新执行命令或生产纯校验器，不因选用社区版或开启 Agent 开关而自动生效。会员报告仍不可用。不得把测试中的 `pass` 回调复制成产品策略，不接受上游原文或四字段执行回执作为报告。

将来接入时，先明确正式最小可交付政策，构造绑定原任务/题材和授权快照的规范封闭引文档案，另行受控登记预算、捕获/校验 `judge` 阶段；当前只接题材L1四个固定章节，引文重复、自由句、公司行、行情、L2/专题等都拒绝。维持当前租约/心跳，逐一核对全部预留与检查点。未完成/未知预留保留现场，不换任务/预留键，不借报告服务补造缺失响应。

真实入口必须先运行 `AIReportSchemaVerifier.Verify` 的联合回滚探针；它不执行迁移、DDL或历史修复。内部账号需十五张原账本/任务/预算/阶段及 `tm_ai_report_commits` 表的SELECT/INSERT/UPDATE/DELETE，来源三表及题材表仅SELECT；在已验两版中共享锁无需给来源写权限。该自检不是来源许可、语义质量、历史账务或全部结构破坏的证明。新增迁移先在隔离库执行，不能直接用于已有未知形态的生产表。

终检将结构与两道纯判定置于事务外，最终当前复核来源/题材、租约、取消、期限及阶段后，原锁价积分终结、任务/事件、终检收据一起提交。成功收据才带内部报告正文；拒绝只留元数据并全额释放。取消优先处理。回执未知时不要重建任务或补扣：用原任务和原judge键重放，核对原收据/任务/冻结/终结流水；无确认就不宣称成功。数据库/父上下文失败可保持冻结，仍由原取消/到期路径治理，不据此声称远端调用停止。

定位用 `verify-ai-source-native.ps1 -ReportsOnly`（与其他专项互斥）；正式完整回归不带专项。测试全部使用显著标注的示例来源/模型和纯判定，不启动 TradingAgents-CN 或真实模型。六个根组、十二个隔离子例、二十个边界、八个原子性、十四个自检和六个恢复子例由CI逐组断言；总下限382。客户端迁移断言新增报告表/CHECK/三个触发器，总数41。原恢复型worker的杀进程证据不冒充本支线的进程崩溃证明，真实自动转换、校验评测与公开报告仍待实现。

### 固定L1统一编排（开发接线，真实调用关闭）

依 ADR-0019，内部 `AIL1ReportWorker.Run` 串起新Agent执行、隔离线索读取、来源编号选择、纯回取、蕴含/语义、本地候选和原子终检。没有新增命令、公众提交接口或自动调度，不能切环境开关启动真实模型；生产规划器与选择/判定准入审计控制器尚无实现。只支持保守L1四节引文档案；会员报告读取与只读页面接线见下节，缺真实发布控制器仍不可公开读取。

定位仍用 `verify-ai-source-native.ps1 -ReportsOnly`，包括 `TestIntegrationAIReportWorkflow*` 四组；测试中的CN/模型均为回环模拟服务，来源显著标为示例。工作流关闭或缺依赖时不访问网络；所有配置引用必须由当前证明核验，测试控制器不能移入产品。蕴含拒绝不调用语义，畸形编号不产生报告；保存/成功/积分结算复用原事务。未知预留/捕获/最终提交按原任务复核，不清表、不重发、不重扣。原文只能留隔离区，不允许通过返回值或日志输出。

最终提交的后台心跳已先停止并等待退出，再续足一分钟租约进入有限终检；主进程中断不等于远端停止。已有旧恢复核杀进程测试不能冒充新统一编排的独立进程验收。生产最低交付、模型质量/硬预算和来源/许可证明仍是外部关口，本地模拟不代替。

### 会员报告读取与页面（开发接线，公众读取关闭）

依ADR-0020和design.md 2.4.4，报告开关 `TM_AI_REPORT_READ_ENABLED` 默认false，且必须同时启用任务读取与有效会员JWT。当前main没有生产发布控制器/策略工厂，误设为true会在注册前启动失败；不能通过开发付费放行绕过。默认关闭不注册路由，现有任务查询不变。禁止把测试发布控制器或模拟模型编号当正式证明。

报告GET使用原任务编号，拒绝查询参数/正文；Bearer身份和实时会员状态仍逐次核对。读取两次复核完整终检/结算链和当前来源授权，两次发布控制核验在锁外；不调用模型、不创建事件/积分流水。已撤回/未交付统一不给正文；未知提交/存储错误固定503，不能取旧缓存补显示。该读核会短暂锁账户/任务并共享锁来源，权限与吞吐不同于仅SELECT的普通历史查询；上线容量另验。

本地定位使用 `verify-ai-source-native.ps1 -ReportsOnly`：报告读取四根组、八个隔离子例及三十二个边界子例，加API启动一组/八子例（含缺准入表及标记保护）；完整回归不带专项。模拟控制器、回环模型及example.test示例来源仅存在测试；反复读取不增加调用/账务/事件，来源撤回后立即拒绝下一次读取。它不替代真实网络网关/外部服务联调。

页面复用现有登录 `token`，不是历史占位 `tm_member_token`；不需要将令牌复制到另一个存储键。开发H5仍由已有 `/api/v1` 代理转发至本地API，不改为生产地址。页面可查本人历史、串行轮询进度、点击报告与确认原文来源；报价与创建交互见下节，真实生成仍未开放。无数据/无权限时如实显示，不导入测试报告到产品页面。

代码级复验：在仓库外隔离安装固定vue/compiler-sfc/server-renderer 3.5.13与typescript 5.7.2，再执行 `node scripts/dev/verify-ai-report-ui.cjs <依赖目录>`。脚本直接执行实际UTS和UVue源码，覆盖DTO/HTTP拒绝、登录令牌、分页、轮询、失效/乱序/跨账号响应、离页清理及文本转义；模拟uni接口和原生标签宿主，不是HBuilderX编译或真实浏览器验收。实际HBuilderX编译、布局和现有站点会员联调须单独取得证据。

本机页面依赖已于2026-10-20补齐：用户提供根目录`uni_modules.zip`，SHA256为`BBD773E0E94735F4DFF756F03C91BB1B845B7A27091A350B4C3258255196AF73`。先核对目录/符号链接/重名及覆盖风险，再仅提取uni_modules内1256个文件，跳过Mac元数据，原包保留；TMUI为1.1.22、配套x-vuei18n-s为1.0.1。依赖仍按原Git忽略规则保留，不提交第三方组件；安装资源边界参考[TMUI4x安装](https://xui.tmui.design/01.%E6%95%99%E7%A8%8B/10.%E5%85%A5%E9%97%A8%E6%8C%87%E5%8D%97/12.%E5%AE%89%E8%A3%85.html)。HBuilderX5.26.2026091802已重新核签，官方编译器及Sass插件下载后，本地开发模式可加载AI页；可空属性生成问题已修正。游客状态、重新核对、前往登录/返回及工具类/图标/长按钮已有实际浏览器证据；随后隔离真库下的会员创建/报告/取消实测见下一节，不等于生产登录验收。375px及整站回归仍未验收。发行命令提示需登录，即使CLI退出0也不算发行构建成功；未代登录、上传、部署或开启真实执行。源码脚本仍仅证明自己的模拟宿主覆盖范围。

### 本地实际H5与隔离真库（仅测试，不开放产品开关）

`server/integration/points/report_h5_integration_test.go`仅在integration构建下可用。测试复用原API/服务/SQL与原L1编排，在`httptest.Server`的回环地址上提供同源H5代理和显著标注的测试控制页；没有生产登录、公开监听或main注册。控制页签发的示例会员令牌只供本地夹具，禁止复制到聊天、日志或生产配置。CN/模型/批准均是原隔离测试服务，不是已部署TradingAgents-CN或真实推理。

普通自动化复验不设置`TM_TEST_AI_H5_ORIGIN`，运行既有原生脚本的`-ReportsOnly -ReportPattern '^TestIntegrationAIReportH5Gateway$'`，分别指定8版、9版MySQL目录。它覆盖RC/RR下四节引文成功、排队取消、资料库公司、创建提交回执未知和运行中取消五种模式，共十个叶子例，另验跨会员404、来源撤回422及原键重放不再执行/改账；前端仅代理显著标注的占位响应，不能作为实际H5截图证据。公司/资料库夹具改动的关联回归使用`^TestIntegrationAIReport(H5Gateway|Companies|Library|MemberSubmission)`，运行中取消/worker准备改动使用`^TestIntegrationAIReport(H5Gateway|MemberSubmission|Workflow)`，两者都不是全部报告组。纯回环源边界测试`TestH5TestOrigin`需另跑，远端CI集成命令则两组都会执行。

人工页面复验须先启动本地HBuilderX AI页，确认实际端口。在新开的一次性PowerShell会话中执行下例，不写项目.env或持久环境，不传入生产DSN；9版目录为`D:\mysql9\mysql-9.7.0-winx64`，RC/RR可独立选择。四节引文、排队取消、资料库、未知回执及运行中取消的叶子例分别为`success`、`cancel`、`library`、`receipt_unknown`及`running_cancel`，每次只选择一个叶子例：

```powershell
$env:GOMAXPROCS = '2'
$env:GOFLAGS = '-p=1'
$env:TM_TEST_AI_H5_ORIGIN = 'http://127.0.0.1:5174'
try {
    .\scripts\dev\verify-ai-source-native.ps1 `
        -MySqlBaseDir 'D:\mysql\mysql-8.0.46-winx64' `
        -RunCount 1 -BashPath 'D:\Git\bin\bash.exe' `
        -ReportsOnly `
        -ReportPattern '^TestIntegrationAIReportH5Gateway$/^Read_Committed$/^library$'
} finally {
    Remove-Item Env:\TM_TEST_AI_H5_ORIGIN -ErrorAction SilentlyContinue
}
```

只允许HTTP字面回环IP及显式有效端口，不接受localhost、远端域名、凭据、其它路径/参数；网关入口URL由测试日志给出，禁止替换成生产站点。浏览器进入`/__test__/`选择测试会员，按实际页面搜索、报价、二次确认创建，再按所选模式查看报告/取消；成功场景另开同一源控制页撤回来源，再次读报告应拒绝并移除正文，切换会员应清除私有状态。取消场景可先保留任务，再确认并独立查询终态；回执不等于退款或远端停止承诺。最后点击“结束并核对测试”，只有Go确认任务终态、冻结归零、调用次数和积分审计且退出0才算该人工场景完成。人工会话最多10分钟，未结束即失败；这不改变自动化120秒、任务300秒预算或脚本20分钟看门狗。成功实例由脚本停止清理，失败实例停止并保留现场。

资料库模式在注册HTTP服务与首次创建前准备已审核全集、事实摘要及原模型/政策，页面报告GET从一开始就绑定该公司政策，不创建隐藏的第二个报告任务。创建确认后应显示前三节AI引文，以及标明人工审核、代码排序、无行情数据的完整公司节；逐条有归属、原文节选、出处与采集时点，不出现审核人、内部摘要或行情0值。

选好测试会员后，可手动进入同一回环源的`/__test__/mobile`：该测试页嵌入宽375、高640像素的真实H5布局视口，顶部明确标注“非移动设备验收”，不修改产品CSS或引入页面模拟数据。需从目录、报价、二次确认走到报告/取消，并检查实际文本换行与滚动。截图是外层浏览器1280×720加375px iframe，不可称为物理手机、移动UA/触摸键盘或iOS安全区验证；父页DOM工具不暴露子文档时不可据此编造整页scrollWidth测量。

`receipt_unknown`模式把原创建SQL事务真实提交后，仅由测试存储层返回未知；故障注入器绑定浏览器实际生成的原键，不改请求头或换键。页面应保留原请求，重载后仍只能显式核查；按原键GET找回同一个任务，再查看与取消并独立确认终态。人工会话结束时断言创建POST恰好一次、CN/模型均0次、报告/成功结算均0、冻结归零且账本审计通过；自动化有一次显式同键POST重放，单独期望总数2，不借此允许页面自动重发。

`running_cancel`模式由原L1 worker真实提交测试CN并轮询，持久Agent记录确认Running后才算进入执行，不能只Claim或手工改状态。创建后在原30秒worker执行窗口内连续完成查看运行态→二次确认取消→核查原任务；取消回执只能说明已登记，独立查询才确认本地取消终态，页面必须保留“不代表远端停止”的说明且不提供报告读取。测试结束还须原worker无错误退出、CN提交1次、模型0次、无报告/成功结算、冻结归零及审计通过；远端记录仍Running，不调用远端停止接口，不把它改成已取消。浏览器多轮观察若耗尽执行窗口，此轮失败并保留实例，不能拉长worker超时让它通过；可按相同上限将已确定的UI动作连续执行。控制页“测试操作已确认”只是控制请求回执，不能当作Go整轮通过。

2026-10-20同冻结`106090457A19F341112E13E2477AA5CC0DA7465F72070442E149208B3AF0996E`下，8/9版H5/会员提交/Workflow关联回归各124条PASS、0失败/跳过、迁移44/44、退出0并清理成功实例；合并单独纯源测试，各17个对应CI表达式匹配，并非105项或全仓SQL。9版RR实际桌面H5运行中取消也已退出0，21条PASS、迁移44/44，创建恰好一次、CN/模型1/0、无报告/成功结算且冻结归零。截图`tm-stock-ai-h5-running-cancel-final-20261020.png`；日志与两次30秒窗口失败记录见tasks.md同日条目，失败实例均停止保留。同冻结完整harness含覆盖率退出0，Go/race/vet/build、13包覆盖率下限及安全/架构/GVA门禁通过，race未降级、Go总语句覆盖70.5%。前端七文件BB608…冻结与原暂存对象不变；这些测试不证明物理手机、远端已停止、真实模型或生产后台持续调度，生产开关仍关闭。

随后84C188…冻结下，8版RC公司报告当前桌面H5再次退出0：人工创建恰好1次、CN/模型1/3、报告/成功结算各1、冻结归零、审计及迁移44/44通过。实际查看四节、公司依据与诚信标注后撤回来源，再读拒绝并清空正文；切换另一示例会员，新视图无他人任务、旧标签清除私有状态。报告及撤回截图、日志名见tasks.md同日条目，不把旧冻结截图替代本轮证据。操作时目录可能同时命中题材与环节，须按可见名称消歧报价按钮；确认框关闭前点击查看若无效，先观察再重做只读查看，绝不补创建。

首轮历史证据（2026-10-20，1298文件冻结`46C7D3C06EC3AD88727FC099A54C216A7B1EF6974864032146820F2DC6B677BA`）：8/9自动化专项各25条PASS、0失败/跳过、迁移44/44；加独立纯源测试后，四个新CI表达式各匹配1/2/4/1。实际1280×720浏览器只验8版RC成功和9版RR排队取消，两个人工进程都退出0并清理实例；测试CN/模型成功计数1/3、取消0/0，未调用真实服务。报告/取消截图及日志名见tasks.md同日记录。此前两次人工场景因120秒上下文退出1且现场保留，不算通过。七文件前端冻结为`BB608CC804935411676367743E8F9D31C9FB3DCDA51E39F14053AEE82017F9D0`；该轮未验375px、资料库公司与未知回执/重载，后续证据见下，不把首轮或旧1297冻结的601条报告组升级为当前全仓SQL通过。真实调用、收费、部署继续关闭。

同冻结最终完整harness含覆盖率退出1，仅既有疑似密钥扫描/自测红，未改扫描器或基线；Go编译/竞态/vet、13包覆盖率棘轮、GVA关键包、合规/架构/pages.json与YAML/24段Shell语法通过。70.5%仅Go单测总语句覆盖，不等于集成测试或模型质量。扫描日志只可在本地按标签/计数核查，禁止将疑似密钥原样贴入验收记录或聊天。

第二轮最终1298文件冻结`3950874B343D260165045A4CC80623A29115AFE211EE143CC02E26A78C413E7B`：关联H5/公司/资料库/会员提交专项在8/9各168条PASS、0失败/跳过、迁移44/44；独立纯源测试通过，对应20/105个CI表达式各匹配，H5守卫为1/2/8/1。9版RR公司报告与8版RC未知创建/重载/原键找回/取消两个实际375×640 iframe会话均退出0，各21条PASS及迁移44/44，人工创建POST均恰好1次、冻结归零并通过积分审计；公司报告/结算各1，未知取消均0。日志和最终截图见tasks.md第二轮记录，较早174A…冻结的证据不替代该版。前端未改，源码交互回归及integration包vet通过。相同冻结最终harness含覆盖率退出1，Go编译/竞态/vet、13包覆盖率棘轮、GVA关键包及合规/架构/pages.json通过，总语句覆盖70.5%；仅疑似密钥扫描及其自测前置失败，自测未执行后续探针，扫描器/基线未改。它不验证真实手机、运行中取消页面、生产并发/吞吐、站点登录/网关或全仓SQL/全部报告组；测试网关串行夹具不能用于这些结论。真实调用、收费、部署继续关闭。

该次密钥扫描全部46条命中的脱敏路径均在四个Git已登记的嵌套工作树中；这只证明本次扫描范围涉及其它checkout，不是对命中内容真伪的判定。用户随后明确授权仅跳过Git登记的其它工作树；`check-secret-scan.sh --all`现用NUL分隔的登记路径逐个精确剪枝，保留既有扩展名与第三方排除口径，不增加基线。当前工作树自身、未登记目录、独立嵌套Git仓库及Git忽略文件仍扫描；`TM_REPO_ROOT`不是该仓库根时不继承父仓库的跳过范围，暂存区仍按当前索引检查。两个文件仅叠加工作区修改，三个原暂存对象及基线均未改，用户工作树未删除。全仓扫描退出0，完整扫描器自测42/42；仅定位范围可用`bash scripts/dev/verify-secret-scan.sh --worktree-scope-only`，其12项不冒充全套自测。扫描器自测前置失败和后续探针失败必须分开记录，不因只显示前30条就断言剩余范围没有命中。

原生迁移探针单独复验：`bash scripts/dev/verify-migrations.sh --self-test-probe`只测固定`SELECT 1;`探测函数，不连接数据库或安装清理trap，也不代替迁移44项。覆盖有效确认行、诊断/大输出、空输出、错误确认行及收到确认行但进程退出7/141的失败保留；最终八项连续20轮通过。默认原生路径先完整读取客户端/tr输出，再按退出码和独立确认行判定，失败只输出固定说明与退出码，不泄露客户端正文。合成旧管道的SIGPIPE反例不等于此前真实连接失败原因已查明。此自测在harness及CI后端作业执行，不前置阻断合规作业。

后续完整报告组证据：1279份Go/SQL摘要`00C7DE81CC6484B58304D2E80989FB97A08B6998FB3EF416F5C9932DD6672B91`下，MySQL8串行与9版各612条PASS、0失败/跳过、迁移44/44、进程退出0且成功实例清理；48个points报告根组与70个对应CI表达式（含独立纯源日志及API启动）逐项核对。9版早于探测脚本修正，8版运行期间仅自测/扫描器迭代，不能称最新全入口双版本整轮；Go/SQL及默认原生探测在8版运行时保持不变。日志名见tasks.md同日记录。这不是105个CI表达式全量或全仓SQL验收。此前8版整组548条PASS后触发20分钟Go看门狗，以及补跑Workflow64条PASS后原生迁移前置失败，均按整体退出1保留；成功串行复验不覆盖原失败记录或证明超时根因。20分钟是points的Go测试进程上限，不是整个PowerShell入口总时长；未放宽单次SQL/网络、租约、业务期限或既有执行数守卫。

扫描范围修正后的1298文件冻结`C820154E53F61E8AB59E59FFFE3C0A72A8A961F4BDBDACD3E214369BBCC9970B`，完整`bash scripts/harness-checks.sh --with-coverage`已退出0，含38项合规自测、8项客户端探测自测、42项扫描器自测、全仓扫描、Go/race/vet/build、13包覆盖率下限及GVA关键包、架构/pages.json；race未降级，Go总语句覆盖70.5%。YAML的25段Shell语法通过，暂存对象及基线再次核对不变。日志`tm-stock-ai-secret-scope-harness-2e517b48640b47cdbdbb00842f213fcc.log`仅保留本地，不公开扫描正文。该绿灯不是全仓SQL、远端CI、模型质量或上线许可，真实调用/收费/部署仍关闭。

### 报告反馈的受理、查询与内部处理（真实调用关闭）

会员与处理方分别使用`TM_AI_FEEDBACK_ENABLED`、`TM_AI_FEEDBACK_ADMIN_ENABLED`，默认关闭。会员还须有效的任务读取/鉴权配置；内部仍走`cmd/ai-admin`专用回环监听和凭据绑定操作人，不能用会员JWT、浏览器Origin或请求体指定actor。需先在获准的同结构副本执行`20261020_create_tm_ai_report_feedbacks.sql`并验证，新表不会自动迁移。启动账号需原报告联合自检的权限，以及两张反馈表SELECT/INSERT/UPDATE/DELETE用于回滚拒写探针；不授DDL/TRIGGER。业务处理不会执行DELETE，启动权限也不能绕过触发器。未演练生产升级或生产受限账号，不直接在线上执行本节。

1. 会员在已完成任务的“举报与反馈”展开后，先GET`/api/v1/ai/tasks/{id}/feedback`；仅确认属于本人且有成功报告才返回。正文当前不可读不阻断历史反馈查询。
2. 选`fact/source/display/other`之一，二次确认后POST同一路径，JSON只有`category`。同人/任务/分类唯一，回执包含受理单编号；不填写正文、账号、联系方式或内部判定。不把定位信息复制当作受理。
3. 内部以专用凭据GET`/internal/v1/ai-feedback?limit=20`，后页使用`nextAfterId`作为`after_id`；GET`/{id}`核对当前版本与不可变事件。无浏览器处理页面，也没有自动指派/通知服务。
4. POST`/internal/v1/ai-feedback/{id}/decisions`，精确JSON`{"expectedVersion":1,"status":"processing","resolution":""}`开始处理；再用版本2提交`resolved`及`reviewed/referred`，或`dismissed`及`unconfirmed/not_applicable`。终态不能重开；同actor同原操作重放返回当前记录，不会重复写事件。冲突409先查询，未知503只查询原记录，不换分类补发。
5. 会员显式“查询反馈状态”看到处理结果。受理/查询/处理均不更改报告、资料授权、模型调用、冻结、结算或退款；“已处理”仅代表有核查记录。报告纠错/授权治理或财务处理另行按原流程批准，不由反馈接口自动实施。

定位验证：`verify-ai-source-native.ps1 -ReportsOnly -ReportPattern '^TestIntegrationAIReport(Feedback|H5Gateway|Schema)'`，分别使用MySQL8/9的已有程序目录，隔离实例只绑定回环。该专项不是全仓SQL验收。`TestIntegrationAIReportH5Gateway/Read_Committed/feedback`可配合`TM_TEST_AI_H5_ORIGIN=http://127.0.0.1:5174`代理已有HBuilderX开发服务，仍只用显著测试会员/CN/模型，不读取生产登录凭据。浏览器从`/__test__/`创建一次测试任务，提交一条反馈后，测试控制页“处理测试反馈”经原内部鉴权及handler推进两次，再回会员页查询；撤回测试来源仍须可查询反馈。375px页只是iframe布局验收，不是物理手机。结束时核验反馈POST恰好1次、至少2次GET、反馈1条/事件3条、报告和结算各1、冻结0、CN/模型1/3及积分审计通过；否则整轮失败保留实例。控制页确认不代替Go脚本退出0。

### 会员报价、创建与原键核查（开发接线，真实调用关闭）

依ADR-0021与design 2.4.5，`TM_AI_TASK_CREATE_ENABLED` 独立默认false。开启还须任务读取/有效会员鉴权、真实准入控制器Ready及任务/账本联合自检；目前API没有生产控制器，误开会拒绝启动，不降级采用测试价格、固定证明或默认通过策略。开发付费放行、Agent、取消、报告开关均不隐含创建许可。当前只实现L1题材新建，不提前冻结L2/专题。

报价POST只收分析类型、主体类型/编号；创建再收价格版本与单个Idempotency-Key头。金额、模型、预算、来源及用户编号不得由客户端决定。报价按数据库UTC和实际已生效未到期批次计算可消费积分，不等于未清扫的账户总余额。新创建复用原冻结/任务/首事件事务；创建收据只有任务号、原冻结积分和版本，不是成功报告或结算证明。

`GET /api/v1/ai/task-receipt` 沿用任务读取开关，即使新建关闭也可核查原用户/原键；拒绝URL参数及正文，不读取模型。只有确认原冻结与原流水的任务才返回收据，他人和不存在同404；未知固定503、空收据。任何来源或模型撤回都不能把创建收据转换为报告读取许可，报告仍逐次终检/当前授权复核。

H5创建组件复用目录搜索→选择题材→只读报价→二次确认。它在POST前持久保存最小请求与安全随机原键，Web Locks协调同账号页签；缺这些能力或本机存储失败则不发送。依据DCloud的[平台API说明](https://doc.dcloud.net.cn/uni-app-x/api/)将浏览器能力置于WEB条件编译内，非WEB不回退成弱键。JWT ID仅分区本地意图，不代替服务器身份检查，也不保存令牌/报告/原文的新副本。离页、超时和重载后仅显式核查原键；404不代表原请求绝不可能稍后提交，不能因此自动换键或清除现场。明确拒绝或确认收据后才允许清除对应本地意图；不得手工改写任务/冻结/流水来解除未知。

代码级测试仍用既有 `verify-ai-report-ui.cjs`，新增实际创建组件/DTO/请求绑定、随机/互斥/存储故障、二次确认、余额、变价、回执未知、重载/续签与账号隔离测试；原报告和依据组件回归不可省略。它不执行实际tmx-ui组件，即使依赖已安装也不是HBuilderX或浏览器验收；实际H5专项另见上节。SQL定位用既有 `-ReportsOnly`，包含会员提交生命周期/边界/并发及真实可消费批次，API创建启动单独必跑；两版完整回归不带专项。所有CN/模型/批准服务仅在测试中显著标注并隔离，未给产品生成演示任务。

首次准入持久记录及执行绑定的增量见下一节，正在验收；仍缺生产价格簿、真实批准登记/实时核验及模型/来源/质量/远端硬预算证明，继续保持关闭。真实页面依赖与验收的缺口见上一节，不能用本节测试证明公众可用或可以上线。

创建收据后的“查看已创建任务”只刷新本人历史与原任务详情两项GET；不再保留创建前的“暂无记录”或旧分页。两项查询独立失败，详情不能被历史回调清除，历史也不按收据补造行。列表状态标明“历史查询”或同范围详情的“当前核对”；这不是报告读取或退款证明。旧首页/分页、重复查看、401/换账号/离页用实际源码交互测试回归，原已发送请求键始终保留，不自动重发创建。

### 首次准入计划持久绑定（ADR-0022，开发验收中）

新会员创建复用原冻结事务，但必须同时写任务上的不可变准入摘要和 `tm_ai_task_admissions`，不再只有价格与预算。记录有版本化规范JSON与摘要，只含资料元数据和配置/批准身份，不存来源正文或Agent原文。创建/首事件/记录任一失败全部回滚；未知提交原键核查须验证原记录，不自动补写。已部署库不得直接运行此迁移，须另行获准并在相同结构副本验证；不会自动迁移或为历史行补造批准。

执行前及选择/判定/规划复核时，核对原任务与持久计划、完整执行摘要，再做当前授权核验。Source/Model等当前证明仍由可信控制器核验，摘要格式不能当授权。带记录的任务只能由完整L1编排驱动，原单Agent `Run/RunOwned` 不能绕过；无标记且无记录的旧内部任务保留既有开发核，不代表公众新建可降级。拒绝或记录损坏不会触发补调用/补扣费，仍可沿原取消/过期路径释放。

新的创建启动自检为 `AIAdmissionSchemaVerifier`，包含原基础联合自检与准入列/表、复合身份、真实拒写、空壳CHECK、不可变与唯一保护探针，全部回滚。原 `ai-task-check` 只检查基础任务核，不能代替新增关口。验收账号需在原任务/账本运行权限之外具备新表SELECT/INSERT/UPDATE/DELETE，以让回滚探针实际验证更新/删除被触发器拒绝；不授DDL/TRIGGER，权限不足时拒绝启动。原键核查和执行绑定读取需要新表读取权限，并短暂遵循原账户→任务锁序；不是无锁或仅SELECT权限的通用历史查询。无新迁移时相关路径失败关闭，不静默按旧内部任务处理。

报告转换、终检/重放及会员报告GET的各次事务内读取同样复核准入；写后确认失败回滚报告、成功事件与结算。已成功报告因记录故障拒绝读取时，不撤销原结算，也不补调用或从旧缓存出正文。报告启动联合自检新增准入保护，验收账号也需上述新表回滚探针权限。

报告侧同时比对首次模型公示、题材和已选快照的全部身份元数据，不只检查记录存在；允许原材料子集，不允许新增其他已授权但未获首次准入的来源。范围不符不改变冻结或补执行，恢复只能按原计划/原键核查，取消/过期仍走原事务。该补验没有改变JSON版本/迁移或开启任何服务，完整执行摘要、当前来源/发布批准仍不可省。测试覆盖层只能用于隔离定位，落地相同文件后不得继续把旧虚拟文件作为新增文件编译；由此发生的重复定义须记为失败并用主工作区重验，不算业务回归通过。

定位沿用 `verify-ai-source-native.ps1 -ReportsOnly`，新增 `TestIntegrationAIReportAdmissionBinding/Atomicity/Schema/Reads/Finalize`；正式回归不带专项。模型、来源与批准服务均为显著标注的测试夹具。分别核验单/多来源正例、规划改变零调用、单Agent旁路拒绝、缺/坏记录、原子回滚/未知提交及缺保护自检；报告侧另验两次读取间失效、提交不明、终检最终写/写后确认故障与原结果恢复，不能只看编译或静态迁移文本。上轮MySQL9完整通过，MySQL8慢响应心跳RR失败；报告侧增量仍待当前双版本取证，不能套用旧冻结结果。慢响应测试新增仅状态/剩余租约/模拟请求数诊断，不续租、不重试或放宽期限。

### 会员任务有限批次编排（ADR-0023，默认关闭）

`NewAIL1ReportQueue`仅接收既有完整L1执行核、最小元数据扫描器和显式开关，不启动循环、注册公开路由或创建任务。`RunBatch`每批固定数据库UTC/主键上界，最多处理100行，另读一行判断后续；旧内部任务、L2、专题、将来可执行任务只跳过。候选资格不授执行权，当前读原记录/时点后仍由原Run核验当前授权、租约与预算。首次没有迁移、准入损坏、缺依赖或未知提交时保留原键与冻结，不自动补记录或释放。

返回的Complete只表示固定窗口遍历完成，TerminalObserved只是白名单终态观察，不是报告发布、结算证明或新增交付数量。LastID是本批已确认前缀，错误行不跨过；读完后新一轮从零开始，禁止把游标当永久消费偏移。响应不明只记Pending，恢复仍走原任务，不换键重发。后台服务/生产控制器没有装配，不代表线上会自动消费。

定位使用已有原生脚本的显式`-ReportsOnly -ReportPattern '^TestIntegrationAIReportQueue'`，默认ReportsOnly仍跑全部报告，完整回归不带专项。选择器仅用于测试定位，不能拿54条专项PASS冒充完整账本/恢复/Agent/GVA回归。四根组覆盖HTTP新建后队列交付/释放、有限游标、身份隔离、来源撤回/缺记录、真实提交后未知、同实例与独立worker竞争；回环模型与示例批准不进入生产路径。双版本专项和20轮竞态已有证据，完整新冻结回归及页面/外部配置关口按tasks.md继续跟踪。

### 后台反馈处理页面与专用桥接（开发完成，默认关闭）

后台登录后可访问`/#/ai-feedback`，复用GVA的Vue/Element Plus与原`x-token`会话。显式开启后台反馈时自动在已有GVA目录登记顶层“报告反馈处理”（组件`view/quant/aiFeedback/index.vue`、名称`AiFeedbackConsole`、路径`aiFeedback`、`keepAlive=false`），不修改角色/菜单关联或Casbin权限；关闭无数据库目录访问。正常目录重复不新增，冲突/软删除/缺表/缺角色锚点均拒启动且不覆盖，不执行DDL。启动账号须具备现有角色行锁及菜单/API目录的读取/插入权限；不具备时关闭此功能并保留现场，不能改成自动建表或忽略错误。菜单目录与三个API同一事务登记，五秒有界，不因目录完成而启用真实调用。固定登录保护入口名称`AiFeedbackDirect`与动态侧栏名不同。仅给指定后台角色显式登记下列三项Casbin API权限（路径相对GVA路由前缀），不批量授予所有管理员：

| 方法 | 路径 |
|---|---|
| GET | `/quant/ai-feedback` |
| GET | `/quant/ai-feedback/:id` |
| POST | `/quant/ai-feedback/:id/decisions` |

管理员进入现有“角色管理→设置权限”：在“角色菜单”勾选“报告反馈处理”，保留该角色已有菜单；在“角色api”仅追加“报告反馈处理组”上述三项，保留原会话基础权限，然后分别确认保存。现有接口为`POST /menu/addMenuAuthority`与`POST /casbin/updateCasbin`（以router中的小写u为准，不照抄历史Swagger注释的`UpdateCasbin`）。两接口都是全量覆盖而非增量追加，不能只传新项冲掉原菜单/会话权限。需要已有的菜单/角色/API管理权限与严格角色树授权；处理角色不获得权限管理接口。处理人重新登录/刷新后从`/#/layout/aiFeedback`进入；API/菜单仅一项获准、会员令牌、当前账号停用或服务端未登记均不能获得处理权。撤权优先撤三API，再撤菜单并刷新；只隐藏菜单不能当作安全撤权。不得将本地测试角色编号或测试凭据复制到正式配置。

还须由服务端运维受控注入以下独立配置；不写VITE变量、可查询的GVA系统配置或聊天，不提交实际凭据：

- `TM_AI_FEEDBACK_CONSOLE_ENABLED`：空或`false`时不注册桥接；仅显式`true`启用，其他值拒启动。
- `TM_AI_FEEDBACK_CONSOLE_UPSTREAM`：原`cmd/ai-admin`的HTTP字面回环IP及明确数字端口，无路径、凭据、参数；不接受域名、重定向或外部代理。原内部`TM_AI_FEEDBACK_ADMIN_ENABLED`及回滚结构自检仍须单独满足，不能转向会员API。
- `TM_AI_FEEDBACK_CONSOLE_WEB_ORIGIN`：管理前端精确HTTPS Origin，无尾斜杠/路径；仅本机开发允许字面回环HTTP。浏览器传Origin时必须匹配；不把此项当身份认证。
- `TM_AI_FEEDBACK_CONSOLE_REVIEWERS`：JSON数组，元素仅`userId`、`actorId`、`token`。分别填写真实后台用户编号、原内部登记的处理人编号与其专用凭据；三者分别唯一，最多100项。凭据必须与原内部actor绑定一致，禁止会员令牌代替。此配置不经浏览器传递。

开启配置后仍逐请求检查后台JWT类型、改密守卫、Casbin与当前`sys_users`启用/未删除状态及上述逐人登记；`x-user-id`、浏览器请求体和JWT中角色不能代替当前账号核验。错误配置启动失败，失效账号/未登记人员失败关闭。桥接仅转接三个固定反馈端点，关闭HTTP代理/重定向/自动重试；每次上游请求最多6秒，写后的只读核对另有一次请求。写后按原详情历史验证配置actor和原版本事件，不能将其误解为写前绑定证明或跨服务事务。

页面按编号递增分页/按单号查询，查看详情与不可变处理历史；先“开始处理”二次确认，再从固定结果中选择终态、二次确认。每次只发一个POST，成功/冲突/超时都先“核对原受理单”，不自动补写。换账号、离页或停用清空私有页面状态；终态不可重开/删除。它只记录核查，不自动更正报告、恢复授权、通知或退款，也不启用AI执行/创建/发布/收费。没有更改生产菜单、角色、环境或监听。

源码复验：`node scripts/dev/verify-ai-feedback-admin-ui.cjs <隔离依赖目录>`，现有CI隔离依赖为Vue/compiler-sfc/server-renderer3.5.13、TypeScript5.7.2、Axios1.8.2、vue-router4.4.3。脚本执行实际统一Axios请求/响应拦截器（仅底层adapter替换）、原permission守卫与Vue Router内存路由、Vue KeepAlive生命周期；会话store/DOM/Element宿主仍模拟，不代替实际浏览器或生产登录。覆盖Casbin的HTTP200/code7局部归一化、401清登录、409无重试、续签丢弃旧回包、固定/菜单入口和缓存停用。Go专项在`backend/server`执行`go test -race ./config ./service/feedbackconsole ./api/v1/quant ./initialize -run '^TestFeedbackConsole' -count=20`和对应`go vet`，包含三项实时角色接口撤权/零转接。核心桥接覆盖率用`bash scripts/check-feedback-console-coverage.sh`，最低85%，未降低旧门槛。配置专项只选新增组；不要为运行历史配置测试补造未随仓库提供的部署模板。

本地浏览器真库复验需隔离依赖Vue3.5.31、Element Plus2.14.5、Vite8.0.10与插件6.0.6。先运行`node scripts/dev/serve-ai-feedback-admin.mjs <依赖目录>`，只监听127.0.0.1:5176；`--build-only`仅构建实际组件/API/Element Plus验收宿主，不是整套GVA生产构建。宿主替换全局会话与传输层，不包含伪造反馈数据；全量Element Plus的测试包大于500KB警告不能当作生产性能验收。

在`backend/server`用`go test -race -tags=integration ./api/v1/quant -c -o <临时绝对路径.exe>`编译专用测试程序。另一次性PowerShell会话设置`TM_TEST_FEEDBACK_ADMIN_RUNNER=<该exe>`与`TM_TEST_FEEDBACK_UI_ORIGIN=http://127.0.0.1:5176`，再用原生脚本选择`-ReportsOnly -ReportPattern '^TestIntegrationAIReportFeedbackHTTP$'`及8版MySQL程序目录，结束后清除这两个临时变量。父测试启动原会员/内部API及真实MySQL反馈库，并只向子进程传专用测试凭据；子进程用真实GVA JWT/Casbin及当前账号核验，后台账号表是隔离SQLite夹具，不冒充GVA生产MySQL部署。

当前人工会话使用整套GVA构建的本地预览源（例如`vite preview --host 127.0.0.1 --port 5178 --strictPort`，前缀`/api`）；不能再用仅页面宿主替代真实登录。进入日志中的回环`/__test__/`查看隔离管理员/处理人/普通后台账号与本次随机测试口令（仅控制页展示，不写日志/聊天）；控制页不注入JWT。通过原登录页登录管理员，在原角色面板勾选菜单和API并保存，正常登出后登录处理人，由真实侧栏进入，处理受理单至终态并核对三条历史；再登录普通后台确认无菜单、直接入口也被拒。最后控制页“结束并核对”必须确认至少两次登录、菜单及API授权保存、两个反馈处理POST、至少三次GET、原MySQL版本3/三个事件/处理人一致、账务不变、测试CN/模型1/3和积分审计，且父脚本退出0、迁移48/48；控制页回执不等于整轮成功。人工子程序仍8分钟，未放宽原任务/模型期限。账号SQLite使用单连接以支持角色面板并行查询；门禁另有并发反例及真实登录/菜单分配/API授权/撤回原JWT拒绝验证。仅测试夹具使用高验证码触发阈值以隔离验证码服务，不更改产品默认值或生产安全设置。此处CN/模型仍是回环服务，不运行TradingAgents-CN或真实推理。2026-10-20实际浏览器及10-21整包构建/本地登录记录见tasks.md；生产网关、实际线上权限和真实服务仍未验。

整套GVA前端构建须另建仓库外临时目录，仅复制`backend/web`的`src/public/vitePlugin`及`package.json/vite.config.js/uno.config.js/index.html/babel.config.cjs/jsconfig.json`等公开构建源码；不复制.env、用户dist或后台运行配置。在该目录运行`npm install --ignore-scripts --package-lock=false --no-audit --no-fund`，不修改业务依赖或提交锁文件。`@vue-office/{docx,excel,pdf,pptx}`通过`lib/script/postinstall.js`选择Vue3入口，`vue-demi`通过`scripts/postinstall.js`选择版本；必须先读完整脚本及其utils，确认仅复制本包文件，再逐项用Node执行这些已核对的本地初始化脚本，不打开所有安装钩子。设置公开测试前缀`VITE_BASE_API=/api`及字面回环开发代理参数，执行`npm run build`；该步骤不启动服务/触发API，也不是正式部署配置验收。

后台账号库/目录的真MySQL专项：在仓库根执行`./scripts/dev/verify-ai-source-native.ps1 -MySqlBaseDir <MySQL程序绝对目录> -BashPath <Git Bash绝对路径> -FeedbackConsoleOnly -RunCount 3`，分别提供8/9版程序目录。脚本新建隔离实例，不读取本机已有DSN，串行运行`TestFeedbackConsoleMySQLCatalog/Login`并执行全部客户端迁移检查；专项不代表全仓SQL回归，默认完整模式也会追加此组。每轮必有2根组、2隔离级别、12叶场景，0跳过，CI守卫至少16条PASS；原有下限不降。只允许字面回环TCP及`tm_ai_source_test/tm_stock_it`测试入口，另建固定专库`tm_feedback_console_it`，存在就停止，绝不清空接管；只有本次CREATE成功才注册DROP清理。登录子程序每轮独立，使用原密码校验/菜单/Casbin接口并检查授权真实落库；失败只向日志报告已知断言文件行号，不转发登录回包或DSN。此专项不运行浏览器，不替代上一节浏览器会话，也不启用真实模型。当前实例内撤权由反馈路由无缓存复核兜底；跨实例同步、已在途请求和生产最小权限账号另验。

构建后核对`src/pathInfo.json`将`/src/view/quant/aiFeedback/index.vue`映射为`AiFeedbackConsole`且产物包含真实反馈页；ESLint至少覆盖反馈API/页面、路由和旧角色组件。遗留`Datas`已改为明确只读停用提示，保留名称及切页暴露契约，不恢复已删除的角色关系API；现行`setDataScope`/部门集逻辑不变。2026-10-21按当时package.json的范围解析到Vue3.5.44/Vite8.3.4/Element Plus2.14.7/vue-router4.6.4/Axios1.8.2，3906模块完整构建退出0；无锁安装不代表未来版本可重现。第三方BigInt与大包/插件耗时警告保留，不修改全局target掩盖兼容问题，不能升级为旧浏览器、全站功能或生产性能验收。

### 清扫与恢复型 worker 既有验证记录

过期清扫的2026-10-16最终验证记录统一见 tasks.md「显式限量过期清扫」：双版本完整回归及各三轮专项通过，迁移各38/38，成功实例停止并清理，CI表达式本地核对通过。MySQL8另一次客户端连接前置失败的实例已停止并保留，未计为通过，具体临时目录和未定位边界亦记录在那里；总门禁仍仅既有疑似密钥/扫描器自测红。以下仍是恢复型 worker 上一增量的证据，不与清扫的事务错误注入混称杀进程验证。

2026-10-16 恢复型 worker 验证：最终产品/测试冻结后的双版本全套均通过，恢复三组/生命周期两个隔离子例与十六场景/两个真实杀进程子例/四个命令子例全部通过；原仓储、账本、任务、会员只读、预算、阶段、抽取、Agent 及 GVA 回归通过，0 跳过，迁移断言各 38/38，成功实例停止并移除。同一最终代码随后在双版本各连续三轮通过恢复专项（每版三组各三次、六个杀进程子例、十二个命令子例），0 跳过/失败，客户端断言各 38/38，成功实例均停止并清理。实际命令在受限账号下通过联合自检与恢复，DDL 拒绝按 MySQL 权限错误验证。新增核心单测 Recover 93.9%、心跳循环 100%、逐步恢复 79.2%，计划契约 100%、命令包 88.1%；新增题材 SQL 读取合并同版本单测/专项语句块为 5/6（83.3%），数据库错误包装分支未覆盖，不等于全仓储/服务覆盖或生产性能。首轮短心跳 SQL 超时失败已修正，慢响应/超时/父期限回归通过，失败实例停止且证据保留，不算通过次数。Go/race/vet/build、覆盖率棘轮、合规/架构/格式及工作流/脚本语法通过，总门禁仍仅旧工作树疑似密钥扫描和扫描器自测红，未放宽基线、未运行远端 CI。无真实模型/Agent、扣积分、生产迁移、部署、提交或推送。
