<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline"
        @keyup.enter="onSubmit">
        <el-form-item label="姓名" prop="name">
          <el-input v-model="searchInfo.name" placeholder="请输入姓名" />
        </el-form-item>

        <el-form-item label="手机" prop="mobile">
          <el-input v-model="searchInfo.mobile" placeholder="请输入手机" />
        </el-form-item>

        <el-form-item label="备注" prop="tips">
          <el-input v-model="searchInfo.tips" placeholder="请输入备注" />
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="类型" prop="type" >
            <el-select v-model="searchInfo.type" placeholder="请选择类型">
              <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item>

          <el-form-item label="业务" prop="business">
            <el-input v-model="searchInfo.business" placeholder="请输入业务" />
          </el-form-item>

          <el-form-item label="微信" prop="wechat">
            <el-input v-model="searchInfo.wechat" placeholder="请输入微信" />
          </el-form-item>

          <el-form-item label="抖音" prop="douyin">
            <el-input v-model="searchInfo.douyin" placeholder="请输入抖音" />
          </el-form-item>

          <el-form-item label="QQ" prop="qq">
            <el-input v-model="searchInfo.qq" placeholder="请输入QQ" />
          </el-form-item>

          <el-form-item label="来源 " prop="source" >
            <el-select v-model="searchInfo.source" placeholder="请选择来源">
              <el-option v-for="(item, key) in sourceOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item>

          <!-- <el-form-item label="状态 " prop="status" >
            <el-select v-model="searchInfo.status" placeholder="请选择状态">
              <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item> -->
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
        <el-button type="primary" icon="plus" @click="openDialog()">新增</el-button>
        <el-button icon="delete" style="margin-left: 10px;" :disabled="!multipleSelection.length"
          @click="onDelete">删除</el-button>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id"
        @selection-change="handleSelectionChange">
        <el-table-column align="center" type="selection" width="50" />

        <el-table-column align="left" label="ID" prop="id" width="70" />

        <el-table-column sortable align="left" label="类型" prop="type" width="100">
          <template #default="scope">
            <el-tag type="primary" effect="plain"> 
              {{ filterDict(String(scope.row.type), typeOptions) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="姓名" prop="name" width="120">
          <template #default="scope">
            <el-text tag="b">{{ scope.row.name }}</el-text>
          </template>
        </el-table-column>

        <el-table-column align="center" label="性别" prop="gender" width="90">
          <template #default="scope">
            <el-tag type="info" effect="plain"> 
              {{ filterDict(String(scope.row.gender), genderOptions) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column align="left" label="联系方式" prop="mobile" width="200">
          <template #default="scope">
            <el-text tag="p">手机：{{ scope.row.mobile }}</el-text>
            <el-text tag="p">微信：{{ scope.row.wechat }}</el-text>
            <el-text tag="p">抖音：{{ scope.row.doyin }}</el-text>
          </template>
        </el-table-column>

        <el-table-column align="left" label="城市" prop="mobile" width="80">
          <template #default="scope">
            <el-text tag="p">{{ scope.row.city }}</el-text>
            <el-text tag="p">{{ scope.row.district }}</el-text>
          </template>
        </el-table-column>

        <el-table-column align="left" label="业务" prop="business" width="120" />

        <el-table-column align="left" label="备注" prop="tips" width="100" />
       
        <el-table-column align="left" label="来源" prop="source" width="100">
          <template #default="scope">
            <el-tag type="info" effect="plain"> 
              {{ filterDict(String(scope.row.source), sourceOptions) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column sortable align="left" label="创建时间" prop="created_at" width="120">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>

        <!-- <el-table-column sortable align="left" label="状态" prop="status" width="90">
          <template #default="scope">
            <el-tag type="info" effect="plain"> 
              {{ filterDict(String(scope.row.status), statusOptions) }}
            </el-tag>
          </template>
        </el-table-column> -->

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith">
          <template #default="scope">
            <el-button type="primary" link icon="edit" class="table-button"
              @click="updateCustomerFunc(scope.row)">编辑</el-button>
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

    <el-drawer destroy-on-close size="75%" v-model="dialogFormVisible" :show-close="false"
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

      <el-form :model="formData" label-position="right" ref="elFormRef" :rules="rule" label-width="80px">
        <el-row>
          <el-col :span="6">
            <el-form-item label="类型:" prop="type">
              <el-input v-model.number="formData.type" :clearable="true" placeholder="请输入类型" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="姓名:" prop="name">
              <el-input v-model="formData.name" :clearable="true" placeholder="请输入姓名" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="性别:" prop="gender">
              <el-input v-model.number="formData.gender" :clearable="true" placeholder="请输入性别" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="备注:" prop="tips">
              <el-input v-model="formData.tips" :clearable="true" placeholder="请输入备注" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="手机:" prop="mobile">
          <el-input v-model="formData.mobile" :clearable="true" placeholder="请输入手机" />
        </el-form-item>
        
          </el-col>

          <el-col :span="6">
            <el-form-item label="微信:" prop="wechat">
              <el-input v-model="formData.wechat" :clearable="true" placeholder="请输入微信" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="抖音:" prop="douyin">
              <el-input v-model="formData.douyin" :clearable="true" placeholder="请输入抖音" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="QQ:" prop="qq">
              <el-input v-model="formData.qq" :clearable="true" placeholder="请输入QQ" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
             <el-form-item label="电子邮箱:" prop="email">
              <el-input v-model="formData.email" :clearable="true" placeholder="请输入电子邮箱" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="省份:" prop="province">
              <el-input v-model="formData.province" :clearable="true" placeholder="请输入省份" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="城市:" prop="city">
              <el-input v-model="formData.city" :clearable="true" placeholder="请输入城市" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="区县:" prop="district">
              <el-input v-model="formData.district" :clearable="true" placeholder="请输入区县" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="区域:" prop="area">
              <el-input v-model="formData.area" :clearable="true" placeholder="请输入区域" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="地址:" prop="address">
              <el-input v-model="formData.address" :clearable="true" placeholder="请输入地址" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="位置:" prop="location">
              <el-input v-model="formData.location" :clearable="true" placeholder="请输入位置" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="来源:" prop="source">
              <el-input v-model.number="formData.source" :clearable="true" placeholder="请输入来源" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="行业:" prop="industry">
              <el-input v-model="formData.industry" :clearable="true" placeholder="请输入行业" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="公司:" prop="company">
              <el-input v-model="formData.company" :clearable="true" placeholder="请输入公司" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="法人:" prop="legal">
              <el-switch v-model="formData.legal" active-color="#13ce66" inactive-color="#ff4949" active-text="是"
                inactive-text="否" clearable></el-switch>
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="职位:" prop="position">
              <el-input v-model="formData.position" :clearable="true" placeholder="请输入职位" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="部门:" prop="department">
              <el-input v-model="formData.department" :clearable="true" placeholder="请输入部门" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="业务:" prop="business">
              <el-input v-model="formData.business" :clearable="true" placeholder="请输入业务" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="最近联系日期:" prop="last_date">
              <el-input v-model.number="formData.last_date" :clearable="true" placeholder="请输入最近联系日期" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="下次联系日期:" prop="next_date">
              <el-input v-model.number="formData.next_date" :clearable="true" placeholder="请输入下次联系日期" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="出生:" prop="birth">
              <el-input v-model.number="formData.birth" :clearable="true" placeholder="请输入出生" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="等级:" prop="level">
              <el-input v-model.number="formData.level" :clearable="true" placeholder="请输入等级" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="电话:" prop="phone">
              <el-input v-model="formData.phone" :clearable="true" placeholder="请输入电话" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="网站:" prop="web">
              <el-input v-model="formData.web" :clearable="true" placeholder="请输入网站" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="省份ID:" prop="provinceId">
              <el-input v-model.number="formData.provinceId" :clearable="true" placeholder="请输入省份ID" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="城市ID:" prop="cityId">
              <el-input v-model.number="formData.cityId" :clearable="true" placeholder="请输入城市ID" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="区县ID:" prop="districtId">
              <el-input v-model.number="formData.districtId" :clearable="true" placeholder="请输入区县ID" />
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="坐标:" prop="coords">
              // 此字段为json结构，可以前端自行控制展示和数据绑定模式 需绑定json的key为 formData.coords 后端会按照json的类型进行存取
              {{ formData.coords }}
            </el-form-item>
          </el-col>

          <el-col :span="6">
            <el-form-item label="备注:" prop="remark">
              <VditorEditor v-model="formData.remark" height="200px" cache-id="unique-editor-id" />
            </el-form-item>
          </el-col>
        </el-row> 
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true"
      :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="商户ID">
          {{ detailFrom.merchant_id }}
        </el-descriptions-item>
        <el-descriptions-item label="会员ID">
          {{ detailFrom.memberId }}
        </el-descriptions-item>
        <el-descriptions-item label="类型">
          {{ detailFrom.type }}
        </el-descriptions-item>
        <el-descriptions-item label="姓名">
          {{ detailFrom.name }}
        </el-descriptions-item>
        <el-descriptions-item label="性别">
          {{ detailFrom.gender }}
        </el-descriptions-item>
        <el-descriptions-item label="等级">
          {{ detailFrom.level }}
        </el-descriptions-item>
        <el-descriptions-item label="备注">
          {{ detailFrom.tips }}
        </el-descriptions-item>
        <el-descriptions-item label="出生">
          {{ detailFrom.birth }}
        </el-descriptions-item>
        <el-descriptions-item label="法人">
          {{ detailFrom.legal }}
        </el-descriptions-item>
        <el-descriptions-item label="职位">
          {{ detailFrom.position }}
        </el-descriptions-item>
        <el-descriptions-item label="行业">
          {{ detailFrom.industry }}
        </el-descriptions-item>
        <el-descriptions-item label="公司">
          {{ detailFrom.company }}
        </el-descriptions-item>
        <el-descriptions-item label="部门">
          {{ detailFrom.department }}
        </el-descriptions-item>
        <el-descriptions-item label="业务">
          {{ detailFrom.business }}
        </el-descriptions-item>
        <el-descriptions-item label="电话">
          {{ detailFrom.phone }}
        </el-descriptions-item>
        <el-descriptions-item label="手机">
          {{ detailFrom.mobile }}
        </el-descriptions-item>
        <el-descriptions-item label="微信">
          {{ detailFrom.wechat }}
        </el-descriptions-item>
        <el-descriptions-item label="抖音">
          {{ detailFrom.douyin }}
        </el-descriptions-item>
        <el-descriptions-item label="QQ">
          {{ detailFrom.qq }}
        </el-descriptions-item>
        <el-descriptions-item label="电子邮箱">
          {{ detailFrom.email }}
        </el-descriptions-item>
        <el-descriptions-item label="网站">
          {{ detailFrom.web }}
        </el-descriptions-item>
        <el-descriptions-item label="来源">
          {{ detailFrom.source }}
        </el-descriptions-item>
        <el-descriptions-item label="最近联系日期">
          {{ detailFrom.last_date }}
        </el-descriptions-item>
        <el-descriptions-item label="下次联系日期">
          {{ detailFrom.next_date }}
        </el-descriptions-item>
        <el-descriptions-item label="出生省份ID">
          {{ detailFrom.rprProvinceId }}
        </el-descriptions-item>
        <el-descriptions-item label="出生城市ID">
          {{ detailFrom.rprCityId }}
        </el-descriptions-item>
        <el-descriptions-item label="出生区县ID">
          {{ detailFrom.rprDistrictId }}
        </el-descriptions-item>
        <el-descriptions-item label="常住省份ID">
          {{ detailFrom.clrProvinceId }}
        </el-descriptions-item>
        <el-descriptions-item label="常住城市ID">
          {{ detailFrom.clrCityId }}
        </el-descriptions-item>
        <el-descriptions-item label="常住区县ID">
          {{ detailFrom.clrDistrictId }}
        </el-descriptions-item>
        <el-descriptions-item label="省份ID">
          {{ detailFrom.provinceId }}
        </el-descriptions-item>
        <el-descriptions-item label="城市ID">
          {{ detailFrom.cityId }}
        </el-descriptions-item>
        <el-descriptions-item label="区县ID">
          {{ detailFrom.districtId }}
        </el-descriptions-item>
        <el-descriptions-item label="省份">
          {{ detailFrom.province }}
        </el-descriptions-item>
        <el-descriptions-item label="城市">
          {{ detailFrom.city }}
        </el-descriptions-item>
        <el-descriptions-item label="区县">
          {{ detailFrom.district }}
        </el-descriptions-item>
        <el-descriptions-item label="区域">
          {{ detailFrom.area }}
        </el-descriptions-item>
        <el-descriptions-item label="地址">
          {{ detailFrom.address }}
        </el-descriptions-item>
        <el-descriptions-item label="位置">
          {{ detailFrom.location }}
        </el-descriptions-item>
        <el-descriptions-item label="坐标">
          {{ detailFrom.coords }}
        </el-descriptions-item>
        <el-descriptions-item label="
备注">
          <RichView v-model="detailFrom.remark" />
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ detailFrom.status }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>

  </div>
</template>

<script setup>
import {
  createCustomer,
  deleteCustomer,
  deleteCustomerByIds,
  updateCustomer,
  findCustomer,
  getCustomerList
} from '@/api/cloud/customer'
// 富文本组件
// import RichEdit from '@/components/richtext/rich-edit.vue'
import RichView from '@/components/richtext/rich-view.vue'
import VditorEditor from '@/components/vditorEditor/VditorEditor.vue';

// 全量引入格式化工具 请按需保留
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ref, reactive } from 'vue'
import { useAppStore } from "@/pinia"




defineOptions({
  name: 'Customer'
})

// 提交按钮loading
const btnLoading = ref(false)
const appStore = useAppStore()

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false)
const typeOptions = ref();
const genderOptions = ref();
const sourceOptions = ref();
const statusOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  merchant_id: undefined,
  memberId: undefined,
  type: undefined,
  name: '',
  gender: undefined,
  level: undefined,
  tips: '',
  birth: undefined,
  legal: false,
  position: '',
  industry: '',
  company: '',
  department: '',
  business: '',
  phone: '',
  mobile: '',
  wechat: '',
  douyin: '',
  qq: '',
  email: '',
  web: '',
  source: undefined,
  last_date: undefined,
  next_date: undefined,
  rprProvinceId: undefined,
  rprCityId: undefined,
  rprDistrictId: undefined,
  clrProvinceId: undefined,
  clrCityId: undefined,
  clrDistrictId: undefined,
  provinceId: undefined,
  cityId: undefined,
  districtId: undefined,
  province: '',
  city: '',
  district: '',
  area: '',
  address: '',
  location: '',
  coords: {},
  remark: '',
  status: undefined,
})



// 验证规则
const rule = reactive({
  name: [{
    required: true,
    message: '',
    trigger: ['input', 'blur'],
  },
  {
    whitespace: true,
    message: '不能只输入空格',
    trigger: ['input', 'blur'],
  }
  ],
})

const elFormRef = ref()
const elSearchFormRef = ref()

// =========== 表格控制部分 ===========
const page = ref(1)
const total = ref(0)
const pageSize = ref(10)
const tableData = ref([])
const searchInfo = ref({})
// 重置
const onReset = () => {
  searchInfo.value = {}
  getTableData()
}

// 搜索
const onSubmit = () => {
  elSearchFormRef.value?.validate(async (valid) => {
    if (!valid) return
    page.value = 1
    if (searchInfo.value.legal === "") {
      searchInfo.value.legal = null
    }
    getTableData()
  })
}

// 分页
const handleSizeChange = (val) => {
  pageSize.value = val
  getTableData()
}

// 修改页面容量
const handleCurrentChange = (val) => {
  page.value = val
  getTableData()
}

// 查询
const getTableData = async () => {
  const table = await getCustomerList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value })
  if (table.code === 0) {
    tableData.value = table.data.list
    total.value = table.data.total
    page.value = table.data.page
    pageSize.value = table.data.pageSize
  }
}

getTableData()

// ============== 表格控制部分结束 ===============

// 获取需要的字典 可能为空 按需保留
const setOptions = async () => {
  typeOptions.value = await getDictFunc('cloud_customer_type');
  genderOptions.value = await getDictFunc('gender');
  sourceOptions.value = await getDictFunc('cloud_source'); 
  statusOptions.value = await getDictFunc('status'); 
}

// 获取需要的字典 可能为空 按需保留
setOptions()


// 多选数据
const multipleSelection = ref([])
// 多选
const handleSelectionChange = (val) => {
  multipleSelection.value = val
}

// 删除行
const deleteRow = (row) => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    deleteCustomerFunc(row)
  })
}

// 多选删除
const onDelete = async () => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    const ids = []
    if (multipleSelection.value.length === 0) {
      ElMessage({
        type: 'warning',
        message: '请选择要删除的数据'
      })
      return
    }
    multipleSelection.value &&
      multipleSelection.value.map(item => {
        ids.push(item.id)
      })
    const res = await deleteCustomerByIds({ ids })
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '删除成功'
      })
      if (tableData.value.length === ids.length && page.value > 1) {
        page.value--
      }
      getTableData()
    }
  })
}

// 行为控制标记（弹窗内部需要增还是改）
const type = ref('')

// 更新行
const updateCustomerFunc = async (row) => {
  const res = await findCustomer({ id: row.id })
  type.value = 'update'
  if (res.code === 0) {
    formData.value = res.data
    dialogFormVisible.value = true
  }
}


// 删除行
const deleteCustomerFunc = async (row) => {
  const res = await deleteCustomer({ id: row.id })
  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '删除成功'
    })
    if (tableData.value.length === 1 && page.value > 1) {
      page.value--
    }
    getTableData()
  }
}

// 弹窗控制标记
const dialogFormVisible = ref(false)

// 打开弹窗
const openDialog = () => {
  type.value = 'create'
  dialogFormVisible.value = true
}

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false
  formData.value = {
    merchant_id: undefined,
    memberId: undefined,
    type: undefined,
    name: '',
    gender: undefined,
    level: undefined,
    tips: '',
    birth: undefined,
    legal: false,
    position: '',
    industry: '',
    company: '',
    department: '',
    business: '',
    phone: '',
    mobile: '',
    wechat: '',
    douyin: '',
    qq: '',
    email: '',
    web: '',
    source: undefined,
    last_date: undefined,
    next_date: undefined,
    rprProvinceId: undefined,
    rprCityId: undefined,
    rprDistrictId: undefined,
    clrProvinceId: undefined,
    clrCityId: undefined,
    clrDistrictId: undefined,
    provinceId: undefined,
    cityId: undefined,
    districtId: undefined,
    province: '',
    city: '',
    district: '',
    area: '',
    address: '',
    location: '',
    coords: {},
    remark: '',
    status: undefined,
  }
}
// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true
  elFormRef.value?.validate(async (valid) => {
    if (!valid) return btnLoading.value = false
    let res
    switch (type.value) {
      case 'create':
        res = await createCustomer(formData.value)
        break
      case 'update':
        res = await updateCustomer(formData.value)
        break
      default:
        res = await createCustomer(formData.value)
        break
    }
    btnLoading.value = false
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '创建/更改成功'
      })
      closeDialog()
      getTableData()
    }
  })
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
  const res = await findCustomer({ id: row.id })
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
</script>

<style>
.el-table .el-table__row td .cell {
  line-height: 20px;
}
</style>
