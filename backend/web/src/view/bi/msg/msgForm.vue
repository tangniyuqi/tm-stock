<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="客户端ID:" prop="client_id">
          <el-input v-model.number="formData.client_id" clearable placeholder="请输入客户端ID" />
        </el-form-item>
        <el-form-item label="发送者ID:" prop="sender_id">
          <el-input v-model="formData.sender_id" clearable placeholder="请输入发送者ID" />
        </el-form-item>
        <el-form-item label="接收者ID:" prop="receiver_id">
          <el-input v-model="formData.receiver_id" clearable placeholder="请输入接收者ID" />
        </el-form-item>
        <el-form-item label="MID:" prop="msg_id">
          <el-input v-model="formData.msg_id" clearable placeholder="请输入MID" />
        </el-form-item>
        <el-form-item label="GID:" prop="gid">
          <el-input v-model="formData.gid" clearable placeholder="请输入GID" />
        </el-form-item>
        <el-form-item label="群组:" prop="group">
          <el-input v-model="formData.group" clearable placeholder="请输入群组" />
        </el-form-item>
        <el-form-item label="类型:" prop="type">
          <el-input v-model="formData.type" clearable placeholder="请输入类型" />
        </el-form-item>
        <el-form-item label="发送者:" prop="sender">
          <el-input v-model="formData.sender" clearable placeholder="请输入发送者" />
        </el-form-item>
        <el-form-item label="发送者备注:" prop="sender_remark">
          <el-input v-model="formData.sender_remark" clearable placeholder="请输入发送者备注" />
        </el-form-item>
        <el-form-item label="接收者:" prop="receiver">
          <el-input v-model="formData.receiver" clearable placeholder="请输入接收者" />
        </el-form-item>
        <el-form-item label="发送时间:" prop="send_time">
          <el-date-picker v-model="formData.send_time" type="date" style="width: 100%" placeholder="选择日期" clearable />
        </el-form-item>
        <el-form-item label="内容:" prop="content">
          <el-input v-model="formData.content" clearable placeholder="请输入内容" />
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
import { createMsg, updateMsg, findMsg } from '@/api/bi/msg';

defineOptions({
  name: 'MsgForm'
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
  client_id: undefined,
  sender_id: '',
  receiver_id: '',
  msg_id: '',
  gid: '',
  group: '',
  type: '',
  sender: '',
  sender_remark: '',
  receiver: '',
  send_time: new Date(),
  content: '',
  status: undefined,
  created_by: undefined,
  updated_by: undefined,
  deleted_by: undefined
});
// 验证规则
const rule = reactive({
  client_id: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur']
    }
  ],
  msg_id: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur']
    }
  ],
  type: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur']
    }
  ],
  sender: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur']
    }
  ],
  receiver: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur']
    }
  ],
  send_time: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur']
    }
  ],
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
  // 建议通过url传参获取目标数据id 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
  if (route.query.id) {
    const res = await findMsg({ id: route.query.id });
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
        res = await createMsg(formData.value);
        break;
      case 'update':
        res = await updateMsg(formData.value);
        break;
      default:
        res = await createMsg(formData.value);
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
