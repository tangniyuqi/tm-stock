<template>
  <div ref="editorContainer" class="vditor-editor"></div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, watch, defineProps, defineEmits } from 'vue';
import { useThemeStore } from '@/pinia';
import { storeToRefs } from 'pinia';
import Vditor from 'vditor';
import 'vditor/dist/index.css';
import { getUrl } from '@/utils/image';

const themeStore = useThemeStore();
const { isDark } = storeToRefs(themeStore);
const basePath = import.meta.env.VITE_BASE_API;

const props = defineProps({
  modelValue: {
    type: String,
    default: ''
  },
  mode: {
    type: String,
    default: 'ir'
  },
  cacheId: {
    type: String,
    default: 'vditor-default-id'
  },
  minHeight: {
    type: [String, Number],
    default: '100px'
  },
  height: {
    type: [String, Number],
    default: 'auto'
  }
})

const emit = defineEmits(['update:modelValue'])

const editorContainer = ref(null);
const vditor = ref(null);
const theme = ref('classic');

if (isDark.value) {
  theme.value = 'dark';
}

// 初始化编辑器
const initEditor = () => {
  if (!editorContainer.value) return

  vditor.value = new Vditor(editorContainer.value, {
    theme: theme.value,
    width: '100%',
    height: props.height,
    minHeight: props.minHeight,
    mode: props.mode,
    value: props.modelValue,
    cache: {
      id: props.cacheId, // 添加 cache.id
      enable: true // 启用缓存
    },
    image: {
      isPreview: false
    },
    upload: {
      url: basePath + '/fileUploadAndDownload/upload?noSave=1',
      handler(files) {
        return new Promise((resolve, reject) => {
          const file = files[0];
          const formData = new FormData();
          formData.append('file', file);

          fetch(basePath + '/fileUploadAndDownload/upload?noSave=1', {
            method: 'POST',
            body: formData,
          })
            .then(response => response.json())
            .then(res => {
              if (res.code === 0 && res.data && res.data.file && res.data.file.url) {
                const url = getUrl(res.data.file.url);
                const markdownImage = `![${file.name}](${url})`;
                vditor.value.insertValue(markdownImage); // 手动插入到编辑器

                const result = JSON.stringify({
                  msg: '上传成功!',
                  code: 0,
                  data: {
                    errFiles: [file.name],
                    succMap: {
                      [file.name]: url,
                    },
                  },
                });

                resolve(result);
              } else {
                reject(new Error('上传失败!'));
              }
            })
            .catch(error => {
              reject(error);
            });
        });
      }
    },
    input: (value) => {
      emit('update:modelValue', value)
    },
    after: () => {
      vditor.value.setTheme(theme.value, theme.value, theme.value);
      vditor.value.setValue(props.modelValue);
    }
  })
}

// 响应外部内容变化
watch(() => props.modelValue, (newVal) => {
  if (vditor.value && vditor.value.getValue() !== newVal) {
    vditor.value.setValue(newVal)
  }
})

// 生命周期
onMounted(() => {
  initEditor()
})

onBeforeUnmount(() => {
  if (vditor.value) {
    vditor.value.destroy()
  }
})

// 暴露方法（可选）
defineExpose({
  getContent: () => vditor.value?.getValue(),
  setContent: (content) => vditor.value?.setValue(content)
})
</script>

<style lang="scss">
.vditor-editor {}

.vditor-reset ul {
  list-style-type: disc;
}

.vditor-reset ol {
  list-style-type: decimal;
}

.vditor-reset input {
  appearance: auto;
}

/* .vditor-reset img {
  max-width: 30%;
} */
</style>