<template>
  <div class="screener-page">
    <!-- 页面标题卡片 -->
    <el-card shadow="never" class="header-card">
      <div class="page-header">
        <el-icon :size="32" color="#409EFF"><Document /></el-icon>
        <div class="header-content">
          <h2>AI选股器</h2>
          <p>输入条件，多维筛选，精准捕捉</p>
        </div>
      </div>
    </el-card>

    <el-card shadow="never" class="search-card">
      <el-form :inline="false" @submit.prevent class="form-center">
        <el-form-item class="form-item-fixed">
          <div class="textarea-wrapper">
            <el-input v-model="query" type="textarea" :autosize="{ minRows: 3, maxRows: 12 }" class="query-textarea" placeholder="选择或输入条件，例如：涨停，量比大于2，总市值大于100亿（多个条件用逗号隔开）" />

            <div class="bottom-actions">
              <el-popover placement="bottom-start" :width="580" trigger="hover">
                <template #reference>
                  <el-button type="info" plain icon="Clock"> 历史记录 </el-button>
                </template>

                <div class="history-container">
                  <div class="history-header">
                    <span class="history-title">查询历史</span>
                    <el-button type="danger" text @click="clearAllHistory">清空</el-button>
                  </div>

                  <div v-if="searchHistory.length === 0" class="history-empty">暂无历史记录</div>
                  <div v-else class="history-list">
                    <div v-for="(item, index) in searchHistory" :key="index" class="history-item">
                      <span class="history-text" @click="selectHistory(item)">{{ item }}</span>
                      <el-icon class="history-delete" @click="deleteHistory(index)"><Close /></el-icon>
                    </div>
                  </div>
                </div>
              </el-popover>

              <el-button type="primary" plain class="condition-btn" @click="conditionDialogVisible = true" :disabled="loading"> 条件选择 </el-button>

              <div class="pro-actions">
                <el-switch v-model="pro" active-text="PRO" />
                <el-button v-if="pro" type="warning" plain size="small" @click="cookieDialogVisible = true"> 设置COOKIE </el-button>
              </div>
            </div>
          </div>
        </el-form-item>

        <el-form-item class="form-actions">
          <el-button type="primary" icon="search" @click="handleSearch" :disabled="loading" :loading="loading"> 查询 </el-button>
          <el-button type="danger" icon="close" @click="handleStop" :disabled="!loading"> 停止 </el-button>
          <el-button icon="refresh" @click="handleReset" :disabled="loading"> 重置 </el-button>
        </el-form-item>
      </el-form>

      <el-dialog v-model="conditionDialogVisible" title="条件选择" width="70%" top="320px" :modal="false">
        <div class="condition-area">
          <div v-for="category in conditionOptions" :key="category.id" class="condition-category">
            <div class="category-name">【{{ category.name }}】</div>
            <div class="category-items">
              <template v-for="subCategory in category.options" :key="subCategory.id">
                <span v-if="!subCategory.options" class="condition-tag" @click="appendCondition(subCategory.realname || subCategory.name)">
                  {{ subCategory.name }}
                </span>
                <template v-else>
                  <el-dropdown trigger="click" @command="appendCondition">
                    <span class="condition-tag dropdown-tag">
                      {{ subCategory.name }} <el-icon><ArrowDown /></el-icon>
                    </span>
                    <template #dropdown>
                      <el-dropdown-menu style="max-height: 300px; overflow-y: auto">
                        <el-dropdown-item v-for="item in subCategory.options" :key="item.id" :command="item.realname || item.name">
                          {{ item.name }}
                        </el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </template>
              </template>
            </div>
          </div>
        </div>

        <template #footer>
          <div class="dialog-footer">
            <el-button @click="handleReset" :disabled="loading">清空</el-button>
            <el-button type="primary" @click="conditionDialogVisible = false">完成</el-button>
          </div>
        </template>
      </el-dialog>

      <el-dialog v-model="cookieDialogVisible" title="设置COOKIE" width="500px">
        <el-input v-model="cookieValue" type="textarea" :rows="4" placeholder="COOKIE粘贴到这里..." />
        <template #footer>
          <span class="dialog-footer">
            <el-button @click="cookieDialogVisible = false">取消</el-button>
            <el-button type="primary" @click="saveCookie">保存</el-button>
          </span>
        </template>
      </el-dialog>
    </el-card>

    <el-card shadow="never" class="table-card">
      <div v-if="!hasSearched && tableData.length === 0" class="empty-tip">
        <el-empty description="请输入指令进行选股" />
      </div>

      <el-table v-else v-loading="loading" :data="paginatedData" style="width: 100%" :height="tableHeight" border stripe scrollbar-always-on>
        <el-table-column v-for="col in columns" :key="col.prop" :prop="col.prop" :label="col.label" :min-width="col.minWidth" :sortable="isSortableColumn(col.prop)" :sort-method="(a, b) => sortMethod(a, b, col.prop)" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="col.prop === '最新涨跌幅'" :class="rateClass(row[col.prop])">{{ formatNumber(row[col.prop]) }}%</span>
            <span v-else>{{ formatNumber(row[col.prop]) }}</span>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper" v-if="tableData.length > 0">
        <el-button :icon="isTableExpanded ? 'ArrowDown' : 'ArrowUp'" @click="toggleTableHeight">
          {{ isTableExpanded ? '收起' : '展开' }}
        </el-button>
        <el-pagination v-model:current-page="currentPage" v-model:page-size="pageSize" :page-sizes="[10, 20, 50, 100]" :background="true" layout="total, sizes, prev, pager, next, jumper" :total="tableData.length" @size-change="handleSizeChange" @current-change="handleCurrentChange" />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ArrowDown, Close } from '@element-plus/icons-vue';
import { queryWencai } from '@/api/quant/wencai';
import { conditionOptions } from '@/data/conditionOptions';
import { createScreenerRecord } from '@/api/quant/screenerRecord';

defineOptions({
  name: 'WencaiScreener',
});

const query = ref('');
const loading = ref(false);
const tableData = ref([]);
const columns = ref([]);
const hasSearched = ref(false);
const conditionDialogVisible = ref(false);
let abortController = null;

const currentPage = ref(1);
const pageSize = ref(20);

// 表格高度控制
const isTableExpanded = ref(false);
const tableHeight = computed(() => {
  return isTableExpanded.value ? 'calc(100vh - 200px)' : 'calc(100vh - 420px)';
});

const toggleTableHeight = () => {
  isTableExpanded.value = !isTableExpanded.value;
};

const paginatedData = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value;
  const end = start + pageSize.value;
  return tableData.value.slice(start, end);
});

const handleSizeChange = val => {
  pageSize.value = val;
  currentPage.value = 1;
};

const handleCurrentChange = val => {
  currentPage.value = val;
};

const pro = ref(localStorage.getItem('quant_pro') === 'true');
const cookieDialogVisible = ref(false);
const cookieValue = ref(localStorage.getItem('quant_cookie') || '');

// 历史记录
const searchHistory = ref([]);

// 加载历史记录
const loadHistory = () => {
  const saved = localStorage.getItem('quant_search_history');
  if (saved) {
    try {
      searchHistory.value = JSON.parse(saved);
    } catch (e) {
      searchHistory.value = [];
    }
  }
};

// 保存历史记录
const saveHistory = () => {
  localStorage.setItem('quant_search_history', JSON.stringify(searchHistory.value));
};

// 添加历史记录
const addHistory = queryText => {
  if (!queryText || !queryText.trim()) return;
  const trimmed = queryText.trim();
  // 移除重复项
  searchHistory.value = searchHistory.value.filter(item => item !== trimmed);
  // 添加到开头
  searchHistory.value.unshift(trimmed);
  // 限制最多保存20条
  if (searchHistory.value.length > 20) {
    searchHistory.value = searchHistory.value.slice(0, 20);
  }
  saveHistory();
};

// 选择历史记录
const selectHistory = item => {
  query.value = item;
};

// 删除单条历史记录
const deleteHistory = index => {
  searchHistory.value.splice(index, 1);
  saveHistory();
};

// 清空所有历史记录
const clearAllHistory = () => {
  ElMessageBox.confirm('确定要清空所有历史记录吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  })
    .then(() => {
      searchHistory.value = [];
      saveHistory();
      ElMessage.success('已清空历史记录');
    })
    .catch(() => {});
};

watch(pro, newVal => {
  localStorage.setItem('quant_pro', String(newVal));
});

const saveCookie = () => {
  localStorage.setItem('quant_cookie', cookieValue.value);
  cookieDialogVisible.value = false;
  ElMessage.success('Cookie 已保存');
};

onMounted(() => {
  loadHistory();
});

const appendCondition = condition => {
  if (!condition) return;
  const sep = '，';
  const parts = query.value
    .split(/[，,]/)
    .map(s => s.trim())
    .filter(Boolean);
  if (!parts.includes(condition)) {
    parts.push(condition);
  }
  query.value = parts.join(sep);
};

const rateClass = val => {
  if (val === null || val === undefined || val === '') return '';
  const num = parseFloat(val);
  if (Number.isNaN(num)) return '';
  if (num > 0) return 'rate-rise';
  if (num < 0) return 'rate-fall';
  return '';
};

const formatNumber = val => {
  if (val === null || val === undefined || val === '') return val;
  if (typeof val === 'number') {
    return Number.isInteger(val) ? val : Number(val.toFixed(2));
  }
  if (typeof val === 'string') {
    const str = val.trim();
    if (!str.includes('.')) return str;

    const num = Number(str);
    if (!Number.isNaN(num)) return Number(num.toFixed(2));
  }
  return val;
};
const handleSearch = async () => {
  if (!query.value.trim()) {
    ElMessage.warning('请输入选股指令');
    return;
  }

  loading.value = true;
  hasSearched.value = true;
  abortController = new AbortController();

  try {
    const params = {
      query: query.value.trim(),
    };
    if (pro.value) {
      params.pro = true;
      if (cookieValue.value) {
        params.cookie = cookieValue.value;
      }
    }

    const res = await queryWencai(params, abortController.signal);
    if (res.code === 0) {
      const data = res.data || [];
      const prompt = query.value.trim();

      // 保存记录到本地
      addHistory(prompt);

      // 保存记录到后台
      addRecord(prompt);

      if (Array.isArray(data) && data.length > 0) {
        // Generate columns based on keys of the first item
        const firstItem = data[0];
        const newColumns = [];

        // Ensure "股票代码" and "股票简称" are at the beginning if they exist
        const priorityKeys = ['股票代码', '股票简称', '最新价', '最新涨跌幅'];

        priorityKeys.forEach(key => {
          if (firstItem[key] !== undefined) {
            newColumns.push({
              prop: key,
              label: key,
              minWidth: 150,
            });
          }
        });

        // Add remaining keys
        Object.keys(firstItem).forEach(key => {
          if (!priorityKeys.includes(key)) {
            // filter out some internal or raw json fields if they look ugly
            if (!key.includes('明细数据') && key !== 'market_code' && key !== 'code') {
              newColumns.push({
                prop: key,
                label: key,
                minWidth: 150,
              });
            }
          }
        });

        columns.value = newColumns;
        tableData.value = data;
        currentPage.value = 1;
        ElMessage.success(`查询成功，共 ${data.length} 条记录`);
      } else {
        columns.value = [];
        tableData.value = [];
        currentPage.value = 1;
        ElMessage.warning('没有找到符合条件的股票');
      }
    } else {
      ElMessage.error(res.msg || '查询失败');
    }
  } catch (error) {
    // 检查是否为用户主动取消
    const ERRORS = ['AbortError', 'CanceledError', 'ERR_CANCELED'];
    if (ERRORS.includes(error?.name) || ERRORS?.includes(error.code)) {
      return;
    }
    console.error('AI query error:', error);
    ElMessage.error('请求失败，请稍后重试');
  } finally {
    loading.value = false;
    abortController = null;
  }
};

// 保存记录到后台
const addRecord = async prompt => {
  try {
    await createScreenerRecord({
      prompt: prompt,
    });
  } catch (error) {
    // 静默失败，不影响用户体验
    console.error('保存记录失败:', error);
  }
};

const handleStop = () => {
  if (abortController) {
    abortController.abort();
    abortController = null;
  }
  loading.value = false;
  ElMessage.info('已停止查询');
};

const isSortableColumn = prop => {
  const sortableColumns = ['股票代码', '最新价', '最新涨跌幅'];
  return sortableColumns.includes(prop);
};

const sortMethod = (a, b, prop) => {
  const valA = a[prop];
  const valB = b[prop];

  // 处理空值
  if (valA === null || valA === undefined || valA === '') return 1;
  if (valB === null || valB === undefined || valB === '') return -1;

  // 转换为数字进行比较
  const numA = parseFloat(valA);
  const numB = parseFloat(valB);

  // 如果都是有效数字，按数字排序
  if (!isNaN(numA) && !isNaN(numB)) {
    return numA - numB;
  }

  // 否则按字符串排序
  return String(valA).localeCompare(String(valB));
};

const handleReset = () => {
  query.value = '';
  tableData.value = [];
  columns.value = [];
  hasSearched.value = false;
  currentPage.value = 1;
};
</script>

<style scoped>
.screener-page {
  padding: 15px 0;
}

.search-card {
  margin-top: 15px;
}

.table-card {
  margin-top: 15px;
}

.header-card {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 8px;
  margin-bottom: 15px;

  .page-header {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 8px 16px;

    .el-icon {
      color: #fff;
    }

    .header-content {
      h2 {
        font-size: 24px;
        font-weight: 500;
        color: white;
      }

      p {
        margin: 0;
        font-size: 14px;
        color: rgba(255, 255, 255, 0.9);
      }
    }
  }
}

.pagination-wrapper {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  margin-top: 16px;

  .el-pagination {
    margin-top: 0;
  }
}

.empty-tip {
  padding: 40px 0;
}

.condition-area {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.condition-category {
  display: flex;
  align-items: flex-start;
  font-size: 14px;
}

.category-name {
  width: 90px;
  color: #333;
  font-weight: bold;
}

.category-items {
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.condition-tag {
  display: inline-block;
  padding: 4px 12px;
  background-color: #f4f4f5;
  color: #606266;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.3s;
}

.condition-tag:hover {
  background-color: #ecf5ff;
  color: #409eff;
}

.dropdown-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.textarea-wrapper {
  position: relative;
  width: 100%;
}

.bottom-actions {
  position: absolute;
  left: 10px;
  bottom: 10px;
  z-index: 10;
  display: flex;
  align-items: center;
}

.pro-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-left: 12px;
}

.query-textarea :deep(.el-textarea__inner) {
  line-height: 1.6;
  min-height: 40px;
  font-size: 14px;
  font-weight: bold;
  padding: 12px 14px 40px 14px;
  border-radius: 6px;
  /* border: 1px #ccc solid; */
  resize: vertical;
}
.query-textarea :deep(.el-textarea__inner::placeholder) {
  font-weight: normal;
}

.stock-name {
  color: #409eff;
}

.rate-rise {
  color: #f56c6c;
}

.rate-fall {
  color: #67c23a;
}

.form-center {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.form-item-fixed {
  width: 100%;
  margin: 0 auto;
}

.form-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-top: 15px;
}

.history-container {
  display: flex;
  flex-direction: column;
  max-height: 400px;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 8px;
  margin-bottom: 12px;
  border-bottom: 1px solid #ebeef5;
  flex-shrink: 0;
}

.history-title {
  font-size: 16px;
  font-weight: bold;
}

.history-empty {
  text-align: center;
  color: #909399;
  font-size: 14px;
  padding: 20px 0;
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}

.history-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 2px 12px;
  border-radius: 4px;
  transition: all 0.3s;
}

.history-text {
  flex: 1;
  cursor: pointer;
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-text:hover {
  color: #409eff;
}

.history-delete {
  cursor: pointer;
  color: #909399;
  font-size: 14px;
  margin-left: 8px;
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.3s;
}

.history-item:hover .history-delete {
  opacity: 1;
}

.history-delete:hover {
  color: #f56c6c;
}

.el-form-item {
  margin-bottom: 0;
}
</style>
