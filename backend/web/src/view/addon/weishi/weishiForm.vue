
<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="账号:" prop="account">
          <el-input v-model="formData.account" :clearable="true"  placeholder="请输入账号" />
        </el-form-item>
        <el-form-item label="密码:" prop="password">
          <el-input v-model="formData.password" :clearable="true"  placeholder="请输入密码" />
        </el-form-item>
        <el-form-item label="授权类型:" prop="iAuthType">
          <el-input v-model.number="formData.iAuthType" :clearable="true" placeholder="请输入" />
        </el-form-item>
        <el-form-item label="登录类型:" prop="main_login">
          <el-input v-model="formData.main_login" :clearable="true"  placeholder="请输入登录类型" />
        </el-form-item>
        <el-form-item label="OPENID:" prop="openid">
          <el-input v-model="formData.openid" :clearable="true"  placeholder="请输入OPENID" />
        </el-form-item>
        <el-form-item label="会话密钥:" prop="sSessionKey">
          <el-input v-model="formData.sSessionKey" :clearable="true"  placeholder="请输入会话密钥" />
        </el-form-item>
        <el-form-item label="用户ID:" prop="person_id">
          <el-input v-model="formData.person_id" :clearable="true"  placeholder="请输入用户ID" />
        </el-form-item>
        <el-form-item label="昵称:" prop="nickname">
          <el-input v-model="formData.nickname" :clearable="true"  placeholder="请输入昵称" />
        </el-form-item>
        <el-form-item label="注销时间:" prop="cancelled_at">
          <el-input v-model.number="formData.cancelled_at" :clearable="true" placeholder="请输入" />
        </el-form-item>
        <el-form-item label="状态:"  prop="status" >
          <el-select v-model="formData.status" placeholder="请选择状态" style="width:100%" :clearable="true" >
            <el-option v-for="(item,key) in weishi_statusOptions" :key="key" :label="item.label" :value="item.value" />
          </el-select>
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
  createWeishi,
  updateWeishi,
  findWeishi
} from '@/api/addon/weishi'

defineOptions({
    name: 'WeishiForm'
})

// 自动获取字典
import { getDictFunc } from '@/utils/format'
import { useRoute, useRouter } from "vue-router"
import { ElMessage } from 'element-plus'
import { ref, reactive } from 'vue'


const route = useRoute()
const router = useRouter()

// 提交按钮loading
const btnLoading = ref(false)
const weishi_statusOptions = ref([])

const type = ref('')
const formData = ref({
            account: '',
            password: '',
            iAuthType: 0,
            main_login: '',
            openid: '',
            sSessionKey: '',
            person_id: '',
            nickname: '',
            cancelled_at: '',
            status: [],
        })
// 验证规则
const rule = reactive({
               account : [{
                   required: true,
                   message: '',
                   trigger: ['input','blur'],
               }],
               password : [{
                   required: true,
                   message: '',
                   trigger: ['input','blur'],
               }],
               iAuthType : [{
                   required: true,
                   message: '',
                   trigger: ['input','blur'],
               }],
               main_login : [{
                   required: true,
                   message: '',
                   trigger: ['input','blur'],
               }],
               openid : [{
                   required: true,
                   message: '',
                   trigger: ['input','blur'],
               }],
               sSessionKey : [{
                   required: true,
                   message: '',
                   trigger: ['input','blur'],
               }],
               person_id : [{
                   required: true,
                   message: '',
                   trigger: ['input','blur'],
               }],
               cancelled_at : [{
                   required: false,
                   message: '',
                   trigger: ['input','blur'],
               }],
               status : [{
                   required: false,
                   message: '',
                   trigger: ['input','blur'],
               }],
})

const elFormRef = ref()

// 初始化方法
const init = async () => {
 // 建议通过url传参获取目标数据ID 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
    if (route.query.id) {
      const res = await findWeishi({ ID: route.query.id })
      if (res.code === 0) {
        formData.value = res.data
        type.value = 'update'
      }
    } else {
      type.value = 'create'
    }

  weishi_statusOptions.value = await getDictFunc('weishi_status')
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
               res = await createWeishi(formData.value)
               break
             case 'update':
               res = await updateWeishi(formData.value)
               break
             default:
               res = await createWeishi(formData.value)
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
