<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item v-auth="btnAuth.add" label="用户" prop="member_id">
          <el-input v-model.number="searchInfo.member_id" placeholder="请输入用户ID" />
        </el-form-item>

        <el-form-item v-auth="btnAuth.add" label="账户" prop="account_id">
          <el-input v-model.number="searchInfo.account_id" placeholder="请输入账户ID" />
        </el-form-item>

        <el-form-item label="策略ID" prop="strategy_id">
          <el-input v-model.number="searchInfo.strategy_id" placeholder="请输入策略ID" />
        </el-form-item>

        <el-form-item label="名称" prop="name">
          <el-input v-model="searchInfo.name" placeholder="请输入名称" />
        </el-form-item>

        <el-form-item label="备注" prop="remark">
          <el-input v-model="searchInfo.remark" placeholder="请输入备注" />
        </el-form-item>

        <el-form-item label="状态" prop="status">
          <el-select v-model="searchInfo.status" placeholder="请选择状态" clearable style="width: 178px">
            <el-option label="运行中" :value="1" />
            <el-option label="未运行" :value="0" />
          </el-select>
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="日期" prop="created_at_range">
            <el-date-picker v-model="searchInfo.created_at_range" type="daterange" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间" style="width: 240px" />
          </el-form-item>
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
      <div class="gva-table-box-head">
        <div class="gva-btn-list">
          <el-button v-auth="btnAuth.add" type="primary" icon="plus" @click="openDialog()">新增</el-button>
          <el-button v-auth="btnAuth.batchDelete" icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
        </div>

        <div class="gva-pagination">
          <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
        </div>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="ID" @selection-change="handleSelectionChange">
        <el-table-column v-auth="btnAuth.batchDelete" type="selection" width="55" />

        <el-table-column align="left" label="ID" prop="id" width="90" />

        <el-table-column align="left" label="用户ID" prop="member_id" width="90" />

        <el-table-column align="left" label="账户ID" prop="account_id" width="90" />

        <el-table-column align="left" label="策略ID" prop="strategy_id" width="90" />

        <el-table-column align="left" label="名称" prop="name" min-width="150" />

        <el-table-column align="left" label="备注" prop="remark" min-width="150" />

        <el-table-column sortable align="left" label="创建时间" prop="created_at" width="180">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>

        <el-table-column sortable align="left" label="最近更新时间" prop="updated_at" width="180">
          <template #default="scope">{{ formatDate(scope.row.updated_at) }}</template>
        </el-table-column>

        <el-table-column align="left" label="状态" prop="status" width="120">
          <template #default="scope">
            <el-tag v-if="scope.row.status === 1" type="success">运行中</el-tag>
            <el-tag v-else type="info">未运行</el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" min-width="180" v-if="btnAuth.info || btnAuth.edit || btnAuth.delete">
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
        <el-form-item label="用户ID" prop="member_id">
          <el-input v-model.number="formData.member_id" :clearable="true" placeholder="请输入用户ID" />
        </el-form-item>

        <el-form-item label="账户ID" prop="account_id">
          <el-input v-model.number="formData.account_id" :clearable="true" placeholder="请输入账户ID" />
        </el-form-item>

        <el-form-item label="策略ID" prop="strategy_id">
          <el-input v-model.number="formData.strategy_id" :clearable="true" placeholder="请输入策略ID" />
        </el-form-item>

        <el-form-item label="名称" prop="name">
          <el-input v-model="formData.name" :clearable="true" placeholder="请输入名称" />
        </el-form-item>

        <el-form-item label="备注" prop="remark">
          <el-input v-model="formData.remark" :clearable="true" placeholder="请输入备注" />
        </el-form-item>

        <el-form-item label="状态" prop="status">
          <el-input v-model.number="formData.status" :clearable="true" placeholder="请输入状态" />
        </el-form-item>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="用户ID">
          {{ detailForm.member_id }}
        </el-descriptions-item>
        <el-descriptions-item label="账户ID">
          {{ detailForm.account_id }}
        </el-descriptions-item>
        <el-descriptions-item label="策略ID">
          {{ detailForm.strategy_id }}
        </el-descriptions-item>
        <el-descriptions-item label="名称">
          {{ detailForm.name }}
        </el-descriptions-item>
        <el-descriptions-item label="股票">
          {{ detailForm.stock }}
        </el-descriptions-item>
        <el-descriptions-item label="配置">
          {{ detailForm.config }}
        </el-descriptions-item>
        <el-descriptions-item label="备注">
          {{ detailForm.remark }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag v-if="detailForm.status === 1" type="success">运行中</el-tag>
          <el-tag v-else type="info">未运行</el-tag>
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
import { createTradeTask, deleteTradeTask, deleteTradeTaskByIds, updateTradeTask, findTradeTask, getTradeTaskList } from '@/api/quant/tradeTask';

defineOptions({
  name: 'TradeTask',
});
// 按钮权限实例化
const btnAuth = useBtnAuth();

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  member_id: undefined,
  account_id: undefined,
  strategy_id: undefined,
  name: '',
  stock: {},
  config: {},
  remark: '',
  status: undefined,
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
  const table = await getTradeTaskList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
    deleteTradeTaskFunc(row);
  });
};

// 多选删除
const onDelete = async () => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(async () => {
    const IDs = [];
    if (multipleSelection.value.length === 0) {
      ElMessage({
        type: 'warning',
        message: '请选择要删除的数据',
      });
      return;
    }
    multipleSelection.value &&
      multipleSelection.value.map(item => {
        IDs.push(item.ID);
      });
    const res = await deleteTradeTaskByIds({ IDs });
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '删除成功',
      });
      if (tableData.value.length === IDs.length && page.value > 1) {
        page.value--;
      }
      getTableData();
    }
  });
};

// 行为控制标记（弹窗内部需要增还是改）
const type = ref('');

// 更新行
const updateTradeTaskFunc = async row => {
  const res = await findTradeTask({ ID: row.ID });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteTradeTaskFunc = async row => {
  const res = await deleteTradeTask({ ID: row.ID });
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
    name: '',
    stock: {},
    config: {},
    remark: '',
    status: undefined,
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
        res = await createTradeTask(formData.value);
        break;
      case 'update':
        res = await updateTradeTask(formData.value);
        break;
      default:
        res = await createTradeTask(formData.value);
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

// 查看详情控制标记
const detailShow = ref(false);

// 打开详情弹窗
const openDetailShow = () => {
  detailShow.value = true;
};

// 打开详情
const getDetails = async row => {
  // 打开弹窗
  const res = await findTradeTask({ ID: row.ID });
  if (res.code === 0) {
    detailForm.value = res.data;
    openDetailShow();
  }
};

// 关闭详情弹窗
const closeDetailShow = () => {
  detailShow.value = false;
  detailForm.value = {};
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
</style>
