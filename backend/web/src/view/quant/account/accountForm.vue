<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="用户ID:" prop="member_id">
          <el-input v-model.number="formData.member_id" :clearable="true" placeholder="请输入用户ID" />
        </el-form-item>
        <el-form-item label="配置ID:" prop="config_id">
          <el-input v-model.number="formData.config_id" :clearable="true" placeholder="请输入配置ID" />
        </el-form-item>
        <el-form-item label="策略ID:" prop="strategy_id">
          <el-input v-model.number="formData.strategy_id" :clearable="true" placeholder="请输入策略ID" />
        </el-form-item>
        <el-form-item label="类型:" prop="type">
          <el-input v-model.number="formData.type" :clearable="true" placeholder="请输入类型" />
        </el-form-item>
        <el-form-item label="称呼:" prop="name">
          <el-input v-model="formData.name" :clearable="true" placeholder="请输入称呼" />
        </el-form-item>
        <el-form-item label="券商:" prop="security">
          <el-input v-model="formData.security" :clearable="true" placeholder="请输入券商" />
        </el-form-item>
        <el-form-item label="账号:" prop="accountNo">
          <el-input v-model="formData.accountNo" :clearable="true" placeholder="请输入账号" />
        </el-form-item>
        <el-form-item label="密码:" prop="passcode">
          <el-input v-model="formData.passcode" :clearable="true" placeholder="请输入密码" />
        </el-form-item>
        <el-form-item label="资金:" prop="amount">
          <el-input-number v-model="formData.amount" style="width:100%" :precision="2" :clearable="true" />
        </el-form-item>
        <el-form-item label="备注:" prop="remark">
          <el-input v-model="formData.remark" :clearable="true" placeholder="请输入备注" />
        </el-form-item>
        <el-form-item label="状态:" prop="status">
          <el-input v-model.number="formData.status" :clearable="true" placeholder="请输入状态" />
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
  createAccount,
  updateAccount,
  findAccount
} from '@/api/quant/account'

defineOptions({
  name: 'AccountForm'
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

const type = ref('')
const formData = ref({
  member_id: undefined,
  config_id: undefined,
  strategy_id: undefined,
  type: undefined,
  name: '',
  security: '',
  accountNo: '',
  passcode: '',
  amount: 0,
  remark: '',
  status: undefined,
})
// 验证规则
const rule = reactive({
})

const elFormRef = ref()

// 初始化方法
const init = async () => {
  // 建议通过url传参获取目标数据ID 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
  if (route.query.id) {
    const res = await findAccount({ ID: route.query.id })
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
const save = async () => {
  btnLoading.value = true
  elFormRef.value?.validate(async (valid) => {
    if (!valid) return btnLoading.value = false
    let res
    switch (type.value) {
      case 'create':
        res = await createAccount(formData.value)
        break
      case 'update':
        res = await updateAccount(formData.value)
        break
      default:
        res = await createAccount(formData.value)
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

<style></style>
