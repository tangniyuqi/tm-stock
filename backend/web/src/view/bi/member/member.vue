<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="姓名" prop="name">
          <el-input v-model="searchInfo.name" clearable placeholder="请输入姓名" />
        </el-form-item>

        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="searchInfo.nickname" clearable placeholder="请输入昵称" />
        </el-form-item>

        <el-form-item label="手机" prop="mobile">
          <el-input v-model="searchInfo.mobile" clearable placeholder="请输入手机" />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
          <el-button link type="primary" icon="arrow-down" @click="showAllQuery = true" v-if="!showAllQuery">展开</el-button>
          <el-button link type="primary" icon="arrow-up" @click="showAllQuery = false" v-else>收起</el-button>
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="微信" prop="wechat">
            <el-input v-model="searchInfo.wechat" clearable placeholder="请输入微信" />
          </el-form-item>

          <el-form-item label="业务" prop="business">
            <el-input v-model="searchInfo.business" clearable placeholder="请输入业务关键词" />
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

            <el-date-picker v-model="searchInfo.createdAtRange" class="w-[380px]" type="datetimerange" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间" />
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
        <el-table-column type="selection" align="center" width="30" />

        <el-table-column align="left" label="ID" prop="id" width="70" />

        <el-table-column align="left" label="基本信息" prop="sender" width="180">
          <template #default="scope">
            <div class="cell-style">
              <el-text tag="p">客户端：{{ scope.row.client_id }}</el-text>
              <el-text tag="p">消息ID：{{ scope.row.msg_id }}</el-text>
            </div>
          </template>
        </el-table-column>

        <el-table-column align="left" label="昵称" prop="nickname" width="120" />

        <el-table-column align="left" label="联系方式" prop="sender" width="120">
          <template #default="scope">
            <div class="cell-style">
              <el-text tag="p">{{ scope.row.name }}</el-text>
              <el-text tag="p">{{ scope.row.mobile }}</el-text>
              <el-text tag="p">{{ scope.row.wechat }}</el-text>
            </div>
          </template>
        </el-table-column>

        <el-table-column align="left" label="城市" prop="city" width="120" />

        <el-table-column align="left" label="业务" prop="business" width="150" />

        <el-table-column align="left" label="备注" prop="remark" width="120" />

        <el-table-column sortable align="left" label="日期" prop="created_at" width="120">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith">
          <template #default="scope">
            <el-button type="primary" link class="table-button" @click="getDetails(scope.row)">
              <el-icon style="margin-right: 5px">
                <InfoFilled />
              </el-icon>
              查看
            </el-button>
            <el-button type="primary" link icon="edit" class="table-button" @click="updateMemberFunc(scope.row)">编辑</el-button>
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

      <el-form ref="elFormRef" :model="formData" label-position="right" :rules="rule" label-width="80px" @keyup.enter="handleEnter">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="客户端" prop="client_id">
              <el-input v-model.number="formData.client_id" clearable placeholder="请输入客户端ID" />
            </el-form-item>
          </el-col>

          <el-col :span="8">
            <el-form-item label="消息ID" prop="msg_id">
              <el-input v-model.number="formData.msg_id" clearable placeholder="请输入消息ID" />
            </el-form-item>
          </el-col>

          <el-col :span="4">
            <el-input v-model.number="formData.group_id" clearable placeholder="请输入群组ID" />
          </el-col>

          <!-- <el-col :span="12">
            <el-form-item label="GID" prop="gid">
              <el-input v-model.trim="formData.gid" clearable placeholder="请输入GID" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="UID" prop="uid">
              <el-input v-model.trim="formData.uid" clearable placeholder="请输入UID" />
            </el-form-item>
          </el-col> -->

          <el-col :span="12">
            <el-form-item label="姓名" prop="name">
              <el-input v-model.trim="formData.name" clearable placeholder="请输入姓名" />
            </el-form-item>
          </el-col>

          <el-col :span="8">
            <el-form-item label="昵称" prop="nickname">
              <el-input v-model.trim="formData.nickname" clearable placeholder="请输入昵称" />
            </el-form-item>
          </el-col>

          <el-col :span="4">
            <el-input v-model.trim="formData.city" clearable placeholder="请输入城市" />
          </el-col>

          <el-col :span="12">
            <el-form-item label="手机" prop="mobile">
              <el-input v-model.trim="formData.mobile" clearable placeholder="请输入手机" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="微信" prop="wechat">
              <el-input v-model.trim="formData.wechat" clearable placeholder="请输入微信" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="业务" prop="business">
              <el-input v-model.trim="formData.business" clearable placeholder="请输入业务" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="备注" prop="remark">
              <el-input v-model.trim="formData.remark" type="textarea" :autosize="{ minRows: 6, maxRows: 12 }" placeholder="请输入备注" />
            </el-form-item>
          </el-col>

          <!-- <el-col :span="12">
            <el-form-item label="状态" prop="status">
              <el-select v-model="formData.status" placeholder="请选择状态">
                <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col> -->
        </el-row>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="客户端ID">
          {{ detailForm.client_id }}
        </el-descriptions-item>
        <el-descriptions-item label="群组ID">
          {{ detailForm.group_id }}
        </el-descriptions-item>
        <el-descriptions-item label="GID">
          {{ detailForm.gid }}
        </el-descriptions-item>
        <el-descriptions-item label="UID">
          {{ detailForm.uid }}
        </el-descriptions-item>
        <el-descriptions-item label="头像">
          {{ detailForm.avatar }}
        </el-descriptions-item>
        <el-descriptions-item label="昵称">
          {{ detailForm.nickname }}
        </el-descriptions-item>
        <el-descriptions-item label="姓名">
          {{ detailForm.name }}
        </el-descriptions-item>
        <el-descriptions-item label="业务">
          {{ detailForm.business }}
        </el-descriptions-item>
        <el-descriptions-item label="手机">
          {{ detailForm.mobile }}
        </el-descriptions-item>
        <el-descriptions-item label="微信">
          {{ detailForm.wechat }}
        </el-descriptions-item>
        <el-descriptions-item label="QQ">
          {{ detailForm.qq }}
        </el-descriptions-item>
        <el-descriptions-item label="抖音">
          {{ detailForm.douyin }}
        </el-descriptions-item>
        <el-descriptions-item label="TikTok">
          {{ detailForm.tiktok }}
        </el-descriptions-item>
        <el-descriptions-item label="城市">
          {{ detailForm.city }}
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
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { createMember, deleteMember, deleteMemberByIds, updateMember, findMember, getMemberList } from '@/api/bi/member';

defineOptions({
  name: 'Member'
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  client_id: '',
  group_id: '',
  gid: '',
  uid: '',
  avatar: '',
  nickname: '',
  name: '',
  business: '',
  mobile: '',
  wechat: '',
  qq: '',
  douyin: '',
  tiktok: '',
  city: '',
  remark: '',
  status: ''
});

// 验证规则
const rule = reactive({});

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
  const table = await getMemberList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
const setOptions = async () => {};

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
    deleteMemberFunc(row);
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
    const res = await deleteMemberByIds({ ids });
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
const updateMemberFunc = async (row) => {
  const res = await findMember({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteMemberFunc = async (row) => {
  const res = await deleteMember({ id: row.id });
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

const init = () => {
  let msg_id = route.query.msg_id;
  let client_id = route.query.client_id;
  let name = route.query.name;
  let nickname = route.query.nickname;

  if (msg_id && client_id) {
    formData.value.msg_id = msg_id;
    formData.value.client_id = client_id;
    formData.value.name = name;
    formData.value.nickname = nickname;

    openDialog();
  }
};

init();

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;
  formData.value = {
    client_id: '',
    group_id: '',
    gid: '',
    uid: '',
    avatar: '',
    nickname: '',
    name: '',
    business: '',
    mobile: '',
    wechat: '',
    qq: '',
    douyin: '',
    tiktok: '',
    city: '',
    remark: '',
    status: ''
  };
};

// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  formData.value.client_id = Number(formData.value.client_id);
  formData.value.group_id = Number(formData.value.group_id);
  formData.value.msg_id = Number(formData.value.msg_id);
  formData.value.status = Number(formData.value.status);

  elFormRef.value?.validate(async (valid) => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createMember(formData.value);
        break;
      case 'update':
        res = await updateMember(formData.value);
        break;
      default:
        res = await createMember(formData.value);
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
  // 打开弹窗
  const res = await findMember({ id: row.id });
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
