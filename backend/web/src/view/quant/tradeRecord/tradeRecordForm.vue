<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="用户ID:" prop="member_id">
          <el-input v-model.number="formData.member_id" :clearable="true" placeholder="请输入用户ID" />
        </el-form-item>
        <el-form-item label="账户ID:" prop="account_id">
          <el-input v-model.number="formData.account_id" :clearable="true" placeholder="请输入账户ID" />
        </el-form-item>
        <el-form-item label="策略ID:" prop="strategy_id">
          <el-input v-model.number="formData.strategy_id" :clearable="true" placeholder="请输入策略ID" />
        </el-form-item>
        <el-form-item label="交易时间:" prop="traded_at">
          <el-date-picker v-model="formData.traded_at" type="date" style="width:100%" placeholder="选择日期"
            :clearable="true" />
        </el-form-item>
        <el-form-item label="方向:" prop="direction">
          <el-input v-model="formData.direction" :clearable="true" placeholder="请输入方向" />
        </el-form-item>
        <el-form-item label="类型:" prop="type">
          <el-input v-model="formData.type" :clearable="true" placeholder="请输入类型" />
        </el-form-item>
        <el-form-item label="标识:" prop="symbol">
          <el-input v-model="formData.symbol" :clearable="true" placeholder="请输入标识" />
        </el-form-item>
        <el-form-item label="名称:" prop="name">
          <el-input v-model="formData.name" :clearable="true" placeholder="请输入名称" />
        </el-form-item>
        <el-form-item label="价格:" prop="price">
          <el-input-number v-model="formData.price" style="width:100%" :precision="2" :clearable="true" />
        </el-form-item>
        <el-form-item label="数量:" prop="volume">
          <el-input-number v-model="formData.volume" style="width:100%" :precision="2" :clearable="true" />
        </el-form-item>
        <el-form-item label="金额:" prop="amount">
          <el-input-number v-model="formData.amount" style="width:100%" :precision="2" :clearable="true" />
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
  createTradeRecord,
  updateTradeRecord,
  findTradeRecord
} from '@/api/quant/tradeRecord'

defineOptions({
  name: 'TradeRecordForm'
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
  account_id: undefined,
  strategy_id: undefined,
  traded_at: new Date(),
  direction: '',
  type: '',
  symbol: '',
  name: '',
  price: 0,
  volume: 0,
  amount: 0,
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
    const res = await findTradeRecord({ ID: route.query.id })
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
        res = await createTradeRecord(formData.value)
        break
      case 'update':
        res = await updateTradeRecord(formData.value)
        break
      default:
        res = await createTradeRecord(formData.value)
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
