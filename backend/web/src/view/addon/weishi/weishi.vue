
<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" :rules="searchRule" @change="onSubmit" @keyup.enter="onSubmit">
        <el-form-item label="账号" prop="account">
         <el-input v-model="searchInfo.account" placeholder="账号" />
        </el-form-item>

        <el-form-item label="用户ID" prop="person_id">
         <el-input v-model="searchInfo.person_id" placeholder="用户ID" />
        </el-form-item>
        
        <el-form-item label="状态" prop="status" >
          <el-select v-model="searchInfo.status" placeholder="请选择状态">
            <el-option v-for="(item, key) in weishi_statusOptions" :key="key" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>

        <template v-if="showAllQuery">
          <!-- 将需要控制显示状态的查询条件添加到此范围内 -->
        </template>

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
          <el-button link type="primary" icon="arrow-down" @click="showAllQuery=true" v-if="!showAllQuery">展开</el-button>
          <el-button link type="primary" icon="arrow-up" @click="showAllQuery=false" v-else>收起</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="gva-table-box">
        <div class="gva-btn-list">
            <el-button type="primary" icon="plus" @click="openDialog()">新增</el-button>
            <el-button type="primary" icon="import" @click="openBatchDrawer()">批量导入</el-button>
            <el-button type="danger" icon="delete" style="margin-left: 10px;" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
        </div>

        <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="ID" @sort-change="sortChange" @selection-change="handleSelectionChange">
          <el-table-column align="center" type="selection" width="60" />
          <el-table-column align="left" label="ID" prop="ID" width="90" sortable="custom" />
          <el-table-column align="left" label="账号" prop="account" width="90" />
          <el-table-column align="left" label="密码" prop="password" width="120" />
         <!--  <el-table-column align="center" label="授权类型" prop="iAuthType" width="90" />
          <el-table-column align="center" label="登录类型" prop="main_login" width="90" />
          <el-table-column align="left" label="OPENID" prop="openid" width="150" />
          <el-table-column align="left" label="会话密钥" prop="sSessionKey" width="300" /> -->
          <el-table-column align="left" label="用户ID" prop="person_id" width="180" />
          <el-table-column align="left" label="昵称" prop="nickname" width="120" />

          <el-table-column align="left" label="状态" prop="status" width="90">
            <template #default="scope">
              <el-tag :type="getStatusColor(scope.row.status)" effect="plain"> 
                {{ filterDict(String(scope.row.status), weishi_statusOptions) }}
              </el-tag>
            </template>
          </el-table-column>

          <el-table-column align="left" label="注销时间" width="180">
            <template #default="scope">
              <div v-if="scope.row.status == 0"> - </div>
              <div v-if="scope.row.status > 0">{{ formatToDateTime(scope.row.cancelled_at, 'YYYY-MM-DD HH:mm') }}</div>
              <div v-if="scope.row.status == 1">剩余：{{ getCountdownDays(scope.row.cancelled_at, 15) }}天</div>
            </template>
          </el-table-column>
        
          <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith">
            <template #default="scope">
              <el-button type="primary" link icon="Link" class="table-button" @click="copyText(scope.row.person_id)">链接</el-button>
              <el-button type="primary" link icon="Remove" class="table-button" @click="cancel(scope.row)">注销</el-button>
              <el-button type="primary" link class="table-button" @click="getDetails(scope.row)"><el-icon style="margin-right: 5px"><InfoFilled /></el-icon>查看</el-button>
              <el-button type="primary" link icon="edit" class="table-button" @click="updateWeishiFunc(scope.row)">编辑</el-button>
              <el-button type="primary" link icon="delete" @click="deleteRow(scope.row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="gva-pagination">
          <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
        </div>
    </div>

    <el-drawer destroy-on-close v-model="dialogFormVisible" :size="appStore.drawerSize" :show-close="false" :before-close="closeDialog">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">{{type==='create'?'新增':'编辑'}}</span>
          <div>
            <el-button :loading="btnLoading" type="primary" @click="enterDialog">确 定</el-button>
            <el-button @click="closeDialog">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form :model="formData" label-position="top" ref="elFormRef" :rules="rule" label-width="80px">
        <el-form-item label="账号:"  prop="account" >
          <el-input v-model="formData.account" clearable placeholder="请输入账号" />
        </el-form-item>
        <el-form-item label="密码:"  prop="password" >
          <el-input v-model="formData.password" clearable placeholder="请输入密码" />
        </el-form-item>
        <el-form-item label="授权类型:"  prop="iAuthType" >
          <el-input v-model.number="formData.iAuthType" clearable placeholder="请输入授权类型" />
        </el-form-item>
        <el-form-item label="登录类型:"  prop="main_login" >
          <el-input v-model="formData.main_login" clearable placeholder="请输入登录类型" />
        </el-form-item>
        <el-form-item label="OPENID:"  prop="openid" >
          <el-input v-model="formData.openid" clearable placeholder="请输入OPENID" />
        </el-form-item>
        <el-form-item label="会话密钥:"  prop="sSessionKey" >
          <el-input v-model="formData.sSessionKey" clearable placeholder="请输入会话密钥" />
        </el-form-item>
        <el-form-item label="用户ID:"  prop="person_id" >
          <el-input v-model="formData.person_id" clearable placeholder="请输入用户ID" />
        </el-form-item>
        <el-form-item label="昵称:"  prop="nickname" >
          <el-input v-model="formData.nickname" clearable placeholder="请输入昵称" />
        </el-form-item>
        <!-- <el-form-item label="状态:"  prop="status" >
          <el-select v-model="formData.status" clearable placeholder="请选择状态">
            <el-option v-for="(item, key) in weishi_statusOptions" :key="key" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item> -->
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close v-model="detailShow" :size="appStore.drawerSize" :show-close="true" :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="账号">
          {{ detailFrom.account }}
        </el-descriptions-item>
        <el-descriptions-item label="密码">
          {{ detailFrom.password }}
        </el-descriptions-item>
        <el-descriptions-item label="授权类型">
          {{ detailFrom.iAuthType }}
        </el-descriptions-item>
        <el-descriptions-item label="登录类型">
          {{ detailFrom.main_login }}
        </el-descriptions-item>
        <el-descriptions-item label="OPENID">
          {{ detailFrom.openid }}
        </el-descriptions-item>
        <el-descriptions-item label="会话密钥">
          {{ detailFrom.sSessionKey }}
        </el-descriptions-item>
        <el-descriptions-item label="用户ID">
          {{ detailFrom.person_id }}
        </el-descriptions-item>
        <el-descriptions-item label="昵称">
          {{ detailFrom.nickname }}
        </el-descriptions-item>
        <el-descriptions-item label="注销时间">
          <div v-if="detailFrom.status == 0"> - </div>
          <div v-if="detailFrom.status > 0">{{ formatToDateTime(detailFrom.cancelled_at, 'YYYY-MM-DD HH:mm') }}</div>
          <div v-if="detailFrom.status == 1">剩余：{{ getCountdownDays(detailFrom.cancelled_at, 15) }}天</div>
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ filterDict(String(detailFrom.status), weishi_statusOptions) }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
    
    <el-drawer destroy-on-close v-model="batchDrawerShow" :size="appStore.drawerSize" :show-close="false" :before-close="closeBatchDrawer">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">批量导入</span>
          <div>
            <el-button :loading="btnLoading" type="primary" @click="batchSubmit">导 入</el-button>
            <el-button @click="closeBatchDrawer">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form ref="batchFormRef" :model="batchFormData" :rules="batchRules" label-position="top" label-width="110px">
        <el-form-item label="数据内容:" prop="content">
          <el-input type="textarea" v-model="batchFormData.content" placeholder="请输入数据内容" :rows="18" clearable />
        </el-form-item>
      </el-form>

      <div class="usage-instructions bg-gray-100 border border-gray-300 rounded-lg p-4 mt-5">
        <h3 class="mb-3 text-lg text-gray-800">使用说明</h3>

        <p class="mb-2 text-sm text-gray-600">
          请注意数据内容的完整性、正确性等。
        </p>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import {
  createWeishi,
  batchCreateWeishi,
  deleteWeishi,
  deleteWeishiByIds,
  updateWeishi,
  cancelWeishi,
  findWeishi,
  getWeishiList
} from '@/api/addon/weishi'

// 全量引入格式化工具 请按需保留
import { getDictFunc, formatDate, getDaysDiff, formatBoolean, filterDict ,filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';

import { ElMessage, ElMessageBox } from 'element-plus';
import { toSQLLine } from '@/utils/stringFun';
import { ref, reactive } from 'vue';
import { useAppStore } from "@/pinia";
import { setSSRHandler } from '@vueuse/core';

defineOptions({
  name: 'Weishi'
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);

const weishi_statusOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  account: '',
  password: '',
  iAuthType: undefined,
  main_login: '',
  openid: '',
  sSessionKey: '',
  person_id: '',
  nickname: '',
  cancelled_at: '',
  status: '0',
});

const batchFormData = ref({
  content: '',
});

// 验证规则
const rule = reactive({
    account : [{
        required: true,
        message: '',
        trigger: ['input','blur'],
    },
    {
        whitespace: true,
        message: '不能只输入空格',
        trigger: ['input', 'blur'],
  }
  ],
    password : [{
        required: true,
        message: '',
        trigger: ['input','blur'],
    },
    {
        whitespace: true,
        message: '不能只输入空格',
        trigger: ['input', 'blur'],
  }
  ],
    iAuthType : [{
        required: true,
        message: '',
        trigger: ['input','blur'],
    },
  ],
    main_login : [{
        required: true,
        message: '',
        trigger: ['input','blur'],
    },
    {
        whitespace: true,
        message: '不能只输入空格',
        trigger: ['input', 'blur'],
  }
  ],
    openid : [{
        required: true,
        message: '',
        trigger: ['input','blur'],
    },
    {
        whitespace: true,
        message: '不能只输入空格',
        trigger: ['input', 'blur'],
  }
  ],
    sSessionKey : [{
        required: true,
        message: '',
        trigger: ['input','blur'],
    },
    {
        whitespace: true,
        message: '不能只输入空格',
        trigger: ['input', 'blur'],
  }
  ],
    person_id : [{
        required: true,
        message: '',
        trigger: ['input','blur'],
    },
    {
        whitespace: true,
        message: '不能只输入空格',
        trigger: ['input', 'blur'],
  }
  ],
    cancelled_at : [{
        required: true,
        message: '',
        trigger: ['input','blur'],
    },
  ],
    status : [{
        required: true,
        message: '',
        trigger: ['input','blur'],
    },
  ],
})

// 验证规则
const batchRules = reactive({
  content: [
    {
      required: true,
      message: '请输入数据内容',
      trigger: ['input','blur'],
    },
    {
      whitespace: true,
      message: '不能只输入空格',
      trigger: ['input', 'blur'],
    }
  ]
});

const searchRule = reactive({
  createdAt: [
    { validator: (rule, value, callback) => {
      if (searchInfo.value.startCreatedAt && !searchInfo.value.endCreatedAt) {
        callback(new Error('请填写结束日期'));
      } else if (!searchInfo.value.startCreatedAt && searchInfo.value.endCreatedAt) {
        callback(new Error('请填写开始日期'));
      } else if (searchInfo.value.startCreatedAt && searchInfo.value.endCreatedAt && (searchInfo.value.startCreatedAt.getTime() === searchInfo.value.endCreatedAt.getTime() || searchInfo.value.startCreatedAt.getTime() > searchInfo.value.endCreatedAt.getTime())) {
        callback(new Error('开始日期应当早于结束日期'));
      } else {
        callback();
      }
    }, trigger: 'change' }
  ],
})

const elFormRef = ref();
const elSearchFormRef = ref();
const batchFormRef = ref();

// =========== 表格控制部分 ===========
const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
const tableData = ref([]);
const searchInfo = ref({});

// 重置
const onReset = () => {
  searchInfo.value = {}
  getTableData();
}

// 搜索
const onSubmit = () => {
  elSearchFormRef.value?.validate(async(valid) => {
    if (!valid) return;
    page.value = 1;
    getTableData();
  });
}

// 分页
const handleSizeChange = (val) => {
  pageSize.value = val;
  getTableData();
}

// 修改页面容量
const handleCurrentChange = (val) => {
  page.value = val;
  getTableData();
}

// 排序
const sortChange = ({ prop, order }) => {
  if (prop) {
    if (prop === 'ID') {
      prop = 'id'
    }

    searchInfo.value.order = toSQLLine(prop);
    searchInfo.value.desc = order === 'descending';
  }

  getTableData();
}

// 查询
const getTableData = async() => {
  const table = await getWeishiList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });

  if (table.code === 0) {
    tableData.value = table.data.list;
    total.value = table.data.total;
    page.value = table.data.page;
    pageSize.value = table.data.pageSize;
  }
}

getTableData();

// ============== 表格控制部分结束 ===============

// 获取需要的字典 可能为空 按需保留
const setOptions = async () => {
  weishi_statusOptions.value = await getDictFunc('weishi_status');
}

// 获取需要的字典 可能为空 按需保留
setOptions();

// 多选数据
const multipleSelection = ref([]);

// 多选
const handleSelectionChange = (val) => {
  multipleSelection.value = val;
}

// 删除行
const deleteRow = (row) => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    deleteWeishiFunc(row);
  });
}

// 多选删除
const onDelete = async() => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async() => {
    const IDs = [];

    if (multipleSelection.value.length === 0) {
      ElMessage({
        type: 'warning',
        message: '请选择要删除的数据'
      });

      return;
    }

    multipleSelection.value && multipleSelection.value.map(item => {
      IDs.push(item.ID)
    })

    const res = await deleteWeishiByIds({ IDs });

    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '删除成功'
      });

      if (tableData.value.length === IDs.length && page.value > 1) {
        page.value--
      }

      getTableData();
    }
  });
}

// 行为控制标记（弹窗内部需要增还是改）
const type = ref('')

// 更新行
const updateWeishiFunc = async(row) => {
  const res = await findWeishi({ ID: row.ID });
  type.value = 'update';

  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
}

// 删除行
const deleteWeishiFunc = async (row) => {
  const res = await deleteWeishi({ ID: row.ID });

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
}

// 弹窗控制标记
const dialogFormVisible = ref(false);

// 打开弹窗
const openDialog = () => {
  type.value = 'create';
  dialogFormVisible.value = true;
}

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;

  formData.value = {
    account: '',
    password: '',
    iAuthType: 0,
    main_login: '',
    openid: '',
    sSessionKey: '',
    person_id: '',
    nickname: '',
    cancelled_at: '',
    status: '0',
  }
}

// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;

  elFormRef.value?.validate( async (valid) => {
    if (!valid) return btnLoading.value = false;
    let res;

    formData.value.status = Number(formData.value.status);

    switch (type.value) {
      case 'create':
        res = await createWeishi(formData.value);
        break
      case 'update':
        res = await updateWeishi(formData.value);
        break
      default:
        res = await createWeishi(formData.value);
        break
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
}

const detailFrom = ref({})

// 查看详情控制标记
const detailShow = ref(false)

// 打开详情弹窗
const openDetailShow = () => {
  detailShow.value = true
}

// 打开详情
const getDetails = async (row) => {
  // 打开弹窗
  const res = await findWeishi({ ID: row.ID })
  if (res.code === 0) {
    detailFrom.value = res.data
    openDetailShow()
  }
}

// 关闭详情弹窗
const closeDetailShow = () => {
  detailShow.value = false
  detailFrom.value = {}
}

// 复制链接
const copyText = (person_id) => {
  const baseUrl = 'https://isee.weishi.qq.com/ws/app-pages/wspersonal/index.html?_wv=1&id=';
  let url = baseUrl + person_id;
  url += '&spid=' + Date.now();

  try {
    navigator.clipboard.writeText(url);
    ElMessage({
      type: 'success',
      message: '主页链接已复制到剪贴板'
    });
  } catch (err) {
    console.error('复制失败: ', err);
  }
}

const batchDrawerShow = ref(false);

const openBatchDrawer = () => {
  batchDrawerShow.value = true;
  type.value = 'batchCreate';
}

const closeBatchDrawer = () => {
  batchDrawerShow.value = false;
  batchFormData.value = {};
}

// 提交导入内容
const batchSubmit = async () => {
  btnLoading.value = true;
  batchFormRef.value?.validate(async(valid) => {
    if (!valid) return btnLoading.value = false;
    let res = await batchCreateWeishi(batchFormData.value);
    btnLoading.value = false;

    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '批量创建成功!'
      });

      batchDrawerShow.value = false;
      batchFormData.value.content = '';
      getTableData();
    }
  });
}

// 注销账号
const cancel = async(row) => {
  ElMessageBox.confirm('确定要注销吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async() => {
    if (row.status > 0) {
      ElMessage({
        type: 'warning',
        message: '不能重复注销!'
      });

      return ;
    }

    let res = await cancelWeishi(row);

    if (res.code === 0 && res.data.ret === 0) {
      ElMessage({
        type: 'success',
        message: '已申请注销!'
      });

      getTableData();
    }
  });
}

const statusColorMap = ref({
  0: 'success',
  1: 'danger',
  2: 'info'
});

// 获取颜色
const getStatusColor = (status) => {
  return statusColorMap.value[status] || 'primary';
}
</script>

<style>

</style>
