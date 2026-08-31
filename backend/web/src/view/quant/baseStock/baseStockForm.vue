
<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="TS代码（唯一标识，如：000001.SZ）:" prop="tsCode">
    <el-input v-model="formData.tsCode" :clearable="true" placeholder="请输入TS代码（唯一标识，如：000001.SZ）" />
</el-form-item>
        <el-form-item label="股票代码（如：000001）:" prop="symbol">
    <el-input v-model="formData.symbol" :clearable="true" placeholder="请输入股票代码（如：000001）" />
</el-form-item>
        <el-form-item label="股票名称:" prop="name">
    <el-input v-model="formData.name" :clearable="true" placeholder="请输入股票名称" />
</el-form-item>
        <el-form-item label="地域（如：广东、上海）:" prop="area">
    <el-input v-model="formData.area" :clearable="true" placeholder="请输入地域（如：广东、上海）" />
</el-form-item>
        <el-form-item label="所属行业（如：银行、半导体）:" prop="industry">
    <el-input v-model="formData.industry" :clearable="true" placeholder="请输入所属行业（如：银行、半导体）" />
</el-form-item>
        <el-form-item label="股票全称:" prop="fullname">
    <el-input v-model="formData.fullname" :clearable="true" placeholder="请输入股票全称" />
</el-form-item>
        <el-form-item label="英文全称:" prop="enname">
    <el-input v-model="formData.enname" :clearable="true" placeholder="请输入英文全称" />
</el-form-item>
        <el-form-item label="拼音缩写（如：ZGPA）:" prop="cnspell">
    <el-input v-model="formData.cnspell" :clearable="true" placeholder="请输入拼音缩写（如：ZGPA）" />
</el-form-item>
        <el-form-item label="市场类型（如：主板、创业板、科创板）:" prop="market">
    <el-input v-model="formData.market" :clearable="true" placeholder="请输入市场类型（如：主板、创业板、科创板）" />
</el-form-item>
        <el-form-item label="交易所代码（如：SZ深圳证券交易所、SH上海证券交易所）:" prop="exchange">
    <el-input v-model="formData.exchange" :clearable="true" placeholder="请输入交易所代码（如：SZ深圳证券交易所、SH上海证券交易所）" />
</el-form-item>
        <el-form-item label="交易货币（默认人民币CNY）:" prop="currType">
    <el-input v-model="formData.currType" :clearable="true" placeholder="请输入交易货币（默认人民币CNY）" />
</el-form-item>
        <el-form-item label="上市状态（L=上市，D=退市，P=暂停上市）:" prop="listStatus">
    <el-input v-model="formData.listStatus" :clearable="true" placeholder="请输入上市状态（L=上市，D=退市，P=暂停上市）" />
</el-form-item>
        <el-form-item label="上市日期（格式：YYYY-MM-DD）:" prop="listDate">
    <el-date-picker v-model="formData.listDate" type="date" style="width:100%" placeholder="选择日期" :clearable="true" />
</el-form-item>
        <el-form-item label="退市日期（格式：YYYY-MM-DD，未退市则为NULL）:" prop="delistDate">
    <el-date-picker v-model="formData.delistDate" type="date" style="width:100%" placeholder="选择日期" :clearable="true" />
</el-form-item>
        <el-form-item label="是否沪深港通标的（N=否，H=沪股通，S=深股通）:" prop="isHs">
    <el-input v-model="formData.isHs" :clearable="true" placeholder="请输入是否沪深港通标的（N=否，H=沪股通，S=深股通）" />
</el-form-item>
        <el-form-item label="实控人名称:" prop="actName">
    <el-input v-model="formData.actName" :clearable="true" placeholder="请输入实控人名称" />
</el-form-item>
        <el-form-item label="实控人企业性质（如：国有企业、民营企业）:" prop="actEntType">
    <el-input v-model="formData.actEntType" :clearable="true" placeholder="请输入实控人企业性质（如：国有企业、民营企业）" />
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
  createBaseStock,
  updateBaseStock,
  findBaseStock
} from '@/api/quant/baseStock'

defineOptions({
    name: 'BaseStockForm'
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
            tsCode: '',
            symbol: '',
            name: '',
            area: '',
            industry: '',
            fullname: '',
            enname: '',
            cnspell: '',
            market: '',
            exchange: '',
            currType: '',
            listStatus: '',
            listDate: new Date(),
            delistDate: new Date(),
            isHs: '',
            actName: '',
            actEntType: '',
        })
// 验证规则
const rule = reactive({
})

const elFormRef = ref()

// 初始化方法
const init = async () => {
 // 建议通过url传参获取目标数据ID 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
    if (route.query.id) {
      const res = await findBaseStock({ ID: route.query.id })
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
               res = await createBaseStock(formData.value)
               break
             case 'update':
               res = await updateBaseStock(formData.value)
               break
             default:
               res = await createBaseStock(formData.value)
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
