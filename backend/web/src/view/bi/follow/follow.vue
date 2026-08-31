<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="跟进方式" prop="type">
          <el-select v-model="searchInfo.type" clearable placeholder="请选择跟进方式">
            <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item label="跟进内容" prop="content">
          <el-input v-model="searchInfo.content" placeholder="搜索条件" />
        </el-form-item>

        <el-form-item label="反馈类型" prop="feedback">
          <el-select v-model="searchInfo.feedback" clearable placeholder="请选择反馈类型">
            <el-option v-for="(item, key) in feedbackOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
          <el-button link type="primary" icon="arrow-down" @click="showAllQuery = true" v-if="!showAllQuery">展开</el-button>
          <el-button link type="primary" icon="arrow-up" @click="showAllQuery = false" v-else>收起</el-button>
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="商机ID" prop="deal_id">
            <el-input v-model.number="searchInfo.deal_id" placeholder="搜索条件" />
          </el-form-item>

          <el-form-item label="消息ID" prop="msg_id">
            <el-input v-model.number="searchInfo.msg_id" placeholder="搜索条件" />
          </el-form-item>

          <el-form-item label="用户ID" prop="member_id">
            <el-input v-model.number="searchInfo.member_id" placeholder="搜索条件" />
          </el-form-item>

          <el-form-item label="客户ID" prop="customer_id">
            <el-input v-model.number="searchInfo.customer_id" placeholder="搜索条件" />
          </el-form-item>

          <el-form-item label="下次跟进计划" prop="plan">
            <el-input v-model="searchInfo.plan" placeholder="搜索条件" />
          </el-form-item>

          <el-form-item label="原因" prop="reason">
            <el-input v-model="searchInfo.reason" placeholder="搜索条件" />
          </el-form-item>

          <el-form-item label="跟进时间" prop="follow_time">
            <template #label>
              <span>
                跟进时间
                <el-tooltip content="搜索范围是开始日期（包含）至结束日期（不包含）">
                  <el-icon>
                    <QuestionFilled />
                  </el-icon>
                </el-tooltip>
              </span>
            </template>

            <el-date-picker class="w-[380px]" v-model="searchInfo.follow_timeRange" type="datetimerange" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间"></el-date-picker>
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
        <el-table-column type="selection" align="center" width="60" />

        <el-table-column align="left" label="ID" prop="id" width="90" />

        <el-table-column align="left" label="消息ID" prop="msg_id" width="120">
          <template #default="scope">
            <el-button size="small" @click="pushTo('msg', { id: scope.row.msg_id })">{{ scope.row.msg_id }}</el-button>
          </template>
        </el-table-column>

        <!-- <el-table-column align="left" label="商机ID" prop="deal_id" width="120" />

        <el-table-column align="left" label="消息ID" prop="msg_id" width="120" />

        <el-table-column align="left" label="用户ID" prop="member_id" width="120" />

        <el-table-column align="left" label="客户ID" prop="customer_id" width="120" /> -->

        <el-table-column align="left" label="跟进时间" prop="follow_time" width="120">
          <template #default="scope">{{ formatDate(scope.row.follow_time) }}</template>
        </el-table-column>

        <el-table-column align="left" label="跟进方式" prop="type" width="90">
          <template #default="scope">
            <el-tag type="primary">{{ filterDict(String(scope.row.type), typeOptions) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="跟进内容" prop="content" min-width="240" />

        <el-table-column align="left" label="下次跟进计划" prop="plan" width="180" />

        <el-table-column align="left" label="反馈类型" prop="feedback" width="90">
          <template #default="scope">
            <el-tag type="warning">{{ filterDict(String(scope.row.feedback), feedbackOptions) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="原因" prop="reason" width="180" />

        <el-table-column sortable align="left" label="创建时间" prop="CreatedAt" width="120">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith">
          <template #default="scope">
            <el-button type="primary" link class="table-button" @click="pushTo('msg', { id: scope.row.msg_id })">
              <el-icon style="margin-right: 5px"><ChatLineRound /></el-icon>
              查看消息
            </el-button>

            <br />

            <el-button type="primary" link class="table-button" @click="getDetails(scope.row)">
              <el-icon style="margin-right: 5px"><InfoFilled /></el-icon>
              查看详情
            </el-button>

            <br />

            <el-button type="primary" link icon="edit" class="table-button" @click="updateFollowFunc(scope.row)">编辑跟进</el-button>
            <br />
            <el-button type="info" link icon="delete" @click="deleteRow(scope.row)">删除</el-button>
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

      <el-form ref="elFormRef" :model="formData" label-position="right" :rules="rule" label-width="100px" @keyup.enter="handleEnter">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="商机ID" prop="deal_id">
              <el-input v-model.number="formData.deal_id" clearable placeholder="请输入商机ID" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="消息ID" prop="msg_id">
              <el-input v-model.number="formData.msg_id" clearable placeholder="请输入消息ID" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="用户ID" prop="member_id">
              <el-input v-model.number="formData.member_id" clearable placeholder="请输入用户ID" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="客户ID" prop="customer_id">
              <el-input v-model.number="formData.customer_id" clearable placeholder="请输入客户ID" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="跟进时间" prop="follow_time">
              <el-date-picker v-model="formData.follow_time" type="date" style="width: 100%" placeholder="选择日期" clearable />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="跟进方式" prop="type">
              <el-select v-model.number="formData.type" placeholder="请选择跟进方式">
                <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="跟进内容" prop="content">
              <el-input v-model.trim="formData.content" type="textarea" :autosize="{ minRows: 6, maxRows: 12 }" placeholder="请输入跟进内容" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="下次跟进计划" prop="plan">
              <el-input v-model.trim="formData.plan" type="textarea" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="请输入下次跟进计划" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="反馈类型" prop="feedback">
              <el-select v-model="formData.feedback" placeholder="请选择反馈类型">
                <el-option v-for="(item, key) in feedbackOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="原因" prop="reason">
              <el-input v-model.trim="formData.reason" type="textarea" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="请输入原因" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="商机ID">
          {{ detailForm.deal_id }}
        </el-descriptions-item>
        <el-descriptions-item label="消息ID">
          {{ detailForm.msg_id }}
        </el-descriptions-item>
        <el-descriptions-item label="用户ID">
          {{ detailForm.member_id }}
        </el-descriptions-item>
        <el-descriptions-item label="客户ID">
          {{ detailForm.customer_id }}
        </el-descriptions-item>
        <el-descriptions-item label="跟进时间">
          {{ detailForm.follow_time }}
        </el-descriptions-item>
        <el-descriptions-item label="跟进方式">
          {{ detailForm.type }}
        </el-descriptions-item>
        <el-descriptions-item label="跟进内容">
          {{ detailForm.content }}
        </el-descriptions-item>
        <el-descriptions-item label="下次跟进计划">
          {{ detailForm.plan }}
        </el-descriptions-item>
        <el-descriptions-item label="反馈类型">
          {{ detailForm.feedback }}
        </el-descriptions-item>
        <el-descriptions-item label="原因">
          {{ detailForm.reason }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { createFollow, deleteFollow, deleteFollowByIds, updateFollow, findFollow, getFollowList } from '@/api/bi/follow';

defineOptions({
  name: 'Follow'
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const typeOptions = ref();
const feedbackOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  deal_id: '',
  msg_id: '',
  member_id: '',
  customer_id: '',
  follow_time: new Date(),
  type: 1,
  content: '',
  plan: '',
  feedback: 1,
  reason: ''
});

// 验证规则
const rule = reactive({
  content: [
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

const route = useRoute();
const router = useRouter();
const elFormRef = ref();
const elSearchFormRef = ref();

// =========== 表格控制部分 ===========
const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
const tableData = ref([]);
const searchInfo = ref({});
searchInfo.value.msg_id = route.query.msg_id;

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
  const table = await getFollowList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });

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
  typeOptions.value = await getDictFunc('bi_im_follow_type');
  feedbackOptions.value = await getDictFunc('bi_im_follow_feedback');
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
    deleteFollowFunc(row);
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

    const res = await deleteFollowByIds({ ids });

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
const updateFollowFunc = async (row) => {
  const res = await findFollow({ id: row.id });
  type.value = 'update';

  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteFollowFunc = async (row) => {
  const res = await deleteFollow({ id: row.id });

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

const pushTo = (name, query) => {
  if (query) {
    router.push({
      name: name,
      query: query
    });
  } else {
    router.push({ name: name });
  }
};

// 弹窗控制标记
const dialogFormVisible = ref(false);

// 打开弹窗
const openDialog = () => {
  type.value = 'create';
  dialogFormVisible.value = true;
  formData.value.msg_id = route.query.msg_id;
};

const init = () => {
  let msg_id = route.query.msg_id;
  let client_id = route.query.client_id;

  if (msg_id && client_id) {
    formData.value.msg_id = msg_id;
    formData.value.client_id = client_id;
    openDialog();
  }
};

init();

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;

  formData.value = {
    deal_id: '',
    msg_id: '',
    member_id: '',
    customer_id: '',
    follow_time: new Date(),
    type: 1,
    content: '',
    plan: '',
    feedback: 1,
    reason: ''
  };
};

// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  formData.value.deal_id = Number(formData.value.deal_id);
  formData.value.msg_id = Number(formData.value.msg_id);
  formData.value.member_id = Number(formData.value.member_id);
  formData.value.customer_id = Number(formData.value.customer_id);

  elFormRef.value?.validate(async (valid) => {
    if (!valid) return (btnLoading.value = false);
    let res;

    switch (type.value) {
      case 'create':
        res = await createFollow(formData.value);
        break;
      case 'update':
        res = await updateFollow(formData.value);
        break;
      default:
        res = await createFollow(formData.value);
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

const detailForm = ref({});

// 查看详情控制标记
const detailShow = ref(false);

// 打开详情弹窗
const openDetailShow = () => {
  detailShow.value = true;
};

// 打开详情
const getDetails = async (row) => {
  const res = await findFollow({ id: row.id });

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

<style></style>
