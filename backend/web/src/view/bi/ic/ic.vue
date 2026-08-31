<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="发送人" prop="sender">
          <el-input v-model="searchInfo.sender" clearable placeholder="请输入发送人" />
        </el-form-item>

        <el-form-item label="IC型号" prop="name">
          <el-input v-model="searchInfo.name" clearable placeholder="请输入IC型号" />
        </el-form-item>

        <el-form-item label="类型" prop="type">
          <el-select v-model="searchInfo.type" placeholder="请选择类型">
            <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="创建时间" prop="created_at_range">
            <template #label>
              <span>创建时间</span>
            </template>

            <el-date-picker v-model="searchInfo.created_at_range" class="w-[228px]" type="daterange" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间" />
          </el-form-item>

          <el-form-item label="状态" prop="status">
            <el-select v-model="searchInfo.status" placeholder="请选择状态">
              <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
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
          <el-button v-auth="btnAuth.add" type="primary" icon="plus" disabled @click="openDialog()">新增</el-button>
          <el-button v-auth="btnAuth.batchDelete" icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
        </div>

        <div class="gva-pagination">
          <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
        </div>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id" @selection-change="handleSelectionChange">
        <el-table-column v-auth="btnAuth.batchDelete" align="center" type="selection" width="60" />

        <el-table-column align="left" label="ID" prop="id" width="120" />

        <el-table-column align="left" label="MID" prop="msg_id" width="120" />

        <el-table-column align="left" label="发送人" prop="sender" width="120" />

        <el-table-column align="left" label="IC型号" prop="name" width="240" />

        <el-table-column align="left" label="类型" prop="type" width="90" />

        <el-table-column align="left" label="次数" prop="times" width="90" />

        <el-table-column align="left" label="状态" prop="status" width="90" />

        <el-table-column sortable align="left" label="创建时间" prop="CreatedAt" width="120">
          <template #default="scope">{{ formatToDateTime(scope.row.CreatedAt, 'MM-DD HH:mm') }}</template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" min-width="90px">
          <template #default="scope">
            <el-button v-auth="btnAuth.edit" type="primary" link icon="edit" class="table-button" @click="updateIcFunc(scope.row)"></el-button>
            <el-button v-auth="btnAuth.delete" type="info" link icon="delete" @click="deleteRow(scope.row)"></el-button>
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

      <el-form ref="elFormRef" :model="formData" label-position="right" :rules="rule" label-width="90px" @keyup.enter="handleEnter">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="ID" prop="id">
              <el-input v-model="formData.id" disabled />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="MID" prop="msg_id">
              <el-input v-model="formData.msg_id" disabled />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="发送者" prop="sender">
              <el-input v-model="formData.sender" clearable placeholder="请输入发送者" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="类型" prop="type">
              <el-select v-model="formData.type" placeholder="请选择类型">
                <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="IC型号" prop="name">
              <el-input v-model="formData.name" clearable placeholder="请输入IC型号" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="次数" prop="times">
              <el-input v-model="formData.times" disabled />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import { createIc, deleteIc, deleteIcByIds, updateIc, findIc, getIcList } from '@/api/bi/ic';

defineOptions({
  name: 'Ic'
});
// 按钮权限实例化
const btnAuth = useBtnAuth();

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const typeOptions = ref();
const statusOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  msg_id: '',
  sender: '',
  type: '',
  name: '',
  times: undefined,
  status: undefined
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
  const table = await getIcList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
  typeOptions.value = await getDictFunc('bi_im_ic_type');
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
    deleteIcFunc(row);
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
    const res = await deleteIcByIds({ ids });
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
const updateIcFunc = async (row) => {
  const res = await findIc({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteIcFunc = async (row) => {
  const res = await deleteIc({ id: row.id });
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
    msg_id: '',
    sender: '',
    type: '',
    name: '',
    times: undefined,
    status: undefined
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
        res = await createIc(formData.value);
        break;
      case 'update':
        res = await updateIc(formData.value);
        break;
      default:
        res = await createIc(formData.value);
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

const handleEnter = (event) => {
  if (event.shiftKey || event.ctrlKey || event.metaKey) {
    return;
  } else {
    event.preventDefault();
    enterDialog();
  }
};
</script>

<style scoped>
.gva-search-box {
  .el-button {
    margin-left: 0;
    margin-right: 12px;
  }
}

.gva-table-box-head {
  display: flex;
  justify-content: space-between;

  .el-pagination {
    margin-top: 1em;
    margin-bottom: 2em;
  }
}
</style>
