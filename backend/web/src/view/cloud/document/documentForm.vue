
<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="商户ID:" prop="merchant_id">
          <el-input v-model.number="formData.merchant_id" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="标题:" prop="title">
          <el-input v-model="formData.title" clearable  placeholder="请输入标题" />
       </el-form-item>
        <el-form-item label="分类ID:" prop="cateId">
          <el-input v-model.number="formData.cateId" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="外链地址:" prop="url">
          <el-input v-model="formData.url" clearable  placeholder="请输入外链地址" />
       </el-form-item>
        <el-form-item label="跳转地址:" prop="link">
          <el-input v-model="formData.link" clearable  placeholder="请输入跳转地址" />
       </el-form-item>
        <el-form-item label="摘要:" prop="summary">
          <el-input v-model="formData.summary" clearable  placeholder="请输入摘要" />
       </el-form-item>
        <el-form-item label="内容:" prop="content">
          <RichEdit v-model="formData.content" />
       </el-form-item>
        <el-form-item label="标签:" prop="tags">
          <ArrayCtrl v-model="formData.tags" editable />
       </el-form-item>
        <el-form-item label="排序:" prop="sort">
          <el-input v-model.number="formData.sort" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="浏览量:" prop="view">
          <el-input v-model.number="formData.view" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="顶置:" prop="top">
          <el-input v-model.number="formData.top" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="精华:" prop="digest">
          <el-input v-model.number="formData.digest" clearable placeholder="请输入" />
       </el-form-item>
        <el-form-item label="状态:" prop="status">
          <el-input v-model.number="formData.status" clearable placeholder="请输入" />
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
  createDocument,
  updateDocument,
  findDocument
} from '@/api/cloud/document'

defineOptions({
    name: 'DocumentForm'
})

// 自动获取字典
import { getDictFunc } from '@/utils/format'
import { useRoute, useRouter } from "vue-router"
import { ElMessage } from 'element-plus'
import { ref, reactive } from 'vue'
// 富文本组件
import RichEdit from '@/components/richtext/rich-edit.vue'
import ArrayCtrl from '@/components/arrayCtrl/arrayCtrl.vue'

const route = useRoute()
const router = useRouter()

// 提交按钮loading
const btnLoading = ref(false)

const type = ref('')
const formData = ref({
            merchant_id: undefined,
            title: '',
            cateId: undefined,
            url: '',
            link: '',
            summary: '',
            content: '',
            tags: {},
            sort: undefined,
            view: undefined,
            top: undefined,
            digest: undefined,
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
      const res = await findDocument({ ID: route.query.id })
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
               res = await createDocument(formData.value)
               break
             case 'update':
               res = await updateDocument(formData.value)
               break
             default:
               res = await createDocument(formData.value)
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
