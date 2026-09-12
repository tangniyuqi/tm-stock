<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" @keyup.enter="onSubmit">
        <el-form-item label="手机号" prop="mobile">
          <el-input v-model="searchInfo.mobile" clearable placeholder="请输入手机号" />
        </el-form-item>

        <el-form-item label="业务类型" prop="bizType">
          <el-select v-model="searchInfo.bizType" clearable placeholder="请选择业务类型">
            <el-option label="验证码登录" value="login" />
            <el-option label="注册" value="register" />
            <el-option label="找回密码" value="forgot" />
            <el-option label="绑定手机号" value="bind_mobile" />
          </el-select>
        </el-form-item>

        <el-form-item label="状态" prop="status">
          <el-select v-model="searchInfo.status" clearable placeholder="请选择状态">
            <el-option label="待验证" :value="0" />
            <el-option label="已用" :value="1" />
            <el-option label="已过期" :value="2" />
            <el-option label="发送失败" :value="3" />
          </el-select>
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="类型" prop="type">
            <el-select v-model="searchInfo.type" clearable placeholder="请选择类型">
              <el-option label="验证码" :value="1" />
              <el-option label="通知" :value="2" />
            </el-select>
          </el-form-item>

          <el-form-item label="发送时间" prop="sendTimeRange">
            <el-date-picker v-model="searchInfo.sendTimeRange" type="datetimerange" range-separator="至"
              start-placeholder="开始时间" end-placeholder="结束时间" value-format="YYYY-MM-DD HH:mm:ss" />
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
        <el-button icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">
          批量删除
        </el-button>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id"
        @selection-change="handleSelectionChange">
        <el-table-column type="selection" align="center" width="55" />

        <el-table-column align="left" label="ID" prop="id" width="80" />

        <el-table-column align="left" label="手机号" prop="mobile" width="130" />

        <el-table-column align="left" label="类型" prop="type" width="80">
          <template #default="scope">
            {{ scope.row.type === 1 ? '验证码' : '通知' }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="业务类型" prop="biz_type" width="110">
          <template #default="scope">
            {{ bizLabel(scope.row.biz_type) }}
          </template>
        </el-table-column>

        <el-table-column align="center" label="状态" prop="status" width="100">
          <template #default="scope">
            <el-tag :type="statusType(scope.row.status)">
              {{ statusLabel(scope.row.status) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="验证码" prop="code" width="90" />

        <el-table-column align="left" label="文本内容" prop="content" min-width="180" show-overflow-tooltip />

        <el-table-column align="left" label="IP" prop="ip" width="130" />

        <el-table-column align="left" label="失败次数" prop="fail_count" width="90" />

        <el-table-column align="center" label="发送时间" prop="send_time" width="170">
          <template #default="scope">{{ formatDate(scope.row.send_time) }}</template>
        </el-table-column>

        <el-table-column align="center" label="使用时间" prop="use_time" width="170">
          <template #default="scope">{{ formatDate(scope.row.use_time) }}</template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith">
          <template #default="scope">
            <el-button type="primary" link class="table-button" @click="getDetails(scope.row)">
              <el-icon style="margin-right: 5px">
                <InfoFilled />
              </el-icon>
              查看
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

    <!-- 查看详情弹窗 -->
    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true"
      :before-close="closeDetailShow" title="查看短信日志">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="ID">
          {{ detailForm.id }}
        </el-descriptions-item>
        <el-descriptions-item label="手机号">
          {{ detailForm.mobile }}
        </el-descriptions-item>
        <el-descriptions-item label="类型">
          {{ detailForm.type === 1 ? '验证码' : '通知' }}
        </el-descriptions-item>
        <el-descriptions-item label="业务类型">
          {{ bizLabel(detailForm.biz_type) }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="statusType(detailForm.status)">
            {{ statusLabel(detailForm.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="验证码">
          {{ detailForm.code }}
        </el-descriptions-item>
        <el-descriptions-item label="文本内容">
          {{ detailForm.content }}
        </el-descriptions-item>
        <el-descriptions-item label="业务关联ID">
          {{ detailForm.biz_id }}
        </el-descriptions-item>
        <el-descriptions-item label="IP">
          {{ detailForm.ip }}
        </el-descriptions-item>
        <el-descriptions-item label="返回消息ID">
          {{ detailForm.out_id }}
        </el-descriptions-item>
        <el-descriptions-item label="返回状态码">
          {{ detailForm.resp_code }}
        </el-descriptions-item>
        <el-descriptions-item label="返回错误描述">
          {{ detailForm.resp_msg }}
        </el-descriptions-item>
        <el-descriptions-item label="重试次数">
          {{ detailForm.retry_count }}
        </el-descriptions-item>
        <el-descriptions-item label="输错次数">
          {{ detailForm.fail_count }}
        </el-descriptions-item>
        <el-descriptions-item label="发送时间">
          {{ formatDate(detailForm.send_time) }}
        </el-descriptions-item>
        <el-descriptions-item label="使用时间">
          {{ formatDate(detailForm.use_time) }}
        </el-descriptions-item>
        <el-descriptions-item label="创建时间">
          {{ formatDate(detailForm.created_at) }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useAppStore } from '@/pinia'
import { ElMessage, ElMessageBox } from 'element-plus'
import { formatDate } from '@/utils/format'
import { getSmsLogList, deleteSmsLog, deleteSmsLogByIds } from '@/api/member/smsLog'

defineOptions({
  name: 'SmsLog'
})

const appStore = useAppStore()

const showAllQuery = ref(false)

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
  const table = await getSmsLogList({
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
    deleteSmsLogFunc(row)
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
    const res = await deleteSmsLogByIds({ ids })
    if (res.code === 0) {
      ElMessage({ type: 'success', message: '删除成功' })
      if (tableData.value.length === ids.length && page.value > 1) {
        page.value--
      }
      getTableData()
    }
  })
}

const deleteSmsLogFunc = async (row) => {
  const res = await deleteSmsLog({ id: row.id })
  if (res.code === 0) {
    ElMessage({ type: 'success', message: '删除成功' })
    if (tableData.value.length === 1 && page.value > 1) {
      page.value--
    }
    getTableData()
  }
}

// =========== 查看详情 ===========
const detailForm = ref({})
const detailShow = ref(false)

const getDetails = (row) => {
  detailForm.value = row
  detailShow.value = true
}

const closeDetailShow = () => {
  detailShow.value = false
  detailForm.value = {}
}

// =========== 工具函数 ===========
const bizLabel = (val) => {
  switch (val) {
    case 'login':
      return '验证码登录'
    case 'register':
      return '注册'
    case 'forgot':
      return '找回密码'
    case 'bind_mobile':
      return '绑定手机号'
    default:
      return val || '-'
  }
}

const statusLabel = (val) => {
  switch (val) {
    case 0:
      return '待验证'
    case 1:
      return '已用'
    case 2:
      return '已过期'
    case 3:
      return '发送失败'
    default:
      return '-'
  }
}

const statusType = (val) => {
  switch (val) {
    case 0:
      return 'primary'
    case 1:
      return 'success'
    case 2:
      return 'info'
    case 3:
      return 'danger'
    default:
      return 'info'
  }
}
</script>

<style scoped></style>
