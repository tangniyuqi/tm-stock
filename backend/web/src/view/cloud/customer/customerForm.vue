
<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="商户ID:" prop="merchant_id">
    <el-input v-model.number="formData.merchant_id" clearable placeholder="请输入商户ID" />
</el-form-item>
        <el-form-item label="会员ID:" prop="memberId">
    <el-input v-model.number="formData.memberId" clearable placeholder="请输入会员ID" />
</el-form-item>
        <el-form-item label="类型:" prop="type">
    <el-input v-model.number="formData.type" clearable placeholder="请输入类型" />
</el-form-item>
        <el-form-item label="姓名:" prop="name">
    <el-input v-model="formData.name" clearable placeholder="请输入姓名" />
</el-form-item>
        <el-form-item label="性别:" prop="gender">
    <el-input v-model.number="formData.gender" clearable placeholder="请输入性别" />
</el-form-item>
        <el-form-item label="等级:" prop="level">
    <el-input v-model.number="formData.level" clearable placeholder="请输入等级" />
</el-form-item>
        <el-form-item label="备注:" prop="tips">
    <el-input v-model="formData.tips" clearable placeholder="请输入备注" />
</el-form-item>
        <el-form-item label="出生:" prop="birth">
    <el-input v-model.number="formData.birth" clearable placeholder="请输入出生" />
</el-form-item>
        <el-form-item label="法人:" prop="legal">
    <el-switch v-model="formData.legal" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable ></el-switch>
</el-form-item>
        <el-form-item label="职位:" prop="position">
    <el-input v-model="formData.position" clearable placeholder="请输入职位" />
</el-form-item>
        <el-form-item label="行业:" prop="industry">
    <el-input v-model="formData.industry" clearable placeholder="请输入行业" />
</el-form-item>
        <el-form-item label="公司:" prop="company">
    <el-input v-model="formData.company" clearable placeholder="请输入公司" />
</el-form-item>
        <el-form-item label="部门:" prop="department">
    <el-input v-model="formData.department" clearable placeholder="请输入部门" />
</el-form-item>
        <el-form-item label="业务:" prop="business">
    <el-input v-model="formData.business" clearable placeholder="请输入业务" />
</el-form-item>
        <el-form-item label="电话:" prop="phone">
    <el-input v-model="formData.phone" clearable placeholder="请输入电话" />
</el-form-item>
        <el-form-item label="手机:" prop="mobile">
    <el-input v-model="formData.mobile" clearable placeholder="请输入手机" />
</el-form-item>
        <el-form-item label="微信:" prop="wechat">
    <el-input v-model="formData.wechat" clearable placeholder="请输入微信" />
</el-form-item>
        <el-form-item label="抖音:" prop="douyin">
    <el-input v-model="formData.douyin" clearable placeholder="请输入抖音" />
</el-form-item>
        <el-form-item label="QQ:" prop="qq">
    <el-input v-model="formData.qq" clearable placeholder="请输入QQ" />
</el-form-item>
        <el-form-item label="电子邮箱:" prop="email">
    <el-input v-model="formData.email" clearable placeholder="请输入电子邮箱" />
</el-form-item>
        <el-form-item label="网站:" prop="web">
    <el-input v-model="formData.web" clearable placeholder="请输入网站" />
</el-form-item>
        <el-form-item label="来源:" prop="source">
    <el-input v-model.number="formData.source" clearable placeholder="请输入来源" />
</el-form-item>
        <el-form-item label="最近联系日期:" prop="last_date">
    <el-input v-model.number="formData.last_date" clearable placeholder="请输入最近联系日期" />
</el-form-item>
        <el-form-item label="下次联系日期:" prop="next_date">
    <el-input v-model.number="formData.next_date" clearable placeholder="请输入下次联系日期" />
</el-form-item>
        <el-form-item label="出生省份ID:" prop="rprProvinceId">
    <el-input v-model.number="formData.rprProvinceId" clearable placeholder="请输入出生省份ID" />
</el-form-item>
        <el-form-item label="出生城市ID:" prop="rprCityId">
    <el-input v-model.number="formData.rprCityId" clearable placeholder="请输入出生城市ID" />
</el-form-item>
        <el-form-item label="出生区县ID:" prop="rprDistrictId">
    <el-input v-model.number="formData.rprDistrictId" clearable placeholder="请输入出生区县ID" />
</el-form-item>
        <el-form-item label="常住省份ID:" prop="clrProvinceId">
    <el-input v-model.number="formData.clrProvinceId" clearable placeholder="请输入常住省份ID" />
</el-form-item>
        <el-form-item label="常住城市ID:" prop="clrCityId">
    <el-input v-model.number="formData.clrCityId" clearable placeholder="请输入常住城市ID" />
</el-form-item>
        <el-form-item label="常住区县ID:" prop="clrDistrictId">
    <el-input v-model.number="formData.clrDistrictId" clearable placeholder="请输入常住区县ID" />
</el-form-item>
        <el-form-item label="省份ID:" prop="provinceId">
    <el-input v-model.number="formData.provinceId" clearable placeholder="请输入省份ID" />
</el-form-item>
        <el-form-item label="城市ID:" prop="cityId">
    <el-input v-model.number="formData.cityId" clearable placeholder="请输入城市ID" />
</el-form-item>
        <el-form-item label="区县ID:" prop="districtId">
    <el-input v-model.number="formData.districtId" clearable placeholder="请输入区县ID" />
</el-form-item>
        <el-form-item label="省份:" prop="province">
    <el-input v-model="formData.province" clearable placeholder="请输入省份" />
</el-form-item>
        <el-form-item label="城市:" prop="city">
    <el-input v-model="formData.city" clearable placeholder="请输入城市" />
</el-form-item>
        <el-form-item label="区县:" prop="district">
    <el-input v-model="formData.district" clearable placeholder="请输入区县" />
</el-form-item>
        <el-form-item label="区域:" prop="area">
    <el-input v-model="formData.area" clearable placeholder="请输入区域" />
</el-form-item>
        <el-form-item label="地址:" prop="address">
    <el-input v-model="formData.address" clearable placeholder="请输入地址" />
</el-form-item>
        <el-form-item label="位置:" prop="location">
    <el-input v-model="formData.location" clearable placeholder="请输入位置" />
</el-form-item>
        <el-form-item label="坐标:" prop="coords">
    // 此字段为json结构，可以前端自行控制展示和数据绑定模式 需绑定json的key为 formData.coords 后端会按照json的类型进行存取
    {{ formData.coords }}
</el-form-item>
        <el-form-item label="
备注:" prop="remark">
    <RichEdit v-model="formData.remark"/>
</el-form-item>
        <el-form-item label="状态:" prop="status">
    <el-input v-model.number="formData.status" clearable placeholder="请输入状态" />
</el-form-item>
        <el-form-item>
          <el-button :loading="btnLoading" type="primary" @click="save">保存</el-button>
          <el-button type="primary" @click="back">返回</el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import {
  createCustomer,
  updateCustomer,
  findCustomer
} from '@/api/cloud/customer'

defineOptions({
    name: 'CustomerForm'
})

// 自动获取字典
import { getDictFunc } from '@/utils/format'
import { useRoute, useRouter } from "vue-router"
import { ElMessage } from 'element-plus'
import { ref, reactive } from 'vue'
// 富文本组件
import RichEdit from '@/components/richtext/rich-edit.vue'


const route = useRoute()
const router = useRouter()

// 提交按钮loading
const btnLoading = ref(false)

const type = ref('')
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
               name : [{
                   required: true,
                   message: '',
                   trigger: ['input','blur'],
               }],
})

const elFormRef = ref()

// 初始化方法
const init = async () => {
 // 建议通过url传参获取目标数据ID 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
    if (route.query.id) {
      const res = await findCustomer({ ID: route.query.id })
      if (res.code === 0) {
        formData.value = res.data
        type.value = 'update'
      }
    } else {
      type.value = 'create'
    }
}

init()
// 保存按钮
const save = async() => {
      btnLoading.value = true
      elFormRef.value?.validate( async (valid) => {
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
           }
       })
}

// 返回按钮
const back = () => {
    router.go(-1)
}

</script>

<style>
</style>
