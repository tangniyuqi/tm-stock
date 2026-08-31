
<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="名称:" prop="title">
          <el-input v-model="formData.title" clearable  placeholder="请输入名称" />
       </el-form-item>
        <el-form-item label="金额:" prop="amount">
          <el-input-number v-model="formData.amount" :precision="2" clearable></el-input-number>
       </el-form-item>
        <el-form-item label="阶段:" prop="stage">
          <el-input v-model.number="formData.stage" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="可能性:" prop="possibility">
          <el-input v-model.number="formData.possibility" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="描述:" prop="description">
          <RichEdit v-model="formData.description"/>
       </el-form-item>
        <el-form-item label="记录:" prop="records">
          // 此字段为json结构，可以前端自行控制展示和数据绑定模式 需绑定json的key为 formData.records 后端会按照json的类型进行存取
          {{ formData.records }}
       </el-form-item>
        <el-form-item label="备注:" prop="remark">
          <RichEdit v-model="formData.remark"/>
       </el-form-item>
        <el-form-item label="类型:" prop="type">
          <el-input v-model.number="formData.type" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="来源:" prop="source">
          <el-input v-model.number="formData.source" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="成交日期:" prop="close_date">
          <el-input v-model.number="formData.close_date" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="最近联系日期:" prop="last_date">
          <el-input v-model.number="formData.last_date" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="下次联系日期:" prop="next_date">
          <el-input v-model.number="formData.next_date" clearable placeholder="请输入" />
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
  createDeal,
  updateDeal,
  findDeal
} from '@/api/cloud/deal'

defineOptions({
    name: 'DealForm'
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
            title: '',
            amount: 0,
            stage: undefined,
            possibility: undefined,
            description: '',
            records: {},
            remark: '',
            type: undefined,
            source: undefined,
            close_date: undefined,
            last_date: undefined,
            next_date: undefined,
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
      const res = await findDeal({ ID: route.query.id })
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
               res = await createDeal(formData.value)
               break
             case 'update':
               res = await updateDeal(formData.value)
               break
             default:
               res = await createDeal(formData.value)
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
