<template>
  <div>
    <el-button v-if="showBtn" type="primary" :loading="state.btnLoading" @click="onCheckUpdate(false)">检查更新</el-button>

    <el-dialog v-model="state.checkVisible" title="检查更新" top="30vh" draggable destroy-on-close :close-on-click-modal="false" :close-on-press-escape="false" :show-close="false" :center="false">
      <div>
        <el-icon v-if="state.code == 1" :size="18" color="#67C23A" style="top: 4px"><SuccessFilled /></el-icon>
        <el-icon v-else-if="state.code == 0" :size="18" color="#E6A23C" style="top: 4px"><WarningFilled /></el-icon>
        <el-icon v-else-if="state.code == -1" :size="18" color="#F56C6C" style="top: 4px"><CircleCloseFilled /></el-icon>
        {{ state.msg }}
      </div>

      <div v-if="state.code == 0 && state.body != ''" class="update-info">
        <div v-for="item in state.body">{{ item }}</div>
      </div>

      <template #footer>
        <span class="dialog-footer">
          <el-link type="info" v-if="state.link != ''" class="mr-5" @click="onOpenLink">手动更新</el-link>
          <el-button v-if="state.code == 0" @click="state.checkVisible = false">取消</el-button>
          <el-button type="primary" @click="onConfirm">确认</el-button>
        </span>
      </template>
    </el-dialog>

    <el-dialog v-model="state.downloadVisible" title="下载更新" align-center draggable destroy-on-close :close-on-click-modal="false" :close-on-press-escape="false" :show-close="false">
      <div>
        <div class="mb6">
          <el-icon :size="14" class="is-loading mr2" style="top: 2px" color="#337ecc"><Loading /></el-icon>
          正在下载更新...
        </div>

        <el-progress :text-inside="true" :stroke-width="25" :percentage="state.downloadPercentage">
          <span>{{ state.downloadSizeShow }}</span>
        </el-progress>
      </div>

      <div v-if="state.link != ''" class="tip">
        若网速不理想，请尝试
        <span><el-link type="primary" @click="onOpenLink" class="tip-sd">手动更新</el-link></span>
      </div>
      
      <template #footer>
        <span class="dialog-footer">
          <el-link v-if="state.link != ''" class="mr-5" type="info" @click="onOpenLink">手动更新</el-link>
          <el-button @click="onCancel">取消</el-button>
          <el-button type="primary" @click="onBack">后台更新</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ElMessage } from 'element-plus';
import { reactive, onMounted } from 'vue';
import { Refresh, Loading, SuccessFilled, WarningFilled, CircleCloseFilled } from '@element-plus/icons-vue';

const props = defineProps({
  showBtn: {
    type: Boolean,
    default: true
  },
  autoCheck: {
    type: Boolean,
    default: true
  }
})

const state = reactive({
  checkVisible: false,
  btnLoading: false,
  code: 0, // 0=>有新版本; -1=>联网失败; 1=>已经是最新版本
  msg: '',
  link: '', // 手动更新网址
  body: [], // 版本介绍
  downloadVisible: false,
  downloadSizeShow: '', // 下载过程中大小数值展示
  downloadPercentage: 0, // 下载过程中大小百分比
  backUpdate: false, // 是否后台更新
  timer: ''
});

onMounted(() => {
  setPy2Js(); // 来自py的调用
  // 如果是路由跳转过来，window.pywebview可能已经存在，需要主动触发
  if (props.autoCheck && window.pywebview) {
    onCheckUpdate(true);
  }
});

// 监听pywebview是否已经准备好了
window.addEventListener('pywebviewready', async () => {
  if (props.autoCheck) {
    onCheckUpdate(true); // 程序第一次打开，自动检查更新
  }
});

const setPy2Js = () => {
  // 来自py的调用
  window['py2js_updateAppProgress'] = (res) => {
    const resDict = JSON.parse(res);
    // console.log('js', resDict)
    state.downloadSizeShow = resDict['sizeShow'];
    state.downloadPercentage = resDict['percentage'];
  };
};

// 检查更新
const onCheckUpdate = (init = false) => {
  if (state.backUpdate) {
    // 从后台更新恢复过来
    state.downloadVisible = true;
    state.btnLoading = false;
  } else {
    // 第一次打开
    state.btnLoading = true;
    window.pywebview.api.system_checkNewVersion().then((res) => {
      // console.log(init, res)
      // 程序第一次打开，自动检查更新 或 手动点击检查更新
      if (!init || res.code == 0) {
        state.code = res.code;
        state.msg = res.msg;
        if (res.link != undefined) {
          state.link = res.link;
        }
        if (res.body != undefined) {
          let body = res.body;
          body = body.replaceAll('\r', '');
          state.body = body.split('\n');
        }
        state.checkVisible = true;
      }
      state.btnLoading = false;
    });
  }
};

// 手动更新
const onOpenLink = () => {
  // console.log(state.link)
  window.pywebview.api.system_pyOpenFile(state.link);
  state.checkVisible = false;
};

// 确认更新 - 检查更新
const onConfirm = () => {
  state.checkVisible = false;
  if (state.code == 0) {
    state.downloadVisible = true;
    window.pywebview.api.system_downloadNewVersion().then((res) => {
      // console.log('res', res)
      state.downloadVisible = false;
      if (res.code == 0) {
        ElMessage.success('下载完成');
        state.btnLoading = false;
        window.pywebview.api.system_pyOpenFile(res.downloadPath);
      } else {
        ElMessage.error(res.msg);
      }
    });
  }
};

// 取消更新 - 下载更新
const onCancel = () => {
  state.backUpdate = false;
  state.downloadVisible = false;
  state.btnLoading = false;
  window.pywebview.api.system_cancelDownloadNewVersion();
};

// 后台更新 - 下载更新
const onBack = () => {
  state.backUpdate = true;
  state.downloadVisible = false;
  state.btnLoading = true;
};
</script>

<style scoped>
.update-info {
  margin-left: 20px;
  margin-top: 10px;
  padding: 10px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  overflow: scroll;
  background-color: var(--el-fill-color-dark);
  max-height: 150px;
}

.tip {
  margin-top: 10px;
  color: #a8abb2;
  font-size: 11px;
}

.tip-sd {
  font-size: 11px;
  top: -1px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  align-items: center;
}
</style>
