<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="客户端ID:" prop="client_id">
          <el-input v-model.number="formData.client_id" clearable placeholder="请输入客户端ID" />
        </el-form-item>
        <el-form-item label="群组ID:" prop="group_id">
          <el-input v-model.number="formData.group_id" clearable placeholder="请输入群组ID" />
        </el-form-item>
        <el-form-item label="GID:" prop="gid">
          <el-input v-model="formData.gid" clearable placeholder="请输入GID" />
        </el-form-item>
        <el-form-item label="UID:" prop="uid">
          <el-input v-model="formData.uid" clearable placeholder="请输入UID" />
        </el-form-item>
        <el-form-item label="头像:" prop="avatar">
          <el-input v-model="formData.avatar" clearable placeholder="请输入头像" />
        </el-form-item>
        <el-form-item label="昵称:" prop="nickname">
          <el-input v-model="formData.nickname" clearable placeholder="请输入昵称" />
        </el-form-item>
        <el-form-item label="姓名:" prop="name">
          <el-input v-model="formData.name" clearable placeholder="请输入姓名" />
        </el-form-item>
        <el-form-item label="业务:" prop="business">
          <el-input v-model="formData.business" clearable placeholder="请输入业务" />
        </el-form-item>
        <el-form-item label="电话:" prop="mobile">
          <el-input v-model="formData.mobile" clearable placeholder="请输入电话" />
        </el-form-item>
        <el-form-item label="微信:" prop="wechat">
          <el-input v-model="formData.wechat" clearable placeholder="请输入微信" />
        </el-form-item>
        <el-form-item label="QQ:" prop="qq">
          <el-input v-model="formData.qq" clearable placeholder="请输入QQ" />
        </el-form-item>
        <el-form-item label="抖音:" prop="douyin">
          <el-input v-model="formData.douyin" clearable placeholder="请输入抖音" />
        </el-form-item>
        <el-form-item label="TikTok:" prop="tiktok">
          <el-input v-model="formData.tiktok" clearable placeholder="请输入TikTok" />
        </el-form-item>
        <el-form-item label="城市:" prop="city">
          <el-input v-model="formData.city" clearable placeholder="请输入城市" />
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
import { createMember, updateMember, findMember } from '@/api/bi/member';

defineOptions({
  name: 'MemberForm'
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
  group_id: undefined,
  gid: '',
  uid: '',
  avatar: '',
  nickname: '',
  name: '',
  business: '',
  mobile: '',
  wechat: '',
  qq: '',
  douyin: '',
  tiktok: '',
  city: '',
  remark: '',
  status: undefined
});
// 验证规则
const rule = reactive({});

const elFormRef = ref();

// 初始化方法
const init = async () => {
  // 建议通过url传参获取目标数据id 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
  if (route.query.id) {
    const res = await findMember({ id: route.query.id });
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
        res = await createMember(formData.value);
        break;
      case 'update':
        res = await updateMember(formData.value);
        break;
      default:
        res = await createMember(formData.value);
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
