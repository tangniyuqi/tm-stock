<template>
  <div class="quant-dashboard">
    <div class="w-full h-0.5 bg-slate-900 dark:bg-slate-800 sticky top-0 z-50"></div>

    <el-header class="header bg-slate-900 sticky top-0.5">
      <div class="logo">
        <el-icon :size="24"><TrendCharts /></el-icon>
        <span>
          智子量化 - 多任务自动交易系统
          <span class="version" @click="router.push('/layout/quant-client/update')">
            <el-icon class="ml-3" v-if="!isPyWebView"><Download /></el-icon>
            {{ isPyWebView ? clientVersion : '客户端下载' }}
          </span>
        </span>
      </div>

      <div class="user-info" style="display: flex; align-items: center; gap: 10px">
        <el-tooltip v-if="!route.path.includes('/layout')" class="" effect="dark" content="切换主题" placement="bottom">
          <span class="w-8 h-8 p-2 rounded-full flex items-center justify-center shadow border border-gray-200 dark:border-gray-600 cursor-pointer border-solid">
            <el-icon v-if="appStore.isDark" @click="appStore.toggleTheme(false)">
              <Sunny />
            </el-icon>
            <el-icon v-else @click="appStore.toggleTheme(true)">
              <Moon />
            </el-icon>
          </span>
        </el-tooltip>

        <el-button v-if="!route.path.includes('/layout')" class="switch-mode-btn" size="small" round @click="router.push('/layout/quant/console')">
          <el-icon><Grid /></el-icon>
          切换到标准版
        </el-button>
        <el-button v-else class="switch-mode-btn" size="small" round @click="router.push('/quant')">
          <el-icon><Monitor /></el-icon>
          切换到简洁版
        </el-button>
      </div>
    </el-header>

    <div class="main-content">
      <el-row :gutter="20">
        <el-col v-for="element in layoutColumns" :key="element.id" :span="element.span">
          <div v-if="element.id === 'main'">
            <Draggable v-model="dashboardComponents" item-key="id" handle=".card-header, .terminal-header" :animation="200" ghost-class="ghost-card" drag-class="drag-card" :force-fallback="true">
              <template #item="{ element }">
                <div class="draggable-item">
                  <el-card v-if="element.id === 'account'" class="box-card dark:bg-slate-900 mb-20" shadow="never">
                    <template #header>
                      <div class="card-header" style="cursor: move">
                        <span class="title">
                          <el-icon><CreditCard /></el-icon>
                          证券账户
                          <span class="account-info" v-show="accountData.account_no">
                            - {{ accountData.broker }}：{{ maskedAccountNo }}
                            <span class="expiration-date ml-3" style="color: red" v-if="isExpired">授权已到期</span>
                            <el-tooltip :content="`账户于${formatToDateTime(accountData.expiration_date, 'YYYY年MM月DD日')}到期，到期后将无法创建任务、启动任务等操作。`.replace('T', ' ')">
                              <el-icon class="ml-2"><InfoFilled /></el-icon>
                            </el-tooltip>
                          </span>

                          <el-tag type="danger" round class="ml-2" v-show="!accountData.account_no">未绑定证券账户</el-tag>
                        </span>

                        <div class="header-controls">
                          <el-tooltip :content="`成功连接服务器才可正常运行任务，最后更新于：${formatToDateTime(accountData.updated_at, 'YYYY年MM月DD日 HH时mm分ss秒')}`">
                            <el-tag v-if="serverStatus.text" :type="serverStatus.type" effect="light" round>
                              <span style="display: flex; align-items: center">
                                <el-icon style="margin-right: 4px"><Monitor /></el-icon>
                                {{ serverStatus.text }}
                              </span>
                            </el-tag>
                          </el-tooltip>

                          <el-tooltip :content="`测试服务器状态是否正常`">
                            <el-button :icon="Lightning" link class="ml-2" @click="checkServerStatus(false)" />
                          </el-tooltip>

                          <el-tooltip :content="`修改配置未生效时，可刷新账户配置`">
                            <el-button :icon="Refresh" link class="ml-2" @click="fetchAccountData" />
                          </el-tooltip>

                          <el-tooltip :content="`切换显示/隐藏资金`">
                            <el-button link class="ml-5" @click="showFunds = !showFunds">
                              <el-icon>
                                <View v-if="!showFunds" />
                                <Hide v-if="showFunds" />
                              </el-icon>
                            </el-button>
                          </el-tooltip>

                          <el-button :icon="Setting" circle plain class="ml-4" @click="openAccountSettings" title="账户设置" />

                          <el-tooltip :content="sidebarVisible ? '隐藏右侧边栏' : '显示右侧边栏'">
                            <el-button link class="ml-2" @click="toggleSidebar">
                              <el-icon>
                                <DArrowRight v-if="sidebarVisible" />
                                <DArrowLeft v-else />
                              </el-icon>
                            </el-button>
                          </el-tooltip>
                        </div>
                      </div>
                    </template>
                    <el-row :gutter="20" class="account-metrics">
                      <el-col :span="4">
                        <div class="metric-item">
                          <div class="value primary">{{ showFunds ? formatSmartNumber(accountData.total_asset) : '****' }}</div>
                          <div class="label">总资产 (元)</div>
                        </div>
                      </el-col>
                      <el-col :span="4">
                        <div class="metric-item">
                          <div class="value">{{ showFunds ? formatSmartNumber(accountData.market_value) : '****' }}</div>
                          <div class="label">持仓市值 (元)</div>
                        </div>
                      </el-col>
                      <el-col :span="4">
                        <div class="metric-item">
                          <div class="value">{{ showFunds ? formatSmartNumber(accountData.available_balance) : '****' }}</div>
                          <div class="label">可用资金 (元)</div>
                        </div>
                      </el-col>
                      <el-col :span="4">
                        <div class="metric-item">
                          <div class="value">{{ showFunds ? formatSmartNumber(accountData.withdrawable_cash) : '****' }}</div>
                          <div class="label">可取资金 (元)</div>
                        </div>
                      </el-col>
                      <el-col :span="4">
                        <div class="metric-item">
                          <div class="value">
                            {{ showFunds ? (accountData.total_pl_amount >= 0 ? '+' : '') + formatSmartNumber(accountData.total_pl_amount) : '****' }}
                          </div>
                          <div class="label">持仓盈亏 (元)</div>
                        </div>
                      </el-col>
                      <el-col :span="4">
                        <div class="metric-item">
                          <div class="value">
                            <div :class="[accountData.daily_pl_amount >= 0 ? 'text-red' : 'text-green']">
                              {{ showFunds ? (accountData.daily_pl_amount >= 0 ? '+' : '') + formatSmartNumber(accountData.daily_pl_amount) : '****' }}
                              <span style="font-size: 14px" v-if="showFunds">{{ (accountData.daily_pl_ratio >= 0 ? '+' : '') + Number(accountData.daily_pl_ratio).toFixed(2) }}%</span>
                            </div>
                          </div>
                          <div class="label">今日盈亏 (元)</div>
                        </div>
                      </el-col>
                    </el-row>
                  </el-card>

                  <el-card v-if="element.id === 'task'" class="box-card dark:bg-slate-900 mb-20" shadow="never" :body-style="{ padding: '10px 20px' }">
                    <template #header>
                      <div class="card-header" style="cursor: move">
                        <span class="title">
                          <el-icon><Grid /></el-icon>
                          交易任务
                          <el-tooltip :content="`当前运行任务数：${accountData.running_task_count}，最大运行任务数：${accountData.max_running_task}`" placement="top">
                            <el-tag :type="accountData.running_task_count > 0 ? 'success' : 'info'" :effect="accountData.running_task_count > 0 ? 'dark' : 'plain'" round class="ml-3 font-normal">当前运行任务：{{ accountData.running_task_count }} / {{ accountData.max_running_task }}</el-tag>
                          </el-tooltip>
                        </span>
                        <div class="header-actions">
                          <el-button type="primary" @click="openDrawer('create')">创建任务</el-button>
                        </div>
                      </div>
                    </template>

                    <div class="task-info" style="display: flex; justify-content: space-between; align-items: center">
                      <div class="actions">
                        <el-button type="primary" plain :loading="isRunningSelected" @click="runSelectedTask">批量启动</el-button>
                        <el-button type="warning" plain :loading="isStoppingSelected" @click="stopSelectedTask">批量停止</el-button>
                        <el-button type="info" plain @click="deleteSelectedTask">删除</el-button>
                      </div>

                      <div class="search" style="display: flex; align-items: center">
                        <el-button :icon="Refresh" link plain title="刷新任务列表" @click="getTradeTasks" />

                        <el-input v-model="taskSearchKeyword" placeholder="搜索任务名称/标的" style="width: 200px; margin-left: 10px" clearable @clear="getTradeTasks" @keyup.enter="getTradeTasks">
                          <template #append>
                            <el-button :icon="Search" @click="getTradeTasks" />
                          </template>
                        </el-input>
                      </div>
                    </div>

                    <el-table :data="tradeTasks" class="mt-3" stripe @selection-change="handleSelectionChange">
                      <el-table-column type="expand">
                        <template #default="props">
                          <div style="padding: 10px 20px" v-if="getPositions(props.row).length > 0">
                            <div v-for="(pos, index) in getPositions(props.row)" :key="index" :style="{ marginBottom: index < getPositions(props.row).length - 1 ? '15px' : '0' }">
                              <div style="margin-bottom: 8px; font-weight: bold; font-size: 14px; border-left: 3px solid #409eff; padding-left: 8px">
                                {{ pos.stock_name || props.row.stock?.name || '-' }}
                                <span style="font-weight: normal; font-size: 13px; color: #666; margin-left: 5px">({{ pos.ts_code || props.row.stock?.ts_code || '-' }})</span>
                              </div>
                              <el-descriptions :column="4" border size="small">
                                <el-descriptions-item label="持仓市值">￥{{ pos.market_value ? Number(pos.market_value).toFixed(2) : '0' }}</el-descriptions-item>
                                <el-descriptions-item label="持仓比例">
                                  {{ pos.position_ratio ? Number(pos.position_ratio).toFixed(2) + '%' : '0%' }}
                                </el-descriptions-item>
                                <el-descriptions-item label="持仓数量">{{ pos.total_quantity ?? '-' }}（股）</el-descriptions-item>
                                <el-descriptions-item label="可用数量">{{ pos.available_quantity ?? '-' }}（股）</el-descriptions-item>
                                <el-descriptions-item label="成本价">￥{{ pos.cost_price ? Number(pos.cost_price).toFixed(3) : '0' }}</el-descriptions-item>
                                <el-descriptions-item label="现价">￥{{ pos.current_price ? Number(pos.current_price).toFixed(3) : '-' }}</el-descriptions-item>
                                <el-descriptions-item label="冻结数量">{{ pos.frozen_quantity ?? '-' }}（股）</el-descriptions-item>
                                <el-descriptions-item label="今日买/卖">
                                  <span class="text-red">{{ pos.daily_buy_quantity ?? 0 }}（股）</span>
                                  /
                                  <span class="text-green">{{ pos.daily_sell_quantity ?? 0 }}（股）</span>
                                </el-descriptions-item>
                                <el-descriptions-item label="盈亏">
                                  <span :class="[(pos.total_pl_amount || pos.profit || 0) >= 0 ? 'text-red' : 'text-green']">￥{{ pos.total_pl_amount ? Number(pos.total_pl_amount).toFixed(2) : pos.profit ? Number(pos.profit).toFixed(2) : '-' }}</span>
                                </el-descriptions-item>
                                <el-descriptions-item label="收益率">
                                  <span :class="[(pos.total_pl_ratio || 0) >= 0 ? 'text-red' : 'text-green']">
                                    {{ pos.total_pl_ratio !== undefined ? Number(pos.total_pl_ratio).toFixed(2) + '%' : '-' }}
                                  </span>
                                </el-descriptions-item>
                                <el-descriptions-item label="当日盈亏">
                                  <span :class="[(pos.daily_pl_amount || 0) >= 0 ? 'text-red' : 'text-green']">￥{{ pos.daily_pl_amount !== undefined ? Number(pos.daily_pl_amount).toFixed(2) : '-' }}</span>
                                </el-descriptions-item>
                                <el-descriptions-item label="当日收益率">
                                  <span :class="[(pos.daily_pl_ratio || 0) >= 0 ? 'text-red' : 'text-green']">
                                    {{ pos.daily_pl_ratio !== undefined ? Number(pos.daily_pl_ratio).toFixed(2) + '%' : '-' }}
                                  </span>
                                </el-descriptions-item>
                              </el-descriptions>
                            </div>
                          </div>
                          <div style="padding: 10px 20px" v-else>
                            <el-empty description="暂无持仓" :image-size="60"></el-empty>
                          </div>
                        </template>
                      </el-table-column>

                      <el-table-column fixed type="selection" width="40" />
                      <el-table-column fixed prop="id" label="#" width="80" />

                      <el-table-column fixed prop="name" label="任务名称" min-width="130" max-width="360" show-overflow-tooltip>
                        <template #default="scope">
                          <div class="row-item">
                            <div style="display: flex; align-items: center">
                              <div class="name">{{ scope.row.name }}</div>
                              <el-tooltip v-if="scope.row.remark" :content="scope.row.remark" placement="top">
                                <el-icon style="margin-left: 4px; cursor: pointer; color: #909399"><InfoFilled /></el-icon>
                              </el-tooltip>
                            </div>
                            <div class="status" v-if="scope.row.status === 1">
                              <el-tag type="success" size="small" effect="dark">运行中</el-tag>
                            </div>
                          </div>
                        </template>
                      </el-table-column>

                      <el-table-column prop="strategy_name" label="策略类型" min-width="130" max-width="180" show-overflow-tooltip />

                      <el-table-column prop="stock" label="任务标的" min-width="100" max-width="150">
                        <template #default="scope">
                          <div class="row-item">
                            <div class="name">{{ scope.row.symbol_name }}</div>
                            <div class="symbol">{{ scope.row.symbol_code }}</div>
                          </div>
                        </template>
                      </el-table-column>

                      <el-table-column label="任务有效期" width="150" align="center">
                        <template #default="scope">
                          <div class="row-item">
                            <el-tooltip v-if="scope.row.config?.validityPeriod" :content="getValidityTooltip(scope.row.config.validityPeriod)" placement="top">
                              <el-tag :type="getValidityType(scope.row.config.validityPeriod)" size="small" effect="plain">
                                {{ scope.row.config.validityPeriod }}
                              </el-tag>
                            </el-tooltip>
                            <span v-else>-</span>
                          </div>
                        </template>
                      </el-table-column>

                      <el-table-column prop="created_at" label="创建/更新时间" width="150">
                        <template #default="scope">
                          <div class="row-item">
                            <el-text type="info">{{ scope.row.created_at }}</el-text>
                          </div>
                          <div class="row-item">
                            <el-text type="info">{{ scope.row.updated_at }}</el-text>
                          </div>
                        </template>
                      </el-table-column>

                      <!-- <el-table-column prop="created_at" label="创建时间" width="120">
                          <template #default="scope">
                            <div class="row-item">{{ scope.row.created_at }}</div>
                          </template>
                        </el-table-column> -->

                      <el-table-column fixed="right" width="150">
                        <template #default="scope">
                          <el-button type="primary" size="small" plain @click="runTask(scope.row.id)" v-if="scope.row.status !== 1" :loading="taskActionLoading[scope.row.id] === 'starting'">启动</el-button>
                          <el-button type="warning" size="small" plain @click="stopTask(scope.row.id)" v-if="scope.row.status === 1" :loading="taskActionLoading[scope.row.id] === 'stopping'">停止</el-button>
                          <el-button size="small" plain @click="openDrawer('update', scope.row)">配置</el-button>
                          <!-- <el-button size="small" plain :disabled="scope.row.status === 1" :title="scope.row.status === 1 ? '任务停止时才能编辑' : ''" @click="openDrawer('update', scope.row)">配置</el-button> -->
                        </template>
                      </el-table-column>
                    </el-table>

                    <div class="gva-pagination">
                      <el-pagination size="small" layout="total, sizes, prev, pager, next, jumper" :current-page="taskPage" :page-size="taskPageSize" :page-sizes="[1, 2, 5, 10, 20, 30, 50, 100]" :total="taskTotal" @current-change="handleTaskPageChange" @size-change="handleTaskSizeChange" />
                    </div>
                  </el-card>

                  <el-card v-if="element.id === 'record'" class="box-card dark:bg-slate-900 mb-20" shadow="never" :body-style="{ padding: '10px 20px' }">
                    <template #header>
                      <div class="card-header" style="cursor: move">
                        <span class="title">
                          <el-icon><Coin /></el-icon>
                          委托记录
                          <el-tooltip content="此为任务策略触发指令委托记录，实际是否成交以券商记录为准。">
                            <el-icon><InfoFilled /></el-icon>
                          </el-tooltip>
                        </span>
                      </div>
                    </template>

                    <div class="text-align-right">
                      <el-button :icon="Refresh" link plain title="刷新委托记录" @click="getTradeRecords" />

                      <el-select v-model="recordAction" placeholder="全部" style="width: 80px; margin: 0 10px" clearable @change="getTradeRecords">
                        <el-option label="买入" value="buy" />
                        <el-option label="卖出" value="sell" />
                      </el-select>

                      <el-input v-model="recordSearchKeyword" placeholder="搜索标的/委托号" style="width: 200px" clearable @clear="getTradeRecords" @keyup.enter="getTradeRecords">
                        <template #append>
                          <el-button :icon="Search" @click="getTradeRecords" />
                        </template>
                      </el-input>
                    </div>

                    <el-table :data="tradeRecords" class="mt-3" stripe>
                      <el-table-column fixed prop="id" label="#" width="80" />
                      <el-table-column fixed prop="time" label="委托时间" width="90" />
                      <el-table-column fixed prop="symbol" label="交易标的" min-width="100">
                        <template #default="scope">
                          <div class="row-item">
                            <div class="name">{{ scope.row.name }}</div>
                            <div class="symbol">{{ scope.row.symbol }}</div>
                          </div>
                        </template>
                      </el-table-column>

                      <el-table-column prop="action" label="交易指令" width="90">
                        <template #default="scope">
                          <el-tag :type="scope.row.action === 'buy' ? 'danger' : 'success'" effect="plain" size="small">
                            {{ scope.row.action === 'buy' ? '买入' : '卖出' }}
                          </el-tag>
                        </template>
                      </el-table-column>

                      <el-table-column prop="price" label="价格 (元)" width="90" />
                      <el-table-column prop="quantity" label="数量 (股)" width="90" />
                      <el-table-column prop="amount" label="金额 (元)" width="90" />
                      <el-table-column prop="reason" label="原因" width="120" />

                      <el-table-column prop="task_name" label="所属任务" min-width="120">
                        <template #default="scope">
                          <div class="row-item">
                            <div class="name">{{ scope.row.task_name }}</div>
                          </div>
                        </template>
                      </el-table-column>

                      <el-table-column prop="date" label="委托日期" width="110">
                        <template #default="scope">
                          <el-text type="info">{{ scope.row.date }}</el-text>
                        </template>
                      </el-table-column>
                    </el-table>

                    <div class="gva-pagination">
                      <el-pagination size="small" layout="total, sizes, prev, pager, next, jumper" :current-page="recordPage" :page-sizes="[1, 2, 5, 10, 20, 30, 50, 100]" :total="recordTotal" @current-change="handleTradeRecordPageChange" @size-change="handleTradeRecordSizeChange" />
                    </div>
                  </el-card>

                  <div ref="terminalRef" v-if="element.id === 'terminal'" class="terminal" style="height: 400px">
                    <div class="terminal-header" style="cursor: move">
                      <span class="terminal-dot red"></span>
                      <span class="terminal-dot yellow"></span>
                      <span class="terminal-dot green"></span>
                      <span class="terminal-title">智子量化交易终端</span>
                      <div class="terminal-actions">
                        <el-button type="primary" size="small" link plain @click="copyConsoleLogs">
                          <el-icon><CopyDocument /></el-icon>
                          拷贝
                        </el-button>
                        <el-button type="danger" size="small" link plain @click="clearConsoleLogs">
                          <el-icon><Delete /></el-icon>
                          清空
                        </el-button>
                        <el-switch v-model="autoScrollConsole" size="small" active-text="滚动" inactive-text="关闭" inline-prompt title="打开/关闭自动滚动" class="ml-3" />
                      </div>
                    </div>
                    <div class="terminal-body">
                      <div v-for="(log, index) in consoleLogs" :key="index" :class="['terminal-line', `log-${log.level.toLowerCase()}`]">
                        <span class="terminal-prompt">[{{ log.time }}]</span>
                        <span class="terminal-module">{{ log.module }}:</span>
                        <span class="terminal-message">{{ log.message }}</span>
                      </div>
                      <div class="terminal-line prompt-line">
                        <span class="terminal-prompt">></span>
                        <span class="terminal-cursor"></span>
                      </div>
                    </div>
                  </div>
                </div>
              </template>
            </Draggable>
          </div>

          <div v-if="element.id === 'sidebar'">
            <el-card class="box-card" shadow="never">
              <template #header>
                <div class="card-header market-info-header">
                  <div class="market-time-info">
                    <div class="time-row">
                      <span class="time">{{ currentTime }}</span>

                      <div class="market-status ml-2 mb-1">
                        <el-tag :type="marketStatus.type" effect="dark" size="small" round>
                          {{ marketStatus.text }}
                        </el-tag>
                      </div>
                    </div>

                    <div class="date-row mt-1">
                      <span class="date">{{ currentDate }}</span>
                      <span class="week ml-2">{{ currentWeek }}</span>
                      <el-tag size="small" :type="isTradingDay ? 'primary' : 'info'" effect="light" class="ml-2 mb-1" round>
                        {{ isTradingDay ? '交易日' : '休市日' }}
                      </el-tag>
                    </div>
                  </div>
                </div>
              </template>

              <el-tabs v-model="activeInfoTab" type="border-card">
                <el-tab-pane label="7x24 财经快报" name="news">
                  <div style="margin-bottom: 15px">
                    <el-input v-model="newsKeyword" placeholder="搜索快报内容..." clearable prefix-icon="Search" @keyup.enter="getNews" @clear="getNews">
                      <template #append>
                        <el-button :icon="Search" @click="getNews" />
                      </template>
                    </el-input>
                  </div>

                  <div class="news-list">
                    <div v-if="newsItems.length > 0">
                      <div class="news-item" v-for="(item, index) in newsItems" :key="index" @dblclick="copyNews(item)">
                        <span class="news-time">{{ formatToDateTime(item.ctime, 'HH:mm:ss') }} - </span>
                        <span class="news-text" v-html="highlightKeywords(useFormatText(item.content), newsKeyword)"></span>

                        <el-link
                          icon="ChatRound"
                          :href="`https://www.doubao.com/chat/url-action?action=${encodeURIComponent(
                            JSON.stringify({
                              pluginId: 'Send_Message',
                              payload: {
                                up_template_key: 'address_bar_up',
                                text: item.content,
                                reportParams: { is_address: 1, scene: 'address_bar' },
                              },
                            })
                          )}`"
                          target="_blank"
                          class="ml-2"
                          title="打开豆包对话"
                        />
                        <el-link icon="CopyDocument" class="ml-2" underline="never" title="复制内容" @click="copyNews(item)" />
                      </div>
                    </div>

                    <div class="empty-tip" v-else>
                      <el-empty description="暂无快报数据" />
                    </div>
                  </div>
                </el-tab-pane>

                <el-tab-pane label="最近60天大事件" name="event">
                  <div style="margin-bottom: 15px">
                    <el-input v-model="eventKeyword" placeholder="搜索事件名称..." clearable prefix-icon="Search" @keyup.enter="getEvents" @clear="getEvents">
                      <template #append>
                        <el-button :icon="Search" @click="getEvents" />
                      </template>
                    </el-input>
                  </div>
                  <div class="event-list">
                    <div v-for="(item, index) in eventItems" :key="index" class="event-item" @dblclick="copyEvent(item)" v-if="eventItems.length > 0">
                      <div class="event-date">
                        <span class="day">{{ formatToDateTime(item.date, 'DD') }}</span>
                        <span class="month">{{ formatToDateTime(item.date, 'MM') }}月</span>
                      </div>

                      <div class="event-detail">
                        <div class="event-title">
                          <span v-html="highlightKeywords(item.name, eventKeyword)"></span>
                          <el-link icon="Search" :href="`//baidu.com/s?wd=${item.name}`" target="_blank" class="ml-2" title="打开百度查询" />

                          <el-link
                            icon="ChatRound"
                            :href="`https://www.doubao.com/chat/url-action?action=${encodeURIComponent(
                              JSON.stringify({
                                pluginId: 'Send_Message',
                                payload: {
                                  up_template_key: 'address_bar_up',
                                  text: item.name,
                                  reportParams: { is_address: 1, scene: 'address_bar' },
                                },
                              })
                            )}`"
                            target="_blank"
                            class="ml-2"
                            title="打开豆包对话"
                          />
                        </div>

                        <div class="event-industry" v-if="item.industry">
                          <el-tag type="primary" size="small" effect="plain">
                            <span v-html="highlightKeywords(item.industry, eventKeyword)"></span>
                          </el-tag>
                        </div>
                      </div>
                    </div>

                    <div class="empty-tip" v-else>
                      <el-empty description="暂无大事件数据" />
                    </div>
                  </div>
                </el-tab-pane>
              </el-tabs>
            </el-card>
          </div>
        </el-col>
      </el-row>
    </div>

    <!-- 任务配置 Drawer -->
    <el-drawer v-model="drawerVisible" title="交易计划任务" size="70%" resizable :show-close="false" destroy-on-close @close="resetTaskForm">
      <template #header="{ close, titleId, titleClass }">
        <h4 :id="titleId" :class="titleClass">交易计划任务</h4>

        <el-button type="primary" @click="saveTask" :loading="isSaving">
          {{ drawerType === 'create' ? '保存任务' : '修改任务' }}
        </el-button>

        <el-button @click="close">取消</el-button>
      </template>

      <div class="config-content">
        <div class="mb-4" v-if="taskForm.status === 1">
          <el-alert title="当前任务正在运行中" type="warning" description="修改配置后，需要重新启动任务才能生效。" show-icon :closable="false" />
        </div>

        <el-form ref="taskFormRef" :model="taskForm" :rules="taskRules" label-width="130px" class="task-form">
          <el-card class="config-group">
            <template #header>
              <div class="group-title">
                <span>任务信息</span>
              </div>
            </template>

            <el-row :gutter="20" class="form-row">
              <el-col :span="12">
                <el-form-item label="策略类型" prop="strategy_id" class="form-item">
                  <el-select v-model="taskForm.strategy_id" placeholder="选择策略类型" :loading="strategySearchLoading" remote-show-suffix clearable filterable remote :remote-method="remoteStrategySearch" @change="handleStrategyChange" class="form-control">
                    <el-option v-for="strategy in availableStrategies" :key="strategy.id" :label="strategy.name" :value="Number(strategy.id)" :disabled="!strategy.status" />
                  </el-select>
                </el-form-item>
              </el-col>

              <el-col :span="12"></el-col>

              <el-col :span="12">
                <el-form-item label="任务名称" prop="name" class="form-item">
                  <el-input v-model="taskForm.name" clearable placeholder="请输入任务名称，便于区分多个任务" class="form-control" />
                </el-form-item>
              </el-col>

              <el-col :span="12">
                <el-form-item label="备注" prop="remark" class="form-item">
                  <el-input v-model="taskForm.remark" clearable placeholder="请输入备注" class="form-control" />
                </el-form-item>
              </el-col>

              <!-- <el-col :span="24">
                <el-form-item class="form-item">
                  <el-text type="primary">{{ formatMarketsText }}</el-text>
                </el-form-item>
              </el-col> -->
            </el-row>
          </el-card>
        </el-form>

        <!-- 策略参数配置 -->
        <el-card class="config-group">
          <template #header>
            <div class="group-title">
              <span>策略配置</span>
            </div>
          </template>

          <!-- 策略未选择时的提示 -->
          <div v-if="!taskForm.strategy_id" class="strategy-tips">
            <el-alert type="warning" title="请先选择策略类型" description="选择策略后将自动加载对应的配置项" :closable="false" show-icon />
          </div>

          <form-create v-else v-model:api="fApi" v-model="taskForm.config" :rule="formRules" :option="formOptions" />
        </el-card>

        <div style="margin-top: 15px; display: flex; align-items: center; justify-content: center">
          <el-checkbox v-model="isAgreementChecked" size="small"> <el-text type="info">我已阅读并同意</el-text> </el-checkbox>
          <el-button link type="primary" @click="agreementVisible = true" style="margin-left: 5px">《用户服务协议》</el-button>
        </div>
      </div>
    </el-drawer>

    <!-- 账户设置 Drawer -->
    <el-drawer v-model="accountDrawerVisible" :title="isEditAccount ? '修改证券账户' : '添加证券账户'" size="65%" :show-close="false" destroy-on-close>
      <template #header="{ close, titleId, titleClass }">
        <h4 :id="titleId" :class="titleClass">{{ isEditAccount ? '修改证券账户' : '添加证券账户' }}</h4>
        <div>
          <el-button @click="close">取消</el-button>
          <el-button type="primary" @click="saveAccount" :loading="isSavingAccount">保存</el-button>
        </div>
      </template>

      <el-form :model="accountForm" label-position="top" ref="accountFormRef" label-width="100px">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="开户姓名" prop="name" required>
              <el-input v-model="accountForm.name" clearable placeholder="请输入开户姓名" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="开户券商" prop="broker" required>
              <el-select v-model="accountForm.broker" filterable clearable placeholder="请选择或输入券商名称">
                <el-option-group v-for="group in brokerOptions" :key="group.label" :label="group.label">
                  <el-option v-for="(item, index) in group.options" :key="index" :label="item" :value="item" />
                </el-option-group>
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="资金账号" prop="account_no" required>
              <el-input v-model="accountForm.account_no" clearable placeholder="请输入资金账号" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="交易密码" prop="passcode" required>
              <el-input v-model="accountForm.passcode" type="password" show-password maxlength="6" clearable placeholder="请输入交易密码" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="交易板块（可多选。须实际已开通对应的交易权限，以免影响实盘交易）" prop="markets" required>
              <el-select v-model="accountForm.markets" multiple placeholder="请选择支持的市场" style="width: 100%">
                <el-option v-for="(item, key) in marketOptions" :key="key" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">数据配置</el-divider>

        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="数据TOKEN" prop="server.data_token" required>
              <el-input v-model="accountForm.server.data_token" type="password" show-password clearable placeholder="请输入数据TOKEN" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">AI配置</el-divider>

        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="AI大模型配置">
              <div style="width: 100%">
                <div class="flex justify-between items-center mb-2">
                  <div class="text-sm text-gray-500" v-if="accountForm.server.ai_models?.length">默认模型：{{ getAiModelDisplayName(accountForm.server.ai_models.find(m => m.id === accountForm.server.ai_default_id)) }}</div>
                  <div v-else class="text-sm text-gray-500">未配置</div>
                  <el-button type="primary" icon="plus" @click="addAiModelConfig">新增模型</el-button>
                </div>

                <el-table :data="accountForm.server.ai_models || []" border style="width: 100%" :row-key="row => row.id">
                  <el-table-column label="名称" min-width="140">
                    <template #default="{ row }">
                      <el-input v-model="row.name" clearable placeholder="例如：豆包/通义/DeepSeek" />
                    </template>
                  </el-table-column>
                  <el-table-column label="模型" min-width="160">
                    <template #default="{ row }">
                      <el-select v-model="row.model" filterable allow-create default-first-option clearable placeholder="请选择或输入模型" @change="val => handleAiModelSelectChange(row, val)">
                        <el-option v-for="(item, key) in serverAiModelOptions" :key="key" :label="item.label" :value="item.value" />
                      </el-select>
                    </template>
                  </el-table-column>
                  <el-table-column label="Key" min-width="160">
                    <template #default="{ row }">
                      <el-input v-model="row.key" type="password" show-password clearable placeholder="请输入Key" @input="syncLegacyAiFields" />
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="180" fixed="right">
                    <template #default="{ row, $index }">
                      <el-tag v-if="row.id === accountForm.server.ai_default_id" type="success" class="mr-2">默认</el-tag>
                      <el-button v-else type="primary" link @click="setDefaultAiModelConfig(row.id)">设为默认</el-button>
                      <el-button type="danger" link @click="removeAiModelConfig($index)">删除</el-button>
                    </template>
                  </el-table-column>
                </el-table>
              </div>
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">通知推送</el-divider>

        <el-row :gutter="20">
          <el-col :span="6">
            <el-form-item label="Webhook类型" prop="server.webhook_type" required>
              <el-select v-model="accountForm.server.webhook_type" placeholder="请选择Webhook类型" style="width: 100%">
                <el-option v-for="(item, key) in serverWebhookTypeOptions" :key="key" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="18">
            <el-form-item label="Webhook地址" prop="server.webhook_url" required>
              <el-input v-model="accountForm.server.webhook_url" clearable placeholder="请输入Webhook地址" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <!-- 用户协议弹窗 -->
    <el-dialog v-model="agreementVisible" title="" width="60%" append-to-body>
      <div style="max-height: 60vh; overflow-y: auto; padding: 10px; line-height: 1.6">
        <h3 style="font-size: 18px; text-align: center; margin-bottom: 20px">用户服务协议</h3>
        <p><strong>1. 特别提示</strong></p>
        <p>欢迎您使用智子量化多任务自动交易系统（以下简称“本软件”）。请您在开始使用本软件之前，务必仔细阅读并充分理解本协议，特别是涉及免除或者限制责任的条款、法律适用和争议解决条款。</p>
        <br />
        <p><strong>2. 风险揭示</strong></p>
        <p>2.1 量化交易具有极高的风险性，市场行情波动、系统故障、网络延迟、交易所规则变更等多种不可控因素均可能导致交易损失。</p>
        <p>2.2 本软件提供的策略模型、回测数据及交易信号仅供参考，不构成任何投资建议。您应根据自身的风险承受能力和投资经验，独立做出投资决策，并自行承担全部风险。</p>
        <br />
        <p><strong>3. 免责声明</strong></p>
        <p>3.1 开发者不对本软件的稳定性、准确性、完整性、及时性做出任何承诺或保证。</p>
        <p>3.2 因使用本软件而产生的任何直接或间接损失（包括但不限于资金损失、数据丢失、利润损失等），开发者不承担任何法律责任。</p>
        <p>3.3 用户在使用本软件过程中应遵守相关法律法规及证券交易所的交易规则，不得利用本软件进行任何违法违规操作。</p>
        <br />
        <p><strong>4. 知识产权</strong></p>
        <p>本软件的一切知识产权及相关权益归开发者所有。未经授权，任何单位和个人不得擅自复制、修改、传播或用于商业用途。</p>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button
            type="danger"
            @click="
              isAgreementChecked = false;
              agreementVisible = false;
            "
            >不同意</el-button
          >
          <el-button
            type="primary"
            @click="
              isAgreementChecked = true;
              agreementVisible = false;
            "
            >我已阅读并同意</el-button
          >
        </span>
      </template>
    </el-dialog>

    <!-- 升级检测 -->
    <BtnUpdate :show-btn="false" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, watch, nextTick, shallowRef } from 'vue';
import { useAppStore } from '@/pinia';
import { useUserStore } from '@/pinia/modules/user';
import { useBtnAuth } from '@/utils/btnAuth';
import { useRoute, useRouter } from 'vue-router';
import Draggable from 'vuedraggable';
import { ElMessage, ElMessageBox, ElEmpty } from 'element-plus';
import { Sunny, Moon, InfoFilled, TrendCharts, Operation as SOperation, Monitor, More, Notification, Calendar, Money, CreditCard, Grid, Platform, Delete, CopyDocument, Lightning, Plus, Refresh, RefreshLeft, RefreshRight, Setting, View, Hide, Coin, Search, ChatRound, DArrowLeft, DArrowRight } from '@element-plus/icons-vue';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile, formatSmartNumber } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import formCreate from '@form-create/element-ui';
import BtnUpdate from '@/components/tm-update/BtnUpdate.vue';
import { brokerOptions } from '@/data/brokerOptions';
import { getBaseStockPublic } from '@/api/quant/baseStock';
import { getStrategyList } from '@/api/quant/strategy';
import { getMyAccount, createAccount, updateAccount } from '@/api/quant/account';
import { createTradeTask, updateTradeTask, getTradeTaskList, deleteTradeTaskByIds } from '@/api/quant/tradeTask';
import { getTradeRecordList } from '@/api/quant/tradeRecord';
import { getNewsList } from '@/api/quant/news';
import { getEventList } from '@/api/quant/event';

const dashboardComponents = ref([{ id: 'account' }, { id: 'task' }, { id: 'record' }, { id: 'terminal' }]);
const sidebarVisible = ref(true);
const layoutColumns = ref([
  { id: 'main', span: 18 },
  { id: 'sidebar', span: 6 },
]);

const toggleSidebar = () => {
  sidebarVisible.value = !sidebarVisible.value;
  if (sidebarVisible.value) {
    layoutColumns.value = [
      { id: 'main', span: 18 },
      { id: 'sidebar', span: 6 },
    ];
  } else {
    layoutColumns.value = [{ id: 'main', span: 24 }];
  }
};

const btnAuth = useBtnAuth();
const appStore = useAppStore();
const userStore = useUserStore();
const route = useRoute();
const router = useRouter();

const currentAccount = ref(null);
const isEditAccount = ref(false);
const isTradingTime = ref(false);
const isTradingDay = ref(true); // 新增交易日状态
const showFunds = ref(true);
const clientVersion = ref('1.0.0');
const isPyWebView = ref(false);

const checkEnvironment = () => {
  if (window.pywebview) {
    isPyWebView.value = true;
    try {
      window.pywebview.api.system_getAppInfo().then(res => {
        if (res && res.appVersion) {
          clientVersion.value = res.appVersion;
        }
      });
    } catch (e) {
      console.error(e);
    }
  } else {
    isPyWebView.value = false;
  }
};

const currentDate = ref('');
const currentWeek = ref('');
const currentTime = ref('');
let timer1 = null;
let timer2 = null;

const getPositions = row => {
  if (row.positions && Array.isArray(row.positions) && row.positions.length > 0) {
    return row.positions;
  }
  return [];
};

const updateTime = () => {
  const now = new Date();
  const weeks = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'];
  currentDate.value = formatToDateTime(now, 'YYYY年MM月DD日');
  currentWeek.value = weeks[now.getDay()];
  currentTime.value = formatToDateTime(now, 'HH:mm:ss');
};

const getValidityType = dateStr => {
  if (!dateStr) return 'info';
  const date = new Date(dateStr);
  const now = new Date();
  now.setHours(0, 0, 0, 0); // Reset time part for today
  if (date < now) return 'danger'; // Expired

  const diffTime = date.getTime() - now.getTime();
  const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

  if (diffDays <= 7) return 'warning'; // Warning (<= 7 days)

  return 'success'; // OK
};

const getValidityTooltip = dateStr => {
  if (!dateStr) return '';
  const date = new Date(dateStr);
  const now = new Date();
  now.setHours(0, 0, 0, 0);

  if (date < now) return '已过期';

  const diffTime = date.getTime() - now.getTime();
  const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

  if (diffDays === 0) return '今天到期';
  return `剩余 ${diffDays} 天`;
};

// 市场状态
const marketStatus = reactive({
  text: '休市中',
  type: 'info',
});

// 服务器状态
const serverStatus = reactive({
  text: '交易服务器待连接',
  type: 'info',
});

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const statusOptions = ref();
const marketOptions = ref([]);
const serverWebhookTypeOptions = ref();
const serverAiModelOptions = ref();

// 获取需要的字典 可能为空 按需保留
const setOptions = async () => {
  statusOptions.value = await getDictFunc('status');
  marketOptions.value = await getDictFunc('quant_base_stock_market');
  serverWebhookTypeOptions.value = await getDictFunc('quant_account_server_webhook_type');
  serverAiModelOptions.value = await getDictFunc('quant_account_server_ai_model');
};

setOptions();

// 证券账户资金概览数据
const accountData = reactive({
  broker: '',
  account_no: '',
  max_task: 1,
  max_running_task: 1,
  running_task_count: 0,
  total_asset: 0,
  market_value: 0,
  total_pl_amount: 0,
  daily_pl_amount: 0,
  daily_pl_ratio: 0.0,
  available_balance: 0,
  withdrawable_cash: 0,
  expiration_date: null,
  updated_at: '',
});

const isExpired = computed(() => {
  if (!accountData.expiration_date) return false;
  try {
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const exp = new Date(accountData.expiration_date);
    if (isNaN(exp.getTime())) return false;
    const expDate = new Date(exp.getFullYear(), exp.getMonth(), exp.getDate());
    return today >= expDate;
  } catch (e) {
    return false;
  }
});

const maskedAccountNo = computed(() => {
  const accNo = accountData.account_no;
  if (!accNo || accNo.length < 6) {
    return '****';
  }
  const prefix = accNo.substring(0, 2);
  const suffix = accNo.slice(-4);
  return `${prefix}****${suffix}`;
});

// 格式化markets数据为可读文本
const formatMarketsText = computed(() => {
  if (!currentAccount.value || !currentAccount.value.markets) {
    return '请确保已开通对应交易板块的交易权限，以免影响实盘交易。';
  }

  let markets = currentAccount.value.markets;

  if (markets) {
    return `当前证券账户交易板块支持：${markets.join('、')}。`;
  }
});

const checkServerStatus = async (silent = true) => {
  if (!window.pywebview) {
    serverStatus.text = '交易服务器未连接';
    serverStatus.type = 'warning';
    if (!silent) {
      const msg = '获取服务器状态失败！请在客户端操作';
      ElMessage.warning(msg);
      addConsoleLog('WARNING', 'QuantClient', msg);
    }
    return;
  }

  if (!currentAccount.value) {
    if (!silent) ElMessage.warning('账户数据未加载');
    return;
  }

  let msg = '';
  if (!silent) {
    msg = `正在获取服务器状态...`;
    ElMessage.primary(msg);
  }

  try {
    const env = import.meta.env;
    const refreshPayload = {
      backend_url: env.VITE_BASE_PATH + (env.DEV ? ':' + env.VITE_CLI_PORT : '') + env.VITE_BASE_API,
      token: userStore.token,
      account: currentAccount.value,
    };

    const refreshRes = await window.pywebview.api.quant_refreshAccount(refreshPayload);
    if (refreshRes && !refreshRes.success) {
      serverStatus.text = '交易服务器状态异常';
      serverStatus.type = 'danger';
      if (!silent) {
        msg = `服务器状态请求未执行: ${refreshRes.msg}`;
        ElMessage.warning(msg);
        addConsoleLog('WARNING', 'QuantClient', msg);
      }
    } else {
      serverStatus.text = '交易服务器连接成功';
      serverStatus.type = 'success';
      if (!silent) {
        msg = `服务器状态请求成功。`;
        ElMessage.success(msg);
        addConsoleLog('INFO', 'QuantClient', msg);
      }
    }
  } catch (e) {
    serverStatus.text = '交易服务器连接断开';
    serverStatus.type = 'danger';
    if (!silent) {
      msg = `服务器状态请求触发异常: ${e.message}`;
      ElMessage.error(msg);
      addConsoleLog('ERROR', 'QuantClient', msg);
    }
  }

  await fetchAccountData();
};

const fetchAccountData = async () => {
  try {
    const res = await getMyAccount();
    if (res.code === 0 && res.data) {
      currentAccount.value = res.data;
      accountData.broker = res.data.broker;
      accountData.account_no = res.data.account_no;
      accountData.max_task = res.data.max_task || 1;
      accountData.max_running_task = res.data.max_running_task || 1;
      accountData.running_task_count = res.data.running_task_count || 0;
      accountData.expiration_date = res.data.expiration_date;

      let summary = res.data.summary || {};
      accountData.total_asset = summary.total_asset || 0;
      accountData.market_value = summary.market_value || 0;
      accountData.available_balance = summary.available_balance || 0;
      accountData.withdrawable_cash = summary.withdrawable_cash || 0;
      accountData.total_pl_amount = summary.total_pl_amount || 0;
      accountData.daily_pl_amount = summary.daily_pl_amount || 0;

      const prevAsset = accountData.total_asset - accountData.daily_pl_amount;
      if (prevAsset !== 0) {
        accountData.daily_pl_ratio = (accountData.daily_pl_amount / prevAsset) * 100;
      } else {
        accountData.daily_pl_ratio = 0;
      }

      accountData.updated_at = summary.updated_at || '';

      if (window.pywebview) {
        const runningRes = await window.pywebview.api.quant_getRunningTasks();
        if (runningRes && runningRes.success && Array.isArray(runningRes.data)) {
          accountData.running_task_count = runningRes.data.length;
        }
      }
    }
  } catch (error) {
    console.error('获取账户数据失败:', error);
    ElMessage.error('账户数据拉取失败');
  }
};

// 账户设置Drawer
const accountDrawerVisible = ref(false);
const isSavingAccount = ref(false);
const accountFormRef = ref(null);

const genAiModelId = () => `${Date.now()}_${Math.random().toString(16).slice(2)}`;

const getAiModelDisplayName = m => {
  if (!m) return '-';
  return m.name || m.model || '-';
};

const hydrateAiModelUrlBackupFromOptions = server => {
  if (!server || !Array.isArray(server.ai_models) || server.ai_models.length === 0) return;
  const options = serverAiModelOptions.value || [];
  if (!options.length) return;

  server.ai_models.forEach(m => {
    if (!m || typeof m !== 'object' || Array.isArray(m)) return;
    if (m.url) return;
    const matched = options.find(o => o.value === m.model);
    if (matched && matched.extend !== undefined && matched.extend !== null) {
      m.url = matched.extend;
    }
  });
};

const normalizeServerForForm = serverLike => {
  let server = serverLike;
  if (typeof server === 'string') {
    try {
      server = JSON.parse(server);
    } catch (e) {
      server = {};
    }
  }
  if (!server || typeof server !== 'object' || Array.isArray(server)) server = {};

  const defaultServer = {
    data_token: '',
    ai_model: '',
    ai_key: '',
    ai_url: '',
    ai_models: [],
    ai_default_id: '',
  };

  server = { ...defaultServer, ...server };

  if (typeof server.ai_models === 'string') {
    try {
      server.ai_models = JSON.parse(server.ai_models);
    } catch (e) {
      server.ai_models = [];
    }
  }
  if (!Array.isArray(server.ai_models)) server.ai_models = [];

  server.ai_models = server.ai_models.map(m => {
    let item = m;
    if (typeof item === 'string') {
      try {
        item = JSON.parse(item);
      } catch (e) {
        item = { model: m };
      }
    }
    if (!item || typeof item !== 'object' || Array.isArray(item)) item = {};
    const model = item.model || item.ai_model || '';
    let url = '';
    if (typeof item.url === 'string') url = item.url;
    else if (typeof item.extend === 'string') url = item.extend;
    else if (typeof item.ai_extend === 'string') url = item.ai_extend;
    if (!url && model) {
      const options = serverAiModelOptions.value || [];
      const matched = options.find(o => o.value === model);
      if (matched && matched.extend !== undefined && matched.extend !== null) {
        url = matched.extend;
      }
    }
    return {
      id: item.id || genAiModelId(),
      name: item.name || '',
      model,
      key: item.key || item.ai_key || '',
      url,
    };
  });

  if (!server.ai_models.length && (server.ai_model || server.ai_key)) {
    let url = '';
    if (server.ai_model) {
      const options = serverAiModelOptions.value || [];
      const matched = options.find(o => o.value === server.ai_model);
      if (matched && matched.extend !== undefined && matched.extend !== null) {
        url = matched.extend;
      }
    }
    server.ai_models = [
      {
        id: genAiModelId(),
        name: '默认',
        model: server.ai_model || '',
        key: server.ai_key || '',
        url,
      },
    ];
  }

  if (server.ai_models.length && !server.ai_default_id) {
    server.ai_default_id = server.ai_models[0].id;
  }

  return server;
};

const syncLegacyAiFields = () => {
  const server = accountForm.server;
  if (!server || !Array.isArray(server.ai_models) || server.ai_models.length === 0) {
    if (server) {
      server.ai_model = '';
      server.ai_key = '';
    }
    return;
  }

  const selected = server.ai_models.find(m => m.id === server.ai_default_id) || server.ai_models[0];
  server.ai_model = selected?.model || '';
  server.ai_key = selected?.key || '';
};

const handleAiModelSelectChange = (row, val) => {
  const options = serverAiModelOptions.value || [];
  const matched = options.find(o => o.value === val);
  if (matched && matched.extend !== undefined && matched.extend !== null) {
    row.url = matched.extend;
  } else if (matched && matched.extend === '') {
    row.url = '';
  } else {
    row.url = row.url || '';
  }
  syncLegacyAiFields();
};

const addAiModelConfig = () => {
  const server = accountForm.server;
  if (!server) return;
  if (!Array.isArray(server.ai_models)) server.ai_models = [];
  const id = genAiModelId();
  server.ai_models.push({ id, name: '', model: '', key: '', url: '' });
  if (!server.ai_default_id) server.ai_default_id = id;
  syncLegacyAiFields();
};

const setDefaultAiModelConfig = id => {
  const server = accountForm.server;
  if (!server) return;
  server.ai_default_id = id;
  syncLegacyAiFields();
};

const removeAiModelConfig = index => {
  const server = accountForm.server;
  if (!server || !Array.isArray(server.ai_models)) return;
  const removed = server.ai_models.splice(index, 1)[0];
  if (removed && removed.id && removed.id === server.ai_default_id) {
    server.ai_default_id = server.ai_models[0]?.id || '';
  }
  syncLegacyAiFields();
};

const accountForm = reactive({
  name: '',
  broker: '',
  account_no: '',
  passcode: '',
  type: 0,
  amount: 0,
  remark: '',
  status: 1,
  markets: [],
  server: normalizeServerForForm({}),
});

const openAccountSettings = () => {
  if (currentAccount.value) {
    isEditAccount.value = true;
    Object.assign(accountForm, currentAccount.value);
    accountForm.server = normalizeServerForForm(accountForm.server);
    hydrateAiModelUrlBackupFromOptions(accountForm.server);
    syncLegacyAiFields();

    if (typeof accountForm.markets === 'string') {
      try {
        accountForm.markets = JSON.parse(accountForm.markets);
      } catch (e) {
        accountForm.markets = [];
      }
    }

    if (!accountForm.markets) accountForm.markets = [];
  } else {
    isEditAccount.value = false;
    Object.assign(accountForm, {
      name: '',
      broker: '',
      account_no: '',
      passcode: '',
      type: 0,
      amount: 0,
      remark: '',
      status: 1,
      markets: [],
      server: normalizeServerForForm({}),
    });
    syncLegacyAiFields();
  }
  accountDrawerVisible.value = true;
};

const saveAccount = async () => {
  isSavingAccount.value = true;
  try {
    accountForm.server = normalizeServerForForm(accountForm.server);
    hydrateAiModelUrlBackupFromOptions(accountForm.server);
    syncLegacyAiFields();
    let res;
    if (isEditAccount.value) {
      res = await updateAccount(accountForm);
    } else {
      res = await createAccount(accountForm);
    }

    if (res.code === 0) {
      accountDrawerVisible.value = false;
      await fetchAccountData(); // 刷新账户信息
    } else {
      ElMessage.error(res.msg || (isEditAccount.value ? '更新失败' : '设置失败'));
    }
  } catch (error) {
    console.error('保存账户失败:', error);
    ElMessage.error('保存账户失败');
  } finally {
    isSavingAccount.value = false;
  }
};

// 可用策略列表
const availableStrategies = ref([]);
const strategySearchLoading = ref(false);

// Drawer 相关
const drawerVisible = ref(false);
const drawerType = ref('create'); // create:新增 update:编辑
const currentEditStrategy = ref(null);

// 用户协议弹窗
const agreementVisible = ref(false);
const isAgreementChecked = ref(true);

// 1. 定义基础配置常量
const defaulTaskForm = {
  id: 0, // 任务ID
  member_id: 0, // 会员ID
  account_id: 0, // 账户ID
  strategy_id: null, // 策略ID
  name: '', // 任务名称
  stock: {
    name: '', // 股票名称
    symbol: '', // 股票代码
    ts_code: '', // TS代码
    market: '', // 市场
  },
  config: {}, // 策略配置，由具体策略的rules动态生成
  remark: '', // 备注
  status: 0, // 0:停止 1:运行中 -1:已删除
};

// 2. 基于基础配置初始化 taskForm（保持响应性）
const taskForm = reactive(structuredClone(defaulTaskForm));
const taskFormRef = ref(null);
const taskRules = reactive({
  strategy_id: [{ required: true, message: '请选择策略类型', trigger: 'change' }],
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
});
const formRules = ref([]);
const formOptions = ref({});
const fApi = ref({});

// 简化重置函数（直接复用基础配置）
const resetTaskForm = () => {
  Object.assign(taskForm, structuredClone(defaulTaskForm));
  formRules.value = [];
  currentEditStrategy.value = null;
  isAgreementChecked.value = true;
};

// 注入有效期快捷选项
const injectValidityLogic = rules => {
  const validityRule = rules.find(r => r.field === 'validityPeriod');
  if (validityRule) {
    if (!validityRule.props) validityRule.props = {};
    validityRule.props.shortcuts = [
      {
        text: '今天',
        value: () => new Date(),
      },
      {
        text: '明天',
        value: () => {
          const date = new Date();
          date.setTime(date.getTime() + 3600 * 1000 * 24);
          return date;
        },
      },
      {
        text: '后天',
        value: () => {
          const date = new Date();
          date.setTime(date.getTime() + 3600 * 1000 * 24 * 2);
          return date;
        },
      },
      {
        text: '本周',
        value: () => {
          const date = new Date();
          const day = date.getDay();
          let diff = 5 - day; // 周五
          if (diff < 0) diff += 7;
          date.setDate(date.getDate() + diff);
          return date;
        },
      },
      {
        text: '7天',
        value: () => {
          const date = new Date();
          date.setTime(date.getTime() + 3600 * 1000 * 24 * 7);
          return date;
        },
      },
      {
        text: '15天',
        value: () => {
          const date = new Date();
          date.setTime(date.getTime() + 3600 * 1000 * 24 * 15);
          return date;
        },
      },
      {
        text: '1个月',
        value: () => {
          const date = new Date();
          date.setMonth(date.getMonth() + 1);
          return date;
        },
      },
      {
        text: '3个月',
        value: () => {
          const date = new Date();
          date.setMonth(date.getMonth() + 3);
          return date;
        },
      },
      {
        text: '半年',
        value: () => {
          const date = new Date();
          date.setMonth(date.getMonth() + 6);
          return date;
        },
      },
      {
        text: '1年',
        value: () => {
          const date = new Date();
          date.setFullYear(date.getFullYear() + 1);
          return date;
        },
      },
    ];
    validityRule.props.disabledDate = time => {
      const maxDate = new Date();
      maxDate.setFullYear(maxDate.getFullYear() + 1);
      return time.getTime() < Date.now() - 8.64e7 || time.getTime() > maxDate.getTime();
    };

    // 设置默认值为1年后
    if (!validityRule.value) {
      const defaultDate = new Date();
      defaultDate.setFullYear(defaultDate.getFullYear() + 1);
      const year = defaultDate.getFullYear();
      const month = String(defaultDate.getMonth() + 1).padStart(2, '0');
      const day = String(defaultDate.getDate()).padStart(2, '0');
      validityRule.value = `${year}-${month}-${day}`;
    }
  }
};

// 打开Drawer
const openDrawer = (type, task = null) => {
  if (type === 'create') {
    if (isExpired.value) {
      ElMessage.warning('账户已到期，无法创建新任务！');
      return;
    }
    if (!accountData.account_no) {
      ElMessage.warning('请先设置证券账户！');
      return;
    }
    // if (taskTotal.value >= accountData.max_task) {
    //   ElMessage.warning(`无法创建新任务：已达到最大任务数量限制 (${accountData.max_task})`);
    //   return;
    // }
  }
  drawerType.value = type;
  drawerVisible.value = true;
  if (type === 'update' && task) {
    currentEditStrategy.value = task;
    const newFormState = structuredClone(defaulTaskForm);
    // 基础字段赋值
    newFormState.id = task.id;
    newFormState.member_id = task.member_id;
    newFormState.account_id = task.account_id;
    newFormState.strategy_id = task.strategy_id;
    newFormState.status = task.status;

    if (task.raw && task.raw.strategy && task.raw.strategy.rules) {
      formRules.value = task.raw.strategy.rules;
      formOptions.value = task.raw.strategy.options || {};
    } else {
      const strategy = availableStrategies.value.find(s => s.id === task.strategy_id);
      if (strategy && strategy.rules && strategy.rules.length > 0) {
        formRules.value = strategy.rules;
        formOptions.value = strategy.options || {};
      } else {
        formRules.value = [];
      }
    }
    // 注入有效期逻辑
    injectValidityLogic(formRules.value);

    newFormState.name = task.name;
    newFormState.remark = task.remark;

    // 填充嵌套对象
    if (task.stock) {
      Object.assign(newFormState.stock, task.stock);
      // 将 stock 对象赋值给 config.symbol，用于表单回显
      if (!newFormState.config) {
        newFormState.config = {};
      }
      newFormState.config.symbol = task.stock;
    }
    if (task.config) {
      Object.assign(newFormState.config, task.config);
    }

    Object.assign(taskForm, newFormState);

    // 编辑模式下，需要将当前股票对象添加到下拉选项中
    if (task.stock && fApi.value && fApi.value.updateRule) {
      nextTick(() => {
        fApi.value.updateRule('symbol', {
          options: [
            {
              value: task.stock,
              label: `${task.stock.name} (${task.stock.ts_code})`,
            },
          ],
        });
      });
    }
  } else {
    resetTaskForm();
  }
};

// 获取任务列表
const taskPage = ref(1);
const taskTotal = ref(0);
const taskPageSize = ref(5);
const taskSearchKeyword = ref(''); // 任务搜索关键词

// 同步任务状态（客户端被强制关闭或意外退出，保证任务状态的一致性）
const syncTaskState = async () => {
  if (!window.pywebview) return;

  let stateChanged = false;

  try {
    const res = await window.pywebview.api.quant_getRunningTasks();
    if (res.success) {
      const runningIds = res.data.map(id => String(id));

      for (const task of tradeTasks.value) {
        // 如果服务端显示运行中，但本地未运行
        if (task.status === 1) {
          if (!runningIds.includes(String(task.id))) {
            try {
              await updateTradeTask({ id: task.id, status: 0 });
              task.status = 0;
              addConsoleLog('WARNING', 'System', `任务(${task.id})状态同步：已停止（异常退出检测）`);
              stateChanged = true;
            } catch (e) {
              addConsoleLog('ERROR', 'System', `任务(${task.id})状态同步失败：${e.message}`);
            }
          }
        }
      }

      if (stateChanged) {
        fetchAccountData();
      }
    }
  } catch (e) {
    addConsoleLog('ERROR', 'System', `任务状态同步失败：${e.message}`);
  }
};

const getTradeTasks = async () => {
  const res = await getTradeTaskList({ page: taskPage.value, pageSize: taskPageSize.value, keyword: taskSearchKeyword.value });

  if (res.code === 0 && res.data.list) {
    taskTotal.value = res.data.total;
    tradeTasks.value = res.data.list.map(task => {
      return {
        id: task.id,
        member_id: task.member_id,
        account_id: task.account_id,
        strategy_id: task.strategy_id,
        name: task.name,
        strategy_name: task.strategy?.name || '-',
        stock: task.stock,
        config: task.config,
        positions: task.positions || [],
        symbol_name: task.stock?.name || '-',
        symbol_code: task.stock?.ts_code,
        status: task.status,
        remark: task.remark,
        created_at: formatToDateTime(task.created_at, 'YYYY-MM-DD HH:mm'),
        updated_at: formatToDateTime(task.updated_at, 'YYYY-MM-DD HH:mm'),
        raw: task, // 保存原始任务数据，方便编辑时回填
      };
    });

    // 获取列表后同步状态
    await syncTaskState();
  }
};

const handleTaskPageChange = val => {
  taskPage.value = val;
  getTradeTasks();
};

const handleTaskSizeChange = val => {
  taskPageSize.value = val;
  getTradeTasks();
};

// 任务相关
const tradeTasks = ref([]);

// Loading states for buttons
const isRunningSelected = ref(false);
const isStoppingSelected = ref(false);
const taskActionLoading = reactive({});
const startingTaskIds = reactive(new Set());

// 选中的任务
const selectedTasks = ref([]);

const handleSelectionChange = val => {
  selectedTasks.value = val;
};

// 保存/更新任务
const isSaving = ref(false);
const saveTask = async () => {
  // 0. 协议校验
  if (!isAgreementChecked.value) {
    ElMessage.warning('请阅读并同意《用户服务协议》');
    return;
  }

  // 1. 基础表单校验
  if (!taskFormRef.value) return;
  try {
    await taskFormRef.value.validate();
  } catch (e) {
    ElMessage.warning('请检查任务信息是否填写完整');
    return;
  }

  // 2. 策略配置校验
  if (fApi.value && fApi.value.validate) {
    try {
      await fApi.value.validate();
    } catch (e) {
      ElMessage.warning('请检查策略配置是否填写完整');
      return;
    }
  }

  isSaving.value = true;
  try {
    const dataToSubmit = {
      ...taskForm,
    };

    // 将 config.symbol 的完整股票对象同步到 stock 字段
    if (dataToSubmit.config && dataToSubmit.config.symbol) {
      dataToSubmit.stock = dataToSubmit.config.symbol;
      // 从 config 中移除 symbol，避免重复存储
      delete dataToSubmit.config.symbol;
    }

    let res;
    if (drawerType.value === 'create') {
      dataToSubmit.member_id = currentAccount.value.member_id;
      dataToSubmit.account_id = currentAccount.value.id;
      res = await createTradeTask(dataToSubmit);
    } else {
      res = await updateTradeTask(dataToSubmit);
    }

    if (res.code === 0) {
      ElMessage.success(drawerType.value === 'create' ? '任务创建成功' : '任务修改成功');
      addConsoleLog('INFO', 'QuantClient', `任务(${res.data.id || dataToSubmit.id}) ${drawerType.value === 'create' ? '已创建' : '已修改'}成功。`);
      drawerVisible.value = false;
      getTradeTasks(); // 刷新列表
    } else {
      // ElMessage.error(res.msg || '操作失败'); // 与后端重复
    }
  } catch (error) {
    console.error('保存任务失败:', error);
  } finally {
    isSaving.value = false;
  }
};

// 运行选中任务
const runSelectedTask = async () => {
  if (isExpired.value) {
    ElMessage.warning('账户已到期，无法批量启动任务！');
    return;
  }

  if (!window.pywebview) {
    ElMessage.warning('批量启动任务失败！请在客户端启动');
    return;
  }

  if (selectedTasks.value.length === 0) {
    ElMessage.warning('请先选择任务');
    return;
  }

  // 过滤出未运行的任务
  const canRunTasks = selectedTasks.value.filter(s => s.status !== 1 && !startingTaskIds.has(s.id));
  if (canRunTasks.length === 0) {
    ElMessage.info('选中的任务均已在运行中');
    return;
  }

  isRunningSelected.value = true;
  try {
    const maxRunningTask = Number(accountData.max_running_task || 0);
    const runningRes = await window.pywebview.api.quant_getRunningTasks();
    const runningCount = runningRes && runningRes.success && Array.isArray(runningRes.data) ? runningRes.data.length : 0;
    const startingCount = startingTaskIds.size;
    const remainingSlots = maxRunningTask > 0 ? Math.max(0, maxRunningTask - runningCount - startingCount) : canRunTasks.length;
    if (remainingSlots <= 0) {
      const msg = `批量启动任务被拦截：任务运行数量已达最大上限`;
      ElMessage.warning(msg);
      addConsoleLog('WARNING', 'QuantClient', `${msg}`);
      return;
    }

    const tasksToRun = canRunTasks.slice(0, remainingSlots);
    const promises = tasksToRun.map(task => runTask(task.id, false));
    const results = await Promise.all(promises);
    const successCount = results.filter(r => r === true).length;
    if (successCount > 0) {
      ElMessage.success(`已启动 ${successCount} 个任务`);
      fetchAccountData();
    } else {
      ElMessage.warning('未能启动任何任务，请检查日志或状态');
    }
  } finally {
    isRunningSelected.value = false;
  }
};

// 核心新增：停止选中任务
const stopSelectedTask = async () => {
  if (!window.pywebview) {
    ElMessage.warning('批量停止任务失败！请在客户端停止');
    return;
  }

  if (selectedTasks.value.length === 0) {
    ElMessage.warning('请先选择任务');
    return;
  }

  // 过滤出运行中的任务
  const canStopTasks = selectedTasks.value.filter(s => s.status === 1);
  if (canStopTasks.length === 0) {
    ElMessage.info('选中的任务均已停止');
    return;
  }

  isStoppingSelected.value = true;
  try {
    const promises = canStopTasks.map(task => stopTask(task.id, false));
    await Promise.all(promises);
    ElMessage.success(`已停止 ${canStopTasks.length} 个任务`);
    fetchAccountData();
  } finally {
    isStoppingSelected.value = false;
  }
};

// 删除选中任务
const deleteSelectedTask = () => {
  if (selectedTasks.value.length === 0) {
    ElMessage.warning('请先选择任务');
    return;
  }

  ElMessageBox.confirm(`确定要删除选中的 ${selectedTasks.value.length} 个任务吗？`, '删除任务', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(async () => {
    const idsToDelete = selectedTasks.value.map(s => s.id);
    const res = await deleteTradeTaskByIds({ ids: idsToDelete });
    if (res.code === 0) {
      ElMessage.success('任务已删除');
      addConsoleLog('INFO', 'task_manager', `已删除 ${idsToDelete.length} 个任务`);

      if (tradeTasks.value.length === idsToDelete.length && taskPage.value > 1) {
        taskPage.value--;
      }

      selectedTasks.value = [];
      getTradeTasks();
    }
  });
};

// 运行单个任务
const runTask = (id, refresh = true) => {
  if (isExpired.value) {
    ElMessage.warning('账户已到期，无法启动任务！');
    return Promise.resolve(false);
  }

  const task = tradeTasks.value.find(s => s.id === id);
  if (!task) return Promise.resolve(false);
  if (task.status === 1 || startingTaskIds.has(id)) return Promise.resolve(false);
  taskActionLoading[id] = 'starting';
  startingTaskIds.add(id);

  if (window.pywebview) {
    const env = import.meta.env;
    const data = {
      id: id,
      strategy_id: task.strategy_id,
      name: task.name,
      token: userStore.token,
      backend_url: env.VITE_BASE_PATH + (env.DEV ? ':' + env.VITE_CLI_PORT : '') + env.VITE_BASE_API,
      account: currentAccount.value,
      task: task.raw, //...(task.raw || task),
    };

    return window.pywebview.api
      .quant_getRunningTasks()
      .then(async runningRes => {
        const maxRunningTask = Number(accountData.max_running_task || 0);
        const runningCount = runningRes && runningRes.success && Array.isArray(runningRes.data) ? runningRes.data.length : 0;
        const startingCount = startingTaskIds.size;
        if (maxRunningTask > 0 && runningCount + startingCount > maxRunningTask) {
          const msg = `任务(${id})：启动失败！任务运行数量已达最大上限`;
          ElMessage.warning(msg);
          addConsoleLog('WARNING', 'QuantClient', msg);
          return false;
        }

        return window.pywebview.api.quant_startTask(data);
      })
      .then(async res => {
        if (res === false) {
          return false;
        }

        if (res && res.success) {
          try {
            const updateRes = await updateTradeTask({ id: task.id, status: 1 });
            if (updateRes.code === 0) {
              task.status = 1;
              addConsoleLog('INFO', 'QuantClient', `任务(${id})：已启动，正在运行中...(请勿关闭客户端[可最小化]，禁用电脑睡眠状态！)`);
              ElMessage.success(`任务(${id})：已成功启动`);
              if (refresh) fetchAccountData();
              return true;
            } else {
              await window.pywebview.api.quant_stopTask(id);
              addConsoleLog('WARNING', 'QuantClient', `任务(${id}): ${updateRes.msg}`);
              return false;
            }
          } catch (error) {
            console.error('Failed to update status to backend', error);
            await window.pywebview.api.quant_stopTask(id);
            addConsoleLog('WARNING', 'QuantClient', `任务(${id}): ${error}`);
            ElMessage.error(`任务(${id})：已自动停止，同步状态失败`);
            return false;
          }
        } else {
          const errorMsg = res?.msg || '未知错误';
          addConsoleLog('ERROR', 'QuantClient', `任务(${id})：启动失败！${errorMsg}`);
          ElMessage.error(`任务(${id})：启动失败！${errorMsg}`);
          return false;
        }
      })
      .catch(err => {
        // console.error(err);
        addConsoleLog('ERROR', 'QuantTrader', `${err}`);
        ElMessage.error(`任务(${id})：启动失败！请查看终端日志`);
        return false;
      })
      .finally(() => {
        delete taskActionLoading[id];
        startingTaskIds.delete(id);
      });
  } else {
    const msg = `任务(${id})：启动失败！请在客户端启动`;
    ElMessage.warning(msg);
    addConsoleLog('WARNING', 'QuantClient', msg);
    delete taskActionLoading[id];
    startingTaskIds.delete(id);
    return Promise.resolve(false);
  }
};

// 停止单个任务
const stopTask = (id, refresh = true) => {
  const task = tradeTasks.value.find(s => s.id === id);
  if (!task) return Promise.resolve();

  taskActionLoading[id] = 'stopping';

  if (window.pywebview) {
    return window.pywebview.api
      .quant_stopTask(id)
      .then(async res => {
        if (res.success) {
          try {
            await updateTradeTask({ id: task.id, status: 0 });
            task.status = 0;
            addConsoleLog('INFO', 'QuantClient', `任务(${id})：已成功停止。`);
            ElMessage.success(`任务(${id})：已成功停止`);
            if (refresh) fetchAccountData();
          } catch (error) {
            addConsoleLog('WARNING', 'QuantClient', `任务(${id})：停止成功，但同步状态失败: ${error}`);
            ElMessage.error(`任务(${id})：停止成功，但同步状态失败`);
          }
        } else {
          addConsoleLog('ERROR', 'QuantClient', `任务(${id})：停止失败！${res.msg}`);
          ElMessage.error(`任务(${id})：停止失败！${res.msg}`);
        }
      })
      .catch(err => {
        console.error(err);
        addConsoleLog('ERROR', 'QuantClient', `调用Sophon API失败: ${err}`);
      })
      .finally(() => {
        delete taskActionLoading[id];
      });
  } else {
    const msg = `任务(${id})：停止失败！请在客户端停止`;
    ElMessage.warning(msg);
    addConsoleLog('WARNING', 'QuantClient', msg);
    delete taskActionLoading[id];
    return Promise.resolve();
  }
};

// 处理策略选择变更
const handleStrategyChange = selectedId => {
  const selectedStrategy = availableStrategies.value.find(s => s.id === selectedId);
  if (!selectedStrategy) return;

  if (selectedStrategy.rules && selectedStrategy.rules.length > 0) {
    formRules.value = selectedStrategy.rules;
    // 注入有效期逻辑
    injectValidityLogic(formRules.value);
  } else {
    formRules.value = [];
  }

  if (selectedStrategy.options) {
    formOptions.value = selectedStrategy.options || {};
  } else {
    formOptions.value = {};
  }
};

const tradeRecords = ref([]);
const recordPage = ref(1);
const recordTotal = ref(0);
const recordPageSize = ref(5);
const recordSearchKeyword = ref(''); // 委托记录搜索关键词
const recordAction = ref(''); // 委托记录交易指令过滤

const getTradeRecords = async () => {
  const res = await getTradeRecordList({ page: recordPage.value, pageSize: recordPageSize.value, keyword: recordSearchKeyword.value, action: recordAction.value });
  if (res.code === 0) {
    recordTotal.value = res.data.total;
    tradeRecords.value = res.data.list.map(item => {
      return {
        id: item.id,
        task_id: item.task_id,
        task_name: item.trade_task?.name || '',
        name: item.name,
        symbol: item.symbol,
        price: item.price,
        quantity: item.quantity,
        amount: item.amount,
        action: item.action,
        reason: item.reason,
        time: formatToDateTime(item.traded_at, 'HH:mm:ss'),
        date: formatToDateTime(item.traded_at, 'YYYY-MM-DD'),
      };
    });
  }
};

// 分页
const handleTradeRecordSizeChange = val => {
  recordPageSize.value = val;
  getTradeRecords();
};

const handleTradeRecordPageChange = val => {
  recordPage.value = val;
  getTradeRecords();
};

// 终端日志相关
const terminalRef = ref(null);
const consoleLogs = ref([
  { time: new Date().toLocaleTimeString(), level: 'INFO', module: 'System', message: '智子量化实盘交易引擎 v2.0.0 已启动。' },
  { time: new Date().toLocaleTimeString(), level: 'INFO', module: 'ConfigLoader', message: '系统配置与模块已加载完成。' },
  { time: new Date().toLocaleTimeString(), level: 'INFO', module: 'Initialization', message: '系统初始化完成，等待任务启动。' },
]);
const hasNewConsoleLog = ref(false);
const autoScrollConsole = ref(true);

// 添加终端日志
const addConsoleLog = (level, module, message) => {
  const now = new Date();
  const timeStr = now.toLocaleTimeString('en-US', { hour12: false });

  consoleLogs.value.push({
    time: timeStr,
    level: level.toUpperCase(),
    module: module,
    message: message,
  });

  hasNewConsoleLog.value = true;

  if (autoScrollConsole.value) {
    nextTick(() => {
      let terminal = terminalRef.value;
      if (terminal && terminal.$el) {
        terminal = terminal.$el;
      }
      if (terminal && typeof terminal.querySelector === 'function') {
        const body = terminal.querySelector('.terminal-body');
        if (body) body.scrollTop = body.scrollHeight;
      }
    });
  }

  setTimeout(() => {
    hasNewConsoleLog.value = false;
  }, 3000);
};

// 在Python后端中暴露给前端
// 触发账户资产、交易任务、交易记录更新
window.quant_addConsoleLog = (level, module, message) => {
  // 特殊指令处理：资产更新触发
  if (message === 'ASSET_UPDATE_TRIGGER') {
    fetchAccountData();
    return;
  }
  // 特殊指令处理：交易任务更新触发
  if (message === 'TRADE_TASK_UPDATE_TRIGGER') {
    getTradeTasks();
    return;
  }
  // 特殊指令处理：交易记录更新触发
  if (message === 'TRADE_RECORD_UPDATE_TRIGGER') {
    getTradeRecords();
    return;
  }
  addConsoleLog(level, module, message);
};

// 清空终端日志
const clearConsoleLogs = () => {
  ElMessageBox.confirm('确定要清空终端日志吗？此操作不可恢复', '清空日志', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  })
    .then(() => {
      consoleLogs.value = [];
      addConsoleLog('INFO', 'Console', '用户已清空终端日志。');
      ElMessage.success('终端日志已清空。');
    })
    .catch(() => {
      ElMessage.info('已取消清空操作。');
    });
};

// 复制终端日志
const copyConsoleLogs = () => {
  const logText = consoleLogs.value.map(log => `[${log.time}] ${log.module}: ${log.message}`).join('\n');

  navigator.clipboard
    .writeText(logText)
    .then(() => {
      ElMessage.success('终端日志已复制到剪贴板');
    })
    .catch(() => {
      ElMessage.error('复制失败，请手动复制');
    });
};

// 模拟Python实时日志
watch(
  () => tradeTasks.value.filter(s => s.status === 'running').length,
  newVal => {
    if (newVal > 0) {
      const logInterval = setInterval(() => {
        const runningStrategies = tradeTasks.value.filter(s => s.status === 'running');
        if (runningStrategies.length === 0) {
          clearInterval(logInterval);
          return;
        }

        const logTypes = [
          { level: 'DEBUG', module: 'price_monitor', messages: [`价格更新: ${runningStrategies[0].symbol} = ${(15 + Math.random() * 10).toFixed(2)}`, `网格检查: 当前价格在网格范围内 (${(14 + Math.random()).toFixed(2)} - ${(16 + Math.random()).toFixed(2)})`] },
          { level: 'INFO', module: 'position_tracker', messages: [`当前持仓: ${runningStrategies[0].symbol} = ${Math.floor(Math.random() * 1000)} 股`, `浮动盈亏: ${(Math.random() * 200 - 100).toFixed(2)} 元`] },
          { level: 'WARNING', module: 'risk_control', messages: [`持仓预警: ${runningStrategies[0].symbol} 接近最大持仓 (${Math.floor(Math.random() * 100)}%)`, `市场波动增加: 过去5分钟 ${(Math.random() * 5).toFixed(2)}%`] },
        ];

        const randomLog = logTypes[Math.floor(Math.random() * logTypes.length)];
        const randomMessage = randomLog.messages[Math.floor(Math.random() * randomLog.messages.length)];

        addConsoleLog(randomLog.level, randomLog.module, randomMessage);
      }, 5000);
    }
  }
);

// 初始化市场时间检查
const checkTradingTime = () => {
  const now = new Date();
  const day = now.getDay(); // 0 is Sunday, 6 is Saturday
  const hours = now.getHours();
  const minutes = now.getMinutes();
  const currentTime = hours * 100 + minutes;

  // 周末休市
  if (day === 0 || day === 6) {
    isTradingTime.value = false;
    isTradingDay.value = false; // 标记为非交易日
    marketStatus.text = '休市中';
    marketStatus.type = 'info';
    return;
  }

  isTradingDay.value = true; // 标记为交易日

  // A股交易时段判断
  if (currentTime >= 915 && currentTime < 925) {
    isTradingTime.value = false;
    marketStatus.text = '集合竞价';
    marketStatus.type = 'warning';
  } else if (currentTime >= 925 && currentTime < 930) {
    isTradingTime.value = false;
    marketStatus.text = '等待开盘';
    marketStatus.type = 'warning';
  } else if (currentTime >= 930 && currentTime < 1130) {
    isTradingTime.value = true;
    marketStatus.text = '交易中';
    marketStatus.type = 'success';
  } else if (currentTime >= 1130 && currentTime < 1300) {
    isTradingTime.value = false;
    marketStatus.text = '午间休市';
    marketStatus.type = 'primary';
  } else if (currentTime >= 1300 && currentTime < 1457) {
    isTradingTime.value = true;
    marketStatus.text = '交易中';
    marketStatus.type = 'success';
  } else if (currentTime >= 1457 && currentTime < 1500) {
    isTradingTime.value = true;
    marketStatus.text = '尾盘竞价';
    marketStatus.type = 'warning';
  } else {
    isTradingTime.value = false;
    marketStatus.text = '休市中';
    marketStatus.type = 'info';
  }
};

// 财经资讯相关
const page = ref(1);
const total = ref(0);
const pageSize = ref(100);
const newsItems = ref([]);
const eventItems = ref([]); // 初始化为空数组
const newsKeyword = ref(''); // 财经快报搜索关键词
const eventKeyword = ref(''); // 大事件搜索关键词
const keywordData = ref([]);
const searchInfo = ref({});
const activeInfoTab = ref('news'); // 财经资讯Tabs默认选中"财经快报"

// 获取新闻数据
const getNews = async () => {
  const res = await getNewsList({ page: page.value, pageSize: pageSize.value, keyword: newsKeyword.value, ...searchInfo.value });
  if (res.code === 0) {
    newsItems.value = res.data.list;
    total.value = res.data.total;
    page.value = res.data.page;
    pageSize.value = res.data.pageSize;
  }
};

// 获取事件数据
const getEvents = async () => {
  const start = new Date();
  start.setHours(0, 0, 0, 0);
  start.setDate(start.getDate() - 2);
  const end = new Date(start);
  end.setDate(start.getDate() + 60);

  const searchParams = {
    pageSize: 1000,
    ...searchInfo.value,
    keyword: eventKeyword.value,
    date_range: [start, end], //[formatToDateTime(start, 'YYYY-MM-DD'), formatToDateTime(end, 'YYYY-MM-DD')]
    orderKey: 'date',
  };

  const res = await getEventList(searchParams);
  if (res.code === 0) {
    let list = res.data.list;
    // 前端辅助过滤，确保搜索生效（以防后端忽略keyword参数）
    if (eventKeyword.value) {
      const kw = eventKeyword.value.trim().toLowerCase();
      list = list.filter(item => (item.name && item.name.toLowerCase().includes(kw)) || (item.industry && item.industry.toLowerCase().includes(kw)));
    }
    eventItems.value = list.map(item => ({
      date: item.date,
      name: item.name,
      industry: item.industry,
      city: item.city,
    }));
  }
};

// 获取可用股票列表
window.remoteStockSearch = async query => {
  const res = await getBaseStockPublic({ q: query, pageSize: 100 });

  if (res.code === 0 && res.data) {
    const options = res.data.list.map(item => ({
      value: item, // 保存完整的股票对象
      label: `${item.name} (${item.ts_code}) (${item.market})`,
    }));

    fApi.value.updateRule('symbol', { options });
  }
};

// 获取可用策略列表
const remoteStrategySearch = async query => {
  strategySearchLoading.value = true;

  try {
    const res = await getStrategyList({ name: query, pageSize: 100 });

    if (res.code === 0 && res.data) {
      availableStrategies.value = res.data.list;
    }
  } finally {
    strategySearchLoading.value = false;
  }
};

// 在挂载时获取数据
onMounted(() => {
  checkEnvironment();
  window.addEventListener('pywebviewready', checkEnvironment);
  updateTime();
  timer1 = setInterval(updateTime, 1000);
  checkTradingTime();
  getNews();
  getEvents();
  getTradeTasks();
  getTradeRecords();
  fetchAccountData().then(() => {
    if (currentAccount.value) {
      checkServerStatus(true);
    }
  });
  timer2 = setInterval(() => {
    if (sidebarVisible.value) {
      checkTradingTime();
      getNews();
    }
  }, 10000);
});

onUnmounted(() => {
  if (timer1) {
    clearInterval(timer1);
    timer1 = null;
  }
  if (timer2) {
    clearInterval(timer2);
    timer2 = null;
  }
});

// 新闻文本格式化
const useFormatText = str => {
  if (!str || typeof str !== 'string') return str;
  return str.replace(/【(.*?)】/g, '【<strong>$1</strong>】');
};

// 关键词高亮方法
const highlightKeywords = (text, keyword) => {
  if (!text || !keyword) return text;
  const keywords = keyword.trim().split(' ');
  let highlighted = text;
  keywords.forEach(k => {
    if (k) {
      const regex = new RegExp(k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
      highlighted = highlighted.replace(regex, match => `<span style="color: #F00;">${match}</span>`);
    }
  });
  return highlighted;
};

// 复制内容
const copyNews = row => {
  if (!row) return;
  const time = formatToDateTime(row.ctime, 'HH:mm:ss');
  const text = `${time} - ${row.content}`;

  navigator.clipboard
    .writeText(text)
    .then(() => {
      ElMessage.success('复制成功');
    })
    .catch(() => {
      ElMessage.error('复制失败');
    });
};

const copyEvent = row => {
  if (!row) return;
  const date = formatToDateTime(row.date, 'YYYY-MM-DD');
  const text = `${row.name}（${date}）`;

  navigator.clipboard
    .writeText(text)
    .then(() => {
      ElMessage.success('复制成功');
    })
    .catch(() => {
      ElMessage.error('复制失败');
    });
};

// 路由跳转
const pushTo = (name, query) => {
  if (query) {
    router.push({
      name: name,
      query: query,
    });
  } else {
    router.push({ name: name });
  }
};
</script>

<style scoped lang="scss">
@use './index.scss';

.quant-dashboard {
  .column-drag-handle {
    cursor: move;
    text-align: center;
    margin-bottom: 10px;
    color: #999;
    border-radius: 4px;
    padding: 4px 0;
    user-select: none;
    -webkit-app-region: no-drag; /* 防止 pywebview 窗口拖拽干扰 */
    transition: background-color 0.2s, border-color 0.2s;
    border: 1px solid transparent;

    &:hover,
    &:active {
      border: 1px solid #999;
    }

    .el-icon {
      pointer-events: none;
    }
  }

  .draggable-item {
    margin-bottom: 20px;
  }

  .ghost-card {
    opacity: 0.5;
    background: #f0f9eb;
    border: 1px dashed #67c23a;
  }

  .drag-card {
    opacity: 1;
    background: #999;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  }
}
</style>
