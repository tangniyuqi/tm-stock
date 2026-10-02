<template>
  <div>
    <!-- =========== 题材股票详情视图（页面内嵌，参考 baseStock 布局） =========== -->
    <div v-if="detailVisible" class="theme-stock-detail">
      <div class="gva-btn-list my-3">
        <el-button icon="back" @click="backToList">返回列表</el-button>
        <el-button icon="refresh" @click="getDetails({ id: detailForm.id })">刷新</el-button>
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

          <el-descriptions-item label="依据类型">{{ sourceTypeLabel(detailForm.source_type) }}</el-descriptions-item>
          <el-descriptions-item label="采集时点">{{ detailForm.collected_at ? formatToDateTime(detailForm.collected_at,
            'YYYY-MM-DD HH:mm') : '-' }}</el-descriptions-item>
          <el-descriptions-item label="审核状态">
            <el-tag :type="auditTagType(detailForm.audit_status)">{{ auditStatusLabel(detailForm.audit_status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="审核时间">{{ detailForm.audit_at ? formatToDateTime(detailForm.audit_at,
            'YYYY-MM-DD HH:mm') : '-' }}</el-descriptions-item>
          <el-descriptions-item v-if="detailForm.audit_status === 3" label="驳回原因" :span="2">{{ detailForm.reject_reason
            || '-' }}</el-descriptions-item>
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

      <!-- 归属依据：这家公司凭什么归到这个题材（客观事实，可溯源）；依据缺失时禁止入库 -->
      <el-card shadow="never" class="detail-card">
        <template #header>
          <div class="detail-header">
            <span class="font-bold">归属依据</span>
            <span v-if="detailForm.collected_at" class="reason-meta">采集于 {{ formatToDateTime(detailForm.collected_at,
              'YYYY-MM-DD HH:mm') }}（{{ daysAgoText(detailForm.collected_at) }}）</span>
          </div>
        </template>

        <div class="reason-full">{{ detailForm.source_excerpt || '-' }}</div>

        <div v-if="detailForm.source_url" class="mt-2">
          <el-link :href="detailForm.source_url" target="_blank" rel="noopener noreferrer" type="primary">查看原文：{{
            detailForm.source_url }}</el-link>
        </div>

        <div class="reason-meta mt-2">只有审核“已通过”的关联才会对 C 端可见；修改所属题材、股票或任一项依据后，原审核结论作废，需重新审核。</div>
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

          <el-form-item label="审核状态" prop="audit_status">
            <el-select v-model="searchInfo.audit_status" clearable placeholder="请选择审核状态" style="width: 160px">
              <el-option v-for="item in auditStatusOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>

          <template v-if="showAllQuery">
            <el-form-item label="依据类型" prop="source_type">
              <el-select v-model="searchInfo.source_type" clearable placeholder="请选择依据类型" style="width: 180px">
                <el-option v-for="item in sourceTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>

            <el-form-item label="摘录关键词" prop="source_excerpt">
              <el-input v-model="searchInfo.source_excerpt" :clearable="true" placeholder="请输入摘录中的关键词" />
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

          <el-table-column align="left" label="依据类型" prop="source_type" width="110">
            <template #default="scope">{{ sourceTypeLabel(scope.row.source_type) }}</template>
          </el-table-column>

          <el-table-column align="left" label="原文摘录" min-width="260">
            <template #default="scope">
              <el-tooltip v-if="scope.row.source_excerpt" placement="top" :show-after="300">
                <template #content>
                  <div class="reason-tooltip">{{ scope.row.source_excerpt }}</div>
                </template>
                <div class="reason-cell">{{ scope.row.source_excerpt }}</div>
              </el-tooltip>
              <span v-else>-</span>
            </template>
          </el-table-column>

          <el-table-column sortable="custom" align="center" label="审核状态" prop="audit_status" width="110">
            <template #default="scope">
              <el-tag :type="auditTagType(scope.row.audit_status)">{{ auditStatusLabel(scope.row.audit_status) }}</el-tag>
            </template>
          </el-table-column>

          <el-table-column sortable="custom" align="left" label="采集时点" prop="collected_at" width="140">
            <template #default="scope">{{ scope.row.collected_at ? formatToDateTime(scope.row.collected_at,
              'YYYY-MM-DD HH:mm') : '-' }}</template>
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

          <el-col :span="12">
            <el-form-item label="依据类型" prop="source_type">
              <el-select v-model="formData.source_type" placeholder="请选择依据类型" style="width: 100%">
                <el-option v-for="item in sourceTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="采集时点" prop="collected_at">
              <el-date-picker v-model="formData.collected_at" type="datetime" style="width: 100%"
                placeholder="选择采集时点" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="原文摘录" prop="source_excerpt">
              <el-input v-model="formData.source_excerpt" type="textarea" :autosize="{ minRows: 4, maxRows: 10 }"
                maxlength="1000" show-word-limit
                placeholder="粘贴原文中支撑该归属的那一句话（客观事实）。不得填“见链接”“详见公告”等占位，也不得含评价、预测或买卖类措辞" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="原文链接" prop="source_url">
              <el-input v-model="formData.source_url" :clearable="true" placeholder="https://……（公告、年报、招股书等原文地址）" />
            </el-form-item>
          </el-col>

          <el-col v-if="type === 'update'" :span="8">
            <el-form-item label="审核状态" prop="audit_status">
              <el-select v-model="formData.audit_status" placeholder="请选择审核状态" style="width: 100%">
                <el-option v-for="item in auditStatusOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col v-if="type === 'update' && formData.audit_status === 3" :span="16">
            <el-form-item label="驳回原因" prop="reject_reason">
              <el-input v-model="formData.reject_reason" :clearable="true" maxlength="250" show-word-limit
                placeholder="驳回必须填写原因" />
            </el-form-item>
          </el-col>

          <el-col v-if="type === 'update'" :span="24">
            <div class="reason-meta">提示：修改所属题材、股票或任一项依据后，原审核结论作废，状态会重置为草稿并需要重新审核；新建的关联一律是草稿；只有“已通过”的关联才会对 C 端可见。</div>
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
  </div>
</template>

<script setup>
import { ref, reactive, watch } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { useRoute } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { formatToDateTime } from '@/utils/dateTimeUtils';
import { getDictFunc, filterDict } from '@/utils/format';
import { createThemeStock, deleteThemeStock, deleteThemeStockByIds, updateThemeStock, getThemeStockList, findThemeStock } from '@/api/quant/themeStock';
import { getThemeList } from '@/api/quant/theme';
import { getBaseStockList, findBaseStock } from '@/api/quant/baseStock';
// 富文本组件
// import RichEdit from '@/components/richtext/rich-edit.vue';
import RichView from '@/components/richtext/rich-view.vue';
import ArrayCtrl from '@/components/arrayCtrl/arrayCtrl.vue';

defineOptions({
  name: 'ThemeStock',
});
// 按钮权限实例化
const btnAuth = useBtnAuth();

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();
const route = useRoute();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const statusOptions = ref([]);
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
  source_type: undefined,
  source_excerpt: '',
  source_url: '',
  collected_at: new Date(),
  audit_status: 0,
  reject_reason: '',
  sort: 0,
  status: 1,
});

const createDefaultSearchInfo = () => ({
  id: undefined,
  theme_id: resolveRouteThemeId(),
  stock_id: undefined,
  source_type: undefined,
  source_excerpt: undefined,
  audit_status: undefined,
  createdAtRange: undefined,
});

// 自动化生成的字典（可能为空）以及字段
const formData = ref(createDefaultFormData());

// 验证规则：依据四项缺一不可（与后端、数据库约束一致；后端还会拦占位文字与评价类措辞）
const rule = reactive({
  theme_id: [{ required: true, message: '请选择所属题材', trigger: 'change' }],
  stock_id: [{ required: true, message: '请选择股票', trigger: 'change' }],
  source_type: [{ required: true, message: '请选择依据类型', trigger: 'change' }],
  source_excerpt: [
    { required: true, message: '请填写原文摘录', trigger: 'blur' },
    { min: 8, message: '摘录至少 8 个字', trigger: 'blur' },
  ],
  source_url: [
    { required: true, message: '请填写原文链接', trigger: 'blur' },
    { pattern: /^https?:\/\/\S+$/, message: '链接必须是完整的 http(s) 地址', trigger: 'blur' },
  ],
  collected_at: [{ required: true, message: '请选择采集时点', trigger: 'change' }],
});

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
const SORTABLE_KEYS = ['id', 'change_pct', 'sort', 'collected_at', 'audit_status'];

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
  [statusOptions.value] = await Promise.all([getDictFunc('status')]);
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
        message: res.msg || '创建/更改成功',
        duration: 5000,
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


const getStatusType = status => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};

// 依据类型（与后端 source_type 取值一致：1公告 2年报 3招股书 4官方产业目录 5互动易问答）
const sourceTypeOptions = [
  { value: 1, label: '公告' },
  { value: 2, label: '年报' },
  { value: 3, label: '招股书' },
  { value: 4, label: '官方产业目录' },
  { value: 5, label: '互动易问答' },
];
const sourceTypeLabel = v => sourceTypeOptions.find(o => o.value === v)?.label ?? '-';

// 审核状态：只有"已通过"的关联才会对 C 端可见
const auditStatusOptions = [
  { value: 0, label: '草稿' },
  { value: 1, label: '待审' },
  { value: 2, label: '已通过' },
  { value: 3, label: '已驳回' },
];
const auditStatusLabel = v => auditStatusOptions.find(o => o.value === v)?.label ?? '-';
const auditTagType = v => (v === 2 ? 'success' : v === 3 ? 'danger' : v === 1 ? 'warning' : 'info');

// 涨跌幅红涨绿跌样式类
const rateClass = val => {
  if (val === null || val === undefined || val === '') return '';
  const num = Number(val);
  if (Number.isNaN(num)) return '';
  if (num > 0) return 'rate-rise';
  if (num < 0) return 'rate-fall';
  return '';
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

/* 详情归属依据：完整段落展示 */
.reason-full {
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.8;
  font-size: 14px;
}

/* 详情归属依据头部的采集时间 */
.reason-meta {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

/* 原文摘录：保留换行段落，列表里只显示 1 行，超出省略 */
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

/* 悬停提示顶部的时间说明 */
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
