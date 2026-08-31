<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="名称" prop="title">
          <el-input v-model="searchInfo.title" clearable placeholder="请输入名称关键词" />
        </el-form-item>

        <el-form-item label="行业" prop="industry">
          <el-input v-model="searchInfo.industry" clearable placeholder="请输入行业关键词" />
        </el-form-item>

        <el-form-item label="类型" prop="type">
          <el-select v-model="searchInfo.type" placeholder="请选择类型">
            <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="等级" prop="level">
            <el-select v-model="searchInfo.level" placeholder="请选择等级">
              <el-option v-for="(item, key) in levelOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item>

          <el-form-item label="机构" prop="institution">
            <el-input v-model="searchInfo.institution" clearable placeholder="请输入发布机构" />
          </el-form-item>

          <el-form-item label="作者" prop="analyst">
            <el-input v-model="searchInfo.analyst" clearable placeholder="请输入分析师" />
          </el-form-item>

          <el-form-item label="摘要" prop="conclusion">
            <el-input v-model="searchInfo.conclusion" clearable placeholder="请输入摘要关键词" />
          </el-form-item>

          <el-form-item label="发布日期" prop="publish_date_range">
            <template #label>
              <span>发布日期</span>
            </template>

            <el-date-picker v-model="searchInfo.publish_date_range" class="w-[228px]" type="daterange" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间" />
          </el-form-item>

          <!-- <el-form-item label="备注" prop="remark">
            <el-input v-model="searchInfo.remark" clearable placeholder="请输入备注关键词" />
          </el-form-item> -->

          <!-- <el-form-item label="状态" prop="status">
            <el-select v-model="searchInfo.status" placeholder="请选择状态">
              <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item> -->
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

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id" @selection-change="handleSelectionChange">
        <el-table-column v-auth="btnAuth.batchDelete" align="center" type="selection" width="50" />

        <el-table-column align="left" label="ID" prop="id" width="70" />

        <el-table-column align="left" label="研报名称" prop="title" min-width="300">
          <template #default="scope">
            <el-text class="mr-1" v-html="`${highlightKeywords(scope.row.title, searchInfo.title)}`" @click="openFilePreviewDialog(scope.row)" />
            <el-tag size="small" :type="getFileExtType(scope.row.file_ext)" v-if="scope.row.file_ext">{{ scope.row.file_ext }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="行业" prop="industry" width="120" />

        <el-table-column align="left" label="类型" prop="type" width="90">
          <template #default="scope">
            {{ filterDict(String(scope.row.type), typeOptions) }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="等级" prop="level" width="90">
          <template #default="scope">
            {{ filterDict(String(scope.row.level), levelOptions) }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="发布机构" prop="institution" width="100" />

        <el-table-column align="left" label="分析师" prop="analyst" width="120" />

        <el-table-column align="left" label="发布日期" prop="publish_date" width="120">
          <template #default="scope">{{ formatToDateTime(scope.row.publish_date, 'YYYY-MM-DD') }}</template>
        </el-table-column>

        <!-- <el-table-column align="left" label="有效日期" prop="deadline_date" width="120">
          <template #default="scope">{{ formatToDateTime(scope.row.deadline_date, 'YYYY-MM-DD') }}</template>
        </el-table-column> -->

        <!-- <el-table-column align="left" label="结论摘要" prop="conclusion" width="120" /> -->

        <!-- <el-table-column align="left" label="页数" prop="page_count" width="120" /> -->

        <!-- <el-table-column align="left" label="备注" prop="remark" width="120" /> -->

        <el-table-column sortable align="left" label="创建时间" prop="created_at" width="150">
          <template #default="scope">{{ formatToDateTime(scope.row.created_at, 'YYYY-MM-DD HH:mm') }}</template>
        </el-table-column>

        <!-- <el-table-column align="center" label="状态" prop="status" width="90">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">{{ filterDict(String(scope.row.status), statusOptions) }}</el-tag>
          </template>
        </el-table-column> -->

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith - 120">
          <template #default="scope">
            <el-button v-auth="btnAuth.info" type="primary" link icon="view" class="table-button" @click="openFilePreviewDialog(scope.row)" />
            <el-button v-auth="btnAuth.edit" type="primary" link icon="edit" class="table-button" @click="updateReportFunc(scope.row)" />
            <el-button v-auth="btnAuth.delete" link icon="delete" @click="deleteRow(scope.row)" />
          </template>
        </el-table-column>
      </el-table>

      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
      </div>
    </div>

    <el-drawer destroy-on-close :size="appStore.drawerSize * 1.2" v-model="dialogFormVisible" :show-close="false" :before-close="closeDialog">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">{{ type === 'create' ? '新增' : '编辑' }}</span>
          <div>
            <el-button :loading="btnLoading" type="primary" @click="enterDialog">确 定</el-button>
            <el-button @click="closeDialog">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form ref="elFormRef" :model="formData" :rules="rule" label-position="right" label-width="90px">
        <el-row>
          <el-col :span="24">
            <el-form-item label="研报名称" prop="title">
              <el-input v-model="formData.title" :clearable="true" placeholder="请输入研报名称" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="行业" prop="industry">
              <el-input v-model="formData.industry" clearable placeholder="请输入行业" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="类型" prop="type">
              <el-select v-model="formData.type" placeholder="请选择类型">
                <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="等级" prop="level">
              <el-select v-model="formData.level" placeholder="请选择等级">
                <el-option v-for="(item, key) in levelOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="发布机构" prop="institution">
              <el-input v-model="formData.institution" clearable placeholder="请输入发布机构" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="分析师" prop="analyst">
              <el-input v-model="formData.analyst" clearable placeholder="请输入分析师" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="发布日期" prop="publish_date">
              <el-date-picker v-model="formData.publish_date" type="date" style="width: 100%" placeholder="选择日期" :clearable="true" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="有效日期" prop="deadline_date">
              <el-date-picker v-model="formData.deadline_date" type="date" style="width: 100%" placeholder="选择日期" :clearable="true" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="研报上传" prop="file">
              <upload-file v-model="fileList" accept=".pdf,.docx,.xlsx,.pptx" autoUpload showFileList updateBtnName="请选择需要上传的文件" @on-success="uploadSuccess" @on-remove="uploadRemove" />
            </el-form-item>
          </el-col>

          <!-- <el-col :span="12">
            <el-form-item label="研报上传" prop="file">
              <attachment-manager v-model="formData.attachments" mode="operator" />
            </el-form-item>
          </el-col> -->

          <el-col :span="24">
            <el-form-item label="研报摘要" prop="conclusion">
              <VditorEditor v-model="formData.conclusion" :min-height="150" cache-id="unique-editor-id" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="备注" prop="remark">
              <el-input v-model="formData.remark" type="textarea" :rows="3" placeholder="请输入备注" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" show-close :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="标题">
          {{ detailForm.title }}
        </el-descriptions-item>

        <el-descriptions-item label="类型">
          {{ detailForm.type }}
        </el-descriptions-item>

        <el-descriptions-item label="行业">
          {{ detailForm.industry }}
        </el-descriptions-item>
        <el-descriptions-item label="等级">
          {{ detailForm.level }}
        </el-descriptions-item>
        <el-descriptions-item label="发布机构">
          {{ detailForm.institution }}
        </el-descriptions-item>
        <el-descriptions-item label="分析师">
          {{ detailForm.analyst }}
        </el-descriptions-item>

        <el-descriptions-item label="发布日期">
          {{ detailForm.publish_date }}
        </el-descriptions-item>

        <el-descriptions-item label="有效日期">
          {{ detailForm.deadline_date }}
        </el-descriptions-item>

        <el-descriptions-item label="结论摘要">
          {{ detailForm.conclusion }}
        </el-descriptions-item>

        <el-descriptions-item label="文件">
          {{ detailForm.file }}
        </el-descriptions-item>

        <el-descriptions-item label="页数">
          {{ detailForm.page_count }}
        </el-descriptions-item>

        <el-descriptions-item label="备注">
          {{ detailForm.remark }}
        </el-descriptions-item>

        <el-descriptions-item label="状态">
          {{ detailForm.status }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>

    <el-drawer destroy-on-close size="100%" direction="btt" :title="reportTitle" v-model="reportFileVisible" show-close>
      <vue-office-pdf class="office-container" :src="reportFilePath" v-if="reportFileExt == 'pdf'" />
      <vue-office-docx class="office-container" :src="reportFilePath" v-if="reportFileExt == 'docx'" />
      <vue-office-excel class="office-container" :src="reportFilePath" v-if="reportFileExt == 'xlsx'" />
      <vue-office-pptx class="office-container" :src="reportFilePath" v-if="reportFileExt == 'pptx'" />
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { getBaseUrl, getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import { createReport, deleteReport, deleteReportByIds, updateReport, findReport, getReportList } from '@/api/quant/report';
import { ElMessage, ElMessageBox } from 'element-plus';
import RichView from '@/components/richtext/rich-view.vue';
import UploadFile from '@/components/tm-upload/file.vue';
import VditorEditor from '@/components/vditorEditor/VditorEditor.vue';
import VueOfficePdf from '@vue-office/pdf';
import VueOfficeDocx from '@vue-office/docx';
import VueOfficeExcel from '@vue-office/excel';
import VueOfficePptx from '@vue-office/pptx';
// import AttachmentManager from '@/plugin/fileManager/components/AttachmentManager.vue';

defineOptions({
  name: 'Report'
});

// 按钮权限实例化
const btnAuth = useBtnAuth();

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const typeOptions = ref();
const levelOptions = ref();
const fileTypeOptions = ref();
const statusOptions = ref();
const fileList = ref([]);

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  title: '',
  type: undefined,
  industry: '',
  level: undefined,
  institution: '',
  analyst: '',
  publish_date: new Date(),
  deadline_date: new Date(),
  conclusion: '',
  file_path: '',
  file_ext: '',
  page_count: undefined,
  remark: '',
  status: undefined
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
    if (searchInfo.value.type === '') {
      searchInfo.value.type = null;
    }
    if (searchInfo.value.level === '') {
      searchInfo.value.level = null;
    }
    if (searchInfo.value.status === '') {
      searchInfo.value.status = null;
    }
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
  const table = await getReportList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
  typeOptions.value = await getDictFunc('quant_report_type');
  levelOptions.value = await getDictFunc('quant_report_level');
  fileTypeOptions.value = await getDictFunc('file_type');
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
    deleteReportFunc(row);
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
    const res = await deleteReportByIds({ ids });
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
const updateReportFunc = async (row) => {
  const res = await findReport({ id: row.id });
  type.value = 'update';

  if (res.code === 0) {
    formData.value = res.data;

    if (formData.value.file_path) {
      fileList.value.push({
        name: formData.value.title + '.' + formData.value.file_ext,
        url: formData.value.file_path
      });
    }

    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteReportFunc = async (row) => {
  const res = await deleteReport({ id: row.id });
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
  fileList.value = [];

  formData.value = {
    title: '',
    type: undefined,
    industry: '',
    level: undefined,
    institution: '',
    analyst: '',
    publish_date: new Date(),
    deadline_date: new Date(),
    conclusion: '',
    file_path: '',
    file_ext: '',
    page_count: undefined,
    remark: '',
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
        res = await createReport(formData.value);
        break;
      case 'update':
        res = await updateReport(formData.value);
        break;
      default:
        res = await createReport(formData.value);
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
  const res = await findReport({ id: row.id });
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

// 文件预览控制标记
const reportFileVisible = ref(false);
const reportTitle = ref('');
const reportFilePath = ref('');
const reportFileExt = ref('');

// 打开文件预览弹窗
const openFilePreviewDialog = (row) => {
  reportTitle.value = row.title || '无标题';
  reportFilePath.value = '/' + row.file_path;
  reportFileExt.value = row.file_ext;
  reportFileVisible.value = true;
};

// 关键词高亮方法
const highlightKeywords = (text, keyword) => {
  if (!text || !keyword) return text;
  const keywords = keyword.trim().split(' ');
  let highlighted = text;
  keywords.forEach((k) => {
    if (k) {
      const regex = new RegExp(k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
      highlighted = highlighted.replace(regex, (match) => `<span style="color: #F00;">${match}</span>`);
    }
  });
  return highlighted;
};

const getFileExtType = (ext) => {
  return ext == 'pdf' ? 'warning' : ext == 'docx' ? 'info' : 'success';
};

const getStatusType = (status) => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};

const uploadSuccess = (res) => {
  formData.value.file_path = res.data.file.url;
  formData.value.file_ext = res.data.file.tag;
};

const uploadRemove = (file) => {
  formData.value.file_path = '';
  formData.value.file_ext = '';
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

.el-table .el-table__row td .cell {
  line-height: 26px;
}

.office-container {
  width: 100%;
  height: 100vh;
}
</style>
