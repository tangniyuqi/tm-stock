<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="top" :rules="rule" label-width="80px">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="题材ID:" prop="theme_id">
              <el-input v-model.number="formData.theme_id" :clearable="true" placeholder="请输入题材ID" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="股票ID :" prop="stock_id">
              <el-input v-model.number="formData.stock_id" :clearable="true" placeholder="请输入股票ID " />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="相关度:" prop="relevance">
              <el-input-number v-model="formData.relevance" style="width: 100%" :precision="2" :clearable="true" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="纳入日期:" prop="in_date">
              <el-date-picker v-model="formData.in_date" type="date" style="width: 100%" placeholder="选择日期"
                :clearable="true" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="入选逻辑:" prop="reason">
              <el-input v-model="formData.reason" type="textarea" :autosize="{ minRows: 4, maxRows: 8 }"
                :clearable="true" placeholder="请输入入选逻辑" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="AI入选逻辑:" prop="ai_reason">
              <el-input v-model="formData.ai_reason" type="textarea" :autosize="{ minRows: 4, maxRows: 8 }"
                :clearable="true" placeholder="请输入AI分析入选逻辑" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="梯队:" prop="tier">
              <el-select v-model="formData.tier" placeholder="请选择梯队" clearable>
                <el-option v-for="(item, key) in tierOptions" :key="key" :label="item.label"
                  :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="排序:" prop="sort">
              <el-input v-model.number="formData.sort" :clearable="true" placeholder="请输入排序" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="状态:" prop="status">
              <el-select v-model="formData.status" placeholder="请选择状态" clearable>
                <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label"
                  :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item>
              <el-button :loading="btnLoading" type="primary" @click="save">保存</el-button>
              <el-button type="primary" @click="back">返回</el-button>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { createThemeStock, updateThemeStock, findThemeStock } from '@/api/quant/themeStock';

defineOptions({
  name: 'ThemeStockForm',
});

// 自动获取字典
import { getDictFunc } from '@/utils/format';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { ref, reactive } from 'vue';

const route = useRoute();
const router = useRouter();

// 提交按钮loading
const btnLoading = ref(false);

const type = ref('');
const statusOptions = ref([]);
const tierOptions = ref([]);
const formData = ref({
  theme_id: undefined,
  stock_id: undefined,
  relevance: 0,
  reason: '',
  ai_reason: '',
  in_date: new Date(),
  tier: 0,
  sort: 0,
  status: 1,
});
// 验证规则
const rule = reactive({});

const elFormRef = ref();

// 初始化方法
const init = async () => {
  [statusOptions.value, tierOptions.value] = await Promise.all([getDictFunc('status'), getDictFunc('quant_theme_stock_tier')]);
  // 建议通过url传参获取目标数据ID 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
  if (route.query.id) {
    const res = await findThemeStock({ ID: route.query.id });
    if (res.code === 0) {
      formData.value = res.data;
      type.value = 'update';
    }
  } else {
    type.value = 'create';
  }
};

init();
// 保存按钮
const save = async () => {
  btnLoading.value = true;
  elFormRef.value?.validate(async valid => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createThemeStock(formData.value);
        break;
      case 'update':
        res = await updateThemeStock(formData.value);
        break;
      default:
        res = await createThemeStock(formData.value);
        break;
    }
    btnLoading.value = false;
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '创建/更改成功',
      });
    }
  });
};

// 返回按钮
const back = () => {
  router.go(-1);
};
</script>

<style></style>
