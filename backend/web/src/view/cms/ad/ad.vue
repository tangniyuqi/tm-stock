<template>
  <div>
    <div class="gva-search-box">
      <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline"
        @keyup.enter="onSubmit">
        <el-form-item label="标题" prop="title">
          <el-input v-model="searchInfo.title" clearable placeholder="请输入标题" />
        </el-form-item>

        <el-form-item label="状态" prop="status">
          <el-select v-model="searchInfo.status" placeholder="请选择状态" clearable>
            <el-option v-for="(item, key) in statusOptions" :key="key" :label="item.label"
              :value="Number(item.value)" />
          </el-select>
        </el-form-item>

        <template v-if="showAllQuery"></template>

        <el-form-item>
          <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
          <el-button icon="refresh" @click="onReset">重置</el-button>
          <el-button link type="primary" icon="arrow-down" @click="showAllQuery = true"
            v-if="!showAllQuery">展开</el-button>
          <el-button link type="primary" icon="arrow-up" @click="showAllQuery = false" v-else>收起</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="gva-table-box">
      <div class="gva-btn-list">
        <el-button type="primary" icon="plus" @click="openDialog()">新增</el-button>
        <el-button icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length"
          @click="onDelete">删除</el-button>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id"
        @selection-change="handleSelectionChange">
        <el-table-column align="center" type="selection" width="60" />

        <el-table-column align="left" label="ID" prop="id" width="120" />

        <el-table-column align="left" label="标题" prop="title" min-width="240">
          <template #default="scope">
            <span>{{ scope.row.title || '-' }}</span>
          </template>
        </el-table-column>

        <el-table-column align="left" label="分类ID" prop="cateId" width="120" />

        <el-table-column align="left" label="封面" min-width="120">
          <template #default="scope">
            <el-image v-if="scope.row.cover" :src="scope.row.cover" style="width: 80px; height: 45px" fit="cover"
              :preview-src-list="[scope.row.cover]" />
            <span v-else>-</span>
          </template>
        </el-table-column>

        <el-table-column align="left" label="广告位ID" prop="locationId" width="120" />

        <el-table-column align="left" label="文本描述" prop="silderText" min-width="180">
          <template #default="scope">
            <span>{{ scope.row.silder_text || '-' }}</span>
          </template>
        </el-table-column>

        <el-table-column align="left" label="链接类型" prop="linkType" width="100">
          <template #default="scope">
            {{ formatBoolean(scope.row.link_type) }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="链接ID" prop="linkId" width="100" />

        <el-table-column align="left" label="优先级" prop="sort" width="90" />

        <el-table-column align="center" label="状态" prop="status" width="100">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">{{ filterDict(String(scope.row.status), statusOptions)
              }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column sortable align="left" label="创建日期" prop="createdAt" width="180">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" :min-width="appStore.operateMinWith">
          <template #default="scope">
            <el-button type="primary" link class="table-button" @click="getDetails(scope.row)">
              <el-icon style="margin-right: 5px">
                <InfoFilled />
              </el-icon>
              查看
            </el-button>
            <el-button type="primary" link icon="edit" class="table-button"
              @click="updateAdFunc(scope.row)">编辑</el-button>
            <el-button type="primary" link icon="delete" @click="deleteRow(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize"
          :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange"
          @size-change="handleSizeChange" />
      </div>
    </div>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="dialogFormVisible" :show-close="false"
      :before-close="closeDialog">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-lg">{{ type === 'create' ? '新增' : '编辑' }}</span>
          <div>
            <el-button :loading="btnLoading" type="primary" @click="enterDialog">确 定</el-button>
            <el-button @click="closeDialog">取 消</el-button>
          </div>
        </div>
      </template>

      <el-form :model="formData" label-position="top" ref="elFormRef" :rules="rule" label-width="80px">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="标题" prop="title">
              <el-input v-model="formData.title" :clearable="true" placeholder="请输入标题" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="分类ID" prop="cateId">
              <el-input-number v-model="formData.cate_id" :min="0" :controls="false" style="width: 100%" clearable
                placeholder="请输入分类ID" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="封面" prop="cover">
              <el-input v-model="formData.cover" :clearable="true" placeholder="请输入封面地址" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="广告位ID" prop="locationId">
              <el-input-number v-model="formData.location_id" :min="0" :controls="false" style="width: 100%" clearable
                placeholder="请输入广告位ID" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="文本描述" prop="silderText">
              <el-input v-model="formData.silder_text" :clearable="true" placeholder="请输入文本描述" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="链接类型" prop="linkType">
              <el-switch v-model="formData.link_type" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="链接ID" prop="linkId">
              <el-input-number v-model="formData.link_id" :min="0" :controls="false" style="width: 100%" clearable
                placeholder="请输入链接ID" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="优先级" prop="sort">
              <el-input-number v-model="formData.sort" :min="0" :controls="false" style="width: 100%" clearable
                placeholder="请输入优先级" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="状态" prop="status">
              <el-switch v-model="formData.status" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-drawer>

    <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true"
      :before-close="closeDetailShow" title="查看">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="标题">
          {{ detailForm.title }}
        </el-descriptions-item>
        <el-descriptions-item label="分类ID">
          {{ detailForm.cate_id }}
        </el-descriptions-item>
        <el-descriptions-item label="封面">
          {{ detailForm.cover }}
        </el-descriptions-item>
        <el-descriptions-item label="广告位ID">
          {{ detailForm.location_id }}
        </el-descriptions-item>
        <el-descriptions-item label="文本描述">
          {{ detailForm.silder_text }}
        </el-descriptions-item>
        <el-descriptions-item label="链接类型">
          {{ formatBoolean(detailForm.link_type) }}
        </el-descriptions-item>
        <el-descriptions-item label="链接ID">
          {{ detailForm.link_id }}
        </el-descriptions-item>
        <el-descriptions-item label="优先级">
          {{ detailForm.sort }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          {{ formatBoolean(detailForm.status) }}
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue';
import { useAppStore } from '@/pinia';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict } from '@/utils/format';
import { createAd, deleteAd, deleteAdByIds, updateAd, findAd, getAdList } from '@/api/cms/ad'

defineOptions({
  name: 'Ad'
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);
const statusOptions = ref([]);

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  title: '',
  cate_id: undefined,
  cover: '',
  location_id: undefined,
  silder_text: '',
  link_type: false,
  link_id: undefined,
  start_time: undefined,
  end_time: undefined,
  sort: undefined,
  status: false
});

// 验证规则
const rule = reactive({});

const elFormRef = ref();
const elSearchFormRef = ref();

// =========== 表格控制部分 ===========
const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
const tableData = ref([]);
const searchInfo = ref({});
// 重置
const onReset = () => {
  searchInfo.value = {};
  getTableData();
};

// 搜索
const onSubmit = () => {
  elSearchFormRef.value?.validate(async (valid) => {
    if (!valid) return;
    page.value = 1;
    getTableData();
  });
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
  const table = await getAdList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
  if (table.code === 0) {
    tableData.value = table.data.list;
    total.value = table.data.total;
    page.value = table.data.page;
    pageSize.value = table.data.pageSize;
  }
};

getTableData();

// ============== 表格控制部分结束 ===============

// 获取需要的字典 可能为空 按需保留
const setOptions = async () => {
  statusOptions.value = await getDictFunc('status');
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
    deleteAdFunc(row);
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
    const res = await deleteAdByIds({ ids });
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

// 行为控制标记（弹窗内部需要增还是改）
const type = ref('');

// 更新行
const updateAdFunc = async (row) => {
  const res = await findAd({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteAdFunc = async (row) => {
  const res = await deleteAd({ id: row.id });
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
    title: '',
    cate_id: undefined,
    cover: '',
    location_id: undefined,
    silder_text: '',
    link_type: false,
    link_id: undefined,
    start_time: undefined,
    end_time: undefined,
    sort: undefined,
    status: false
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
        res = await createAd(formData.value);
        break;
      case 'update':
        res = await updateAd(formData.value);
        break;
      default:
        res = await createAd(formData.value);
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

const detailForm = ref({});

// 查看详情控制标记
const detailShow = ref(false);

// 打开详情弹窗
const openDetailShow = () => {
  detailShow.value = true;
};

// 打开详情
const getDetails = async (row) => {
  // 打开弹窗
  const res = await findAd({ id: row.id });
  if (res.code === 0) {
    detailForm.value = res.data;
    openDetailShow();
  }
};

// 关闭详情弹窗
const closeDetailShow = () => {
  detailShow.value = false;
  detailForm.value = {};
};

const getStatusType = (status) => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};
</script>

<style></style>