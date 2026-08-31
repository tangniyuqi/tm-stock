<template>
  <div class="single-analysis-page">
    <!-- 页面标题卡片 -->
    <el-card shadow="never" class="header-card">
      <div class="page-header">
        <el-icon :size="32" color="#409EFF">
          <Document />
        </el-icon>
        <div class="header-content">
          <h2>个股分析</h2>
          <p>AI驱动的智能股票分析，多维度评估股票价值与风险</p>
        </div>
      </div>
    </el-card>

    <!-- 主内容区域 -->
    <el-row :gutter="20" class="main-content">
      <!-- 左侧：分析配置 -->
      <el-col :span="18">
        <el-card shadow="never" class="config-card">
          <template #header>
            <div class="card-header">
              <span>
                <el-icon>
                  <Setting />
                </el-icon>
                <span class="ml-2">分析配置</span>
              </span>
            </div>
          </template>

          <!-- 股票信息 -->
          <el-divider content-position="left" class="section-divider">
            <span class="section-emoji">📊</span>
            <span class="ml-2">股票信息</span>
          </el-divider>

          <el-form :model="form" label-width="80px" size="large">
            <el-row :gutter="20">
              <el-col :span="12">
                <el-form-item label="股票代码" required>
                  <el-select v-model="form.stockCode" placeholder="输入股票名称/代码/拼音首字母" :loading="stockSearchLoading"
                    remote-show-suffix clearable filterable remote :remote-method="remoteStockSearch"
                    @change="handleSymbolChange" style="width: 100%">
                    <el-option v-for="stock in availableStocks" :key="stock.ts_code"
                      :label="`${stock.name} (${stock.ts_code}) (${stock.market})`" :value="stock.ts_code">
                      <span>{{ stock.name }}</span>
                      <span class="stock-option-meta">({{ stock.ts_code }}) ({{ stock.market }})</span>
                    </el-option>
                  </el-select>
                </el-form-item>
              </el-col>

              <el-col :span="12">
                <el-form-item label="市场类型" required>
                  <el-select v-model="form.market" placeholder="请选择" style="width: 100%">
                    <el-option label="🇨🇳 A股市场" value="A股" />
                    <el-option label="🇭🇰 港股市场（暂不支持）" value="港股" disabled />
                    <el-option label="🇺🇸 美股市场（暂不支持）" value="美股" disabled />
                  </el-select>
                </el-form-item>
              </el-col>

              <el-col :span="12">
                <el-form-item label="分析日期" required>
                  <el-date-picker v-model="form.analysisDate" type="date" placeholder="选择日期" style="width: 100%"
                    value-format="YYYY-MM-DD" :disabled-date="disabledDate" :shortcuts="dateShortcuts" />
                </el-form-item>
              </el-col>
            </el-row>
          </el-form>

          <!-- 分析深度 -->
          <el-divider content-position="left" class="section-divider">
            <span class="section-emoji">🎯</span>
            <span class="ml-2">分析深度</span>
          </el-divider>

          <el-radio-group v-model="form.analysisDepth" class="depth-radio-group">
            <el-radio v-for="item in analysisDepthOptions" :key="item.value" :value="item.value" border
              class="depth-radio">
              <div class="depth-content">
                <el-icon :size="20" :color="item.color">
                  <component :is="item.icon" />
                </el-icon>
                <div class="depth-info">
                  <div class="depth-label">{{ item.label }}</div>
                  <div class="depth-desc-row">
                    <span class="depth-desc">{{ item.desc }}</span>
                    <el-tag size="small" type="info" effect="plain">{{ item.time }}</el-tag>
                  </div>
                </div>
                <el-icon v-if="form.analysisDepth === item.value" class="check-icon" :size="20">
                  <Select />
                </el-icon>
              </div>
            </el-radio>
          </el-radio-group>

          <!-- 分析师团队 -->
          <el-divider content-position="left" class="section-divider">
            <span class="section-emoji">👥</span>
            <span class="ml-2">分析师团队（可多选）</span>
          </el-divider>

          <el-checkbox-group v-model="form.selectedAnalysts" class="analyst-checkbox-group">
            <el-checkbox v-for="analyst in analystOptions" :key="analyst.value" :value="analyst.value" border
              class="analyst-checkbox">
              <div class="analyst-content">
                <el-icon :size="24" :color="analyst.color">
                  <component :is="analyst.icon" />
                </el-icon>
                <div class="analyst-info">
                  <div class="analyst-label">{{ analyst.label }}</div>
                  <div class="analyst-desc">{{ analyst.desc }}</div>
                </div>
                <el-icon v-if="form.selectedAnalysts.includes(analyst.value)" class="check-icon" :size="20">
                  <Select />
                </el-icon>
              </div>
            </el-checkbox>
          </el-checkbox-group>

          <!-- 开始分析按钮 -->
          <el-divider />
          <div class="action-section">
            <el-button type="primary" size="large" :icon="Promotion" @click="startAnalysis" class="start-btn"> 开始智能分析
            </el-button>
          </div>
        </el-card>
      </el-col>

      <!-- 右侧：高级配置 -->
      <el-col :span="6">
        <el-card shadow="never" class="advanced-card">
          <template #header>
            <div class="card-header">
              <span>
                <el-icon>
                  <Tools />
                </el-icon>
                <span class="ml-2">高级配置</span>
              </span>
            </div>
          </template>

          <!-- AI模型配置 -->
          <el-divider>
            <span>AI模型配置</span>
          </el-divider>

          <div class="model-config-form">
            <div class="model-item">
              <div class="model-label">
                <span>快速分析厂商</span>
                <el-tooltip content="用于市场分析、新闻分析、基本面分析、社媒分析、大盘分析、板块分析等" placement="top">
                  <el-icon class="help-icon">
                    <QuestionFilled />
                  </el-icon>
                </el-tooltip>
              </div>
              <el-select v-model="advancedConfig.quickProvider" placeholder="请选择厂商" @change="onQuickProviderChange">
                <el-option v-for="item in providerOptions" :key="item.value" :label="item.label" :value="item.value"
                  :disabled="item.disabled" />
              </el-select>
            </div>

            <div class="model-item">
              <div class="model-label">
                <span>快速分析模型</span>
                <el-tooltip content="用于市场分析、新闻分析、基本面分析、社媒分析、大盘分析、板块分析等" placement="top">
                  <el-icon class="help-icon">
                    <QuestionFilled />
                  </el-icon>
                </el-tooltip>
              </div>
              <el-select v-model="advancedConfig.quickModel" placeholder="请选择模型" :disabled="!currentQuickModels.length">
                <el-option v-for="m in currentQuickModels" :key="m.value" :label="m.label" :value="m.value" />
              </el-select>
            </div>

            <div class="model-item">
              <div class="model-label">
                <span>深度决策厂商</span>
                <el-tooltip content="用于研究管理者综合决策、风险管理者最终评估" placement="top">
                  <el-icon class="help-icon">
                    <QuestionFilled />
                  </el-icon>
                </el-tooltip>
              </div>
              <el-select v-model="advancedConfig.deepProvider" placeholder="请选择厂商" @change="onDeepProviderChange">
                <el-option v-for="item in providerOptions" :key="item.value" :label="item.label" :value="item.value"
                  :disabled="item.disabled" />
              </el-select>
            </div>

            <div class="model-item">
              <div class="model-label">
                <span>深度决策模型</span>
                <el-tooltip content="用于研究管理者综合决策、风险管理者最终评估" placement="top">
                  <el-icon class="help-icon">
                    <QuestionFilled />
                  </el-icon>
                </el-tooltip>
              </div>
              <el-select v-model="advancedConfig.deepModel" placeholder="请选择模型" :disabled="!currentDeepModels.length">
                <el-option v-for="m in currentDeepModels" :key="m.value" :label="m.label" :value="m.value" />
              </el-select>
            </div>

            <div class="model-item">
              <div class="model-label">
                <span>接口模式</span>
                <el-tooltip content="responses=响应接口；chat-completions=对话接口" placement="top">
                  <el-icon class="help-icon">
                    <QuestionFilled />
                  </el-icon>
                </el-tooltip>
              </div>
              <el-select v-model="advancedConfig.api_format" placeholder="请选择接口模式">
                <el-option label="响应接口" value="responses" />
                <el-option label="对话接口" value="chat-completions" />
              </el-select>
            </div>

            <div v-if="advancedConfig.api_format === 'chat-completions'" class="model-item">
              <div class="model-label">
                <span>搜索引擎</span>
                <el-tooltip content="对话接口模式下联网检索可选的搜索引擎" placement="top">
                  <el-icon class="help-icon">
                    <QuestionFilled />
                  </el-icon>
                </el-tooltip>
              </div>
              <el-select v-model="advancedConfig.search_engine" placeholder="请选择搜索引擎">
                <el-option v-for="item in webSearchOptions" :key="item.value" :label="item.label" :value="item.value"
                  :disabled="item.disabled" />
              </el-select>
            </div>

            <div class="model-item">
              <div class="model-label">
                <span>联网搜索</span>
                <el-tooltip content="AI 分析时强制联网检索最新资讯" placement="top">
                  <el-icon class="help-icon">
                    <QuestionFilled />
                  </el-icon>
                </el-tooltip>
              </div>
              <el-switch v-model="advancedConfig.web_search" />
            </div>
          </div>

          <!-- 模型推荐 -->
          <el-divider>模型推荐</el-divider>

          <el-alert type="warning" :closable="false" show-icon>
            <template #title>
              <div class="text-sm">标准分析：快速模型用基础级，深度模型用标准级以上</div>
            </template>
          </el-alert>

          <!-- 分析选项 -->
          <el-divider>分析选项</el-divider>

          <div class="option-list">
            <div class="option-item">
              <div class="option-info">
                <div class="option-label">情绪分析</div>
                <div class="option-desc">分析市场情绪和投资者心理</div>
              </div>
              <el-switch v-model="advancedConfig.sentimentAnalysis" />
            </div>

            <el-divider style="margin: 12px 0" />

            <div class="option-item">
              <div class="option-info">
                <div class="option-label">风险评估</div>
                <div class="option-desc">量化评估潜在风险点</div>
              </div>
              <el-switch v-model="advancedConfig.riskAssessment" />
            </div>

            <el-divider style="margin: 12px 0" />

            <div class="option-item">
              <div class="option-info">
                <div class="option-label">语言偏好</div>
              </div>
              <el-select v-model="advancedConfig.language" size="small" style="width: 100px">
                <el-option label="中文" value="zh" />
                <el-option label="English" value="en" />
              </el-select>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { computed, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { getBaseStockPublic } from '@/api/quant/baseStock';
import { providerOptions } from '@/data/aiProviderOptions';
import { webSearchOptions } from '@/data/webSearchOptions';

defineOptions({
  name: 'SingleAnalysis',
});

// 表单数据
const form = reactive({
  stockCode: '',
  market: 'A股',
  analysisDate: '2026-04-07',
  analysisDepth: 3,
  selectedAnalysts: ['market', 'fundamental'],
});

// 股票搜索相关
const availableStocks = ref([]);
const stockSearchLoading = ref(false);

// 高级配置
const advancedConfig = reactive({
  // 快速分析模型（厂商 + 模型联动，参考 themeStock 大模型选择）
  quickProvider: 'qwen',
  quickModel: 'qwen3.7-flash',
  // 深度决策模型（厂商 + 模型联动）
  deepProvider: 'qwen',
  deepModel: 'qwen3.7-max',
  // 接口与联网（参考 themeStock 大模型选择）
  api_format: 'responses', // responses=响应接口；chat-completions=对话接口
  web_search: true, // true=强制开启联网搜索；false=强制关闭
  search_engine: 'baidu', // 联网搜索引擎（chat-completions 模式且开启联网搜索时生效）
  // 分析选项
  sentimentAnalysis: true,
  riskAssessment: true,
  language: 'zh',
});

// 当前快速分析厂商的模型列表
const currentQuickModels = computed(() => {
  const provider = providerOptions.find(item => item.value === advancedConfig.quickProvider);
  return provider?.models || [];
});

// 切换快速分析厂商时自动带上该厂商第一个模型
const onQuickProviderChange = () => {
  advancedConfig.quickModel = currentQuickModels.value[0]?.value || '';
};

// 当前深度决策厂商的模型列表
const currentDeepModels = computed(() => {
  const provider = providerOptions.find(item => item.value === advancedConfig.deepProvider);
  return provider?.models || [];
});

// 切换深度决策厂商时自动带上该厂商第一个模型
const onDeepProviderChange = () => {
  advancedConfig.deepModel = currentDeepModels.value[0]?.value || '';
};

// 分析深度选项
const analysisDepthOptions = [
  { value: 1, label: '1级 - 快速分析', desc: '基础数据概览，快速洞察', time: '2~3分钟', icon: 'Edit', color: '#409EFF' },
  { value: 2, label: '2级 - 基础分析', desc: '常规数据深度', time: '3~5分钟', icon: 'Warning', color: '#E6A23C' },
  { value: 3, label: '3级 - 标准分析', desc: '技术+基本面，推荐', time: '5-8分钟', icon: 'Aim', color: '#F56C6C' },
  { value: 4, label: '4级 - 深度分析', desc: '全场景深，深度研究', time: '10~12分钟', icon: 'Search', color: '#909399' },
  { value: 5, label: '5级 - 全面分析', desc: '最全面分析报告', time: '15-20分钟', icon: 'Trophy', color: '#F56C6C' },
];

// 分析师选项
const analystOptions = [
  { value: 'market', label: '市场分析师', desc: '分析市场趋势，行业动态和宏观因素', icon: 'TrendCharts', color: '#409EFF' },
  { value: 'fundamental', label: '基本面分析师', desc: '深入分析财务数据，盈亏表和现金流', icon: 'DataAnalysis', color: '#67C23A' },
  { value: 'news', label: '新闻分析师', desc: '分析新闻资讯，公告和市场舆情影响', icon: 'Document', color: '#E6A23C' },
  { value: 'social', label: '社媒分析师', desc: '分析社交媒体情绪，投资者观点和舆论', icon: 'User', color: '#909399' },
  { value: 'macro', label: '大盘分析师', desc: '专注于大盘指数走势，成交量和市场情绪', icon: 'DataAnalysis', color: '#F56C6C' },
  { value: 'sector', label: '板块分析师', desc: '分析板块轮动效应，热点板块资金流向', icon: 'TrendCharts', color: '#909399' },
];

// 禁用今天之后的日期
const disabledDate = time => {
  return time.getTime() > Date.now();
};

// 日期快捷选项
const dateShortcuts = [
  {
    text: '今天',
    value: () => {
      return new Date();
    },
  },
  {
    text: '昨天',
    value: () => {
      const date = new Date();
      date.setDate(date.getDate() - 1);
      return date;
    },
  },
  {
    text: '前天',
    value: () => {
      const date = new Date();
      date.setDate(date.getDate() - 2);
      return date;
    },
  },
];

// 开始分析
const startAnalysis = () => {
  if (!form.stockCode) {
    ElMessage.warning('请输入股票代码');
    return;
  }

  console.log('开始分析', {
    ...form,
    ...advancedConfig,
  });

  ElMessage.success('分析任务已提交');
};

// 获取可用股票列表
const remoteStockSearch = async query => {
  stockSearchLoading.value = true;

  try {
    const res = await getBaseStockPublic({ q: query, pageSize: 100 });

    if (res.code === 0 && res.data) {
      availableStocks.value = res.data.list;
    }
  } catch (error) {
    console.error('股票搜索失败:', error);
    ElMessage.error('股票搜索失败，请重试');
  } finally {
    stockSearchLoading.value = false;
  }
};

// 股票选择变化处理
const handleSymbolChange = value => {
  const selectedStock = availableStocks.value.find(stock => stock.ts_code === value);
  if (selectedStock) {
    console.log('选中股票:', selectedStock);
  }
};
</script>

<style scoped lang="scss" src="./single.scss"></style>
