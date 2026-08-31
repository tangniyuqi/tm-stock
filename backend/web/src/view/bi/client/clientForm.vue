<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="名称:" prop="name">
          <el-input v-model="formData.name" clearable placeholder="请输入名称" />
        </el-form-item>
        <el-form-item label="服务器:" prop="server">
          <el-input v-model="formData.server" clearable placeholder="请输入服务器" />
        </el-form-item>
        <el-form-item label="备注:" prop="remark">
          <el-input v-model="formData.remark" clearable placeholder="请输入备注" />
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
import { createClient, updateClient, findClient } from '@/api/bi/client';

defineOptions({
  name: 'ClientForm'
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
const formData = ref({
  name: '',
  server: '',
  remark: '',
  status: undefined
});
// 验证规则
const rule = reactive({
  name: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur']
    }
  ]
});

const elFormRef = ref();

// 初始化方法
const init = async () => {
  // 建议通过url传参获取目标数据id 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
  if (route.query.id) {
    const res = await findClient({ id: route.query.id });
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
  elFormRef.value?.validate(async (valid) => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createClient(formData.value);
        break;
      case 'update':
        res = await updateClient(formData.value);
        break;
      default:
        res = await createClient(formData.value);
        break;
    }
    btnLoading.value = false;
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '创建/更改成功'
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
