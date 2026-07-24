# Vue 组件规范

> Vue 组件开发规范，适用于 tmx-ui 组件和自定义组件。
> 官方 Vue 兼容性文档：https://doc.dcloud.net.cn/uni-app-x/vue/

---

## 1. 组件注册

通过 easycom 自动注册，组件路径：

```
uni_modules/tmx-ui/components/x-xxx/x-xxx.uvue
```

页面中直接使用 `<x-xxx>`（kebab-case）。

## 2. defineOptions

所有组件必须设置组件名，放在 script 顶部：

```uts
defineOptions({ name: "xModal" })
```

命名：`x` 前缀 + PascalCase。

## 3. Props 定义

### 步骤

1. 定义 Props 类型（`type xXxxPropsType`）
2. 使用 `withDefaults(defineProps<T>(), { ... })` 设置默认值

```uts
type xModalPropsType = {
	/** 自定义样式 */
	customStyle: string,
	/** 标题 */
	title: string,
	/** 是否显示底部 */
	showFooter: boolean,
	/** 关闭前回调 */
	beforeClose: callbackType,
}

const props = withDefaults(defineProps<xModalPropsType>(), {
	customStyle: "",
	title: "",
	showFooter: true,
	beforeClose: (): Promise<boolean> => Promise.resolve(true),
})
```

### 默认值规则

| 类型 | 默认值写法 |
|------|-----------|
| string | `""` 或 `'white'` |
| number | `0` 或 `300` |
| boolean | `true` / `false` |
| Array | `(): T[] => [] as T[]` |
| Map | `(): Map<K, V> => new Map<K, V>()` |
| Function | `(): Promise<boolean> => Promise.resolve(true)` |

### 可选属性

```uts
type XTabsProps = {
	round?: string,
	width?: string,
	modelValue?: string | number,
}
```

### 导出 Props 类型

需要外部引用时导出：

```uts
export type xFormPropsType = { ... }
```

## 4. Emits 定义

### 类型化写法（首选）

```uts
const emit = defineEmits<{
	click: []
	close: []
	'update:show': [value: boolean]
	'item-click': [index: number, item: any]
}>()
```

### 数组写法

```uts
const emits = defineEmits(['cancel', 'click', 'close', 'update:show', 'item-click'])
```

### v-model 事件

```uts
// 单个 v-model
const emit = defineEmits(['update:modelValue'])

// 命名 v-model
const emit = defineEmits(['update:show'])
```

## 5. Slots 定义

### defineSlots（带作用域）

```uts
defineSlots<{
	trigger(props: { show: boolean }): any
}>()

defineSlots<{
	default(props: { active: boolean; item: TABS_ITEM }): any
}>()
```

### 模板中声明

```html
<!-- 默认插槽 -->
<slot></slot>

<!-- 具名插槽 -->
<slot name="trigger"></slot>
<slot name="title"></slot>
<slot name="footer"></slot>

<!-- 作用域插槽 -->
<slot name="trigger" :show="_show"></slot>
<slot :item="item" :active="nowActiveId == item.id"></slot>
```

### 插槽 JSDoc

```html
<!--
@slot 标签触发显示遮罩
@prop {Boolean} show - 当前是否已显示
-->
<slot name="trigger" :show="_show"></slot>
```

### 页面使用插槽

```html
<x-action-menu :list="list">
	<template #trigger="{ show }">
		<x-button :block="true">打开菜单</x-button>
	</template>
</x-action-menu>

<template v-slot:default="{ item, active }">
	<x-text :color="active ? 'white' : 'primary'">{{ item.title }}</x-text>
</template>
```

## 6. defineExpose

### 弹层类

```uts
defineExpose({
	open: () => showAlert(),
	close: () => closeAlert()
})
```

### 表单类

```uts
defineExpose({
	id: "xFormParent-" + getUid(),
	valid(keys: string[]): FORM_SUBMIT_RESULT { ... },
	submit(): FORM_SUBMIT_RESULT { ... },
	clearValid() { ... },
})
```

### 页面调用

```uts
const form = ref<XFormComponentPublicInstance | null>(null)
form.value!.submit()
form.value!.clearValid()
```

## 7. 响应式数据

### ref（主要方式）

```uts
const elementRef = ref<UniElement | null>(null)
const showOverlay = ref<boolean>(false)
const id = ref<string>("xModal" + getUid())
const status = ref<string>("")
```

项目中基本都用 `ref`，极少使用 `reactive`（`reactive` 仅用于全局配置如 `xConfig`）。

## 8. computed

**必须标注返回类型：**

```uts
const _title = computed((): string => {
	if (props.title == '') return i18n.t("tmui4x.modal.title")
	return props.title
})

const _round = computed((): string => {
	if (props.round == "") return checkIsCssUnit(xConfig.modalRadius, xConfig.unit)
	return checkIsCssUnit(props.round, xConfig.unit)
})
```

## 9. watch

使用 getter 监听 props（不要直接监听 props 对象）：

```uts
watch((): boolean => props.show, (newVal: boolean) => {
	if (newVal) showAlert()
	else closeAlert()
})

watch((): any => props.modelValue, (newValue: any) => {
	// 处理变更
}, { deep: true })
```

## 10. provide / inject

### provide（父组件）

值为 `computed` 以保持响应式：

```uts
provide('XFORMITEM_TOP', computed(() => top.value))
provide('XFORMITEM_LABEL_WIDTH', computed(() => props.labelWidth))
provide('xGridCol', _col)
provide('xGridHeight', _itemHeight)
```

### inject（子组件 — Composition API）

```uts
const col = inject<Ref<number>>('xGridCol')
```

### inject（子组件 — Options API）

```uts
inject: {
	XFORMITEM_TOP: { type: Number, default: 0 },
	XFORMITEM_LABEL_WIDTH: { type: String, default: '100' },
}
```

## 11. 模板指令

### v-if / v-show

```html
<view v-if="showOverlay"></view>
<view v-show="_visible"></view>
```

### v-for

```html
<view v-for="(item, index) in _list" :key="index" @click="itemClick(index)">
```

### 条件编译

```html
<!-- #ifdef H5 -->
<teleport :to="teleportTarget">...</teleport>
<!-- #endif -->

<!-- #ifdef MP-WEIXIN -->
<root-portal>...</root-portal>
<!-- #endif -->

<!-- #ifndef APP-HARMONY -->
<view class="custom-refresher">...</view>
<!-- #endif -->
```

### 事件绑定

```html
@click="onClick"
@click.stop=""
@touchstart="onTouchStart"
@touchmove="onTouchMove"
@touchend="onTouchEnd"
@transitionend="onEnd"
@scroll="onScroll"
@scrolltolower="onLoadMore"
```

## 12. 组件通信模式总结

| 模式 | 方向 | 场景 |
|------|------|------|
| Props | 父→子 | 传递配置、数据 |
| Emits | 子→父 | 事件通知 |
| v-model | 双向 | 表单值、弹层显隐 |
| provide/inject | 祖先→后代 | 跨层级配置共享（如 form→form-item、grid→grid-item） |
| ref + expose | 父→子 | 命令式调用（open/close/submit） |
| uni.$on/$emit | 全局 | 页面生命周期事件广播 |

## 13. 函数声明顺序（重要）

> **UTS 要求：被调用的函数、变量、类型必须在调用处之前定义，不存在变量提升。**

`<script setup>` 中代码自上而下执行：
- 函数 A 调用函数 B → B 必须写在 A **之前**
- `computed`、`watch`、生命周期中引用的函数 → 必须在它们之前定义
- `ref` / `let` 声明的变量 → 必须在被引用之前声明

```uts
// 正确：底层工具 → 业务函数 → 组合函数 → computed → 生命周期
const formatValue = (v : string) : string => checkIsCssUnit(v, xConfig.unit)
const calcSize = () : number => { /* 可调用 formatValue */ }
const _size = computed(() : number => calcSize())

// 错误：调用在定义之前
const _size = computed(() : number => calcSize())  // 报错：calcSize 未定义
const calcSize = () : number => { ... }
```

## 14. 编码风格补充

- **组件文件**：使用 `<script lang="ts" setup>`（也可用 `lang="uts"`）
- **Style**：`<style scoped>`，必要时 `<style lang="scss">`
- **注释**：组件头部使用 JSDoc 记录 `@name`、`@description`、`@page`、`@category`、`@constant`（平台兼容表）
- **i18n**：`xConfig.i18n.t("tmui4x.modal.title")` 获取翻译文本
- **ID 生成**：`getUid()` 确保唯一性
