<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="用户ID" prop="member_id">
          <el-input v-model.number="searchInfo.member_id" clearable placeholder="请输入用户ID" />
        </el-form-item>

        <el-form-item label="提示词" prop="prompt">
          <el-input v-model="searchInfo.prompt" clearable placeholder="请输入提示词关键词" />
        </el-form-item>

        <!-- <el-form-item label="状态" prop="status">
          <el-select v-model="searchInfo.status" clearable placeholder="请选择状态">
            <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item> -->

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
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
        <el-table-column align="left" label="用户ID" prop="member_id" width="100" />
        <el-table-column align="left" label="提示词" prop="prompt" min-width="300" show-overflow-tooltip />
        <el-table-column align="center" label="次数" prop="times" width="180" />
        <el-table-column align="left" label="创建时间" min-width="150">
          <template #default="scope">
            {{ formatToDateTime(scope.row.created_at, 'YYYY-MM-DD HH:mm:ss') }}
          </template>
        </el-table-column>
        <el-table-column align="left" label="状态" prop="status" width="80">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">{{ filterDict(String(scope.row.status), statusOptions) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column align="center" label="操作" fixed="right" :min-width="160">
          <template #default="scope">
            <div class="cell">
              <el-button type="primary" link @click="getDetails(scope.row)">查看</el-button>
              <el-button type="primary" link @click="updateScreenerRecordFunc(scope.row)">编辑</el-button>
              <el-button type="danger" link @click="deleteRow(scope.row)">删除</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
      </div>
    </div>

    <el-drawer destroy-on-close size="50%" v-model="dialogFormVisible" :show-close="false" :before-close="closeDialog">
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
          <el-input v-model.number="formData.member_id" clearable placeholder="请输入用户ID" />
        </el-form-item>

        <el-form-item label="提示词" prop="prompt">
          <el-input v-model="formData.prompt" type="textarea" :rows="4" clearable placeholder="请输入提示词" />
        </el-form-item>

        <el-form-item label="次数" prop="times">
          <el-input-number v-model="formData.times" style="width: 100%" :min="0" clearable />
        </el-form-item>

        <el-form-item label="状态" prop="status">
          <el-select v-model="formData.status" placeholder="请选择状态" style="width: 100%">
            <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close size="50%" v-model="detailShow" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="ID">
          {{ detailFrom.id }}
        </el-descriptions-item>
        <el-descriptions-item label="用户ID">
          {{ detailFrom.member_id }}
        </el-descriptions-item>
        <el-descriptions-item label="提示词">
          {{ detailFrom.prompt }}
        </el-descriptions-item>
        <el-descriptions-item label="次数">
          {{ detailFrom.times }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ filterDict(String(detailFrom.status), statusOptions) }}
        </el-descriptions-item>
        <el-descriptions-item label="创建时间">
          {{ formatToDateTime(detailFrom.created_at, 'YYYY-MM-DD HH:mm:ss') }}
        </el-descriptions-item>
        <el-descriptions-item label="更新时间">
          {{ formatToDateTime(detailFrom.updated_at, 'YYYY-MM-DD HH:mm:ss') }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue';
import { useUserStore } from '@/pinia';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, filterDict } from '@/utils/format';
import { formatToDateTime } from '@/utils/dateTimeUtils';
import { createScreenerRecord, deleteScreenerRecord, deleteScreenerRecordByIds, updateScreenerRecord, findScreenerRecord, getScreenerRecordList } from '@/api/quant/screenerRecord';

defineOptions({
  name: 'ScreenerRecord'
});

const btnLoading = ref(false);
const userStore = useUserStore();

const statusOptions = ref();

const formData = ref({
  member_id: undefined,
  prompt: '',
  times: 0,
  status: 1
});

const rule = reactive({
  prompt: [{ required: true, message: '请输入提示词', trigger: 'blur' }]
});

const elFormRef = ref();
const elSearchFormRef = ref();

const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
const tableData = ref([]);
const searchInfo = ref({});

const onReset = () => {
  searchInfo.value = {};
  getTableData();
};

const onSubmit = () => {
  elSearchFormRef.value?.validate(async (valid) => {
    if (!valid) return;
    page.value = 1;
    getTableData();
  });
};

const handleSizeChange = (val) => {
  pageSize.value = val;
  getTableData();
};

const handleCurrentChange = (val) => {
  page.value = val;
  getTableData();
};

const getTableData = async () => {
  const table = await getScreenerRecordList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
  if (table.code === 0) {
    tableData.value = table.data.list;
    total.value = table.data.total;
    page.value = table.data.page;
    pageSize.value = table.data.pageSize;
  }
};

getTableData();

const setOptions = async () => {
  statusOptions.value = await getDictFunc('status');
};

setOptions();

const multipleSelection = ref([]);
const handleSelectionChange = (val) => {
  multipleSelection.value = val;
};

const deleteRow = (row) => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    deleteScreenerRecordFunc(row);
  });
};

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
    const res = await deleteScreenerRecordByIds({ ids });
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

const type = ref('');

const updateScreenerRecordFunc = async (row) => {
  const res = await findScreenerRecord({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

const deleteScreenerRecordFunc = async (row) => {
  const res = await deleteScreenerRecord({ id: row.id });
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

const dialogFormVisible = ref(false);

const openDialog = () => {
  type.value = 'create';
  formData.value.member_id = userStore.userInfo.ID;
  dialogFormVisible.value = true;
};

const closeDialog = () => {
  dialogFormVisible.value = false;
  formData.value = {
    member_id: undefined,
    prompt: '',
    times: 0,
    status: 1
  };
};

const enterDialog = async () => {
  btnLoading.value = true;
  elFormRef.value?.validate(async (valid) => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createScreenerRecord(formData.value);
        break;
      case 'update':
        res = await updateScreenerRecord(formData.value);
        break;
      default:
        res = await createScreenerRecord(formData.value);
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
const detailShow = ref(false);

const openDetailShow = () => {
  detailShow.value = true;
};

const getDetails = async (row) => {
  const res = await findScreenerRecord({ id: row.id });
  if (res.code === 0) {
    detailFrom.value = res.data;
    openDetailShow();
  }
};

const closeDetailShow = () => {
  detailShow.value = false;
  detailFrom.value = {};
};

const getStatusType = (status) => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};
</script>

<style></style>
