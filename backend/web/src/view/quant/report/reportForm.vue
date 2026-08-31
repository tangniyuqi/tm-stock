
<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="名称:" prop="name">
    <el-input v-model="formData.name" :clearable="true" placeholder="请输入名称" />
</el-form-item>
        <el-form-item label="类型:" prop="type">
    <el-switch v-model="formData.type" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable ></el-switch>
</el-form-item>
        <el-form-item label="行业:" prop="industry">
    <el-input v-model="formData.industry" :clearable="true" placeholder="请输入行业" />
</el-form-item>
        <el-form-item label="等级:" prop="level">
    <el-switch v-model="formData.level" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable ></el-switch>
</el-form-item>
        <el-form-item label="发布机构:" prop="institution">
    <el-input v-model="formData.institution" :clearable="true" placeholder="请输入发布机构" />
</el-form-item>
        <el-form-item label="分析师:" prop="analyst">
    <el-input v-model="formData.analyst" :clearable="true" placeholder="请输入分析师" />
</el-form-item>
        <el-form-item label="发布日期:" prop="publishDate">
    <el-date-picker v-model="formData.publishDate" type="date" style="width:100%" placeholder="选择日期" :clearable="true" />
</el-form-item>
        <el-form-item label="有效日期:" prop="deadlineDate">
    <el-date-picker v-model="formData.deadlineDate" type="date" style="width:100%" placeholder="选择日期" :clearable="true" />
</el-form-item>
        <el-form-item label="结论摘要:" prop="conclusion">
    <el-input v-model="formData.conclusion" :clearable="true" placeholder="请输入结论摘要" />
</el-form-item>
        <el-form-item label="文件:" prop="file">
    <el-input v-model="formData.file" :clearable="true" placeholder="请输入文件" />
</el-form-item>
        <el-form-item label="页数:" prop="page">
    <el-input v-model.number="formData.page" :clearable="true" placeholder="请输入页数" />
</el-form-item>
        <el-form-item label="备注:" prop="remark">
    <el-input v-model="formData.remark" :clearable="true" placeholder="请输入备注" />
</el-form-item>
        <el-form-item label="状态:" prop="status">
    <el-switch v-model="formData.status" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable ></el-switch>
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
  createReport,
  updateReport,
  findReport
} from '@/api/quant/report'

defineOptions({
    name: 'ReportForm'
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
            name: '',
            type: false,
            industry: '',
            level: false,
            institution: '',
            analyst: '',
            publishDate: new Date(),
            deadlineDate: new Date(),
            conclusion: '',
            file: '',
            page: undefined,
            remark: '',
            status: false,
        })
// 验证规则
const rule = reactive({
})

const elFormRef = ref()

// 初始化方法
const init = async () => {
 // 建议通过url传参获取目标数据ID 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
    if (route.query.id) {
      const res = await findReport({ ID: route.query.id })
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
               res = await createReport(formData.value)
               break
             case 'update':
               res = await updateReport(formData.value)
               break
             default:
               res = await createReport(formData.value)
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
