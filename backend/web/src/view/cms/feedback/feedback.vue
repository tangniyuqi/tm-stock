<template>
    <div>
        <div class="gva-search-box">
            <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline"
                @keyup.enter="onSubmit">
                <el-form-item label="标题" prop="title">
                    <el-input v-model="searchInfo.title" clearable placeholder="请输入标题" />
                </el-form-item>

                <el-form-item label="称呼" prop="name">
                    <el-input v-model="searchInfo.name" clearable placeholder="请输入称呼" />
                </el-form-item>

                <el-form-item label="手机" prop="mobile">
                    <el-input v-model="searchInfo.mobile" clearable placeholder="请输入手机" />
                </el-form-item>

                <el-form-item label="微信" prop="wechat">
                    <el-input v-model="searchInfo.wechat" clearable placeholder="请输入微信" />
                </el-form-item>

                <el-form-item label="反馈类型" prop="type">
                    <el-select v-model="searchInfo.type" placeholder="请选择反馈类型" clearable>
                        <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label"
                            :value="Number(item.value)" />
                    </el-select>
                </el-form-item>

                <el-form-item label="审核" prop="review">
                    <el-select v-model="searchInfo.review" placeholder="请选择审核" clearable>
                        <el-option label="通过" :value="true" />
                        <el-option label="待审" :value="false" />
                    </el-select>
                </el-form-item>

                <el-form-item label="状态" prop="status">
                    <el-select v-model="searchInfo.status" placeholder="请选择状态" clearable>
                        <el-option label="启用" :value="true" />
                        <el-option label="禁用" :value="false" />
                    </el-select>
                </el-form-item>

                <el-form-item>
                    <el-button type="primary" icon="search" @click="onSubmit">查询</el-button>
                    <el-button icon="refresh" @click="onReset">重置</el-button>
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

                <el-table-column align="left" label="ID" prop="id" width="80" />

                <el-table-column align="left" label="反馈类型" prop="type" width="150">
                    <template #default="scope">
                        {{ filterDict(String(scope.row.type), typeOptions) }}
                    </template>
                </el-table-column>

                <el-table-column align="left" label="标题" prop="title" min-width="200">
                    <template #default="scope">
                        <span>{{ scope.row.title || '-' }}</span>
                    </template>
                </el-table-column>

                <el-table-column align="left" label="称呼" prop="name" width="100" />

                <el-table-column align="left" label="手机" prop="mobile" width="130" />

                <el-table-column align="left" label="微信" prop="wechat" min-width="150">
                    <template #default="scope">
                        <span>{{ scope.row.wechat || '-' }}</span>
                    </template>
                </el-table-column>

                <el-table-column align="center" label="审核" prop="review" width="100">
                    <template #default="scope">
                        <el-tag :type="scope.row.review ? 'success' : 'info'">{{ formatBoolean(scope.row.review)
                        }}</el-tag>
                    </template>
                </el-table-column>

                <el-table-column align="center" label="状态" prop="status" width="100">
                    <template #default="scope">
                        <el-tag :type="scope.row.status ? 'success' : 'info'">{{ formatBoolean(scope.row.status)
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
                            @click="updateFeedbackFunc(scope.row)">编辑</el-button>
                        <el-button type="primary" link icon="delete" @click="deleteRow(scope.row)">删除</el-button>
                    </template>
                </el-table-column>
            </el-table>

            <div class="gva-pagination">
                <el-pagination layout="total, sizes, prev, pager, next, jumper" :current-page="page"
                    :page-size="pageSize" :page-sizes="[10, 30, 50, 100]" :total="total"
                    @current-change="handleCurrentChange" @size-change="handleSizeChange" />
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
                    <el-divider content-position="left">基本信息</el-divider>
                    <el-col :span="12">
                        <el-form-item label="标题" prop="title">
                            <el-input v-model="formData.title" :clearable="true" placeholder="请输入标题" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item label="反馈类型" prop="type">
                            <el-select v-model="formData.type" :clearable="true" placeholder="请选择反馈类型"
                                style="width: 100%">
                                <el-option v-for="(item, key) in typeOptions" :key="key" :label="item.label"
                                    :value="Number(item.value)" />
                            </el-select>
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item label="封面" prop="cover">
                            <el-input v-model="formData.cover" :clearable="true" placeholder="请输入封面地址" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item label="图集" prop="images">
                            <el-input v-model="formData.images" :clearable="true" placeholder="请输入图集（JSON）" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="24">
                        <el-form-item label="内容" prop="content">
                            <VditorEditor v-model="formData.content" :min-height="320" cache-id="cms-feedback-editor" />
                        </el-form-item>
                    </el-col>

                    <el-divider content-position="left">联系方式</el-divider>
                    <el-col :span="12">
                        <el-form-item label="称呼" prop="name">
                            <el-input v-model="formData.name" :clearable="true" placeholder="请输入称呼" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item label="手机" prop="mobile">
                            <el-input v-model="formData.mobile" :clearable="true" placeholder="请输入手机" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item label="微信" prop="wechat">
                            <el-input v-model="formData.wechat" :clearable="true" placeholder="请输入微信" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item label="电子邮箱" prop="email">
                            <el-input v-model="formData.email" :clearable="true" placeholder="请输入电子邮箱" />
                        </el-form-item>
                    </el-col>

                    <el-divider content-position="left">管理</el-divider>
                    <el-col :span="12">
                        <el-form-item label="会员ID" prop="memberId">
                            <el-input-number v-model="formData.member_id" :min="0" :controls="false" style="width: 100%"
                                clearable placeholder="请输入会员ID" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item label="排序" prop="sort">
                            <el-input-number v-model="formData.sort" :min="0" :controls="false" style="width: 100%"
                                clearable placeholder="请输入排序" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item label="审核" prop="review">
                            <el-switch v-model="formData.review" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item label="状态" prop="status">
                            <el-switch v-model="formData.status" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item label="备注" prop="remark">
                            <el-input v-model="formData.remark" :clearable="true" placeholder="请输入备注" />
                        </el-form-item>
                    </el-col>
                </el-row>
            </el-form>
        </el-drawer>

        <el-drawer destroy-on-close :size="appStore.drawerSize" v-model="detailShow" :show-close="true"
            :before-close="closeDetailShow" title="查看">
            <el-descriptions :column="1" border>
                <el-descriptions-item label="会员ID">
                    {{ detailForm.member_id }}
                </el-descriptions-item>
                <el-descriptions-item label="标题">
                    {{ detailForm.title }}
                </el-descriptions-item>
                <el-descriptions-item label="分类ID">
                    {{ detailForm.cate_id }}
                </el-descriptions-item>
                <el-descriptions-item label="封面">
                    {{ detailForm.cover }}
                </el-descriptions-item>
                <el-descriptions-item label="图集">
                    {{ detailForm.images }}
                </el-descriptions-item>
                <el-descriptions-item label="内容">
                    {{ detailForm.content }}
                </el-descriptions-item>
                <el-descriptions-item label="称呼">
                    {{ detailForm.name }}
                </el-descriptions-item>
                <el-descriptions-item label="手机">
                    {{ detailForm.mobile }}
                </el-descriptions-item>
                <el-descriptions-item label="微信">
                    {{ detailForm.wechat }}
                </el-descriptions-item>
                <el-descriptions-item label="电子邮箱">
                    {{ detailForm.email }}
                </el-descriptions-item>
                <el-descriptions-item label="反馈类型">
                    {{ filterDict(String(detailForm.type), typeOptions) }}
                </el-descriptions-item>
                <el-descriptions-item label="排序">
                    {{ detailForm.sort }}
                </el-descriptions-item>
                <el-descriptions-item label="审核">
                    {{ formatBoolean(detailForm.review) }}
                </el-descriptions-item>
                <el-descriptions-item label="备注">
                    {{ detailForm.remark }}
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
import { formatDate, formatBoolean, getDictFunc, filterDict } from '@/utils/format';
import { createFeedback, deleteFeedback, deleteFeedbackByIds, updateFeedback, findFeedback, getFeedbackList } from '@/api/cms/feedback'
import VditorEditor from '@/components/vditorEditor/VditorEditor.vue'

defineOptions({
    name: 'Feedback'
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

const formData = ref({
    member_id: undefined,
    title: '',
    cate_id: undefined,
    cover: '',
    images: '',
    content: '',
    name: '',
    mobile: '',
    wechat: '',
    email: '',
    type: undefined,
    sort: undefined,
    review: false,
    remark: '',
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
const typeOptions = ref([]);
// 获取反馈类型字典 options
const setOptions = async () => {
    typeOptions.value = await getDictFunc('cms_feedback_type');
};

setOptions();
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
    const table = await getFeedbackList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
    if (table.code === 0) {
        tableData.value = table.data.list;
        total.value = table.data.total;
        page.value = table.data.page;
        pageSize.value = table.data.pageSize;
    }
};

getTableData();

// ============== 表格控制部分结束 ===============

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
        deleteFeedbackFunc(row);
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
        const res = await deleteFeedbackByIds({ ids });
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
const updateFeedbackFunc = async (row) => {
    const res = await findFeedback({ id: row.id });
    type.value = 'update';
    if (res.code === 0) {
        formData.value = res.data;
        dialogFormVisible.value = true;
    }
};

// 删除行
const deleteFeedbackFunc = async (row) => {
    const res = await deleteFeedback({ id: row.id });
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
        member_id: undefined,
        title: '',
        cate_id: undefined,
        cover: '',
        images: '',
        content: '',
        name: '',
        mobile: '',
        wechat: '',
        email: '',
        type: undefined,
        sort: undefined,
        review: false,
        remark: '',
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
                res = await createFeedback(formData.value);
                break;
            case 'update':
                res = await updateFeedback(formData.value);
                break;
            default:
                res = await createFeedback(formData.value);
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
    const res = await findFeedback({ id: row.id });
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
</script>

<style></style>
