# 系统组件与 API

> 系统内置组件和 uni API 使用规范。
> 官方组件文档：https://doc.dcloud.net.cn/uni-app-x/component/
> 官方 API 文档：https://doc.dcloud.net.cn/uni-app-x/api/

---

## 1. 内置组件使用

### 常用组件

| 组件 | 用途 | 关键属性/事件 |
|------|------|--------------|
| `view` | 布局容器 | `ref`、`@click`、`@touchstart/move/end`、`@longpress`、`@transitionend` |
| `text` | 文本 | `selectable`、`:style` |
| `scroll-view` | 滚动容器 | `direction="horizontal/vertical"`、`@scroll`、`@scrolltolower`、`refresher-enabled`、`refresher-triggered` |
| `list-view` | 长列表 | `direction="vertical"`、`@scrolltolower`、`refresher-enabled`、`refresher-triggered` |
| `list-item` | 列表项 | 配合 `list-view` |
| `image` | 图片 | `src`、`mode` |
| `video` | 视频 | `src` |
| `input` | 输入框 | `type`、`v-model`、`@input`、`@focus`、`@blur` |
| `canvas` | 画布 | `id`、`canvas-id` |
| `picker-view` | 选择器 | `@change`、`value` |
| `picker-view-column` | 选择器列 | 配合 `picker-view` |

### scroll-view 下拉刷新

```html
<scroll-view
	direction="vertical"
	refresher-enabled
	:refresher-triggered="refreshing"
	@refresherpulling="onPulling"
	@refresherrefresh="onRefresh"
	@refresherrestore="onRestore"
	@refresherabort="onAbort"
	@scrolltolower="onLoadMore"
	@scroll="onScroll"
>
	<slot name="refresher"></slot>
	<!-- 内容 -->
</scroll-view>
```

### list-view 用法

```html
<list-view
	direction="vertical"
	refresher-enabled
	:refresher-triggered="refreshing"
	@scrolltolower="loadMore"
>
	<list-item v-for="(item, index) in list" :key="index">
		<!-- 内容 -->
	</list-item>
</list-view>
```

## 2. uni-app API

### 路由导航

```uts
uni.navigateTo({ url: '/pages/detail/detail?id=' + id })
uni.redirectTo({ url: '/pages/login/login' })
uni.switchTab({ url: '/pages/index/index' })
uni.navigateBack()
```

### 界面反馈

```uts
uni.showToast({ title: '成功', icon: 'success' })
uni.showModal({ title: '提示', content: '确定删除？' })
uni.showLoading({ title: '加载中...' })
uni.hideLoading()
```

### 存储

```uts
uni.setStorageSync('key', value)
const val = uni.getStorageSync('key')
```

### 系统信息

```uts
const sysInfo = uni.getSystemInfoSync()
const winInfo = uni.getWindowInfo()
// winInfo.windowWidth, winInfo.windowHeight, winInfo.windowTop
// winInfo.pixelRatio, winInfo.safeAreaInsets
```

### DOM 查询

```uts
// createSelectorQuery
uni.createSelectorQuery().in(this).select('.className').boundingClientRect().exec((res) => { ... })

// getElementById
const el = uni.getElementById(id)
el?.style?.setProperty('transform', 'translateX(100px)')

// getBoundingClientRectAsync（App 端）
element.getBoundingClientRectAsync((rect) => {
	// rect.left, rect.top, rect.width, rect.height
})
```

### 媒体

```uts
uni.chooseImage({ count: 1, success: (res) => { ... } })
uni.previewImage({ urls: imageList, current: index })
uni.uploadFile({ url, filePath, name, success: (res) => { ... } })
uni.downloadFile({ url, success: (res) => { ... } })
```

### 其他

```uts
uni.rpx2px(750)              // rpx 转 px
uni.loadFontFace({ ... })    // 加载字体
uni.makePhoneCall({ phoneNumber: '10086' })
```

## 3. 平台适配 — 弹层组件

### 模式

H5 使用 `<teleport>`，微信小程序使用 `<root-portal>`，通过条件编译切换：

```html
<!-- #ifdef H5 -->
<teleport :to="teleportTarget || teleportElH5" :disabled="!teleportTarget">
<!-- #endif -->

	<!-- 弹层内容 -->

<!-- #ifdef H5 -->
</teleport>
<!-- #endif -->

<!-- #ifdef MP-WEIXIN -->
<root-portal>
	<!-- 弹层内容 -->
</root-portal>
<!-- #endif -->
```

### teleport 目标

```uts
// #ifdef H5
const teleportElH5 = ref("uni-page")

function getTeleportTarget(): string {
	// 尝试 uni-page → uni-app → #app → body
}
// #endif
```

### 使用此模式的组件

`x-modal`、`x-action-modal`、`x-action-menu`、`x-drawer`、`x-overlay`、`x-dropdown-menu`、`x-popover`

## 4. 生命周期

### 全局 Mixin（xui.uts）

项目通过 Vue 插件 mixin 统一 emit 页面生命周期：

```uts
app.mixin({
	onPageScroll(e) { uni.$emit('onPageScroll', ...) },
	onResize()      { uni.$emit('onResize', ...) },
	onLoad(query)   { uni.$emit('onLoad', ...) },
	onHide()        { uni.$emit('onHide', ...) },
	onReady()       { uni.$emit('onReady', ...) },
	onShow()        { uni.$emit('onShow', ...) },
})
```

### 组件监听全局生命周期

```uts
onMounted(() => {
	uni.$on('onPageScroll', handleScroll)
	uni.$on('onResize', handleResize)
})
onBeforeUnmount(() => {
	uni.$off('onPageScroll', handleScroll)
	uni.$off('onResize', handleResize)
})
```

### Vue 生命周期

| 钩子 | 用途 |
|------|------|
| `onMounted` | 获取窗口信息、初始化 DOM、注册全局事件 |
| `onBeforeUnmount` | 清理定时器、动画、注销事件 |
| `onUpdated` | H5 中更新 teleport 目标 |

### 页面生命周期

| 钩子 | 用途 |
|------|------|
| `onLoad(query)` | 获取页面参数 |
| `onReady` | DOM 就绪后初始化 |
| `onShow` | 页面显示（从后台恢复等） |
| `onPageScroll(e)` | 页面滚动（吸顶、导航栏透明等） |
| `onPullDownRefresh` | 下拉刷新 |
| `onShareAppMessage` | 微信分享 |

## 5. 事件参数类型

```uts
const onClick = (evt: UniPointerEvent) => { ... }
const onTouch = (evt: UniTouchEvent) => { ... }
const onMove = (evt: TouchEvent) => { evt.preventDefault() }
```

### 阻止冒泡/默认

```uts
evt.stopPropagation()
evt.preventDefault()
```

```html
@click.stop=""
```

## 6. UniResizeObserver

```uts
const observer = new UniResizeObserver((entries) => {
	const entry = entries[0]
	// entry.contentRect.width, entry.contentRect.height
})
observer.observe(element)
// 清理
observer.disconnect()
```

## 7. 尺寸获取

### App 端

```uts
element.getBoundingClientRectAsync((rect) => {
	// rect.left, rect.top, rect.width, rect.height
})
```

### Web / 小程序端

```uts
uni.createSelectorQuery().in(this).select('.el').boundingClientRect().exec((res) => { ... })
```

### 跨平台

```uts
// #ifdef APP
element.getBoundingClientRectAsync(callback)
// #endif
// #ifdef WEB || MP
uni.createSelectorQuery().in(t).select(sel).boundingClientRect().exec(callback)
// #endif
```

## 8. UTS 原生插件

项目中使用的原生插件：

| 插件 | 用途 |
|------|------|
| `x-mlkit-scannig-u/s` | 扫码 |
| `x-facedetection` | 人脸检测 |
| `x-webview-u` | WebView |
| `x-modal-s`、`x-toast-s`、`x-loading-s` | 替代 uni.showModal/showToast/showLoading |
| `x-keyboardheightchange-s` | 键盘高度监听 |
| `x-sqlite-s` | SQLite |
| `x-camrea-u` | 相机 |
