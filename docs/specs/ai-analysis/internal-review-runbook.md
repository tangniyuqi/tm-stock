# AI 内部操作手册（L0 审核与隔离 Agent 执行）

## 会话保存档案引用专项（2026-10-30，仅本地）

引用服务、独立API和会话保存/恢复页面已接本地装配。`TM_AI_LOCAL_CONVERSATION_DOSSIER_ENABLED`独立且默认关闭，显式开启依赖原档案读取及会话开关、同库隔离配置与会员认证；真实调用仍须关闭。准备账号检查新表列、生成列、索引、双方版本/归属外键、不可变触发器及实际回滚探针后关闭连接；运行账号只新增引用表SELECT/INSERT，原资料表保持只读。权限预检用空INSERT核权限，不保留行；准备探针可能消耗会话/引用自增编号，编号连续性不是验收标准。关闭新开关不要求安装新表，不改变原档案只读入口。

在仓库根目录用PowerShell7串行运行，不与其他真库或完整门禁并行：

```powershell
./scripts/dev/verify-ai-source-native.ps1 -MySqlBaseDir D:/mysql/mysql-8.0.46-winx64 -BashPath D:/Git/bin/bash.exe -RunCount 1 -ReportsOnly -ReportPattern '^TestIntegrationAIReportConversationDossiers$'
./scripts/dev/verify-ai-source-native.ps1 -MySqlBaseDir D:/mysql9/mysql-9.7.0-winx64 -BashPath D:/Git/bin/bash.exe -RunCount 1 -ReportsOnly -ReportPattern '^TestIntegrationAIReportConversationDossiers$'
```

上述内核专项预期精确1根、2隔离级别及32叶通过，零跳过；父脚本还须启动检查、65项迁移断言及停止清理成功。SQL使用显著合成版本/测试批准，身份为服务替身，不代替下方原登录引用API或H5证据。提交确认丢失、数据库末尾时钟跳变和末尾身份失效须标为故障注入。引用表与原会话/消息/任务/账本隔离，检查无新执行或账务；归档与撤权后的原组合仅恢复本人保存收据，正文仍重验当前全部权限。

引用结构/原登录API专项将选择器改为`^TestIntegrationAIReport(ConversationDossier(Schema|Process)|DossierProcess)$`，仍按上方两个MySQL版本串行运行。新Schema根组精确11叶、Process根组精确6叶；另保留旧DossierProcess的5叶回归。必须核真探针回滚、同名失效保护拒绝、运行账号只读/插入增量、原密码JWT后的保存/重放/目录/收据/正文、撤权和归档，以及消息/任务/账务/模型无新增。HTTP进程通过不代替浏览器操作。

会话页面只在主动展开时查询引用；保存已打开的单/双版前先读取当前会话版本。提交结果未知时只提供原组合收据查询，不自动重发；成功后刷新“本会话已保存档案”。目录只展示本人保存编号及版本，打开和复制每次重读当前原文；保存不改变问答的题材检索范围。账号、会话或页签切换清空正文并丢弃迟到响应。

原迁移目录已改逐表读取，禁止用GROUP_CONCAT结果判断尾部表存在性；自测超过1024字符仍保留尾表，同名前缀与非表标记不得冒充。新表加入全迁移顺序，旧断言不移除；首轮62/64的失败不能算通过。设计/API契约见design13.8，实际证据与剩余事项见tasks的DR-4。

## 专题档案存储专项（2026-10-30，仅本地）

本地专题读取入口：`TM_AI_LOCAL_DOSSIER_READ_ENABLED`缺省关闭。仅在已有`TM_AI_LOCAL_CONFIG`隔离配置、会员读取认证和真实调用关闭前提下，显式启用并安全注入`TM_AI_LOCAL_DOSSIER_PREPARE_DSN`；它与原业务DSN必须指向同一回环地址和专属库，使用不同数据库账号。准备账号执行隔离标记与结构回滚检查后连接关闭；业务读取账号须有隔离标记、会员/题材及专题/来源/快照/正文表的读取权限，不因此授予写入/TRIGGER。完整检查后才注册版本目录及`GET /api/v1/ai/themes/{id}/dossiers/{version}`和可选compareVersion，失败无路由。不要在终端日志打印DSN；不开生产或候选写接口。摘录对照不代表语义核验或事实更正。

原生ReportsOnly会额外执行`TestIntegrationAIReportDossierStartup`（当前15场景），可指定`-APICoveragePath`收集该启动窗口的API/handler/service跨层覆盖；这不代替完整HTTP进程、原密码登录或H5验收。

版本目录为`GET /api/v1/ai/themes/{id}/dossiers`，可选`beforeVersion`；最多每页10个已重新核验版本，不返回不可读数量。扫描预算32候选未完成时应返回暂不可用，不能把它当真实空目录。会话中先选择问答题材，再点“查询可读档案版本”；可阅读单版、加入两个版本对照，复制任何出处时重新读取原单/双版。此选择只用于本次查阅；启用上节独立保存开关后可主动保存为会话引用，引用仍不是聊天消息，问答检索范围仍为题材。

独立进程专项将下方选择器改为`^TestIntegrationAIReportDossierProcess$`，仍串行运行MySQL8/9。预期根组及5个具名子例通过；实际原密码HTTP路由校验、JWT、同库资料和独立`cmd/api`参与，未启动worker。末尾检查任务/积分流水无新增、报告为零及模型/Agent替身零请求，不据此宣称浏览器已经操作。CI分别守卫该根组、每个子例及只转发网关测试，实际浏览器专项不混入普通自动化执行。

实际H5专项先用下述官方编译器生成当前源码产物，通过回环静态服务提供；将`TM_TEST_AI_H5_ORIGIN`设置为该服务实际源，再运行：

```powershell
$env:TM_TEST_AI_H5_DOSSIER='local-fixture-only'
./scripts/dev/verify-ai-source-native.ps1 -MySqlBaseDir D:/mysql/mysql-8.0.46-winx64 -BashPath D:/Git/bin/bash.exe -RunCount 1 -ReportsOnly -DossierBrowser -ReportPattern '^TestIntegrationAIReportDossierH5$'
```

打开日志中的本轮`/__test__/`入口，在辅助页私下取得随机测试口令，用原H5密码表单先验证错误口令拒绝，再登录会员一并新建一个会话。选择合成题材，检查375×640输入草稿、档案目录/单版/双版对照；档案展开时输入框移回文档流，关闭后恢复固定。完成读取/对照后从控制页撤回旧版，点原来源入口确认拒读及旧正文清空，刷新目录只见新版；恢复发布后重新读取/对照。最后通过控制页清理本轮token，会员二原登录，核对没有会员一会话/引文/草稿，点击“结束并核验”。网关只转发业务响应；控制页只修改显著合成测试发布及本轮登录态。最终须等原登录子程序、Go根组、父脚本迁移与清理均退出0，不能把“操作已确认”当最终通过。十分钟为本轮人工交互上限，不修改业务租约；超时是失败，保留日志和停止实例，另起测试须先确认原进程终态。成功复制提示仅证明API复验及回调，系统剪贴板内容未读回时仍列未验证。

验收保存/刷新恢复时，在上述命令前额外设置`$env:TM_TEST_AI_H5_DOSSIER_REFERENCES='local-fixture-only'`，其他参数不变；普通档案专项需先清除此变量，不混用证据。保存模式先完成原目录/单版/双版读取，再仅保存一次版本1与2组合；打开引用、刷新整页、从历史打开原会话、查询保存目录并再次打开。控制页撤回旧版后通过引用的原文入口验证拒绝及整组正文清空，恢复后再次打开；最后验证第二会员无会话、旧标签私有状态清空并结束核验。网关要求一次保存、至少两次引用目录与三次成功引用读取，父SQL要求总引用恰好一条双版组合及零原消息/任务/账务/模型活动；达到计数不替代实际刷新、布局或账号操作。不要提交问答、创建第二会话或为凑计数重复保存。

本机官方编译器已核实在`D:/workspace/services/tm-stock-hx-5.26/HBuilderX/plugins`；工作目录须切到其`uniapp-cli-vite`，用`../node/node.exe`执行`node_modules/@dcloudio/vite-plugin-uni/bin/uni.js build -p h5`。一次性进程设置`UNI_INPUT_DIR`为仓库绝对路径、`UNI_OUTPUT_DIR`为新建仓库外临时目录、`UNI_HBUILDERX_PLUGINS`为plugins目录、`UNI_APP_X=true`及`UNI_PLATFORM=h5`。5.26会重新计算UNI_CLI_CONTEXT，单独设置此变量不足以在仓库根启动；相邻about包识别及失败原文见现有tmui4x技能page-dev-guide第7节。必须检查退出码、日志末尾Build complete及输出index.html，不以退出0或旧产物替代；这是编译，不是浏览器/手机验收，也不发布站点。工具目录变动时先核实际路径，不扩大扫描到业务资料。

在仓库根目录使用PowerShell 7串行运行两个版本，不与其他真库套件共用实例：

```powershell
./scripts/dev/verify-ai-source-native.ps1 -MySqlBaseDir D:/mysql/mysql-8.0.46-winx64 -BashPath D:/Git/bin/bash.exe -RunCount 1 -ReportsOnly -ReportPattern '^TestIntegrationAIReportDossierStore$'
./scripts/dev/verify-ai-source-native.ps1 -MySqlBaseDir D:/mysql9/mysql-9.7.0-winx64 -BashPath D:/Git/bin/bash.exe -RunCount 1 -ReportsOnly -ReportPattern '^TestIntegrationAIReportDossierStore$'
```

脚本只创建临时回环实例；专项使用随机隔离库、合成来源和测试批准。预期精确1根、2隔离级别、42具体场景通过，零跳过；父脚本还运行原启动/迁移检查，退出0且停止清理后才计通过。新增独立只读账号实际回取两版、逐表撤权/恢复及越权拒绝，以及版本目录分页/撤权/未知结果/扫描预算；读取权限预检不代表结构/触发器检查或正式进程验收已完成。测试身份替身不证明JWT/原密码登录；锁互斥以另一真实连接收到MySQL1205证明，时间跳变和提交确认丢失是故障注入，须与真实执行层分开记录。

四张新表保存候选版本/依据及独立发布/会员读取许可；导入不自动发布，两种许可默认关闭。当前仅有隔离本地读取接口，没有生产工厂或批准管理入口，不将测试登记语句用于线上。受限账号、结构自检及实际H5读取已有分层证据，生产批准变更审计仍待实现。已有摘录比较不覆盖全部档案版本语义，A→B→A的存储通过不等于语义核验或事实更正完成。

结构保护专项将上方选择器改为`^TestIntegrationAIReportDossier(Store|Schema)$`，仍串行运行两个MySQL版本。Schema根组精确12个场景通过，Store当前42个；失败日志不得替换为成功证明。`AIDossierSchemaVerifier.Verify`由准备账号执行实际回滚探针，原读取账号只执行`VerifyReadAccess`，不为通过探针而给读取账号增加写入/TRIGGER权限。探针不保留测试行，但来源表自增序列可能消耗编号。本地启动装配已接线，仍不能把历史日志当作当前运行进程的启动批准。

适用：内部授权资料登记、快照导入/采集、L0 抽取与人工核对。默认不调用模型；只有单独配置内部批准门禁后，`extract` 才可能调用付费 Provider。本工具不接积分收费、不开放公众 AI 报告；GVA 界面不参与审核写入。

外部 Agent 使用独立的 `cmd/ai-agent`，不复用 `extract` 的无工具契约；要求与限制见末节。未配置、未批准时不运行真实模型。

## 会话助手当前交付边界（2026-10-26）

### 本地装配启动预检（2026-10-29，首个增量）

真实实际H5专项复用下文完整HBuilderX构建与回环静态服务；在受控父Node的`startChainVerification`选项中设置`processes:true,processBrowser:true,conversation:true,analysisType:'light'`（或deep），并提供`origin:'http://127.0.0.1:<完整H5静态端口>'`和原工具路径。父进程固定选择精确`TestIntegrationAIReportLocalAPIWorkerLiveH5`，只传本轮临时桥会话；不得同时设置替身H5专项标记。每次单独新建隔离库，计划在任务创建前声明十五分钟预算，浏览器交互上下文十八分钟，模型单次网络时限仍三十秒；这不延长原队列RunBatch固定五分钟上下文，超出原执行窗口仍须失败保留证据。控制页明确提示真实社区V2及付费模型，没有“允许测试Agent完成”按钮，`/__test__/complete`返回404；不得用替身开关、修改数据库或重交任务催出结果。

从Go日志的本轮`/__test__/`入口读取临时口令至内存，按原表单先验错误密码、再登录会员一，创建会话、选择轻深技能、查询合成题材、保存原报价意图并重新核对/确认冻结，等待弹窗实际隐藏后打开关联任务及公开SSE。真实报告须从页面读取两次，核对章节与引文；第二会员原表单登录后应无他人会话/任务，旧页私有态清空。最后在控制页结束，等待原Go根组/密码桥/worker全部成功及父脚本清理。`local-chain-receipt.json`须verified=true、go_exit=0、browser_verified=true、process_h5_verified=true及H5进程scope，并有独立`TM_STOCK_LIVE_PROCESS_H5_VERIFIED`；process_http_verified可以为false，因为它是另一个入口的证据。只有HTTP标记、页面成功提示或Go零用例均不构成此项通过。实际供应商/用量以同轮父桥回执为准，未知保留失败原记录，不自动重跑。仅合成资料和测试积分，不部署线上、不授予生产批准。

本地受控模型桥可由环境`TM_AI_LOCAL_MODEL_SESSION`注入本轮随机64位小写十六进制临时会话；空值继续使用原无认证回环协议，非法值在数据库连接前拒绝。格式校验不证明会话来源，也不授予真实调用批准。真实供应商Key须仍由独立父桥保管，不交给Go进程；执行JSON不接受Key/会话字段，原无密钥工厂继续拒绝cfg.APIKey。临时会话不进入执行档案摘要，全部原准入、资料、预算、审计、发布复核继续生效。实际worker的session场景须验证三次带认证模型请求、六审计、一次结算和输出无会话明文；默认网络边界仍限字面回环。

需要真实HTTP进程专项时，受控父Node通过`startLocalChain`构造一次性桥后，调用`startChainVerification(bridge,{powershell,mysql,bash,goBin,gccBin,processes:true,conversation:true,analysisType:'light'})`（或deep）。此模式自行编译原密码桥，不接收旧loginRunner；只运行精确`TestIntegrationAIReportLocalAPIWorkerLive`，不需要H5源。真实调用仍须独立liveagents标签和显式本地单任务标志。父进程只传临时模型/Agent会话，Go原密码HTTP路由签发会员JWT，实际API创建会话意图、独立worker扫描队列，原终检和测试账本结算；报告/意图重读、终态SSE及跨会员拒绝沿原接口验证。测试计划在创建前声明15分钟任务预算及较大的令牌上界，模型单次网络期限仍30秒，不因此改生产或旧用例时限。回执scope为local_http_process_conversation_async_real_v2_test_ledger、browser_verified明确false，必须另有TM_STOCK_LIVE_PROCESS_HTTP_VERIFIED证明，不能拿旧会话日志替代。模型失败或未知不换键重交，收尾仍须等待原进程、保留实际用量及失败回执。

实际API与独立worker同库验收使用原生脚本`-ReportsOnly -ReportPattern '^TestIntegrationAIReportLocalAPIWorker$' -RunCount 1`，分别指定MySQL8/9程序目录。先编译实际cmd/api、cmd/ai-worker和原GVA密码登录桥，再在每个新建隔离库执行轻深用例；原密码HTTP登录签发JWT，两进程用不同逐表受限账号，API会员资料只读id/status/deleted_at，固定元数据过程只授EXECUTE，不授DDL/TRIGGER、密码、Agent存储或调用审计写权限。API深度开关仅在本轮有深度计划时启用。用例先从会话报价、准备/确认并核冻结，再启动实际worker扫描队列，验证报告终检、一次结算、报告复读、成功后意图回执及SSE终态续读；跨会员与匿名拒绝，原账本审计通过。CI须1根/2级别/4叶用例/0跳过；隔离级别指准备及验收仓储，实际进程沿用自身事务设置。模型与Agent均为字面回环替身，HTTP密码登录不是H5表单操作，终态续读也不替代执行中断网、浏览器或真实模型验收。

实际H5接同一组正式进程时，先按本手册官方HBuilderX步骤完整构建到仓库外目录，再用字面回环静态服务提供产物。设置`TM_TEST_AI_H5_LOCAL_API_WORKER=local-fixture-only`、`TM_TEST_AI_H5_LOCAL_DEPTH=light`或`deep`、`TM_TEST_AI_H5_ORIGIN=http://127.0.0.1:<静态端口>`，运行原生脚本`-ReportsOnly -LocalAPIWorkerBrowser -ReportPattern '^TestIntegrationAIReportLocalAPIWorkerH5$' -RunCount 1`并指定MySQL及Git Bash目录。缺显式入口、未知深度或混用真实/其它浏览器专项均拒绝，不允许零用例通过。程序自行编译API、worker和原密码桥，在新库补齐合成题材目录字段；业务请求只转发原API，不在网关执行任务、拼装搜索结果或改写报告/SSE。

从日志给出的`/__test__/`打开本轮账号说明；随机测试口令只在内存/表单使用，不输出到日志。先核错误密码拒绝，再原表单登录会员一，创建会话、选择本轮轻深技能、查询“合成”、报价7/30测试积分、保存/复核/二次确认，打开关联任务。此专项的新任务计划预先声明五分钟预算，交互上下文最多十分钟；原自动化三十秒任务和模型/Agent网络时限保持原值。回环Agent先报告running，页面连上公开SSE后从控制页“允许测试Agent完成”；该操作仅改变协议替身的返回，不调用任务执行核或改数据库状态。报告须从原公开API读取至少两次，切换/清理登录态后原页不得保留私有报告，第二会员须原登录且会话/任务为空，再点“结束并核验”。结束会亲查唯一准入/关联消息、原五阶段、一次提交/三次模型请求/六审计、一次报告和结算、冻结0及原账本审计，并核原登录时间落库。成功后停止队列、API、密码桥并清理临时库；浏览器标签和静态服务另行关闭。此模式仅提供回环Agent/模型证据，不能替代真实多Agent、SSE故障恢复、物理手机或生产批准。

本地常驻队列入口在server目录运行`go run ./cmd/ai-worker run-local -config <启动文件绝对路径> -plans <计划文件绝对路径> -execution <执行文件绝对路径>`。必须显式提供三份文件、回环隔离DSN和环境TM_AI_LOCAL_AGENT_BEARER；没有默认档案或真实模型密钥回退。执行文件字段/限制见design.md 12.5，仅synthetic-loopback-only；监听/批量/扫描间隔仍用版本1启动文件。先执行原报告深度、调用审计、Agent存储保护自检，核实际模型/Agent摘要与每条计划，再进入两原队列。就绪标记不代表报告成功；错误/未知结果退出后按原键核查，不能自动换键重提或释放。

显式追加`-stop-on-stdin`才启用管道停止，写入stop行或关闭标准输入请求收尾；正常管道停止退出0，父上下文取消按130，执行/配置错误按1，均不代表会员任务已取消或远端已停止。不开此标志时使用原系统信号上下文；操作系统信号需独立验收，强杀/自然租约专项见下。执行账号仅需8账本、任务/事件、预算/预留、阶段结果/检查点、准入、调用审计、报告提交及Agent档案/任务/事件等20表逐表DML；实例标记、题材、来源/快照/正文等5表只读，不授DDL/TRIGGER，禁止修改来源或标记。程序不创建账号、迁移或修复结构。

人工核查本轮已有任务，在server目录使用`go run ./cmd/ai-worker resume-local -config <同实例启动文件绝对路径> -plans <同源计划绝对路径> -execution <同源执行文件绝对路径> -user <内部会员编号> -task <已有任务编号>`；继续使用相同回环受限DSN及环境Bearer，不提供stdin停止标志。它先做与常驻命令相同的自检，拒绝当前排队/不存在/非本人/计划外目标，原运行租约未到期仍拒绝，不能改库时间、改围栏、缩短旧租约或换键催跑。0仅表示原任务已确认终态（含失败/取消/超期），仍通过原报告API核可读；2为仍未知，1为拒绝/未完成，130为本地中断。未知保留原预留及冻结，不能据退出码自动退款或重提。

强杀复现入口：`scripts/dev/verify-ai-source-native.ps1 -MySqlBaseDir <已有MySQL程序目录> -BashPath <bash绝对路径> -ReportsOnly -RunCount 1 -ReportPattern '^TestIntegrationAIReportLocalWorkerRecovery$'`。根组直接构建实际worker，按RC/RR×轻深×final/agent/model/queued串行新库验收；final只用本轮SQL空区间锁阻塞最终报告事务，并从performance_schema核真实锁等待后强杀；回滚测试锁后确认旧事务无报告/结算，再等原一分钟租约自然到期，用实际resume-local回放完整回执并核一次报告/结算及旧围栏拒写。agent/model在原远端running或模型调用前审计已保存时强杀，恢复只沿原键核查，不补造原围栏缺失的Capture；queued验证恢复命令不领取排队任务，另核原取消终态。该专项需测试初始化账号可读取本轮performance_schema锁信息，实际worker账号仍无此权限；网络只用回环替身，不代表真实多Agent、OS信号或浏览器恢复验收。

上列RC/RR指准备、创建及验收仓储的隔离级别；实际worker进程沿自己的默认事务设置，最终提交测试锁固定使用RR以阻塞空索引区间。父Go测试带-race，子进程按实际命令普通构建；不能把这组标签解释成实际worker分别强制采用了两种隔离级别或带竞态插桩。

系统信号复现使用相同脚本及显式参数，将ReportPattern改为`^TestIntegrationAIReportLocalWorkerSignal$`；该根组按RC/RR准备与验收仓储×轻深×idle/agent/model共12场景启动实际worker，不启用-stop-on-stdin停止参数。Windows使用本轮新建隐藏控制台，发送器核该控制台只包含自身及worker，再对worker的新进程组发送Ctrl+Break；实际worker退出130后宿主才转交130和固定退出证明，父进程还核原worker阶段5停止路径。Unix直接向本轮子PID发SIGTERM；只完成跨平台编译不能写成该平台已运行。所有场景核无额外外呼、报告/settle、释放或会员取消，保留原冻结及远端running，完成原账本对账；不把本地退出说成远端已取消。

小屏复验继续使用上方正式进程H5入口，已有页改尺寸后必须从DOM核innerWidth/innerHeight，不能只信工具set返回成功。本轮IAB仅在新页初建时应用普通尺寸覆写，已有页通过受支持CDP开发能力设置精确视口；结束清除单页指标覆写并reset浏览器尺寸。先保存未提交草稿，再改尺寸核原文、输入/提交按钮及工具栏实际位置；滚动只从实际阅读区域操作，读取可滚动且有内容的原scroll-view，不能把隐藏页签的零scrollTop当作当前阅读位置。抽屉关闭须等标题实际消失，再核前后位置。截图受浏览器宿主画布裁切时只认完整可见范围，DOM尺寸证据另列；桌面尺寸覆写不等于软键盘、字体缩放、安全区或物理手机验收。最新版小屏记录及限制见tasks.md顶部。

Windows另以`^TestIntegrationAIReportLocalWorkerSignalControl$`验证控制管道收到stop时拒绝发送信号，清理并Wait自己启动的实际worker，退出1且无signal/实际130证明；不把管道指令当成OS信号测试。`TestLocalWorkerSignalTarget`只验零和自身PID在接触控制台前拒绝，绝不发送广播事件。两项均不接用户控制台、不传真实模型凭据；控制台关闭/注销/关机、Linux运行及真机键盘分别保留其验收边界。实际矩阵结果以tasks.md同轮日志为准。

独立进程专项为`-ReportsOnly -ReportPattern '^TestIntegrationAIReport(LocalWorkerProcess|LocalPublication|WorkflowLifecycle)$' -RunCount 1`，显式指定本机MySQL程序及Git Bash目录，逐版运行。LocalWorkerProcess必须1根/2级别/12叶场景/0跳过：轻度、深度、未知模型响应、远端已记录running时管道停止、空队列配置撤回、启动摘要错绑。父测试编译并直接启动实际worker二进制，使用随机受限账号和最小白名单环境；先等就绪再由原创建服务提交任务，子进程通过原数据库扫描发现，不能把指定任务调用worker当常驻进程。RC/RR指创建/验收仓储，子进程使用实际程序默认RC。网络仍为回环替身，没有H5或真实模型。

成功例保持进程两个原扫描间隔再停止，须一次Agent提交/三次模型请求/六条持久调用审计/五阶段/一次settle/冻结0；四节报告重读可用，深度至少六引文。未知响应只能一次模型请求且保留两条调用审计、原冻结及原键；运行中停止不发模型、不结算或释放，远端running记录不因本地退出变成停止。配置撤回和错绑均零上游调用，子进程收尾后再核账本审计。此为独立CLI＋真实SQL/原服务验证，尚不替代原网站登录→实际API/worker合流、真实多Agent或进程恢复验收。

`-LocalReportBootstrap -RunCount 1`包含下述任务初始化全部路径，另显式开启原报告读取开关；真实调用仍为false。实际cmd/api启动须验证深度报告schema并装配同一文件来源的本地发布控制器，仍无worker循环或报告生成。原GVA密码/JWT请求实际API，取消后的本人报告须422、另一会员404、匿名401。最后仍是两任务/六积分流水/零Agent与报告提交；不能用这个拒读测试声称成功报告已生成。命令还须提供MySqlBaseDir与BashPath，不能与报告专项、真实Agent或多轮执行混用；成功停止并清理本轮API/MySQL。

成功读取正例独立运行`-ReportsOnly -ReportPattern '^TestIntegrationAIReportLocalPublication$' -RunCount 1`并提供同样程序目录，分别使用MySQL8/9。必须有一个根组及RC/RR×类型1/2四子例通过、0跳过，父脚本成功且迁移64/64；CI保留精确守卫。该专项在随机隔离库用原创建/worker/终检/账本和新发布控制器验证四节可读、来源/文件撤回拒读、恢复及重放零调用/零账务变化，保留至少一次真实settle及账本审计证据。Agent/模型为回环协议替身，HTTP令牌由测试签发；不替代上节实际API原登录拒读证据、浏览器或真实模型试跑。新读取策略从原任务准入逐一解析，不要求每种类型只配置一个题材；所有当前来源、原完整提交/预算/结算保护继续生效。关闭读取不构造依赖，非本地未注入控制面仍拒绝启动。

同一专项现用新的本地选句/双判定控制器替代测试授权Recheck，并用原AITaskRepo和实际AIReportCallAuditService持久审计；回环适配器由同一原配置构造，实际摘要须与计划一致，名称/时限不同也必须拒绝。每次调用前先对实际Start注入提示、模型、追溯键和输入摘要四种篡改，均拒绝且审计表不增加，再放行原请求；最终只有三次协议请求、六条真实调用前后审计及一次settle。登记后的重复Start须ErrUnconfirmed；两事务并发保存相同Finish及后续原结果重放都只保留一条，不同结果必须冲突。每次Finish期间暂撤来源，仍须成功留证，再恢复来源让原后续核验继续。读取及撤回后恢复不增加调用/审计/账务。

新增迁移20261029_create_tm_ai_report_call_audits.sql只建审计表/保护，不迁移历史、不启动调用；完整客户端迁移核验现64项，原62项均保留。专项须明确确认UPDATE/DELETE、终态补造start及错围栏finish均收到MySQL1644；表结构仅含身份/摘要，无原文或错误字符串。调用前存证还要求原预留/当前租约/步骤和原准入，不能把审计接口直接当成授权器。正式常驻worker命令尚未接入，审计schema启动正反例关口见下节；本专项不代替实际API到独立worker进程或新浏览器、真实模型验证。单测覆盖提交确认丢失后拒绝重发、保存失败传播、结果幂等/冲突、取消上下文及旧租约/来源撤回后留证；不将测试故障注入当作真实进程崩溃验收。

审计启动预检使用server目录下`go run ./cmd/ai-worker check-local-audit -config <本地启动文件绝对路径>`，仅以环境TM_DB_DSN注入本轮隔离连接。该命令是DML回滚探针，不是只读：账号须对8张新积分账本表、任务/事件、预算/预留、阶段结果/检查点、首次准入及调用审计等16表持SELECT/INSERT/UPDATE/DELETE，对tm_ai_local_instance只持SELECT；不授DDL/TRIGGER或mysql.user读取。先核配置与实例再自检，不创建/修复结构。输出仅阶段编号或有限核验成功，不输出DSN、口令、内部SQL错误。成功不启动常驻worker或真实调用，规划与实际执行配置仍须另核。

自动化执行原生脚本`-ReportsOnly -ReportPattern '^TestIntegrationAIReport(CallAuditStartup|LocalPublication)$' -RunCount 1`，分别指定MySQL8/9与Git Bash程序目录。新Startup根组要求RC/RR各17例，共34例，0跳过；缺表、CHECK/主键/外键、错排序规则、缺失或同名失效触发器/约束全部拒绝。有效例创建逐表受限账号实际运行worker命令，验证无触发器元数据可见权限仍通过；改成同名失效删除保护须阶段4拒绝，恢复后通过，DDL/mysql.user读取须明确1044/1142/1143。前后核对全部8张账本及相关任务、准入、审计表行数，确认回滚无业务行残留；数据库自增计数可能因探针推进，不将行数回滚描述为所有实例状态不变。相关CI新增1根/2隔离级别/34子例/0跳过守卫；常驻命令、运行中取消及新浏览器/真实模型仍另验。

`-LocalTaskBootstrap -RunCount 1`在新建临时实例内执行原迁移、合成计划和原会员初始化，先用原PointsService给两名测试会员各发50测试积分，再显式开启实际API的任务创建/深度/意图/取消/余额/SSE开关。原GVA密码路由签发JWT，HTTP测试依次核轻深报价、错误价格/未确认/自填金额拒绝、prepared不冻结、确认冻结、同键重放及原意图恢复、消息任务关联、跨会员读写拒绝、queued取消一次释放，以及取消后再确认仍指向原任务。终态SSE分别从0/1/2游标与同值Last-Event-ID读回2/1/0事件；这是终态续读的真实HTTP证据，不替代执行中断网/代理或原H5页面验收。

API使用逐表受限写账号以保留所有原保护探针；会员仍只读三列，无DDL/TRIGGER，不读密码、不改来源/正文。worker预检另用仅SELECT账号复验同一计划。脚本还验证缺意图元数据过程阶段2拒绝、同名无效删除保护阶段3拒绝、恢复后实际API重新就绪；同库原非本地核验器也执行回归。最终应为两任务/两关联消息/四进度事件，两笔测试发放、两笔冻结和两笔释放，两个测试账户各50可消费、冻结与欠款0；Agent/报告提交/供应商预约均0。所有数据与配置成功后清理，失败停止实例保留诊断目录。真实调用仍强制false，报告/问答未启用；本模式不算真实多Agent→成功报告结算通过。

另准备第三个意图后变更本轮计划文件版本，确认须经原键核对后返回not_created/admission_unavailable，且任务/消息/冻结均不增加；恢复原文件后重放同一意图仍拒绝，不自动补提。原控制器曾将文件拒绝当作通用错误，可能滞留unconfirmed；现已在读取前后统一分类为创建前准入不可用，保留取消上下文和实例错误。最终意图为两条linked、一条not_created。此反例只修改本轮同实例、synthetic-local-only计划文件，退出路径恢复原字节；不是未知模型调用的自动释放策略。

`-LocalPlanBootstrap -RunCount 1`在本轮新建专属库和全部迁移后，由受控Go测试子进程建立明确标注的合成题材/来源/快照及无密钥计划文件。测试子进程在stdin取得本轮库和文件路径，拒绝已有计划文件或非空业务库；调用原API的创建服务装配，核轻深报价、无积分创建拒绝、来源撤回/恢复及原版本文件变价拒绝，检查零任务/账户/积分流水。此处验证服务装配，不是原登录表单或任务HTTP提交闭环。

随后实际worker执行`check-local-plans -config <启动文件绝对路径> -plans <计划文件绝对路径>`，用仅SELECT本地标记、题材及来源/快照/正文等所需表的账号重验同一文件和当前库；撤回正文表SELECT须明确失败，恢复后再次通过。API通过TM_AI_LOCAL_PLANS读取同一文件启动，任务/模型/报告开关仍关闭；继续核healthz/目录查询及任务入口404，禁止来源/正文修改。这个子命令不做全业务schema检查、不启动队列、不构造模型、不写积分，成功不等于正式批准或可执行任务服务。模型/Agent配置摘要是显式合成元数据，本轮没有实际Provider身份探测。真实调用仍由版本1启动文件强制false；不得用计划文件替代上线前的定价、资料许可、质量/备案和发布批准。

`-LocalConversationBootstrap -RunCount 1`显式开启任务读取和会话管理，复用同一随机签名配置：GVA原bcrypt登录路由签发JWT后，直接请求实际cmd/api。验证双会员会话创建/列表/详情/回执/消息空态、同键重放和冲突、改名版本冲突及归档、跨会员读写404、匿名/错误令牌/停用/软删除401；关闭任务创建（POST为405）及余额入口（404）。测试登录调用原路由夹具，AI请求走实际HTTP进程；本模式没有H5浏览器或常驻登录服务，不作为完整助手/真实多Agent链路的最新验收。

会话运行账号仅获两张会话表的SELECT/INSERT/UPDATE/DELETE（删除和改消息权限用于原保护探针获得明确触发器拒绝，并非允许绕过保护），任务/意图只读及会员id/status/deleted_at列只读；不读密码、不改会员、不写任务或账本、不持TRIGGER/DDL权限。初始化所有者建立固定无参、READS SQL DATA、SQL SECURITY DEFINER过程`tm_ai_local_conversation_trigger_metadata`，仅返回本库两张会话表的触发器身份字段，运行账号仅获该过程EXECUTE；不暴露定义正文或动态SQL。MySQL的DEFINER视图实测不能继承information_schema.triggers可见性，因此不采用视图或静态快照伪装实时核验。启用本地核验器仍须先通过实例标记，全部原CHECK/索引/外键元数据及真实正反例探针保留；未配置本地文件的原核验路径不变。启动错误仅公开阶段编号。

脚本在停止API后由初始化所有者制造两项结构反例：缺固定元数据过程须在阶段2拒绝；同名无实际保护的删除触发器须在阶段3拒绝。恢复原定义和EXECUTE授权后重新启动实际API成功，核对仅两条本轮测试会话/零消息/零任务/零积分流水，最后清理本轮实例。MySQL8/9日志分别为`%TEMP%/tm-stock-local-conversation-guards-mysql8-20261029.log`与`%TEMP%/tm-stock-local-conversation-guards-mysql9-20261029.log`。只证明本地会话管理装配，计划/资料许可/发布批准、任务提交/报告/问答工厂和常驻worker仍未接完；真实调用继续显式false。

`-LocalMemberBootstrap -RunCount 1`在API初始化模式上加入原会员验证：构建GVA原路由测试程序，只有回环专属库名及唯一实例标记同时通过才允许原AutoMigrate初始化会员表；签名配置和本轮DSN从stdin传入，子程序不读取产品配置。复用原bcrypt登录/JWT/资料接口，检查错误密码、两账号身份和登录时点落库，再检查错误标记拒绝并恢复原标记。随机测试口令/JWT不输出。本模式仍属于隔离夹具初始化，不是生产账号或登录服务；尚未验证GVA令牌与实际API会话路由互通，AI入口仍404。会话启动探针还检查触发器元数据，不能为让探针通过就给运行账号TRIGGER/DDL权限；迁移/结构核验与运行权限须继续分离后再开放会话写入。

`-LocalAPIBootstrap -RunCount 1`在原初始化模式上继续执行全部21项原迁移、创建受限只读API账号并实际启动cmd/api，验证healthz、空题材目录查询和默认关闭的AI任务/会话/余额404；最后核对零任务/零积分流水并关闭本轮进程。仍只在本脚本新建的MySQL/数据目录运行。上游两表复用verify-migrations.sh的最小测试DDL，不复制其中示例证券行，它们不是生产schema权威；迁移按StringComparer.Ordinal排序。只读API账号仅获实例标记、题材/股票/关联和两个日数据表的SELECT，拒绝INSERT、改标记、DROP和mysql.user查询，必须得到明确权限错误。此模式不含会员/收费任务、报告发布控制器或长期运行服务，不把健康检查通过当成完整AI助手可用。

受控初始化验收入口：`pwsh -NoProfile -File scripts/dev/verify-ai-source-native.ps1 -MySqlBaseDir <本机MySQL目录> -BashPath <本机bash.exe> -LocalBootstrapOnly -RunCount 1`（Go须在PATH）。它只启动自己新建临时数据目录中的MySQL，随后新建随机专属库、实例标记及随机口令的核验账号；不接受外部DSN或已有数据目录，不执行`IF NOT EXISTS`接管旧库。核验账号仅获标记表SELECT权限，实际worker以最小子环境运行check-local-db；不继承root、MYSQL_PWD、TM_TEST_DSN或宿主模型凭据。正例后检查重复建库错误1007，以及删除/更新标记、建表、读取mysql.user的权限错误，再重复worker核验，禁止把SQL/连接错误当作权限测试通过。成功后关闭且清理本轮实例和配置；失败停止实例并保留诊断目录。这是初始化与权限验收模式，尚不提供持续运行的业务服务或业务账号权限。

后续增量增加`check-local-db -config <绝对路径>`：使用环境DSN打开数据库，以5秒超时的只读RR事务检查`DATABASE()`与`@@server_uuid`，以及`tm_ai_local_instance`唯一标记。标记字段为instance_id、database_name、server_uuid、environment和contains_real_users，必须与当前文件/连接一致、local-isolated且真实用户标志为0；无表、空表、额外行、错库/实例/服务或取消均拒绝。API本地模式在注册业务路由之前执行同一核验。核验不自动建表、插行或修复，不将该标记当作许可/生产批准；持有数据库写权限的管理员仍能伪造标记，因此它只是受控本地初始化流程中的防误连检查，不是生产数据隔离的独立安全证明。初始化器、受限运行账号、业务结构及控制器装配继续待办。

用户已选择受控本地配置文件方案。版本1文件精确字段和约束见design.md 12.5；文件不保存DSN或密钥，必须是普通文件的绝对路径，上限8192字节，拒绝符号链接、读取时文件身份变化、未知/重复/大小写别名字段、null或尾随内容。`real_calls:false`必须显式出现；当前版本不能启动真实调用。

`TM_AI_LOCAL_CONFIG`未设置时保持原默认行为。设置后，API在连接数据库前核对文件与环境变量`TM_DB_DSN`，并使用文件内字面回环监听地址；创建/报告/问答的缺控制器启动保护仍然生效，不因有文件就放开。实例库名必须与32位小写十六进制instance_id匹配为`tm_ai_local_<instance_id>`，环境DSN必须指向该库的字面回环TCP端口且开启parseTime，不能套用现有生产库。

在server目录执行`go run ./cmd/ai-worker check-local -config <本地配置绝对路径>`可做无数据库连接的预检；成功输出明确说明尚未验证数据库所有权、结构和批准。实际命令正例及跨库反例已有验收，此子命令不启动队列、迁移、模型或任务，不能当作本地服务已运行。数据库隔离标记、计划/批准控制器及完整API/worker入口仍在开发；不要将配置示例或成功预检迁为生产批准。

### 受控本地反馈管理进程

`cmd/ai-admin`也接受上述`TM_AI_LOCAL_CONFIG`。本地模式只注册三个原反馈内部端点，不注册来源管理、审核、抽取或公开会员路由；必须显式设置`TM_AI_FEEDBACK_ADMIN_ENABLED=true`、保持`TM_AI_L0_ENABLED=false`。配置中的监听地址覆盖原管理监听，须为字面回环。环境变量`TM_DB_DSN`单独注入同一隔离库的管理运行账号，`TM_AI_ADMIN_REVIEWERS`仍按原内部身份绑定规则注入，不写入文件或前端变量。未设置本地文件时原管理装配不变。

启动先核验同库唯一实例标记，再运行原反馈结构回滚探针，全部通过才监听；不自动迁移或补造实例标记。初始化所有者须事先执行反馈迁移。运行账号的逐表权限由探针与业务决定：18张账本/任务/准入/报告/反馈表具备原回滚探针所需DML，共用来源/实例标记等表仅SELECT，不授会员密码读取、Agent执行或DDL权限；具体表清单见`localFeedbackAdminDSN`与`localProcessDSN`。运行账号不能替代初始化所有者；标记只防误连，不能作为生产批准。实例核验在启动时完成，不声称运行期自动撤回批准。

自动化复验在仓库根执行原生脚本，传本机MySQL及Git Bash目录，选择`-ReportsOnly -ReportPattern '^TestIntegrationAIReportLocalFeedbackProcess$' -RunCount 1`，分别验证8/9版。根组编译实际API、worker及管理程序，使用原会员密码HTTP登录，在RC/RR各生成轻深报告；再经实际会员API提交/重复提交，独立管理进程查询/处理/重放，会员读取原结果。检查错误身份、跨会员、跳级/竞争终态及多余正文拒绝、来源撤回后报告拒读而反馈可查、三条不可变事件及零额外模型/积分流水。Agent和模型仅回环协议替身，不是本轮真实推理或会员浏览器证据。

后台浏览器复验须先按后文整套GVA隔离构建流程准备静态前端；仅一次性测试进程设置`TM_TEST_AI_LOCAL_FEEDBACK_CONSOLE=local-fixture-only`及`TM_TEST_FEEDBACK_UI_ORIGIN=<完整GVA的127.0.0.1 HTTP源>`，清除真实调用标志。原生入口选择`-ReportsOnly -LocalFeedbackConsoleBrowser -ReportPattern '^TestIntegrationAIReportLocalFeedbackConsoleH5$' -RunCount 1`。从日志控制页取得当轮账号，在原登录/角色界面保存菜单及三API权限，处理人由侧栏处理原MySQL受理单至版本3，核对三条历史；普通后台账号旧入口应被拒。最后控制页结束，还须检查根组PASS、父脚本迁移64/64及临时实例清理。原人工子程序仍8分钟；超时保留失败记录，不延长时限或重写终态。后台身份/目录使用既有隔离SQLite夹具，反馈/报告/账本在实际MySQL；生产GVA MySQL权限由另一专项覆盖，不能混为本轮浏览器证据。会员提交这一段是原HTTP路由，新装配下会员H5反馈仍待独立验收。

### 受控本地资料问答与报告追问

实际`cmd/api`新增可选`TM_AI_LOCAL_QA=<仓库外普通文件绝对路径>`，必须同时配置版本1隔离运行文件及原问答Read开关；未配置时生产三个可信工厂仍为空，误开提交继续拒启动。文件上限32KiB，复用原严格文件读取及JSON重复/精确键检查，拒相对路径、符号链接、未知/缺少/null字段及读取期间身份变化。版本1字段以`config.AILocalQA`为准：`version=1`、同一`instance_id`、`revision`、`scope=synthetic-loopback-only`、毫秒`issued_at_ms/expires_at_ms`（不超过24小时）、升序唯一`members/subjects`（各1至16项）、三个固定顺序模型、`allowed_domains`及`limits`。三模型依次用于选句、蕴含、语义判定；元数据审批引用均须`local-eval:`前缀，配置不提供密钥字段。

每个模型的`endpoint`只含`name/base_url/model/timeout_ms`，地址必须为无凭据、参数、转义路径的字面127.0.0.1 HTTP数字端口1024至65535，路径只允许空、`/`或`/v1`；不读取宿主供应商Key，不经过外部代理、重定向或自动重试。`profile_digest`须匹配原Direct配置身份；构造校验不发网络请求，不代表远端服务获生产批准。限额字段为来源数、句数、证据/提示字节、输入/输出/总token及毫秒时限，沿用原问答核边界，时限1至20000毫秒，先限幅再做Duration换算。资料范围用`QAConfig().PolicyDigest()`登记到原`tm_ai_qa_scopes`，仍需原快照/许可/范围证据和`tm_ai_qa_limits`显式启用；文件本身不创建或更新这些批准表，也不迁移积分。

运行账号除既有API权限外，还需五张问答表的原回滚探针/业务所需逐表DML，以及固定无参`tm_ai_local_qa_trigger_metadata`的EXECUTE。由初始化所有者建立SQL SECURITY DEFINER、READS SQL DATA过程，固定筛选当前专属库这五张QA表的触发器，按顺序仅返回`trigger_name,event_object_table,event_manipulation,action_timing,action_statement`。QA自检仍比对所有触发器的完整规范定义及实际回滚反例；会话触发器复用原只读元数据过程。不能用返回常量的过程、同名空触发器或放宽TRIGGER权限绕过。删除重建过程后须重新授予EXECUTE，不能将权限错误误计为定义拒绝。无此过程、定义伪造或实例不符拒启动，原非本地验证路径不变。

`TM_AI_QA_READ_ENABLED`、`TM_AI_QA_SUBMIT_ENABLED`及`TM_AI_REPORT_QA_SUBMIT_ENABLED`仍分别控制读取、资料提交、报告提交。本地文件变更/过期或实例撤回后历史答案也失败关闭，不自动补交未知问题；只读装配不给控制器ForPlan权限。报告追问只从当前本人报告读引文，不扩检资料库、公司事实行或网络。新报告仍走原报价确认/任务/终检/结算，问答没有积分扣费路径。

原生复验：分别指定MySQL8/9程序和Git Bash路径，运行`verify-ai-source-native.ps1 -ReportsOnly -ReportPattern '^TestIntegrationAIReportLocalQAProcess$' -RunCount 1`。原密码HTTP登录、实际API与worker先生成轻深报告，再在同一会话提交资料问答与报告追问、复读和原键重放，核对受限账号的完整元数据正反例、逐次调用存证、来源/文件撤回拒读、跨会员拒绝及零额外账务。模型只用回环替身；须父脚本成功、RC/RR×轻深四叶通过、迁移64/64及临时实例清理，不能用该HTTP专项替代新装配的实际H5或真实模型证据。

### 新账本余额只读接口

`GET /api/v1/ai/points/balance`使用原Bearer会员鉴权，只返回本人`availablePoints`（当前有效批次可消费额）、`frozenPoints`和`checkedAt`（数据库毫秒时点）。不接受查询参数或请求体，成功响应禁止缓存。无账户返回零余额，不自动创建账户；查询失败返回503，客户端必须显示暂不可用，不能将错误当零余额或回退旧资产。

`TM_AI_POINTS_BALANCE_ENABLED`独立且默认关闭，仅接受`true/false`；开启依赖任务只读及有效会员鉴权配置，并检查账本结构。开发付费放行不隐式开启它。用户于2026-10-29确认会员页“我的积分”显示新账本可消费余额，冻结额单列，旧积分不迁移、不合并。接口不可用显示“暂不可用”，不得回退旧资产；积分入口也不再导向旧资产页面。

隔离验收：运行`verify-ai-source-native.ps1 -ReportsOnly -ReportPattern '^TestIntegrationAIReportBalanceRead$' -RunCount 1`，并提供本机`MySqlBaseDir`、`BashPath`。脚本也运行`TestIntegrationAIReportReadStartup/points_balance_auth`：检查正式注册、真实JWT校验与数据库会员状态、本人/另一会员隔离、无账户不建账和关闭路由404。JWT由测试签发，此结果不替代原会员登录页面到余额接口的浏览器联调。

本手册后续包含会话、问答、任务、报告和反馈的本地隔离验收。各专项的当前通过范围以`tasks.md`的C表和对应日志为准；早期段落中的“未实现”是历史阶段说明，不应直接据此启动或发布当前程序。隔离网关复用实际业务服务、原会员登录和临时MySQL，但它不是可公开部署的常驻应用入口。

| 待办类别 | 当前证据与限制 | 放行前需要完成 |
|---|---|---|
| 正式API与执行进程装配 | 显式本地配置已装配创建准入及报告读取控制器，限同实例/合成资料；非本地仍无默认可信控制面。`cmd/api/ai_qa.go`的三个可信工厂没有非测试赋值，`cmd/ai-worker`已有显式run-local本地常驻装配，限同源合成资料及回环协议服务 | 继续验证原登录到实际API/worker成功交付及独立进程恢复；生产另需可核验的定价、资料许可、模型/质量/预算与发布批准。不能只改环境开关 |
| 默认关闭 | Agent、任务创建、深度提交及问答读写各有独立配置校验；空配置关闭，依赖缺失拒绝启动 | 对实际部署环境逐项核对配置和启动结果。代码默认关闭不是已读取生产环境的证明；禁止把本地测试标志或测试凭据带上线 |
| 资料与服务准入 | 已有测试资料、固定上游及真实模型隔离试跑记录；这些不授予生产资料、模型或价格批准 | 完成仓库既定的资料使用、上游分层许可、模型/质量/预算、正式价格及免费额度审批，保留可核验记录 |
| 数据库与身份权限 | MySQL8/9及旧L1增量升级已有隔离证据；原GVA菜单/角色与会员登录另有专项 | 在生产副本演练迁移/备份恢复、实际受限账号、JWT配置及后台角色授权；不套用测试root或清库脚本 |
| 浏览器与运维 | 原H5链路、SSE异常和反馈已有分项证据；物理手机键盘等移动组合仍未齐 | 补完C-8移动体验；生产代理SSE缓冲/超时、容量、监控、停机恢复与对账均须独立验证 |
| 发布 | 本目标仅授权本地开发验收 | 上述条件满足后按WORKFLOW取得人类上线确认；当前不部署、不扣真实用户积分 |

上述“正式装配”是尚需工程实现和验证的项目，不应记成“已有功能，只缺密钥”。本地链路验收仍按下文原入口复现，不能以生产入口故意失败关闭替代功能试跑，也不能以本地通过声称生产已可用。

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

桥的状态发布须晚于`async-job.json.new`写入及原子替换成功，不能先改内存终态再等待文件；写入失败保留最后已确认非终态，结果接口仍拒绝交付，不允许再次提交。关闭须等待本轮运行及日志写入收尾后再关闭监听/清理目录。无Key守卫命令为`node --test scripts/dev/tradingcn-live-gateway.test.mjs scripts/dev/tradingcn-local-chain.test.mjs`：三个成功/失败/未知路径在终态写入前用受控文件系统暂停，HTTP与原日志均须仍为running；终态写入持续失败反例同样不得返回终态或结果。受控I/O只用于测试，实际入口使用默认文件系统；原子替换成功不是fsync、整机掉电或生产持久队列恢复证明。

通过受控本地环境将新模型Key载入父Node进程，调用`startLocalChain({credential,source,python})`；不要打印credential、bridge对象或reportEndpoint对象。默认路由为DeepSeek官方；千问Token Plan凭据须显式传`provider:'qwen-token-plan'`，固定连接`token-plan.maas.qianwenaiapi.com/compatible-mode/v1/chat/completions`和`qwen3.7-plus`，拒绝自定义地址。隔离子进程的`deepseek-flash`为协议别名，实际供应商/模型在provider回执中单列，不能据此宣称调用了DeepSeek；仅千问路由翻译关闭思考字段，保留原输出和Go终检。网关要求固定社区提交与原跟踪文件干净，模型Key仅在父内存，Python及Go/PowerShell子进程均不继承产品配置，只接收临时桥会话凭据。`startChainVerification(bridge,{powershell,mysql,bash,goBin,gccBin,origin})`接受本机程序绝对路径，由既有原生脚本新建独占MySQL实例；读取`<output>/local-chain-go.log`里的`/__test__/`地址。最小Windows子环境保留PATHEXT，否则PowerShell会把.exe误作“文档”而无法在管道中执行；日志明确UTF-8。

验收入口必须同时带`integration,liveagents`构建标签、`TM_TEST_AI_LIVE=local-fixture-only`、回环临时桥地址/凭据和实际H5源。原生脚本的`-LiveAgents`只接受`-ReportsOnly -ReportPattern '^TestIntegrationAIReportLiveAgents$' -RunCount 1`；默认套件和CI不编译该付费测试，也不读取模型Key。不要手工改产品开关来替代该入口。内部测试批准/备案/质量/虚构价格仍不是生产证明。

最新会话入口须调用`startChainVerification(bridge,{powershell,mysql,bash,goBin,gccBin,origin,conversation:true,loginRunner})`，其中`loginRunner`为本节“原会员登录”流程编译的仓库外测试程序绝对路径。入口只向最小子环境追加显式会话标志、原意图标志及登录模式/运行器，不继承模型Key；缺项、相对路径、未知模式拒绝启动。未选择会话模式的旧入口仍仅证明独立任务。Go自身亦校验精确单根/count=1及完整配置，拒绝混用问答、深度或流故障专项。

会话模式使用控制页随机测试口令，通过原登录表单验证错误密码及会员一登录。实际新建会话、选择轻度技能、搜索示例题材、保存报价意图（不冻结），重新核价并二次确认一次；从原会话消息进入任务，完成后读取并重读报告。会员二原登录后须无他人会话，再结束核验。Go额外要求独立创建POST=0、会话/Prepare/Confirm/原创建核各1、唯一linked意图及任务消息、公开报告GET至少2次，以及原登录子程序最终通过。只有这些断言及原真实执行/终检/账本断言完成才输出专用证明；父进程退出0、真实provider观测和该证明同时满足，回执才可标记`scope=local_conversation_async_real_v2_test_ledger, verified=true`。没有证明时即使旧独立链成功也不能升级为会话通过。轻度实际通过记录见下，不代表深度会话真实链通过。

深度真实会话使用同一入口显式增加`analysisType:'deep'`，必须同时有`conversation:true`和原登录运行器；未知类型或独立模式下的深度拒绝启动。测试装配复用原L2准入/执行计划、SQL限流器与深度终检政策，不把轻度成功结果改名。浏览器选深度技能，核对30测试积分报价及50测试余额，保存、复核、确认唯一原意图；报告须四节、至少六条与原测试快照逐字一致的引文，读取两次后换账号验证隔离。结束还要求原报告类型2、结算30、余额20/冻结0，并输出独立`TM_STOCK_LIVE_DEEP_VERIFIED`完整行。新回执含`analysis_type`；深度缺少会话或深度证明，即使核心成功及原Go退出0仍为verified=false。历史无此字段的轻度回执保留原样，不能补字段当深度验收。模型和浏览器总时限保持原真实试跑配置，不延长以掩盖失败；实际结果另记。

浏览器选测试会员一，实际页面搜索“示例”、核对7测试积分报价、二次确认创建，进入创建收据对应任务，等待后点击“核对并查看报告”。原Go适配器提交600001仅为格式占位，桥原引擎始终分析非证券TMEVAL；桥固定执行完整原V2图，不声称模拟上游Web的分析师/风险参数裁剪。报告正文必须是测试快照中的四节原句，披露真实`deepseek-flash`及显式test-only编号；原Agent报告只作隔离选句线索。

DeepSeek Chat文档仅保证[JSON对象模式](https://api-docs.deepseek.com/zh-cn/guides/json_mode/)。本地桥显式把json_schema请求译成json_object，并原样附加schema作为系统约束；不改写任何回包，Go严格JSON、枚举/用量/完成原因、原文回取、双独立判定及终检保持不变。三次报告模型调用限额独立，未验证核心前不能使用报告凭据；请求和用量分core/report记录，报告回包不能填补缺失的核心观测。它不是远端strict schema或生产质量证明。

看到报告后回到同一源控制页点击“结束并核对测试”。只有原任务成功、冻结归零、恰好一次原子报告/settle、三个真实报告步骤审计、Agent/模型替身零调用、其他会员404、终态重放无重复结算及积分审计均满足，Go才通过；随后原生脚本仍需启动门禁和当前62项迁移检查通过。执行`await finishChainVerification(bridge,verification)`，检查`local-chain-receipt.json`的`verified=true`、`go_exit=0`、唯一提交/取结果、真实provider用量齐全且无unknown；函数最后清除父模型Key并关闭模型/异步监听。失败也要执行收尾，不能仅等Node内核销毁；未通过的回执不能升级为成功。运行器路径先用本机Get-Command核对；Go启动失败且引擎尚为waiting时记已知未执行失败再签发失败回执，不能把它当作模型运行失败或成功。若已经运行/未知，仍保留原状态与用量，不按未启动处理。

2026-10-23会话模式已按上述步骤取得新实跑证据：`C:/Users/liufm/AppData/Local/Temp/tm-stock-tradingcn-live-RBXnaF/local-chain-receipt.json`为会话scope且verified=true/go_exit=0，原登录、唯一原任务绑定、页面报告重读及测试账本断言通过；20次确认HTTP200（核心17/报告3），unknown=false。本轮是轻度套餐和显著测试资料，完整V2核心并不使它成为深度套餐验收。初次路径错误零调用记录、诊断日志、截图、资源收尾及未验范围见tasks.md同日条目；不据此启用生产调用。

深度首轮`tm-stock-tradingcn-live-yq5FFC`为失败回执：原模型将非连续编号[1,5]及[2,6]合在单条选择中，转换器拒绝，只有1次报告模型调用，未生成报告。保留原回包及冻结释放事件的数据库二进制日志，不为报告重读而重复提交任务。现已澄清选句提示的分条规则并加入原失败形状拒绝/合法分条正例；修复后须用新任务和新摘要独立复验，不能修改旧回包或恢复失败任务。实际结果及诊断资源范围见tasks.md同日条目。

2026-10-23修复后已按此规程重新完成深度试跑：`C:/Users/liufm/AppData/Local/Temp/tm-stock-tradingcn-live-yAOhbt/local-chain-receipt.json`为analysis_type=deep、verified=true/go_exit=0，含独立会话/深度证明；原H5读取并重读四节六引文，原测试余额50→20/冻结0/settle1，会员二及旧标签隔离通过。21次确认HTTP200（核心18/报告3），unknown=false；父脚本41条PASS、迁移62/62后清理成功，四测试监听关闭。该成功记录不覆盖首轮失败，也不改变模型/授权/价格等测试配置不是生产批准的边界。下一次源码或配置改变后须按实际变更重新验收，不能只引用本回执。

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

当前AI页默认进入会话，桌面历史栏与消息/报告双栏、窄屏历史抽屉已接入；资料问答、报告追问及轻度/深度题材任务按各自原接口和开关验收，专题仍未开放。新建先保存独立原键，未知仅GET原收据；损坏存储/缺WebCrypto或WebLocks时拒绝POST，不手工清键绕过。改名、归档及消息变化按当前会话组件同步原列表；归档不能删除历史、取消原任务或退款。原独立任务创建不自动绑定当前会话。最新修改后的浏览器、真库及真实模型证据必须分别核对tasks.md，不把这里的操作说明视为全目标通过。

在仓库外隔离依赖目录按CI固定版本安装Vue/compiler-sfc/server-renderer3.5.13、TypeScript5.7.2、Axios1.8.2、vue-router4.4.3；分别执行`node scripts/dev/verify-ai-report-ui.cjs <依赖目录>`及`node scripts/dev/verify-ai-task-stream.cjs <依赖目录>`。前者执行实际组件/封装与状态隔离，后者执行实际UTS解析器/订阅器及可控流/时钟反例；不把VM宿主当作真实H5。实际编译使用已安装HBuilderX内的官方uni-app x编译器，不另装不匹配的uni/Vite依赖、不修改vendor。

2026-10-26补充订阅器断言：首次收到序号2后断线，原游标重连；连续收到两个重复序号2时，事件回调和状态GET均不增加。随后两个相同序号3完成帧只发一次事件回调和一次独立状态GET，查询确认终态后无剩余定时器或重连。该用例贯穿原解析器与订阅器，区别于单独解析器去重测试；实际HTTP帧复放和浏览器展示仍按后文独立专项验收。

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

### 本地原会员登录与会话闭环验收

不能用测试页写入JWT代替原会员登录。先用当前HBuilderX启动本工程H5；后端测试网关只接`/api/v1/`与三个明确的会员登录/资料路径，`/api/auth.uts`等源码模块必须转交官方编译服务，不能被宽泛`/api/`代理截走。此模式只运行本地隔离MySQL，不读取产品数据库或生产凭据。

1. 在仓库根用`go -C backend/server test -race -tags integration -c -o <仓库外测试程序绝对路径.exe> ./router/client`编译登录测试子程序。普通回归`go -C backend/server test -race ./router/client -run '^TestAIMemberPasswordLogin$' -count=3`覆盖原路由、密码错误/停用/不存在账号、会员令牌主体与资料读取；它不等于浏览器验收。
2. 仅给本次测试进程设置`TM_TEST_AI_H5_ORIGIN`为已启动的字面回环H5地址，`TM_TEST_AI_H5_LOGIN=local-fixture-only`、`TM_TEST_AI_H5_LOGIN_RUNNER`为上述绝对路径、`TM_TEST_AI_H5_INTENTS=local-fixture-only`、`TM_TEST_AI_H5_REPORT_QA=1`。执行`verify-ai-source-native.ps1 -ReportsOnly -ReportPattern '^TestIntegrationAIReportH5Gateway$/^Read_Committed$/^feedback$' -RunCount 1`，同时显式指定MySQL与Git Bash程序目录。子程序仅从stdin接收本次随机库、测试签名与内部控制值，不继承供应商凭据；账号、原登录、报告、反馈及账本使用同一个本次创建的随机MySQL库，测试程序不进入产品构建。
3. 打开日志里的`/__test__/`；本次随机口令只从控制页查看，不抄入日志、源码或聊天。先用原表单验证错误密码拒绝，再登录会员一，完成新建会话→轻度技能→搜索/报价→保存意图→再次核价与确认→原任务→报告→一次追问→一次反馈；控制页处理反馈后，会员显式查询。撤回来源后，原答案/报告须拒读，但反馈仍可查。清理测试登录态后，以会员二通过原表单登录，确认没有会员一的会话、报告或反馈，旧标签清空。此控制页没有注入JWT功能，不测试短信登录、注册或密码修改。
4. “结束并核对测试”还须验证两个会员实际密码登录与资料请求、至少一次拒绝及原`last_login_at`落库，并等待子程序最终PASS和父脚本退出0。原会话准备/确认/创建核各1次、任务/报告/结算唯一、冻结0、追问1条/3次模型调用存证、反馈1条/3事件、账本审计及客户端迁移检查均必须通过。这里只是回环模型，不算真实多Agent验收。

历史追问专项可在独立一轮设置`TM_TEST_AI_H5_QA_HISTORY=1`：问题终态核实后清理本地原键，刷新网页、从服务端历史消息进入原任务/原序号，重新核对报告及答案。必须自然等待原证明的两分钟执行期限经过，不修改时钟、原证明或数据库截止时点；网关从数据库原`proof_json`确认至少一次成功读取发生于过期后，且模型调用仍仅原三次。该标志并不放宽产品执行期限。来源撤回与账号隔离反例仍执行；控制页确认不代替最终进程退出。

跨标签归档同步复验沿用报告追问/原登录/意图模式，不设`TM_TEST_AI_H5_QA_HISTORY`：标签A生成报告并重读，提交一次报告追问，待终态后保留本机原键；标签B在同源原页面打开该会话，通过原确认弹窗归档。回A点击报告追问的“核查本机原问题”，由原saved通知刷新会话；应同时移除会话写入口并在报告追问显示已归档只读。核实终态清理本机原键后仍不得出现新追问输入。随后从原历史消息进入原报告/序号，当前授权有效时仍可读取引文；来源撤回则拒读。反馈受理/处理查询、两会员隔离及结束账本检查照常完成。只在本次可丢弃的测试会话操作，不修改数据库状态来替代第二标签的归档请求；截图和脚本退出码分别留证。

轻深技能切换专项沿用原密码登录、回环H5及会话意图配置，清除报告追问、SSE故障/复放、资料问答及真实Agent标志，精确选择`^TestIntegrationAIReportH5Gateway$/^Read_Committed$/^success$`并仅运行一次。会员一在轻度技能保存原7积分示例意图但不确认，切到深度技能后应明确显示原轻度意图；只读恢复→按原类型重新核价→二次弹窗仍为轻度/7积分，确认并等弹窗隐藏后打开唯一关联任务。须交付四节示例报告，会员二原登录后无他人会话、旧标签清空，结束审计证明Prepare/Confirm/原创建核各1、报告/成功结算各1、冻结0。此浏览器场景验证的是轻度旧意图在深度入口恢复，不是深度任务执行成功。反向切换与报价非硬编码在`verify-ai-report-ui.cjs`实际组件中验证；其中37积分仅为合成接口回包反例，不是修改后端深度开发价格或正式售价。实际深度交付及反向切换按下段独立专项验收，不以本段轻度结果代替。

深度任务本身的实际H5专项：沿用上述原密码登录运行器、回环编译源和`TM_TEST_AI_H5_INTENTS=local-fixture-only`，增加`TM_TEST_AI_H5_DEEP=local-fixture-only`，清除其它问答/流故障/恢复及真实Agent专项标志。执行`verify-ai-source-native.ps1 -MySqlBaseDir <MySQL程序绝对目录> -BashPath <Git Bash绝对路径> -DeepH5Tests -ReportsOnly -ReportPattern '^TestIntegrationAIReportH5Deep$' -RunCount 1`。普通HTTP回归改选`^TestIntegrationDeepH5GatewayHTTP$`，不设浏览器配置，分别用8/9程序目录执行；该根内部覆盖RC/RR，不能把占位前端代理算成实际页面。

实际页面依次执行错误密码拒绝、会员一登录、新会话、深度技能、示例题材报价（本次隔离余额50、报价30测试积分）、保存意图但不确认。切换轻度技能或重载后恢复原意图，核价和二次弹窗仍须深度/原金额；确认后等弹窗隐藏再打开唯一关联任务。读取四节六条逐字引文报告并再次点原“核对并查看报告”，不重新创建。会员二通过原登录返回“我的”，进入资料整理确认无他人会话，再控制页结束。人工结束要求公开报告GET至少两次，并等原worker结束、原读取器两次校验、重读零改账、余额20/冻结0/单次settle、Prepare/Confirm/原创建核各1、账本及原登录子程序断言；最终父脚本退出0与迁移通过才算完成。交互窗口10分钟不延长worker30秒或任务300秒；超时失败保留，禁止改库终态。2026-10-23已取得MySQL8实际H5证据，反向技能恢复仍是同一深度任务；不代替真实模型、手机或生产调度验证。

源码问答/报告验证器必须显式传入仓库外依赖目录，并观察完整末尾输出；它们包含`beforeExit`未完成断言，不能把悬挂Promise造成的提前退出记为通过。源码套件、实际H5、数据库与真实模型证据分别记录，不互相替代。

原登录错误事件另运行`node scripts/dev/verify-member-login-script.cjs <隔离依赖绝对路径>`：只执行原UVue脚本，不渲染模板或替代鉴权。业务/网络拒绝已由原请求层提示，页面必须消费拒绝、不重复弹错、不写令牌/导航且释放loading；空响应、成功、资料读取失败与重复点击分别核对。实际原表单仍须按上述错误密码→正确密码→第二会员流程复验，并检查浏览器错误/警告日志，不能只因后续登录成功就忽略未处理Promise。

该脚本同时执行原会员页：游客零请求，设置失败/空响应不阻断资料查询，正常资料及资料失败分别核对。实际H5须从直接打开的原登录页成功回“我的”，确认设置查询失败时仍读取本人资料，不停在“登录/注册”旧空态；随后经原“资料整理”入口核对本人会话。当前隔离网关未开放`/member/settings`，返回纯文本404会留下一条框架JSON解析诊断；应明确记录，不把它当作设置服务成功，也不能用开放假设置响应隐藏未处理Promise。脚本与实际页面均须无新增未处理拒绝，此项不等于所有会员缓存或跨页并发场景已验收。

原密码登录模式的开关、运行器绝对路径与回环编译源必须同时配置；漏配或未知模式拒绝启动，不能退回控制页注入JWT。报告历史只读启动与新提交分别验收：可信报告工厂可在只读时提供证据/无密钥配置，模型工厂不得被调用；任何不完整报告工厂均失败关闭。资料提交开启也不能隐含报告追问提交许可。`TestIntegrationAIReportQA`覆盖不装配模型客户端的原答案回放、拒绝新问题及来源撤回拒读，API启动组另验只读路由与工厂错误；这两项均不等于实际生产工厂或真实模型已通过。

资料问答实际H5专项：沿用原登录测试程序、`TM_TEST_AI_H5_LOGIN=local-fixture-only`与回环`TM_TEST_AI_H5_ORIGIN`，再设置`TM_TEST_AI_H5_MATERIAL_QA=local-fixture-only`；执行原生脚本`-MaterialQABrowser -ReportsOnly -ReportPattern '^TestIntegrationAIReportMaterialQAH5$' -RunCount 1`，显式提供MySQL/Git Bash目录。只有此模式编译`integration,browser`专用根组，普通`integration`不编译它，缺显式标记不允许把未运行算通过，也不能混用`-LiveAgents`。

控制页提供本轮随机账号及原登录入口：会员一先验证错误密码拒绝，再正确登录并打开“示例问答会话”，选择资料问答、搜索“示例”、选择题材并提交“示例题材目录列出什么分类？”。读取原答案、核实终态清理本地原键，刷新页面从服务端历史再次读取；控制页撤回来源后重读必须拒绝。再切换无证据范围，提交不同事实问题，读取固定拒答与“本次未调用模型”标识。清理登录态、会员二从原表单登录应无会员一会话，旧标签清空。最后“结束并核对测试”要求仅两次问题POST、正常原答案至少两次读取、拒答与撤权拒读、请求2/消息2/模型调用存证3、零任务/冻结及原账本审计；原登录子程序和父脚本均须最终退出0、客户端迁移62/62。目录表启动前执行原Search查询，不用固定搜索回包替代真实SQL。此模式只有回环模型，不表示真实问答质量、多Agent或手机布局通过。

会话编辑/后台刷新回归：先以仓库外隔离依赖目录运行`node scripts/dev/verify-ai-report-ui.cjs <依赖绝对路径>`，确认最后包含九类编辑交错的通过行且进程退出0。该套件将历史刷新通知放在标题PATCH或归档确认期间，断言通知合并为一次只读GET、反向刷新中的按钮与函数均拒绝写入、失败/401/账号变化/卸载不恢复旧状态；另覆盖关联任务先于详情/消息及晚于消息三种打开顺序。实际H5沿用上述资料问答专项，在首次提问前改名，问题保存/历史刷新并读取引文后再次改名，再按原步骤完成重读和拒读反例；第二条拒答读取后取消一次归档确认，再显式确认归档、查询归档列表。正常浏览器操作不代替源码中精确控制响应顺序的竞争验证，也不能据此宣告原偶发任务详情消失已解决。

输入完整性验收：自动化输入与点击提交之间必须观察一次实际输入框，按只读DOM文本或可访问树确认全文（使用带唯一句末校验文字的中文长句），再通过原按钮提交；服务端问题状态和历史消息均须与该全文逐字一致。不得只凭输入工具返回成功认定所有字符已完成输入，也不能以“请求数量正确”代替文字完整性。若文本不符，先保留输入前后证据，区分输入动作尚未完成、组件事件同步、提交快照及服务端保存，不能直接给产品加延时掩盖。输入法组合态快速提交须单列真实输入法测试，普通文本输入通过不能替代该项。

### 会话双栏与窄屏抽屉回归

紧凑固定输入复验：原资料问答浏览器专项内，实测375×568时读取主滚动区和固定输入的边界，二者顶部差值是未被覆盖的正文高度；同时记录textarea实际高度、scrollWidth及完整值，不能以声明的64px当成实际文本区。打开/关闭技能抽屉保留草稿，按全文观察→原按钮提交→状态/历史逐字比对走两问、重读、撤权拒读、无证据拒答和双会员，等待最终退出0。2026-10-26实测主区top242、输入top334/bottom508，正文92px、textarea48px、无横向溢出；专项日志`tm-stock-compact-composer-h5-20261026.log`，迁移62/62，完整harness日志同前缀harness。该结果不替代物理键盘、输入法组合态或长句光标滚动编辑；报告内恢复112px与零请求仍由源码用例分别检查。

固定报告输入提交另验：使用上文原登录`feedback`专项并显式开启`TM_TEST_AI_H5_REPORT_QA=1`。原轻度任务交付后填写带句末校验的完整中文问题，固定输入并确认实际视口375×568、唯一输入和完整文本，再点原提交按钮；原状态及历史必须逐字一致。读取答案、核实终态清理本机键，再由历史追问入口核对报告与答案，历史态不可出现新输入。原反馈提交/控制页处理/查询完成后撤来源，答案和报告均须拒读而反馈继续可查；两会员及最终账本、迁移断言不可省略。2026-10-26该专项父脚本退出0，迁移62/62，日志`tm-stock-report-dock-submit-h5-20261026.log`；唯一原问题/三次模型存证、单次结算及反馈三事件通过。控制页处理仅验证内部处理链，不代替GVA后台页面与角色权限浏览器专项；本轮没有启用自然超期历史标记，不能据此刷新该项旧证据。

报告追问固定输入回归：先运行问答/报告完整源码套件，确认布局属性切换不丢草稿、不发业务请求、只有一个输入实例，以及父页面切页签/会话/报告清空退出固定模式。沿用深度实际H5专项，从原登录、保存深度意图、切轻度恢复原意图、报价确认到可读报告；输入完整中文草稿后仅切换“将报告追问固定到底部／恢复报告内输入”，实测1280与375×568尺寸、唯一固定输入和原草稿。打开技能抽屉应显示当前报告追问，选择资料问答退出固定但保留报告草稿。之后原报告重读、第二会员隔离、结束账本核对及迁移必须完成。2026-10-26此专项父脚本退出0，迁移62/62，日志`tm-stock-report-dock-h5-20261026.log`；截图同前缀desktop/narrow。深度夹具未启用报告问答提交，禁止在该流程里提交问题并据此宣称问答成功；实际提交/答案重读/撤权拒读须另用报告问答夹具。375×568固定输入top286/bottom508，正文可视空间仍偏少；浏览器视口不是物理软键盘验收。

视口前置核验：不要只相信尺寸设置调用成功。先对当前浏览器设置375×812，再新建本地标签，读取`document.documentElement.clientWidth/clientHeight/scrollWidth`确认真实尺寸；随后对同一标签切1280×720再切回375×812，每次重新读取。2026-10-25按此顺序已实测1280→375生效，窄屏scrollWidth=clientWidth=375，截图`tm-stock-viewport-375-20261025.png`为未登录页面；此前旧标签设置375后仍1280的记录不升级为通过。真实登录后仍必须另验抽屉、固定输入及草稿不丢失。结束恢复默认视口并关闭临时标签，不能通过改DOM样式或组件状态伪造尺寸事件。

紧凑技能入口补验：源码套件须断言选择事件使用原key，当前详情随active更新，重复当前项、禁用报告追问均不发事件，非法key只提示且选择组件零请求/存储。原H5先切深度，检查当前摘要、计费提示及报告追问禁用，再返回资料问答走完整两问专项；首问完成清原键后切深度，从历史问题入口恢复并重读，继续撤权/无证据/双会员。父控制器原有只读意图GET允许，不能把它误判为付费任务创建。2026-10-25桌面专项及源码、全量门禁通过，日志`tm-stock-skills-{h5,qa,report,harness}-20261025.log`；四按钮桌面单行不代表窄屏或固定底部工具栏已验。

独立滚动补验：在原资料问答专项登录后，先读取页面高度与左右滚动区边界，再用真实鼠标拖动右侧滚动条，读取两区scrollTop，必须只有右区变化且页面不被内容撑高；不要只看标签存在或模拟宿主。选定题材、输入草稿后重复检查固定输入及末尾留白，再走完整两问/撤权/双会员结束断言。2026-10-25桌面通过，右区698→1030时左区保持34，日志`tm-stock-scroll-h5-20261025.log`；长报告、窄屏低高度及软键盘是独立待验项。

先运行原`verify-ai-report-ui.cjs`、`verify-ai-qa-ui.cjs`，传入仓库外隔离依赖目录并确认完整结束。新增窗口监听替身只验证状态和请求次数，不能冒充实际布局：375/959/960/1280切换不重挂原报告插槽、不清当前会话/未保存标题、不重发业务请求，切页签/账号关闭抽屉，卸载解绑监听。问答技能目录须提示实际报价确认，不能将开发价格写成固定售价。

实际浏览器沿用上面的深度原密码登录专项，不配置真实供应商。原表单错误密码→会员一登录；1280宽填写新会话标题，待输入完整后切375打开历史抽屉，核对草稿，再回1280创建。保存原深度报价意图后切375继续复核及二次确认，等待弹窗实际关闭后打开唯一关联任务；读取并重读四节六引文，切回1280确认报告仍在右栏。再开关375抽屉，分别测页面宽度与scrollWidth、保存实际截图；最后会员二原登录无他人会话，旧标签清空，控制页结束并等待父进程退出0。不能用VM注入、直接写JWT、强行改任务终态或延长原worker期限代替上述路径。

2026-10-23该原MySQL8专项退出0，41条PASS、零失败/跳过、迁移62/62，原30测试积分单次settle、冻结0、报告重读零改账；日志`tm-stock-layout-h5-20261023.log`在系统临时目录。1280桌面左右栏及375历史抽屉/报告已实际核对，无页面横向溢出。当前输入/技能处于内容流末尾，固定底栏、独立消息滚动、回到底部、真机软键盘仍未验，不把这一轮当成完整移动端完成；未部署、改生产开关或扣真实会员积分。

历史资料问题技能定位反例：在资料问答H5专项中，正常答案读取并核实终态清本机原键后，选择深度技能，再点击原“查询问题状态 #序号”。必须自动切回资料问答卡片，独立核对并显示原答案；不能只看网络GET成功就算完成。该夹具不开放任务意图，深度入口的服务未开放提示不等于生成失败或积分扣除。继续来源撤回拒读、无证据第二问及第二会员隔离，最终审计仍须两次问题POST/三次模型存证/零任务冻结，防止历史入口偷偷重提。2026-10-23实际H5日志`tm-stock-history-skill-h5-20261023.log`通过；这一轮没有重载页面，不替代既有重载历史专项或移动端键盘验收。

进入后续专项前，固定输入增量可沿用上述资料问答原登录夹具复验：先跑问答与报告完整源码脚本。选定题材并输入完整草稿，375/1280切换、全部任务/会话页签往返及抽屉开关后，原草稿与对象保持；任务页签无固定输入，抽屉遮罩覆盖输入，页面无横向溢出，底部滚动后原对象选择可点。提交持久原键后固定输入收起，只从原入口核查/读取；按原两问、撤回来源、无证据、第二会员流程完成结束断言。截图应含对象、输入及提交边界，保存实际页面而非示意图。2026-10-23该专项父进程退出0（`tm-stock-composer-h5-20261023.log`）、迁移62/62，临时实例清理。全站免责声明已移至顶部，报告追问仍内联；不能把375桌面浏览器视口称为物理手机软键盘/iOS安全区验收，也不代表独立消息滚动或技能工具栏完成。

### SSE提前结束、原游标恢复与取消专项

2026-10-26当前布局复验：首次分散工具调用跨过原worker窗口，最终取消事务超时，失败现场按原规则保留；不能把“已登记取消”当作释放。新实例在确认冻结后一次连续浏览器操作中等待弹窗隐藏、打开关联任务、等控制页真实两次轮询、申请取消/保留/再次确认，约11秒完成，原GET确认已取消；父进程退出0、迁移62/62，日志`tm-stock-current-stream-h5-retry-20261026.log`。如果浏览器单次元素等待提前返回观察超时，可有界续等同一可见状态，不重启用例、不补详情GET，也不改业务期限；仍须在原窗口内完成，否则保留为失败并另验恢复路径。

沿用上述原登录程序、回环H5、`TM_TEST_AI_H5_INTENTS=local-fixture-only`，仅测试进程增加`TM_TEST_AI_H5_STREAM_FAULTS=local-fixture-only`，以`-ReportsOnly -ReportPattern '^TestIntegrationAIReportH5Gateway$/^Read_Committed$/^running_cancel$' -RunCount 1`运行原生脚本。标志与真实浏览器源必须同时存在。网关前两次将原handler的真实连接帧flush后结束本次上下文，第三次及以后对已通过原身份/归属校验的连接返回503；不改任务、事件、模型回包或执行时限。包装器保留原ResponseController/写期限能力，不用Recorder冒充流式宿主。

控制页仅增加“等待真实轮询观测/已取得两次真实轮询读取”只读计数，每250ms查询本测试网关计数，不触发业务读取或写入。该计数实际统计传输拒绝后的原详情GET，操作序列期间禁止手点“重新核对状态”等主动GET，只有结合完整浏览器动作记录才能证明自动轮询，不能把计数单独当证明。原密码登录并从会话创建唯一任务，待服务端关联消息加载后打开任务，观察重连及原状态查询。看到控制页“两次真实轮询”后，通过原取消面板确认暂停、保留恢复、再次确认取消，再从原任务GET确认“任务已取消”。此夹具原worker单次窗口为30秒，不能把人工停留时间加到任务/worker预算上；核心交互应在一次有界浏览器操作中完成，不能为收集截图等待超过窗口。窗口超时仍只登记取消、不伪造终态；这类轮次必须失败保留，并另验恢复worker，不得算成功取消。

结束前按原表单登录第二会员并核对隔离。结束断言两次真实连接提前结束、前三次请求原游标一致、后续游标不倒退、至少两次原详情GET、原取消受理及worker提交终态、冻结0、无报告/成功结算、原会话准备/确认/创建核各1和原账本审计；独立Agent记录须仍Running，不能因本地取消就声称远端已停。还须原登录子程序/父脚本退出0及客户端迁移62/62。此专项不发送新的公开业务事件，因此不能证明真实事件流重复去重；源码解释器的重放/乱序/UTF8/EOF及换账号等反例用`verify-ai-task-stream.cjs <隔离依赖目录>`另验，并保持未完成退出守卫。

### 原公开事件复放、续读与页面去重专项

沿用原登录程序、`TM_TEST_AI_H5_LOGIN=local-fixture-only`、绝对运行器路径与实际回环H5源，设置`TM_TEST_AI_H5_INTENTS=local-fixture-only`、`TM_TEST_AI_H5_STREAM_REPLAY=local-fixture-only`；清除上一专项的`TM_TEST_AI_H5_STREAM_FAULTS`，不要混用资料问答或真实Agent标志。一次性进程运行原生脚本`-StreamReplayBrowser -ReportsOnly -ReportPattern '^TestIntegrationAIReportH5StreamReplay$' -RunCount 1`，显式提供MySQL和Git Bash目录。缺标记、未知模式、非精确根或多次执行必须在启动前拒绝；普通integration不编译浏览器根组。

原表单错误密码拒绝→会员一正确登录→新建唯一会话→轻度技能/题材目录/报价→保存原意图/再次核价/确认冻结→等待原确认弹窗实际隐藏→打开关联任务。消息文字出现不代表覆盖其上的关闭动画已结束；不能把点击调用返回当成业务事件派发成功。首次原SSE连接flush后才由原worker启动回环Agent；首个公开事件同连接逐字重复一次并中断。第二连接须携带原确认游标，逐字复放该原帧一次，然后才放行回环Agent完成；不修改事件、任务、时钟、模型正文或原30秒worker窗口。观察实际事件序号仅出现一次且有后续新序号，原GET确认任务完成后独立核对四节报告，不能手工推进终态或补造事件历史。

切会员二原密码登录后确认无原会话，旧标签任务/报告清空，再结束测试。最终必须逐字比对每个传输原帧与原数据库Events DTO；原Agent隔离正文/摘要有效且原上游完成审计恰好一次，不能误断言它停在中间`upstream_completed`。还须原准备/确认/创建核各1、唯一关联、报告与成功结算各1、冻结0、CN测试提交1/回环模型3及账本审计，登录子程序与父进程退出0、迁移62/62。页面序号去重由实际AX/DOM及截图另核，不以网关计数替代。此项不代表真实供应商模型、完整事件审计历史或超时恢复/释放通过。原“首次打开详情消失”路径已用临时业务入口日志区分为关闭动画期间点击未派发，不是已证实的任务状态被清空；不得因此排除其它未知竞争，若弹窗退出后仍丢详情须另取证。

新建列表同步另核：新建/查原收据后当前会话保持打开，实际列表GET返回对应行，不再同时显示“暂无会话”。源码用例包含列表503保留当前详情、401清理、换账号与卸载拒绝迟到列表/消息；测试详情必须先满足原`version > lastSeq`契约，畸形夹具失败不能算作已定位产品缺陷。可在同轮打开既有`/__test__/mobile`的375×640 iframe，查询本人会话/已完成任务/报告及来源按钮，再验证换账号后旧iframe与桌面页均清空。仅视口布局与操作检查，不等于真机、软键盘、安全区或所有移动端异常态验收。

包装与配置反例另跑`go -C server test -race -tags integration ./integration/points -run '^TestH5StreamReplay(Configuration|Writer)$' -count=3 -v`，无数据库、模型或真实浏览器。它验证逐字复放、一次中断、原宿主能力、缺帧/错ID/未缓存游标拒绝及8种默认/回环/登录/混合故障配置；Recorder只用于该包装单测，实际流宿主必须由上面的真实HTTP浏览器专项证明。默认H5及worker相关回归使用原生脚本`-ReportsOnly -ReportPattern '^TestIntegrationAIReport(H5Gateway|Workflow)' -RunCount 1`，先清除本节全部测试环境标志，不把它称为全仓SQL验收。

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

### 同一会话资料问答、报告及反馈组合验收

实际API/独立worker版本使用`-ReportsOnly -LocalAssistantBrowser -ReportPattern '^TestIntegrationAIReportLocalAssistantH5$' -RunCount 1`，另传本机MySQL/Git Bash目录；在一次性进程配置`TM_TEST_AI_H5_LOCAL_ASSISTANT=local-fixture-only`、`TM_TEST_AI_H5_LOCAL_DEPTH=light`或`deep`及完整H5字面回环源`TM_TEST_AI_H5_ORIGIN`。清除原LocalAPIWorker、真实调用及其它浏览器标记；不需要手工注入登录程序、供应商凭据或JWT。精确入口缺模式/混用/多轮须在初始化前拒绝，缺显式编译标志不能零用例通过。原任务预算五分钟、浏览器十分钟，不延长既有自动化或模型时限。

从日志控制页读取仅本轮测试账号，再由原表单做错误密码反例及会员一登录。新建唯一会话→选题材做资料问答/核答案并复读→核实终态清理原键→选轻/深技能核价/保存意图/复核/确认冻结，等原弹窗实际关闭再打开任务。原SSE连接/队列执行后，控制页允许测试Agent完成；原报告读取/复读后提交报告追问、核引文并复读，原页面固定分类反馈。控制页“原管理进程处理反馈”只调用本轮独立管理HTTP两次，原页面查询已处理。清理原问题键，切资料问答；控制页“改为无证据范围”后再问一次，须固定拒答、零额外模型。最后“撤回测试来源”，原页面核报告与旧答案拒读、反馈仍可查，第二会员原登录无他人会话/旧页清空，控制页结束后须父脚本0/迁移64/64。该控制页不是后台角色UI；已有后台页面专项仍独立验收。原键清理后若其它面板尚显示旧阻断，重新核对报告会重建追问面板，不能直接清除存储绕过。

父核验分别统计kind=task的任务消息与kind=question/skill=report_qa的报告引用，不能只用task_id>0统计，后者会把合法追问误当重复创建。须核每轮原任务一次报告/结算、六报告审计、问答answered2/refused1/六次调用、反馈版本3/三事件/两个actor7处理事件及原账本审计。截图、控制页成功、复用旧测试或回环模型都不能替代这些最新父进程/SQL断言，也不代表真实模型质量或生产批准。当前轻/深证据见tasks.md。以下旧组合入口基于同库服务宿主，保留用于服务层回归，不作为新实际进程的浏览器证明。

自动化使用`verify-ai-source-native.ps1 -ReportsOnly -ReportPattern '^TestIntegrationAIReportAssistantCombined$' -RunCount 1`，指定本机MySQL程序目录与Git Bash；一次覆盖RC/RR。模型仅为回环替身，原问答、会话意图、任务worker、报告终检、账本和反馈接口均访问同一临时库。用例先确认资料问答及保存报价意图不改账，再经确认生成报告，受理/处理反馈，最后撤回来源验证问答及报告拒读。CI要求1根、2隔离子例，不允许漏跑。

实际浏览器使用同一根组的`/Read_Committed`选择器，设置官方完整H5的回环源`TM_TEST_AI_H5_ORIGIN`，以及原密码登录节的`TM_TEST_AI_H5_LOGIN=local-fixture-only`和`TM_TEST_AI_H5_LOGIN_RUNNER`；清除其他H5故障注入/历史到期标记。根组自行开启仅测试的资料范围、问答与会话意图，不需要密钥，不开启生产服务。原表单错误密码反例→会员一登录→新建会话→选择示例题材→提问“示例题材是什么？”并读出处→同一会话选择轻度技能→报价确认→打开原任务并读取/重读报告→提交一次事实反馈→控制页处理反馈→原页面查询处理历史→撤回来源并检查拒读→会员二登录无他人会话→控制页结束。处理控制按钮调用原内部接口，不代表后台页面操作；后台菜单/角色界面另按既有专项验收。

结束须检查父进程退出0及账本、同会话消息绑定、原意图/创建核各一次、问答三次模型存证、单次报告/结算、反馈三条事件和双会员原登录断言。不能用自动化HTTP分支代替实际H5操作，不能把控制页返回成功当父脚本通过。2026-10-26实际H5组合父进程已退出0，日志`%TEMP%/tm-stock-assistant-combined-browser-20261026.log`；发现的资料原键误指任务#0及后续修复验证边界见tasks当前记录。

### 清扫与恢复型 worker 既有验证记录

#### 会话链自然执行超时后的取消恢复（2026-10-23）

持续队列专项用原生脚本`-ReportsOnly -ReportPattern '^TestIntegrationAIReportQueueLoop$' -RunCount 1`，分别指定MySQL8/9及Git Bash路径。须确认RC/RR两组、新任务后续扫描执行、重复扫描单次结算、context取消退出和父脚本成功。该专项同进程启动服务层RunLoop，不代表正式CLI生命周期或浏览器验收。

启用深度报告读取前，必须执行`20261023_allow_deep_ai_report_commits.sql`并通过`AIReportSchemaVerifier.VerifyWithDeep`。该检查在回滚事务中实际提交类型2探针并检查保护，不能以轻度自检通过或触发器名称存在代替。旧库允许继续做轻度自检，但深度启动应拒绝。复现命令为原生脚本`-ReportsOnly -ReportPattern '^TestIntegrationAIReportDeepUpgradeExecution$' -RunCount 1`；两种MySQL版本均需验证旧库拒绝及升级后通过。

深度队列独立回归：`verify-ai-source-native.ps1 -ReportsOnly -ReportPattern '^TestIntegrationAIReportDeepQueue$' -RunCount 1`，另传MySQL程序目录与Git Bash路径，分别运行8/9。必须出现一个根组及RC/RR两子组通过，父进程退出0。该专项从Queued原任务开始经真实扫描/准入/worker/终检/结算，再从头重扫验证无额外调用或流水；不使用真实模型密钥，不等价于浏览器或正式调度进程验收。

新账本余额接入后，此自动化专项同时经原鉴权HTTP接口检查：创建前20/0、worker自然超时后13/7、取消受理未终态时13/7、原租约到期恢复及重放后20/0（可消费/冻结）。这些断言验证接口与恢复账务的一致性，不能替代会员页实际刷新展示的浏览器验收。普通`running_cancel`夹具不启动超时恢复；若交互超过worker期限，不应将该夹具的失败当作已验证恢复故障，也不能用调大期限或改数据库终态使其通过。

自动化专项在一次性进程清除本手册其它H5/真实Agent标记，执行`verify-ai-source-native.ps1 -TimeoutRecoveryTests -ReportsOnly -ReportPattern '^TestIntegrationAIReportTimeoutRecovery$' -RunCount 1`，显式指定MySQL程序目录及Git Bash目录。分别指定8版、9版；每版实际等待两组原30秒worker与一分钟租约，正常约三分钟。专项标签为`integration,browser`但此根仅走真实HTTP/数据库与占位前端，不冒充实际浏览器；CI独立精确运行，1根/RC及RR各1/零跳过缺一不可，不改变默认套件时限。漏专项标志、混合模式、非精确根或count非1必须在初始化前拒绝，不能因根组没编译而零用例通过。

实际H5另用`-ReportPattern '^TestIntegrationAIReportH5TimeoutRecovery$'`，保留`-TimeoutRecoveryTests -ReportsOnly -RunCount 1`。按“原会员密码登录”节编译原登录运行器，设置实际回环H5源、`TM_TEST_AI_H5_LOGIN=local-fixture-only`、运行器绝对路径、`TM_TEST_AI_H5_INTENTS=local-fixture-only`及`TM_TEST_AI_H5_TIMEOUT_RECOVERY=local-fixture-only`；清除SSE故障/复放、资料问答和真实Agent标志。此入口仅回环CN/模型，不需要供应商密钥。

用原表单做一次错误密码反例并登录会员一；新建唯一会话、轻度技能、选择示例题材，保存意图及重新核价后确认冻结。等原弹窗实际隐藏，再从原消息打开任务。等待日志实际观察原worker自然超时（不是看到30秒时间流逝就算），显式GET仍显示处理中。随后通过原取消入口二次确认，页面只能显示已登记、等待后台；应在原租约仍有效时完成此步，否则不能验证拒绝提前接管，本轮应保留为失败，不回拨时钟。

测试网关只把真实原取消回执交给主测试循环；原恢复服务首次必须返回尚不可接管，再按数据库真实UTC等原lease_until到期后恢复。等待不占住API互斥锁，页面仍可正常读取。恢复确认后独立查询取消终态，必须保留“不代表远端停止”，无报告入口；刷新并从原会话消息读取同一任务。切会员二登录，核实其无前会员会话、旧标签清空。控制页结束后须父脚本退出0：原意图准备/确认/创建核各1、上游提交1/模型0、原远端记录仍Running、恰好一次本地释放、冻结0、无报告/结算、旧租约拒写、恢复及L1终态重放不加调用/账务、原登录和账本审计及迁移全部通过。控制页确认、源码宿主或回环Agent不等于整轮成功、真实模型质量或生产调度验收。

过期清扫的2026-10-16最终验证记录统一见 tasks.md「显式限量过期清扫」：双版本完整回归及各三轮专项通过，迁移各38/38，成功实例停止并清理，CI表达式本地核对通过。MySQL8另一次客户端连接前置失败的实例已停止并保留，未计为通过，具体临时目录和未定位边界亦记录在那里；总门禁仍仅既有疑似密钥/扫描器自测红。以下仍是恢复型 worker 上一增量的证据，不与清扫的事务错误注入混称杀进程验证。

2026-10-16 恢复型 worker 验证：最终产品/测试冻结后的双版本全套均通过，恢复三组/生命周期两个隔离子例与十六场景/两个真实杀进程子例/四个命令子例全部通过；原仓储、账本、任务、会员只读、预算、阶段、抽取、Agent 及 GVA 回归通过，0 跳过，迁移断言各 38/38，成功实例停止并移除。同一最终代码随后在双版本各连续三轮通过恢复专项（每版三组各三次、六个杀进程子例、十二个命令子例），0 跳过/失败，客户端断言各 38/38，成功实例均停止并清理。实际命令在受限账号下通过联合自检与恢复，DDL 拒绝按 MySQL 权限错误验证。新增核心单测 Recover 93.9%、心跳循环 100%、逐步恢复 79.2%，计划契约 100%、命令包 88.1%；新增题材 SQL 读取合并同版本单测/专项语句块为 5/6（83.3%），数据库错误包装分支未覆盖，不等于全仓储/服务覆盖或生产性能。首轮短心跳 SQL 超时失败已修正，慢响应/超时/父期限回归通过，失败实例停止且证据保留，不算通过次数。Go/race/vet/build、覆盖率棘轮、合规/架构/格式及工作流/脚本语法通过，总门禁仍仅旧工作树疑似密钥扫描和扫描器自测红，未放宽基线、未运行远端 CI。无真实模型/Agent、扣积分、生产迁移、部署、提交或推送。
