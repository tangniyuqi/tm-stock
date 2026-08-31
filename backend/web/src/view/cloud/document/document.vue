<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" inline :model="searchInfo" class="demo-form-inline" :rules="searchRule" @keyup.enter="onSubmit">
        <el-form-item label="分类" prop="type">
          <el-select v-model="searchInfo.type" clearable placeholder="请选择分类">
            <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item label="标题" prop="title">
          <el-input v-model="searchInfo.title" clearable placeholder="请输入标题关键词" />
        </el-form-item>

        <el-form-item label="内容" prop="content">
          <el-input v-model="searchInfo.content" clearable placeholder="请输入内容关键词" />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
          <el-button link type="primary" icon="arrow-down" @click="showAllQuery = true" v-if="!showAllQuery">展开</el-button>
          <el-button link type="primary" icon="arrow-up" @click="showAllQuery = false" v-else>收起</el-button>
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="标签" prop="tags">
            <el-input v-model="searchInfo.tags" clearable placeholder="请输入标签" />
          </el-form-item>

          <el-form-item label="创建日期" prop="createdAt">
            <el-date-picker v-model="searchInfo.startcreated_at" type="datetime" placeholder="开始日期" :disabled-date="time => (searchInfo.endcreated_at ? time.getTime() > searchInfo.endcreated_at.getTime() : false)"></el-date-picker>
            —
            <el-date-picker v-model="searchInfo.endcreated_at" type="datetime" placeholder="结束日期" :disabled-date="time => (searchInfo.startcreated_at ? time.getTime() < searchInfo.startcreated_at.getTime() : false)"></el-date-picker>
          </el-form-item>
        </template>
      </el-form>
    </div>
    <div class="gva-table-box">
      <div class="gva-btn-list">
        <el-button type="primary" icon="plus" @click="openDialog()">新增</el-button>
        <el-button icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id" @selection-change="handleSelectionChange">
        <el-table-column align="center" type="selection" width="50" />
        <el-table-column align="left" label="ID" prop="id" width="80" />

        <el-table-column align="left" label="标题" prop="title" min-width="320">
          <template #default="scope">
            <el-text class="text-content" v-html="highlightKeywords(scope.row.title, searchInfo.title)" />
          </template>
        </el-table-column>

        <el-table-column align="left" label="分类" prop="type" width="120">
          <template #default="scope">
            <el-tag type="info" effect="plain">
              {{ filterDict(String(scope.row.type), typeOptions) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column align="center" label="排序" prop="sort" width="90" />

        <el-table-column align="left" label="创建时间" prop="createdAt" width="180">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>

        <el-table-column align="left" label="最近更新时间" prop="updated_at" width="180">
          <template #default="scope">{{ formatDate(scope.row.updated_at) }}</template>
        </el-table-column>

        <!-- <el-table-column sortable align="left" label="状态" prop="status" width="100">
          <template #default="scope">
            <el-tag type="info" effect="plain">
              {{ filterDict(String(scope.row.status), statusOptions) }}
            </el-tag>
          </template>
        </el-table-column> -->

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith - 80">
          <template #default="scope">
            <el-button type="primary" link class="table-button" @click="getDetails(scope.row)"
              ><el-icon style="margin-right: 5px"> <InfoFilled /> </el-icon>查看</el-button
            >
            <el-button type="primary" link icon="edit" class="table-button" @click="updateDocumentFunc(scope.row)">编辑</el-button>
            <el-button type="primary" link icon="delete" @click="deleteRow(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
      </div>
    </div>
    <el-drawer destroy-on-close size="80%" v-model="dialogFormVisible" :show-close="false" :before-close="closeDialog">
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
          <el-col :span="12">
            <el-form-item label="标题" prop="title">
              <el-input v-model="formData.title" clearable placeholder="请输入标题" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="分类" prop="type">
              <el-select v-model="formData.type" placeholder="请选择分类">
                <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="外链地址" prop="url">
              <el-input v-model="formData.url" clearable placeholder="请输入外链地址" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="跳转地址" prop="link">
              <el-input v-model="formData.link" clearable placeholder="请输入跳转地址" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="内容" prop="content">
              <VditorEditor v-model="formData.content" :min-height="320" cache-id="unique-editor-id" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="标签" prop="tags">
              <!-- <ArrayCtrl v-model="formData.tags" editable /> -->
              <el-input-tag v-model="formData.tags" :max="10" draggable clearable placeholder="请输入标签，按enter键，最多10个标签" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close size="65%" v-model="detailShow" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="标题">
          {{ detailFrom.title }}
        </el-descriptions-item>
        <el-descriptions-item label="分类">
          {{ detailFrom.type }}
        </el-descriptions-item>
        <el-descriptions-item label="外链地址">
          {{ detailFrom.url }}
        </el-descriptions-item>
        <el-descriptions-item label="跳转地址">
          {{ detailFrom.link }}
        </el-descriptions-item>
        <el-descriptions-item label="摘要">
          {{ detailFrom.summary }}
        </el-descriptions-item>
        <el-descriptions-item label="内容">
          <RichView v-model="detailFrom.content" />
        </el-descriptions-item>
        <el-descriptions-item label="标签">
          {{ detailFrom.tags }}
        </el-descriptions-item>
        <el-descriptions-item label="排序">
          {{ detailFrom.sort }}
        </el-descriptions-item>
        <el-descriptions-item label="浏览量">
          {{ detailFrom.view }}
        </el-descriptions-item>
        <el-descriptions-item label="顶置">
          {{ detailFrom.top }}
        </el-descriptions-item>
        <el-descriptions-item label="精华">
          {{ detailFrom.digest }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ detailFrom.status }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { createDocument, deleteDocument, deleteDocumentByIds, updateDocument, findDocument, getDocumentList } from '@/api/cloud/document';
// 富文本组件
// import RichEdit from '@/components/richtext/rich-edit.vue';
import RichView from '@/components/richtext/rich-view.vue';
import ArrayCtrl from '@/components/arrayCtrl/arrayCtrl.vue';
import VditorEditor from '@/components/vditorEditor/VditorEditor.vue';

// 全量引入格式化工具 请按需保留
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime } from '@/utils/dateTimeUtils';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';

defineOptions({
  name: 'Document',
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const typeOptions = ref();
const statusOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  merchant_id: 0,
  title: '',
  type: 0,
  url: '',
  link: '',
  summary: '',
  content: '',
  tags: [],
  sort: 0,
  view: 0,
  top: 0,
  digest: 0,
  status: 0,
});

// 验证规则
const rule = reactive({
  title: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur'],
    },
    {
      whitespace: true,
      message: '不能只输入空格',
      trigger: ['input', 'blur'],
    },
  ],
});

const searchRule = reactive({
  createdAt: [
    {
      validator: (rule, value, callback) => {
        if (searchInfo.value.startcreated_at && !searchInfo.value.endcreated_at) {
          callback(new Error('请填写结束日期'));
        } else if (!searchInfo.value.startcreated_at && searchInfo.value.endcreated_at) {
          callback(new Error('请填写开始日期'));
        } else if (searchInfo.value.startcreated_at && searchInfo.value.endcreated_at && (searchInfo.value.startcreated_at.getTime() === searchInfo.value.endcreated_at.getTime() || searchInfo.value.startcreated_at.getTime() > searchInfo.value.endcreated_at.getTime())) {
          callback(new Error('开始日期应当早于结束日期'));
        } else {
          callback();
        }
      },
      trigger: 'change',
    },
  ],
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
  const table = await getDocumentList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
  typeOptions.value = await getDictFunc('cloud_document_type');
  statusOptions.value = await getDictFunc('status');
};

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
    deleteDocumentFunc(row);
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
    const res = await deleteDocumentByIds({ ids });
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
const updateDocumentFunc = async row => {
  const res = await findDocument({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteDocumentFunc = async row => {
  const res = await deleteDocument({ id: row.id });
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
    merchant_id: 0,
    title: '',
    type: 0,
    url: '',
    link: '',
    summary: '',
    content: '',
    tags: [],
    sort: 0,
    view: 0,
    top: 0,
    digest: 0,
    status: 0,
  };
};
// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  formData.value.type = parseInt(formData.value.type);

  elFormRef.value?.validate(async valid => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createDocument(formData.value);
        break;
      case 'update':
        res = await updateDocument(formData.value);
        break;
      default:
        res = await createDocument(formData.value);
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
  const res = await findDocument({ id: row.id });
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

// 关键词高亮方法
const highlightKeywords = (text, keyword) => {
  if (!text || !keyword) return text;
  const keywords = keyword.trim().split(' ');
  let highlighted = text;
  keywords.forEach(k => {
    if (k) {
      const regex = new RegExp(k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
      highlighted = highlighted.replace(regex, match => `<span style="color: #F00;">${match}</span>`);
    }
  });
  return highlighted;
};
</script>

<style></style>
