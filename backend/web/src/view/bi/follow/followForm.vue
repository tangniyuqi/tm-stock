<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="商机ID:" prop="deal_id">
          <el-input v-model.number="formData.deal_id" clearable placeholder="请输入商机ID" />
        </el-form-item>
        <el-form-item label="消息ID:" prop="msg_id">
          <el-input v-model.number="formData.msg_id" clearable placeholder="请输入消息ID" />
        </el-form-item>
        <el-form-item label="用户ID:" prop="member_id">
          <el-input v-model.number="formData.member_id" clearable placeholder="请输入用户ID" />
        </el-form-item>
        <el-form-item label="客户ID:" prop="customer_id">
          <el-input v-model.number="formData.customer_id" clearable placeholder="请输入客户ID" />
        </el-form-item>
        <el-form-item label="跟进时间:" prop="follow_time">
          <el-date-picker v-model="formData.follow_time" type="date" style="width: 100%" placeholder="选择日期" clearable />
        </el-form-item>
        <el-form-item label="跟进方式:" prop="type">
          <el-input v-model.number="formData.type" clearable placeholder="请输入跟进方式" />
        </el-form-item>
        <el-form-item label="跟进内容:" prop="content">
          <el-input v-model="formData.content" clearable placeholder="请输入跟进内容" />
        </el-form-item>
        <el-form-item label="下次跟进计划:" prop="plan">
          <el-input v-model="formData.plan" clearable placeholder="请输入下次跟进计划" />
        </el-form-item>
        <el-form-item label="反馈类型:" prop="feedback">
          <el-input v-model.number="formData.feedback" clearable placeholder="请输入反馈类型" />
        </el-form-item>
        <el-form-item label="原因:" prop="reason">
          <el-input v-model="formData.reason" clearable placeholder="请输入原因" />
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
import { createFollow, updateFollow, findFollow } from '@/api/bi/follow';

defineOptions({
  name: 'FollowForm'
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
  deal_id: undefined,
  msg_id: undefined,
  member_id: undefined,
  customer_id: undefined,
  follow_time: new Date(),
  type: undefined,
  content: '',
  plan: '',
  feedback: undefined,
  reason: ''
});
// 验证规则
const rule = reactive({
  content: [
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
  // 建议通过url传参获取目标数据ID 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
  if (route.query.id) {
    const res = await findFollow({ ID: route.query.id });
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
        res = await createFollow(formData.value);
        break;
      case 'update':
        res = await updateFollow(formData.value);
        break;
      default:
        res = await createFollow(formData.value);
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
