<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="用户ID:" prop="member_id">
          <el-input v-model.number="formData.member_id" :clearable="true" placeholder="请输入用户ID" />
        </el-form-item>
        <el-form-item label="名称:" prop="name">
          <el-input v-model="formData.name" :clearable="true" placeholder="请输入名称" />
        </el-form-item>
        <el-form-item label="交易时间检查周期:" prop="trading_check_interval">
          <el-input v-model.number="formData.trading_check_interval" :clearable="true" placeholder="请输入交易时间检查周期" />
        </el-form-item>
        <el-form-item label="股票监控周期:" prop="stock_monitor_interval">
          <el-input v-model.number="formData.stock_monitor_interval" :clearable="true" placeholder="请输入股票监控周期" />
        </el-form-item>
        <el-form-item label="持仓检查周期:" prop="holdings_check_interval">
          <el-input v-model.number="formData.holdings_check_interval" :clearable="true" placeholder="请输入持仓检查周期" />
        </el-form-item>
        <el-form-item label="收盘时间检查周期:" prop="market_check_interval">
          <el-input v-model.number="formData.market_check_interval" :clearable="true" placeholder="请输入收盘时间检查周期" />
        </el-form-item>
        <el-form-item label="交易量计算方式:" prop="volume_calc">
          <el-input v-model.number="formData.volume_calc" :clearable="true" placeholder="请输入交易量计算方式" />
        </el-form-item>
        <el-form-item label="基础交易量（股）:" prop="base_volume">
          <el-input v-model.number="formData.base_volume" :clearable="true" placeholder="请输入基础交易量（股）" />
        </el-form-item>
        <el-form-item label="最小交易量（股）:" prop="min_volume">
          <el-input v-model.number="formData.min_volume" :clearable="true" placeholder="请输入最小交易量（股）" />
        </el-form-item>
        <el-form-item label="最大单次交易量（股）:" prop="max_volume">
          <el-input v-model.number="formData.max_volume" :clearable="true" placeholder="请输入最大单次交易量（股）" />
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
  createStrategy,
  updateStrategy,
  findStrategy
} from '@/api/quant/strategy'

defineOptions({
  name: 'StrategyForm'
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
  name: '',
  trading_check_interval: undefined,
  stock_monitor_interval: undefined,
  holdings_check_interval: undefined,
  market_check_interval: undefined,
  volume_calc: undefined,
  base_volume: undefined,
  min_volume: undefined,
  max_volume: undefined,
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
    const res = await findStrategy({ ID: route.query.id })
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
        res = await createStrategy(formData.value)
        break
      case 'update':
        res = await updateStrategy(formData.value)
        break
      default:
        res = await createStrategy(formData.value)
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
