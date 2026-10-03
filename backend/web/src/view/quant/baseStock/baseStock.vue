<template>
  <div>
    <!-- =========== 股票详情视图（页面内嵌，参考 aiTask 布局） =========== -->
    <div v-if="detailVisible" class="base-stock-detail">
      <div class="gva-btn-list my-3">
        <el-button icon="back" @click="backToList">返回列表</el-button>
        <el-button icon="refresh" @click="refreshDetail">刷新</el-button>
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
            v-if="btnAuth.info || btnAuth.delete">
            <template #default="scope">
              <el-button v-auth="btnAuth.info" type="primary" link @click="getDetails(scope.row)">查看</el-button>
              <el-button v-auth="btnAuth.delete" type="primary" link @click="deleteRow(scope.row)">删除</el-button>
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
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import { createBaseStock, deleteBaseStock, deleteBaseStockByIds, updateBaseStock, findBaseStock, getBaseStockList, sync, clear, updateAllChangePct } from '@/api/quant/baseStock';

defineOptions({
  name: 'BaseStock',
});

// 按钮权限实例化
const btnAuth = useBtnAuth();

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

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

// 同步（增量更新：已存在的股票更新基础信息，新增的股票插入，保留本地行情数据）
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

// 刷新详情数据（重新拉取最新数据）
const refreshDetail = async () => {
  if (!detailForm.value.id) return;
  const res = await findBaseStock({ id: detailForm.value.id });
  if (res.code === 0) {
    detailForm.value = res.data;
    ElMessage({ type: 'success', message: '刷新成功' });
  }
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

.gva-table-box-head {
  display: flex;
  justify-content: space-between;

  .el-pagination {
    margin-top: 1em;
    margin-bottom: 2em;
  }
}

/* 涨跌幅：红涨绿跌 */
.rate-rise {
  color: #f56c6c;
}

.rate-fall {
  color: #67c23a;
}
</style>
