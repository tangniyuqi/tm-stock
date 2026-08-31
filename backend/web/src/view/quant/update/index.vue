<template>
  <div class="update-page">
    <div class="gva-card-box">
      <el-card class="box-card">
        <template #header>
          <div class="card-header">
            <span class="title">客户端下载&升级</span>

            <tm-BtnUpdate v-if="isClient" />
          </div>
        </template>

        <div v-if="releaseInfo" class="release-info">
          <el-descriptions title="最新版本信息" :column="1" border label-width="200px">
            <el-descriptions-item label="软件名称">{{ releaseInfo.name }}</el-descriptions-item>
            <el-descriptions-item label="版本号">
              {{ releaseInfo.version }}
              <span v-if="currentVersion">（本机版本：v{{ currentVersion.replace(/^v/, '') }}）</span>
            </el-descriptions-item>
            <el-descriptions-item label="发布时间">{{ formatDate(releaseInfo.published_at) }}</el-descriptions-item>
            <el-descriptions-item label="更新内容">
              <div class="markdown-body" v-html="renderMarkdown(releaseInfo.log[0].content)"></div>
            </el-descriptions-item>
            <el-descriptions-item label="软件简介">
              <div class="markdown-body" v-html="renderMarkdown(releaseInfo.content)"></div>
            </el-descriptions-item>
          </el-descriptions>

          <div class="assets-section" v-if="releaseInfo.assets && releaseInfo.assets.length">
            <h3>安装包下载</h3>
            <el-table :data="releaseInfo.assets" style="width: 100%">
              <el-table-column prop="name" label="文件名" />
              <el-table-column prop="size" label="大小" width="120">
                <template #default="scope">
                  {{ formatSize(scope.row.size) }}
                </template>
              </el-table-column>
              <el-table-column label="操作" width="120" fixed="right">
                <template #default="scope">
                  <el-button type="primary" plain @click="downloadAsset(scope.row.url)">立即下载</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <div class="history-section" v-if="releaseInfo.log && releaseInfo.log.length">
            <el-divider content-position="left">历史更新日志</el-divider>
            <el-timeline>
              <el-timeline-item v-for="(item, index) in releaseInfo.log" :key="index" :timestamp="item.date" placement="top" :type="index === 0 ? 'primary' : ''">
                <el-card class="history-card">
                  <h4>{{ item.version }}</h4>
                  <div class="markdown-body" v-html="renderMarkdown(item.content)"></div>
                </el-card>
              </el-timeline-item>
            </el-timeline>
          </div>
        </div>

        <div v-else-if="searched && !loading" class="empty-state">
          <el-empty description="未找到更新信息或检查失败" />
        </div>

        <div v-else class="initial-state">
          <el-empty description="点击上方按钮检查更新" :image-size="100" />
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import service from '@/utils/request';
import { marked } from 'marked';
import moment from 'moment';
import { ElMessage } from 'element-plus';
import TmBtnUpdate from '@/components/tm-update/BtnUpdate.vue';

defineOptions({
  name: 'ClientUpdate'
});

const loading = ref(false);
const releaseInfo = ref(null);
const searched = ref(false);
const isClient = ref(false);
const currentVersion = ref('');

const checkUpdate = async () => {
  loading.value = true;
  searched.value = true;
  releaseInfo.value = null;
  try {
    const res = await service({
      url: '/quant/update/client',
      method: 'get'
    });
    if (res.code === 0 && res.data) {
      releaseInfo.value = res.data;
    }
  } catch (error) {
    console.error('Check update failed:', error);
  } finally {
    loading.value = false;
  }
};

const renderMarkdown = (text) => {
  if (!text) return '';
  const formattedText = text.replace(/\n/g, '  \n');
  return marked.parse(formattedText);
};

const formatDate = (dateStr) => {
  if (!dateStr) return '';
  return moment.utc(dateStr).format('YYYY-MM-DD HH:mm:ss');
};

const formatSize = (bytes) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const downloadAsset = (url) => {
  if (window.pywebview) {
    window.pywebview.api.system_pyOpenFile(url);
  } else {
    window.open(url, '_blank');
  }
};

// 在挂载时获取数据
onMounted(() => {
  if (typeof window.pywebview !== 'undefined') {
    isClient.value = true;
    getCurrentVersion();
  } else {
    window.addEventListener('pywebviewready', () => {
      isClient.value = true;
      getCurrentVersion();
    });
  }
  checkUpdate();
});

const getCurrentVersion = async () => {
  if (window.pywebview) {
    try {
      const info = await window.pywebview.api.system_getAppInfo();
      if (info && info.appVersion) {
        currentVersion.value = info.appVersion;
      }
    } catch (e) {
      console.error('Failed to get app info', e);
    }
  }
};
</script>

<style scoped>
.update-page {
  padding: 20px;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.title {
  font-weight: bold;
  font-size: 16px;
}
.btn-group {
  display: flex;
  gap: 10px;
}
.release-info {
  margin-top: 20px;
}
.assets-section {
  margin-top: 30px;
}
.assets-section h3 {
  margin-bottom: 15px;
  font-size: 16px;
}
.history-section {
  margin-top: 30px;
}
.history-card h4 {
  margin-top: 0;
  margin-bottom: 10px;
}
.markdown-body {
  line-height: 1.6;
}
.initial-state {
  padding: 40px 0;
}
</style>
