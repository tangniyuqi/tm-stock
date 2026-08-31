<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline" @keyup.enter="onSubmit">
        <el-form-item label="内容" prop="content">
          <el-input v-model="searchInfo.content" clearable placeholder="请输入内容关键词" style="width: 100%" />
        </el-form-item>

        <el-form-item label="优级" prop="priority">
          <el-select v-model="searchInfo.priority" clearable placeholder="请选择优先级">
            <el-option v-for="(item, key) in priorityOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item label="商机" prop="oppty">
          <el-select v-model="searchInfo.oppty" clearable placeholder="请选择商机类型">
            <el-option v-for="(item, key) in opptyOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <!-- <el-form-item label="意图" prop="intent">
          <el-input v-model="searchInfo.intent" clearable placeholder="请输入意图关键词" />
        </el-form-item> -->

        <el-form-item label="星级" prop="level">
          <el-select v-model="searchInfo.level" clearable placeholder="请选择重要性">
            <el-option v-for="(item, key) in levelOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item label="状态" prop="status" v-if="!status">
          <el-select v-model="searchInfo.status" clearable placeholder="请选择状态">
            <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <el-form-item label="昵称" prop="sender" v-if="!!status">
          <el-input v-model="searchInfo.sender" clearable placeholder="请输入昵称" />
        </el-form-item>

        <el-form-item label="备注" prop="remark">
          <el-input v-model="searchInfo.remark" clearable placeholder="请输入备注" />
        </el-form-item>

        <el-form-item label="群名" prop="group">
          <el-input v-model="searchInfo.group" clearable placeholder="请输入群名关键词" />
        </el-form-item>

        <template v-if="showAllQuery">
          <el-form-item label="编号" prop="id">
            <el-input v-model="searchInfo.id" clearable placeholder="请输入ID编号" />
          </el-form-item>

          <el-form-item label="终端" prop="client_id">
            <el-input v-model="searchInfo.client_id" clearable placeholder="请输入客户端ID" />
          </el-form-item>

          <el-form-item label="昵称" prop="sender" v-if="!status">
            <el-input v-model="searchInfo.sender" clearable placeholder="请输入昵称" />
          </el-form-item>

          <el-form-item label="别名" prop="sender_remark">
            <el-input v-model="searchInfo.sender_remark" clearable placeholder="请输入备注名" />
          </el-form-item>
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
      <el-button v-for="(item, index) in keywordData.slice(0, 36)" :key="index" :type="item.type" plain class="mb-3" @click="checkKeyword(item.name)">
        {{ item.name }}
      </el-button>

      <template v-if="showAllKeyword">
        <el-button v-for="(item, index) in keywordData.slice(36)" :key="index" :type="item.type" plain class="mb-3" @click="checkKeyword(item.name)">
          {{ item.name }}
        </el-button>
      </template>

      <el-button link icon="arrow-down" class="mb-3" @click="showAllKeyword = true" v-if="!showAllKeyword">展开</el-button>
      <el-button link icon="arrow-up" class="mb-3" @click="showAllKeyword = false" v-else>收起</el-button>
    </div>

    <div class="gva-table-box">
      <div class="gva-table-box-head">
        <div class="gva-btn-list">
          <el-button v-auth="btnAuth.add" type="primary" icon="plus" disabled @click="openDialog()">新增</el-button>
          <el-button v-auth="btnAuth.batchDelete" icon="delete" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
          <el-button v-auth="btnAuth.softDeleteDuplicate" @click="onDeleteDuplicate">软清理</el-button>
          <el-button v-auth="btnAuth.hardDeleteDuplicate" @click="onDeleteDuplicate({ force: 'true'})">硬清理</el-button>
        </div>

        <div class="gva-pagination">
          <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
        </div>
      </div>

      <el-table ref="multipleTable" tooltip-effect="dark" :data="tableData" row-key="id" style="width: 100%" @selection-change="handleSelectionChange">
        <el-table-column v-auth="btnAuth.batchDelete" align="center" type="selection" width="30" />

        <!-- <el-table-column align="center" label="ID" prop="id" width="120" /> -->

        <el-table-column align="left" label="基本信息" prop="sender" min-width="180" max-width="240">
          <template #default="scope">
            <div class="cell-style">
              <el-text tag="p">MID：{{ scope.row.id }}</el-text>
              <el-text tag="p">CID：{{ scope.row.client_id }}</el-text>
              <el-text tag="p">SID：{{ scope.row.msg_id }}</el-text>
              <el-text tag="p">TIME：{{ formatDate(scope.row.created_at) }}</el-text>
              <el-text tag="p" class="text-title" v-html="highlightKeywords(scope.row.group, searchInfo.group)" />
            </div>
          </template>
        </el-table-column>

        <el-table-column align="left" label="消息内容" prop="content" min-width="360" max-width="600">
          <template #default="scope">
            <el-text tag="p" class="text-title">
              {{ scope.row.sender }}
              {{ scope.row.sender_remark ? `（备注名：${scope.row.sender_remark}）` : '' }}
              {{ formatToDateTime(scope.row.send_time, 'MM-DD HH:mm:ss') }}
            </el-text>

            <el-text tag="div" class="text text-content" v-html="highlightKeywords(replaceKeywords(scope.row.content), searchInfo.content)" />

            <el-text tag="div" class="text text-remark" v-html="highlightKeywords(replaceKeywords(scope.row.remark), searchInfo.remark)" @dblclick="updateMsgFunc(scope.row)" v-if="!!scope.row.remark" />

            <el-text tag="div" class="text-rate">
              <el-text type="info" size="small">
                {{ filterDict(String(scope.row.oppty), opptyOptions) }}
                （{{ filterDict(String(scope.row.priority), priorityOptions) }}）
              </el-text>

              <el-rate v-model="scope.row.level" :max="5" clearable @change="handleRateChange(scope.row)" />
              
              <div>
                <el-button type="warning" size="small" link @click="handleDeal(scope.row)" v-if="scope.row.status == 1">
                  <el-icon color="#E6A23C"><Flag /></el-icon>
                  出库 ({{ filterDict(String(scope.row.status), statusOptions) }})
                </el-button>

                <el-button type="primary" size="small" link @click="handleDeal(scope.row)" v-else>
                  <el-icon color="#409EFF"><Flag /></el-icon>
                  入库 ({{ filterDict(String(scope.row.status), statusOptions) }})
                </el-button>
              </div>
            </el-text>
            <!-- <el-rate :model-value="scope.row.status" :size="scope.row.status == 1 ? 'large' : 'small'" text-color="#F90" :max="1" @click="updateMsgFunc(scope.row)" v-if="scope.row.status != -1" /> -->
          </template>
        </el-table-column>

        <el-table-column align="center" label="IC型号" prop="ics" min-width="210" max-width="420">
          <template #default="scope">
            <div class="text text-ics" v-if="scope.row.ics && scope.row.ics.length > 0">
              <div class="link-ics">
                <el-text class="text-ics-title">共 {{ scope.row.ics.length }} 个型号</el-text>
                <el-text type="info">数量</el-text>
              </div>

              <div v-for="(item, index) in scope.row.ics" :key="item.index">
                <div class="link-ics">
                  <el-link underline="naver" @click="pushTo('msg', { 'content': item.name, 'sender': '!' + scope.row.sender, 'status_str': '!1' })">
                    {{ item.name }}
                  </el-link>

                  <el-text type="info" v-if="item.count > 0">{{ item.count }}条</el-text>
                </div>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" min-width="82px">
          <template #default="scope">
            <el-button v-auth="btnAuth.edit" type="info" link icon="flag" @click="updateMsgFunc(scope.row)">备注/设置</el-button>
            <br />
            <el-button type="info" link icon="user" @click="pushTo('member', scope.row)">创建客户</el-button>
            <br />
            <el-button type="info" link icon="list" @click="pushTo('follow', scope.row)">创建跟进</el-button>
            <br />
            <el-button v-auth="btnAuth.delete" type="info" link icon="delete" @click="deleteRow(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
      </div>
    </div>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="dialogFormVisible" :show-close="false" :before-close="closeDialog">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">{{ type === 'create' ? '新增' : '编辑' }}</span>
          <div>
            <el-button :loading="btnLoading" type="primary" @click="enterDialog">确 定</el-button>
            <el-button @click="closeDialog">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form ref="elFormRef" :model="formData" label-position="right" :rules="rule" label-width="90px" @keyup.enter="handleEnter">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="ID" prop="id">
              <el-input v-model="formData.id" disabled />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="客户端" prop="client_id">
              <el-input v-model="formData.client_id" disabled />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="群名称" prop="id">
              <el-input v-model.trim="formData.group" disabled />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="MsgID" prop="msg_id">
              <el-input v-model.trim="formData.msg_id" disabled />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="发送者" prop="sender">
              <el-input v-model.trim="formData.sender" disabled placeholder="请输入发送者" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="备注名" prop="sender_remark">
              <el-input v-model.trim="formData.sender_remark" clearable placeholder="请输入备注名" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="发送时间" prop="send_time">
              <el-date-picker v-model="formData.send_time" type="datetime" disabled clearable style="width: 100%" placeholder="选择日期" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="类型" prop="type">
              <el-input v-model="formData.type" disabled placeholder="请输入消息类型" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="消息内容" prop="content">
              <el-input v-model.trim="formData.content" type="textarea" disabled :autosize="{ minRows: 6, maxRows: 12 }" placeholder="请输入内容" />
            </el-form-item>
          </el-col>

          <!-- <el-col :span="24">
            <el-form-item label="IC型号" prop="ics">
              <ArrayCtrl v-model="formData.ics" editable />
              <el-input-tag v-model="formData.ics" :max="1000" draggable clearable placeholder="请输入IC型号，按enter键" />
            </el-form-item>
          </el-col> -->

          <el-col :span="12">
            <el-form-item label="优先级" prop="status">
              <el-select v-model="formData.priority" placeholder="请选择优先级">
                <el-option v-for="(item, key) in priorityOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="机会类型" prop="oppty">
              <el-select v-model="formData.oppty" placeholder="请选择机会类型">
                <el-option v-for="(item, key) in opptyOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="备注" prop="remark">
              <el-input v-model.trim="formData.remark" type="textarea" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="请输入备注" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="入库状态" prop="status">
              <el-select v-model="formData.status" placeholder="请选择状态">
                <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive, nextTick } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox, ElPopover, ElButton, ElBacktop } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';
import { createMsg, deleteMsg, deleteMsgByIds, updateMsg, findMsg, getMsgList, deleteDuplicatePublic } from '@/api/bi/msg';
import { getKeywordList, incrementTimes } from '@/api/bi/keyword';
// import ArrayCtrl from '@/components/arrayCtrl/arrayCtrl.vue';

defineOptions({
  name: 'Msg'
});

// 按钮权限实例化
const btnAuth = useBtnAuth();
const route = useRoute();
const router = useRouter();
const status = ref();
status.value = route.query.status;

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const showAllKeyword = ref(false);
const priorityOptions = ref();
const opptyOptions = ref();
const levelOptions = ref();
const statusOptions = ref();

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  client_id: undefined,
  sender_id: '',
  receiver_id: '',
  msg_id: '',
  gid: '',
  group: '',
  type: '',
  sender: '',
  sender_remark: '',
  receiver: '',
  send_time: new Date(),
  content: '',
  ics: [],
  level: 0,
  remark: '',
  status: 0
});

// 验证规则
const rule = reactive({
  /* client_id: [{
required: true,
message: '',
trigger: ['input', 'blur'],
},
],
msg_id: [{
required: true,
message: '',
trigger: ['input', 'blur'],
},
{
whitespace: true,
message: '不能只输入空格',
trigger: ['input', 'blur'],
}
],
type: [{
required: true,
message: '',
trigger: ['input', 'blur'],
},
{
whitespace: true,
message: '不能只输入空格',
trigger: ['input', 'blur'],
}
],
sender: [{
required: true,
message: '',
trigger: ['input', 'blur'],
},
{
whitespace: true,
message: '不能只输入空格',
trigger: ['input', 'blur'],
}
],
receiver: [
{
whitespace: true,
message: '不能只输入空格',
trigger: ['input', 'blur'],
}
],
send_time: [{
required: true,
message: '',
trigger: ['input', 'blur'],
},
], */
  content: [
    {
      required: true,
      message: '',
      trigger: ['input', 'blur']
    },
    {
      whitespace: true,
      message: '不能只输入空格',
      trigger: ['input', 'blur']
    }
  ]
});

const elFormRef = ref();
const elSearchFormRef = ref();

// =========== 表格控制部分 ===========
const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
const tableData = ref([]);
const keywordData = ref([]);
const searchInfo = ref({});
searchInfo.value.id = route.query.id;
searchInfo.value.content = route.query.content;
searchInfo.value.sender = route.query.sender;
searchInfo.value.status_str = route.query.status_str;

// 重置
const onReset = () => {
  searchInfo.value = {};
  getTableData();
  getKeywordData();
};

// 搜索
const onSubmit = () => {
  elSearchFormRef.value?.validate(async (valid) => {
    if (!valid) return;
    page.value = 1;
    getTableData();
    incrementKeywordTimes();
  });
};

// 选择关键词
const checkKeyword = (keyword) => {
  searchInfo.value.content = keyword;
  onSubmit();
};

// 分页
const handleSizeChange = (val) => {
  pageSize.value = val;
  getTableData();
};

// 修改页面容量
const handleCurrentChange = (val) => {
  page.value = val;
  getTableData();
};

// 查询
const getTableData = async () => {
  const table = await getMsgList({ page: page.value, pageSize: pageSize.value, status: status.value, ...searchInfo.value });

  if (table.code === 0) {
    tableData.value = table.data.list;
    total.value = table.data.total;
    page.value = table.data.page;
    pageSize.value = table.data.pageSize;
  }
};

getTableData();

// 更新关键词查询次数
const incrementKeywordTimes = async () => {
  await incrementTimes({ name: searchInfo.value.content });
};

// 获取关键词
const getKeywordData = async () => {
  const res = await getKeywordList({ sort_type: 'times_desc' });

  if (res.code === 0) {
    const buttonTypes = ['primary', 'success', 'info', 'warning', 'danger', ''];

    keywordData.value = res.data.list.map((item, index) => {
      const randomType = buttonTypes[index % buttonTypes.length];
      return {
        ...item,
        type: randomType,
        text: item.title || '新闻精选'
      };
    });
  }
};

getKeywordData();

// 获取需要的字典 可能为空 按需保留
const setOptions = async () => {
  priorityOptions.value = await getDictFunc('bi_im_msg_priority');
  opptyOptions.value = await getDictFunc('bi_im_msg_oppty');
  levelOptions.value = await getDictFunc('bi_im_msg_level');
  statusOptions.value = await getDictFunc('bi_im_msg_status');
};

// 获取需要的字典 可能为空 按需保留
setOptions();

// 多选数据
const multipleSelection = ref([]);

// 多选
const handleSelectionChange = (val) => {
  multipleSelection.value = val;
};

// 删除行
const deleteRow = (row) => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    deleteMsgFunc(row);
  });
};

// 多选删除
const onDelete = async () => {
  ElMessageBox.confirm('确定要删除吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    const ids = [];

    if (multipleSelection.value.length === 0) {
      ElMessage({
        type: 'warning',
        message: '请选择要删除的数据'
      });

      return;
    }

    multipleSelection.value &&
      multipleSelection.value.map((item) => {
        ids.push(item.id);
      });

    const res = await deleteMsgByIds({ ids });

    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '删除成功'
      });

      if (tableData.value.length === ids.length && page.value > 1) {
        page.value--;
      }

      getTableData();
    }
  });
};

// 清理重复
const onDeleteDuplicate = async (params) => {
  ElMessageBox.confirm('确定要清理重复数据吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    const res = await deleteDuplicatePublic(params);

    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '清理成功'
      });

      getTableData();
    }
  });
};

// 行为控制标记（弹窗内部需要增还是改）
const type = ref('');

// 更新行
const updateMsgFunc = async (row) => {
  const res = await findMsg({ id: row.id });
  type.value = 'update';

  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteMsgFunc = async (row) => {
  const res = await deleteMsg({ id: row.id });

  if (res.code === 0) {
    ElMessage({
      type: 'success',
      message: '删除成功'
    });

    if (tableData.value.length === 1 && page.value > 1) {
      page.value--;
    }

    getTableData();
  }
};

const pushTo = (name, query) => {
  if (query) {
    router.push({
      name: name,
      query: query
    });
  } else {
    router.push({ name: name });
  }
};

// 弹窗控制标记
const dialogFormVisible = ref(false);

// 打开弹窗
const openDialog = () => {
  type.value = 'create';
  dialogFormVisible.value = true;
};

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;

  formData.value = {
    client_id: 0,
    sender_id: '',
    receiver_id: '',
    msg_id: '',
    gid: '',
    group: '',
    type: '',
    sender: '',
    sender_remark: '',
    receiver: '',
    send_time: new Date(),
    content: '',
    ics: [],
    level: 0,
    remark: '',
    status: 0
  };
};

// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;

  elFormRef.value?.validate(async (valid) => {
    if (!valid) return (btnLoading.value = false);

    let res;

    switch (type.value) {
      case 'create':
        res = await createMsg(formData.value);
        break;
      case 'update':
        res = await updateMsg(formData.value);
        break;
      default:
        res = await createMsg(formData.value);
        break;
    }

    btnLoading.value = false;

    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '创建/更改成功'
      });

      closeDialog();
      getTableData();
    }
  });
};

const handleEnter = (event) => {
  if (event.shiftKey || event.ctrlKey || event.metaKey) {
    return;
  } else {
    event.preventDefault();
    enterDialog();
  }
};

const detailForm = ref({});

const getStatusType = (status) => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};

// 处理消息中的换行符
const replaceKeywords = (str) => {
  return str.replace(/\n/g, '<br>');
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

  keywords.forEach((k) => {
    if (k) {
      const regex = new RegExp(k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
      highlighted = highlighted.replace(regex, (match) => `<span style="color: #F00;">${match}</span>`);
    }
  });

  return highlighted;
};

// 修改评价等级
const handleRateChange = (row) => {
  formData.value = row;
  updateMsg(formData.value);
};

// 快捷入库
const handleDeal = (row) => {
  formData.value = row;
  formData.value.status = formData.value.status == 1 ? -1 : 1;
  updateMsg(formData.value);
};
</script>

<style scoped>
.gva-search-box {
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

.cell-style {
  line-height: 22px;
}

.text-title {
  line-height: 28px;
  font-size: 15px;
}

.text {
  line-height: 26px;
  color: #000;
  font-size: 15px;
  border-radius: 5px;
  padding: 10px 12px;
  text-align: justify;
}

.text-content {
  background: #98ea70;
}

.text-remark {
  background: #eee;
  margin: 10px 0 0 0;
}

.text-ics {
  background: #FFF;
  border: 1px #EEE solid;
  border-radius: 6px;
  white-space: pre-line;
  padding: 5px 10px;
  color: #333;

  .text-ics-title {
    color: #333;
  }

  .link-ics {
    display: flex;
    justify-content: space-between;

    .el-link {
      color: #555;
      font-weight: normal
    }
  }
}

.text-rate {
  margin: 5px 0 0 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.light {
  .text-title {
    color: #333;
  }

  .text-content {
    background: #98ea70;
    color: #333;
  }

  .text-ics-title {
    color: #333;
  }
}

.selection-popover-demo {
  max-width: 800px;
  margin: 20px auto;
  padding: 20px;
}

.content {
  line-height: 1.8;
  color: #333;
}

.popover-actions {
  display: flex;
  gap: 8px;
}

.reference-element {
  position: fixed;
  width: 1px;
  height: 1px;
  visibility: hidden; /* 不显示但保留位置 */
  z-index: 9999;
}
</style>
