<template>
  <div>
    <!-- =========== 任务详情视图（带 task_id 或选中任务时） =========== -->
    <div v-if="detailVisible" class="ai-task-detail">
      <div class="gva-btn-list my-3">
        <el-button icon="back" @click="backToList">返回列表</el-button>
        <el-button v-if="task.status === 0 || task.status === 4" icon="refresh" :loading="refreshing"
          @click="loadTask">刷新</el-button>
        <el-button v-if="task.status === 0 || task.status === 4" type="danger" icon="video-pause" :loading="acting"
          @click="onStopTask">停止</el-button>
        <el-button v-if="task.status === 2 || task.status === 3" type="success" icon="video-play" :loading="acting"
          @click="onRestartTask">启动</el-button>
      </div>

      <el-card shadow="never" class="mb-3">
        <template #header>
          <div class="flex justify-between items-center">
            <span class="font-bold">任务 #{{ task.id }} · {{ taskTypeLabel }}</span>
            <el-tag :type="statusTagType(task.status)" :effect="task.status === 0 ? 'dark' : 'light'">{{
              statusLabel(task.status) }}</el-tag>
          </div>
        </template>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="任务名称">{{ task.name || '-' }}</el-descriptions-item>
          <el-descriptions-item label="大模型">{{ taskParams?.model || '-' }}</el-descriptions-item>
          <el-descriptions-item label="任务类型">{{ taskTypeLabel }}</el-descriptions-item>
          <el-descriptions-item label="是否联网">
            <el-tag v-if="taskParams?.web_search" type="success" size="small">是</el-tag>
            <el-tag v-else type="danger" size="small">否</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="子任务数">{{ task.count ?? '-' }}</el-descriptions-item>
          <el-descriptions-item label="当前处理">{{ task.current || '-' }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatToDateTime(task.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="完成时间">{{ task.finished_at ? formatToDateTime(task.finished_at) : '-'
            }}</el-descriptions-item>
          <el-descriptions-item label="计划执行">
            {{ task.scheduled_at ? formatToDateTime(task.scheduled_at) : '立即执行' }}
          </el-descriptions-item>
        </el-descriptions>
      </el-card>

      <el-card shadow="never" class="mb-3">
        <template #header><span class="font-bold">执行进度</span></template>
        <el-progress :percentage="progressPercent" :status="progressStatus" :stroke-width="18" :text-inside="true" />
        <div class="mt-2 text-sm text-gray-500">已完成 {{ task.done }} / {{ task.total }}</div>
      </el-card>

      <el-card v-if="task.status === 1 && resultSummary" shadow="never" class="mb-3">
        <template #header><span class="font-bold">执行结果</span></template>
        <el-descriptions :column="1" border>
          <el-descriptions-item v-for="(value, key) in resultSummary" :key="key" :label="resultLabel(key)">
            <span v-if="Array.isArray(value)">
              <div v-for="(item, idx) in value" :key="idx" class="result-line">{{ item }}</div>
            </span>
            <span v-else>{{ value }}</span>
          </el-descriptions-item>
        </el-descriptions>
      </el-card>

      <el-card v-if="task.status === 2" shadow="never" class="mb-3">
        <template #header><span class="font-bold text-red-500">失败原因</span></template>
        <pre class="whitespace-pre-wrap">{{ task.error || '-' }}</pre>
      </el-card>

      <el-card shadow="never">
        <template #header><span class="font-bold">执行日志</span></template>
        <div class="ai-task-logs" ref="logsRef">
          <el-timeline v-if="logs.length">
            <el-timeline-item v-for="(log, idx) in logs" :key="idx" :timestamp="log.time" placement="top">
              <div class="log-text">{{ log.text }}</div>
            </el-timeline-item>
          </el-timeline>
          <el-empty v-else description="暂无日志" :image-size="60" />
        </div>
      </el-card>
    </div>

    <!-- =========== 任务列表视图 =========== -->
    <div v-else>
      <div class="gva-search-box">
        <el-form :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
          <el-form-item label="任务类型" prop="type">
            <el-select v-model="searchInfo.type" clearable placeholder="全部" style="width: 160px">
              <el-option v-for="item in taskTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="状态" prop="status">
            <el-select v-model="searchInfo.status" clearable placeholder="全部" style="width: 120px">
              <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
            <el-button icon="refresh" @click="onReset">重置</el-button>
          </el-form-item>
        </el-form>
      </div>

      <div class="gva-table-box">
        <el-table :data="tableData" style="width: 100%" @row-click="viewDetail" row-class-name="clickable-row">
          <el-table-column label="ID" prop="id" width="80" />

          <el-table-column label="类型" min-width="150">
            <template #default="scope">
              {{ taskTypeMap[scope.row.type]?.label || scope.row.type || '-' }}
            </template>
          </el-table-column>

          <el-table-column label="名称" prop="name" min-width="240" show-overflow-tooltip>
            <template #default="scope">{{ scope.row.name || '-' }}</template>
          </el-table-column>

          <el-table-column label="大模型" min-width="180" show-overflow-tooltip>
            <template #default="scope">{{ rowParams(scope.row)?.model || '-' }}</template>
          </el-table-column>

          <el-table-column label="联网" min-width="60" align="center">
            <template #default="scope">
              <el-tag v-if="rowParams(scope.row)?.web_search" type="success" size="small">是</el-tag>
              <el-tag v-else type="info" size="small">否</el-tag>
            </template>
          </el-table-column>

          <el-table-column label="数量" prop="count" min-width="80" align="center" />

          <el-table-column label="进度" min-width="180">
            <template #default="scope">
              <el-progress class="task-progress" :percentage="calcPercent(scope.row)"
                :status="scope.row.status === 2 ? 'exception' : ''" :stroke-width="12" :text-inside="true" />
            </template>
          </el-table-column>

          <el-table-column label="状态" min-width="120" align="center">
            <template #default="scope">
              <el-tag :type="statusTagType(scope.row.status)">{{ statusLabel(scope.row.status) }}</el-tag>
            </template>
          </el-table-column>

          <el-table-column label="创建时间" min-width="150">
            <template #default="scope">{{ formatToDateTime(scope.row.created_at, 'yyyy-MM-DD HH:mm') }}</template>
          </el-table-column>

          <el-table-column label="操作" min-width="160">
            <template #default="scope">
              <el-button type="primary" link icon="view" @click.stop="viewDetail(scope.row)">查看</el-button>
              <el-button v-if="scope.row.status === 0 || scope.row.status === 4" type="danger" link icon="video-pause"
                :loading="acting" @click.stop="onStopTask(scope.row)">停止</el-button>
              <el-button v-else-if="scope.row.status === 2 || scope.row.status === 3" type="success" link
                icon="video-play" :loading="acting" @click.stop="onRestartTask(scope.row)">启动</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="gva-pagination">
          <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize"
            :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange"
            @size-change="handleSizeChange" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount, nextTick } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { formatToDateTime } from '@/utils/dateTimeUtils';
import { getAiTask, getAiTaskList, stopAiTask, restartAiTask } from '@/api/quant/aiTask';

defineOptions({
  name: 'AiTask',
});

const route = useRoute();

// 任务类型映射
const taskTypeOptions = [
  { value: 'ai_add', label: '题材股票（选股）' },
  { value: 'ai_update_one', label: '题材股票' },
  { value: 'ai_update_batch', label: '题材股票（批量）' },
  { value: 'ai_analyze', label: '基础股票' },
];
const taskTypeMap = Object.fromEntries(taskTypeOptions.map(item => [item.value, item]));

// 状态映射：0=运行中 1=成功 2=失败 3=已取消 4=待调度（定时任务等待计划时间）
const statusOptions = [
  { value: 0, label: '运行中' },
  { value: 1, label: '成功' },
  { value: 2, label: '失败' },
  { value: 3, label: '已取消' },
  { value: 4, label: '待调度' },
];
const statusMap = Object.fromEntries(statusOptions.map(item => [item.value, item]));

const statusLabel = status => statusMap[status]?.label || '未知';
const statusTagType = status => {
  if (status === 0) return 'primary';
  if (status === 1) return 'success';
  if (status === 2) return 'danger';
  if (status === 4) return 'warning';
  return 'info';
};

// =========== 详情视图 ===========
const detailVisible = ref(false);
const task = ref({ status: 0, done: 0, total: 0, logs: '' });
const refreshing = ref(false);
const logsRef = ref();
let pollTimer = null;
let currentTaskId = null;

// 当前任务参数（脱敏摘要，JSON 字符串解析）
const taskParams = computed(() => {
  if (!task.value.params) return null;
  try {
    return JSON.parse(task.value.params);
  } catch {
    return null;
  }
});

// 当前任务结果（JSON 字符串解析）
const taskResult = computed(() => {
  if (!task.value.result) return null;
  try {
    return JSON.parse(task.value.result);
  } catch {
    return null;
  }
});

const taskTypeLabel = computed(() => taskTypeMap[task.value.type]?.label || task.value.type || '-');
const progressPercent = computed(() => {
  const { done, total } = task.value;
  if (!total) return 0;
  return Math.min(100, Math.round((done / total) * 100));
});
const progressStatus = computed(() => {
  if (task.value.status === 2) return 'exception';
  if (task.value.status === 1) return 'success';
  return '';
});

// 执行日志：按行拆分 [HH:mm:ss] 时间戳与文本；无时间戳的续行沿用上一行时间，
// 保证时间线每个节点的位置一致整齐，避免出现无时间戳行导致节点悬空/参差
const logs = computed(() => {
  const raw = task.value.logs || '';
  let lastTime = '';
  return raw
    .split('\n')
    .filter(line => line.trim())
    .map(line => {
      const match = line.match(/^\[(\d{2}:\d{2}:\d{2})\]\s*(.*)$/);
      if (match) {
        lastTime = match[1];
        return { time: match[1], text: match[2] };
      }
      return { time: lastTime, text: line };
    });
});

// 结果展示 label
const resultLabel = key => {
  const labels = { added: '新增', updated: '覆盖更新', flagged: '自动下架暴雷', skipped: '跳过明细', success: '成功', fail: '失败', analyzed: '分析成功', total: '总数量', failed: '失败明细' };
  return labels[key] || key;
};

// 按任务类型组装结果摘要（描述顺序展示）
const resultSummary = computed(() => {
  const result = taskResult.value;
  if (!result) return null;
  if (task.value.type === 'ai_add') {
    return {
      added: result.added ?? 0,
      updated: result.updated ?? 0,
      flagged: result.flagged ?? 0,
      skipped: result.skipped || [],
    };
  }
  if (task.value.type === 'ai_update_one') {
    return { stock_name: result.stock_name || '-' };
  }
  if (task.value.type === 'ai_update_batch') {
    const list = result || [];
    const successCount = list.filter(item => item?.success).length;
    const failList = list.filter(item => !item?.success);
    return {
      success: `${successCount} / ${list.length}`,
      fail: failList.map(item => `${item.stock_name || `ID:${item.id}`}：${item.message || '未知原因'}`),
    };
  }
  if (task.value.type === 'ai_analyze') {
    return {
      analyzed: result.analyzed ?? 0,
      total: result.total ?? 0,
      failed: result.failed || [],
    };
  }
  return result;
});

// 加载任务详情
const loadTask = async () => {
  if (currentTaskId === null || currentTaskId === undefined) return;
  refreshing.value = true;
  const res = await getAiTask({ id: currentTaskId });
  refreshing.value = false;
  if (res.code === 0) {
    task.value = res.data;
    // 完成后停止轮询（待调度任务持续轮询以检测自动启动）
    if (task.value.status !== 0 && task.value.status !== 4 && pollTimer) {
      stopPoll();
    } else if ((task.value.status === 0 || task.value.status === 4) && !pollTimer) {
      startPoll();
    }
    // 日志滚动到底部
    await nextTick();
    if (logsRef.value) {
      logsRef.value.scrollTop = logsRef.value.scrollHeight;
    }
  }
};

const startPoll = () => {
  stopPoll();
  pollTimer = setInterval(loadTask, 2000);
};

const stopPoll = () => {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
};

// 进入详情：按任务ID加载并开始轮询
const viewDetail = row => {
  currentTaskId = row.id;
  task.value = { ...row };
  detailVisible.value = true;
  startPoll();
};

// 返回列表
const backToList = () => {
  stopPoll();
  currentTaskId = null;
  detailVisible.value = false;
  getTableData();
};

// 任务操作（停止/启动）请求进行中标识
const acting = ref(false);

// 停止运行中的任务
const onStopTask = async row => {
  const taskId = detailVisible.value ? currentTaskId : row.id;
  try {
    await ElMessageBox.confirm('确定停止该任务吗？停止后任务将标记为已取消。', '提示', { type: 'warning', confirmButtonText: '停止', cancelButtonText: '取消' });
  } catch {
    return;
  }
  acting.value = true;
  try {
    const res = await stopAiTask({ id: taskId });
    if (res.code === 0) {
      ElMessage.success('任务已停止');
      if (detailVisible.value) {
        loadTask();
      } else {
        getTableData();
      }
    }
  } catch (e) {
    // 请求异常（网络错误/HTTP错误）已由全局错误处理提示，这里兜底
    if (detailVisible.value) {
      loadTask();
    }
  } finally {
    acting.value = false;
  }
};

// 启动已取消的任务
const onRestartTask = async row => {
  const taskId = detailVisible.value ? currentTaskId : row.id;
  try {
    await ElMessageBox.confirm('确定重新启动该任务吗？将按原参数重新执行。', '提示', { type: 'warning', confirmButtonText: '启动', cancelButtonText: '取消' });
  } catch {
    return;
  }
  acting.value = true;
  try {
    const res = await restartAiTask({ id: taskId });
    if (res.code === 0) {
      ElMessage.success('任务已重新启动');
    }
    // 无论成功与否都刷新，避免界面停留在旧状态
    if (detailVisible.value) {
      loadTask();
    } else {
      getTableData();
    }
  } catch (e) {
    if (detailVisible.value) {
      loadTask();
    }
  } finally {
    acting.value = false;
  }
};

// 从路由参数进入详情（themeStock 提交后跳转 name: AiTask 并携带 task_id）
const initFromRoute = async () => {
  const rawTaskId = route.query.task_id;
  if (rawTaskId === undefined || rawTaskId === null || rawTaskId === '') {
    // 无任务ID（如从菜单直接进入）：加载列表
    getTableData();
    return;
  }
  const taskId = Number(Array.isArray(rawTaskId) ? rawTaskId[0] : rawTaskId);
  if (Number.isNaN(taskId)) {
    getTableData();
    return;
  }
  const res = await getAiTask({ id: taskId });
  if (res.code === 0) {
    currentTaskId = taskId;
    task.value = res.data;
    detailVisible.value = true;
    startPoll();
  } else {
    // 任务查询失败（如权限不足）时回退到列表
    getTableData();
  }
};

// 监听路由参数变化：页面被 keep-alive 缓存时，从 themeStock 再次跳转进入需重新加载对应任务
watch(
  () => route.query.task_id,
  newVal => {
    if (newVal === undefined || newVal === null || newVal === '') {
      backToList();
      return;
    }
    initFromRoute();
  }
);

// =========== 列表视图 ===========
const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
const tableData = ref([]);
const searchInfo = ref({ type: undefined, status: undefined });

const calcPercent = row => {
  if (!row.total) return 0;
  return Math.min(100, Math.round((row.done / row.total) * 100));
};

// 解析单行任务的参数摘要（JSON 字符串），用于展示大模型、是否联网等字段
const rowParams = row => {
  if (!row.params) return null;
  try {
    return JSON.parse(row.params);
  } catch {
    return null;
  }
};

const getTableData = async () => {
  const table = await getAiTaskList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
  if (table.code === 0) {
    tableData.value = table.data.list;
    total.value = table.data.total;
    page.value = table.data.page;
    pageSize.value = table.data.pageSize;
  }
};

const onSubmit = () => {
  page.value = 1;
  getTableData();
};

const onReset = () => {
  searchInfo.value = { type: undefined, status: undefined };
  page.value = 1;
  getTableData();
};

const handleSizeChange = val => {
  pageSize.value = val;
  getTableData();
};

const handleCurrentChange = val => {
  page.value = val;
  getTableData();
};

initFromRoute();

onBeforeUnmount(() => {
  stopPoll();
});
</script>

<style scoped>
.ai-task-logs {
  max-height: 420px;
  overflow-y: auto;
  padding-right: 8px;
}

.log-text {
  font-size: 13px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.result-line {
  font-size: 13px;
  line-height: 1.7;
}

.clickable-row {
  cursor: pointer;
}

/* 列表进度条：内部百分比文字垂直居中（默认 line-height 超出条高导致不居中） */
.task-progress :deep(.el-progress-bar__inner) {
  display: flex;
  align-items: center;
}

.task-progress :deep(.el-progress-bar__innerText) {
  line-height: 1;
  margin: 0 5px;
}

.mb-3 {
  margin-bottom: 12px;
}

.mt-2 {
  margin-top: 8px;
}
</style>
