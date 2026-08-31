<template>
  <div>
    <!-- =========== 题材股票详情视图（页面内嵌，参考 baseStock 布局） =========== -->
    <div v-if="detailVisible" class="theme-stock-detail">
      <div class="gva-btn-list my-3">
        <el-button icon="back" @click="backToList">返回列表</el-button>
        <el-button icon="refresh" @click="getDetails({ id: detailForm.id })">刷新</el-button>
        <el-button v-auth="btnAuth.aiAdd" type="success" icon="magic-stick"
          @click="aiUpdateRow(detailForm)">AI概念分析</el-button>
        <el-button v-auth="btnAuth.aiAdd" type="success" icon="magic-stick"
          @click="openAiFundDrawer">AI基本面分析</el-button>
        <el-button v-auth="btnAuth.edit" type="primary" icon="edit"
          @click="updateThemeStockFunc(detailForm)">编辑</el-button>
        <el-button v-auth="btnAuth.delete" type="danger" icon="delete" @click="deleteRow(detailForm)">删除</el-button>
      </div>

      <el-card shadow="never" class="detail-card">
        <template #header>
          <div class="detail-header">
            <span class="font-bold">{{ detailForm.stock?.name || '-' }}（{{ detailForm.stock?.symbol || '-' }}）</span>
            <el-tag v-if="detailForm.theme?.name" type="primary">{{ detailForm.theme.name }}</el-tag>
          </div>
        </template>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="股票名称">{{ detailForm.stock?.name || '-' }}</el-descriptions-item>
          <el-descriptions-item label="股票代码">{{ detailForm.stock?.symbol || '-' }}</el-descriptions-item>
          <el-descriptions-item label="所属题材">{{ detailForm.theme?.name || '-' }}</el-descriptions-item>
          <el-descriptions-item label="TS代码">{{ detailForm.stock?.ts_code || '-' }}</el-descriptions-item>

          <el-descriptions-item label="梯队">
            <el-tag v-if="detailForm.tier" :type="getTierType(detailForm.tier)">{{ filterDict(String(detailForm.tier),
              tierOptions) }}</el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="相关度">{{ detailForm.relevance ?? '-' }}</el-descriptions-item>
          <el-descriptions-item label="纳入日期">{{ detailForm.in_date ? formatToDateTime(detailForm.in_date, 'YYYY-MM-DD')
            : '-'
            }}</el-descriptions-item>
          <el-descriptions-item label="排序">{{ detailForm.sort ?? '-' }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag v-if="detailForm.status !== null && detailForm.status !== undefined"
              :type="getStatusType(detailForm.status)">{{ filterDict(String(detailForm.status), statusOptions)
              }}</el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="更新时间">{{ detailForm.updated_at ? formatToDateTime(detailForm.updated_at,
            'YYYY-MM-DD HH:mm') : '-' }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ detailForm.created_at ? formatToDateTime(detailForm.created_at,
            'YYYY-MM-DD HH:mm') : '-' }}</el-descriptions-item>
        </el-descriptions>
      </el-card>

      <!-- 人工精选逻辑 -->
      <el-card shadow="never" class="detail-card">
        <template #header>
          <div class="detail-header">
            <span class="font-bold">精选逻辑（人工）</span>
            <span v-if="detailForm.updated_at" class="reason-meta">更新于 {{ formatToDateTime(detailForm.updated_at,
              'YYYY-MM-DD HH:mm') }}（{{ daysAgoText(detailForm.updated_at) }}）</span>
          </div>
        </template>

        <div class="reason-full markdown-body" v-html="renderMarkdown(detailForm.reason)"></div>

        <div v-if="!detailForm.reason" class="empty-analyze">
          <el-icon class="empty-analyze-icon">
            <MagicStick />
          </el-icon>

          <span class="empty-analyze-text">该股票暂无入选逻辑</span>

          <el-button v-auth="btnAuth.edit" type="primary" icon="edit" plain
            @click="updateThemeStockFunc(detailForm)">编辑</el-button>
        </div>
      </el-card>

      <!-- AI分析入选逻辑 -->
      <el-card shadow="never" class="detail-card">
        <template #header>
          <div class="detail-header">
            <span class="font-bold">AI入选逻辑</span>
            <span v-if="detailForm.updated_at" class="reason-meta">更新于 {{ formatToDateTime(detailForm.updated_at,
              'YYYY-MM-DD HH:mm') }}（{{ daysAgoText(detailForm.updated_at) }}）</span>
          </div>
        </template>

        <div class="reason-full markdown-body" v-html="renderMarkdown(detailForm.ai_reason)"></div>

        <div v-if="!detailForm.ai_reason" class="empty-analyze">
          <el-icon class="empty-analyze-icon">
            <MagicStick />
          </el-icon>
          <span class="empty-analyze-text">该股票暂无入选逻辑</span>
          <el-button v-auth="btnAuth.aiAdd" type="success" icon="magic-stick"
            @click="aiUpdateRow(detailForm)">AI概念分析</el-button>
        </div>
      </el-card>

      <el-card v-if="detailForm.stock?.ai_analyzed_at" shadow="never" class="detail-card">
        <template #header>
          <div class="detail-header">
            <span class="font-bold">基本面分析</span>
            <span v-if="detailForm.stock?.ai_analyzed_at" class="reason-meta">更新于 {{
              formatToDateTime(detailForm.stock.ai_analyzed_at,
                'YYYY-MM-DD HH:mm') }}（{{ daysAgoText(detailForm.stock.ai_analyzed_at) }}）</span>
          </div>
        </template>
        <el-tabs type="border-card">
          <el-tab-pane label="基本面分析">
            <div class="whitespace-pre-wrap">{{ detailForm.stock.fundamentals || '-' }}</div>
          </el-tab-pane>
          <el-tab-pane label="财务分析">
            <div class="whitespace-pre-wrap">{{ detailForm.stock.financial || '-' }}</div>
          </el-tab-pane>
          <el-tab-pane label="落地&业绩兑现">
            <div class="whitespace-pre-wrap">{{ detailForm.stock.realization || '-' }}</div>
          </el-tab-pane>
          <el-tab-pane label="题材热度&动量">
            <div class="whitespace-pre-wrap">{{ detailForm.stock.momentum || '-' }}</div>
          </el-tab-pane>
          <el-tab-pane label="风险提示">
            <div class="whitespace-pre-wrap">{{ detailForm.stock.risk || '-' }}</div>
          </el-tab-pane>
        </el-tabs>
      </el-card>

      <!-- 无基本面分析结果时提示更新 -->
      <el-card v-else-if="detailForm.stock?.id" shadow="never" class="detail-card">
        <div class="empty-analyze">
          <el-icon class="empty-analyze-icon">
            <MagicStick />
          </el-icon>
          <span class="empty-analyze-text">该股票尚未进行基本面分析</span>
          <el-button v-auth="btnAuth.aiAdd" type="success" icon="magic-stick"
            @click="openAiFundDrawer">AI基本面分析</el-button>
        </div>
      </el-card>
    </div>

    <template v-else>
      <div class="gva-search-box">
        <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline"
          @keyup.enter="onSubmit">
          <el-form-item label="ID" prop="id">
            <el-input-number v-model.number="searchInfo.id" :controls="false" placeholder="请输入ID" />
          </el-form-item>

          <el-form-item label="所属题材" prop="theme_id">
            <el-tree-select v-model="searchInfo.theme_id" :data="themeTreeOptions" node-key="value"
              :props="{ label: 'label', children: 'children' }" check-strictly filterable clearable placeholder="请选择题材"
              style="width: 220px" :loading="themeLoading" />
          </el-form-item>

          <el-form-item label="股票" prop="stock_id">
            <el-select v-model="searchInfo.stock_id" filterable remote clearable reserve-keyword placeholder="输入名称/代码搜索"
              :remote-method="loadStockOptions" :loading="stockLoading" style="width: 220px">
              <el-option v-for="item in stockOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>

          <template v-if="showAllQuery">
            <el-form-item label="精选逻辑" prop="reason">
              <el-input v-model="searchInfo.reason" :clearable="true" placeholder="请输入精选逻辑" />
            </el-form-item>

            <el-form-item label="梯队" prop="tier">
              <el-select v-model="searchInfo.tier" clearable placeholder="请选择梯队">
                <el-option v-for="(item, key) in tierOptions" :key="key" :label="item.label"
                  :value="Number(item.value)" />
              </el-select>
            </el-form-item>

            <el-form-item label="创建日期" prop="createdAtRange">
              <template #label>
                <span>
                  创建日期
                  <el-tooltip content="搜索范围是开始日期（包含）至结束日期（不包含）">
                    <el-icon>
                      <QuestionFilled />
                    </el-icon>
                  </el-tooltip>
                </span>
              </template>

              <el-date-picker v-model="searchInfo.createdAtRange" class="!w-280px" type="daterange" range-separator="至"
                start-placeholder="开始时间" end-placeholder="结束时间" />
            </el-form-item>
          </template>

          <el-form-item>
            <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
            <el-button icon="refresh" @click="onReset">重置</el-button>
            <el-button link type="primary" icon="arrow-down" @click="showAllQuery = true"
              v-if="!showAllQuery">展开</el-button>
            <el-button link type="primary" icon="arrow-up" @click="showAllQuery = false" v-else>收起</el-button>
          </el-form-item>
        </el-form>
      </div>

      <div class="gva-table-box">
        <div class="gva-btn-list">
          <el-button v-auth="btnAuth.add" type="primary" icon="plus" @click="openDialog()">新增</el-button>
          <el-button v-auth="btnAuth.aiAdd" type="success" icon="magic-stick" @click="openAiDrawer()">AI智能选股</el-button>
          <el-button v-auth="btnAuth.aiAdd" type="warning" icon="magic-stick" :disabled="!multipleSelection.length"
            @click="openAiBatchUpdate()">AI批量更新</el-button>
          <el-button v-auth="btnAuth.batchDelete" icon="delete" style="margin-left: 10px"
            :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
        </div>
        <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id"
          @selection-change="handleSelectionChange" @sort-change="handleSortChange">
          <el-table-column v-auth="btnAuth.batchDelete" type="selection" width="55" />

          <el-table-column sortable="custom" align="left" label="ID" prop="id" width="90" />

          <el-table-column align="left" label="股票/代码" min-width="210">
            <template #default="scope">
              <el-text v-if="scope.row.stock?.name" @click="getDetails(scope.row)">{{ scope.row.stock.name }}（{{
                scope.row.stock?.ts_code || '-' }}）</el-text>
              <span v-else>-</span>
            </template>
          </el-table-column>

          <el-table-column sortable="custom" align="left" label="涨跌幅" prop="change_pct" width="90">
            <template #default="scope">
              <span v-if="scope.row.stock?.change_pct !== null" :class="rateClass(scope.row.stock.change_pct)">{{
                Number(scope.row.stock.change_pct).toFixed(2) }}%</span>
              <span v-else>-</span>
            </template>
          </el-table-column>

          <el-table-column align="left" label="所属题材" min-width="130">
            <template #default="scope">
              <el-tag type="primary" v-if="scope.row.theme?.name">{{ scope.row.theme.name }}</el-tag>
              <span v-else>-</span>
            </template>
          </el-table-column>

          <el-table-column align="left" label="入选逻辑" min-width="210">
            <template #default="scope">
              <el-tooltip v-if="scope.row.ai_reason || scope.row.reason" placement="top" :show-after="300">
                <template #content>
                  <div v-if="scope.row.updated_at" class="reason-tooltip-time">
                    <div v-if="scope.row.updated_at">更新于 {{ formatToDateTime(scope.row.updated_at, 'YYYY-MM-DD HH:mm')
                      }}（{{ daysAgoText(scope.row.updated_at) }}）</div>
                  </div>
                  <div class="reason-tooltip">{{ scope.row.ai_reason || scope.row.reason }}</div>
                </template>
                <div class="reason-cell">{{ scope.row.ai_reason || scope.row.reason }}</div>
              </el-tooltip>
              <span v-else>-</span>
            </template>
          </el-table-column>

          <el-table-column sortable="custom" align="left" label="梯队" prop="tier" width="90">
            <template #default="scope">
              <el-tag v-if="scope.row.tier" :type="getTierType(scope.row.tier)">{{ filterDict(String(scope.row.tier),
                tierOptions) }}</el-tag>
              <span v-else>-</span>
            </template>
          </el-table-column>

          <el-table-column sortable="custom" align="left" label="相关度" prop="relevance" width="90" />

          <el-table-column align="left" label="纳入日期" prop="in_date" width="120">
            <template #default="scope">{{ formatToDateTime(scope.row.in_date, 'YYYY-MM-DD') }}</template>
          </el-table-column>

          <el-table-column sortable="custom" align="left" label="排序" prop="sort" width="80" />

          <el-table-column align="center" label="状态" prop="status" width="90">
            <template #default="scope">
              <el-tag :type="getStatusType(scope.row.status)">{{ filterDict(String(scope.row.status), statusOptions)
                }}</el-tag>
            </template>
          </el-table-column>

          <el-table-column sortable align="left" label="更新时间" prop="updated_at" min-width="120">
            <template #default="scope">{{ formatToDateTime(scope.row.updated_at, 'MM-DD HH:mm') }}</template>
          </el-table-column>

          <el-table-column sortable align="left" label="创建时间" prop="created_at" min-width="120">
            <template #default="scope">{{ formatToDateTime(scope.row.created_at, 'MM-DD HH:mm') }}</template>
          </el-table-column>

          <el-table-column align="left" label="操作" fixed="right" :min-width="210">
            <template #default="scope">
              <el-button v-auth="btnAuth.info" type="primary" link icon="view" class="table-button"
                @click="getDetails(scope.row)">查看</el-button>
              <el-button v-auth="btnAuth.aiAdd" type="success" link icon="magic-stick" class="table-button"
                :loading="aiUpdatingId === scope.row.id" @click="aiUpdateRow(scope.row)">AI</el-button>
              <el-button v-auth="btnAuth.edit" type="primary" link icon="edit" class="table-button"
                @click="updateThemeStockFunc(scope.row)">编辑</el-button>
              <!-- <el-button v-auth="btnAuth.delete" type="primary" link icon="delete"
                @click="deleteRow(scope.row)">删除</el-button> -->
            </template>
          </el-table-column>
        </el-table>

        <div class="gva-pagination">
          <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize"
            :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange"
            @size-change="handleSizeChange" />
        </div>
      </div>
    </template>

    <el-drawer destroy-on-close :size="appStore.drawerSize * 1.2" v-model="dialogFormVisible" :show-close="false"
      :before-close="closeDialog">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">{{ type === 'create' ? '新增' : '编辑' }}</span>
          <div>
            <el-button :loading="btnLoading" type="primary" @click="enterDialog">确 定</el-button>
            <el-button @click="closeDialog">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form :model="formData" label-position="top" ref="elFormRef" :rules="rule" label-width="80px">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="所属题材" prop="theme_id">
              <el-tree-select v-model="formData.theme_id" :data="themeTreeOptions" node-key="value"
                :props="{ label: 'label', children: 'children' }" check-strictly filterable clearable
                placeholder="请选择题材" style="width: 100%" :loading="themeLoading" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="股票" prop="stock_id">
              <el-select v-model="formData.stock_id" filterable remote clearable reserve-keyword placeholder="输入名称/代码搜索"
                :remote-method="loadStockOptions" :loading="stockLoading" style="width: 100%">
                <el-option v-for="item in stockOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="精选逻辑" prop="reason">
              <VditorEditor v-model="formData.reason" :min-height="200" cache-id="theme-stock-reason" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="AI入选逻辑" prop="ai_reason">
              <VditorEditor v-model="formData.ai_reason" :min-height="200" cache-id="theme-stock-ai-reason" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="梯队" prop="tier">
              <el-select v-model="formData.tier" placeholder="请选择梯队">
                <el-option v-for="(item, key) in tierOptions" :key="key" :label="item.label"
                  :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="相关度" prop="relevance">
              <el-input-number v-model="formData.relevance" style="width: 100%" :precision="2" :clearable="true" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="纳入日期" prop="in_date">
              <el-date-picker v-model="formData.in_date" type="date" style="width: 100%" placeholder="选择日期"
                :clearable="true" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="排序" prop="sort">
              <el-input v-model.number="formData.sort" :clearable="true" placeholder="请输入排序" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="状态" prop="status">
              <el-select v-model="formData.status" placeholder="请选择状态">
                <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label"
                  :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <!-- AI 智能选股 Drawer -->
    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="aiDrawerVisible" :show-close="false"
      :before-close="closeAiDrawer">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">AI智能选股</span>
          <div>
            <el-button :loading="aiBtnLoading" type="success" @click="enterAiDrawer">开始选股</el-button>
            <el-button @click="closeAiDrawer">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form :model="aiFormData" label-position="top" ref="aiFormRef" :rules="aiRule" label-width="80px">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="概念/题材" prop="theme_id">
              <el-tree-select v-model="aiFormData.theme_id" :data="themeTreeOptions" node-key="value"
                :props="{ label: 'label', children: 'children' }" check-strictly filterable clearable
                placeholder="请选择概念" style="width: 100%" :loading="themeLoading" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="最大股票数" prop="max_stocks">
              <el-input-number v-model="aiFormData.max_stocks" style="width: 100%" :min="1" :max="100"
                placeholder="建议 1-20" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="AI提供商" prop="provider">
              <el-select v-model="aiFormData.provider" style="width: 100%" placeholder="请选择厂商"
                @change="onProviderChange">
                <el-option v-for="item in providerOptions" :key="item.value" :label="item.label" :value="item.value"
                  :disabled="item.disabled" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="大模型" prop="model">
              <el-select v-model="aiFormData.model" style="width: 100%" placeholder="请选择模型"
                :disabled="!currentProviderModels.length">
                <el-option v-for="m in currentProviderModels" :key="m.value" :label="m.label" :value="m.value" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="接口模式" prop="api_format">
              <el-select v-model="aiFormData.api_format" style="width: 100%">
                <el-option label="响应接口" value="responses" />
                <el-option label="对话接口" value="chat-completions" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="4">
            <el-form-item label="联网搜索">
              <div class="flex items-center">
                <el-switch v-model="aiFormData.web_search" :active-value="true" :inactive-value="false" />
              </div>
            </el-form-item>
          </el-col>

          <el-col v-if="aiFormData.api_format === 'chat-completions' && aiFormData.web_search" :span="8">
            <el-form-item label="搜索引擎" prop="search_engine">
              <el-select v-model="aiFormData.search_engine" style="width: 100%">
                <el-option v-for="item in webSearchOptions" :key="item.value" :label="item.label" :value="item.value"
                  :disabled="item.disabled" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="相关度过滤" prop="min_relevance">
              <template #label>
                <span>
                  相关度过滤
                  <el-tooltip content="AI分析结果中相关度低于该数值的股票不入库（默认 10）">
                    <el-icon>
                      <QuestionFilled />
                    </el-icon>
                  </el-tooltip>
                </span>
              </template>
              <el-input-number v-model="aiFormData.min_relevance" style="width: 100%" :min="0" :max="100"
                placeholder="低于该值不入库" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="纳入日期" prop="in_date">
              <el-date-picker v-model="aiFormData.in_date" type="date" style="width: 100%" placeholder="选择纳入日期"
                :clearable="true" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="已存在股票" prop="exist_handling">
              <el-radio-group v-model="aiFormData.exist_handling">
                <el-radio value="overwrite">覆盖（更新入选逻辑/梯队/相关度）</el-radio>
                <el-radio value="skip">跳过</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>

          <el-col :span="4">
            <el-form-item label="定时执行">
              <el-switch v-model="aiFormData.scheduled_enabled" @change="onScheduledEnabledChange(aiFormData, $event)"
                active-text="开" inactive-text="关" />
            </el-form-item>
          </el-col>
          <el-col v-if="aiFormData.scheduled_enabled" :span="8">
            <el-form-item label="执行时间" prop="scheduled_at">
              <el-date-picker v-model="aiFormData.scheduled_at" type="datetime" style="width: 100%" placeholder="选择执行时间"
                :disabled-date="disabledScheduledDate" value-format="YYYY-MM-DDTHH:mm:ssZ" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <!-- AI 一键更新个股：手动指定大模型 Drawer -->
    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="aiUpdateDrawerVisible" :show-close="false"
      :before-close="closeAiUpdateDrawer">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">{{ aiUpdateDrawerTitle }}</span>
          <div>
            <el-button :loading="aiUpdatingId !== null || aiBatchUpdating" type="success"
              @click="submitAiUpdate">开始分析</el-button>
            <el-button @click="closeAiUpdateDrawer">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form :model="aiUpdateFormData" label-position="top">
        <el-form-item label="个股">
          <el-input :model-value="aiUpdateStockName" disabled />
        </el-form-item>

        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="AI提供商" prop="provider">
              <el-select v-model="aiUpdateFormData.provider" style="width: 100%" placeholder="请选择厂商"
                @change="onAiUpdateProviderChange">
                <el-option v-for="item in providerOptions" :key="item.value" :label="item.label" :value="item.value"
                  :disabled="item.disabled" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="大模型" prop="model">
              <el-select v-model="aiUpdateFormData.model" style="width: 100%" placeholder="请选择模型"
                :disabled="!currentAiUpdateModels.length">
                <el-option v-for="m in currentAiUpdateModels" :key="m.value" :label="m.label" :value="m.value" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="接口模式" prop="api_format">
              <el-select v-model="aiUpdateFormData.api_format" style="width: 100%">
                <el-option label="响应接口" value="responses" />
                <el-option label="对话接口" value="chat-completions" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="4">
            <el-form-item label="联网搜索">
              <div class="flex items-center">
                <el-switch v-model="aiUpdateFormData.web_search" :active-value="true" :inactive-value="false" />
              </div>
            </el-form-item>
          </el-col>

          <el-col v-if="aiUpdateFormData.api_format === 'chat-completions' && aiUpdateFormData.web_search" :span="8">
            <el-form-item label="搜索引擎" prop="search_engine">
              <el-select v-model="aiUpdateFormData.search_engine" style="width: 100%">
                <el-option v-for="item in webSearchOptions" :key="item.value" :label="item.label" :value="item.value"
                  :disabled="item.disabled" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="4">
            <el-form-item label="定时执行">
              <el-switch v-model="aiUpdateFormData.scheduled_enabled"
                @change="onScheduledEnabledChange(aiUpdateFormData, $event)" active-text="开" inactive-text="关" />
            </el-form-item>
          </el-col>

          <el-col v-if="aiUpdateFormData.scheduled_enabled" :span="8">
            <el-form-item label="执行时间">
              <el-date-picker v-model="aiUpdateFormData.scheduled_at" type="datetime" style="width: 100%"
                placeholder="选择执行时间" :disabled-date="disabledScheduledDate" value-format="YYYY-MM-DDTHH:mm:ssZ" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <!-- AI 基本面分析 Drawer：手动指定大模型（参考 baseStock AI 自动分析） -->
    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="aiFundDrawerVisible" :show-close="false"
      :before-close="closeAiFundDrawer">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">AI基本面分析</span>
          <div>
            <el-button :loading="aiFundBtnLoading" type="success" @click="submitAiFundAnalyze">开始分析</el-button>
            <el-button @click="closeAiFundDrawer">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form :model="aiFundFormData" label-position="top" ref="aiFundFormRef" :rules="aiFundRule" label-width="80px">
        <el-form-item label="个股">
          <el-input
            :model-value="detailForm.stock?.name ? `${detailForm.stock.name}（${detailForm.stock.symbol || '-'}）` : '-'"
            disabled />
        </el-form-item>

        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="AI提供商" prop="provider">
              <el-select v-model="aiFundFormData.provider" style="width: 100%" placeholder="请选择厂商"
                @change="onAiFundProviderChange">
                <el-option v-for="item in providerOptions" :key="item.value" :label="item.label" :value="item.value"
                  :disabled="item.disabled" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="大模型" prop="model">
              <el-select v-model="aiFundFormData.model" style="width: 100%" placeholder="请选择模型"
                :disabled="!currentAiFundModels.length">
                <el-option v-for="m in currentAiFundModels" :key="m.value" :label="m.label" :value="m.value" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="接口模式" prop="api_format">
              <el-select v-model="aiFundFormData.api_format" style="width: 100%">
                <el-option label="响应接口" value="responses" />
                <el-option label="对话接口" value="chat-completions" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="4">
            <el-form-item label="联网搜索">
              <div class="flex items-center">
                <el-switch v-model="aiFundFormData.web_search" :active-value="true" :inactive-value="false" />
              </div>
            </el-form-item>
          </el-col>

          <el-col v-if="aiFundFormData.api_format === 'chat-completions' && aiFundFormData.web_search" :span="8">
            <el-form-item label="搜索引擎" prop="search_engine">
              <el-select v-model="aiFundFormData.search_engine" style="width: 100%">
                <el-option v-for="item in webSearchOptions" :key="item.value" :label="item.label" :value="item.value"
                  :disabled="item.disabled" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="4">
            <el-form-item label="定时执行">
              <el-switch v-model="aiFundFormData.scheduled_enabled"
                @change="onScheduledEnabledChange(aiFundFormData, $event)" active-text="开" inactive-text="关" />
            </el-form-item>
          </el-col>

          <el-col v-if="aiFundFormData.scheduled_enabled" :span="8">
            <el-form-item label="执行时间">
              <el-date-picker v-model="aiFundFormData.scheduled_at" type="datetime" style="width: 100%"
                placeholder="选择执行时间" :disabled-date="disabledScheduledDate" value-format="YYYY-MM-DDTHH:mm:ssZ" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive, watch, computed } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { formatToDateTime } from '@/utils/dateTimeUtils';
import { marked } from 'marked';
import { getDictFunc, filterDict } from '@/utils/format';
import { createThemeStock, deleteThemeStock, deleteThemeStockByIds, updateThemeStock, getThemeStockList, findThemeStock, aiAddThemeStocks, aiUpdateThemeStock, aiUpdateThemeStocks } from '@/api/quant/themeStock';
import { getThemeList } from '@/api/quant/theme';
import { getBaseStockList, findBaseStock, aiAnalyzeStocks } from '@/api/quant/baseStock';
import { providerOptions } from '@/data/aiProviderOptions';
import { webSearchOptions } from '@/data/webSearchOptions';
// 富文本组件
// import RichEdit from '@/components/richtext/rich-edit.vue';
import RichView from '@/components/richtext/rich-view.vue';
import ArrayCtrl from '@/components/arrayCtrl/arrayCtrl.vue';
import VditorEditor from '@/components/vditorEditor/VditorEditor.vue';

defineOptions({
  name: 'ThemeStock',
});
// 按钮权限实例化
const btnAuth = useBtnAuth();

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();
const route = useRoute();
const router = useRouter();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const statusOptions = ref([]);
const tierOptions = ref([]);
const themeTreeOptions = ref([]);
const themeLoading = ref(false);
const stockOptions = ref([]);
const stockLoading = ref(false);

const resolveRouteThemeId = () => {
  const rawThemeId = route.query.theme_id;
  if (rawThemeId === undefined || rawThemeId === null || rawThemeId === '') return undefined;
  const value = Array.isArray(rawThemeId) ? rawThemeId[0] : rawThemeId;
  const parsed = Number(value);
  return Number.isNaN(parsed) ? undefined : parsed;
};

const createDefaultFormData = () => ({
  theme_id: resolveRouteThemeId(),
  stock_id: undefined,
  relevance: 0,
  reason: '',
  ai_reason: '',
  in_date: new Date(),
  tier: 0,
  sort: 0,
  status: 1,
});

const createDefaultSearchInfo = () => ({
  id: undefined,
  theme_id: resolveRouteThemeId(),
  stock_id: undefined,
  reason: undefined,
  tier: undefined,
  createdAtRange: undefined,
});

// 自动化生成的字典（可能为空）以及字段
const formData = ref(createDefaultFormData());

// 验证规则
const rule = reactive({});

const elFormRef = ref();
const elSearchFormRef = ref();
// 表格实例（用于清空排序状态等）
const multipleTable = ref();

// =========== 表格控制部分 ===========
const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
const tableData = ref([]);
const searchInfo = ref(createDefaultSearchInfo());

// 判断当前激活路由的页面组件是否为本页面。
// 注意：watch 回调触发时 route 已更新为新路由，而 onDeactivated 晚于 watch 执行，
// 因此不能用激活状态标记（如 onDeactivated）拦截，必须直接比较当前激活组件
const isCurrentPage = () => {
  const last = route.matched[route.matched.length - 1];
  return last?.components?.default?.name === 'ThemeStock';
};

watch(
  () => route.query.theme_id,
  () => {
    // 页面被 keep-alive 缓存后，切换到其他路由（如提交 AI 后跳转 aiTask）也会引起 query 变化，
    // 此时当前激活组件不是本页面，直接忽略，避免发起无意义的 getThemeStockList 请求
    if (!isCurrentPage()) return;
    const routeThemeId = resolveRouteThemeId();
    searchInfo.value = {
      ...searchInfo.value,
      theme_id: routeThemeId,
    };
    formData.value = {
      ...formData.value,
      theme_id: routeThemeId,
    };
    getTableData();
  }
);

// 支持服务端排序的表头字段（对应后端 orderKey 白名单）
const SORTABLE_KEYS = ['id', 'change_pct', 'tier', 'relevance', 'sort'];

// 排序变更处理（服务端排序）
const handleSortChange = ({ prop, order }) => {
  if (!SORTABLE_KEYS.includes(prop)) return;
  // order: ascending(升序) / descending(降序) / null(取消排序)
  if (order === 'ascending') {
    searchInfo.value.orderKey = prop;
    searchInfo.value.desc = false;
  } else if (order === 'descending') {
    searchInfo.value.orderKey = prop;
    searchInfo.value.desc = true;
  } else {
    // 取消排序，恢复默认排序
    delete searchInfo.value.orderKey;
    delete searchInfo.value.desc;
  }
  page.value = 1;
  getTableData();
};

// 重置
const onReset = () => {
  searchInfo.value = createDefaultSearchInfo();
  multipleTable.value?.clearSort();
  getTableData();
};

// 搜索
const onSubmit = () => {
  elSearchFormRef.value?.validate(async valid => {
    if (!valid) return;
    page.value = 1;
    getTableData();
  });
};

// 分页
const handleSizeChange = val => {
  pageSize.value = val;
  getTableData();
};

// 修改页面容量
const handleCurrentChange = val => {
  page.value = val;
  getTableData();
};

// 查询
const getTableData = async () => {
  const searchParams = { ...searchInfo.value };
  const routeThemeId = resolveRouteThemeId();
  if (routeThemeId !== undefined && searchParams.theme_id === undefined) {
    searchParams.theme_id = routeThemeId;
  }

  const table = await getThemeStockList({ page: page.value, pageSize: pageSize.value, ...searchParams });
  if (table.code === 0) {
    tableData.value = table.data.list;
    total.value = table.data.total;
    page.value = table.data.page;
    pageSize.value = table.data.pageSize;
  }
};

getTableData();

// ============== 表格控制部分结束 ===============

// 获取需要的字典 可能为空 按需保留
const setOptions = async () => {
  [statusOptions.value, tierOptions.value] = await Promise.all([getDictFunc('status'), getDictFunc('quant_theme_stock_tier')]);
};

// 递归构建 el-tree-select 所需的树形结构
const buildTreeOptions = list => {
  return list.map(item => ({
    value: item.id,
    label: item.name,
    children: item.children?.length ? buildTreeOptions(item.children) : undefined,
  }));
};

const loadThemeOptions = async () => {
  themeLoading.value = true;
  const res = await getThemeList();
  themeLoading.value = false;
  if (res.code === 0 && Array.isArray(res.data)) {
    themeTreeOptions.value = buildTreeOptions(res.data);
  }
};

// 格式化股票选项 label：名称 (代码)
const formatStockLabel = item => {
  const name = item?.name || '';
  const symbol = item?.symbol ? ` (${item.symbol})` : '';
  return `${name}${symbol}`;
};

// 远程搜索股票
const loadStockOptions = async query => {
  stockLoading.value = true;
  const res = await getBaseStockList({ page: 1, pageSize: 50, q: query || undefined });
  stockLoading.value = false;
  if (res.code === 0 && Array.isArray(res.data.list)) {
    stockOptions.value = res.data.list.map(item => ({
      value: item.id,
      label: formatStockLabel(item),
    }));
  }
};

// 回显搜索区已选股票的名称（刷新页面后下拉框正常显示）
const loadSearchStockLabel = async () => {
  const stockId = searchInfo.value.stock_id;
  if (stockId === undefined || stockId === null || stockId === '') return;
  if (stockOptions.value.some(item => item.value === Number(stockId))) return;
  const res = await findBaseStock({ ID: stockId });
  if (res.code === 0 && res.data) {
    stockOptions.value = [{ value: res.data.id, label: formatStockLabel(res.data) }];
  }
};

// 获取需要的字典 可能为空 按需保留
setOptions();
loadThemeOptions();
// 回显搜索区已选股票的名称（需在 loadSearchStockLabel 定义之后调用）
loadSearchStockLabel();

// 多选数据
const multipleSelection = ref([]);
// 多选
const handleSelectionChange = val => {
  multipleSelection.value = val;
};

// 删除行
const deleteRow = row => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(() => {
    deleteThemeStockFunc(row);
  });
};

// 多选删除
const onDelete = async () => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(async () => {
    const ids = [];
    if (multipleSelection.value.length === 0) {
      ElMessage({
        type: 'warning',
        message: '请选择要删除的数据',
      });
      return;
    }
    multipleSelection.value &&
      multipleSelection.value.map(item => {
        ids.push(item.id);
      });
    const res = await deleteThemeStockByIds({ ids });
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '删除成功',
      });
      if (tableData.value.length === ids.length && page.value > 1) {
        page.value--;
      }
      getTableData();
    }
  });
};

// 行为控制标记（弹窗内部需要增还是改）
const type = ref('');

// 更新行
const updateThemeStockFunc = async row => {
  type.value = 'update';
  formData.value = {
    ...row,
    theme_id: row.theme_id ?? resolveRouteThemeId(),
  };
  // 回显股票名称：将已选股票补充到下拉选项中
  if (row.stock_id && row.stock) {
    stockOptions.value = [
      {
        value: row.stock_id,
        label: formatStockLabel(row.stock),
      },
    ];
  }
  dialogFormVisible.value = true;
};

// 删除行
const deleteThemeStockFunc = async row => {
  const res = await deleteThemeStock({ ID: row.id });
  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '删除成功',
    });

    // 详情视图内删除后返回列表
    if (detailVisible.value) {
      backToList();
      return;
    }

    if (tableData.value.length === 1 && page.value > 1) {
      page.value--;
    }
    getTableData();
  }
};

const detailForm = ref({});

// 页面内嵌详情视图控制标记（参考 baseStock 布局）
const detailVisible = ref(false);

// 打开详情（页面内嵌视图）
const getDetails = async row => {
  const res = await findThemeStock({ ID: row.id });
  if (res.code === 0) {
    detailForm.value = res.data;
    detailVisible.value = true;
  }
};

// 返回列表
const backToList = () => {
  detailVisible.value = false;
  detailForm.value = {};
  getTableData();
};

// =========== AI 参数选择记忆（AI供应商/大模型/接口模式/联网搜索/搜索引擎） ===========
// 记录用户上一次在任意 AI drawer 中选择的 AI 参数，下次打开对应 drawer 时回填；
// 持久化到 localStorage，刷新页面后依然生效
const AI_SELECTION_STORAGE_KEY = 'themeStock_aiSelection';

// 从 localStorage 读取记忆缓存，缺失或解析失败时返回 null（回归默认值）
const readCachedAiSelection = () => {
  try {
    const raw = localStorage.getItem(AI_SELECTION_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (!parsed) return null;
    return {
      provider: parsed.provider,
      model: parsed.model,
      api_format: parsed.api_format,
      web_search: parsed.web_search,
      search_engine: parsed.search_engine,
    };
  } catch {
    return null;
  }
};

const aiSelectionMemory = ref(readCachedAiSelection());

// 将表单中的 AI 参数记录为"上一次的选择"，并写入 localStorage
const rememberAiSelection = source => {
  if (!source) return;
  const mem = {
    provider: source.provider,
    model: source.model,
    api_format: source.api_format,
    web_search: !!source.web_search,
    search_engine: source.search_engine,
  };
  aiSelectionMemory.value = mem;
  try {
    localStorage.setItem(AI_SELECTION_STORAGE_KEY, JSON.stringify(mem));
  } catch {
    // 存储不可用（如隐私模式）时仅保留内存记忆
  }
};

// 用记忆覆盖默认值；无记忆时返回原默认值
const applyAiSelection = defaults => {
  const mem = aiSelectionMemory.value;
  if (!mem) return defaults;
  return {
    ...defaults,
    provider: mem.provider,
    model: mem.model,
    api_format: mem.api_format,
    web_search: mem.web_search,
    search_engine: mem.search_engine,
  };
};
// =========== AI 参数选择记忆结束 ===========

// =========== AI 基本面分析部分（参考 baseStock AI 自动分析，手动指定大模型） ===========
// AI 基本面分析 drawer 显示控制
const aiFundDrawerVisible = ref(false);
// 提交按钮 loading
const aiFundBtnLoading = ref(false);
const aiFundFormRef = ref();

// 获取默认定时执行时间（下一个空闲时段，避开高峰 9:00-12:00、14:00-18:00）
const getDefaultScheduledAt = () => {
  // 北京时间 = UTC + 8 小时（无夏令时），加上 10 分钟缓冲
  // Date.now() 返回 UTC 毫秒；在 UTC 时间上直接加 8 小时得到北京时间对应的毫秒值
  const bjMs = Date.now() + 8 * 3600000 + 10 * 60000;
  const d = new Date(bjMs);
  // 因为已经加了 +8h 偏移，用 getUTC* 方法读取的就是北京时间各分量
  let h = d.getUTCHours();
  let m = d.getUTCMinutes();
  let s = d.getUTCSeconds();
  // 高峰时段避开：9:00-12:00（含 9 不含 12）和 14:00-18:00（含 14 不含 18）
  if ((h >= 9 && h < 12) || (h >= 14 && h < 18)) {
    if (h < 12) { h = 12; m = 0; s = 0; }
    else { h = 18; m = 0; s = 0; }
  }
  const pad = n => String(n).padStart(2, '0');
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}T${pad(h)}:${pad(m)}:${pad(s)}+08:00`;
};

// 定时执行开关变更时自动设置默认时间
const onScheduledEnabledChange = (formData, enabled) => {
  if (enabled) formData.scheduled_at = getDefaultScheduledAt();
  else formData.scheduled_at = null;
};

const createDefaultAiFundFormData = () => applyAiSelection({
  provider: 'deepseek',
  model: 'deepseek-v4-flash',
  web_search: true, // true=强制开启联网搜索；false=强制关闭
  api_format: 'responses', // 接口模式：responses=Responses API；chat-completions=OpenAI 兼容接口
  search_engine: 'baidu', // 联网搜索引擎（chat-completions 模式且开启联网搜索时生效）
  scheduled_enabled: false, // 定时执行开关
  scheduled_at: getDefaultScheduledAt(), // 默认为最近的空闲时段
});

const aiFundFormData = ref(createDefaultAiFundFormData());

// AI 基本面分析表单验证规则
const aiFundRule = reactive({
  provider: [{ required: true, message: '请选择厂商', trigger: 'change' }],
  model: [{ required: true, message: '请选择模型', trigger: 'change' }],
});

// 当前所选厂商的模型列表
const currentAiFundModels = computed(() => {
  const provider = providerOptions.find(item => item.value === aiFundFormData.value.provider);
  return provider?.models || [];
});

// 切换厂商时自动带上该厂商第一个模型
const onAiFundProviderChange = () => {
  aiFundFormData.value.model = currentAiFundModels.value[0]?.value || '';
};

// 打开 AI 基本面分析 drawer
const openAiFundDrawer = () => {
  const stockId = detailForm.value.stock?.id;
  if (!stockId) {
    ElMessage({
      type: 'warning',
      message: '未获取到关联股票信息，无法进行AI分析',
    });
    return;
  }
  aiFundFormData.value = createDefaultAiFundFormData();
  aiFundDrawerVisible.value = true;
};

// 关闭 AI 基本面分析 drawer（关闭前记录本次选择，下次打开时回填）
const closeAiFundDrawer = () => {
  rememberAiSelection(aiFundFormData.value);
  aiFundDrawerVisible.value = false;
  aiFundFormData.value = createDefaultAiFundFormData();
};

// 提交 AI 基本面分析（异步任务化：提交后立即返回任务ID，跳转执行进度页面查看进度）
const submitAiFundAnalyze = async () => {
  const stockId = detailForm.value.stock?.id;
  if (!stockId) {
    ElMessage({
      type: 'warning',
      message: '未获取到关联股票信息，无法进行AI分析',
    });
    return;
  }
  // 定时执行校验：开启定时但未选择执行时间时提示
  if (aiFundFormData.value.scheduled_enabled && !aiFundFormData.value.scheduled_at) {
    ElMessage({ type: 'warning', message: '请选择定时执行时间' });
    return;
  }
  aiFundBtnLoading.value = true;
  aiFundFormRef.value?.validate(async valid => {
    if (!valid) return (aiFundBtnLoading.value = false);
    try {
      const res = await aiAnalyzeStocks({
        stock_ids: [stockId],
        provider: aiFundFormData.value.provider,
        model: aiFundFormData.value.model,
        web_search: aiFundFormData.value.web_search,
        api_format: aiFundFormData.value.api_format,
        search_engine: aiFundFormData.value.search_engine,
        scheduled_at: buildScheduledAt(aiFundFormData.value),
      });
      if (res.code === 0) {
        ElMessage({
          type: 'success',
          message: '任务已创建，正在前往执行进度页',
        });
        closeAiFundDrawer();
        goAiTaskPage(res.data?.task_id);
      }
    } finally {
      aiFundBtnLoading.value = false;
    }
  });
};
// =========== AI 基本面分析部分结束 ===========

// 弹窗控制标记
const dialogFormVisible = ref(false);

// 打开弹窗
const openDialog = () => {
  type.value = 'create';
  const routeThemeId = resolveRouteThemeId();
  formData.value = {
    ...createDefaultFormData(),
    theme_id: routeThemeId !== undefined ? routeThemeId : formData.value.theme_id,
  };
  dialogFormVisible.value = true;
};

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;
  formData.value = createDefaultFormData();
};
// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  elFormRef.value?.validate(async valid => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createThemeStock(formData.value);
        break;
      case 'update':
        res = await updateThemeStock(formData.value);
        break;
      default:
        res = await createThemeStock(formData.value);
        break;
    }
    btnLoading.value = false;
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '创建/更改成功',
      });
      const editingId = type.value === 'update' ? formData.value.id : undefined;
      closeDialog();
      getTableData();
      // 详情视图内编辑后，重新拉取详情保持展示最新数据
      if (detailVisible.value && editingId) {
        getDetails({ id: editingId });
      }
    }
  });
};

// =========== AI 智能选股部分 ===========
// AI 选股 drawer 显示控制
const aiDrawerVisible = ref(false);
// AI 选股提交按钮 loading
const aiBtnLoading = ref(false);
const aiFormRef = ref();

// 大模型厂商及模型选项，已抽离至 src/data/aiProviderOptions.js
const currentProviderModels = computed(() => {
  const provider = providerOptions.find(item => item.value === aiFormData.value.provider);
  return provider?.models || [];
});

// 切换厂商时自动带上该厂商第一个模型
const onProviderChange = () => {
  aiFormData.value.model = currentProviderModels.value[0]?.value || '';
};



const createDefaultAiFormData = () => applyAiSelection({
  theme_id: resolveRouteThemeId(),
  max_stocks: 10,
  in_date: new Date(),
  provider: 'deepseek',
  model: 'deepseek-v4-flash',
  exist_handling: 'overwrite',
  min_relevance: 10,
  web_search: true, // true=强制开启联网搜索；false=强制关闭；不传(undefined)=跟随模型配置
  api_format: 'responses', // 接口模式：''=跟随配置；responses=Responses API；chat-completions=OpenAI 兼容接口
  search_engine: 'baidu', // 联网搜索引擎（chat-completions 模式且开启联网搜索时生效；默认百度搜索）
  scheduled_enabled: false, // 定时执行开关
  scheduled_at: null, // 未开启定时时为空，避免提交时被后端当作定时任务；开启定时时由 onScheduledEnabledChange 填入默认空闲时段
});

const aiFormData = ref(createDefaultAiFormData());

// AI 选股表单验证规则
const aiRule = reactive({
  theme_id: [{ required: true, message: '请选择概念/题材', trigger: 'change' }],
  max_stocks: [{ required: true, message: '请输入最大股票数', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择大模型', trigger: 'change' }],
});

// 打开 AI 选股 drawer
const openAiDrawer = () => {
  aiFormData.value = {
    ...createDefaultAiFormData(),
    theme_id: resolveRouteThemeId() ?? aiFormData.value.theme_id,
  };
  aiDrawerVisible.value = true;
};

// 关闭 AI 选股 drawer（关闭前记录本次选择，下次打开时回填）
const closeAiDrawer = () => {
  rememberAiSelection(aiFormData.value);
  aiDrawerVisible.value = false;
  aiFormData.value = createDefaultAiFormData();
};

// 跳转到 AI 执行记录页面查看任务进度
// 使用路由 name 跳转（GVA 动态路由 name 全局唯一，兼容标准版/简洁版菜单路径差异）
const goAiTaskPage = taskId => {
  if (!taskId) {
    getTableData();
    return;
  }
  router.push({ name: 'aiTask', query: { task_id: taskId } });
};

// 定时执行参数：开关开启且已选时间时返回时间字符串（RFC3339 格式，后端 *time.Time 直接解析），
// 否则返回 null（后端视为未指定，任务立即执行）
const buildScheduledAt = formData => {
  if (!formData.scheduled_enabled || !formData.scheduled_at) return null;
  return formData.scheduled_at;
};

// 禁用已过去的时间（定时执行不允许选择过去时刻）
const disabledScheduledDate = time => time.getTime() < Date.now();

// 提交 AI 选股（异步任务化：提交后立即返回任务ID，跳转执行进度页面查看进度）
const enterAiDrawer = async () => {
  // 定时执行校验：开启定时但未选择执行时间时提示
  if (aiFormData.value.scheduled_enabled && !aiFormData.value.scheduled_at) {
    ElMessage({ type: 'warning', message: '请选择定时执行时间' });
    return;
  }
  aiBtnLoading.value = true;
  aiFormRef.value?.validate(async valid => {
    if (!valid) return (aiBtnLoading.value = false);
    try {
      const res = await aiAddThemeStocks({
        ...aiFormData.value,
        scheduled_at: buildScheduledAt(aiFormData.value),
      });
      if (res.code === 0) {
        ElMessage({ type: 'success', message: '任务已创建，正在前往执行进度页' });
        closeAiDrawer();
        goAiTaskPage(res.data?.task_id);
      }
    } finally {
      aiBtnLoading.value = false;
    }
  });
};
// =========== AI 智能选股部分结束 ===========

// =========== AI 一键更新个股部分 ===========
// 当前正在 AI 更新的行 ID（用于行内按钮 loading）
const aiUpdatingId = ref(null);
// 是否正在批量 AI 更新（用于批量按钮 loading）
const aiBatchUpdating = ref(false);
// AI 更新 drawer 显示控制
const aiUpdateDrawerVisible = ref(false);
// AI 更新目标行（单只或批量勾选均存为数组）
const aiUpdateTargetRows = ref([]);
// AI 更新表单（厂商/模型，默认继承 AI 选股 drawer 的当前选择）
const aiUpdateFormData = ref({ provider: '', model: '', web_search: true, api_format: '', search_engine: 'baidu', scheduled_enabled: false, scheduled_at: getDefaultScheduledAt() });

// 是否为批量 AI 更新
const isAiBatchUpdate = computed(() => aiUpdateTargetRows.value.length > 1);

// AI 更新 drawer 标题（批量时附加数量）
const aiUpdateDrawerTitle = computed(() => {
  const suffix = isAiBatchUpdate.value ? `（${aiUpdateTargetRows.value.length}只）` : '';
  return `AI概念分析${suffix}`;
});

// AI 更新目标个股名称（单只显示股票名，批量显示数量）
const aiUpdateStockName = computed(() => {
  const rows = aiUpdateTargetRows.value;
  if (rows.length === 0) return '';
  if (rows.length === 1) return rows[0].stock?.name || rows[0].stock_id || '';
  return `已选 ${rows.length} 只股票`;
});

// 当前所选厂商的模型列表（AI 更新 drawer）
const currentAiUpdateModels = computed(() => {
  const provider = providerOptions.find(item => item.value === aiUpdateFormData.value.provider);
  return provider?.models || [];
});

// 切换厂商时自动带上该厂商第一个模型
const onAiUpdateProviderChange = () => {
  aiUpdateFormData.value.model = currentAiUpdateModels.value[0]?.value || '';
};

// 打开 AI 更新 drawer（单只/批量共用）：回填厂商、模型等参数（继承上一次 AI 选择）
const openAiUpdateDrawer = rows => {
  if (!rows || rows.length === 0) return;
  aiUpdateTargetRows.value = rows;
  aiUpdateFormData.value = applyAiSelection({
    provider: 'deepseek',
    model: 'deepseek-v4-flash',
    web_search: true, // 联网搜索默认开启
    api_format: 'responses', // 接口模式：responses=Responses API；chat-completions=OpenAI 兼容接口
    search_engine: 'baidu', // 联网搜索引擎（chat-completions 模式且开启联网搜索时生效）
    scheduled_enabled: false, // 定时执行开关
    scheduled_at: getDefaultScheduledAt(), // 默认非高峰时段（凌晨 03:00）
  });
  aiUpdateDrawerVisible.value = true;
};

// 点击行内 AI 按钮：弹出 drawer 手动指定大模型
const aiUpdateRow = row => {
  openAiUpdateDrawer([row]);
};

// 点击批量 AI 更新按钮：使用当前勾选的个股
const openAiBatchUpdate = () => {
  if (multipleSelection.value.length === 0) {
    ElMessage({ type: 'warning', message: '请先勾选要更新的个股' });
    return;
  }
  openAiUpdateDrawer([...multipleSelection.value]);
};

// 确认开始 AI 更新（单只走单只接口，批量走批量接口）
// 异步任务化：提交后立即返回任务ID，跳转执行进度页面查看进度
const submitAiUpdate = async () => {
  if (!aiUpdateFormData.value.provider || !aiUpdateFormData.value.model) {
    ElMessage({ type: 'warning', message: '请选择厂商和模型' });
    return;
  }
  // 定时执行校验：开启定时但未选择执行时间时提示
  if (aiUpdateFormData.value.scheduled_enabled && !aiUpdateFormData.value.scheduled_at) {
    ElMessage({ type: 'warning', message: '请选择定时执行时间' });
    return;
  }
  const rows = aiUpdateTargetRows.value;
  if (rows.length === 0) return;
  const isBatch = rows.length > 1;
  if (isBatch) {
    aiBatchUpdating.value = true;
  } else {
    aiUpdatingId.value = rows[0].id;
  }
  const commonParams = {
    provider: aiUpdateFormData.value.provider,
    model: aiUpdateFormData.value.model,
    web_search: aiUpdateFormData.value.web_search,
    api_format: aiUpdateFormData.value.api_format,
    search_engine: aiUpdateFormData.value.search_engine,
    scheduled_at: buildScheduledAt(aiUpdateFormData.value),
  };
  try {
    let res;
    if (isBatch) {
      res = await aiUpdateThemeStocks({ ids: rows.map(r => r.id), ...commonParams });
    } else {
      res = await aiUpdateThemeStock({ id: rows[0].id, ...commonParams });
    }
    if (res.code === 0) {
      ElMessage({ type: 'success', message: '任务已创建，正在前往执行进度页' });
      closeAiUpdateDrawer();
      goAiTaskPage(res.data?.task_id);
    }
  } finally {
    aiUpdatingId.value = null;
    aiBatchUpdating.value = false;
  }
};

// 取消 AI 更新：仅关闭 drawer（关闭前记录本次选择，下次打开时回填；任务提交后立即返回，无需中止请求）
const closeAiUpdateDrawer = () => {
  rememberAiSelection(aiUpdateFormData.value);
  aiUpdateDrawerVisible.value = false;
};
// =========== AI 一键更新个股部分结束 ===========

const getStatusType = status => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};

// 梯队 tag 颜色：1 红、2 黄、3 灰，0 及空值不显示 tag
const getTierType = tier => {
  if (tier === 1) return 'danger';
  if (tier === 2) return 'warning';
  if (tier === 3) return 'info';
  return '';
};

// 涨跌幅红涨绿跌样式类
const rateClass = val => {
  if (val === null || val === undefined || val === '') return '';
  const num = Number(val);
  if (Number.isNaN(num)) return '';
  if (num > 0) return 'rate-rise';
  if (num < 0) return 'rate-fall';
  return '';
};

// markdown 渲染（reason / ai_reason 由 Vditor 编辑器产生，存储为 markdown 格式）
const renderMarkdown = text => {
  if (!text) return '';
  // marked 默认单换行不换行，追加两个空格使换行生效（与 quant/update 页面保持一致）
  const formattedText = text.replace(/\n/g, '  \n');
  return marked.parse(formattedText);
};

// 相对天数文案：按自然日差计算，当天显示"今天"，其余显示"N天前"
const daysAgoText = time => {
  const now = new Date();
  now.setHours(0, 0, 0, 0);
  const date = new Date(time);
  date.setHours(0, 0, 0, 0);
  const days = Math.round((now - date) / 86400000);
  return days <= 0 ? '今天' : `${days}天前`;
};
</script>

<style>
/* =========== 页面内嵌详情视图样式（参考 baseStock 布局） =========== */
.theme-stock-detail .detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.theme-stock-detail .detail-card {
  margin-bottom: 12px;
}

/* 详情入选逻辑：完整段落展示 */
.reason-full {
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.8;
  font-size: 14px;
}

/* markdown 渲染样式（reason / ai_reason） */
.markdown-body h1,
.markdown-body h2,
.markdown-body h3,
.markdown-body h4,
.markdown-body h5,
.markdown-body h6 {
  margin: 0.8em 0 0.4em;
  font-weight: 600;
  line-height: 1.4;
}

.markdown-body h1 {
  font-size: 1.6em;
}

.markdown-body h2 {
  font-size: 1.4em;
}

.markdown-body h3 {
  font-size: 1.2em;
}

.markdown-body p {
  margin: 0;
}

.markdown-body ul,
.markdown-body ol {
  margin: 0.5em 0;
  padding-left: 2em;
}

.markdown-body ul {
  list-style-type: disc;
}

.markdown-body ol {
  list-style-type: decimal;
}

.markdown-body li {
  margin: 0.25em 0;
}

.markdown-body code {
  padding: 2px 4px;
  border-radius: 4px;
  background-color: rgba(0, 0, 0, 0.06);
  font-size: 0.9em;
}

.markdown-body pre {
  margin: 0.6em 0;
  padding: 12px;
  border-radius: 6px;
  background-color: rgba(0, 0, 0, 0.06);
  overflow-x: auto;
}

.markdown-body pre code {
  padding: 0;
  background: none;
}

.markdown-body blockquote {
  margin: 0.6em 0;
  padding: 4px 12px;
  border-left: 4px solid var(--el-border-color, #dcdfe6);
  color: var(--el-text-color-secondary);
}

.markdown-body a {
  color: var(--el-color-primary, #409eff);
}

.markdown-body img {
  max-width: 100%;
}

.markdown-body table {
  border-collapse: collapse;
  margin: 0.6em 0;
}

.markdown-body th,
.markdown-body td {
  border: 1px solid var(--el-border-color, #dcdfe6);
  padding: 6px 12px;
}

.markdown-body th {
  background-color: rgba(0, 0, 0, 0.04);
}

/* 详情入选逻辑头部更新时间 */
.reason-meta {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

/* 无 AI 分析结果时的提示更新区域 */
.empty-analyze {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 16px 0;
}

.empty-analyze-icon {
  font-size: 22px;
  color: var(--el-color-primary);
}

.empty-analyze-text {
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

/* 入选逻辑：保留换行段落，最多显示 2 行，超出省略 */
.reason-cell {
  white-space: pre-wrap;
  word-break: break-word;
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* 悬停提示：完整段落展示，超长可滚动 */
.reason-tooltip {
  white-space: pre-wrap;
  word-break: break-word;
  max-width: 600px;
  max-height: 320px;
  overflow-y: auto;
  line-height: 1.6;
  font-size: 14px;
}

/* 悬停提示底部：入选逻辑更新时间与大模型 */
.reason-tooltip-time {
  margin-bottom: 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
  color: rgba(255, 255, 255, 0.75);
  font-size: 13px;
  line-height: 1.6;
}

/* 涨跌幅：红涨绿跌 */
.rate-rise {
  color: #f56c6c;
}

.rate-fall {
  color: #67c23a;
}
</style>
