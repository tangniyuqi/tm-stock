<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="方向" prop="direction">
          <el-input v-model="searchInfo.direction" clearable placeholder="请输入方向" />
        </el-form-item>

        <el-form-item label="类型" prop="type">
          <el-input v-model="searchInfo.type" clearable placeholder="请输入类型" />
        </el-form-item>

        <template v-if="showAllQuery"></template>

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
          <el-button link type="primary" icon="arrow-down" @click="showAllQuery = true" v-if="!showAllQuery">展开</el-button>
          <el-button link type="primary" icon="arrow-up" @click="showAllQuery = false" v-else>收起</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="gva-table-box">
      <div class="gva-table-box-head">
        <div class="gva-btn-list">
          <el-button v-auth="btnAuth.add" type="primary" icon="plus" @click="openDialog()">新增</el-button>
          <el-button v-auth="btnAuth.batchDelete" icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
        </div>

        <div class="gva-pagination">
          <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
        </div>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id" @selection-change="handleSelectionChange">
        <el-table-column v-auth="btnAuth.batchDelete" align="center" type="selection" width="60" />

        <el-table-column align="left" label="ID" prop="id" width="90" />

        <el-table-column align="left" label="用户ID" prop="member_id" width="90" />

        <el-table-column align="left" label="账户ID" prop="account_id" width="90" />

        <el-table-column align="left" label="交易时间" prop="traded_at" width="120">
          <template #default="scope">{{ formatToDateTime(scope.row.traded_at, 'HH:mm:ss') }}</template>
        </el-table-column>

        <el-table-column align="left" label="交易标的" prop="symbol_name" width="120">
          <template #default="scope">
            <div class="row-item">
              <div class="name">{{ scope.row.name }}</div>
              <div class="symbol">{{ scope.row.symbol }}</div>
            </div>
          </template>
        </el-table-column>

        <el-table-column align="center" label="交易指令" prop="action" width="120" />

        <el-table-column align="left" label="价格 (元)" prop="price" width="100" />

        <el-table-column align="left" label="数量（股）" prop="quantity" width="100" />

        <el-table-column align="left" label="金额 (元)" prop="amount" width="100" />

        <el-table-column align="left" label="原因" prop="reason" min-width="120" />

        <el-table-column align="left" label="创建时间" prop="created_at" width="180">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" width="180" v-if="btnAuth.info || btnAuth.edit || btnAuth.delete">
          <template #default="scope">
            <el-button v-auth="btnAuth.info" type="primary" link class="table-button" @click="getDetails(scope.row)">查看</el-button>
            <el-button v-auth="btnAuth.edit" type="primary" link class="table-button" @click="updateAccountFunc(scope.row)">编辑</el-button>
            <el-button v-auth="btnAuth.delete" type="primary" link @click="deleteRow(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
      </div>
    </div>
    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="dialogFormVisible" :show-close="false" :before-close="closeDialog">
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
        <el-form-item label="用户ID:" prop="member_id">
          <el-input v-model.number="formData.member_id" clearable placeholder="请输入用户ID" />
        </el-form-item>
        <el-form-item label="账户ID:" prop="account_id">
          <el-input v-model.number="formData.account_id" clearable placeholder="请输入账户ID" />
        </el-form-item>
        <el-form-item label="策略ID:" prop="strategy_id">
          <el-input v-model.number="formData.strategy_id" clearable placeholder="请输入策略ID" />
        </el-form-item>
        <el-form-item label="交易时间:" prop="traded_at">
          <el-date-picker v-model="formData.traded_at" type="date" style="width: 100%" placeholder="选择日期" clearable />
        </el-form-item>
        <el-form-item label="方向:" prop="direction">
          <el-input v-model="formData.direction" clearable placeholder="请输入方向" />
        </el-form-item>
        <el-form-item label="类型:" prop="type">
          <el-input v-model="formData.type" clearable placeholder="请输入类型" />
        </el-form-item>
        <el-form-item label="标识:" prop="symbol">
          <el-input v-model="formData.symbol" clearable placeholder="请输入标识" />
        </el-form-item>
        <el-form-item label="名称:" prop="name">
          <el-input v-model="formData.name" clearable placeholder="请输入名称" />
        </el-form-item>
        <el-form-item label="价格:" prop="price">
          <el-input-number v-model="formData.price" style="width: 100%" :precision="2" clearable />
        </el-form-item>
        <el-form-item label="数量:" prop="volume">
          <el-input-number v-model="formData.volume" style="width: 100%" :precision="2" clearable />
        </el-form-item>
        <el-form-item label="金额:" prop="amount">
          <el-input-number v-model="formData.amount" style="width: 100%" :precision="2" clearable />
        </el-form-item>
        <el-form-item label="创建者:" prop="created_by">
          <el-input v-model.number="formData.created_by" clearable placeholder="请输入创建者" />
        </el-form-item>
        <el-form-item label="更新者:" prop="updated_by">
          <el-input v-model.number="formData.updated_by" clearable placeholder="请输入更新者" />
        </el-form-item>
        <el-form-item label="删除者:" prop="deleted_by">
          <el-input v-model.number="formData.deleted_by" clearable placeholder="请输入删除者" />
        </el-form-item>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="用户ID">
          {{ detailFrom.member_id }}
        </el-descriptions-item>
        <el-descriptions-item label="账户ID">
          {{ detailFrom.account_id }}
        </el-descriptions-item>
        <el-descriptions-item label="策略ID">
          {{ detailFrom.strategy_id }}
        </el-descriptions-item>
        <el-descriptions-item label="交易时间">
          {{ detailFrom.traded_at }}
        </el-descriptions-item>
        <el-descriptions-item label="方向">
          {{ detailFrom.direction }}
        </el-descriptions-item>
        <el-descriptions-item label="类型">
          {{ detailFrom.type }}
        </el-descriptions-item>
        <el-descriptions-item label="标识">
          {{ detailFrom.symbol }}
        </el-descriptions-item>
        <el-descriptions-item label="价格">
          {{ detailFrom.price }}
        </el-descriptions-item>
        <el-descriptions-item label="数量">
          {{ detailFrom.volume }}
        </el-descriptions-item>
        <el-descriptions-item label="金额">
          {{ detailFrom.amount }}
        </el-descriptions-item>
        <el-descriptions-item label="创建者">
          {{ detailFrom.created_by }}
        </el-descriptions-item>
        <el-descriptions-item label="更新者">
          {{ detailFrom.updated_by }}
        </el-descriptions-item>
        <el-descriptions-item label="删除者">
          {{ detailFrom.deleted_by }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue';
import { useBtnAuth } from '@/utils/btnAuth';
import { useAppStore } from '@/pinia';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import { createTradeRecord, deleteTradeRecord, deleteTradeRecordByIds, updateTradeRecord, findTradeRecord, getTradeRecordList } from '@/api/quant/tradeRecord';

defineOptions({
  name: 'TradeRecord',
});

const btnAuth = useBtnAuth();
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  member_id: undefined,
  account_id: undefined,
  strategy_id: undefined,
  traded_at: new Date(),
  direction: '',
  type: '',
  symbol: '',
  price: 0,
  volume: 0,
  amount: 0,
  created_by: undefined,
  updated_by: undefined,
  deleted_by: undefined,
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
  const table = await getTradeRecordList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
const setOptions = async () => {};

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
    deleteTradeRecordFunc(row);
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
    const res = await deleteTradeRecordByIds({ ids });
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
const updateTradeRecordFunc = async row => {
  const res = await findTradeRecord({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteTradeRecordFunc = async row => {
  const res = await deleteTradeRecord({ id: row.id });
  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '删除成功',
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
  dialogFormVisible.value = true;
};

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;
  formData.value = {
    member_id: undefined,
    account_id: undefined,
    strategy_id: undefined,
    traded_at: new Date(),
    direction: '',
    type: '',
    symbol: '',
    price: 0,
    volume: 0,
    amount: 0,
    created_by: undefined,
    updated_by: undefined,
    deleted_by: undefined,
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
        res = await createTradeRecord(formData.value);
        break;
      case 'update':
        res = await updateTradeRecord(formData.value);
        break;
      default:
        res = await createTradeRecord(formData.value);
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

const detailFrom = ref({});

// 查看详情控制标记
const detailShow = ref(false);

// 打开详情弹窗
const openDetailShow = () => {
  detailShow.value = true;
};

// 打开详情
const getDetails = async row => {
  // 打开弹窗
  const res = await findTradeRecord({ id: row.id });
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
</script>

<style>
.gva-table-box-head {
  display: flex;
  justify-content: space-between;

  .el-pagination {
    margin-top: 1em;
    margin-bottom: 2em;
  }
}

.row-item {
  line-height: 20px;

  .name {
    /* color: #333; */
  }

  .symbol {
    color: #999;
  }
}
</style>
