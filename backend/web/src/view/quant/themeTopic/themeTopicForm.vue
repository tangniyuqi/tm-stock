<template>
  <div>
    <div class="gva-form-box">
      <el-form :model="formData" ref="elFormRef" label-position="right" :rules="rule" label-width="80px">
        <el-form-item label="题材ID:" prop="themeId">
          <el-input v-model.number="formData.themeId" :clearable="true" placeholder="请输入题材ID" />
        </el-form-item>
        <el-form-item label="题材名称:" prop="themeName">
          <el-input v-model="formData.themeName" :clearable="true" placeholder="请输入题材名称" />
        </el-form-item>
        <el-form-item label="标题:" prop="title">
          <el-input v-model="formData.title" :clearable="true" placeholder="请输入标题" />
        </el-form-item>
        <el-form-item label="类型:" prop="type">
          <el-input v-model.number="formData.type" :clearable="true" placeholder="请输入类型" />
        </el-form-item>
        <el-form-item label="摘要:" prop="summary">
          <el-input v-model="formData.summary" :clearable="true" placeholder="请输入摘要" />
        </el-form-item>
        <el-form-item label="内容:" prop="content">
          <el-input v-model="formData.content" :clearable="true" placeholder="请输入内容" />
        </el-form-item>
        <el-form-item label="情绪:" prop="sentiment">
          <el-input v-model.number="formData.sentiment" :clearable="true" placeholder="请输入情绪" />
        </el-form-item>
        <el-form-item label="来源:" prop="source">
          <el-input v-model="formData.source" :clearable="true" placeholder="请输入来源" />
        </el-form-item>
        <el-form-item label="外链:" prop="url">
          <el-input v-model="formData.url" :clearable="true" placeholder="请输入外链" />
        </el-form-item>
        <el-form-item label="发布时间:" prop="publishTime">
          <el-date-picker v-model="formData.publishTime" type="date" style="width: 100%" placeholder="选择日期"
            :clearable="true" />
        </el-form-item>
        <el-form-item label="排序:" prop="sort">
          <el-input v-model.number="formData.sort" :clearable="true" placeholder="请输入排序" />
        </el-form-item>
        <el-form-item label="置顶:" prop="top">
          <el-input v-model.number="formData.top" :clearable="true" placeholder="请输入置顶" />
        </el-form-item>
        <el-form-item label="热门:" prop="hot">
          <el-input v-model.number="formData.hot" :clearable="true" placeholder="请输入热门" />
        </el-form-item>
        <el-form-item label="热度值:" prop="heat">
          <el-input v-model.number="formData.heat" :clearable="true" placeholder="请输入热度值" />
        </el-form-item>
        <el-form-item label="浏览量:" prop="view">
          <el-input v-model.number="formData.view" :clearable="true" placeholder="请输入浏览量" />
        </el-form-item>
        <el-form-item label="分享数:" prop="share">
          <el-input v-model.number="formData.share" :clearable="true" placeholder="请输入分享数" />
        </el-form-item>
        <el-form-item label="状态:" prop="status">
          <el-input v-model.number="formData.status" :clearable="true" placeholder="请输入状态" />
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
import { createThemeTopic, updateThemeTopic, findThemeTopic } from '@/api/quant/themeTopic';

defineOptions({
  name: 'TopicForm',
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
  themeId: undefined,
  themeName: '',
  title: '',
  type: undefined,
  summary: '',
  content: '',
  sentiment: undefined,
  source: '',
  url: '',
  publishTime: new Date(),
  sort: undefined,
  top: undefined,
  hot: undefined,
  heat: undefined,
  view: undefined,
  share: undefined,
  status: undefined,
});
// 验证规则
const rule = reactive({
  title: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur'],
    },
  ],
});

const elFormRef = ref();

// 初始化方法
const init = async () => {
  // 建议通过url传参获取目标数据ID 调用 find方法进行查询数据操作 从而决定本页面是create还是update 以下为id作为url参数示例
  if (route.query.id) {
    const res = await findThemeTopic({ ID: route.query.id });
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
        res = await createThemeTopic(formData.value);
        break;
      case 'update':
        res = await updateThemeTopic(formData.value);
        break;
      default:
        res = await createThemeTopic(formData.value);
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
