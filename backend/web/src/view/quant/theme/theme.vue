<template>
  <div>
    <div class="gva-table-box">
      <div class="gva-btn-list">
        <el-button v-auth="btnAuth.add" type="primary" icon="plus" @click="openDialog()">新增</el-button>
        <el-button v-auth="btnAuth.batchDelete" icon="delete" style="margin-left: 10px" :disabled="!multipleSelection.length" @click="onDelete">删除</el-button>
      </div>

      <el-table ref="multipleTable" style="width: 100%" tooltip-effect="dark" :data="tableData" row-key="id" @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="55" />

        <el-table-column align="left" label="题材名称" prop="name" min-width="150" />

        <el-table-column align="left" label="题材代码" prop="code" min-width="150" />

        <el-table-column align="left" label="涨跌幅" prop="change_pct" width="90">
          <template #default="scope">
            {{ scope.row.change_pct !== undefined && scope.row.change_pct !== null ? Number(scope.row.change_pct).toFixed(2) + '%' : '' }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="数量" prop="stock_count" width="90" />

        <el-table-column align="left" label="层级" prop="level" width="90">
          <template #default="scope">
            {{ filterDict(String(scope.row.level), levelOptions) }}
          </template>
        </el-table-column>

        <!-- <el-table-column align="left" label="备注" prop="remark" min-width="90" /> -->

        <el-table-column align="left" label="来源" prop="source" width="90">
          <template #default="scope">
            {{ filterDict(String(scope.row.source), sourceOptions) }}
          </template>
        </el-table-column>

        <el-table-column align="left" label="排序" prop="sort" width="60" />

        <el-table-column align="center" label="状态" prop="status" width="120">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">{{ filterDict(String(scope.row.status), statusOptions) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column sortable align="left" label="创建时间" prop="created_at" min-width="100">
          <template #default="scope">{{ formatToDateTime(scope.row.created_at, 'MM-DD HH:mm') }}</template>
        </el-table-column>

        <el-table-column align="left" label="操作" fixed="right" :min-width="270">
          <template #default="scope">
            <el-button v-auth="btnAuth.add" type="primary" link class="table-button" @click="openDialog(scope.row)">
              <el-icon style="margin-right: 5px"><CirclePlus /></el-icon>子题材
            </el-button>

            <el-button v-auth="btnAuth.add" type="primary" link class="table-button" @click="goToThemeStock(scope.row)">
              <el-icon style="margin-right: 5px"><TrendCharts /></el-icon>股票
            </el-button>

            <el-button v-auth="btnAuth.edit" type="primary" link icon="edit" class="table-button" @click="updateThemeFunc(scope.row)">编辑</el-button>
            <el-button v-if="!scope.row.children?.length" v-auth="btnAuth.delete" type="primary" link icon="delete" @click="deleteRow(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="gva-pagination">
        <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page" :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total" @current-change="handleCurrentChange" @size-change="handleSizeChange" />
      </div>
    </div>

    <el-drawer destroy-on-close :size="appStore.drawerSize * 1.2" v-model="dialogFormVisible" :show-close="false" :before-close="closeDialog">
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
            <el-form-item label="父级" prop="parent_id">
              <el-tree-select v-model="formData.parent_id" :data="[rootNode, ...tableData]" check-strictly :render-after-expand="false" :props="defaultProps" clearable style="width: 100%" placeholder="根节点" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="名称" prop="name">
              <el-input v-model="formData.name" :clearable="true" placeholder="请输入名称" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="代码" prop="code">
              <el-input v-model="formData.code" :clearable="true" placeholder="请输入代码" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="涨跌幅" prop="change_pct">
              <el-input-number v-model="formData.change_pct" :controls="true" :step="0.01" :precision="2" style="width: 100%" placeholder="请输入涨跌幅" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="层级" prop="level">
              <el-select v-model="formData.level" disabled clearable placeholder="父级自动计算层级">
                <el-option v-for="(item, key) in levelOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="描述" prop="description">
              <el-input v-model="formData.description" type="textarea" :rows="6" :clearable="true" placeholder="请输入描述" />
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="备注" prop="remark">
              <el-input v-model="formData.remark" :clearable="true" placeholder="请输入备注" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="来源" prop="source">
              <el-select v-model="formData.source" clearable placeholder="请选择来源">
                <el-option v-for="(item, key) in sourceOptions" :key="key" :label="item.label" :value="Number(item.value)" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="排序" prop="sort">
              <el-input v-model.number="formData.sort" :clearable="true" placeholder="请输入排序" />
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="状态" prop="status">
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
import { ref, reactive, onUnmounted, watch } from 'vue';
import { useAppStore } from '@/pinia';
import { useBtnAuth } from '@/utils/btnAuth';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { getDictFunc, formatDate, formatBoolean, filterDict, filterDataSource, returnArrImg, onDownloadFile } from '@/utils/format';
import { formatToDateTime, getStartOfDayTimestamp, getDateDifference, getCountdownDays } from '@/utils/dateTimeUtils';

import { createTheme, deleteTheme, deleteThemeByIds, updateTheme, findTheme, getThemeList } from '@/api/quant/theme';

defineOptions({
  name: 'Theme',
});
// 按钮权限实例化
const btnAuth = useBtnAuth();
const route = useRoute();
const router = useRouter();

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);

const statusOptions = ref([]);
const levelOptions = ref([]);
const sourceOptions = ref([]);

// 自动化生成的字典（可能为空）以及字段
const formData = ref({
  name: '',
  code: '',
  change_pct: 0,
  level: 1,
  parent_id: 0,
  description: '',
  remark: '',
  source: 0,
  sort: 0,
  status: 1,
});

// 验证规则
const rule = reactive({
  name: [
    {
      required: true,
      message: '请输入名称',
      trigger: ['input', 'blur'],
    },
    {
      whitespace: true,
      message: '不能只输入空格',
      trigger: ['input', 'blur'],
    },
  ],
});

const elFormRef = ref();
const elSearchFormRef = ref();

// =========== 表格控制部分 ===========
const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
const tableData = ref([]);
const searchInfo = ref({});
// 树选择器配置
const defaultProps = {
  children: 'children',
  label: 'name',
  value: 'id',
};

const rootNode = {
  id: 0,
  name: '根节点',
  children: [],
};

const findNodeById = (id, nodes = []) => {
  for (const node of nodes) {
    if (node.id === id) {
      return node;
    }
    if (node.children?.length) {
      const found = findNodeById(id, node.children);
      if (found) {
        return found;
      }
    }
  }
  return null;
};

const updateLevelByParent = parentId => {
  if (!parentId) {
    formData.value.level = 1;
    return;
  }
  const parent = findNodeById(parentId, tableData.value);
  const parentLevel = parent && parent.level != null ? Number(parent.level) : 0;
  formData.value.level = parentLevel + 1;
};

watch(
  () => formData.value.parent_id,
  newParentId => {
    updateLevelByParent(newParentId);
  }
);

// 查询
const getTableData = async () => {
  const table = await getThemeList();
  if (table.code === 0) {
    tableData.value = table.data || [];
  }
};

getTableData();

// ============== 表格控制部分结束 ===============

// 获取需要的字典 可能为空 按需保留
const setOptions = async () => {
  [statusOptions.value, levelOptions.value, sourceOptions.value] = await Promise.all([getDictFunc('status'), getDictFunc('quant_theme_level'), getDictFunc('quant_theme_source')]);
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
    deleteThemeFunc(row);
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
    const res = await deleteThemeByIds({ ids });
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

// 行为控制标记（弹窗内部需要增还是改）
const type = ref('');

const goToThemeStock = row => {
  const themeId = row?.id;
  if (!themeId) return;
  const targetPath = route.path.includes('/layout') ? route.path.replace(/\/theme$/, '/themeStock') : '/themeStock';
  router.push({
    path: targetPath,
    query: { theme_id: themeId },
  });
};

// 更新行
const updateThemeFunc = async row => {
  const res = await findTheme({ id: row.id });
  type.value = 'update';
  if (res.code === 0) {
    formData.value = res.data;
    updateLevelByParent(formData.value.parent_id);
    dialogFormVisible.value = true;
  }
};

// 删除行
const deleteThemeFunc = async row => {
  const res = await deleteTheme({ id: row.id });
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

// 弹窗控制标记
const dialogFormVisible = ref(false);

// 打开弹窗
const openDialog = row => {
  type.value = 'create';
  formData.value.parent_id = row ? row.id : 0;
  updateLevelByParent(formData.value.parent_id);
  dialogFormVisible.value = true;
};

// 关闭弹窗
const closeDialog = () => {
  dialogFormVisible.value = false;
  formData.value = {
    name: '',
    code: '',
    change_pct: 0,
    level: 1,
    parent_id: 0,
    description: '',
    remark: '',
    source: 0,
    sort: 0,
    status: 1,
  };
};
// 弹窗确定
const enterDialog = async () => {
  btnLoading.value = true;
  elFormRef.value?.validate(async valid => {
    if (!valid) return (btnLoading.value = false);
    let res;
    switch (type.value) {
      case 'create':
        res = await createTheme(formData.value);
        break;
      case 'update':
        res = await updateTheme(formData.value);
        break;
      default:
        res = await createTheme(formData.value);
        break;
    }
    btnLoading.value = false;
    if (res.code === 0) {
      ElMessage({
        type: 'success',
        message: '创建/更改成功',
      });
      closeDialog();
      getTableData();
    }
  });
};

const getStatusType = status => {
  return status === -1 ? 'warning' : status === 0 ? 'info' : 'success';
};
</script>

<style></style>
