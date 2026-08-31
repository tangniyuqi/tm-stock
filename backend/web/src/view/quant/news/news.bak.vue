<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" inline :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="全文" prop="keyword">
          <el-input v-model="searchInfo.keyword" placeholder="关键词" />
        </el-form-item>

        <el-form-item label="等级" prop="level">
          <el-select v-model="searchInfo.level" placeholder="请选择等级">
            <el-option v-for="(item, key) in levelOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item label="加粗" prop="bold">
          <el-select v-model="searchInfo.bold" placeholder="请选择加粗">
            <el-option v-for="(item, key) in boldOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="来源" prop="source">
            <el-select v-model="searchInfo.source" placeholder="请选择来源">
              <el-option v-for="(item, key) in sourceOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item>

          <el-form-item label="标题" prop="title">
            <el-input v-model="searchInfo.title" placeholder="标题" />
          </el-form-item>

          <el-form-item label="内容" prop="content">
            <el-input v-model="searchInfo.content" placeholder="内容" />
          </el-form-item>

          <!-- <el-form-item label="性质" prop="nature">
            <el-select v-model="searchInfo.nature" placeholder="请选择性质">
              <el-option v-for="(item, key) in natureOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item> -->

          <el-form-item label="日期" prop="created_at_range">
            <el-date-picker v-model="searchInfo.created_at_range" type="daterange" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间" style="width: 240px" />
          </el-form-item>

          <!-- <el-form-item label="状态" prop="status">
            <el-select v-model="searchInfo.status" placeholder="请选择状态">
              <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
            </el-select>
          </el-form-item> -->
        </template>

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
          <el-button link type="primary" icon="arrow-down" @click="showAllQuery = true" v-if="!showAllQuery">展开</el-button>
          <el-button link type="primary" icon="arrow-up" @click="showAllQuery = false" v-else>收起</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="gva-search-box">
      <el-button v-for="(item, index) in keywordData.slice(0, 48)" :key="index" :type="item.type" plain class="mb-3" @click="checkKeyword(item.name)">
        {{ item.name }}
      </el-button>

      <template v-if="showAllKeyword">
        <el-button v-for="(item, index) in keywordData.slice(48)" :key="index" :type="item.type" plain class="mb-3" @click="checkKeyword(item.name)">
          {{ item.name }}
        </el-button>
      </template>

      <el-button link icon="arrow-down" class="mb-3" @click="showAllKeyword = true" v-if="!showAllKeyword">展开</el-button>
      <el-button link icon="arrow-up" class="mb-3" @click="showAllKeyword = false" v-else>收起</el-button>
    </div>

    <div class="gva-table-box">
      <!-- <div class="gva-btn-list">
        <el-button icon="delete" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
      </div> -->

      <!-- <el-tabs type="card" class="mb-2" @tab-click="handleClick">
        <el-tab-pane label="新闻精选" />
        <el-tab-pane label="机器人" />
        <el-tab-pane label="算力" />
        <el-tab-pane label="固态电池" />
        <el-tab-pane label="光模块" />
        <el-tab-pane label="CPO" />
        <el-tab-pane label="PCB" />
      </el-tabs> -->

      <div class="gva-table-box-head">
        <div class="gva-btn-list">
          <el-button type="primary" link @click="onReset">每5s更新一次（飞书群为实时推送~）</el-button>
          <!-- <el-button type="primary" icon="plus" disabled @click="openDialog()">新增</el-button>
          <el-button icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button> -->
        </div>

        <div class="gva-pagination">
          <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
        </div>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id" @selection-change="handleSelectionChange" @sort-change="sortChange">
        <!-- <el-table-column align="center" type="selection" width="60" /> -->

        <el-table-column align="center" label="ID" prop="id" width="120" />

        <el-table-column label="快讯内容" prop="source" min-width="640">
          <template #default="scope">
            <el-text tag="div" class="text-title mt-3" v-html="`${formatToDateTime(scope.row.ctime, 'HH:mm:ss')} - ${highlightKeywords(scope.row.title || '无', searchInfo.keyword)}`" />

            <el-text tag="div" class="text-content mt-3" v-html="highlightKeywords(replaceKeywords(scope.row.content), searchInfo.keyword)" />

            <el-link
              icon="ChatRound"
              :href="`https://www.doubao.com/chat/url-action?action=${encodeURIComponent(
                JSON.stringify({
                  pluginId: 'Send_Message',
                  payload: {
                    up_template_key: 'address_bar_up',
                    text: scope.row.content,
                    reportParams: { is_address: 1, scene: 'address_bar' },
                  },
                })
              )}`"
              target="_blank"
              title="打开豆包对话"
            />

            <el-link icon="CopyDocument" class="ml-2" underline="never" title="复制内容" @click="copyNews(scope.row)" />
          </template>
        </el-table-column>

        <!-- <el-table-column label="标签" prop="tags" width="90">
            <template #default="scope">
                [JSON]
            </template>
        </el-table-column> -->

        <!-- <el-table-column align="left" label="作者" prop="author" width="120" /> -->

        <el-table-column align="center" label="来源" prop="source" width="120" v-auth="btnAuth.source">
          <template #default="scope">
            <el-tag type="info" effect="plain">
              {{ filterDict(String(scope.row.source), sourceOptions) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column align="center" label="性质" prop="nature" width="60" />
        <el-table-column align="center" label="等级" prop="level" width="60" />
        <el-table-column align="center" label="加粗" prop="bold" width="60" />
        <!-- <el-table-column align="center" label="浏览量" prop="view" width="60" />
        <el-table-column sortable align="center" label="分享数" prop="share" width="60" /> -->

        <el-table-column align="left" label="入库时间" prop="created_at" width="120">
          <template #default="scope">{{ formatToDateTime(scope.row.created_at, 'YYYY-MM-DD HH:mm:ss') }}</template>
        </el-table-column>

        <!-- <el-table-column sortable align="left" label="状态" prop="status" width="90">
          <template #default="scope">
            <el-tag type="info" effect="plain"> 
              {{ filterDict(String(scope.row.status), statusOptions) }}
            </el-tag>
          </template>
        </el-table-column> -->
      </el-table>

      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onUnmounted } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import { createNews, deleteNews, deleteNewsByIds, updateNews, findNews, getNewsList } from '@/api/quant/news';
import { getNewsKeywordList, incrementTimes } from '@/api/quant/newsKeyword';

defineOptions({
  name: 'News',
});

// 按钮权限实例化
const btnAuth = useBtnAuth();
// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const showAllKeyword = ref(false);
const natureOptions = ref();
const levelOptions = ref();
const boldOptions = ref();
const sourceOptions = ref();
const statusOptions = ref();

const elFormRef = ref();
const elSearchFormRef = ref();

// =========== 表格控制部分 ===========
const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
const tableData = ref([]);
const keywordData = ref([]);
const searchInfo = ref({});
// 排序
const sortChange = ({ prop, order }) => {
  const sortMap = {
    nature: 'nature',
    level: 'level',
    bold: 'bold',
    view: 'view',
    share: 'share',
    ctime: 'ctime',
    status: 'status',
  };

  let sort = sortMap[prop];
  if (!sort) {
    sort = prop.replace(/[A-Z]/g, match => `_${match.toLowerCase()}`);
  }

  searchInfo.value.sort = sort;
  searchInfo.value.order = order;
  getTableData();
};

// 重置
const onReset = () => {
  searchInfo.value = {};
  getTableData();
  getKeywordData();
};

// 搜索
const onSubmit = () => {
  elSearchFormRef.value?.validate(async valid => {
    if (!valid) return;
    page.value = 1;
    getTableData();
    incrementKeywordTimes();
  });
};

// 选择关键词
const checkKeyword = keyword => {
  searchInfo.value.keyword = keyword;
  onSubmit();
};

// 分页
const handleSizeChange = val => {
  pageSize.value = val;
  getTableData();
};

// 修改页面容量
const handleCurrentChange = val => {
  page.value = val;
  getTableData();
};

// 查询
const getTableData = async () => {
  const table = await getNewsList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });

  if (table.code === 0) {
    tableData.value = table.data.list;
    total.value = table.data.total;
    page.value = table.data.page;
    pageSize.value = table.data.pageSize;
  }
};

getTableData();

// 定时刷新数据
const timer = setInterval(() => {
  getTableData();
}, 5000);

onUnmounted(() => {
  clearInterval(timer);
});

// 更新关键词查询次数
const incrementKeywordTimes = async () => {
  await incrementTimes({ name: searchInfo.value.keyword });
};

// 获取关键词
const getKeywordData = async () => {
  const res = await getNewsKeywordList({ sort_type: 'times_desc' });

  if (res.code === 0) {
    const buttonTypes = ['primary', 'success', 'info', 'warning', 'danger', ''];

    keywordData.value = res.data.list.map((item, index) => {
      const randomType = buttonTypes[index % buttonTypes.length];
      return {
        ...item,
        type: randomType,
        text: item.title || '新闻精选',
      };
    });
  }
};

getKeywordData();

// 获取需要的字典 可能为空 按需保留
const setOptions = async () => {
  natureOptions.value = await getDictFunc('quant_news_nature');
  levelOptions.value = await getDictFunc('quant_news_level');
  boldOptions.value = await getDictFunc('quant_news_bold');
  sourceOptions.value = await getDictFunc('quant_news_source');
  statusOptions.value = await getDictFunc('status');
};

// 获取需要的字典 可能为空 按需保留
setOptions();

// 多选数据
const multipleSelection = ref([]);
// 多选
const handleSelectionChange = val => {
  multipleSelection.value = val;
};

// 删除行
const deleteRow = row => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(() => {
    deleteNewsFunc(row);
  });
};

// 多选删除
const onDelete = async () => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(async () => {
    const ids = [];
    if (multipleSelection.value.length === 0) {
      ElMessage({
        type: 'warning',
        message: '请选择要删除的数据',
      });
      return;
    }
    multipleSelection.value &&
      multipleSelection.value.map(item => {
        ids.push(item.id);
      });
    const res = await deleteNewsByIds({ ids });
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '删除成功',
      });
      if (tableData.value.length === ids.length && page.value > 1) {
        page.value--;
      }
      getTableData();
    }
  });
};

// 删除行
const deleteNewsFunc = async row => {
  const res = await deleteNews({ id: row.id });
  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '删除成功',
    });
    if (tableData.value.length === 1 && page.value > 1) {
      page.value--;
    }
    getTableData();
  }
};

// 处理消息中的换行符
const replaceKeywords = str => {
  return str.replace(/\n/g, '<div class="mt-4"></div>');
};

// 处理消息中的换行符（	计算属性）
/* const formattedItems = computed(() => {
return items.value.map(item => ({
...item,
formattedMsg: formatMessage(item.msg)
}))
}); */

// 关键词高亮方法
const highlightKeywords = (text, keyword) => {
  if (!text || !keyword) return text;
  const keywords = keyword.trim().split(' ');
  let highlighted = text;
  keywords.forEach(k => {
    if (k) {
      const regex = new RegExp(k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
      highlighted = highlighted.replace(regex, match => `<span style="color: #F00;">${match}</span>`);
    }
  });
  return highlighted;
};

// 复制内容
const copyNews = row => {
  if (!row) return;
  const time = formatToDateTime(row.ctime, 'HH:mm:ss');
  const text = `${time} - ${row.content}`;

  navigator.clipboard
    .writeText(text)
    .then(() => {
      ElMessage.success('复制成功');
    })
    .catch(() => {
      ElMessage.error('复制失败');
    });
};

const getStatusType = status => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};
</script>

<style scoped>
.gva-search-box {
  max-height: 400px;
  overflow: auto;

  .el-button {
    margin-left: 0;
    margin-right: 12px;
  }
}

.gva-table-box-head {
  display: flex;
  justify-content: space-between;

  .el-pagination {
    margin-top: 1em;
    margin-bottom: 2em;
  }
}

.text-title {
  line-height: 28px;
  font-size: 16px;
}

.text-content {
  line-height: 24px;
  background: #98ea70;
  color: #333;
  font-size: 15px;
  border-radius: 5px;
  padding: 10px 12px;
  text-align: justify;
}

.light {
  .text-title {
    color: #000;
  }

  .text-content {
    color: #333;
  }
}
</style>
