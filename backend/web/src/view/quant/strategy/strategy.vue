<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="名称" prop="name">
          <el-input v-model="searchInfo.name" clearable placeholder="请输入名称关键词" />
        </el-form-item>

        <el-form-item label="状态" prop="status">
          <el-select v-model="searchInfo.status" placeholder="请选择状态">
            <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
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
      <div class="gva-btn-list">
        <el-button type="primary" icon="plus" @click="openDialog()">新增</el-button>
        <el-button icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id" @selection-change="handleSelectionChange" @sort-change="handleSortChange">
        <el-table-column align="center" type="selection" width="60" />

        <el-table-column align="left" label="策略ID" prop="id" width="90" />

        <el-table-column align="left" label="用户" prop="member_id" width="90" />

        <el-table-column align="left" label="策略名称" prop="name" min-width="240" />

        <el-table-column align="left" label="备注" prop="remark" min-width="150" />

        <el-table-column align="left" label="排序" prop="sort" width="90" sortable="custom" />

        <el-table-column align="left" label="创建时间" prop="created_at" width="180">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>

        <el-table-column align="left" label="最近更新时间" prop="updated_at" width="180">
          <template #default="scope">{{ formatDate(scope.row.updated_at) }}</template>
        </el-table-column>

        <el-table-column align="left" label="状态" prop="status" width="90">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">{{ filterDict(String(scope.row.status), statusOptions) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" min-width="210">
          <template #default="scope">
            <el-button type="primary" link class="table-button" @click="getDetails(scope.row)">
              <el-icon style="margin-right: 5px">
                <InfoFilled />
              </el-icon>
              查看
            </el-button>
            <el-button type="primary" link icon="edit" class="table-button" @click="updateStrategyFunc(scope.row)">编辑</el-button>
            <el-button type="primary" link icon="delete" @click="deleteRow(scope.row)">删除</el-button>
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
        <el-form-item label="名称" prop="name">
          <el-input v-model="formData.name" clearable placeholder="请输入名称" />
        </el-form-item>

        <el-form-item label="备注" prop="remark">
          <el-input v-model="formData.remark" clearable placeholder="请输入备注" />
        </el-form-item>

        <el-form-item label="排序" prop="sort">
          <el-input-number v-model="formData.sort" :min="0" placeholder="请输入排序值" />
        </el-form-item>

        <el-form-item label="状态" prop="status">
          <el-select v-model="formData.status" placeholder="请选择状态">
            <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="用户ID">
          {{ detailFrom.member_id }}
        </el-descriptions-item>
        <el-descriptions-item label="名称">
          {{ detailFrom.name }}
        </el-descriptions-item>
        <el-descriptions-item label="交易时间检查周期">
          {{ detailFrom.trading_check_interval }}
        </el-descriptions-item>
        <el-descriptions-item label="股票监控周期">
          {{ detailFrom.stock_monitor_interval }}
        </el-descriptions-item>
        <el-descriptions-item label="备注">
          {{ detailFrom.remark }}
        </el-descriptions-item>
        <el-descriptions-item label="排序">
          {{ detailFrom.sort }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ detailFrom.status }}
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
import { createStrategy, deleteStrategy, deleteStrategyByIds, updateStrategy, findStrategy, getStrategyList } from '@/api/quant/strategy';

// 全量引入格式化工具 请按需保留
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';

defineOptions({
  name: 'Strategy'
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const statusOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  member_id: undefined,
  name: '',
  trading_check_interval: undefined,
  stock_monitor_interval: undefined,
  holdings_check_interval: undefined,
  market_check_interval: undefined,
  volume_calc: undefined,
  base_volume: undefined,
  min_volume: undefined,
  max_volume: undefined,
  remark: '',
  sort: 0,
  status: undefined,
  created_by: undefined,
  updated_by: undefined,
  deleted_by: undefined
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

// 排序变更处理
const handleSortChange = ({ column, prop, order }) => {
  // order: ascending(升序) / descending(降序) / null(取消排序)
  if (prop === 'sort') {
    if (order === 'ascending') {
      searchInfo.value.orderKey = 'sort';
      searchInfo.value.desc = false;
    } else if (order === 'descending') {
      searchInfo.value.orderKey = 'sort';
      searchInfo.value.desc = true;
    } else {
      // 取消排序，恢复默认排序
      delete searchInfo.value.orderKey;
      delete searchInfo.value.desc;
    }
    getTableData();
  }
};

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
  const table = await getStrategyList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
    deleteStrategyFunc(row);
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
    const res = await deleteStrategyByIds({ ids });
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
const updateStrategyFunc = async (row) => {
  const res = await findStrategy({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteStrategyFunc = async (row) => {
  const res = await deleteStrategy({ id: row.id });
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
  dialogFormVisible.value = true;
};

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;
  formData.value = {
    member_id: undefined,
    name: '',
    trading_check_interval: undefined,
    stock_monitor_interval: undefined,
    holdings_check_interval: undefined,
    market_check_interval: undefined,
    volume_calc: undefined,
    base_volume: undefined,
    min_volume: undefined,
    max_volume: undefined,
    remark: '',
    sort: 0,
    status: undefined,
    created_by: undefined,
    updated_by: undefined,
    deleted_by: undefined
  };
};
// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  elFormRef.value?.validate(async (valid) => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createStrategy(formData.value);
        break;
      case 'update':
        res = await updateStrategy(formData.value);
        break;
      default:
        res = await createStrategy(formData.value);
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
  const res = await findStrategy({ id: row.id });
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
</script>

<style></style>
