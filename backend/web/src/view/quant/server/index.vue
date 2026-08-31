<template>
  <div class="server-dashboard">
    <div class="server-list-view">
      <div class="gva-card-box">
        <div class="gva-card gva-top-card">
          <div class="gva-top-card-left">
            <div class="gva-top-card-title">服务器</div>
            <div class="gva-top-card-content">管理服务器实例配置（本地存储）</div>
          </div>

          <div class="gva-top-card-right">
            <el-button type="primary" icon="plus" @click="openAddServerDialog">添加服务器</el-button>
          </div>
        </div>
      </div>

      <el-card class="box-card">
        <div class="gva-table-box">
          <el-table :data="serverList" style="width: 100%">
            <el-table-column label="服务器名称" prop="name" min-width="150" show-overflow-tooltip />
    
            <el-table-column label="端口" prop="port" min-width="90" />

            <el-table-column label="TOKEN" prop="token" min-width="150" show-overflow-tooltip />

            <el-table-column label="客户端" prop="clientType" min-width="150">
              <template #default="scope">
                <el-tag>{{ scope.row.clientType || '未知' }}</el-tag>
              </template>
            </el-table-column>

            <el-table-column label="安装目录" prop="clientPath" min-width="220" show-overflow-tooltip />
            <el-table-column label="备注" prop="remark" min-width="150" />

            <el-table-column label="状态" width="100">
              <template #default="scope">
                <el-tag :type="scope.row.running ? 'success' : 'info'">{{ scope.row.running ? '运行中' : '未运行' }}</el-tag>
              </template>
            </el-table-column>

            <el-table-column label="操作" width="240" fixed="right">
              <template #default="scope">
                <el-button :type="scope.row.running ? 'danger' : 'success'" link :icon="scope.row.running ? 'video-pause' : 'video-play'" :loading="scope.row.loading" @click="toggleServerStatus(scope.row)">
                  {{ scope.row.running ? '停止' : '启动' }}
                </el-button>

                <el-button type="primary" link icon="edit" :disabled="scope.row.running" @click="editServer(scope.row)">编辑</el-button>
                <el-button type="danger" link icon="delete" :disabled="scope.row.running" @click="deleteServer(scope.$index)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-card>
    </div>

    <!-- Add/Edit Server Dialog -->
    <el-dialog v-model="serverDialogVisible" :title="isEdit ? '编辑服务器' : '添加服务器'" width="600px">
      <el-form :model="serverForm" label-width="100px">
        <el-form-item label="服务器名称" required>
          <el-input v-model="serverForm.name" clearable placeholder="如:默认服务器01" />
        </el-form-item>

        <el-form-item label="客户端类型" required>
          <el-select v-model="serverForm.clientType" placeholder="请选择客户端类型" style="width: 100%">
            <el-option label="通用同花顺 (universal_client)" value="universal_client" />
            <el-option label="同花顺 (ths)" value="ths" />
            <el-option label="通达信 (tdx)" value="tdx" />
            <el-option label="雪球 (xq)" value="xq" />
          </el-select>
        </el-form-item>

        <el-form-item label="安装目录" required>
          <el-input v-model="serverForm.clientPath" clearable placeholder="如: C:\同花顺软件\同花顺" />
        </el-form-item>

        <el-form-item label="服务端口" required>
          <div style="display: flex; width: 100%; gap: 10px">
            <el-select v-model="serverForm.protocol" style="width: 230px">
              <el-option label="http" value="http" />
              <el-option label="https" value="https" />
            </el-select>

            <el-input v-model.number="serverForm.port" clearable placeholder="端口（8000-9000）" style="width: 230px" />
          </div>
        </el-form-item>

        <el-form-item label="TOKEN">
          <div style="display: flex; width: 100%; gap: 10px">
            <el-input v-model="serverForm.token" clearable placeholder="验证Token" />
            <el-button @click="generateToken">随机生成</el-button>
          </div>
        </el-form-item>

        <el-form-item label="备注">
          <el-input v-model="serverForm.remark" type="textarea" placeholder="可选备注" />
        </el-form-item>
      </el-form>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="serverDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="saveServer">保存</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { startServer, stopServer, checkServerStatus } from '@/api/quant/serverManager';

defineOptions({
  name: 'QuantServer'
});

// ============ Server List Logic ============
const serverList = ref([]);
const serverDialogVisible = ref(false);
const isEdit = ref(false);
const editIndex = ref(-1);
const serverForm = reactive({
  name: '默认服务器01',
  clientType: 'universal_client',
  clientPath: 'C:\\同花顺软件\\同花顺',
  protocol: 'http',
  port: 8888,
  token: '',
  remark: ''
});

// Load servers from localStorage
onMounted(() => {
  const savedServers = localStorage.getItem('quant_servers');
  let hasData = false;
  if (savedServers) {
    try {
      const parsed = JSON.parse(savedServers);
      if (parsed && parsed.length > 0) {
        hasData = true;
        serverList.value = parsed.map((s) => {
          let newItem = s;
          if (s.url && !s.port) {
            try {
              const urlObj = new URL(s.url);
              newItem = {
                ...s,
                protocol: urlObj.protocol.replace(':', ''),
                port: urlObj.port || (urlObj.protocol === 'https:' ? 443 : 80),
                url: undefined // Remove old url field
              };
            } catch (e) {
              newItem = {
                ...s,
                protocol: 'http',
                port: 8888,
                url: undefined
              };
            }
          }
          return { ...newItem, running: false, loading: false };
        });
        checkAllServersStatus();
      }
    } catch (e) {
      console.error('Failed to parse saved servers', e);
    }
  }

  if (!hasData) {
    serverList.value = [{
      name: '默认服务器01',
      clientType: 'universal_client',
      clientPath: 'C:\\同花顺软件\\同花顺',
      protocol: 'http',
      port: 8888,
      token: '',
      remark: '',
      running: false,
      loading: false
    }];
    saveServersToLocal();
  }
});

const saveServersToLocal = () => {
  localStorage.setItem('quant_servers', JSON.stringify(serverList.value));
};

const openAddServerDialog = () => {
  isEdit.value = false;
  serverForm.name = '默认服务器01';
  serverForm.clientType = 'universal_client';
  serverForm.clientPath = 'C:\\同花顺软件\\同花顺';
  serverForm.protocol = 'http';
  serverForm.ip = '0.0.0.0';
  serverForm.port = 8888;
  serverForm.token = '';
  serverForm.remark = '';
  serverDialogVisible.value = true;
};

const editServer = (row) => {
  if (row.running) {
    ElMessage.warning('服务器正在运行中，请先停止后再编辑');
    return;
  }
  isEdit.value = true;
  editIndex.value = serverList.value.indexOf(row);
  Object.assign(serverForm, row);
  if (!serverForm.clientType) serverForm.clientType = 'universal_client';
  serverDialogVisible.value = true;
};

const generateToken = () => {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
  let token = '';
  for (let i = 0; i < 32; i++) {
    token += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  serverForm.token = token;
};

const saveServer = () => {
  if (!serverForm.name || !serverForm.port) {
    ElMessage.warning('名称和端口不能为空');
    return;
  }

  // Check for duplicate port
  const isDuplicatePort = serverList.value.some((server, index) => {
    // Skip checking against itself when editing
    if (isEdit.value && editIndex.value === index) return false;
    return parseInt(server.port) === parseInt(serverForm.port);
  });

  if (isDuplicatePort) {
    ElMessage.warning(`端口 ${serverForm.port} 已被使用，请更换其他端口`);
    return;
  }

  // Check for duplicate name
  const isDuplicateName = serverList.value.some((server, index) => {
    // Skip checking against itself when editing
    if (isEdit.value && editIndex.value === index) return false;
    return server.name === serverForm.name;
  });

  if (isDuplicateName) {
    ElMessage.warning(`名称 "${serverForm.name}" 已存在，请更换其他名称`);
    return;
  }

  if (isEdit.value && editIndex.value >= 0) {
    serverList.value[editIndex.value] = { ...serverForm };
  } else {
    serverList.value.push({ ...serverForm });
  }
  saveServersToLocal();
  serverDialogVisible.value = false;
  ElMessage.success('保存成功');
  checkAllServersStatus();
};

const deleteServer = (index) => {
  const server = serverList.value[index];
  if (server && server.running) {
    ElMessage.warning('服务器正在运行中，请先停止后再删除');
    return;
  }
  ElMessageBox.confirm('确定要删除该服务器配置吗?', '提示', {
    type: 'warning'
  }).then(() => {
    serverList.value.splice(index, 1);
    saveServersToLocal();
    ElMessage.success('删除成功');
  });
};

const checkAllServersStatus = async () => {
  const promises = serverList.value.map(async (server) => {
    try {
      if (server.loading) return; // Skip if currently toggling
      const res = await checkServerStatus(server);
      if (res.code === 200 || res.code === 0) {
        server.running = res.data.running;
      } else {
        // If check fails (e.g. connection refused), assume stopped
        server.running = false;
      }
    } catch (e) {
      // console.error('Failed to check status for', server.name, e);
      server.running = false;
    }
  });
  await Promise.all(promises);
};

const toggleServerStatus = async (row) => {
  if (row.loading) return;

  if (row.running) {
    // Stop server
    ElMessageBox.confirm(`确定要停止本地服务器 ${row.name} 吗?`, '提示', {
      type: 'warning'
    }).then(async () => {
      row.loading = true;
      try {
        const res = await stopServer(row);
        if (res.code === 200 || res.code === 0) {
          ElMessage.success('服务器停止成功');
          row.running = false;
        } else {
          ElMessage.error(res.msg || '停止失败');
        }
      } catch (e) {
        ElMessage.error('请求失败: ' + e.message);
      } finally {
        row.loading = false;
      }
    }).catch(() => {
      // Cancelled
    });
  } else {
    // Start server
    if (!window.pywebview) {
      ElMessage.warning('非客户端环境无法启动')
      return
    }
    row.loading = true;
    try {
      const res = await startServer(row);
      if (res.code === 200 || res.code === 0) {
        ElMessage.success('服务器启动成功');
        row.running = true;
        // Wait a bit for startup then verify
        setTimeout(() => {
          checkServerStatus(row).then(statusRes => {
            if (statusRes.code === 200 || statusRes.code === 0) {
              row.running = statusRes.data.running;
            }
          });
        }, 2000);
      } else {
        ElMessage.error(res.msg || '启动失败');
      }
    } catch (e) {
      ElMessage.error('请求失败: ' + e.message);
    } finally {
      row.loading = false;
    }
  }
};
</script>

<style scoped>
.server-dashboard {
  padding: 20px;
}
.gva-card-box {
  margin-bottom: 20px;
}
.gva-top-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background-color: var(--el-bg-color);
  padding: 20px;
  border-radius: 4px;
}
.gva-top-card-left {
  flex: 1;
  display: flex;
  flex-direction: column;
}
.gva-top-card-title {
  font-size: 20px;
  font-weight: bold;
  margin-bottom: 5px;
  color: var(--el-text-color-primary);
}
.gva-top-card-content {
  color: var(--el-text-color-secondary);
}
.gva-table-box {
  background-color: var(--el-bg-color);
  padding: 20px;
  border-radius: 4px;
}
</style>
