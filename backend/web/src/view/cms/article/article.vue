<template>
    <div>
        <div class="gva-search-box">
            <el-form ref="elSearchFormRef" :inline="true" :model="searchInfo" class="demo-form-inline"
                @keyup.enter="onSubmit">
                <el-form-item label="标题" prop="title">
                    <el-input v-model="searchInfo.title" clearable placeholder="请输入标题" />
                </el-form-item>

                <template v-if="showAllQuery">
                    <el-form-item label="作者" prop="author">
                        <el-input v-model="searchInfo.author" clearable placeholder="请输入作者" />
                    </el-form-item>

                    <el-form-item label="分类ID" prop="cateId">
                        <el-input-number v-model="searchInfo.cateId" :min="0" :controls="false" placeholder="请输入分类ID" />
                    </el-form-item>
                </template>

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

                <el-table-column align="left" label="ID" prop="id" width="80" />

                <el-table-column align="left" label="标题" prop="title" min-width="200">
                    <template #default="scope">
                        <span>{{ scope.row.title || '-' }}</span>
                    </template>
                </el-table-column>

                <el-table-column align="left" label="分类ID" prop="cateId" width="100" />

                <el-table-column align="left" label="作者" prop="author" width="120" />

                <el-table-column align="left" label="外链" prop="link" min-width="180">
                    <template #default="scope">
                        <a v-if="scope.row.link" :href="scope.row.link" target="_blank" class="text-primary">{{
                            scope.row.link }}</a>
                        <span v-else>-</span>
                    </template>
                </el-table-column>

                <el-table-column align="left" label="封面" min-width="120">
                    <template #default="scope">
                        <el-image v-if="scope.row.cover" :src="scope.row.cover" style="width: 80px; height: 45px"
                            fit="cover" :preview-src-list="[scope.row.cover]" />
                        <span v-else>-</span>
                    </template>
                </el-table-column>

                <el-table-column align="left" label="推荐位" prop="position" width="90" />

                <el-table-column align="left" label="排序" prop="sort" width="90" />

                <el-table-column align="left" label="浏览量" prop="view" width="90" />

                <el-table-column align="center" label="状态" prop="status" width="100">
                    <template #default="scope">
                        <el-tag :type="getStatusType(scope.row.status)">{{ formatBoolean(scope.row.status) }}</el-tag>
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
                            @click="updateArticleFunc(scope.row)">编辑</el-button>
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
                    <el-col :span="12">
                        <el-form-item label="标题" prop="title">
                            <el-input v-model="formData.title" :clearable="true" placeholder="请输入标题" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="分类ID" prop="cateId">
                            <el-input-number v-model="formData.cate_id" :min="0" :controls="false" style="width: 100%"
                                clearable placeholder="请输入分类ID" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="作者" prop="author">
                            <el-input v-model="formData.author" :clearable="true" placeholder="请输入作者" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="外链" prop="link">
                            <el-input v-model="formData.link" :clearable="true" placeholder="请输入外链" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="SEO关键词" prop="seoKeywords">
                            <el-input v-model="formData.seo_keywords" :clearable="true" placeholder="请输入SEO关键词" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="SEO描述" prop="seoDescription">
                            <el-input v-model="formData.seo_description" :clearable="true" placeholder="请输入SEO描述" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="封面" prop="cover">
                            <el-input v-model="formData.cover" :clearable="true" placeholder="请输入封面地址" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="描述" prop="description">
                            <el-input v-model="formData.description" :clearable="true" placeholder="请输入描述" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="推荐位" prop="position">
                            <el-input-number v-model="formData.position" :min="0" :controls="false" style="width: 100%"
                                clearable placeholder="请输入推荐位" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="排序" prop="sort">
                            <el-input-number v-model="formData.sort" :min="0" :controls="false" style="width: 100%"
                                clearable placeholder="请输入排序" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="浏览量" prop="view">
                            <el-input-number v-model="formData.view" :min="0" :controls="false" style="width: 100%"
                                clearable placeholder="请输入浏览量" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="12">
                        <el-form-item label="状态" prop="status">
                            <el-switch v-model="formData.status" />
                        </el-form-item>
                    </el-col>

                    <el-col :span="24">
                        <el-form-item label="内容" prop="content">
                            <VditorEditor v-model="formData.content" :min-height="320" cache-id="cms-article-editor" />
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
                <el-descriptions-item label="作者">
                    {{ detailForm.author }}
                </el-descriptions-item>
                <el-descriptions-item label="外链">
                    {{ detailForm.link }}
                </el-descriptions-item>
                <el-descriptions-item label="SEO关键词">
                    {{ detailForm.seo_keywords }}
                </el-descriptions-item>
                <el-descriptions-item label="SEO描述">
                    {{ detailForm.seo_description }}
                </el-descriptions-item>
                <el-descriptions-item label="封面">
                    {{ detailForm.cover }}
                </el-descriptions-item>
                <el-descriptions-item label="描述">
                    {{ detailForm.description }}
                </el-descriptions-item>
                <el-descriptions-item label="推荐位">
                    {{ detailForm.position }}
                </el-descriptions-item>
                <el-descriptions-item label="排序">
                    {{ detailForm.sort }}
                </el-descriptions-item>
                <el-descriptions-item label="浏览量">
                    {{ detailForm.view }}
                </el-descriptions-item>
                <el-descriptions-item label="内容">
                    {{ detailForm.content }}
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
import { formatDate, formatBoolean } from '@/utils/format';
import { createArticle, deleteArticle, deleteArticleByIds, updateArticle, findArticle, getArticleList } from '@/api/cms/article'
import VditorEditor from '@/components/vditorEditor/VditorEditor.vue'

defineOptions({
    name: 'Article'
});

// 提交按钮loading
const btnLoading = ref(false);
const appStore = useAppStore();

// 控制更多查询条件显示/隐藏状态
const showAllQuery = ref(false);

const formData = ref({
    title: '',
    cate_id: undefined,
    author: '',
    link: '',
    seo_keywords: '',
    seo_description: '',
    cover: '',
    description: '',
    position: undefined,
    content: '',
    sort: undefined,
    view: undefined,
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
    const table = await getArticleList({ page: page.value, pageSize: pageSize.value, ...searchInfo.value });
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
        deleteArticleFunc(row);
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
        const res = await deleteArticleByIds({ ids });
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
const updateArticleFunc = async (row) => {
    const res = await findArticle({ id: row.id });
    type.value = 'update';
    if (res.code === 0) {
        formData.value = res.data;
        dialogFormVisible.value = true;
    }
};

// 删除行
const deleteArticleFunc = async (row) => {
    const res = await deleteArticle({ id: row.id });
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
        author: '',
        link: '',
        seo_keywords: '',
        seo_description: '',
        cover: '',
        description: '',
        position: undefined,
        content: '',
        sort: undefined,
        view: undefined,
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
                res = await createArticle(formData.value);
                break;
            case 'update':
                res = await updateArticle(formData.value);
                break;
            default:
                res = await createArticle(formData.value);
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
    const res = await findArticle({ id: row.id });
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
    return status ? 'success' : 'info';
};
</script>

<style></style>
