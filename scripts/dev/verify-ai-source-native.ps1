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
    [switch]$LocalAPIWorkerBrowser,
    [switch]$LocalBootstrapOnly,
    [switch]$LocalAPIBootstrap,
    [switch]$LocalMemberBootstrap,
    [switch]$LocalConversationBootstrap,
    [switch]$LocalPlanBootstrap,
    [switch]$LocalTaskBootstrap,
    [switch]$LocalReportBootstrap,
    [ValidateLength(1, 256)][string]$ConversationPattern = '^TestIntegrationAIConversations',
    [ValidateLength(1, 256)][string]$ReportPattern = '^TestIntegrationAIReport'
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$liveReportPatterns = @('^TestIntegrationAIReportLiveAgents$', '^TestIntegrationAIReportDeepLiveAgents$', '^TestIntegrationAIReportLocalAPIWorkerLive$', '^TestIntegrationAIReportLocalAPIWorkerLiveH5$')
$localAPIWorkerH5Pattern = '^TestIntegrationAIReportLocalAPIWorkerH5$'
if ($ReportPattern -eq $localAPIWorkerH5Pattern -and -not $LocalAPIWorkerBrowser) { throw '正式进程H5根组需要显式浏览器入口，禁止零用例假通过' }
if ($LocalAPIWorkerBrowser -and ($LiveAgents -or $MaterialQABrowser -or $StreamReplayBrowser -or $TimeoutRecoveryTests -or $DeepH5Tests -or $LocalBootstrapOnly -or $LocalAPIBootstrap -or $LocalMemberBootstrap -or $LocalConversationBootstrap -or $LocalPlanBootstrap -or $LocalTaskBootstrap -or $LocalReportBootstrap -or -not $ReportsOnly -or $RunCount -ne 1 -or $ReportPattern -ne $localAPIWorkerH5Pattern -or $env:TM_TEST_AI_H5_LOCAL_API_WORKER -ne 'local-fixture-only' -or $env:TM_TEST_AI_H5_LOCAL_DEPTH -notin @('light','deep'))) { throw '正式进程H5只允许独立单次轻深隔离浏览器验收' }
if ($LocalReportBootstrap) { $LocalTaskBootstrap = $true }
if ($LocalTaskBootstrap) { $LocalPlanBootstrap = $true; $LocalConversationBootstrap = $true }
if ($LocalPlanBootstrap) { $LocalAPIBootstrap = $true }
if ($LocalConversationBootstrap) { $LocalMemberBootstrap = $true }
if ($LocalMemberBootstrap) { $LocalAPIBootstrap = $true }
if ($LocalAPIBootstrap) { $LocalBootstrapOnly = $true }
if ($LocalBootstrapOnly -and ($RunCount -ne 1 -or $LiveAgents -or $MaterialQABrowser -or $StreamReplayBrowser -or $TimeoutRecoveryTests -or $DeepH5Tests -or $ReportPattern -ne '^TestIntegrationAIReport' -or $RepositoryCoveragePath -ne '' -or $ServiceCoveragePath -ne '' -or $APICoveragePath -ne '')) { throw '本地初始化只允许独立单次隔离执行，不混用其它专项或真实调用' }
if ($ReportPattern -in $liveReportPatterns -and -not $LiveAgents) { throw '真实Agent根组需要显式编译专项标签，禁止零用例假通过' }
if ($ReportPattern -eq '^TestIntegrationAIReportMaterialQAH5$' -and -not $MaterialQABrowser) { throw '资料问答浏览器根组需要显式编译专项标签，禁止零用例假通过' }
if ($ReportPattern -eq '^TestIntegrationAIReportH5StreamReplay$' -and -not $StreamReplayBrowser) { throw 'SSE浏览器根组需要显式编译专项标签，禁止零用例假通过' }
if ($LiveAgents -and (-not $ReportsOnly -or $RunCount -ne 1 -or $ReportPattern -notin $liveReportPatterns -or $env:TM_TEST_AI_LIVE -ne 'local-fixture-only')) { throw '付费联调只允许显式本地单用例、单次执行' }
if ($LiveAgents -and $ReportPattern -eq '^TestIntegrationAIReportLocalAPIWorkerLiveH5$' -and ($env:TM_TEST_AI_LIVE_PROCESS -ne 'local-fixture-only' -or $env:TM_TEST_AI_LIVE_PROCESS_H5 -ne 'local-fixture-only' -or $env:TM_TEST_AI_H5_LOCAL_DEPTH -notin @('light','deep'))) { throw '真实进程H5需要单独声明浏览器及轻深范围' }
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
if (@($AgentsOnly,$TaskBudgetOnly,$TaskStagesOnly,$TaskRecoveryOnly,$TaskCancelOnly,$TaskEventsOnly,$ConversationsOnly,$TaskExpiryOnly,$TaskAgentBridgeOnly,$ReportsOnly,$NonReportsOnly,$FeedbackConsoleOnly,$LocalBootstrapOnly).Where({ [bool]$_ }).Count -gt 1) { throw '专项模式不能同时选择' }
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
    if ($LocalBootstrapOnly) {
        # 仅在本脚本新建且仍持有句柄的MySQL内初始化；不接受外部DSN或已有数据目录。
        if ($null -eq $serverProcess -or $serverProcess.HasExited) { throw '本轮隔离实例未运行' }
        $localInstanceID = [guid]::NewGuid().ToString('N')
        $localDatabase = 'tm_ai_local_' + $localInstanceID
        $localUser = 'tm_ai_local_' + $localInstanceID.Substring(0, 16)
        $localPassword = [guid]::NewGuid().ToString('N')
        $localSQL = @"
CREATE DATABASE $localDatabase CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE TABLE $localDatabase.tm_ai_local_instance (
 instance_id char(32) NOT NULL PRIMARY KEY, database_name varchar(64) NOT NULL,
 server_uuid char(36) NOT NULL, environment varchar(32) NOT NULL, contains_real_users tinyint NOT NULL,
 CHECK (environment='local-isolated'), CHECK (contains_real_users=0)
) ENGINE=InnoDB;
INSERT INTO $localDatabase.tm_ai_local_instance VALUES ('$localInstanceID','$localDatabase',@@server_uuid,'local-isolated',0);
CREATE USER '$localUser'@'127.0.0.1' IDENTIFIED BY '$localPassword';
GRANT SELECT ON $localDatabase.tm_ai_local_instance TO '$localUser'@'127.0.0.1';
"@
        $localSQL | & $clientBinary @clientArgs
        if ($LASTEXITCODE -ne 0) { throw '本地专属库/标记/核验账号初始化失败，禁止继续运行' }
        # 不用IF NOT EXISTS接管旧库；第二次创建必须拒绝且保留原标记。
        $duplicateOutput = & $clientBinary @clientArgs --execute="CREATE DATABASE $localDatabase" 2>&1
        if ($LASTEXITCODE -eq 0 -or ($duplicateOutput -join "`n") -notmatch '\b1007\b') { throw '重复初始化未按库已存在拒绝' }
        $localConfigPath = Join-Path $testRoot 'local-runtime.json'
        $localMetadata = [ordered]@{ version=1; environment='local-isolated'; instance_id=$localInstanceID; database=$localDatabase; listen='127.0.0.1:8099'; real_calls=$false; poll_interval_ms=1000; batch_size=10 }
        if ($LocalAPIBootstrap) {
            $apiListener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
            $apiListener.Start(); $apiPort = $apiListener.LocalEndpoint.Port; $apiListener.Stop()
            $localMetadata.listen = "127.0.0.1:$apiPort"
        }
        [IO.File]::WriteAllText($localConfigPath, ($localMetadata | ConvertTo-Json -Compress), [Text.UTF8Encoding]::new($false))
        $workerPath = Join-Path $testRoot 'ai-worker.exe'
        Push-Location (Join-Path $PSScriptRoot '../../server')
        try { go build -o $workerPath ./cmd/ai-worker; if ($LASTEXITCODE -ne 0) { throw '本地worker构建失败' } } finally { Pop-Location }
        $savedRuntimeDSN = $env:TM_DB_DSN
        $invokeLocalWorker = {
            param([switch]$CheckPlans)
            $startInfo = [Diagnostics.ProcessStartInfo]::new()
            $startInfo.FileName = $workerPath
            $startInfo.UseShellExecute = $false
            $startInfo.CreateNoWindow = $true
            $startInfo.RedirectStandardOutput = $true
            $startInfo.RedirectStandardError = $true
            if ($CheckPlans) { $startInfo.ArgumentList.Add('check-local-plans') } else { $startInfo.ArgumentList.Add('check-local-db') }
            $startInfo.ArgumentList.Add('-config')
            $startInfo.ArgumentList.Add($localConfigPath)
            if ($CheckPlans) { $startInfo.ArgumentList.Add('-plans'); $startInfo.ArgumentList.Add($localPlansPath) }
            $startInfo.Environment.Clear()
            foreach ($runtimeName in @('SystemRoot','WINDIR','PATH','PATHEXT','USERPROFILE','APPDATA','LOCALAPPDATA','TEMP','TMP')) {
                $runtimeValue = [Environment]::GetEnvironmentVariable($runtimeName, 'Process')
                if ($null -ne $runtimeValue) { $startInfo.Environment[$runtimeName] = $runtimeValue }
            }
            # 不继承TM_TEST_DSN、MYSQL_PWD、宿主模型或产品凭据。
            $startInfo.Environment['TM_DB_DSN'] = $env:TM_DB_DSN
            $workerProcess = [Diagnostics.Process]::new()
            $workerProcess.StartInfo = $startInfo
            try {
                if (-not $workerProcess.Start()) { throw '本地worker未启动' }
                if (-not $workerProcess.WaitForExit(15000)) { $workerProcess.Kill(); $workerProcess.WaitForExit(); throw '本地worker预检超时' }
                $workerOutput = $workerProcess.StandardOutput.ReadToEnd()
                $null = $workerProcess.StandardError.ReadToEnd()
                return [pscustomobject]@{ Code=$workerProcess.ExitCode; Text=$workerOutput }
            } finally { $workerProcess.Dispose() }
        }
        try {
            $env:MYSQL_PWD = $localPassword
            $env:TM_DB_DSN = "$($localUser):$($localPassword)@tcp(127.0.0.1:$testPort)/$($localDatabase)?parseTime=true&charset=utf8mb4&loc=UTC"
            $workerResult = & $invokeLocalWorker
            if ($workerResult.Code -ne 0) { throw '核验账号未通过实际worker预检' }
            Write-Output $workerResult.Text.Trim()
            $localArgs = @($clientArgs | Where-Object { -not $_.StartsWith('--user=') }) + "--user=$localUser"
            & $clientBinary @localArgs --execute="SELECT instance_id FROM $localDatabase.tm_ai_local_instance" 2>$null | Out-Null
            if ($LASTEXITCODE -ne 0) { throw '核验账号正例失败，不能将连接失败视为权限拒绝' }
            foreach ($deniedSQL in @("DELETE FROM $localDatabase.tm_ai_local_instance", "UPDATE $localDatabase.tm_ai_local_instance SET environment='other'", "CREATE TABLE $localDatabase.forbidden_table(id int)", 'SELECT User FROM mysql.user')) {
                $deniedOutput = & $clientBinary @localArgs --execute=$deniedSQL 2>&1
                if ($LASTEXITCODE -eq 0 -or ($deniedOutput -join "`n") -notmatch '\b(1044|1142|1143)\b') { throw '核验账号未按权限错误拒绝；不把SQL或连接故障算通过' }
            }
            $workerResult = & $invokeLocalWorker
            if ($workerResult.Code -ne 0) { throw '权限反例修改了原实例标记' }
        } finally { $env:TM_DB_DSN = $savedRuntimeDSN; $env:MYSQL_PWD = $testPassword }
        if ($LocalAPIBootstrap) {
            # 复用既有迁移验收的两张上游测试表定义，只取DDL，不复制其示例证券数据。
            $migrationVerifier = [IO.File]::ReadAllText((Join-Path $PSScriptRoot 'verify-migrations.sh'))
            $upstream = [regex]::Match($migrationVerifier, '(?ms)^UPSTREAM_DDL="\r?\n(.*?)^"')
            $upstreamTables = [regex]::Matches($upstream.Groups[1].Value, '(?ms)^CREATE TABLE .*?;')
            if (-not $upstream.Success -or $upstreamTables.Count -ne 2) { throw '上游测试DDL范围变化，拒绝猜测初始化' }
            ($upstreamTables.Value -join "`n") | & $clientBinary @clientArgs "--database=$localDatabase"
            if ($LASTEXITCODE -ne 0) { throw '上游空测试表建立失败' }
            # PowerShell文化排序忽略部分标点，会把conversations_tasks排在conversations.sql前。
            # 使用明确序数顺序，与现有Bash迁移入口保持一致，不重写迁移或跳过外键。
            $migrationDirectory = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../../server/migrations')).Path
            $migrations = [IO.Directory]::GetFiles($migrationDirectory, '*.sql')
            [Array]::Sort($migrations, [StringComparer]::Ordinal)
            if ($migrations.Count -eq 0) { throw '没有业务迁移，拒绝继续启动' }
            foreach ($migration in $migrations) {
                [IO.File]::ReadAllText($migration) | & $clientBinary @clientArgs "--database=$localDatabase"
                if ($LASTEXITCODE -ne 0) { throw "本地业务迁移失败：$([IO.Path]::GetFileName($migration))" }
            }
            if ($LocalPlanBootstrap) {
                $localPlansPath = Join-Path $testRoot 'local-plans.json'
                $planRunner = Join-Path $testRoot 'local-plan-tests.exe'
                Push-Location (Join-Path $PSScriptRoot '../../server')
                try { go test -c -tags=integration -o $planRunner ./cmd/api; if ($LASTEXITCODE -ne 0) { throw '本地计划测试程序构建失败' } } finally { Pop-Location }
                $planStart = [Diagnostics.ProcessStartInfo]::new()
                $planStart.FileName = $planRunner; $planStart.UseShellExecute = $false; $planStart.CreateNoWindow = $true
                $planStart.RedirectStandardInput = $true; $planStart.RedirectStandardOutput = $true; $planStart.RedirectStandardError = $true
                foreach ($argument in @('-test.run=^TestAILocalPlanBootstrap$','-test.count=1','-test.v')) { $planStart.ArgumentList.Add($argument) }
                $planStart.Environment.Clear()
                foreach ($runtimeName in @('SystemRoot','WINDIR','PATH','PATHEXT','USERPROFILE','APPDATA','LOCALAPPDATA','TEMP','TMP')) {
                    $runtimeValue = [Environment]::GetEnvironmentVariable($runtimeName, 'Process')
                    if ($null -ne $runtimeValue) { $planStart.Environment[$runtimeName] = $runtimeValue }
                }
                $planStart.Environment['TM_TEST_AI_LOCAL_PLAN_CHILD'] = 'local-fixture-only'
                $planProcess = [Diagnostics.Process]::new(); $planProcess.StartInfo = $planStart; $planStarted = $false
                try {
                    if (-not $planProcess.Start()) { throw '本地计划测试程序未启动' }; $planStarted = $true
                    $planInput = @{ dsn="root:$testPassword@tcp(127.0.0.1:$testPort)/$($localDatabase)?parseTime=true&charset=utf8mb4&loc=UTC"; runtime=$localConfigPath; plans=$localPlansPath; fund=[bool]$LocalTaskBootstrap }
                    $planProcess.StandardInput.WriteLine(($planInput | ConvertTo-Json -Compress)); $planProcess.StandardInput.Close()
                    if (-not $planProcess.WaitForExit(100000)) { throw '本地计划数据库验收超时' }
                    $planOutput = $planProcess.StandardOutput.ReadToEnd(); $null = $planProcess.StandardError.ReadToEnd()
                    if ($planProcess.ExitCode -ne 0 -or $planOutput -notmatch 'LOCAL_PLAN_BOOTSTRAP_VERIFIED' -or $planOutput -notmatch '(?m)^--- PASS: TestAILocalPlanBootstrap ') {
                        $safeLine = [regex]::Match($planOutput, 'ai_local_admission_integration_test.go:(\d+):').Groups[1].Value
                        throw "本地计划/报价/撤回验收失败（测试行$safeLine，内部内容已隐藏）"
                    }
                    Write-Output 'LOCAL_PLAN_BOOTSTRAP_VERIFIED：原创建服务装配、轻深报价、来源撤回及文件变价拒绝、无积分创建拒绝通过；零任务/积分流水/模型调用。'
                    if ($LocalTaskBootstrap) {
                        if ($planOutput -notmatch 'LOCAL_TASK_FUNDS_VERIFIED') { throw '本轮测试积分初始化未确认' }
                        Write-Output 'LOCAL_TASK_FUNDS_VERIFIED：仅本轮两个测试会员各50测试积分，不迁移原会员积分。'
                    }
                } finally {
                    if ($planStarted -and -not $planProcess.HasExited) { $planProcess.Kill(); $planProcess.WaitForExit() }
                    $planProcess.Dispose()
                }
            }
            if ($LocalMemberBootstrap) {
                $memberRunner = Join-Path $testRoot 'local-member-tests.exe'
                Push-Location (Join-Path $PSScriptRoot '../../backend/server')
                try { go test -c -tags=integration -o $memberRunner ./router/client; if ($LASTEXITCODE -ne 0) { throw '原会员测试程序构建失败' } } finally { Pop-Location }
                $memberConfig = @{ dsn="root:$testPassword@tcp(127.0.0.1:$testPort)/$($localDatabase)?parseTime=true&charset=utf8mb4&loc=UTC"; signing=([guid]::NewGuid().ToString('N')+[guid]::NewGuid().ToString('N')); control=([guid]::NewGuid().ToString('N')); task_api=[bool]$LocalTaskBootstrap }
                if ($LocalTaskBootstrap) { $memberConfig.plans_path = $localPlansPath }
                if ($LocalReportBootstrap) { $memberConfig.report_api = $true }
                $invokeLocalMember = {
                param([string]$apiURL = '')
                $memberStart = [Diagnostics.ProcessStartInfo]::new()
                $memberStart.FileName = $memberRunner; $memberStart.UseShellExecute = $false; $memberStart.CreateNoWindow = $true
                $memberStart.RedirectStandardInput = $true; $memberStart.RedirectStandardOutput = $true; $memberStart.RedirectStandardError = $true
                foreach ($argument in @('-test.run=^TestAIMemberLocalBootstrap$','-test.count=1','-test.v')) { $memberStart.ArgumentList.Add($argument) }
                $memberStart.Environment.Clear()
                foreach ($runtimeName in @('SystemRoot','WINDIR','PATH','PATHEXT','USERPROFILE','APPDATA','LOCALAPPDATA','TEMP','TMP')) {
                    $runtimeValue = [Environment]::GetEnvironmentVariable($runtimeName, 'Process')
                    if ($null -ne $runtimeValue) { $memberStart.Environment[$runtimeName] = $runtimeValue }
                }
                $memberStart.Environment['TM_TEST_AI_LOGIN_CHILD'] = 'local-fixture-only'
                $memberProcess = [Diagnostics.Process]::new(); $memberProcess.StartInfo = $memberStart
                $memberStarted = $false
                try {
                    if (-not $memberProcess.Start()) { throw '原会员测试程序未启动' }
                    $memberStarted = $true
                    $memberConfig.api_url = $apiURL
                    $memberProcess.StandardInput.WriteLine(($memberConfig | ConvertTo-Json -Compress)); $memberProcess.StandardInput.Close()
                    if (-not $memberProcess.WaitForExit(30000)) { $memberProcess.Kill(); $memberProcess.WaitForExit(); throw '原会员初始化验收超时' }
                    $memberOutput = $memberProcess.StandardOutput.ReadToEnd(); $null = $memberProcess.StandardError.ReadToEnd()
                    if ($memberProcess.ExitCode -ne 0 -or $memberOutput -notmatch 'LOCAL_MEMBER_BOOTSTRAP_VERIFIED' -or $memberOutput -notmatch '(?m)^--- PASS: TestAIMemberLocalBootstrap ') {
                        $safeMemberLine = [regex]::Match($memberOutput,'ai_member_(?:task|conversation|login)_integration_test.go:(\d+):').Groups[1].Value
                        throw "原会员/实际API联调未通过（测试行$safeMemberLine，内部输出已隐藏）"
                    }
                    if ($apiURL -ne '') {
                        if ($memberOutput -notmatch 'LOCAL_CONVERSATION_API_VERIFIED') { throw '实际API会话联调未执行，拒绝零用例通过' }
                        Write-Output 'LOCAL_CONVERSATION_API_VERIFIED：原登录JWT与实际API会话创建/查询/幂等/改名归档/双会员隔离/停用及软删除拒绝均通过；无模型调用。'
                        if ($LocalTaskBootstrap) {
                            if ($memberOutput -notmatch 'LOCAL_TASK_API_VERIFIED') { throw '实际任务HTTP专项未运行，拒绝零用例通过' }
                            Write-Output 'LOCAL_TASK_API_VERIFIED：原登录JWT→实际API轻深报价/确认/冻结/重放/关联/跨会员拒绝/取消一次释放/SSE断点续读通过，零模型调用。'
                        }
                    } else { Write-Output 'LOCAL_MEMBER_BOOTSTRAP_VERIFIED：原密码校验、原JWT、两账号资料及登录存证通过；测试口令未输出，实际API鉴权另行核验。' }
                } finally {
                    if ($memberStarted -and -not $memberProcess.HasExited) { $memberProcess.Kill(); $memberProcess.WaitForExit() }
                    $memberProcess.Dispose()
                }
                }
                & $invokeLocalMember
            }
            $apiUser = 'tm_ai_api_' + $localInstanceID.Substring(0, 16)
            $apiPassword = [guid]::NewGuid().ToString('N')
            $planReadUser = $apiUser; $planReadPassword = $apiPassword
            $apiGrant = "CREATE USER '$apiUser'@'127.0.0.1' IDENTIFIED BY '$apiPassword';"
            foreach ($table in @('tm_ai_local_instance','addon_quant_theme','addon_quant_base_stock','addon_quant_theme_stock','theme_daily_item','theme_daily_quote')) {
                $apiGrant += " GRANT SELECT ON $localDatabase.$table TO '$apiUser'@'127.0.0.1';"
            }
            if ($LocalPlanBootstrap) { foreach ($table in @('tm_ai_sources','tm_ai_source_snapshots','tm_ai_source_bodies')) { $apiGrant += " GRANT SELECT ON $localDatabase.$table TO '$apiUser'@'127.0.0.1';" } }
            if ($LocalConversationBootstrap) {
                # 只读DEFINER视图不能继承I_S.TRIGGERS可见性；固定无参过程在所有者上下文实时读取。
                # 只投影本库两表的触发器身份，不暴露定义正文，也不执行动态SQL/DDL。
                $conversationMetadataDDL = "CREATE PROCEDURE $localDatabase.tm_ai_local_conversation_trigger_metadata() SQL SECURITY DEFINER READS SQL DATA SELECT trigger_schema,event_object_table,trigger_name,event_manipulation,action_timing FROM information_schema.triggers WHERE trigger_schema='$localDatabase' AND event_object_table IN ('tm_ai_conversations','tm_ai_conversation_messages');"
                $apiGrant += " $conversationMetadataDDL"
                $apiGrant += " GRANT EXECUTE ON PROCEDURE $localDatabase.tm_ai_local_conversation_trigger_metadata TO '$apiUser'@'127.0.0.1';"
                foreach ($table in @('tm_ai_conversations','tm_ai_conversation_messages')) {
                    # DELETE/UPDATE仅供原保护探针实际触发1644拒绝，触发器仍禁止删除/改消息。
                    $apiGrant += " GRANT SELECT,INSERT,UPDATE,DELETE ON $localDatabase.$table TO '$apiUser'@'127.0.0.1';"
                }
                foreach ($table in @('tm_ai_tasks','tm_ai_conversation_tasks')) { $apiGrant += " GRANT SELECT ON $localDatabase.$table TO '$apiUser'@'127.0.0.1';" }
                $apiGrant += " GRANT SELECT(id,status,deleted_at) ON $localDatabase.addon_member TO '$apiUser'@'127.0.0.1';"
            }
            if ($LocalTaskBootstrap) {
                # 所有写权限仅在本次新建实例内逐表授予；保留真实拒写探针，不授DDL/TRIGGER。
                foreach ($table in @('tm_points_accounts','tm_points_txns','tm_points_lots','tm_points_entries','tm_points_holds','tm_points_hold_allocs','tm_points_debts','tm_points_debt_allocs','tm_ai_tasks','tm_ai_task_events','tm_ai_task_budgets','tm_ai_task_reservations','tm_ai_task_step_results','tm_ai_task_steps','tm_ai_task_admissions','tm_ai_report_commits','tm_ai_conversation_tasks')) {
                    $apiGrant += " GRANT SELECT,INSERT,UPDATE,DELETE ON $localDatabase.$table TO '$apiUser'@'127.0.0.1';"
                }
                $intentMetadataDDL = "CREATE PROCEDURE $localDatabase.tm_ai_local_intent_trigger_metadata() SQL SECURITY DEFINER READS SQL DATA SELECT trigger_schema,event_object_table,trigger_name,event_manipulation,action_timing FROM information_schema.triggers WHERE trigger_schema='$localDatabase' AND (event_object_table='tm_ai_conversation_tasks' OR event_object_table='tm_ai_conversations' AND trigger_name='tm_ai_conversation_intent_archive_guard');"
                $apiGrant += " $intentMetadataDDL GRANT EXECUTE ON PROCEDURE $localDatabase.tm_ai_local_intent_trigger_metadata TO '$apiUser'@'127.0.0.1';"
                # worker预检单独使用仅SELECT账号，不能把API写账号称作只读证明。
                $planReadUser = 'tm_ai_plan_' + $localInstanceID.Substring(0,16); $planReadPassword = [guid]::NewGuid().ToString('N')
                $apiGrant += " CREATE USER '$planReadUser'@'127.0.0.1' IDENTIFIED BY '$planReadPassword';"
                foreach ($table in @('tm_ai_local_instance','addon_quant_theme','tm_ai_sources','tm_ai_source_snapshots','tm_ai_source_bodies')) { $apiGrant += " GRANT SELECT ON $localDatabase.$table TO '$planReadUser'@'127.0.0.1';" }
            }
            $apiGrant | & $clientBinary @clientArgs
            if ($LASTEXITCODE -ne 0) { throw '只读API账号建立失败' }
            if ($LocalConversationBootstrap) {
                $env:MYSQL_PWD = $apiPassword
                try {
                    $metadataArgs = @($clientArgs | Where-Object { -not $_.StartsWith('--user=') }) + "--user=$apiUser"
                    $triggerRows = & $clientBinary @metadataArgs "--database=$localDatabase" --batch --skip-column-names --execute='CALL tm_ai_local_conversation_trigger_metadata()'
                    if ($LASTEXITCODE -ne 0 -or @($triggerRows).Count -ne 7) { throw '固定触发器元数据过程可见性未确认' }
                    $metadataCounts = & $clientBinary @metadataArgs "--database=$localDatabase" --batch --skip-column-names --execute="SELECT (SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE()),(SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name IN ('tm_ai_conversations','tm_ai_conversation_messages') AND constraint_type='CHECK' AND enforced='YES')"
                    $safeCounts = ($metadataCounts -join '').Trim()
                    if ($LASTEXITCODE -ne 0 -or $safeCounts -notmatch '^\d+\s+\d+$') { throw '固定元数据过程可见性查询失败' }
                    if ($safeCounts -ne "0`t2") { throw "元数据数量不符（直接触发器/CHECK）：$safeCounts" }
                } finally { $env:MYSQL_PWD = $testPassword }
            }
            $apiPath = Join-Path $testRoot 'api.exe'
            Push-Location (Join-Path $PSScriptRoot '../../server')
            try { go build -o $apiPath ./cmd/api; if ($LASTEXITCODE -ne 0) { throw '实际API构建失败' } } finally { Pop-Location }
            $apiStart = [Diagnostics.ProcessStartInfo]::new()
            $apiStart.FileName = $apiPath; $apiStart.UseShellExecute = $false; $apiStart.CreateNoWindow = $true
            $apiStart.RedirectStandardOutput = $true; $apiStart.RedirectStandardError = $true; $apiStart.Environment.Clear()
            foreach ($runtimeName in @('SystemRoot','WINDIR','PATH','PATHEXT','USERPROFILE','APPDATA','LOCALAPPDATA','TEMP','TMP')) {
                $runtimeValue = [Environment]::GetEnvironmentVariable($runtimeName, 'Process')
                if ($null -ne $runtimeValue) { $apiStart.Environment[$runtimeName] = $runtimeValue }
            }
            $apiStart.Environment['TM_AI_LOCAL_CONFIG'] = $localConfigPath
            $apiStart.Environment['TM_DB_DSN'] = "$($apiUser):$($apiPassword)@tcp(127.0.0.1:$testPort)/$($localDatabase)?parseTime=true&charset=utf8mb4&loc=UTC"
            if ($LocalPlanBootstrap) {
                $apiStart.Environment['TM_AI_LOCAL_PLANS'] = $localPlansPath
                $savedPlanDSN = $env:TM_DB_DSN
                try {
                    $env:TM_DB_DSN = "$($planReadUser):$($planReadPassword)@tcp(127.0.0.1:$testPort)/$($localDatabase)?parseTime=true&charset=utf8mb4&loc=UTC"
                    $planWorker = & $invokeLocalWorker -CheckPlans
                    if ($planWorker.Code -ne 0 -or $planWorker.Text -notmatch '本地合成资料计划与当前来源授权核验通过') {
                        $safeStage = [regex]::Match($planWorker.Text,'阶段(\d+)').Groups[1].Value
                        throw "受限只读账号实际worker计划核验未通过（阶段$safeStage）"
                    }
                    Write-Output $planWorker.Text.Trim()
                    & $clientBinary @clientArgs --execute="REVOKE SELECT ON $localDatabase.tm_ai_source_bodies FROM '$planReadUser'@'127.0.0.1'"
                    if ($LASTEXITCODE -ne 0) { throw '正文读取权限撤销反例建立失败' }
                    $blockedPlanWorker = & $invokeLocalWorker -CheckPlans
                    if ($blockedPlanWorker.Code -eq 0 -or $blockedPlanWorker.Text -notmatch '核验失败（阶段4') { throw '缺正文读取权限仍通过计划核验，或未在证据阶段明确拒绝' }
                    & $clientBinary @clientArgs --execute="GRANT SELECT ON $localDatabase.tm_ai_source_bodies TO '$planReadUser'@'127.0.0.1'"
                    if ($LASTEXITCODE -ne 0) { throw '正文只读权限恢复失败' }
                    $restoredPlanWorker = & $invokeLocalWorker -CheckPlans
                    if ($restoredPlanWorker.Code -ne 0) { throw '恢复只读权限后实际worker未通过' }
                    Write-Output 'LOCAL_PLAN_READONLY_WORKER_VERIFIED：实际worker只读核验通过/撤回正文读取权限拒绝/恢复后再次通过；不能改来源或正文。'
                } finally { $env:TM_DB_DSN = $savedPlanDSN }
            }
            if ($LocalConversationBootstrap) {
                $apiStart.Environment['TM_AI_TASK_READ_ENABLED'] = 'true'
                $apiStart.Environment['TM_AI_CONVERSATIONS_ENABLED'] = 'true'
                $apiStart.Environment['TM_JWT_SIGNING_KEY'] = $memberConfig.signing
                $apiStart.Environment['TM_JWT_ISSUER'] = 'GVA'
                $apiStart.Environment['TM_JWT_AUDIENCE'] = 'GVA'
            }
            if ($LocalTaskBootstrap) {
                foreach ($feature in @('TM_AI_TASK_CREATE_ENABLED','TM_AI_DEEP_SUBMIT_ENABLED','TM_AI_CONVERSATION_TASKS_ENABLED','TM_AI_TASK_CANCEL_ENABLED','TM_AI_POINTS_BALANCE_ENABLED','TM_AI_TASK_STREAM_ENABLED')) { $apiStart.Environment[$feature] = 'true' }
            }
            if ($LocalReportBootstrap) { $apiStart.Environment['TM_AI_REPORT_READ_ENABLED'] = 'true' }
            $apiProcess = [Diagnostics.Process]::new(); $apiProcess.StartInfo = $apiStart
            $httpHandler = [Net.Http.HttpClientHandler]::new(); $httpHandler.UseProxy = $false; $httpHandler.AllowAutoRedirect = $false
            $http = [Net.Http.HttpClient]::new($httpHandler); $http.Timeout = [TimeSpan]::FromSeconds(2)
            $apiStarted = $false
            try {
                if (-not $apiProcess.Start()) { throw '实际API未启动' }
                $apiStarted = $true
                $apiReady = $false
                for ($attempt = 0; $attempt -lt 40; $attempt++) {
                    if ($apiProcess.HasExited) { throw '实际API提前退出' }
                    # 仅在有界启动等候中容许短暂连接失败，后续业务请求错误不吞掉。
                    try { $response = $http.GetAsync("http://127.0.0.1:$apiPort/healthz").GetAwaiter().GetResult(); if ($response.StatusCode -eq 200) { $response.Dispose(); $apiReady = $true; break }; $response.Dispose() } catch { if ($apiProcess.HasExited) { throw '实际API启动失败' } }
                    Start-Sleep -Milliseconds 250
                }
                if (-not $apiReady) { throw '实际API未在期限内就绪' }
                $search = $http.GetStringAsync("http://127.0.0.1:$apiPort/api/v1/theme/search?kw=local-fixture").GetAwaiter().GetResult() | ConvertFrom-Json
                if ($search.code -ne 0 -or $search.data.Count -ne 0) { throw '空资料目录查询不符，不应生成或填充证券示例数据' }
                $closedPaths = @('/api/v1/ai/tasks','/api/v1/ai/conversations','/api/v1/ai/points/balance')
                if ($LocalConversationBootstrap) { $closedPaths = @('/api/v1/ai/points/balance'); & $invokeLocalMember "http://127.0.0.1:$apiPort" }
                if ($LocalTaskBootstrap) { $closedPaths = @('/api/v1/ai/tasks/1/report') }
                if ($LocalReportBootstrap) {
                    $closedPaths = @()
                    $anonymousReport = $http.GetAsync("http://127.0.0.1:$apiPort/api/v1/ai/tasks/1/report").GetAwaiter().GetResult()
                    try { if ([int]$anonymousReport.StatusCode -ne 401) { throw '报告读取实际路由未注册或绕过原会员鉴权' } } finally { $anonymousReport.Dispose() }
                }
                foreach ($closedPath in $closedPaths) {
                    $response = $http.GetAsync("http://127.0.0.1:$apiPort$closedPath").GetAwaiter().GetResult()
                    try { if ([int]$response.StatusCode -ne 404) { throw '默认关闭的AI入口意外开放' } } finally { $response.Dispose() }
                }
                $env:MYSQL_PWD = $apiPassword
                $apiArgs = @($clientArgs | Where-Object { -not $_.StartsWith('--user=') }) + "--user=$apiUser"
                $deniedQueries = @("INSERT INTO $localDatabase.addon_quant_theme(name) VALUES('forbidden')", "UPDATE $localDatabase.tm_ai_local_instance SET contains_real_users=1", "DROP TABLE $localDatabase.addon_quant_theme", 'SELECT User FROM mysql.user')
                if ($LocalPlanBootstrap) { $deniedQueries += @("UPDATE $localDatabase.tm_ai_sources SET enabled=0", "UPDATE $localDatabase.tm_ai_source_bodies SET body_text=body_text") }
                if ($LocalConversationBootstrap) {
                    $deniedQueries += @("SELECT password FROM $localDatabase.addon_member", "UPDATE $localDatabase.addon_member SET status=0 WHERE id=1", "DROP TRIGGER $localDatabase.tm_ai_conversation_insert")
                    if (-not $LocalTaskBootstrap) { $deniedQueries += @("INSERT INTO $localDatabase.tm_ai_tasks(id) VALUES(1)", "INSERT INTO $localDatabase.tm_points_txns(id) VALUES(1)") }
                }
                foreach ($deniedSQL in $deniedQueries) {
                    $deniedOutput = & $clientBinary @apiArgs --execute=$deniedSQL 2>&1
                    if ($LASTEXITCODE -eq 0 -or ($deniedOutput -join "`n") -notmatch '\b(1044|1142|1143)\b') { throw '只读API账号权限边界未确认' }
                }
                if ($LocalTaskBootstrap) {
                    foreach ($protectedSQL in @("DELETE FROM $localDatabase.tm_ai_tasks", "DELETE FROM $localDatabase.tm_points_txns", "UPDATE $localDatabase.tm_points_txns SET updated_at=updated_at", "DELETE FROM $localDatabase.tm_ai_conversation_tasks")) {
                        $protectedOutput = & $clientBinary @apiArgs --execute=$protectedSQL 2>&1
                        if ($LASTEXITCODE -eq 0 -or ($protectedOutput -join "`n") -notmatch '\b1644\b') { throw 'API写账号未由真实不可变触发器拒绝破坏历史' }
                    }
                }
                if ($apiProcess.HasExited) { throw '验收期间实际API意外退出' }
            } finally {
                $env:MYSQL_PWD = $testPassword
                $http.Dispose(); $httpHandler.Dispose()
                if ($apiStarted) {
                    if (-not $apiProcess.HasExited) { $apiProcess.Kill(); $apiProcess.WaitForExit() }
                    [IO.File]::WriteAllText((Join-Path $testRoot 'local-api.log'), ($apiProcess.StandardOutput.ReadToEnd() + $apiProcess.StandardError.ReadToEnd()))
                }
                $apiProcess.Dispose()
            }
            if ($LocalConversationBootstrap) {
                # API已停止；仅由本轮迁移所有者制造结构反例，运行账号不获修复或DDL权限。
                $invokeConversationStartup = {
                    param([int]$expectedStage,[switch]$Intent)
                    $probeProcess = [Diagnostics.Process]::new(); $probeProcess.StartInfo = $apiStart
                    $probeHandler = [Net.Http.HttpClientHandler]::new(); $probeHandler.UseProxy = $false; $probeHandler.AllowAutoRedirect = $false
                    $probeHTTP = [Net.Http.HttpClient]::new($probeHandler); $probeHTTP.Timeout = [TimeSpan]::FromSeconds(2)
                    $probeStarted = $false
                    try {
                        if (-not $probeProcess.Start()) { throw '会话启动反例进程未启动' }
                        $probeStarted = $true
                        if ($expectedStage -gt 0) {
                            if (-not $probeProcess.WaitForExit(15000)) { throw '损坏会话结构未在期限内拒绝启动' }
                            $probeOutput = $probeProcess.StandardOutput.ReadToEnd() + $probeProcess.StandardError.ReadToEnd()
                            $expectedPrefix = '会话保护自检失败'; if ($Intent) { $expectedPrefix = '会话任务保护自检失败' }
                            if ($probeProcess.ExitCode -eq 0 -or $probeOutput -notmatch "$expectedPrefix：阶段$expectedStage（内部详情已隐藏）") { throw '会话启动反例没有在预期保护阶段明确拒绝' }
                        } else {
                            $probeReady = $false
                            for ($attempt = 0; $attempt -lt 40; $attempt++) {
                                if ($probeProcess.HasExited) { throw '恢复后实际API提前退出' }
                                try { $probeResponse = $probeHTTP.GetAsync("http://127.0.0.1:$apiPort/healthz").GetAwaiter().GetResult(); try { if ($probeResponse.StatusCode -eq 200) { $probeReady = $true; break } } finally { $probeResponse.Dispose() } } catch { if ($probeProcess.HasExited) { throw '恢复后实际API启动失败' } }
                                Start-Sleep -Milliseconds 250
                            }
                            if (-not $probeReady) { throw '恢复后实际API未在期限内就绪' }
                        }
                    } finally {
                        $probeHTTP.Dispose(); $probeHandler.Dispose()
                        if ($probeStarted -and -not $probeProcess.HasExited) { $probeProcess.Kill(); $probeProcess.WaitForExit() }
                        $probeProcess.Dispose()
                    }
                }
                & $clientBinary @clientArgs "--database=$localDatabase" --execute='DROP PROCEDURE tm_ai_local_conversation_trigger_metadata'
                if ($LASTEXITCODE -ne 0) { throw '元数据过程缺失反例建立失败' }
                & $invokeConversationStartup 2
                # 重建后重新授权，MySQL DROP ROUTINE会一并移除专属EXECUTE授权。
                "$conversationMetadataDDL GRANT EXECUTE ON PROCEDURE $localDatabase.tm_ai_local_conversation_trigger_metadata TO '$apiUser'@'127.0.0.1';" | & $clientBinary @clientArgs "--database=$localDatabase"
                if ($LASTEXITCODE -ne 0) { throw '固定元数据过程恢复失败' }
                & $clientBinary @clientArgs "--database=$localDatabase" --execute='DROP TRIGGER tm_ai_conversation_no_delete; CREATE TRIGGER tm_ai_conversation_no_delete BEFORE DELETE ON tm_ai_conversations FOR EACH ROW SET @tm_ai_local_noop=1'
                if ($LASTEXITCODE -ne 0) { throw '同名失效触发器反例建立失败' }
                & $invokeConversationStartup 3
                $conversationDDL = [IO.File]::ReadAllText((Join-Path $migrationDirectory '20261021_create_tm_ai_conversations.sql'))
                $restoreTrigger = [regex]::Match($conversationDDL, '(?m)^CREATE TRIGGER tm_ai_conversation_no_delete .*;\r?$')
                if (-not $restoreTrigger.Success) { throw '原不可删除触发器定义未匹配，禁止猜测恢复' }
                ('DROP TRIGGER tm_ai_conversation_no_delete; ' + $restoreTrigger.Value) | & $clientBinary @clientArgs "--database=$localDatabase"
                if ($LASTEXITCODE -ne 0) { throw '原不可删除触发器恢复失败' }
                & $invokeConversationStartup 0
                if ($LocalTaskBootstrap) {
                    & $clientBinary @clientArgs "--database=$localDatabase" --execute='DROP PROCEDURE tm_ai_local_intent_trigger_metadata'
                    if ($LASTEXITCODE -ne 0) { throw '意图元数据缺失反例建立失败' }
                    & $invokeConversationStartup 2 -Intent
                    "$intentMetadataDDL GRANT EXECUTE ON PROCEDURE $localDatabase.tm_ai_local_intent_trigger_metadata TO '$apiUser'@'127.0.0.1';" | & $clientBinary @clientArgs "--database=$localDatabase"
                    if ($LASTEXITCODE -ne 0) { throw '意图元数据过程恢复失败' }
                    & $clientBinary @clientArgs "--database=$localDatabase" --execute='DROP TRIGGER tm_ai_intent_no_delete; CREATE TRIGGER tm_ai_intent_no_delete BEFORE DELETE ON tm_ai_conversation_tasks FOR EACH ROW SET @tm_ai_local_noop=1'
                    if ($LASTEXITCODE -ne 0) { throw '同名无效意图触发器建立失败' }
                    & $invokeConversationStartup 3 -Intent
                    $intentDDL = [IO.File]::ReadAllText((Join-Path $migrationDirectory '20261021_create_tm_ai_conversations_tasks.sql'))
                    $restoreIntentTrigger = [regex]::Match($intentDDL,'(?m)^CREATE TRIGGER tm_ai_intent_no_delete .*;\r?$')
                    if (-not $restoreIntentTrigger.Success) { throw '原意图删除保护定义未匹配' }
                    ('DROP TRIGGER tm_ai_intent_no_delete; ' + $restoreIntentTrigger.Value) | & $clientBinary @clientArgs "--database=$localDatabase"
                    if ($LASTEXITCODE -ne 0) { throw '原意图保护触发器恢复失败' }
                    & $invokeConversationStartup 0
                    Write-Output 'LOCAL_INTENT_STARTUP_GUARDS_VERIFIED：缺过程阶段2拒绝/同名无效删除保护阶段3拒绝/原定义恢复后API重启成功。'
                }
                $conversationRows = & $clientBinary @clientArgs "--database=$localDatabase" --batch --skip-column-names --execute='SELECT (SELECT COUNT(*) FROM tm_ai_conversations),(SELECT COUNT(*) FROM tm_ai_conversation_messages)'
                $expectedConversationRows = "2`t0"; if ($LocalTaskBootstrap) { $expectedConversationRows = "3`t2" }
                if ($LASTEXITCODE -ne 0 -or ($conversationRows -join '').Trim() -ne $expectedConversationRows) { throw '结构反例/恢复启动留下了探针数据或更改测试会话' }
                Write-Output 'LOCAL_CONVERSATION_STARTUP_GUARDS_VERIFIED：缺元数据过程在阶段2拒绝/同名失效触发器在阶段3拒绝/恢复原定义后实际API重新就绪/探针零残留。'
            }
            if ($LocalTaskBootstrap) {
                $taskAccounting = & $clientBinary @clientArgs "--database=$localDatabase" --batch --skip-column-names --execute='SELECT (SELECT COUNT(*) FROM tm_ai_tasks),(SELECT COUNT(*) FROM tm_points_txns),(SELECT COUNT(*) FROM tm_points_accounts),(SELECT SUM(avail) FROM tm_points_accounts),(SELECT SUM(frozen) FROM tm_points_accounts),(SELECT SUM(debt) FROM tm_points_accounts),(SELECT COUNT(*) FROM tm_ai_agent_runs),(SELECT COUNT(*) FROM tm_ai_report_commits)'
                if ($LASTEXITCODE -ne 0 -or ($taskAccounting -join '').Trim() -ne "2`t6`t2`t100`t0`t0`t0`t0") { throw '实际任务/测试积分最终账务或零执行存证不符' }
                Write-Output 'LOCAL_TASK_RUNTIME_VERIFIED：两任务/六流水/双会员总可消费100且冻结与欠款0/零Agent与报告提交/实际API与SSE/受限写账号历史保护均通过。'
            } else {
                $uncharged = & $clientBinary @clientArgs "--database=$localDatabase" --batch --skip-column-names --execute='SELECT (SELECT COUNT(*) FROM tm_ai_tasks)+(SELECT COUNT(*) FROM tm_points_txns)'
                if ($LASTEXITCODE -ne 0 -or ($uncharged -join '').Trim() -ne '0') { throw '只读API启动生成了任务或积分流水' }
            }
            if ($LocalTaskBootstrap) {
                Write-Output '任务验收仅使用本轮初始化的测试积分，真实模型/报告生成仍关闭，不等于成功报告结算已验。'
                if ($LocalReportBootstrap) { Write-Output 'LOCAL_REPORT_API_READ_GUARDS_VERIFIED：实际API装配同源发布读取控制器，匿名拒绝/跨会员拒绝/已取消任务不可读取通过；没有生成或伪造可交付报告。' }
            }
            elseif ($LocalConversationBootstrap) { Write-Output 'LOCAL_CONVERSATION_PERMISSIONS_VERIFIED：会话专属写权限/固定只读元数据过程/无TRIGGER或DDL/不可读密码或改会员/不可写任务账本/零任务零积分流水均通过。' }
            elseif ($LocalPlanBootstrap) { Write-Output "LOCAL_PLAN_RUNTIME_VERIFIED：$($migrations.Count)项原迁移/明确合成资料/同源计划配置/受限只读worker/实际API健康与查询/任务入口仍404/零任务零积分流水均通过。" }
            else { Write-Output "LOCAL_API_BOOTSTRAP_VERIFIED：$($migrations.Count)项原迁移/空测试目录/受限只读账号/实际API健康与目录查询/AI入口默认404/零任务零积分流水均通过。" }
        }
        Write-Output 'LOCAL_BOOTSTRAP_VERIFIED：新库/实例标记/仅SELECT核验账号/实际worker/重复初始化拒绝/四项越权拒绝均通过；其它能力以本轮专项结果为准，未启用任务执行。'
        $passed = $true
        return
    }
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
        if ($LocalAPIWorkerBrowser) { $integrationTags = 'integration,browser' }
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
