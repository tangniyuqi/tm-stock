<template>
  <div>
    <!-- =========== 股票详情视图（页面内嵌，参考 aiTask 布局） =========== -->
    <div v-if="detailVisible" class="base-stock-detail">
      <div class="gva-btn-list my-3">
        <el-button icon="back" @click="backToList">返回列表</el-button>
        <el-button icon="refresh" @click="refreshDetail">刷新</el-button>
        <el-button v-auth="btnAuth.sync" type="success" icon="magic-stick"
          @click="aiAnalyzeRow(detailForm)">AI基本面分析</el-button>
        <el-button v-auth="btnAuth.delete" type="danger" icon="delete" @click="deleteRow(detailForm)">删除</el-button>
      </div>

      <el-card shadow="never" class="detail-card">
        <template #header>
          <div class="detail-header">
            <span class="font-bold">{{ detailForm.name || '-' }}（{{ detailForm.symbol || '-' }}）</span>
            <el-tag :type="detailForm.list_status === 'L' ? 'success' : 'info'">{{
              filterDict(String(detailForm.list_status || ''), listStatusOptions) || detailForm.list_status || '-'
            }}</el-tag>
          </div>
        </template>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="股票代码">{{ detailForm.symbol || '-' }}</el-descriptions-item>
          <el-descriptions-item label="股票名称">{{ detailForm.name || '-' }}</el-descriptions-item>
          <el-descriptions-item label="TS代码">{{ detailForm.ts_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="所属行业">{{ detailForm.industry || '-' }}</el-descriptions-item>
          <el-descriptions-item label="地域">{{ detailForm.area || '-' }}</el-descriptions-item>
          <el-descriptions-item label="市场类型">{{ detailForm.market || '-' }}</el-descriptions-item>
          <el-descriptions-item label="交易所代码">{{ detailForm.exchange || '-' }}</el-descriptions-item>
          <el-descriptions-item label="交易货币">{{ detailForm.curr_type || '-' }}</el-descriptions-item>
          <el-descriptions-item label="上市状态">{{ filterDict(String(detailForm.list_status || ''), listStatusOptions) ||
            detailForm.list_status || '-' }}</el-descriptions-item>
          <el-descriptions-item label="上市日期">{{ detailForm.list_date ? formatToDateTime(detailForm.list_date,
            'YYYY-MM-DD') :
            '-' }}</el-descriptions-item>
          <el-descriptions-item label="沪深港通">{{ detailForm.is_hs ?? detailForm.isHs ? filterDict(String(detailForm.is_hs
            ??
            detailForm.isHs), isHsOptions) : '-' }}</el-descriptions-item>
          <el-descriptions-item label="拼音缩写">{{ detailForm.cnspell || '-' }}</el-descriptions-item>
          <el-descriptions-item label="股票全称">{{ detailForm.fullname || '-' }}</el-descriptions-item>
          <el-descriptions-item label="英文全称">{{ detailForm.enname || '-' }}</el-descriptions-item>
          <el-descriptions-item label="实控人名称">{{ detailForm.act_name || '-' }}</el-descriptions-item>
          <el-descriptions-item label="实控人企业性质">{{ detailForm.act_ent_type || '-' }}</el-descriptions-item>
        </el-descriptions>
      </el-card>

      <el-card v-if="detailForm.ai_analyzed_at" shadow="never" class="detail-card-analysis">
        <template #header><span class="font-bold">基本面分析 · {{ formatDate(detailForm.ai_analyzed_at)
        }}</span></template>
        <el-tabs type="border-card">
          <el-tab-pane label="公司情况">
            <div class="whitespace-pre-wrap ai-analysis-text">{{ formatAiText(detailForm.fundamentals) || '-' }}</div>
          </el-tab-pane>
          <el-tab-pane label="财务分析">
            <div class="whitespace-pre-wrap ai-analysis-text">{{ formatAiText(detailForm.financial) || '-' }}</div>
          </el-tab-pane>
          <el-tab-pane label="落地&业绩兑现">
            <div class="whitespace-pre-wrap ai-analysis-text">{{ formatAiText(detailForm.realization) || '-' }}</div>
          </el-tab-pane>
          <el-tab-pane label="题材热度&动量">
            <div class="whitespace-pre-wrap ai-analysis-text">{{ formatAiText(detailForm.momentum) || '-' }}</div>
          </el-tab-pane>
          <el-tab-pane label="风险提示">
            <div class="whitespace-pre-wrap ai-analysis-text">{{ formatAiText(detailForm.risk) || '-' }}</div>
          </el-tab-pane>
        </el-tabs>
      </el-card>

      <!-- 无基本面分析结果时提示更新 -->
      <el-card v-else-if="detailForm.id" shadow="never" class="detail-card">
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
          <el-form-item label="TS代码" prop="name">
            <el-input v-model="searchInfo.ts_code" clearable placeholder="请输入TS代码" />
          </el-form-item>

          <el-form-item label="股票代码" prop="symbol">
            <el-input v-model="searchInfo.symbol" clearable placeholder="请输入股票代码" />
          </el-form-item>

          <el-form-item label="股票名称" prop="name">
            <el-input v-model="searchInfo.name" clearable placeholder="请输入股票名称" />
          </el-form-item>

          <el-form-item label="拼音缩写" prop="cnspell">
            <el-input v-model="searchInfo.cnspell" clearable placeholder="请输入拼音缩写" />
          </el-form-item>

          <el-form-item label="地域" prop="name">
            <el-input v-model="searchInfo.name" clearable placeholder="请输入地域" />
          </el-form-item>

          <el-form-item label="市场类型" prop="market">
            <el-select v-model="searchInfo.market" placeholder="请选择市场类型">
              <el-option v-for="(item, key) in marketOptions" :key="key" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>

          <el-form-item label="交易所" prop="exchange">
            <el-select v-model="searchInfo.exchange" placeholder="请选择交易所">
              <el-option v-for="(item, key) in exchangeOptions" :key="key" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>

          <el-form-item label="沪深港通" prop="is_hs">
            <el-select v-model="searchInfo.is_hs" placeholder="请选择沪深港通">
              <el-option v-for="(item, key) in isHsOptions" :key="key" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>

          <el-form-item label="上市状态" prop="list_status">
            <el-select v-model="searchInfo.list_status" placeholder="请选择市场类型">
              <el-option v-for="(item, key) in listStatusOptions" :key="key" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>

          <template v-if="showAllQuery"> </template>

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
        <div class="gva-table-box-head">
          <div class="gva-btn-list">
            <el-button v-auth="btnAuth.add" type="primary" icon="plus" @click="openDialog()">新增</el-button>
            <el-button v-auth="btnAuth.batchDelete" icon="delete" style="margin-left: 10px"
              :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
            <el-button v-auth="btnAuth.sync" type="success" icon="magic-stick"
              @click="openAiAnalyzeDrawer">AI基本面分析</el-button>
            <el-button v-auth="btnAuth.sync" type="success" plain icon="trend-charts" :loading="changePctLoading"
              @click="updateChangePct">一键更新涨跌幅</el-button>
            <el-button v-auth="btnAuth.sync" type="primary" icon="refresh" @click="syncData">同步股票池</el-button>
            <el-button v-auth="btnAuth.clear" type="warning" icon="delete" @click="clearData">清除全部</el-button>
          </div>

          <div class="gva-pagination">
            <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize"
              :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange"
              @size-change="handleSizeChange" />
          </div>
        </div>

        <el-table ref="multipleTable" show-overflow-tooltip tooltip-effect="dark" :data="tableData" row-key="id"
          style="width: 100%" @selection-change="handleSelectionChange">
          <el-table-column type="selection" align="center" width="55" />

          <el-table-column align="left" label="ID" prop="id" width="90" />

          <el-table-column align="left" label="股票代码" prop="symbol" width="90">
            <template #default="scope">
              <span class="link-type">{{ scope.row.symbol }}</span>
            </template>
          </el-table-column>

          <el-table-column align="left" label="股票名称" prop="name" min-width="120">
            <template #default="scope">
              <el-text tag="strong" class="link-type" @click="getDetails(scope.row)">
                {{ scope.row.name }}
                <el-tooltip v-if="scope.row.ai_analyzed_at" :content="`AI分析完成于 ${formatDate(scope.row.ai_analyzed_at)}`"
                  placement="top">
                  <el-icon class="ml-1 ai-badge-icon">
                    <MagicStick />
                  </el-icon>
                </el-tooltip>
              </el-text>
            </template>
          </el-table-column>

          <el-table-column align="left" label="涨跌幅" prop="change_pct" width="120">
            <template #default="scope">
              <span v-if="scope.row.change_pct !== null && scope.row.change_pct !== undefined"
                :class="rateClass(scope.row.change_pct)">{{ Number(scope.row.change_pct).toFixed(2) }}%</span>
              <span v-else>-</span>
            </template>
          </el-table-column>

          <el-table-column align="left" label="TS代码" prop="ts_code" width="100">
            <template #default="scope">
              <span class="link-type">{{ scope.row.ts_code }}</span>
            </template>
          </el-table-column>

          <el-table-column align="center" label="市场类型" prop="market" width="130" />

          <el-table-column align="left" label="所属行业" prop="industry" width="110" />

          <el-table-column align="left" label="地域" prop="area" width="90" />

          <el-table-column align="left" label="沪/深港通" prop="is_hs" width="90">
            <template #default="scope">
              {{ scope.row.is_hs != 'N' ? filterDict(String(scope.row.is_hs), isHsOptions) : '-' }}
            </template>
          </el-table-column>

          <!-- <el-table-column align="center" label="股票全称" prop="fullname" width="120" />

        <el-table-column align="center" label="英文全称" prop="enname" width="120" />

        <el-table-column align="center" label="拼音缩写" prop="cnspell" width="110" />

        <el-table-column align="center" label="交易所代码" prop="exchange" width="110" />

        <el-table-column align="center" label="交易货币" prop="curr_type" width="90" />

        <el-table-column align="center" label="上市状态" prop="list_status" width="100">
          <template #default="scope">
            <el-tag type="primary" effect="plain">
              {{ filterDict(String(scope.row.list_status), listStatusOptions) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="实控人" prop="act_name" width="150" /> -->

          <el-table-column align="left" label="实控企业性质" prop="act_ent_type" width="120" />

          <el-table-column align="left" label="上市日期" prop="list_date" width="120">
            <template #default="scope">{{ formatToDateTime(scope.row.list_date, 'YYYY-MM-DD') }}</template>
          </el-table-column>

          <el-table-column align="left" label="操作" fixed="right" :min-width="120"
            v-if="btnAuth.info || btnAuth.delete || btnAuth.sync">
            <template #default="scope">
              <el-button v-auth="btnAuth.info" type="primary" link @click="getDetails(scope.row)">查看</el-button>
              <el-button v-auth="btnAuth.delete" type="primary" link @click="deleteRow(scope.row)">删除</el-button>
              <el-button v-auth="btnAuth.sync" type="success" link @click="aiAnalyzeRow(scope.row)">AI分析</el-button>
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

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="dialogFormVisible" :show-close="false"
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
        <el-form-item label="TS代码" prop="ts_code">
          <el-input v-model="formData.ts_code" :clearable="true" placeholder="请输入TS代码" />
        </el-form-item>
        <el-form-item label="股票代码" prop="symbol">
          <el-input v-model="formData.symbol" :clearable="true" placeholder="请输入股票代码" />
        </el-form-item>
        <el-form-item label="股票名称" prop="name">
          <el-input v-model="formData.name" :clearable="true" placeholder="请输入股票名称" />
        </el-form-item>
        <el-form-item label="地域" prop="area">
          <el-input v-model="formData.area" :clearable="true" placeholder="请输入地域" />
        </el-form-item>
        <el-form-item label="所属行业" prop="industry">
          <el-input v-model="formData.industry" :clearable="true" placeholder="请输入所属行业" />
        </el-form-item>
        <el-form-item label="股票全称" prop="fullname">
          <el-input v-model="formData.fullname" :clearable="true" placeholder="请输入股票全称" />
        </el-form-item>
        <el-form-item label="英文全称" prop="enname">
          <el-input v-model="formData.enname" :clearable="true" placeholder="请输入英文全称" />
        </el-form-item>
        <el-form-item label="拼音缩写" prop="cnspell">
          <el-input v-model="formData.cnspell" :clearable="true" placeholder="请输入拼音缩写" />
        </el-form-item>
        <el-form-item label="市场类型" prop="market">
          <el-input v-model="formData.market" :clearable="true" placeholder="请输入市场类型" />
        </el-form-item>
        <el-form-item label="交易所代码" prop="exchange">
          <el-input v-model="formData.exchange" :clearable="true" placeholder="请输入交易所代码" />
        </el-form-item>
        <el-form-item label="交易货币" prop="curr_type">
          <el-input v-model="formData.curr_type" :clearable="true" placeholder="请输入交易货币" />
        </el-form-item>
        <el-form-item label="上市状态" prop="list_status">
          <el-input v-model="formData.list_status" :clearable="true" placeholder="请输入上市状态" />
        </el-form-item>
        <el-form-item label="上市日期" prop="list_date">
          <el-date-picker v-model="formData.list_date" type="date" style="width: 100%" placeholder="选择日期"
            :clearable="true" />
        </el-form-item>
        <el-form-item label="退市日期" prop="delist_date">
          <el-date-picker v-model="formData.delist_date" type="date" style="width: 100%" placeholder="选择日期"
            :clearable="true" />
        </el-form-item>
        <el-form-item label="沪深港通" prop="isHs">
          <el-input v-model="formData.isHs" :clearable="true" placeholder="请输入沪深港通" />
        </el-form-item>
        <el-form-item label="实控人名称" prop="act_name">
          <el-input v-model="formData.act_name" :clearable="true" placeholder="请输入实控人名称" />
        </el-form-item>
        <el-form-item label="实控人企业性质" prop="act_ent_type">
          <el-input v-model="formData.act_ent_type" :clearable="true" placeholder="请输入实控人企业性质" />
        </el-form-item>
      </el-form>
    </el-drawer>

    <!-- AI 自动分析 Drawer -->
    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="aiDrawerVisible" :show-close="false"
      :before-close="closeAiAnalyzeDrawer">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">AI基本面分析</span>
          <div>
            <el-button :loading="aiBtnLoading" type="success" @click="enterAiAnalyze">开始分析</el-button>
            <el-button @click="closeAiAnalyzeDrawer">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form :model="aiFormData" label-position="top" ref="aiFormRef" :rules="aiRule" label-width="80px">
        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="分析范围">
              <el-radio-group v-model="aiAnalyzeType">
                <el-radio-button value="batch">批量分析（表格已选 {{ multipleSelection.length }} 只）</el-radio-button>
                <el-radio-button value="custom">指定个股</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>

          <el-col v-if="aiAnalyzeType === 'custom'" :span="24">
            <el-form-item label="选择个股">
              <el-select v-model="aiFormData.custom_stock_ids" multiple filterable remote reserve-keyword
                placeholder="输入股票名称/代码搜索" :remote-method="remoteSearchStocks" :loading="aiStockLoading"
                style="width: 100%">
                <el-option v-for="item in aiStockOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col v-else :span="24">
            <el-form-item label="批量分析范围">
              <el-alert v-if="multipleSelection.length" :title="`将分析表格中已勾选的 ${multipleSelection.length} 只股票`"
                type="info" :closable="false" show-icon />
              <el-alert v-else title="当前未勾选股票，请先在表格中勾选" type="warning" :closable="false" show-icon />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="AI提供商" prop="provider">
              <el-select v-model="aiFormData.provider" style="width: 100%" placeholder="请选择提供商"
                @change="onProviderChange">
                <el-option v-for="item in providerOptions" :key="item.value" :label="item.label" :value="item.value"
                  :disabled="item.disabled" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="模型" prop="model">
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

          <el-col :span="24"></el-col>

          <el-col :span="4">
            <el-form-item label="定时执行">
              <div class="flex items-center">
                <el-switch v-model="aiFormData.scheduled_enabled" :active-value="true" :inactive-value="false"
                  @change="val => onScheduledEnabledChange(aiFormData, val)" />
              </div>
            </el-form-item>
          </el-col>

          <el-col v-if="aiFormData.scheduled_enabled" :span="8">
            <el-form-item label="执行时间">
              <el-date-picker v-model="aiFormData.scheduled_at" type="datetime" style="width: 100%" placeholder="选择执行时间"
                :disabled-date="disabledScheduledDate" value-format="YYYY-MM-DDTHH:mm:ssZ" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive, computed } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import { createBaseStock, deleteBaseStock, deleteBaseStockByIds, updateBaseStock, findBaseStock, getBaseStockList, sync, clear, aiAnalyzeStocks, updateAllChangePct } from '@/api/quant/baseStock';
import { providerOptions } from '@/data/aiProviderOptions';
import { webSearchOptions } from '@/data/webSearchOptions';

defineOptions({
  name: 'BaseStock',
});

// 按钮权限实例化
const btnAuth = useBtnAuth();

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();
const router = useRouter();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const marketOptions = ref();
const listStatusOptions = ref();
const exchangeOptions = ref();
const isHsOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  ts_code: '',
  symbol: '',
  name: '',
  area: '',
  industry: '',
  fullname: '',
  enname: '',
  cnspell: '',
  market: '',
  exchange: '',
  curr_type: '',
  list_status: '',
  list_date: new Date(),
  delist_date: new Date(),
  isHs: '',
  act_name: '',
  act_ent_type: '',
});

// 验证规则
const rule = reactive({});

const elFormRef = ref();
const elSearchFormRef = ref();

// =========== 表格控制部分 ===========
const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
const tableData = ref([]);
const searchInfo = ref({});
// 重置
const onReset = () => {
  searchInfo.value = {};
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
  const table = await getBaseStockList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
  marketOptions.value = await getDictFunc('quant_base_stock_market');
  listStatusOptions.value = await getDictFunc('quant_base_stock_list_status');
  exchangeOptions.value = await getDictFunc('quant_base_stock_exchange');
  isHsOptions.value = await getDictFunc('quant_base_stock_is_hs');
};

// 获取需要的字典 可能为空 按需保留
setOptions();

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
    deleteBaseStockFunc(row);
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
    const res = await deleteBaseStockByIds({ ids });
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
const updateBaseStockFunc = async row => {
  const res = await findBaseStock({ id: row.id });
  type.value = 'update';

  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteBaseStockFunc = async row => {
  const res = await deleteBaseStock({ id: row.id });

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

// 同步（增量更新：已存在的股票更新基础信息，新增的股票插入，保留本地行情与AI分析结果）
const syncData = async () => {
  ElMessageBox.confirm('确定要增量同步股票数据吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(async () => {
    const res = await sync();
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '同步成功',
      });

      getTableData();
    }
  });
};

// 一键更新全部涨跌幅（通过 Tushare 获取最近交易日行情）
const changePctLoading = ref(false);
const updateChangePct = async () => {
  ElMessageBox.confirm('确定要更新全部股票涨跌幅吗? 将覆盖现有涨跌幅数据。', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(async () => {
    changePctLoading.value = true;
    try {
      const res = await updateAllChangePct();
      if (res.code === 0) {
        ElMessage({
          type: 'success',
          message: res.msg || '更新成功',
        });
        getTableData();
      }
    } catch (e) {
      // 统一错误提示由全局拦截器处理
      throw e;
    } finally {
      changePctLoading.value = false;
    }
  });
};

// 清除全部
const clearData = async () => {
  ElMessageBox.confirm('确定要清除全部数据吗? 此操作不可恢复!', '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(async () => {
    const res = await clear();
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '清除成功',
      });

      getTableData();
    }
  });
};

// 弹窗控制标记
const dialogFormVisible = ref(false);

// 打开弹窗
const openDialog = () => {
  type.value = 'create';
  dialogFormVisible.value = true;
};

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;
  formData.value = {
    ts_code: '',
    symbol: '',
    name: '',
    area: '',
    industry: '',
    fullname: '',
    enname: '',
    cnspell: '',
    market: '',
    exchange: '',
    curr_type: '',
    list_status: '',
    list_date: new Date(),
    delist_date: new Date(),
    isHs: '',
    act_name: '',
    act_ent_type: '',
  };
};
// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  elFormRef.value?.validate(async valid => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createBaseStock(formData.value);
        break;
      case 'update':
        res = await updateBaseStock(formData.value);
        break;
      default:
        res = await createBaseStock(formData.value);
        break;
    }
    btnLoading.value = false;
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '创建/更改成功',
      });
      closeDialog();
      getTableData();
    }
  });
};

const detailForm = ref({});

// 解析 AI 分析文本中的换行：兼容数据库中字面量 \n（反斜杠+n）与真实换行符，方便阅读
const formatAiText = text => {
  if (!text) return '';
  return String(text).replace(/\\n/g, '\n');
};

// 页面内嵌详情视图控制标记（参考 aiTask 布局）
const detailVisible = ref(false);

// 打开详情（页面内嵌视图）
const getDetails = async row => {
  const res = await findBaseStock({ id: row.id });
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

// 刷新详情数据（重新拉取最新数据，如 AI 分析完成后的最新结果）
const refreshDetail = async () => {
  if (!detailForm.value.id) return;
  const res = await findBaseStock({ id: detailForm.value.id });
  if (res.code === 0) {
    detailForm.value = res.data;
    ElMessage({ type: 'success', message: '刷新成功' });
  }
};

// =========== AI 参数选择记忆（AI供应商/大模型/接口模式/联网搜索/搜索引擎） ===========
// 记录用户上一次在 AI drawer 中选择的 AI 参数，下次打开时回填；
// 持久化到 localStorage，刷新页面后依然生效
const AI_SELECTION_STORAGE_KEY = 'baseStock_aiSelection';

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

// =========== AI 自动分析部分 ===========
// AI 分析 drawer 显示控制
const aiDrawerVisible = ref(false);
// AI 分析提交按钮 loading
const aiBtnLoading = ref(false);
const aiFormRef = ref();
// 分析方式: batch 批量分析(custom 指定个股
const aiAnalyzeType = ref('batch');
// 指定个股候选列表
const aiStockOptions = ref([]);
const aiStockLoading = ref(false);

// 大模型厂商及模型选项，已抽离至 src/data/aiProviderOptions.js
const currentProviderModels = computed(() => {
  const provider = providerOptions.find(item => item.value === aiFormData.value.provider);
  return provider?.models || [];
});

// 切换厂商时自动带上该厂商第一个模型
const onProviderChange = () => {
  aiFormData.value.model = currentProviderModels.value[0]?.value || '';
};

// 获取默认定时执行时间（下一个空闲时段，避开高峰 9:00-12:00、14:00-18:00）
const getDefaultScheduledAt = () => {
  // 北京时间 = UTC + 8 小时（无夏令时），加上 10 分钟缓冲
  const bjMs = Date.now() + 8 * 3600000 + 10 * 60000;
  const d = new Date(bjMs);
  let h = d.getUTCHours();
  let m = d.getUTCMinutes();
  let s = d.getUTCSeconds();
  if ((h >= 9 && h < 12) || (h >= 14 && h < 18)) {
    if (h < 12) { h = 12; m = 0; s = 0; }
    else { h = 18; m = 0; s = 0; }
  }
  const pad = n => String(n).padStart(2, '0');
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}T${pad(h)}:${pad(m)}:${pad(s)}+08:00`;
};

// 定时开关切换：开启时设默认时间，关闭时清空
const onScheduledEnabledChange = (formData, enabled) => {
  if (enabled) formData.scheduled_at = getDefaultScheduledAt();
  else formData.scheduled_at = null;
};

// 构建定时参数：未开启定时或未选时间时返回 null
const buildScheduledAt = (formData) => {
  if (!formData.scheduled_enabled || !formData.scheduled_at) return null;
  return formData.scheduled_at;
};

// 禁用已过去的时间
const disabledScheduledDate = time => time.getTime() < Date.now();

const createDefaultAiFormData = () => applyAiSelection({
  provider: 'deepseek',
  model: 'deepseek-v4-flash',
  custom_stock_ids: [],
  web_search: true, // true=默认开启联网搜索；false=强制关闭；不传(undefined)=跟随模型配置
  api_format: 'responses', // 接口模式：''=跟随配置；responses=Responses API；chat-completions=OpenAI 兼容接口
  search_engine: 'baidu', // 联网搜索引擎（chat-completions 模式且开启联网搜索时生效；默认百度搜索）
  scheduled_enabled: false, // 定时执行开关
  scheduled_at: null, // 未开启定时时为空，避免提交时被后端当作定时任务；开启定时时由 onScheduledEnabledChange 填入默认空闲时段
});

const aiFormData = ref(createDefaultAiFormData());

// AI 分析表单验证规则
const aiRule = reactive({
  provider: [{ required: true, message: '请选择大模型', trigger: 'change' }],
});

// 打开 AI 自动分析 drawer
const openAiAnalyzeDrawer = () => {
  aiAnalyzeType.value = multipleSelection.value.length ? 'batch' : 'custom';
  aiFormData.value = createDefaultAiFormData();
  aiStockOptions.value = [];
  aiDrawerVisible.value = true;
};

// 单行 AI 分析：打开 drawer 并自动带入该股票
const aiAnalyzeRow = row => {
  aiAnalyzeType.value = 'custom';
  aiFormData.value = createDefaultAiFormData();
  aiFormData.value.custom_stock_ids = [row.id];
  aiStockOptions.value = [{ value: row.id, label: `${row.name}(${row.symbol})` }];
  aiDrawerVisible.value = true;
};

// 关闭 AI 自动分析 drawer（关闭前记录本次选择，下次打开时回填）
const closeAiAnalyzeDrawer = () => {
  rememberAiSelection(aiFormData.value);
  aiDrawerVisible.value = false;
};

// 涨跌幅颜色：红涨绿跌
const rateClass = val => {
  if (val === null || val === undefined || val === '') return '';
  const num = Number(val);
  if (Number.isNaN(num)) return '';
  if (num > 0) return 'rate-rise';
  if (num < 0) return 'rate-fall';
  return '';
};

// 跳转到 AI 执行记录页面查看任务进度（路由 name 全局唯一，兼容标准版/简洁版菜单路径差异）
const goAiTaskPage = taskId => {
  if (!taskId) {
    getTableData();
    return;
  }
  router.push({ name: 'aiTask', query: { task_id: taskId } });
};

// 远程搜索指定个股
const remoteSearchStocks = async query => {
  if (!query) {
    aiStockOptions.value = [];
    return;
  }
  aiStockLoading.value = true;
  try {
    const res = await getBaseStockList({ page: 1, pageSize: 20, q: query });
    if (res.code === 0) {
      aiStockOptions.value = res.data.list.map(item => ({
        value: item.id,
        label: `${item.name}(${item.symbol})`,
      }));
    }
  } finally {
    aiStockLoading.value = false;
  }
};

// 提交 AI 自动分析
const enterAiAnalyze = async () => {
  if (aiAnalyzeType.value === 'batch' && !multipleSelection.value.length) {
    ElMessage({
      type: 'warning',
      message: '请先在表格中勾选需要分析的股票',
    });
    return;
  }
  if (aiAnalyzeType.value === 'custom' && !aiFormData.value.custom_stock_ids.length) {
    ElMessage({
      type: 'warning',
      message: '请先选择需要分析的个股',
    });
    return;
  }
  // 定时执行校验：开启定时但未选时间
  if (aiFormData.value.scheduled_enabled && !aiFormData.value.scheduled_at) {
    ElMessage({ type: 'warning', message: '请选择定时执行时间' });
    return;
  }
  aiBtnLoading.value = true;
  aiFormRef.value?.validate(async valid => {
    if (!valid) return (aiBtnLoading.value = false);
    try {
      const stockIds = aiAnalyzeType.value === 'batch' ? multipleSelection.value.map(item => item.id) : aiFormData.value.custom_stock_ids;
      const res = await aiAnalyzeStocks({
        stock_ids: stockIds,
        provider: aiFormData.value.provider,
        model: aiFormData.value.model,
        web_search: aiFormData.value.web_search,
        api_format: aiFormData.value.api_format,
        search_engine: aiFormData.value.search_engine,
        scheduled_at: buildScheduledAt(aiFormData.value),
      });
      if (res.code === 0) {
        ElMessage({
          type: 'success',
          message: '任务已创建，正在前往执行进度页',
        });
        closeAiAnalyzeDrawer();
        goAiTaskPage(res.data?.task_id);
      }
    } catch (e) {
      // 统一错误提示由全局拦截器处理
      throw e;
    } finally {
      aiBtnLoading.value = false;
    }
  });
};
// =========== AI 自动分析部分结束 ===========
</script>

<style>
/* =========== 页面内嵌详情视图样式（参考 aiTask 布局） =========== */
.base-stock-detail .detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.base-stock-detail .detail-card {
  margin-bottom: 12px;
}

.base-stock-detail .detail-card-analysis {
  min-height: 800px;
  margin-bottom: 12px;
}

.gva-table-box-head {
  display: flex;
  justify-content: space-between;

  .el-pagination {
    margin-top: 1em;
    margin-bottom: 2em;
  }
}

.ai-badge-icon {
  color: var(--el-color-primary);
  font-size: 14px;
  vertical-align: middle;
  cursor: pointer;
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

/* AI 分析文本：保留换行、优化阅读 */
.ai-analysis-text {
  line-height: 1.8;
  word-break: break-word;
}

/* 涨跌幅：红涨绿跌 */
.rate-rise {
  color: #f56c6c;
}

.rate-fall {
  color: #67c23a;
}
</style>
