<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="名称" prop="name">
          <el-input v-model="searchInfo.name" clearable placeholder="请输入名称关键词" />
        </el-form-item>

        <el-form-item label="行业" prop="industry">
          <el-input v-model="searchInfo.industry" clearable placeholder="请输入行业关键词" />
        </el-form-item>

        <el-form-item label="日期" prop="date_range">
          <el-date-picker v-model="searchInfo.date_range" type="daterange" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间" style="width: 240px" />
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="等级" prop="level">
            <el-select v-model="searchInfo.level" placeholder="请选择等级">
              <el-option v-for="(item, key) in levelOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item>

          <el-form-item label="城市" prop="city">
            <el-input v-model="searchInfo.city" clearable placeholder="请输入城市" />
          </el-form-item>

          <el-form-item label="内容" prop="content">
            <el-input v-model="searchInfo.content" clearable placeholder="请输入内容关键词" />
          </el-form-item>

          <el-form-item label="备注" prop="remark">
            <el-input v-model="searchInfo.remark" clearable placeholder="请输入备注关键词" />
          </el-form-item>

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
        <el-table-column v-auth="btnAuth.batchDelete" align="center" type="selection" width="60" />

        <el-table-column align="center" label="ID" prop="id" width="70" />

        <el-table-column align="left" label="事件名称" prop="name" min-width="150">
          <template #default="scope">
            <el-text class="mr-1" v-html="`${highlightKeywords(scope.row.name || '无', searchInfo.name)}`" />

            <el-link icon="Search" :href="`//baidu.com/s?wd=${scope.row.name}`" target="_blank" class="ml-2" title="打开百度查询" />

            <el-link
              icon="ChatRound"
              :href="`https://www.doubao.com/chat/url-action?action=${encodeURIComponent(
                JSON.stringify({
                  pluginId: 'Send_Message',
                  payload: {
                    up_template_key: 'address_bar_up',
                    text: scope.row.name,
                    reportParams: { is_address: 1, scene: 'address_bar' },
                  },
                })
              )}`"
              target="_blank"
              class="ml-2"
              title="打开豆包对话"
            />
          </template>
        </el-table-column>

        <el-table-column align="left" label="事件日期" prop="date" width="120">
          <template #default="scope">{{ formatToDateTime(scope.row.date, 'YYYY-MM-DD') }}</template>
        </el-table-column>

        <el-table-column align="center" label="等级" prop="level" width="70">
          <template #default="scope">
            {{ filterDict(String(scope.row.level), levelOptions) }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="行业" prop="industry" width="150" />

        <el-table-column align="left" label="地点" prop="city" width="120" />

        <el-table-column align="center" label="内容" prop="content" width="60">
          <template #default="scope">
            <el-tooltip :content="scope.row.content" effect="dark" placement="top" v-if="scope.row.content">
              <el-icon><View /></el-icon>
            </el-tooltip>
          </template>
        </el-table-column>

        <el-table-column align="center" label="备注" prop="remark" width="60">
          <template #default="scope">
            <el-tooltip :content="scope.row.remark" effect="dark" placement="top" v-if="scope.row.remark">
              <el-icon><View /></el-icon>
            </el-tooltip>
          </template>
        </el-table-column>

        <el-table-column sortable align="left" label="入库时间" prop="created_at" width="120">
          <template #default="scope">{{ formatToDateTime(scope.row.created_at, 'MM-DD HH:mm') }}</template>
        </el-table-column>

        <!-- <el-table-column align="center" label="状态" prop="status" width="90">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">{{ filterDict(String(scope.row.status), statusOptions) }}</el-tag>
          </template>
        </el-table-column> -->

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith - 190">
          <template #default="scope">
            <!-- <el-button v-auth="btnAuth.info" type="primary" link class="table-button" @click="getDetails(scope.row)">
              <el-icon><InfoFilled /></el-icon>
            </el-button> -->
            <el-link icon="Search" class="mr-2" :href="`//baidu.com/s?wd=${scope.row.name}`" target="_blank"></el-link>
            <el-button v-auth="btnAuth.edit" type="primary" link icon="edit" class="table-button" @click="updateEventFunc(scope.row)"></el-button>
            <el-button v-auth="btnAuth.delete" link icon="delete" @click="deleteRow(scope.row)"></el-button>
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
            <el-form-item label="名称" prop="name">
              <el-input v-model="formData.name" :clearable="true" placeholder="请输入名称" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="日期" prop="date">
              <el-date-picker v-model="formData.date" type="date" style="width: 100%" placeholder="选择日期" :clearable="true" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="等级" prop="level">
              <el-select v-model="formData.level" placeholder="请选择等级">
                <el-option v-for="(item, key) in levelOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="行业" prop="industry">
              <el-input v-model="formData.industry" :clearable="true" placeholder="请输入行业" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="地点" prop="city">
              <el-input v-model="formData.city" :clearable="true" placeholder="请输入地点" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="内容" prop="content">
              <VditorEditor v-model="formData.content" :min-height="200" cache-id="unique-editor-id" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="备注" prop="remark">
              <VditorEditor v-model="formData.remark" :min-height="150" cache-id="unique-editor-id" />
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
        <el-descriptions-item label="行业">
          {{ detailForm.industry }}
        </el-descriptions-item>
        <el-descriptions-item label="日期">
          {{ detailForm.date }}
        </el-descriptions-item>
        <el-descriptions-item label="等级">
          {{ detailForm.level }}
        </el-descriptions-item>
        <el-descriptions-item label="摘要">
          {{ detailForm.summary }}
        </el-descriptions-item>
        <el-descriptions-item label="内容">
          {{ detailForm.content }}
        </el-descriptions-item>
        <el-descriptions-item label="备注">
          {{ detailForm.remark }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ detailForm.status }}
        </el-descriptions-item>
        <el-descriptions-item label="创建者">
          {{ detailForm.createdBy }}
        </el-descriptions-item>
        <el-descriptions-item label="更新者">
          {{ detailForm.updatedBy }}
        </el-descriptions-item>
        <el-descriptions-item label="删除者">
          {{ detailForm.deletedBy }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { ElMessage, ElMessageBox } from 'element-plus';
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import { createEvent, deleteEvent, deleteEventByIds, updateEvent, findEvent, getEventList } from '@/api/quant/event';
import RichView from '@/components/richtext/rich-view.vue';
import VditorEditor from '@/components/vditorEditor/VditorEditor.vue';

defineOptions({
  name: 'Event',
});

// 按钮权限实例化
const btnAuth = useBtnAuth();

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const levelOptions = ref();
const statusOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  name: '',
  industry: '',
  date: new Date(),
  level: undefined,
  summary: '',
  content: '',
  remark: '',
  status: undefined,
  createdBy: undefined,
  updatedBy: undefined,
  deletedBy: undefined,
});

// 验证规则
const rule = reactive({
  name: [
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

const initSearchInfo = () => {
  const start = new Date();
  start.setHours(0, 0, 0, 0);
  start.setDate(start.getDate() - 2); // 前天
  const end = new Date();
  end.setDate(end.getDate() + 365); // 未来365天
  searchInfo.value.date_range = [start, end];
  searchInfo.value.orderKey = 'date';
  searchInfo.value.orderDesc = false;
};
initSearchInfo();

// 重置
const onReset = () => {
  searchInfo.value = {};
  initSearchInfo();
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
  const params = {
    page: page.value,
    pageSize: pageSize.value,
    ...searchInfo.value,
  };

  if (!searchInfo.value.date_range) {
    params.orderDesc = true;
  }

  const table = await getEventList(params);
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
  levelOptions.value = await getDictFunc('quant_news_level');
  statusOptions.value = await getDictFunc('status');
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
    deleteEventFunc(row);
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
    const res = await deleteEventByIds({ ids });
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
const updateEventFunc = async row => {
  const res = await findEvent({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteEventFunc = async row => {
  const res = await deleteEvent({ id: row.id });
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
    name: '',
    industry: '',
    date: new Date(),
    level: undefined,
    summary: '',
    content: '',
    remark: '',
    status: undefined,
    createdBy: undefined,
    updatedBy: undefined,
    deletedBy: undefined,
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
        res = await createEvent(formData.value);
        break;
      case 'update':
        res = await updateEvent(formData.value);
        break;
      default:
        res = await createEvent(formData.value);
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
  const res = await findEvent({ id: row.id });
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

const getStatusType = status => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};
</script>

<style scoped>
.gva-table-box-head {
  display: flex;
  justify-content: space-between;

  .el-pagination {
    margin-top: 1em;
    margin-bottom: 2em;
  }
}
</style>
