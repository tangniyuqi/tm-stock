<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="名称" prop="name">
          <el-input v-model="searchInfo.name" clearable placeholder="请输入名称关键词" />
        </el-form-item>

        <el-form-item label="服务器" prop="server">
          <el-input v-model="searchInfo.server" clearable placeholder="请输入服务器IP" />
        </el-form-item>

        <el-form-item label="状态" prop="status" v-if="!status">
          <el-select v-model="searchInfo.status" clearable placeholder="请选择状态">
            <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <template v-if="showAllQuery">
          <!-- 将需要控制显示状态的查询条件添加到此范围内 -->
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
        <el-table-column type="selection" align="center" width="60" />

        <el-table-column align="left" label="ID" prop="id" width="90" />

        <el-table-column align="left" label="名称" prop="name" min-width="120" />

        <el-table-column align="left" label="服务器" prop="server" width="130" />

        <el-table-column align="center" label="名单类型" prop="list_type" width="120">
          <template #default="scope">
            <el-tag type="primary">{{ filterDict(String(scope.row.list_type), listTypeOptions) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="备注" prop="remark" width="120" />

        <el-table-column sortable align="left" label="时间" prop="created_at" width="180">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>

        <el-table-column align="center" label="状态" prop="status" width="90">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">{{ filterDict(String(scope.row.status), statusOptions) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith - 80">
          <template #default="scope">
            <el-button type="primary" link class="table-button" @click="getDetails(scope.row)">
              <el-icon style="margin-right: 5px">
                <InfoFilled />
              </el-icon>
              查看
            </el-button>
            <el-button type="primary" link icon="edit" class="table-button" @click="updateClientFunc(scope.row)">编辑</el-button>
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
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="名称" prop="name">
              <el-input v-model="formData.name" clearable placeholder="请输入名称" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="服务器" prop="server">
              <el-input v-model="formData.server" clearable placeholder="请输入服务器" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="群白名单" prop="allow_list">
              <el-input v-model="formData.allow_list" type="textarea" :autosize="{ minRows: 4, maxRows: 8 }" placeholder="请输入白名单群名，一行一个" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="群黑名单" prop="block_list">
              <el-input v-model="formData.block_list" type="textarea" :autosize="{ minRows: 4, maxRows: 8 }" placeholder="请输入黑名单群名，一行一个" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="名单类型" prop="list_type">
              <!-- <el-select v-model="formData.list_type" placeholder="请选择名单类型">
                <el-option v-for="(item, key) in listTypeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select> -->

              <el-radio-group v-model="formData.list_type" class="mb-2">
                <el-radio v-for="(item, key) in listTypeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-radio-group>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="备注" prop="remark">
              <el-input v-model="formData.remark" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="请输入备注" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="状态" prop="status">
              <el-select v-model="formData.status" placeholder="请选择状态">
                <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="名称">
          {{ detailForm.name }}
        </el-descriptions-item>
        <el-descriptions-item label="服务器">
          {{ detailForm.server }}
        </el-descriptions-item>
        <el-descriptions-item label="备注">
          {{ detailForm.remark }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ detailForm.status }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { createClient, deleteClient, deleteClientByIds, updateClient, findClient, getClientList } from '@/api/bi/client';

// 全量引入格式化工具 请按需保留
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';

defineOptions({
  name: 'Client'
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const listTypeOptions = ref();
const statusOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  name: '',
  server: '',
  list_type: 0,
  allow_list: '',
  block_list: '',
  remark: '',
  status: 0
});

// 验证规则
const rule = reactive({
  name: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur']
    },
    {
      whitespace: true,
      message: '不能只输入空格',
      trigger: ['input', 'blur']
    }
  ]
});

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
  const table = await getClientList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
  if (table.code === 0) {
    tableData.value = table.data.list;
    total.value = table.data.total;
    page.value = table.data.page;
    pageSize.value = table.data.pageSize;
  }
};

getTableData();

// ============== 表格控制部分结束 ===============

// 获取需要的字典
const setOptions = async () => {
  listTypeOptions.value = await getDictFunc('bi_im_client_list_type');
  statusOptions.value = await getDictFunc('status');
};

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
    deleteClientFunc(row);
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
    const res = await deleteClientByIds({ ids });
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
const updateClientFunc = async (row) => {
  const res = await findClient({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteClientFunc = async (row) => {
  const res = await deleteClient({ id: row.id });
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
    name: '',
    server: '',
    list_type: 0,
    allow_list: '',
    block_list: '',
    remark: '',
    status: 0
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
        res = await createClient(formData.value);
        break;
      case 'update':
        res = await updateClient(formData.value);
        break;
      default:
        res = await createClient(formData.value);
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

const detailForm = ref({});

// 查看详情控制标记
const detailShow = ref(false);

// 打开详情弹窗
const openDetailShow = () => {
  detailShow.value = true;
};

// 打开详情
const getDetails = async (row) => {
  // 打开弹窗
  const res = await findClient({ id: row.id });
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

const getStatusType = (status) => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};
</script>

<style></style>
