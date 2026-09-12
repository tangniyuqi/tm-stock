<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" @keyup.enter="onSubmit">
        <el-form-item label="用户名" prop="name">
          <el-input v-model="searchInfo.name" clearable placeholder="请输入用户名" />
        </el-form-item>

        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="searchInfo.nickname" clearable placeholder="请输入昵称" />
        </el-form-item>

        <el-form-item label="手机号" prop="mobile">
          <el-input v-model="searchInfo.mobile" clearable placeholder="请输入手机号" />
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="邮箱" prop="email">
            <el-input v-model="searchInfo.email" clearable placeholder="请输入邮箱" />
          </el-form-item>

          <el-form-item label="状态" prop="status">
            <el-select v-model="searchInfo.status" clearable placeholder="请选择状态">
              <el-option label="正常" :value="1" />
              <el-option label="禁用" :value="0" />
            </el-select>
          </el-form-item>
        </template>

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
          <el-button v-if="!showAllQuery" link type="primary" icon="arrow-down" @click="showAllQuery = true">
            展开
          </el-button>
          <el-button v-else link type="primary" icon="arrow-up" @click="showAllQuery = false">
            收起
          </el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="gva-table-box">
      <div class="gva-btn-list">
        <el-button type="primary" icon="plus" @click="openDialog()">新增</el-button>
        <el-button icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">
          删除
        </el-button>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id"
        @selection-change="handleSelectionChange">
        <el-table-column type="selection" align="center" width="60" />

        <el-table-column align="left" label="ID" prop="id" width="90" />

        <el-table-column align="left" label="用户名" prop="name" width="120" />

        <el-table-column align="left" label="昵称" prop="nickname" min-width="140" />

        <el-table-column align="left" label="手机号" prop="mobile" width="140" />

        <el-table-column align="left" label="邮箱" prop="email" width="180" />

        <el-table-column align="left" label="等级" prop="level" width="80" />

        <el-table-column align="left" label="积分" prop="credits" width="100" />

        <el-table-column align="center" label="状态" prop="status" width="100">
          <template #default="scope">
            <el-tag :type="scope.row.status === 1 ? 'success' : 'danger'">
              {{ scope.row.status === 1 ? '正常' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column align="center" label="最近登录" prop="last_login_at" width="180">
          <template #default="scope">
            {{ formatDate(scope.row.last_login_at) }}
          </template>
        </el-table-column>

        <el-table-column align="center" label="创建时间" prop="created_at" width="180">
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
            <el-button type="primary" link icon="edit" class="table-button" @click="updateMemberFunc(scope.row)">
              编辑
            </el-button>
            <el-button type="primary" link icon="key" class="table-button" @click="openResetPwdDialog(scope.row)">
              重置密码
            </el-button>
            <el-button type="primary" link icon="delete" @click="deleteRow(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize"
          :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange"
          @size-change="handleSizeChange" />
      </div>
    </div>

    <!-- 新增/编辑弹窗 -->
    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="dialogFormVisible" :show-close="false"
      :before-close="closeDialog">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">{{ type === 'create' ? '新增' : '编辑' }}会员</span>
          <div>
            <el-button :loading="btnLoading" type="primary" @click="enterDialog">确 定</el-button>
            <el-button @click="closeDialog">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form :model="formData" label-position="top" ref="elFormRef" :rules="rule" label-width="80px">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="用户名" prop="name">
              <el-input v-model="formData.name" :clearable="true" placeholder="请输入用户名" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="昵称" prop="nickname">
              <el-input v-model="formData.nickname" :clearable="true" placeholder="请输入昵称" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="手机号" prop="mobile">
              <el-input v-model="formData.mobile" :clearable="true" placeholder="请输入手机号" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="邮箱" prop="email">
              <el-input v-model="formData.email" :clearable="true" placeholder="请输入邮箱" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="微信" prop="wechat">
              <el-input v-model="formData.wechat" :clearable="true" placeholder="请输入微信号" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="QQ" prop="qq">
              <el-input v-model="formData.qq" :clearable="true" placeholder="请输入QQ号" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="性别" prop="gender">
              <el-select v-model="formData.gender" clearable placeholder="请选择性别">
                <el-option label="未知" :value="0" />
                <el-option label="男" :value="1" />
                <el-option label="女" :value="2" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="等级" prop="level">
              <el-input-number v-model="formData.level" :min="0" :max="99" placeholder="等级" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="积分" prop="credits">
              <el-input-number v-model="formData.credits" :min="0" placeholder="积分" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="状态" prop="status">
              <el-select v-model="formData.status" placeholder="请选择状态">
                <el-option label="正常" :value="1" />
                <el-option label="禁用" :value="0" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="真实姓名" prop="realname">
              <el-input v-model="formData.realname" :clearable="true" placeholder="请输入真实姓名" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="个人简介" prop="bio">
              <el-input v-model="formData.bio" type="textarea" :rows="3" :clearable="true" placeholder="请输入个人简介" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <!-- 查看详情弹窗 -->
    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true"
      :before-close="closeDetailShow" title="查看会员">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="ID">
          {{ detailForm.id }}
        </el-descriptions-item>
        <el-descriptions-item label="用户名">
          {{ detailForm.name }}
        </el-descriptions-item>
        <el-descriptions-item label="昵称">
          {{ detailForm.nickname }}
        </el-descriptions-item>
        <el-descriptions-item label="手机号">
          {{ detailForm.mobile }}
        </el-descriptions-item>
        <el-descriptions-item label="邮箱">
          {{ detailForm.email }}
        </el-descriptions-item>
        <el-descriptions-item label="微信">
          {{ detailForm.wechat }}
        </el-descriptions-item>
        <el-descriptions-item label="QQ">
          {{ detailForm.qq }}
        </el-descriptions-item>
        <el-descriptions-item label="性别">
          {{ genderLabel(detailForm.gender) }}
        </el-descriptions-item>
        <el-descriptions-item label="等级">
          {{ detailForm.level }}
        </el-descriptions-item>
        <el-descriptions-item label="积分">
          {{ detailForm.credits }}
        </el-descriptions-item>
        <el-descriptions-item label="真实姓名">
          {{ detailForm.realname }}
        </el-descriptions-item>
        <el-descriptions-item label="个人简介">
          {{ detailForm.bio }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="detailForm.status === 1 ? 'success' : 'danger'">
            {{ detailForm.status === 1 ? '正常' : '禁用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="最近登录时间">
          {{ formatDate(detailForm.last_login_at) }}
        </el-descriptions-item>
        <el-descriptions-item label="创建时间">
          {{ formatDate(detailForm.created_at) }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>

    <!-- 重置密码弹窗 -->
    <el-dialog v-model="resetPwdVisible" title="重置密码" width="420px" :close-on-click-modal="false">
      <el-form :model="resetPwdForm" label-position="top" ref="resetPwdFormRef" :rules="resetPwdRule">
        <el-form-item label="会员">
          <el-input :model-value="resetPwdForm.nickname || resetPwdForm.mobile" disabled />
        </el-form-item>
        <el-form-item label="新密码" prop="password">
          <el-input v-model="resetPwdForm.password" type="password" show-password :clearable="true"
            placeholder="请输入新密码（至少6位）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetPwdVisible = false">取 消</el-button>
        <el-button type="primary" :loading="resetPwdLoading" @click="submitResetPwd">确 定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useAppStore } from '@/pinia'
import { ElMessage, ElMessageBox } from 'element-plus'
import { formatDate } from '@/utils/format'
import {
  createMember,
  deleteMember,
  deleteMemberByIds,
  updateMember,
  findMember,
  getMemberList,
  resetMemberPassword
} from '@/api/member/member'

defineOptions({
  name: 'Member'
})

const btnLoading = ref(false)
const appStore = useAppStore()

const showAllQuery = ref(false)

const formData = ref({
  name: '',
  nickname: '',
  mobile: '',
  email: '',
  wechat: '',
  qq: '',
  gender: 0,
  level: 0,
  credits: 0,
  status: 1,
  realname: '',
  bio: ''
})

const rule = reactive({
  nickname: [
    { required: true, message: '请输入昵称', trigger: ['input', 'blur'] }
  ],
  mobile: [
    { required: true, message: '请输入手机号', trigger: ['input', 'blur'] }
  ]
})

const elFormRef = ref()
const elSearchFormRef = ref()

// =========== 重置密码 ===========
const resetPwdVisible = ref(false)
const resetPwdLoading = ref(false)
const resetPwdFormRef = ref()
const resetPwdForm = ref({
  id: 0,
  nickname: '',
  mobile: '',
  password: ''
})

const resetPwdRule = reactive({
  password: [
    { required: true, message: '请输入新密码', trigger: ['input', 'blur'] },
    { min: 6, message: '密码至少6位', trigger: ['input', 'blur'] }
  ]
})

const openResetPwdDialog = (row) => {
  resetPwdForm.value = {
    id: row.id,
    nickname: row.name || row.nickname || '',
    mobile: row.mobile || '',
    password: ''
  }
  resetPwdVisible.value = true
}

const submitResetPwd = () => {
  resetPwdFormRef.value?.validate(async (valid) => {
    if (!valid) return
    resetPwdLoading.value = true
    const res = await resetMemberPassword({
      id: resetPwdForm.value.id,
      password: resetPwdForm.value.password
    })
    resetPwdLoading.value = false
    if (res.code === 0) {
      ElMessage({ type: 'success', message: '密码重置成功' })
      resetPwdVisible.value = false
    }
  })
}

// =========== 表格控制部分 ===========
const page = ref(1)
const total = ref(0)
const pageSize = ref(10)
const tableData = ref([])
const searchInfo = ref({})

const onReset = () => {
  searchInfo.value = {}
  getTableData()
}

const onSubmit = () => {
  page.value = 1
  getTableData()
}

const handleSizeChange = (val) => {
  pageSize.value = val
  getTableData()
}

const handleCurrentChange = (val) => {
  page.value = val
  getTableData()
}

const getTableData = async () => {
  const table = await getMemberList({
    page: page.value,
    pageSize: pageSize.value,
    ...searchInfo.value
  })
  if (table.code === 0) {
    tableData.value = table.data.list
    total.value = table.data.total
    page.value = table.data.page
    pageSize.value = table.data.pageSize
  }
}

getTableData()

// =========== 多选删除 ===========
const multipleSelection = ref([])

const handleSelectionChange = (val) => {
  multipleSelection.value = val
}

const deleteRow = (row) => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    deleteMemberFunc(row)
  })
}

const onDelete = async () => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    if (multipleSelection.value.length === 0) {
      ElMessage({ type: 'warning', message: '请选择要删除的数据' })
      return
    }
    const ids = multipleSelection.value.map((item) => item.id)
    const res = await deleteMemberByIds({ ids })
    if (res.code === 0) {
      ElMessage({ type: 'success', message: '删除成功' })
      if (tableData.value.length === ids.length && page.value > 1) {
        page.value--
      }
      getTableData()
    }
  })
}

// =========== 新增/编辑弹窗 ===========
const type = ref('')
const dialogFormVisible = ref(false)

const updateMemberFunc = async (row) => {
  const res = await findMember({ id: row.id })
  type.value = 'update'
  if (res.code === 0) {
    formData.value = {
      id: res.data.id,
      name: res.data.name || '',
      nickname: res.data.nickname || '',
      mobile: res.data.mobile || '',
      email: res.data.email || '',
      wechat: res.data.wechat || '',
      qq: res.data.qq || '',
      gender: res.data.gender || 0,
      level: res.data.level || 0,
      credits: res.data.credits || 0,
      status: res.data.status ?? 1,
      realname: res.data.realname || '',
      bio: res.data.bio || ''
    }
    dialogFormVisible.value = true
  }
}

const deleteMemberFunc = async (row) => {
  const res = await deleteMember({ id: row.id })
  if (res.code === 0) {
    ElMessage({ type: 'success', message: '删除成功' })
    if (tableData.value.length === 1 && page.value > 1) {
      page.value--
    }
    getTableData()
  }
}

const openDialog = () => {
  type.value = 'create'
  formData.value = {
    name: '',
    nickname: '',
    mobile: '',
    email: '',
    wechat: '',
    qq: '',
    gender: 0,
    level: 0,
    credits: 0,
    status: 1,
    realname: '',
    bio: ''
  }
  dialogFormVisible.value = true
}

const closeDialog = () => {
  dialogFormVisible.value = false
  elFormRef.value?.resetFields()
}

const enterDialog = async () => {
  btnLoading.value = true
  elFormRef.value?.validate(async (valid) => {
    if (!valid) return (btnLoading.value = false)
    let res
    switch (type.value) {
      case 'create':
        res = await createMember(formData.value)
        break
      case 'update':
        res = await updateMember(formData.value)
        break
      default:
        res = await createMember(formData.value)
        break
    }
    btnLoading.value = false
    if (res.code === 0) {
      ElMessage({ type: 'success', message: type.value === 'create' ? '创建成功' : '更新成功' })
      closeDialog()
      getTableData()
    }
  })
}

// =========== 查看详情 ===========
const detailForm = ref({})
const detailShow = ref(false)

const getDetails = async (row) => {
  const res = await findMember({ id: row.id })
  if (res.code === 0) {
    detailForm.value = res.data
    detailShow.value = true
  }
}

const closeDetailShow = () => {
  detailShow.value = false
  detailForm.value = {}
}

// =========== 工具函数 ===========
const genderLabel = (val) => {
  if (val === 1) return '男'
  if (val === 2) return '女'
  return '未知'
}
</script>

<style scoped></style>