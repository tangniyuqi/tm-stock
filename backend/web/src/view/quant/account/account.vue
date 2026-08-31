<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="用户ID" prop="member_id">
            <el-input v-model.number="searchInfo.member_id" clearable placeholder="请输入用户ID" />
          </el-form-item>

        <el-form-item label="开户名" prop="name">
          <el-input v-model="searchInfo.name" clearable placeholder="请输入开户名关键词" />
        </el-form-item>

        <el-form-item label="券商" prop="broker">
          <el-input v-model="searchInfo.broker" clearable placeholder="请输入券商名称" />
        </el-form-item>

        <!-- <el-form-item label="资金账号" prop="account_no">
          <el-input v-model="searchInfo.account_no" clearable placeholder="请输入资金账号" />
        </el-form-item> -->

        <el-form-item label="到期时间" prop="expiration_day">
          <el-select v-model="searchInfo.expiration_day" placeholder="请选择到期时间范围">
            <el-option v-for="(item, key) in expirationDayOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <template v-if="showAllQuery">
          
        </template>

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
          <el-button link type="primary" icon="arrow-down" @click="showAllQuery = true" v-if="!showAllQuery">展开</el-button>
          <el-button link type="primary" icon="arrow-up" @click="showAllQuery = false" v-else>收起</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="gva-table-box">
      <div class="gva-btn-list">
        <el-button type="primary" icon="plus" @click="openDialog()">新增</el-button>
        <el-button icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id" @selection-change="handleSelectionChange">
        <el-table-column align="center" type="selection" width="50" />

        <el-table-column align="left" label="#" prop="id" width="70" />

        <el-table-column align="left" label="用户" min-width="180">
          <template #default="scope">
            <div v-if="scope.row.member" style="line-height: 1.5">
              <div>UID：{{ scope.row.member_id || '-' }}</div>
              <div class="font-bold">账号：{{ scope.row.member.userName || '-' }}</div>
              <div>昵称：{{ scope.row.member.nickName || '-' }}</div>
              <div>电话：{{ scope.row.member.phone || '-' }}</div>
            </div>
            <div v-else>-</div>
          </template>
        </el-table-column>

        <el-table-column align="left" label="证券账户" min-width="270">
          <template #default="scope">
            <div style="line-height: 1.5">
              <div class="font-bold">开户姓名：{{ scope.row.name || '-' }}</div>
              <div>开户券商：{{ scope.row.broker || '-' }}</div>
              <div>资金账号：{{ scope.row.account_no || '-' }}</div>
              <div>交易板块：{{ scope.row.markets.join(',') || '-' }}</div>
            </div>
          </template>
        </el-table-column>

        <!-- <el-table-column align="left" label="类型" prop="type" width="70" />   -->

        <el-table-column align="left" label="资金信息" min-width="220">
          <template #default="scope">
            <div v-if="scope.row.summary" style="line-height: 1.6;">
              <div>
                <span>总资产：</span>
                <span>￥{{ (scope.row.summary.total_asset || 0).toFixed(2) }}</span>
              </div>
              <div>
                <span>持仓市值：</span>
                <span>￥{{ (scope.row.summary.market_value || 0).toFixed(2) }}</span>
              </div>
              <div>
                <span >可用资金：</span>
                <span>￥{{ (scope.row.summary.available_balance || 0).toFixed(2) }}</span>
              </div>
              <div>
                <span>可取资金：</span>
                <span>￥{{ (scope.row.summary.withdrawable_cash || 0).toFixed(2) }}</span>
              </div>
              <div>
                <span>持仓盈亏：</span>
                <span :class="(scope.row.summary.total_pl_amount || 0) >= 0 ? 'text-red-500' : 'text-green-500'">
                  {{ (scope.row.summary.total_pl_amount || 0) >= 0 ? '+' : '' }}￥{{ (scope.row.summary.total_pl_amount || 0).toFixed(2) }}
                </span>
              </div>
              <div>
                <span>今日盈亏：</span>
                <span :class="(scope.row.summary.daily_pl_amount || 0) >= 0 ? 'text-red-500' : 'text-green-500'">
                  {{ (scope.row.summary.daily_pl_amount || 0) >= 0 ? '+' : '' }}￥{{ (scope.row.summary.daily_pl_amount || 0).toFixed(2) }}
                  <template v-if="scope.row.summary.total_asset && scope.row.summary.total_asset > 0">
                    {{ (scope.row.summary.daily_pl_amount || 0) >= 0 ? '+' : '' }}{{ ((scope.row.summary.daily_pl_amount || 0) / scope.row.summary.total_asset * 100).toFixed(2) }}%
                  </template>
                </span>
              </div>
              <div v-if="scope.row.summary.updated_at" class="text-gray-500">
                更新时间：{{ scope.row.summary.updated_at }}
              </div>
            </div>
            <div v-else class="text-gray-400">暂无数据</div>
          </template>
        </el-table-column>

        <el-table-column align="center" label="任务数" prop="max_running_task" width="70">
          <template #default="scope">
            <div class="text-gray-500 font-bold">{{ scope.row.max_running_task }} 个</div>
          </template>
        </el-table-column>

        <el-table-column align="center" label="到期日期" prop="expiration_date" min-width="150">
          <template #default="scope">
            <div :style="{ color: getExpirationColor(scope.row.expiration_date), lineHeight: '1.5' }">
              <div class="font-bold">{{ formatExpirationDateOnly(scope.row.expiration_date) }}</div>
              <div class="font-bold">{{ formatRemainingDays(scope.row.expiration_date) }}</div>
            </div>
          </template>
        </el-table-column>

        <el-table-column align="left" label="创建/更新时间" min-width="140">
          <template #default="scope">
            <div style="line-height: 1.5">
              <div class="text-gray-500">{{ formatToDateTime(scope.row.created_at, 'YYYY-MM-DD HH:mm') }}</div>
              <div class="text-gray-500">{{ formatToDateTime(scope.row.updated_at, 'YYYY-MM-DD HH:mm') }}</div>
            </div>
          </template>
        </el-table-column>

        <el-table-column align="left" label="状态" prop="status" width="60">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">{{ filterDict(String(scope.row.status), statusOptions) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column align="center" label="操作" fixed="right" :min-width="120">
          <template #default="scope">
            <div class="cell">
              <el-button type="primary" link @click="getDetails(scope.row)">查看</el-button>
              <el-button type="primary" link @click="updateAccountFunc(scope.row)">编辑</el-button>
              <!-- <el-button type="primary" link @click="deleteRow(scope.row)">删除</el-button> -->
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
      </div>
    </div>

    <el-drawer destroy-on-close size="65%" v-model="dialogFormVisible" :show-close="false" :before-close="closeDialog">
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
          <el-col :span="6">
            <el-form-item label="用户ID" prop="member_id">
              <el-input v-model.number="formData.member_id" clearable placeholder="请输入用户ID" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="账户类型" prop="type">
              <el-select v-model="formData.type" placeholder="请选择类型">
                <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="开户姓名" prop="name">
              <el-input v-model="formData.name" clearable placeholder="请输入姓名" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="开户券商" prop="broker">
              <el-select v-model="formData.broker" filterable clearable placeholder="请选择或输入券商名称">
                <el-option-group v-for="group in brokerOptions" :key="group.label" :label="group.label">
                  <el-option v-for="(item, index) in group.options" :key="index" :label="item" :value="item" />
                </el-option-group>
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="6">
            <el-form-item label="资金账号" prop="account_no">
              <el-input v-model="formData.account_no" clearable placeholder="请输入账号" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="交易密码" prop="passcode">
              <el-input v-model="formData.passcode" type="password" show-password clearable maxlength="6" placeholder="请输入交易密码" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="资金额度" prop="amount">
              <el-input-number v-model="formData.amount" style="width: 100%" :precision="2" clearable />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="到期日期" prop="expiration_date">
              <el-date-picker v-model="formData.expiration_date" type="date" style="width: 100%" placeholder="选择到期日期" clearable />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="支持市场" prop="markets">
              <el-select v-model="formData.markets" multiple placeholder="请选择支持的市场" style="width: 100%">
                <el-option v-for="(item, key) in marketOptions" :key="key" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="6">
            <el-form-item label="最大任务数" prop="max_task">
              <el-input-number v-model="formData.max_task" style="width: 100%" clearable />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="最大运行任务数" prop="max_running_task">
              <el-input-number v-model="formData.max_running_task" style="width: 100%" clearable />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="备注" prop="remark">
              <el-input v-model="formData.remark" clearable placeholder="请输入备注" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="状态" prop="status">
              <el-select v-model="formData.status" placeholder="请选择状态" style="width: 100%">
                <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">服务器配置</el-divider>

        <el-row :gutter="20">
          <el-col :span="6">
            <el-form-item label="模式" prop="server.mode">
              <el-select v-model="formData.server.mode" placeholder="请选择模式" style="width: 100%">
                <el-option v-for="(item, key) in serverModeOptions" :key="key" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="券商" prop="server.broker">
              <el-select v-model="formData.server.broker" placeholder="请选择模式" style="width: 100%">
                <el-option v-for="(item, key) in serverBrokerOptions" :key="key" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="客户端名称" prop="server.client_name">
              <el-input v-model="formData.server.client_name" clearable placeholder="请输入客户端名称" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="服务器名称" prop="server.name">
              <el-input v-model="formData.server.name" clearable placeholder="请输入服务器名称" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="客户端路径" prop="server.client_path">
              <el-input v-model="formData.server.client_path" clearable placeholder="请输入客户端路径" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="6">
            <el-form-item label="协议" prop="server.protocol">
              <el-select v-model="formData.server.protocol" placeholder="请选择协议" style="width: 100%">
                <el-option v-for="(item, key) in serverProtocolOptions" :key="key" :label="item" :value="item" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="主机" prop="server.host">
              <el-input v-model="formData.server.host" clearable placeholder="请输入主机" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="端口" prop="server.port">
              <el-input v-model="formData.server.port" clearable placeholder="请输入端口" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="Token" prop="server.token">
              <el-input v-model="formData.server.token" type="password" show-password clearable placeholder="请输入Token" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="6">
            <el-form-item label="数据平台" prop="server.data_platform">
              <el-select v-model="formData.server.data_platform" placeholder="请选择数据平台" style="width: 100%">
                <el-option v-for="(item, key) in serverDataPlatformOptions" :key="key" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="数据源" prop="server.data_source">
              <el-select v-model="formData.server.data_source" placeholder="请选择数据源" style="width: 100%">
                <el-option v-for="(item, key) in serverDataSourceOptions" :key="key" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="数据TOKEN" prop="server.data_token">
              <el-input v-model="formData.server.data_token" type="password" show-password clearable placeholder="请输入数据TOKEN" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="AI大模型配置">
              <div style="width: 100%">
                <div class="flex justify-between items-center mb-2">
                  <div class="text-sm text-gray-500" v-if="formData.server.ai_models?.length">
                    默认模型：{{ getAiModelDisplayName(formData.server.ai_models.find((m) => m.id === formData.server.ai_default_id)) }}
                  </div>
                  <div v-else class="text-sm text-gray-500">未配置</div>
                  <el-button type="primary" icon="plus" @click="addAiModelConfig">新增模型</el-button>
                </div>

                <el-table :data="formData.server.ai_models || []" border style="width: 100%" :row-key="(row) => row.id">
                  <el-table-column label="名称" min-width="140">
                    <template #default="{ row }">
                      <el-input v-model="row.name" clearable placeholder="例如：豆包/通义/DeepSeek" />
                    </template>
                  </el-table-column>
                  <el-table-column label="模型" min-width="160">
                    <template #default="{ row }">
                      <el-select v-model="row.model" filterable allow-create default-first-option clearable placeholder="请选择或输入模型" @change="(val) => handleAiModelSelectChange(row, val)">
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
                      <el-tag v-if="row.id === formData.server.ai_default_id" type="success" class="mr-2">默认</el-tag>
                      <el-button v-else type="primary" link @click="setDefaultAiModelConfig(row.id)">设为默认</el-button>
                      <el-button type="danger" link @click="removeAiModelConfig($index)">删除</el-button>
                    </template>
                  </el-table-column>
                </el-table>
              </div>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="6">
            <el-form-item label="Webhook类型" prop="server.webhook_type">
              <el-select v-model="formData.server.webhook_type" placeholder="请选择Webhook类型" style="width: 100%">
                <el-option v-for="(item, key) in serverWebhookTypeOptions" :key="key" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="18">
            <el-form-item label="Webhook地址" prop="server.webhook_url">
              <el-input v-model="formData.server.webhook_url" clearable placeholder="请输入Webhook地址" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="服务器备注" prop="server.remark">
              <el-input type="textarea" v-model="formData.server.remark" :rows="5" clearable placeholder="请输入服务器备注" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="用户ID">
          {{ detailFrom.member_id }}
        </el-descriptions-item>
        <el-descriptions-item label="类型">
          {{ detailFrom.type }}
        </el-descriptions-item>
        <el-descriptions-item label="称呼">
          {{ detailFrom.name }}
        </el-descriptions-item>
        <el-descriptions-item label="券商">
          {{ detailFrom.broker }}
        </el-descriptions-item>
        <el-descriptions-item label="账号">
          {{ detailFrom.account_no }}
        </el-descriptions-item>
        <el-descriptions-item label="密码">
          {{ detailFrom.passcode }}
        </el-descriptions-item>
        <el-descriptions-item label="支持市场">
          {{ detailFrom.markets }}
        </el-descriptions-item>
        <el-descriptions-item label="资金">
          {{ detailFrom.amount }}
        </el-descriptions-item>
        <el-descriptions-item label="最大任务数">
          {{ detailFrom.max_task }}
        </el-descriptions-item>
        <el-descriptions-item label="最大运行任务数">
          {{ detailFrom.max_running_task }}
        </el-descriptions-item>
        <el-descriptions-item label="备注">
          {{ detailFrom.remark }}
        </el-descriptions-item>
        <el-descriptions-item label="到期日期">
          {{ formatDate(detailFrom.expiration_date).split(' ')[0] }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ detailFrom.status }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue';
import { useAppStore, useUserStore } from '@/pinia';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import { createAccount, deleteAccount, deleteAccountByIds, updateAccount, findAccount, getAccountList } from '@/api/quant/account';
import { brokerOptions } from '@/data/brokerOptions';

defineOptions({
  name: 'Account'
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();
const userStore = useUserStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const statusOptions = ref();
const typeOptions = ref();
const marketOptions = ref();
const expirationDayOptions = ref();
const serverModeOptions = ref();
const serverBrokerOptions = ref();
const serverDataPlatformOptions = ref();
const serverDataSourceOptions = ref();
const serverWebhookTypeOptions = ref();
const serverAiModelOptions = ref();
const serverProtocolOptions = ref(['http', 'https']);

const genAiModelId = () => `${Date.now()}_${Math.random().toString(16).slice(2)}`;

const getAiModelDisplayName = (m) => {
  if (!m) return '-';
  return m.name || m.model || '-';
};

const hydrateAiModelUrlBackupFromOptions = (server) => {
  if (!server || !Array.isArray(server.ai_models) || server.ai_models.length === 0) return;
  const options = serverAiModelOptions.value || [];
  if (!options.length) return;

  server.ai_models.forEach((m) => {
    if (!m || typeof m !== 'object' || Array.isArray(m)) return;
    if (m.url) return;
    const matched = options.find((o) => o.value === m.model);
    if (matched && matched.extend !== undefined && matched.extend !== null) {
      m.url = matched.extend;
    }
  });
};

const normalizeServerForForm = (serverLike) => {
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
    mode: '',
    broker: '',
    client_name: '',
    client_path: 'C:\\同花顺软件\\同花顺',
    name: '',
    protocol: 'http',
    host: '',
    port: '',
    token: '',
    key: '',
    data_platform: 0,
    data_source: 0,
    data_token: '',
    ai_model: '',
    ai_key: '',
    ai_url: '',
    ai_models: [],
    ai_default_id: '',
    webhook_type: 0,
    webhook_url: '',
    remark: ''
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

  server.ai_models = server.ai_models.map((m) => {
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
      const matched = options.find((o) => o.value === model);
      if (matched && matched.extend !== undefined && matched.extend !== null) {
        url = matched.extend;
      }
    }
    return {
      id: item.id || genAiModelId(),
      name: item.name || '',
      model,
      key: item.key || item.ai_key || '',
      url
    };
  });

  if (!server.ai_models.length && (server.ai_model || server.ai_key)) {
    let url = '';
    if (server.ai_model) {
      const options = serverAiModelOptions.value || [];
      const matched = options.find((o) => o.value === server.ai_model);
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
        url
      }
    ];
  }

  if (server.ai_models.length && !server.ai_default_id) {
    server.ai_default_id = server.ai_models[0].id;
  }

  return server;
};

const syncLegacyAiFields = () => {
  const server = formData.value.server;
  if (!server || !Array.isArray(server.ai_models) || server.ai_models.length === 0) {
    if (server) {
      server.ai_model = '';
      server.ai_key = '';
    }
    return;
  }

  const selected = server.ai_models.find((m) => m.id === server.ai_default_id) || server.ai_models[0];
  server.ai_model = selected?.model || '';
  server.ai_key = selected?.key || '';
};

const handleAiModelSelectChange = (row, val) => {
  const options = serverAiModelOptions.value || [];
  const matched = options.find((o) => o.value === val);
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
  const server = formData.value.server;
  if (!server) return;
  if (!Array.isArray(server.ai_models)) server.ai_models = [];
  const id = genAiModelId();
  server.ai_models.push({ id, name: '', model: '', key: '', url: '' });
  if (!server.ai_default_id) server.ai_default_id = id;
  syncLegacyAiFields();
};

const setDefaultAiModelConfig = (id) => {
  const server = formData.value.server;
  if (!server) return;
  server.ai_default_id = id;
  syncLegacyAiFields();
};

const removeAiModelConfig = (index) => {
  const server = formData.value.server;
  if (!server || !Array.isArray(server.ai_models)) return;
  const removed = server.ai_models.splice(index, 1)[0];
  if (removed && removed.id && removed.id === server.ai_default_id) {
    server.ai_default_id = server.ai_models[0]?.id || '';
  }
  syncLegacyAiFields();
};

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  member_id: undefined,
  type: undefined,
  name: '',
  account_no: '',
  passcode: '',
  amount: 0,
  max_task: 5,
  max_running_task: 5,
  remark: '',
  expiration_date: undefined,
  status: undefined,
  markets: [],
  server: normalizeServerForForm({})
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
  elSearchFormRef.value?.validate(async (valid) => {
    if (!valid) return;
    page.value = 1;
    getTableData();
  });
};

// 分页
const handleSizeChange = (val) => {
  pageSize.value = val;
  getTableData();
};

// 修改页面容量
const handleCurrentChange = (val) => {
  page.value = val;
  getTableData();
};

// 查询
const getTableData = async () => {
  const table = await getAccountList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
  statusOptions.value = await getDictFunc('status');
  typeOptions.value = await getDictFunc('quant_account_type');
  marketOptions.value = await getDictFunc('quant_base_stock_market');
  expirationDayOptions.value = await getDictFunc('quant_account_expiration_day');
  serverModeOptions.value = await getDictFunc('quant_account_server_mode');
  serverBrokerOptions.value = await getDictFunc('quant_account_server_broker');
  serverDataPlatformOptions.value = await getDictFunc('quant_account_server_data_platform');
  serverDataSourceOptions.value = await getDictFunc('quant_account_server_data_source');
  serverWebhookTypeOptions.value = await getDictFunc('quant_account_server_webhook_type');
  serverAiModelOptions.value = await getDictFunc('quant_account_server_ai_model');
};

// 获取需要的字典 可能为空 按需保留
setOptions();

// 多选数据
const multipleSelection = ref([]);
// 多选
const handleSelectionChange = (val) => {
  multipleSelection.value = val;
};

// 删除行
const deleteRow = (row) => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    deleteAccountFunc(row);
  });
};

// 多选删除
const onDelete = async () => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    const ids = [];
    if (multipleSelection.value.length === 0) {
      ElMessage({
        type: 'warning',
        message: '请选择要删除的数据'
      });
      return;
    }
    multipleSelection.value &&
      multipleSelection.value.map((item) => {
        ids.push(item.id);
      });
    const res = await deleteAccountByIds({ ids });
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '删除成功'
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
const updateAccountFunc = async (row) => {
  const res = await findAccount({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    formData.value.server = normalizeServerForForm(formData.value.server);
    hydrateAiModelUrlBackupFromOptions(formData.value.server);
    syncLegacyAiFields();
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteAccountFunc = async (row) => {
  const res = await deleteAccount({ id: row.id });
  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '删除成功'
    });
    if (tableData.value.length === 1 && page.value > 1) {
      page.value--;
    }
    getTableData();
  }
};

// 弹窗控制标记
const dialogFormVisible = ref(false);

// 打开弹窗
const openDialog = () => {
  type.value = 'create';
  formData.value.member_id = userStore.userInfo.ID;
  dialogFormVisible.value = true;
};

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;
  formData.value = {
    member_id: undefined,
    config_id: undefined,
    type: undefined,
    name: '',
    account_no: '',
    passcode: '',
    amount: 0,
    max_task: 5,
    max_running_task: 5,
    remark: '',
    expiration_date: undefined,
    status: undefined,
    markets: [],
    server: normalizeServerForForm({})
  };
};
// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  elFormRef.value?.validate(async (valid) => {
    if (!valid) return (btnLoading.value = false);
    formData.value.server = normalizeServerForForm(formData.value.server);
    hydrateAiModelUrlBackupFromOptions(formData.value.server);
    syncLegacyAiFields();
    let res;
    switch (type.value) {
      case 'create':
        res = await createAccount(formData.value);
        break;
      case 'update':
        res = await updateAccount(formData.value);
        break;
      default:
        res = await createAccount(formData.value);
        break;
    }
    btnLoading.value = false;
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '创建/更改成功'
      });
      closeDialog();
      getTableData();
    }
  });
};

const detailFrom = ref({});

// 查看详情控制标记
const detailShow = ref(false);

// 打开详情弹窗
const openDetailShow = () => {
  detailShow.value = true;
};

// 打开详情
const getDetails = async (row) => {
  // 打开弹窗
  const res = await findAccount({ id: row.id });
  if (res.code === 0) {
    detailFrom.value = res.data;
    openDetailShow();
  }
};

// 关闭详情弹窗
const closeDetailShow = () => {
  detailShow.value = false;
  detailFrom.value = {};
};

const getStatusType = (status) => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};

// 计算到期日期剩余天数
const getRemainingDays = (expirationDate) => {
  if (!expirationDate) return null;
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const expDate = new Date(expirationDate);
  expDate.setHours(0, 0, 0, 0);
  const diffTime = expDate - today;
  const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
  return diffDays;
};

// 根据剩余天数返回颜色
const getExpirationColor = (expirationDate) => {
  const remainingDays = getRemainingDays(expirationDate);
  if (remainingDays === null) return '#606266';
  if (remainingDays < 0) return '#F56C6C'; // 已到期：红色
  if (remainingDays <= 7) return '#E6A23C'; // 0-7天：黄色
  return '#67C23A'; // 7天以上：绿色
};

// 格式化到期日期显示
const formatExpirationDate = (expirationDate) => {
  if (!expirationDate) return '-';
  const dateStr = formatToDateTime(expirationDate, 'YYYY-MM-DD');
  const remainingDays = getRemainingDays(expirationDate);
  if (remainingDays === null) return dateStr;
  if (remainingDays < 0) {
    return `${dateStr}（已到期）`;
  }
  return `${dateStr}（剩余：${remainingDays}天）`;
};

// 格式化到期日期（仅日期）
const formatExpirationDateOnly = (expirationDate) => {
  if (!expirationDate) return '-';
  return formatToDateTime(expirationDate, 'YYYY-MM-DD');
};

// 格式化剩余天数
const formatRemainingDays = (expirationDate) => {
  if (!expirationDate) return '';
  const remainingDays = getRemainingDays(expirationDate);
  if (remainingDays === null) return '';
  if (remainingDays < 0) {
    return '已到期';
  }
  return `剩余：${remainingDays}天`;
};
</script>

<style></style>
