<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" inline :model="searchInfo" class="demo-form-inline" :rules="searchRule" @keyup.enter="onSubmit">
        <el-form-item label="项目" prop="title">
          <el-input v-model="searchInfo.title" clearable placeholder="请输入项目关键词" />
        </el-form-item>

        <el-form-item label="阶段" prop="stage">
          <el-select v-model="searchInfo.stage" clearable placeholder="请选择阶段">
            <el-option v-for="(item, key) in stageOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item label="来源" prop="source">
          <el-select v-model="searchInfo.source" clearable placeholder="请选择来源">
            <el-option v-for="(item, key) in sourceOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
          <el-button link type="primary" icon="arrow-down" @click="showAllQuery = true" v-if="!showAllQuery">展开</el-button>
          <el-button link type="primary" icon="arrow-up" @click="showAllQuery = false" v-else>收起</el-button>
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="类型" prop="type">
            <el-select v-model="searchInfo.type" clearable placeholder="请选择类型">
              <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item>

          <!-- <el-form-item label="状态" prop="status">
            <el-select v-model="searchInfo.status" clearable placeholder="请选择状态">
              <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item> -->

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
        <el-table-column align="left" label="ID" prop="id" width="70" />

        <el-table-column align="left" label="项目名称" prop="title" min-width="240">
          <template #default="scope">
            <el-text class="text-content" v-html="highlightKeywords(scope.row.title, searchInfo.title)" />
          </template>
        </el-table-column>

        <el-table-column align="left" label="金额" prop="possibility" width="110">
          <template #default="scope"> ￥{{ scope.row.amount }} </template>
        </el-table-column>

        <el-table-column align="left" label="阶段" prop="stage" width="140">
          <template #default="scope">
            {{ filterDict(String(scope.row.stage), stageOptions) }}
          </template>
        </el-table-column>

        <el-table-column align="center" label="可能性" prop="possibility" width="90">
          <template #default="scope">
            <span> {{ scope.row.possibility }}% </span>
          </template>
        </el-table-column>

        <el-table-column align="left" label="成交日期" prop="close_date" width="110">
          <template #default="scope">
            {{ formatToDateTime(scope.row.close_date, 'YYYY-MM-DD') }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="联系日期" prop="date" width="150">
          <template #default="scope">
            <el-text tag="p">最近：{{ formatToDateTime(scope.row.last_date, 'YYYY-MM-DD') }}</el-text>
            <el-text tag="p">下次：{{ formatToDateTime(scope.row.next_date, 'YYYY-MM-DD') }}</el-text>
          </template>
        </el-table-column>

        <el-table-column align="left" label="来源" prop="source" width="90">
          <template #default="scope">
            {{ filterDict(String(scope.row.source), sourceOptions) }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="创建日期" prop="createdAt" width="120">
          <template #default="scope">{{ formatToDateTime(scope.row.created_at, 'YYYY-MM-DD') }}</template>
        </el-table-column>

        <!-- <el-table-column align="left" label="状态" prop="status" width="120">
            <template #default="scope">
              <el-tag type="info" effect="plain"> 
                {{ filterDict(String(scope.row.status), statusOptions) }}
              </el-tag>
            </template>
          </el-table-column> -->

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith - 80">
          <template #default="scope">
            <!-- <el-button type="primary" link class="table-button" @click="getDetails(scope.row)"><el-icon style="margin-right: 5px"><InfoFilled /></el-icon>查看</el-button> -->
            <el-button type="primary" link icon="edit" class="table-button" @click="updateDealFunc(scope.row)">编辑</el-button>
            <el-button type="primary" link icon="delete" @click="deleteRow(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
      </div>
    </div>

    <el-drawer destroy-on-close size="65%" v-model="dialogFormVisible" :show-close="false" :before-close="closeDialog">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">{{ type === 'create' ? '新增' : '编辑' }}</span>
          <div>
            <el-button :loading="btnLoading" type="primary" @click="enterDialog">确 定</el-button>
            <el-button @click="closeDialog">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form :model="formData" label-position="right" ref="elFormRef" :rules="rule" label-width="90px">
        <el-row>
          <el-col :span="12">
            <el-form-item label="项目名称" prop="title">
              <el-input v-model="formData.title" clearable placeholder="请输入项目名称" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="阶段" prop="stage">
              <el-select v-model="formData.stage" placeholder="请选择阶段">
                <el-option v-for="(item, key) in stageOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="金额" prop="amount">
              <el-input-number v-model="formData.amount" style="width: 100%" :precision="2" clearable />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="预计成交" prop="close_date">
              <el-date-picker v-model="formData.close_date" type="date" clearable placeholder="请输入成交日期" style="width: 100%" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="最近联系" prop="last_date">
              <el-date-picker v-model="formData.last_date" type="date" clearable placeholder="请输入最近联系日期" style="width: 100%" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="下次联系" prop="next_date">
              <el-date-picker v-model="formData.next_date" type="date" clearable placeholder="请输入下次联系日期" style="width: 100%" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="类型" prop="type">
              <el-select v-model="formData.type" placeholder="请选择类型">
                <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="来源" prop="source">
              <el-select v-model="formData.source" placeholder="请选择来源">
                <el-option v-for="(item, key) in sourceOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <!-- <el-form-item label="描述" prop="description" >
              <el-input v-model="formData.description" type="textarea" :rows="5" clearable  placeholder="请输入描述" />
            </el-form-item> -->
            <el-form-item label="描述" prop="description">
              <VditorEditor v-model="formData.description" :min-height="200" cache-id="unique-editor-id-description" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="备注" prop="remark">
              <VditorEditor v-model="formData.remark" :min-height="100" cache-id="unique-editor-id-remark" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="商户ID">
          {{ detailFrom.merchant_id }}
        </el-descriptions-item>
        <el-descriptions-item label="会员ID">
          {{ detailFrom.memberId }}
        </el-descriptions-item>
        <el-descriptions-item label="名称">
          {{ detailFrom.title }}
        </el-descriptions-item>
        <el-descriptions-item label="客户ID">
          {{ detailFrom.customer_id }}
        </el-descriptions-item>
        <el-descriptions-item label="金额">
          {{ detailFrom.amount }}
        </el-descriptions-item>
        <el-descriptions-item label="阶段">
          {{ detailFrom.stage }}
        </el-descriptions-item>
        <el-descriptions-item label="可能性">
          {{ detailFrom.possibility }}
        </el-descriptions-item>
        <el-descriptions-item label="描述">
          <RichView v-model="detailFrom.description" />
        </el-descriptions-item>
        <el-descriptions-item label="记录">
          {{ detailFrom.records }}
        </el-descriptions-item>
        <el-descriptions-item label="备注">
          <RichView v-model="detailFrom.remark" />
        </el-descriptions-item>
        <el-descriptions-item label="类型">
          {{ detailFrom.type }}
        </el-descriptions-item>
        <el-descriptions-item label="来源">
          {{ detailFrom.source }}
        </el-descriptions-item>
        <el-descriptions-item label="成交日期">
          {{ detailFrom.close_date }}
        </el-descriptions-item>
        <el-descriptions-item label="最近联系日期">
          {{ detailFrom.last_date }}
        </el-descriptions-item>
        <el-descriptions-item label="下次联系日期">
          {{ detailFrom.next_date }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ detailFrom.status }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { createDeal, deleteDeal, deleteDealByIds, updateDeal, findDeal, getDealList } from '@/api/cloud/deal';

import RichView from '@/components/richtext/rich-view.vue';
import VditorEditor from '@/components/vditorEditor/VditorEditor.vue';

// 全量引入格式化工具 请按需保留
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';

defineOptions({
  name: 'Deal',
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const typeOptions = ref();
const stageOptions = ref();
const sourceOptions = ref();
const statusOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  title: '',
  amount: 0,
  stage: undefined,
  possibility: undefined,
  description: '',
  records: {},
  remark: '',
  type: undefined,
  source: undefined,
  close_date: undefined,
  last_date: undefined,
  next_date: undefined,
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
  const table = await getDealList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
  typeOptions.value = await getDictFunc('cloud_deal_type');
  stageOptions.value = await getDictFunc('cloud_deal_stage');
  sourceOptions.value = await getDictFunc('cloud_source');
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
    deleteDealFunc(row);
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
    const res = await deleteDealByIds({ ids });
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
const updateDealFunc = async row => {
  const res = await findDeal({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteDealFunc = async row => {
  const res = await deleteDeal({ id: row.id });
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
    title: '',
    amount: 0,
    stage: 0,
    possibility: 0,
    description: '',
    records: {},
    remark: '',
    type: 0,
    source: 0,
    close_date: 0,
    last_date: 0,
    next_date: 0,
  };
};

// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  formData.value.type = parseInt(formData.value.type);
  formData.value.stage = parseInt(formData.value.stage);
  formData.value.source = parseInt(formData.value.source);

  elFormRef.value?.validate(async valid => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createDeal(formData.value);
        break;
      case 'update':
        res = await updateDeal(formData.value);
        break;
      default:
        res = await createDeal(formData.value);
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
  const res = await findDeal({ id: row.id });
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
