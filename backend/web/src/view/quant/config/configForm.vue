<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="用户ID:" prop="member_id">
          <el-input v-model.number="formData.member_id" :clearable="true" placeholder="请输入用户ID" />
        </el-form-item>
        <el-form-item label="类型:" prop="type">
          <el-input v-model.number="formData.type" :clearable="true" placeholder="请输入类型" />
        </el-form-item>
        <el-form-item label="名称:" prop="name">
          <el-input v-model="formData.name" :clearable="true" placeholder="请输入名称" />
        </el-form-item>
        <el-form-item label="主机:" prop="host">
          <el-input v-model="formData.host" :clearable="true" placeholder="请输入主机" />
        </el-form-item>
        <el-form-item label="端口:" prop="port">
          <el-input v-model="formData.port" :clearable="true" placeholder="请输入端口" />
        </el-form-item>
        <el-form-item label="密钥:" prop="key">
          <el-input v-model="formData.key" :clearable="true" placeholder="请输入密钥" />
        </el-form-item>
        <el-form-item label="客户端:" prop="client">
          <el-input v-model="formData.client" :clearable="true" placeholder="请输入客户端" />
        </el-form-item>
        <el-form-item label="Webhook类型:" prop="webhook_type">
          <el-input v-model.number="formData.webhook_type" :clearable="true" placeholder="请输入Webhook类型" />
        </el-form-item>
        <el-form-item label="Webhook地址:" prop="webhook_url">
          <el-input v-model="formData.webhook_url" :clearable="true" placeholder="请输入Webhook地址" />
        </el-form-item>
        <el-form-item label="数据源:" prop="data_source">
          <el-input v-model.number="formData.data_source" :clearable="true" placeholder="请输入数据源" />
        </el-form-item>
        <el-form-item label="数据TOKEN:" prop="data_token">
          <el-input v-model="formData.data_token" :clearable="true" placeholder="请输入数据TOKEN" />
        </el-form-item>
        <el-form-item label="备注:" prop="remark">
          <el-input v-model="formData.remark" :clearable="true" placeholder="请输入备注" />
        </el-form-item>
        <el-form-item label="状态:" prop="status">
          <el-input v-model.number="formData.status" :clearable="true" placeholder="请输入状态" />
        </el-form-item>
        <el-form-item label="创建者:" prop="created_by">
          <el-input v-model.number="formData.created_by" :clearable="true" placeholder="请输入创建者" />
        </el-form-item>
        <el-form-item label="更新者:" prop="updated_by">
          <el-input v-model.number="formData.updated_by" :clearable="true" placeholder="请输入更新者" />
        </el-form-item>
        <el-form-item label="删除者:" prop="deleted_by">
          <el-input v-model.number="formData.deleted_by" :clearable="true" placeholder="请输入删除者" />
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
  createConfig,
  updateConfig,
  findConfig
} from '@/api/quant/config'

defineOptions({
  name: 'ConfigForm'
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
  type: undefined,
  name: '',
  host: '',
  port: '',
  key: '',
  client: '',
  webhook_type: undefined,
  webhook_url: '',
  data_source: undefined,
  data_token: '',
  remark: '',
  status: undefined,
  created_by: undefined,
  updated_by: undefined,
  deleted_by: undefined,
})
// 验证规则
const rule = reactive({
})

const elFormRef = ref()

// 初始化方法
const init = async () => {
  // 建议通过url传参获取目标数据ID 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
  if (route.query.id) {
    const res = await findConfig({ ID: route.query.id })
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
        res = await createConfig(formData.value)
        break
      case 'update':
        res = await updateConfig(formData.value)
        break
      default:
        res = await createConfig(formData.value)
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
