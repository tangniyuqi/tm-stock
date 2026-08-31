
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
        <el-form-item label="名称:" prop="title">
    <el-input v-model="formData.title" clearable placeholder="请输入名称" />
</el-form-item>
        <el-form-item label="客户ID:" prop="customer_id">
    <el-input v-model.number="formData.customer_id" clearable placeholder="请输入客户ID" />
</el-form-item>
        <el-form-item label="类型:" prop="type">
    <el-input v-model.number="formData.type" clearable placeholder="请输入类型" />
</el-form-item>
        <el-form-item label="收入:" prop="amount_in">
    <el-input-number v-model="formData.amount_in" style="width:100%" :precision="2" clearable />
</el-form-item>
        <el-form-item label="支出:" prop="amount_out">
    <el-input-number v-model="formData.amount_out" style="width:100%" :precision="2" clearable />
</el-form-item>
        <el-form-item label="账户类型:" prop="account_type">
    <el-input v-model.number="formData.account_type" clearable placeholder="请输入账户类型" />
</el-form-item>
        <el-form-item label="支付方式:" prop="pay_mode">
    <el-input v-model.number="formData.pay_mode" clearable placeholder="请输入支付方式" />
</el-form-item>
        <el-form-item label="付款人:" prop="payer">
    <el-input v-model="formData.payer" clearable placeholder="请输入付款人" />
</el-form-item>
        <el-form-item label="经办人:" prop="transactor">
    <el-input v-model="formData.transactor" clearable placeholder="请输入经办人" />
</el-form-item>
        <el-form-item label="经办日期:" prop="handling_date">
    <el-input v-model.number="formData.handling_date" clearable placeholder="请输入经办日期" />
</el-form-item>
        <el-form-item label="凭证类型:" prop="proof_type">
    <el-input v-model.number="formData.proof_type" clearable placeholder="请输入凭证类型" />
</el-form-item>
        <el-form-item label="凭证编号:" prop="proof_number">
    <el-input v-model="formData.proof_number" clearable placeholder="请输入凭证编号" />
</el-form-item>
        <el-form-item label="备注:" prop="remark">
    <RichEdit v-model="formData.remark"/>
</el-form-item>
        <el-form-item label="结平:" prop="balance">
    <el-switch v-model="formData.balance" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable ></el-switch>
</el-form-item>
        <el-form-item label="审核:" prop="review">
    <el-switch v-model="formData.review" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable ></el-switch>
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
  createAccount,
  updateAccount,
  findAccount
} from '@/api/cloud/account'

defineOptions({
    name: 'AccountForm'
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
            title: '',
            customer_id: undefined,
            type: undefined,
            amount_in: 0,
            amount_out: 0,
            account_type: undefined,
            pay_mode: undefined,
            payer: '',
            transactor: '',
            handling_date: undefined,
            proof_type: undefined,
            proof_number: '',
            remark: '',
            balance: false,
            review: false,
            status: undefined,
        })
// 验证规则
const rule = reactive({
               title : [{
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
const save = async() => {
      btnLoading.value = true
      elFormRef.value?.validate( async (valid) => {
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

<style>
</style>
