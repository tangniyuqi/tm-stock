<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline"
        @keyup.enter="onSubmit">
        <el-form-item label="标题" prop="title">
          <el-input v-model="searchInfo.title" placeholder="请输入标题" />
        </el-form-item>

        <el-form-item label="类型" prop="type">
          <el-select v-model="searchInfo.type" clearable placeholder="请选择类型">
            <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item label="题材" prop="theme_id">
          <el-select v-model.number="searchInfo.theme_id" filterable clearable placeholder="请选择题材">
            <el-option v-for="item in themeOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>

        <el-form-item label="情绪" prop="sentiment">
          <el-select v-model="searchInfo.sentiment" clearable placeholder="请选择情绪">
            <el-option v-for="(item, key) in sentimentOptions" :key="key" :label="item.label"
              :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="状态" prop="status">
            <el-select v-model="searchInfo.status" clearable placeholder="请选择状态">
              <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label"
                :value="Number(item.value)" />
            </el-select>
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
        @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="55" />

        <el-table-column align="left" label="标题" prop="title" min-width="420" />

        <el-table-column align="left" label="类型" prop="type" width="90">
          <template #default="scope">
            {{ filterDict(String(scope.row.type), typeOptions) }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="关联题材" width="120">
          <template #default="scope">
            <el-tag>{{ scope.row.theme_name || findThemeLabel(scope.row.theme_id) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="情绪" prop="sentiment" width="90">
          <template #default="scope">
            {{ filterDict(String(scope.row.sentiment), sentimentOptions) }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="发布日期" prop="publish_date" width="120">
          <template #default="scope">{{ formatToDateTime(scope.row.publish_date, 'yyyy-MM-DD') }}</template>
        </el-table-column>

        <el-table-column align="left" label="来源" prop="source" width="90" />

        <el-table-column align="left" label="排序" prop="sort" width="60" />

        <el-table-column align="left" label="置顶" prop="top" width="90">
          <template #default="scope">
            {{ filterDict(String(scope.row.top), whetherOptions) }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="热门" prop="hot" width="60">
          <template #default="scope">
            {{ filterDict(String(scope.row.hot), whetherOptions) }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="热度值" prop="heat" width="70" />

        <el-table-column align="left" label="浏览量" prop="view" width="70" />

        <el-table-column align="left" label="分享数" prop="share" width="70" />

        <el-table-column align="left" label="状态" prop="status" width="90">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">{{ filterDict(String(scope.row.status), statusOptions)
            }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column sortable align="left" label="创建时间" prop="created_at" width="150">
          <template #default="scope">{{ formatToDateTime(scope.row.created_at, 'yyyy-MM-DD HH:mm') }}</template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith - 80">
          <template #default="scope">
            <el-button v-auth="btnAuth.edit" type="primary" link icon="edit" class="table-button"
              @click="updateThemeTopicFunc(scope.row)">编辑</el-button>
            <el-button v-auth="btnAuth.delete" type="primary" link icon="delete"
              @click="deleteRow(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize"
          :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange"
          @size-change="handleSizeChange" />
      </div>
    </div>
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
          <el-col :span="24">
            <el-form-item label="标题" prop="title">
              <el-input v-model="formData.title" :clearable="true" placeholder="请输入标题" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="类型" prop="type">
              <el-select v-model="formData.type" clearable placeholder="请选择类型">
                <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label"
                  :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="情绪" prop="sentiment">
              <el-select v-model="formData.sentiment" clearable placeholder="请选择情绪">
                <el-option v-for="(item, key) in sentimentOptions" :key="key" :label="item.label"
                  :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="题材" prop="theme_id">
              <el-tree-select v-model="formData.theme_id" :data="themeTreeOptions" node-key="value"
                :props="{ label: 'label', children: 'children' }" check-strictly filterable clearable
                placeholder="请选择题材" style="width: 100%" :loading="themeLoading" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="发布日期" prop="publish_date">
              <el-date-picker v-model="formData.publish_date" type="date" style="width: 100%" placeholder="选择日期"
                :clearable="true" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="摘要" prop="summary">
              <el-input v-model="formData.summary" type="textarea" :rows="2" :clearable="true" placeholder="请输入摘要" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="内容" prop="content">
              <el-input v-model="formData.content" type="textarea" :rows="4" :clearable="true" placeholder="请输入内容" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="来源" prop="source">
              <el-input v-model="formData.source" :clearable="true" placeholder="请输入来源" />
            </el-form-item>
          </el-col>

          <el-col :span="18">
            <el-form-item label="外链" prop="url">
              <el-input v-model="formData.url" :clearable="true" placeholder="请输入外链" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="排序" prop="sort">
              <el-input v-model.number="formData.sort" :clearable="true" placeholder="请输入排序" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="置顶" prop="top">
              <el-select v-model="formData.top" clearable placeholder="请选择置顶">
                <el-option v-for="(item, key) in whetherOptions" :key="key" :label="item.label"
                  :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="热门" prop="hot">
              <el-select v-model="formData.hot" clearable placeholder="请选择热门">
                <el-option v-for="(item, key) in whetherOptions" :key="key" :label="item.label"
                  :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="热度值" prop="heat">
              <el-input v-model.number="formData.heat" :clearable="true" placeholder="请输入热度值" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="浏览量" prop="view">
              <el-input v-model.number="formData.view" :clearable="true" placeholder="请输入浏览量" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="分享数" prop="share">
              <el-input v-model.number="formData.share" :clearable="true" placeholder="请输入分享数" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="状态" prop="status">
              <el-select v-model="formData.status" clearable placeholder="请选择状态">
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
import { ref, reactive, onUnmounted, watch } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';

import { createThemeTopic, deleteThemeTopic, deleteThemeTopicByIds, updateThemeTopic, findThemeTopic, getThemeTopicList } from '@/api/quant/themeTopic';
import { getThemeList } from '@/api/quant/theme';

defineOptions({
  name: 'ThemeTopic',
});
// 按钮权限实例化
const btnAuth = useBtnAuth();

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const statusOptions = ref([]);
const typeOptions = ref([]);
const sentimentOptions = ref([]);
const whetherOptions = ref([]);
const themeOptions = ref([]);
const themeTreeOptions = ref([]);
const themeLoading = ref(false);

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  theme_id: undefined,
  theme_name: '',
  title: '',
  type: 1,
  summary: '',
  content: '',
  sentiment: 0,
  source: '',
  url: '',
  publish_date: new Date(),
  publish_time: new Date(),
  sort: undefined,
  top: undefined,
  hot: undefined,
  heat: undefined,
  view: undefined,
  share: undefined,
  status: undefined,
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
  const table = await getThemeTopicList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
  [statusOptions.value, typeOptions.value, sentimentOptions.value, whetherOptions.value] = await Promise.all([getDictFunc('status'), getDictFunc('quant_theme_topic_type'), getDictFunc('quant_theme_topic_sentiment'), getDictFunc('whether')]);
};

const findThemeLabel = id => {
  const option = themeOptions.value.find(item => item.value === id);
  return option ? option.label : '';
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
    const flatten = [];
    const traverse = list => {
      list.forEach(item => {
        if (item && item.id !== undefined && item.name !== undefined) {
          flatten.push({ label: item.name, value: item.id });
        }
        if (item.children?.length) {
          traverse(item.children);
        }
      });
    };
    traverse(res.data);
    themeOptions.value = flatten;
    themeTreeOptions.value = buildTreeOptions(res.data);
  }
};

watch(
  () => formData.value.theme_id,
  value => {
    if (value !== undefined && value !== null) {
      const label = findThemeLabel(value);
      if (label) {
        formData.value.theme_name = label;
      }
    }
  }
);

// 获取需要的字典 可能为空 按需保留
setOptions();
loadThemeOptions();

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
    deleteThemeTopicFunc(row);
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
    const res = await deleteThemeTopicByIds({ ids });
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
const updateThemeTopicFunc = async row => {
  const res = await findThemeTopic({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    const option = themeOptions.value.find(item => item.value === formData.value.theme_id);
    if (option && !formData.value.theme_name) {
      formData.value.theme_name = option.label;
    }
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteThemeTopicFunc = async row => {
  const res = await deleteThemeTopic({ id: row.id });
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

// 重置表单为默认值（类型默认为 1，情绪默认为 0，发布日期默认为今天）
const initForm = () => {
  formData.value = {
    theme_id: undefined,
    theme_name: '',
    title: '',
    type: 1,
    summary: '',
    content: '',
    sentiment: 0,
    source: '',
    url: '',
    publish_date: new Date(),
    publish_time: new Date(),
    sort: undefined,
    top: undefined,
    hot: undefined,
    heat: undefined,
    view: undefined,
    share: undefined,
    status: undefined,
  };
};

// 打开弹窗
const openDialog = () => {
  type.value = 'create';
  initForm();
  dialogFormVisible.value = true;
};

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;
  initForm();
};
// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  elFormRef.value?.validate(async valid => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createThemeTopic(formData.value);
        break;
      case 'update':
        res = await updateThemeTopic(formData.value);
        break;
      default:
        res = await createThemeTopic(formData.value);
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

const getStatusType = status => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};
</script>

<style></style>
