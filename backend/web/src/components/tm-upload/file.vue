<template>
  <div>
    <el-upload class="upload-btn" v-model:file-list="fileList" :action="computedAction" :accept="accept" :auto-upload="autoUpload" :disabled="disabled" :drag="drag" :limit="limit" :multiple="multiple" :show-file-list="showFileList" :headers="{ 'x-token': token }" :on-error="uploadError" :on-success="uploadSuccess" :on-remove="uploadRemove">
      <el-button type="primary" plain>{{ updateBtnName }}</el-button>
    </el-upload>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue';
import { useUserStore } from '@/pinia';
import { ElMessage } from 'element-plus';
import { getBaseUrl } from '@/utils/format';

defineOptions({
  name: 'UploadFile'
});

const props = defineProps({
  action: {
    type: String,
    default: getBaseUrl() + `/fileUploadAndDownload/upload`
  },
  accept: {
    type: String,
    default: ''
  },
  autoUpload: {
    type: Boolean,
    default: false
  },
  disabled: {
    type: Boolean,
    default: false
  },
  drag: {
    type: Boolean,
    default: false
  },
  multiple: {
    type: Boolean,
    default: false
  },
  showFileList: {
    type: Boolean,
    default: false
  },
  save: {
    type: Boolean,
    default: false
  },
  updateBtnName: {
    type: String,
    default: '文件上传'
  },
  limit: {
    type: Number,
    default: 1
  }
});

const userStore = useUserStore();
const token = userStore.token;
const fullscreenLoading = ref(false);
const model = defineModel({ type: Array });
const fileList = ref(model.value);
const emits = defineEmits(['on-success', 'on-error', 'on-remove']);

const computedAction = computed(() => {
  if (!props.save) {
    return `${props.action}?noSave=1`;
  }
  return props.action;
});

const uploadSuccess = (res) => {
  const { data, code } = res;

  if (code !== 0) {
    ElMessage({
      type: 'error',
      message: '上传失败' + res.msg
    });

    fileList.value.pop();
    return;
  }

  model.value.push({
    name: data.file.name,
    url: data.file.url
  });

  emits('on-success', res);
};

const uploadRemove = (file) => {
  const index = model.value.indexOf(file);

  if (index > -1) {
    model.value.splice(index, 1);
    fileList.value = model.value;
  }

  emits('on-remove', fileList.value);
};

const uploadError = (err) => {
  ElMessage({
    type: 'error',
    message: '上传失败'
  });

  fullscreenLoading.value = false;
  emits('on-error', err);
};
</script>