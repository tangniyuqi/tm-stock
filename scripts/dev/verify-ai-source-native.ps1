# 使用已有 MySQL 程序新建临时实例；绝不读取本机数据库配置或连接已有实例。
param(
    [Parameter(Mandatory = $true)][string]$MySqlBaseDir,
    [ValidateRange(1, 20)][int]$RunCount = 3,
    [string]$BashPath = '',
    [string]$RepositoryCoveragePath = '',
    [string]$ServiceCoveragePath = '',
    [string]$APICoveragePath = '',
    [switch]$AgentsOnly,
    [switch]$TaskBudgetOnly,
    [switch]$TaskStagesOnly,
    [switch]$TaskRecoveryOnly,
    [switch]$TaskCancelOnly,
    [switch]$TaskEventsOnly,
    [switch]$ConversationsOnly,
    [switch]$TaskExpiryOnly,
    [switch]$TaskAgentBridgeOnly,
    [switch]$ReportsOnly,
    [switch]$NonReportsOnly,
    [switch]$FeedbackConsoleOnly,
    [switch]$LiveAgents,
    [switch]$MaterialQABrowser,
    [switch]$StreamReplayBrowser,
    [switch]$TimeoutRecoveryTests,
    [switch]$DeepH5Tests,
    [ValidateLength(1, 256)][string]$ConversationPattern = '^TestIntegrationAIConversations',
    [ValidateLength(1, 256)][string]$ReportPattern = '^TestIntegrationAIReport'
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$liveReportPatterns = @('^TestIntegrationAIReportLiveAgents$', '^TestIntegrationAIReportDeepLiveAgents$')
if ($ReportPattern -in $liveReportPatterns -and -not $LiveAgents) { throw '真实Agent根组需要显式编译专项标签，禁止零用例假通过' }
if ($ReportPattern -eq '^TestIntegrationAIReportMaterialQAH5$' -and -not $MaterialQABrowser) { throw '资料问答浏览器根组需要显式编译专项标签，禁止零用例假通过' }
if ($ReportPattern -eq '^TestIntegrationAIReportH5StreamReplay$' -and -not $StreamReplayBrowser) { throw 'SSE浏览器根组需要显式编译专项标签，禁止零用例假通过' }
if ($LiveAgents -and (-not $ReportsOnly -or $RunCount -ne 1 -or $ReportPattern -notin $liveReportPatterns -or $env:TM_TEST_AI_LIVE -ne 'local-fixture-only')) { throw '付费联调只允许显式本地单用例、单次执行' }
if ($MaterialQABrowser -and ($LiveAgents -or -not $ReportsOnly -or $RunCount -ne 1 -or $ReportPattern -ne '^TestIntegrationAIReportMaterialQAH5$' -or $env:TM_TEST_AI_H5_MATERIAL_QA -ne 'local-fixture-only')) { throw '资料问答浏览器只允许显式隔离单用例，不能混用真实Agent模式' }
if ($StreamReplayBrowser -and ($LiveAgents -or $MaterialQABrowser -or -not $ReportsOnly -or $RunCount -ne 1 -or $ReportPattern -ne '^TestIntegrationAIReportH5StreamReplay$' -or $env:TM_TEST_AI_H5_STREAM_REPLAY -ne 'local-fixture-only')) { throw '原事件SSE浏览器只允许独立、显式隔离单用例' }
$timeoutRecoveryPatterns = @('^TestIntegrationAIReportTimeoutRecovery$', '^TestIntegrationAIReportH5TimeoutRecovery$')
if ($ReportPattern -in $timeoutRecoveryPatterns -and -not $TimeoutRecoveryTests) { throw '超时恢复根组需要显式编译专项标签，禁止零用例假通过' }
if ($TimeoutRecoveryTests -and ($LiveAgents -or $MaterialQABrowser -or $StreamReplayBrowser -or -not $ReportsOnly -or $RunCount -ne 1 -or $ReportPattern -notin $timeoutRecoveryPatterns)) { throw '自然超时恢复只允许独立单次精确专项' }
if ($TimeoutRecoveryTests -and $ReportPattern -eq '^TestIntegrationAIReportH5TimeoutRecovery$' -and $env:TM_TEST_AI_H5_TIMEOUT_RECOVERY -ne 'local-fixture-only') { throw '实际H5超时恢复需要显式隔离标志' }
$deepH5Patterns = @('^TestIntegrationDeepH5GatewayHTTP$', '^TestIntegrationAIReportH5Deep$')
if ($ReportPattern -in $deepH5Patterns -and -not $DeepH5Tests) { throw '深度网关精确专项需要显式入口，禁止零用例假通过' }
if ($DeepH5Tests -and ($LiveAgents -or $MaterialQABrowser -or $StreamReplayBrowser -or $TimeoutRecoveryTests -or -not $ReportsOnly -or $RunCount -ne 1 -or $ReportPattern -notin $deepH5Patterns)) { throw '深度网关只允许独立单次精确专项' }
if ($DeepH5Tests -and $ReportPattern -eq '^TestIntegrationAIReportH5Deep$' -and $env:TM_TEST_AI_H5_DEEP -ne 'local-fixture-only') { throw '深度实际H5需要显式隔离标志' }
if ($RepositoryCoveragePath -ne '' -and $ServiceCoveragePath -ne '') { throw '一次只采集一个仓储或服务覆盖率文件' }
if (@($AgentsOnly,$TaskBudgetOnly,$TaskStagesOnly,$TaskRecoveryOnly,$TaskCancelOnly,$TaskEventsOnly,$ConversationsOnly,$TaskExpiryOnly,$TaskAgentBridgeOnly,$ReportsOnly,$NonReportsOnly,$FeedbackConsoleOnly).Where({ [bool]$_ }).Count -gt 1) { throw '专项模式不能同时选择' }
if ($ReportPattern -ne '^TestIntegrationAIReport' -and (-not $ReportsOnly -or (-not $ReportPattern.StartsWith('^TestIntegrationAIReport') -and -not ($DeepH5Tests -and $ReportPattern -eq '^TestIntegrationDeepH5GatewayHTTP$')))) { throw '自定义报告选择器只能用于显式报告专项，且必须保留报告用例前缀' }
if ($ConversationPattern -ne '^TestIntegrationAIConversations' -and (-not $ConversationsOnly -or -not $ConversationPattern.StartsWith('^TestIntegrationAIConversations'))) { throw '自定义会话选择器只能用于显式会话专项，且必须保留会话用例前缀' }
$PSNativeCommandUseErrorActionPreference = $false
if ($BashPath -eq '') {
    $bashCommand = Get-Command bash -ErrorAction SilentlyContinue
    if ($null -eq $bashCommand) { throw '找不到 Git Bash，请通过 -BashPath 指定 bash.exe 的绝对路径' }
    $BashPath = $bashCommand.Source
}
$bashBinary = (Resolve-Path -LiteralPath $BashPath).Path
$resolvedBase = (Resolve-Path -LiteralPath $MySqlBaseDir).Path
$serverBinary = Join-Path $resolvedBase 'bin/mysqld.exe'
$clientBinary = Join-Path $resolvedBase 'bin/mysql.exe'
$adminBinary = Join-Path $resolvedBase 'bin/mysqladmin.exe'
foreach ($binary in @($serverBinary, $clientBinary, $adminBinary)) {
    if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) { throw "找不到 MySQL 程序：$binary" }
}
$tempParent = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath()).TrimEnd('\')
$testName = 'tm-stock-ai-test-' + [guid]::NewGuid().ToString('N')
$testRoot = Join-Path $tempParent $testName
$dataRoot = Join-Path $testRoot 'data'
New-Item -ItemType Directory -Path $testRoot | Out-Null
$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
$listener.Start()
$testPort = $listener.LocalEndpoint.Port
$listener.Stop()
$serverProcess = $null
$passed = $false
$oldDSN = $env:TM_TEST_DSN
$oldPassword = $env:MYSQL_PWD
$oldLoginFile = $env:MYSQL_TEST_LOGIN_FILE
$verifyVariables = @('TM_VERIFY_MYSQL_CLIENT', 'TM_VERIFY_HOST', 'TM_VERIFY_PORT', 'TM_VERIFY_USER', 'TM_VERIFY_PASSWORD')
$oldVerifyEnvironment = @{}
foreach ($name in $verifyVariables) { $oldVerifyEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
$env:MYSQL_PWD = $null
$env:MYSQL_TEST_LOGIN_FILE = Join-Path $testRoot 'unused-login-file'
try {
    & $serverBinary --no-defaults --version
    $commonArgs = @('--no-defaults', '--no-monitor', "--basedir=`"$resolvedBase`"", "--datadir=`"$dataRoot`"", "--log-error=`"$testRoot/mysql.log`"")
    $initialization = Start-Process -FilePath $serverBinary -ArgumentList ($commonArgs + '--initialize-insecure') -WindowStyle Hidden -PassThru -Wait
    if ($initialization.ExitCode -ne 0) { throw '初始化临时测试库失败' }
    $serverArgs = $commonArgs + @("--port=$testPort", '--bind-address=127.0.0.1', '--mysqlx=OFF', '--innodb-buffer-pool-size=64M', '--innodb-redo-log-capacity=64M')
    $serverProcess = Start-Process -FilePath $serverBinary -ArgumentList $serverArgs -WindowStyle Hidden -PassThru
    $clientArgs = @('--no-defaults', '--host=127.0.0.1', "--port=$testPort", '--protocol=TCP', '--user=root', '--connect-timeout=1')
    $ready = $false
    for ($attempt = 0; $attempt -lt 80; $attempt++) {
        if ($serverProcess.HasExited) { throw '临时 MySQL 提前退出' }
        & $clientBinary @clientArgs --batch --skip-column-names --execute='SELECT 1' 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) { $ready = $true; break }
        Start-Sleep -Milliseconds 250
    }
    if (-not $ready) { throw '临时 MySQL 未就绪' }
    # 随机测试口令只经标准输入和子进程环境传递，不输出、不放入命令行。
    $testPassword = [guid]::NewGuid().ToString('N')
    "ALTER USER 'root'@'localhost' IDENTIFIED BY '$testPassword'; CREATE DATABASE tm_ai_source_test CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;" | & $clientBinary @clientArgs
    if ($LASTEXITCODE -ne 0) { throw '建立隔离测试库失败' }
    $env:MYSQL_PWD = $testPassword
    $env:TM_TEST_DSN = "root:$testPassword@tcp(127.0.0.1:$testPort)/tm_ai_source_test?parseTime=true&multiStatements=true&charset=utf8mb4&loc=UTC"
    Write-Output "隔离实例已就绪：回环端口 $testPort；临时目录 $testRoot"
    if (-not $FeedbackConsoleOnly) {
    Push-Location (Join-Path $PSScriptRoot '../../server')
    try {
        if (-not $AgentsOnly) {
        if (-not $TaskBudgetOnly -and -not $TaskStagesOnly -and -not $TaskRecoveryOnly -and -not $TaskCancelOnly -and -not $TaskEventsOnly -and -not $ConversationsOnly -and -not $TaskExpiryOnly -and -not $TaskAgentBridgeOnly -and -not $ReportsOnly) {
        go test -p 1 -race -tags=integration ./internal/repository -count $RunCount -v
        if ($LASTEXITCODE -ne 0) { throw 'AI / 共表迁移 / C 端仓储集成测试失败' }
        }
        # 整包用例已增长；明确每轮20分钟的测试进程看门狗，不改变单次SQL/网络/租约时限。
        $suiteTimeout = [string](20 * $RunCount) + 'm'
        $integrationTags = 'integration'
        if ($LiveAgents) { $integrationTags = 'integration,liveagents' }
        if ($MaterialQABrowser -or $StreamReplayBrowser -or $TimeoutRecoveryTests) { $integrationTags = 'integration,browser' }
        if ($DeepH5Tests -and $ReportPattern -eq '^TestIntegrationAIReportH5Deep$') { $integrationTags = 'integration,browser' }
        $pointsTestArgs = @('test', '-p', '1', '-race', "-tags=$integrationTags", './integration/points', '-timeout', $suiteTimeout, '-count', [string]$RunCount, '-v')
        if ($TaskBudgetOnly) { $pointsTestArgs += @('-run', '^TestIntegrationAITaskBudget') }
        if ($TaskStagesOnly) { $pointsTestArgs += @('-run', '^TestIntegrationAITaskStage') }
        if ($TaskRecoveryOnly) { $pointsTestArgs += @('-run', '^TestIntegrationAITaskRecovery') }
        if ($TaskCancelOnly) { $pointsTestArgs += @('-run', '^TestIntegrationAITaskMemberCancel') }
        if ($TaskEventsOnly) { $pointsTestArgs += @('-run', '^TestIntegrationAITaskMemberEvents') }
        if ($ConversationsOnly) { $pointsTestArgs += @('-run', $ConversationPattern) }
        if ($TaskExpiryOnly) { $pointsTestArgs += @('-run', '^TestIntegrationAITaskExpiry') }
        if ($TaskAgentBridgeOnly) { $pointsTestArgs += @('-run', '^TestIntegrationAITaskAgentBridge') }
        if ($ReportsOnly) {
            Write-Output "报告专项选择器：$ReportPattern；此模式不代表完整原生套件。"
            $pointsTestArgs += @('-run', $ReportPattern)
        }
        if ($NonReportsOnly) {
            # 只与完整ReportsOnly合并核验才能覆盖全范围；默认入口/看门狗/CI下限不变。
            # 其它仓储、Agent、API启动、GVA和迁移仍照默认入口执行。
            Write-Output '非报告专项：仅跳过points中的报告根组；须另验完整报告专项，不代表全量通过。'
            $pointsTestArgs += @('-skip', '^TestIntegrationAIReport')
        }
        if ($RepositoryCoveragePath -ne '') {
            # 由 Go 写出语句计数，不含业务数据；仅跨层套件采集，避免后面的 GVA 套件覆盖。
            $pointsTestArgs += @('-coverpkg=./internal/repository', "-coverprofile=$RepositoryCoveragePath")
        }
        if ($ServiceCoveragePath -ne '') {
            # 真实SQL跨层执行的服务计数，不把内存替身覆盖率冒充完整编排覆盖。
            $pointsTestArgs += @('-coverpkg=./internal/service', "-coverprofile=$ServiceCoveragePath")
        }
        go @pointsTestArgs
        if ($LASTEXITCODE -ne 0) { throw '账本服务跨层集成测试失败' }
        }
        if ($ConversationsOnly) {
        go test -p 1 -race -tags=integration ./cmd/api -run '^TestIntegrationAIReportReadStartup$' -count $RunCount -v
        if ($LASTEXITCODE -ne 0) { throw '会话及报告只读启动装配集成测试失败' }
        }
        if ($TaskCancelOnly -or (-not $AgentsOnly -and -not $TaskBudgetOnly -and -not $TaskStagesOnly -and -not $TaskRecoveryOnly -and -not $TaskEventsOnly -and -not $ConversationsOnly -and -not $TaskExpiryOnly -and -not $TaskAgentBridgeOnly -and -not $ReportsOnly)) {
        $apiTestArgs = @('test', '-p', '1', '-race', '-tags=integration', './cmd/api', '-run', '^TestIntegrationAITaskCancelStartup$', '-count', [string]$RunCount, '-v')
        if ($APICoveragePath -ne '') { $apiTestArgs += "-coverprofile=$APICoveragePath" }
        go @apiTestArgs
        if ($LASTEXITCODE -ne 0) { throw '会员取消启动装配集成测试失败' }
        }
        if ($ReportsOnly -or (-not $AgentsOnly -and -not $TaskBudgetOnly -and -not $TaskStagesOnly -and -not $TaskRecoveryOnly -and -not $TaskCancelOnly -and -not $TaskEventsOnly -and -not $ConversationsOnly -and -not $TaskExpiryOnly -and -not $TaskAgentBridgeOnly)) {
        go test -p 1 -race -tags=integration ./cmd/api -run '^TestIntegration(AIReportRead|AITaskCreate)Startup$' -count $RunCount -v
        if ($LASTEXITCODE -ne 0) { throw '会员创建/报告读取启动装配集成测试失败' }
        }
        if (-not $TaskBudgetOnly -and -not $TaskStagesOnly -and -not $TaskRecoveryOnly -and -not $TaskCancelOnly -and -not $TaskEventsOnly -and -not $ConversationsOnly -and -not $TaskExpiryOnly -and -not $TaskAgentBridgeOnly -and -not $ReportsOnly) {
        go test -p 1 -race -tags=integration ./integration/agents -count $RunCount -v
        if ($LASTEXITCODE -ne 0) { throw '外部 Agent 持久执行链路集成测试失败' }
        }
    } finally { Pop-Location }
    }
    if ($FeedbackConsoleOnly -or (-not $AgentsOnly -and -not $TaskBudgetOnly -and -not $TaskStagesOnly -and -not $TaskRecoveryOnly -and -not $TaskCancelOnly -and -not $TaskEventsOnly -and -not $ConversationsOnly -and -not $TaskExpiryOnly -and -not $TaskAgentBridgeOnly -and -not $ReportsOnly)) {
    Push-Location (Join-Path $PSScriptRoot '../../backend/server')
    try {
        if (-not $FeedbackConsoleOnly) {
        go test -p 1 -race -tags=integration ./service/quant -run MySQL -count $RunCount -v
        if ($LASTEXITCODE -ne 0) { throw 'GVA 共表读写集成测试失败' }
        }
        Write-Output '反馈后台MySQL专项：真实目录行锁/并发、原登录及角色授权；不代表全仓SQL回归。'
        # 两包串行使用本次创建的专库；登录组再用独立进程隔离GVA全局单例。
        go test -p 1 -race -tags=integration ./service/feedbackconsole ./api/v1/quant -run '^TestFeedbackConsoleMySQL(Catalog|Login)$' -count $RunCount -timeout 10m -v
        if ($LASTEXITCODE -ne 0) { throw '反馈后台MySQL目录/登录集成测试失败' }
    } finally { Pop-Location }
    }
    # 同一隔离实例另建可丢弃库，实测 mysql 客户端执行全部迁移，避免驱动与客户端分隔符不一致。
    $env:TM_VERIFY_MYSQL_CLIENT = $clientBinary.Replace('\', '/')
    $env:TM_VERIFY_HOST = '127.0.0.1'
    $env:TM_VERIFY_PORT = [string]$testPort
    $env:TM_VERIFY_USER = 'root'
    $env:TM_VERIFY_PASSWORD = $testPassword
    Push-Location (Join-Path $PSScriptRoot '../..')
    try {
        & $bashBinary scripts/dev/verify-migrations.sh
        if ($LASTEXITCODE -ne 0) { throw 'mysql 客户端迁移与约束验证失败' }
        $passed = $true
    } finally { Pop-Location }
} finally {
    if ($null -ne $serverProcess -and -not $serverProcess.HasExited) {
        & $adminBinary --no-defaults --host=127.0.0.1 "--port=$testPort" --protocol=TCP --user=root --connect-timeout=2 shutdown 2>$null
        if (-not $serverProcess.WaitForExit(10000)) {
            # 仅终止由本脚本创建且仍持有进程句柄的隔离实例，不按进程名称批量终止。
            Stop-Process -InputObject $serverProcess -Force
            $serverProcess.WaitForExit()
        }
    }
    $env:TM_TEST_DSN = $oldDSN
    $env:MYSQL_PWD = $oldPassword
    $env:MYSQL_TEST_LOGIN_FILE = $oldLoginFile
    foreach ($name in $verifyVariables) { [Environment]::SetEnvironmentVariable($name, $oldVerifyEnvironment[$name], 'Process') }
    # 删除前核对绝对路径、唯一名称和实际父目录，绝不接触 MySQL 已有 data 目录。
    $resolvedTestRoot = [System.IO.Path]::GetFullPath($testRoot)
    if ((Split-Path -Parent $resolvedTestRoot) -ne $tempParent -or (Split-Path -Leaf $resolvedTestRoot) -ne $testName -or $testName -notmatch '^tm-stock-ai-test-[a-f0-9]{32}$') {
        throw '临时目录安全校验失败，拒绝清理'
    }
    if ($passed) {
        Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
        Write-Output '已关闭并移除本次可丢弃测试实例。'
    } else {
        Write-Output "测试失败，临时实例已停止，保留日志与数据用于排查：$resolvedTestRoot"
    }
}
