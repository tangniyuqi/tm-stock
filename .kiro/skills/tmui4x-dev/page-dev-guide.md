# 页面开发详细规范

> 本文档是 `pages/` 目录下页面开发的完整参考，包含页面结构、Store 创建、API 请求封装、类型定义，以及每个 tmx-ui 组件的使用示例。
> 在新项目中 demo 页面不存在时，以本文档为唯一参考。

---

## 1. 页面基础结构

每个 `.uvue` 页面遵循以下结构模板：

```uvue
<template>
	<!-- #ifdef APP -->
	<scroll-view style="flex:1">
	<!-- #endif -->
	<!-- #ifdef MP-WEIXIN -->
	<page-meta :page-style="`background-color:${xThemeConfigBgColor}`">
		<navigation-bar :background-color="xThemeConfigNavBgColor"
			:front-color="xThemeConfigNavFontColor"></navigation-bar>
	</page-meta>
	<!-- #endif -->

	<!-- 页面内容 -->

	<view class="py-32"></view>
	<!-- #ifdef APP -->
	</scroll-view>
	<!-- #endif -->
</template>

<script lang="ts" setup>

</script>

<style lang="scss" scoped>

</style>
```

要点：
- APP 端用 `scroll-view style="flex:1"` 包裹实现页面滚动
- 微信小程序用 `page-meta` + `navigation-bar` 配置导航栏和主题色
- 底部留白用 `<view class="py-32"></view>` 或固定高度 view
- 使用 Composition API（`<script setup>`）
- **tmx-ui 容器组件（`x-sheet`、`x-row`、`x-col` 等）禁止多层嵌套**，外层套一层后，内部使用 `view` 等官方基础组件布局

---

## 2. Store 创建规范

在 `pages/libs/` 下创建全局状态管理。

### 2.1 定义类型（interface.uts）

```uts
// pages/libs/interface.uts

// 统一的请求响应结构体
export type resultDataType = {
	msg: string,
	code: number,
	data: any
}

// 用户信息
export type UseInfo = {
	naicename: string,
	avatar: string,
	id: string,
	tags: string[],
	level: number
}

// 统一的列表分页结构体
export type resultListType<T> = {
	pagetCount: number,
	count: number,
	page: number,
	listCount: number,
	datalist: T[]
}
```

### 2.2 创建 Store（useUseStore.uts）

使用 `reactive()` 创建全局响应式状态，导出修改函数：

```uts
// pages/libs/useUseStore.uts
import { UseInfo } from "./interface"

export type useUseStoreType = {
	token: string | null,
	uploadUrl: string,
	imgHost: string,
	user: UseInfo
}

export const useUseStore = reactive({
	token: null,
	uploadUrl: "https://baidu.com",
	imgHost: "https://baidu.com",
	user: {
		naicename: '',
		avatar: '',
		id: '',
		tags: [] as string[],
		level: -1
	} as UseInfo
} as useUseStoreType)

export const setLogin = (token: string | null, user: UseInfo | null) => {
	useUseStore.token = token;
	if (token != null) {
		uni.setStorageSync('token', token!)
	}
	if (user != null) {
		useUseStore.user = user
	}
}

export const loginOut = () => {
	useUseStore.token = null;
	useUseStore.user = {
		naicename: '',
		avatar: '',
		id: '',
		tags: [] as string[],
		level: -1
	} as UseInfo
}
```

要点：
- 用 `reactive()` + `as StoreType` 创建响应式对象
- 导出修改函数（`setLogin`、`loginOut`），不直接暴露 reactive 的内部修改
- token 持久化用 `uni.setStorageSync`

### 2.3 页面中使用 Store

```uts
import { useUseStore, setLogin, loginOut } from "./libs/useUseStore"
import { UseInfo } from "./libs/interface"

// 读取
const token = useUseStore.token
const userName = useUseStore.user.naicename

// 写入
setLogin("new-token", { naicename: '张三', avatar: '', id: '1', tags: [], level: 1 } as UseInfo)

// 退出
loginOut()
```

---

## 3. API 请求封装规范

### 3.1 封装请求函数（api.uts）

```uts
// pages/libs/api.uts
import { xRequest } from "../../uni_modules/tmx-ui/index.uts";
import { xRequestMethond, xRequestOptions, xRequestResult } from "../../uni_modules/tmx-ui/interface";
import { useUseStore } from "./useUseStore";
import { resultDataType, resultListType } from "./interface"

function rq<T>(url: string, method: xRequestMethond = "POST", customData: UTSJSONObject = {} as UTSJSONObject, useCache: boolean = false): Promise<T | null> {
	let xrq = new xRequest()
	let token = useUseStore.token
	xRequest.setHostUrl("https://your-api-host.com")

	return new Promise((resolve, rej) => {
		xrq.request({
			url: url,
			method: method,
			data: customData,
			useCache: useCache,
			loadToastText: "请求中",
			header: {
				"token": token,
				"Content-Type": "application/json"
			} as UTSJSONObject
		} as xRequestOptions)
		.then((resBydata) => {
			let res = resBydata! as xRequestResult;

			if (res.statusCode != 200) {
				uni.showModal({
					title: "系统错误",
					content: `错误码:${res.statusCode},发生了系统错误,请重试`
				})
				rej(null)
				return;
			}

			let reqData = res.data as any | null;
			if (typeof reqData == 'string') {
				reqData = JSON.parseObject(reqData! as string)
			}
			if (typeof reqData != 'object' || reqData == null) {
				uni.showModal({
					title: "服务器异常",
					content: `服务器没有正确返回json数据格式`
				})
				rej(null)
				return;
			}

			let rdata = reqData! as UTSJSONObject;
			let msgdata = JSON.stringify(rdata) as string
			let d = JSON.parse<resultDataType | null>(msgdata)

			if (d == null) { rej(null); return; }

			if (d.code == -1) {
				uni.showToast({ title: "未登录", icon: "none" })
				rej(null)
				return
			}

			if (d.code != 0) {
				uni.showToast({ title: d.msg, icon: "none" })
				rej(null)
				return
			}

			resolve(d.data as T | null)
		})
		.catch(() => { rej(null) })
	})
}

export class api {
	// 请求列表示例
	public static async getArtListArt<T>(arg: UTSJSONObject): Promise<T[] | null> {
		let result = await rq<any>('/admin/get_actilList/', 'POST', arg, true);
		if (result == null) return Promise.reject(null)
		let pds = JSON.stringify(result! as UTSJSONObject)! as string
		let ds = JSON.parse<resultListType<T>>(pds);
		return Promise.resolve(ds!.datalist)
	}
}
```

### 3.2 页面中调用 API

```uts
import { api } from "./libs/api"
import { LISTPAGEART_TYPE } from "./libs/interface"

const loadData = async () => {
	let result = await api.getArtListArt<LISTPAGEART_TYPE>({ page: 1 } as UTSJSONObject)
	if (result != null) {
		list.value = result
	}
}
```

要点：
- `xRequest` 来自 tmx-ui，封装了加载提示、缓存等
- 统一用 `resultDataType` 解析响应，判断 `code` 值
- API 方法用 `class` + `static async` 组织
- 泛型 `<T>` 传递具体业务类型

---

## 4. tmx-ui 组件使用示例

### 4.1 常用组件

#### x-sheet（容器）

> **重要：`x-sheet` 禁止多层嵌套！** 只能在外层套一层 `x-sheet`，内部使用 `view` 或其他非容器类 tmx-ui 组件。
> 同理，`x-row`、`x-col` 等 tmx-ui 容器组件也不建议二次嵌套，内部布局应使用官方基础组件（`view`、`scroll-view` 等）。

```html
<!-- 正确：x-sheet 内部用 view 或其他非容器组件 -->
<x-sheet>
	<view class="flex-row pa-12">
		<x-text>内容</x-text>
		<x-button>按钮</x-button>
	</view>
</x-sheet>

<!-- 错误：x-sheet 嵌套 x-sheet -->
<!-- <x-sheet>
	<x-sheet>不要这样写</x-sheet>
</x-sheet> -->
```

```html
<x-sheet>默认容器</x-sheet>
<x-sheet :round="['12','2']" color="primary">
	<x-text color="white">自定圆角和颜色</x-text>
</x-sheet>
<x-sheet :round="['12','2']" :border="['2','2']" :border-color="['red','primary']">
	<x-text>边线样式</x-text>
</x-sheet>
<x-sheet :linearGradient="['left','#ff667f','#fdb247']">
	<x-text color="white">渐变背景</x-text>
</x-sheet>
<x-sheet :loading="true">
	<x-text>内容加载中</x-text>
</x-sheet>
```

#### x-button（按钮）

```html
<!-- 基础 -->
<x-button :block="true">主色按钮</x-button>
<x-button :block="true" color="error">错误</x-button>
<x-button :block="true" color="success">成功</x-button>

<!-- 尺寸 -->
<x-button size="mini">超小</x-button>
<x-button size="small">小</x-button>
<x-button size="normal">正常</x-button>
<x-button size="large">大</x-button>

<!-- 样式 skin -->
<x-button skin="thin">浅色</x-button>
<x-button skin="outline">镂空</x-button>
<x-button skin="dashed">虚线</x-button>
<x-button skin="text">文本</x-button>

<!-- 图标 -->
<x-button icon="thumb-up-fill">图标按钮</x-button>
<x-button :iconBtn="true" icon="lock-unlock-fill" round="88" width="88"></x-button>

<!-- 渐变 -->
<x-button :block="true" :border="0" round="88" color="#ff667f"
	:linearGradient="['left','#ff667f','#fdb247']">渐变</x-button>

<!-- 状态 -->
<x-button :disabled="true">禁用</x-button>
<x-button :loading="true">加载中</x-button>
```

#### x-text（文本）

```html
<x-text>普通文本</x-text>
<x-text font-size="18" class="text-weight-b">加粗标题</x-text>
<x-text color="#999999">灰色说明</x-text>
<x-text :lines="2">超出2行截断的文本内容...</x-text>

<!-- 高亮 -->
<x-text :highlight="['高亮','关键词']" label="文本中的高亮和关键词会变色"></x-text>

<!-- 正则高亮（电话、邮箱） -->
<x-text @item-click="onClick" highlightColor="red"
	:highlightReg="['1[3456789]\\d{9}','[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}']"
	label="联系电话:17970689633，邮箱:test@qq.com">
</x-text>
```

#### x-icon（图标）

```html
<!-- 使用 remixicon 名称（不含 ri- 前缀） -->
<x-icon name="chat-3-line"></x-icon>
<x-icon name="chat-3-fill" color="primary" font-size="32"></x-icon>
<x-icon name="arrow-up-line" :rotation="90"></x-icon>
<x-icon :spin="true" font-size="64" color="red" name="loader-line"></x-icon>
```

#### x-tag（标签）

```html
<x-tag size="mini">标签</x-tag>
<x-tag size="normal" color="success">成功</x-tag>
<x-tag skin="outline">镂空</x-tag>
<x-tag skin="thin">浅色</x-tag>
<x-tag icon="checkbox-circle-line" :border="0" :round="66"
	:linearGradient="['left','#063CFF','#FF3F3F']">渐变图标标签</x-tag>
```

### 4.2 表单组件

#### x-input（输入框）

```html
<x-input v-model="content"></x-input>
<x-input :password="true"></x-input>
<x-input left-icon="money-cny-circle-line" type="number" placeholder="请输入金额"
	right-text="万元" left-text="物料金额"></x-input>

<!-- 插槽自定义左右内容 -->
<x-input left-icon="terminal-line" placeholder="输入验证码">
	<template v-slot:inputLeft>
		<x-text class="ml-12">4位码</x-text>
	</template>
	<template v-slot:inputRight>
		<x-button round="8" class="mr-2" height="40" width="90">获取</x-button>
	</template>
</x-input>

<!-- 文本域 -->
<x-input type="textarea" :autoHeight="true" :maxlength="255" :showFooter="true">
	<template #footer>
		<x-text color="error">请不要超过字符数量</x-text>
	</template>
</x-input>
```

#### x-form + x-form-item（表单）

```uts
import { FORM_RULE, FORM_SUBMIT_RESULT } from "@/uni_modules/tmx-ui/interface.uts"

type USER_TYPE = {
	username: string;
	title: string;
	price: string;
	num: number;
	radio: string;
	checkbox: string[];
}

const form = ref<XFormComponentPublicInstance | null>(null)

let rules = new Map<string, FORM_RULE[]>([
	["username", [{
		type: "string",
		errorMessage: "姓名不能空且小于4字符",
		trigger: 'blur',
		valid: (val: any | null): boolean => {
			let pval = val as string;
			return pval.length > 0 && pval.length <= 4
		}
	}]],
	["price", [{ type: "number", errorMessage: "价格不能小于30", min: 30 }]],
	["num", [{ type: "number", errorMessage: "库存在2-100之间", min: 2, max: 100 }]],
])

const reqData = ref<USER_TYPE>({
	username: "", title: "", price: "", num: 0, radio: "", checkbox: [] as string[]
})

const submitData = (evt: FORM_SUBMIT_RESULT) => {
	if (!evt.valid) {
		uni.showToast({ title: evt.errorMessage, icon: 'none' })
		return;
	}
	// 提交成功逻辑
}
```

```html
<x-form ref="form" :rules="rules" @submit="submitData" v-model="(reqData as USER_TYPE)">
	<x-form-item field="username" label="联系姓名" :required="true">
		<x-input color='transparent' v-model="(reqData.username as string)" align="right"></x-input>
	</x-form-item>
	<x-form-item field="price" label="产品价格" :required="true">
		<x-input color='transparent' type="number" v-model="(reqData.price as string)"
			right-text="万元" align="right"></x-input>
	</x-form-item>
	<x-form-item field="num" label="库存数量" :required="true">
		<view class="flex flex-row flex-row-center-end">
			<x-stepper width="120" v-model="(reqData.num as number)"></x-stepper>
		</view>
	</x-form-item>
	<x-button form-type="form" :block="true" class="mt-32">提交</x-button>
</x-form>

<!-- ref 方法调用 -->
form.value!.valid(['username'] as string[])   // 验证指定字段
form.value!.submit()                           // 提交
form.value!.clearValid()                       // 清除验证
```

#### x-checkbox / x-checkbox-group

```html
<!-- 单个 -->
<x-checkbox label="苹果"></x-checkbox>
<x-checkbox color="error" unCheckColor="error" label="香蕉"></x-checkbox>

<!-- 组合 -->
<x-checkbox-group v-model="checkbox">
	<x-checkbox v-for="(item,index) in list" :key="index" :label="item.label"
		:value="item.id" class="pr-12 mb-12"></x-checkbox>
</x-checkbox-group>
```

```uts
const checkbox = ref(['2', '3'])
```

#### x-radio / x-radio-group

```html
<x-radio label="苹果"></x-radio>

<x-radio-group v-model="selected">
	<x-radio v-for="(item,index) in list" :key="index" :label="item.label"
		:value="item.id" :onlyChecked="true" class="pr-12 mb-12"></x-radio>
</x-radio-group>
```

```uts
const selected = ref("3")
```

#### x-switch（开关）

```html
<x-switch v-model="switchVal" @change="onchange"></x-switch>
<x-switch size="large" activeIcon="verified-badge-fill" icon="verified-badge-line"></x-switch>
<x-switch :label='["开","关"]'></x-switch>
<x-switch color="danger" :modelValue="true"></x-switch>
<x-switch round="4" :space="3"></x-switch>
```

#### x-slider / x-slider-double（滑块）

```html
<x-slider :model-value="64"></x-slider>
<x-slider v-model="value" :min="50" :max="300" color="success" :show-label="true"></x-slider>

<x-slider-double :model-value="[10,30]"></x-slider-double>
<x-slider-double v-model="range" :min="50" :max="300" :show-label="true"></x-slider-double>
```

#### x-rate（评分）

```html
<x-rate></x-rate>
<x-rate :model-value="2.5" :half="true" size="32" :count="5"></x-rate>
<x-rate color="success" :model-value="5" :show-score="true"></x-rate>
<x-rate :model-value="5" :readonly="true"></x-rate>
```

#### x-stepper（步进器）

```html
<x-stepper width="110"></x-stepper>
<x-stepper :decimal-len="1" :step="0.1" width="110"></x-stepper>
<x-stepper :splitBtn="true" btn-color="primary" btn-font-color="white" :model-value="10" width="110"></x-stepper>
<x-stepper :autoHideBtn="true" :max="5" width="110"></x-stepper>
```

#### x-picker（选择器）

```html
<x-picker v-model="(selecteds as string[])" v-model:model-str="(str as string)" :list="list">
	<template v-slot:default="{label}">
		<x-button :block="true">打开选项({{label}})</x-button>
	</template>
</x-picker>
```

```uts
import { PICKER_ITEM_INFO } from "@/uni_modules/tmx-ui/interface.uts"
const list = ref([
	{
		title: '江西', id: "9-1",
		children: [
			{ title: '南昌', id: "132", children: [
				{ title: '青山湖区', id: "1-2" } as PICKER_ITEM_INFO,
				{ title: '高新区', id: "1-3", disabled: true } as PICKER_ITEM_INFO,
			] as PICKER_ITEM_INFO[] } as PICKER_ITEM_INFO,
		] as PICKER_ITEM_INFO[]
	} as PICKER_ITEM_INFO,
] as PICKER_ITEM_INFO[])
const selecteds = ref<string[]>([])
const str = ref<string>("")
```

#### x-upload-media（图片上传）

```html
<x-upload-media v-model="(list as XUPLOADFILE_FILE_VALUE[])" :column="4"
	:header="headerToken" :beforeUpload="beforeUpload" :before-del="beforeRemove">
</x-upload-media>

<!-- 视频上传 + ref 触发 -->
<x-upload-media mode="video" ref="uploader" :auto-start="false" :column="4"></x-upload-media>
<x-button @click="uploader.value!.upload()">开始上传</x-button>
```

```uts
import { XUPLOADFILE_FILE_VALUE, XUPLOADFILE_FILE_INFO } from "@/uni_modules/tmx-ui/interface.uts"
const list = ref<XUPLOADFILE_FILE_VALUE[]>([
	{ url: 'https://example.com/img1.png' },
	{ url: 'https://example.com/img2.png' },
])
const headerToken = ref<UTSJSONObject>({ accessToken: "token-value" })
const uploader = ref<XUploadMediaComponentPublicInstance | null>(null)
```

### 4.3 导航组件

#### x-tabs（标签页）

```html
<x-tabs v-model="activeId" :list="tabList" item-width="33.3%"></x-tabs>
```

```uts
import { TABS_ITEM_INFO } from "@/uni_modules/tmx-ui/interface.uts"
const activeId = ref<string>("1")
const tabList = ref<TABS_ITEM_INFO[]>([
	{ id: "1", title: "推荐" }, { id: "2", title: "热门" }, { id: "3", title: "最新" }
])
```

#### x-navbar（导航栏）

```html
<x-navbar title="页面标题"></x-navbar>
<x-navbar bg-color="#f5f5f5" active-bg-color="white" title="自定颜色"></x-navbar>
```

#### x-grid + x-grid-item（宫格）

```html
<x-grid :col="4" :show-border="true">
	<x-grid-item :order="0" icon="home-line" text="首页"></x-grid-item>
	<x-grid-item :order="1" icon="user-line" text="我的"></x-grid-item>
	<x-grid-item :order="2" icon="settings-line" text="设置"></x-grid-item>
	<x-grid-item :order="3" icon="more-line" text="更多"></x-grid-item>
</x-grid>
```

#### x-search（搜索）

```html
<x-search v-model="keyword" placeholder="搜索..." @search="onSearch"></x-search>
```

### 4.4 展示组件

#### x-cell（单元格）

```html
<x-cell title="标题" desc="描述"></x-cell>
<x-cell icon="sparkling-line" title="带图标" desc="描述"></x-cell>
<x-cell title="跳转" url="/pages/index/index"></x-cell>
```

#### x-badge（徽标）

```html
<x-badge :count="5">
	<x-button>消息</x-button>
</x-badge>
<x-badge :dot="true">
	<x-icon name="notification-line"></x-icon>
</x-badge>
```

#### x-card（卡片）

```html
<x-card image="https://example.com/img.jpg" title="标题"
	subtitle="副标题" content="内容描述"
	:btns="['付款','详细']">
</x-card>
```

#### x-collapse + x-collapse-item（折叠面板）

```html
<x-collapse v-model="active">
	<x-collapse-item title="标题1" name="1">内容1</x-collapse-item>
	<x-collapse-item title="标题2" name="2">内容2</x-collapse-item>
</x-collapse>
```

#### x-progress（进度条）

```html
<x-progress :value="60"></x-progress>
<x-progress :value="80" color="success" :show-text="true"></x-progress>
```

#### x-notice（通知栏）

```html
<x-notice text="这是一条滚动通知" color="warn"></x-notice>
```

#### x-swiper + x-swiper-item（轮播）

```html
<x-swiper :auto-play="3000" :list="bannerList">
	<x-swiper-item v-for="(item,index) in bannerList" :key="index">
		<x-image :src="item.url" width="100%" height="200"></x-image>
	</x-swiper-item>
</x-swiper>
```

#### x-loading（加载占位）

```html
<x-loading></x-loading>
<x-loading :vertical="false"></x-loading>
<x-loading icon="refresh-line" color="primary"></x-loading>
```

### 4.5 反馈组件

#### x-modal（模态弹窗）

```html
<x-modal v-model:show="showModal" title="提示" @confirm="onConfirm" @cancel="onCancel">
	<x-text>确定要执行此操作吗？</x-text>
</x-modal>

<!-- 插槽触发 -->
<x-modal v-model:show="show" @confirm="onConfirm">
	<template #trigger>
		<x-button :block="true">打开弹窗</x-button>
	</template>
	<x-input v-model="inputVal" placeholder="请输入"></x-input>
	<template #footer>
		<x-button @click="show=false" :block="true">自定底部</x-button>
	</template>
</x-modal>
```

#### x-drawer（抽屉）

```html
<x-drawer v-model:show="showDrawer" position="bottom" height="60%">
	<template #trigger>
		<x-button :block="true">打开抽屉</x-button>
	</template>
	<x-sheet>
		<x-text>抽屉内容</x-text>
	</x-sheet>
</x-drawer>
```

#### x-action-menu（操作菜单）

```html
<x-action-menu @item-click="itemClick" @cancel="onCancel" :list="menuList" title="选择操作">
	<template v-slot:trigger="{show}">
		<x-button :block="true">打开菜单</x-button>
	</template>
</x-action-menu>
```

```uts
import { XACTION_MENU_ITEM_INFO } from "@/uni_modules/tmx-ui/interface.uts"
const menuList = [
	{ title: '拍照', id: '1' },
	{ title: '从相册选择', id: '2' },
	{ title: '删除', id: '3', color: 'red' },
] as XACTION_MENU_ITEM_INFO[]
const itemClick = (index: number) => { console.log(index) }
```

#### x-overlay（遮罩）

```html
<!-- 变量控制 -->
<x-overlay v-model:show="show"
	custom-style="display:flex;align-items:center;justify-content:center;"
	customContentStyle="width:90%;">
	<x-sheet>
		<x-text>遮罩内容</x-text>
		<x-button @click="show=false" :block="true">关闭</x-button>
	</x-sheet>
</x-overlay>

<!-- 插槽触发 -->
<x-overlay :show-close="true" customContentStyle="width:64%;"
	custom-style="display:flex;align-items:center;justify-content:center;">
	<template v-slot:trigger="{show}">
		<x-cell icon="sparkling-line" title="点击打开遮罩"></x-cell>
	</template>
	<x-sheet>
		<x-text>遮罩内容</x-text>
	</x-sheet>
</x-overlay>
```

#### x-dropdown-menu + x-dropdown-item（下拉菜单）

```html
<x-dropdown-menu v-model="activeIndex" @change="onChange">
	<x-dropdown-item title="按销量" key-name="sales">
		<x-text>自定义筛选内容</x-text>
		<x-button @click="activeIndex=-1" :block="true">关闭</x-button>
	</x-dropdown-item>
	<x-dropdown-item title="综合" key-name="all">
		<x-text>内容高度自适应</x-text>
	</x-dropdown-item>
</x-dropdown-menu>
```

```uts
const activeIndex = ref(-1)
const onChange = (index: number, keyName: string, status: boolean) => {
	console.log(index, keyName, status)
}
```

#### x-popover（气泡菜单）

```html
<x-popover position="bl" :show-triangle="true">
	<x-button round="64" :iconBtn="true" icon="menu-line"></x-button>
	<template #menu>
		<view style="width:140px">
			<x-cell v-for="(item,index) in 4" :key="index" :card="false" :title="'菜单-'+item"
				:show-bottom-border="index!=3"></x-cell>
		</view>
	</template>
</x-popover>
```

position 取值：`bl`（底左）、`bc`（底中）、`br`（底右）、`tl`（上左）、`tc`（上中）、`tr`（上右）

#### x-float-button（浮球）

```html
<x-float-button @click="onClick">
	<view class="flex flex-center" style="width:100%;height:100%">
		<x-icon color="white" font-size="30" name="phone-fill"></x-icon>
	</view>
</x-float-button>
<x-float-button bg-color="success" :offset="[-2,-2]" :adsorption="false">
	<view class="flex flex-center" style="width:100%;height:100%">
		<x-icon color="white" font-size="30" name="add-line"></x-icon>
	</view>
</x-float-button>
```

---

## 5. 常用类型导入清单

从 `@/uni_modules/tmx-ui/interface.uts` 导入：

| 类型 | 用途 |
|------|------|
| `TABS_ITEM_INFO` | x-tabs 列表项 |
| `XACTION_MENU_ITEM_INFO` | x-action-menu 列表项 |
| `PICKER_ITEM_INFO` | x-picker 列表项 |
| `FORM_RULE` | x-form 校验规则 |
| `FORM_SUBMIT_RESULT` | x-form 提交结果 |
| `XUPLOADFILE_FILE_VALUE` | x-upload-media 文件值 |
| `XUPLOADFILE_FILE_INFO` | x-upload-media 文件信息 |
| `CHECKBOX_ITEM_INFO` | x-checkbox 项 |
| `RADIO_ITEM_INFO` | x-radio 项 |
| `TABBAR_ITEM_INFO` | x-tabbar 项 |

从 `@/uni_modules/tmx-ui/index.uts` 导入：

| 导出 | 用途 |
|------|------|
| `xStore` | 全局配置（`xStore.xConfig`） |
| `xDate` | 日期工具类 |
| `xRequest` | 网络请求封装 |
| `xColor` | 颜色工具 |

---

## 6. 页面开发 Checklist

- [ ] APP 端用 `scroll-view style="flex:1"` 包裹页面
- [ ] 微信端用 `page-meta` + `navigation-bar` 配置主题
- [ ] 优先用 tmx-ui 的 `x-` 组件构建 UI
- [ ] 布局微调用 `uvuePx.scss` 工具类（`flex-row`、`pa-12` 等）
- [ ] 类型从 `interface.uts` 导入，不在页面中重复定义公共类型
- [ ] 数据用 `ref<T>()`，类型显式标注
- [ ] API 请求通过 `pages/libs/api.uts` 封装调用
- [ ] 全局状态通过 `pages/libs/useUseStore.uts` 管理
- [ ] 底部留白防止内容被遮挡
