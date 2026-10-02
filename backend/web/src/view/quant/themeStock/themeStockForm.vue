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
            <el-form-item label="依据类型:" prop="source_type">
              <el-select v-model="formData.source_type" placeholder="请选择依据类型" clearable style="width: 100%">
                <el-option v-for="item in sourceTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="采集时点:" prop="collected_at">
              <el-date-picker v-model="formData.collected_at" type="datetime" style="width: 100%" placeholder="选择采集时点"
                :clearable="true" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="原文摘录:" prop="source_excerpt">
              <el-input v-model="formData.source_excerpt" type="textarea" :autosize="{ minRows: 4, maxRows: 10 }"
                maxlength="1000" show-word-limit :clearable="true"
                placeholder="粘贴原文中支撑该归属的那一句话（客观事实）。不得填“见链接”“详见公告”等占位，也不得含评价、预测或买卖类措辞" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="原文链接:" prop="source_url">
              <el-input v-model="formData.source_url" :clearable="true" placeholder="https://……（公告、年报、招股书等原文地址）" />
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
// 依据类型（与后端 source_type 取值一致）
const sourceTypeOptions = [
  { value: 1, label: '公告' },
  { value: 2, label: '年报' },
  { value: 3, label: '招股书' },
  { value: 4, label: '官方产业目录' },
  { value: 5, label: '互动易问答' },
];
const formData = ref({
  theme_id: undefined,
  stock_id: undefined,
  source_type: undefined,
  source_excerpt: '',
  source_url: '',
  collected_at: new Date(),
  sort: 0,
  status: 1,
});
// 验证规则：依据四项缺一不可（与后端、数据库约束一致）
const rule = reactive({
  theme_id: [{ required: true, message: '请输入题材ID', trigger: 'blur' }],
  stock_id: [{ required: true, message: '请输入股票ID', trigger: 'blur' }],
  source_type: [{ required: true, message: '请选择依据类型', trigger: 'change' }],
  source_excerpt: [
    { required: true, message: '请填写原文摘录', trigger: 'blur' },
    { min: 8, message: '摘录至少 8 个字', trigger: 'blur' },
  ],
  source_url: [
    { required: true, message: '请填写原文链接', trigger: 'blur' },
    { pattern: /^https?:\/\/\S+$/, message: '链接必须是完整的 http(s) 地址', trigger: 'blur' },
  ],
  collected_at: [{ required: true, message: '请选择采集时点', trigger: 'change' }],
});

const elFormRef = ref();

// 初始化方法
const init = async () => {
  [statusOptions.value] = await Promise.all([getDictFunc('status')]);
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
