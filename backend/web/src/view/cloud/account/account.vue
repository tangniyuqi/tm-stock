<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" inline :model="searchInfo" class="demo-form-inline" :rules="searchRule" @keyup.enter="onSubmit">
        <el-form-item label="类型" prop="type">
          <el-select v-model="searchInfo.type" clearable placeholder="请选择类型">
            <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item label="名称" prop="title">
          <el-input v-model="searchInfo.title" clearable placeholder="请输入名称关键词" />
        </el-form-item>

        <el-form-item label="备注" prop="remark">
          <el-input v-model="searchInfo.remark" clearable placeholder="请输入备注关键词" />
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="结平" prop="balance">
            <el-switch v-model="searchInfo.balance" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable></el-switch>
          </el-form-item>

          <el-form-item label="审核" prop="review">
            <el-switch v-model="searchInfo.review" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable></el-switch>
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
      <div class="gva-btn-list">
        <el-button type="primary" icon="plus" @click="openDialog()">新增</el-button>
        <el-button icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
      </div>
      <el-table ref="multipleTable" show-summary sum-text="合计" tooltip-effect="dark" :data="tableData" row-key="id" style="width: 100%" @selection-change="handleSelectionChange">
        <el-table-column align="center" type="selection" width="60" />

        <!-- <el-table-column align="left" label="ID" prop="id" width="70" /> -->

        <el-table-column align="left" label="名称" prop="title" min-width="320">
          <template #default="scope">
            <el-text class="text-content" v-html="highlightKeywords(scope.row.title, searchInfo.title)" />
          </template>
        </el-table-column>

        <!-- <el-table-column align="left" label="类型" prop="type" width="90" /> -->

        <el-table-column align="left" label="金额" prop="amount" width="120">
          <template #default="scope">{{ scope.row.amount_in - scope.row.amount_out }}</template>
        </el-table-column>

        <el-table-column align="left" label="收入" prop="amount_in" width="120" />

        <el-table-column align="left" label="支出" prop="amount_out" width="120" />

        <el-table-column align="left" label="经办人" prop="transactor" width="120" />

        <el-table-column align="left" label="经办日期" prop="handling_date" width="120">
          <template #default="scope">{{ formatToDateTime(scope.row.handling_date, 'YYYY-MM-DD') }}</template>
        </el-table-column>

        <el-table-column align="center" label="结平" prop="balance" width="90">
          <template #default="scope">{{ formatBoolean(scope.row.balance) }}</template>
        </el-table-column>

        <el-table-column align="center" label="审核" prop="review" width="90">
          <template #default="scope">{{ formatBoolean(scope.row.review) }}</template>
        </el-table-column>

        <!-- <el-table-column sortable align="center" label="状态" prop="status" width="90">
          <template #default="scope">
            <el-tag type="info" effect="plain"> 
              {{ filterDict(String(scope.row.status), statusOptions) }}
            </el-tag>
          </template>
        </el-table-column> -->

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith - 80">
          <template #default="scope">
            <el-button type="primary" link icon="edit" class="table-button" @click="updateAccountFunc(scope.row)">编辑</el-button>
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

      <el-form ref="elFormRef" :model="formData" :rules="rule" label-position="right" label-width="90px">
        <el-row>
          <el-col :span="12">
            <el-form-item label="项目" prop="title">
              <el-input v-model="formData.title" clearable placeholder="请输入名称" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="类型" prop="type">
              <el-select v-model="formData.type" placeholder="请选择类型">
                <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <!-- <el-col :span="12">
            <el-form-item label="客户" prop="customer_id">
              <el-input v-model.number="formData.customer_id" clearable placeholder="请输入客户ID" />
            </el-form-item>
          </el-col> -->

          <el-col :span="12">
            <el-form-item label="收入" prop="amount_in">
              <el-input-number v-model="formData.amount_in" style="width: 100%" :precision="2" clearable />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="支出" prop="amount_out">
              <el-input-number v-model="formData.amount_out" style="width: 100%" :precision="2" clearable />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="账户类型" prop="account_type">
              <el-select v-model="formData.account_type" placeholder="请选择账户类型">
                <el-option v-for="(item, key) in accountTypeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="支付方式" prop="pay_mode">
              <el-select v-model="formData.pay_mode" placeholder="请选择支付方式">
                <el-option v-for="(item, key) in payModeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <!-- <el-col :span="12">
            <el-form-item label="付款人" prop="payer">
              <el-input v-model="formData.payer" clearable placeholder="请输入付款人" />
            </el-form-item>
          </el-col> -->

          <el-col :span="12">
            <el-form-item label="经办人" prop="transactor">
              <el-input v-model="formData.transactor" clearable placeholder="请输入经办人" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="经办日期" prop="handling_date">
              <el-date-picker v-model="formData.handling_date" type="date" clearable placeholder="请选择经办日期" style="width: 100%" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="凭证类型" prop="proof_type">
              <el-select v-model="formData.proof_type" placeholder="请选择凭证类型">
                <el-option v-for="(item, key) in proofTypeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="凭证编号" prop="proof_number">
              <el-input v-model="formData.proof_number" clearable placeholder="请输入凭证编号" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="备注" prop="remark">
              <VditorEditor v-model="formData.remark" :min-height="200" cache-id="unique-editor-id" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="结平" prop="balance">
              <el-switch v-model="formData.balance" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable></el-switch>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="审核" prop="review">
              <el-switch v-model="formData.review" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable></el-switch>
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
        <el-descriptions-item label="类型">
          {{ detailFrom.type }}
        </el-descriptions-item>
        <el-descriptions-item label="收入">
          {{ detailFrom.amount_in }}
        </el-descriptions-item>
        <el-descriptions-item label="支出">
          {{ detailFrom.amount_out }}
        </el-descriptions-item>
        <el-descriptions-item label="账户类型">
          {{ detailFrom.account_type }}
        </el-descriptions-item>
        <el-descriptions-item label="支付方式">
          {{ detailFrom.pay_mode }}
        </el-descriptions-item>
        <el-descriptions-item label="付款人">
          {{ detailFrom.payer }}
        </el-descriptions-item>
        <el-descriptions-item label="经办人">
          {{ detailFrom.transactor }}
        </el-descriptions-item>
        <el-descriptions-item label="经办日期">
          {{ detailFrom.handling_date }}
        </el-descriptions-item>
        <el-descriptions-item label="凭证类型">
          {{ detailFrom.proof_type }}
        </el-descriptions-item>
        <el-descriptions-item label="凭证编号">
          {{ detailFrom.proof_number }}
        </el-descriptions-item>
        <el-descriptions-item label="备注">
          <RichView v-model="detailFrom.remark" />
        </el-descriptions-item>
        <el-descriptions-item label="结平">
          {{ detailFrom.balance }}
        </el-descriptions-item>
        <el-descriptions-item label="审核">
          {{ detailFrom.review }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ detailFrom.status }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { createAccount, deleteAccount, deleteAccountByIds, updateAccount, findAccount, getAccountList } from '@/api/cloud/account';
// 富文本组件
// import RichEdit from '@/components/richtext/rich-edit.vue'
import RichView from '@/components/richtext/rich-view.vue';
import VditorEditor from '@/components/vditorEditor/VditorEditor.vue';

// 全量引入格式化工具 请按需保留
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime } from '@/utils/dateTimeUtils';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';

defineOptions({
  name: 'Account',
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const typeOptions = ref();
const accountTypeOptions = ref();
const payModeOptions = ref();
const proofTypeOptions = ref();
const statusOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  merchant_id: 0,
  memberId: 0,
  title: '',
  customer_id: 0,
  type: 0,
  amount_in: 0,
  amount_out: 0,
  account_type: 0,
  pay_mode: 0,
  payer: '',
  transactor: '',
  handling_date: new Date(),
  proof_type: 0,
  proof_number: '',
  remark: '',
  balance: 0,
  review: 0,
  status: 1,
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

const searchRule = reactive({});

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
    console.log('error submit!', valid);
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
  const table = await getAccountList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
  typeOptions.value = await getDictFunc('cloud_account_type');
  accountTypeOptions.value = await getDictFunc('cloud_account_account_type');
  payModeOptions.value = await getDictFunc('cloud_account_pay_mode');
  proofTypeOptions.value = await getDictFunc('cloud_account_proof_type');
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
    deleteAccountFunc(row);
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
    const res = await deleteAccountByIds({ ids });
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
const updateAccountFunc = async row => {
  const res = await findAccount({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteAccountFunc = async row => {
  const res = await deleteAccount({ id: row.id });
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
    memberId: 0,
    title: '',
    customer_id: 0,
    type: 0,
    amount_in: 0,
    amount_out: 0,
    account_type: 0,
    pay_mode: 0,
    payer: '',
    transactor: '',
    handling_date: new Date(),
    proof_type: 0,
    proof_number: '',
    remark: '',
    balance: 0,
    review: 0,
    status: 1,
  };
};
// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  formData.value.type = parseInt(formData.value.type);
  formData.value.account_type = parseInt(formData.value.account_type);
  formData.value.pay_mode = parseInt(formData.value.pay_mode);
  formData.value.proof_type = parseInt(formData.value.proof_type);

  elFormRef.value?.validate(async valid => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createAccount(formData.value);
        break;
      case 'update':
        res = await updateAccount(formData.value);
        break;
      default:
        res = await createAccount(formData.value);
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
  const res = await findAccount({ id: row.id });
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
