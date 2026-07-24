# UTS 语法规范

> UTS 语法规范，适用于所有 .uts 和 .uvue 文件。
> 官方 UTS 文档：https://doc.dcloud.net.cn/uni-app-x/uts/

---

## 0. 严格类型声明（最重要）

> **UTS 是严格类型语言，除返回 `void` 外，所有数据、数组、函数都必须显式声明类型。不标注类型会编译报错。**

### ref / reactive 必须带泛型

```uts
const count = ref<number>(0)
const name = ref<string>('')
const list = ref<string[]>([])
const items = ref<ItemType[]>([])
const info = ref<UserInfo | null>(null)
const store = reactive({ token: '' } as StoreType)
```

### 函数 / 箭头函数必须标注返回类型（void 除外）

```uts
const getLabel = () : string => 'hello'
const calcSize = (n : number) : number => n * 2
function formatValue(v : string) : string { return v.trim() }

// void 可以省略
const handleClick = (e : UniPointerEvent) => { emits('click', e) }
```

### computed 必须标注返回类型

```uts
const _show = computed(() : boolean => props.show)
const _bgColor = computed(() : string => getDefaultColor(props.bgColor))
const _list = computed(() : ItemType[] => props.items.filter((it : ItemType) : boolean => it.visible))
```

### watch getter 必须标注返回类型

```uts
watch(() : boolean => props.disabled, () => { getNodes() })
watch(() : string => props.color, (val : string) => { update(val) })
watch(() : number => props.size, (newVal : number, oldVal : number) => { resize(newVal) })
```

### 数组方法回调必须标注返回类型

```uts
list.filter((item : ItemType) : boolean => item.active)
list.map((item : ItemType) : string => item.name)
list.forEach((item : ItemType) => { process(item) })  // void 可省略
list.find((item : ItemType) : boolean => item.id == targetId)
list.sort((a : ItemType, b : ItemType) : number => a.order - b.order)
```

### Vue reactive 在 UTS 中的注意事项

```uts
// 1. 模板中可直接使用 prop 名（不需要 computed 包装）
// 错误：const _disabled = computed(() : boolean => props.disabled)
// 正确：模板中直接写 disabled

// 2. reactive 容器中的 ref 会被 Vue 自动解包
// 存在 reactive 数组/对象中的 ref，赋值不加 .value
const map = reactive(new Map<string, any>())
map.set('key', { myRef: someRef })
// 读写 myRef：
// map.get('key').myRef = 'newValue'  ← 正确（Vue 代理自动 set ref.value）
// map.get('key').myRef.value = 'x'  ← 错误（myRef 已被解包为原始值）

// 3. provide 需要 reactive 值时，用 computed 内联
provide('xGridCol', computed(() : number => props.col))

// 4. 微信 MP 中 defineExpose 的方法无法通过 $callMethod 或直接属性调用
// 需要通过 provide/inject 共享 reactive ref 或使用 Options API
```

---

## 1. 类型定义

> **只能使用 `type`，禁止 `interface`。禁止内联类型和嵌套类型。必须平铺定义后引用。**

### 只能用 `type`

```uts
// 对象类型
export type NODE_INFO = {
	left : number,
	width : number,
	height : number,
	bottom : number,
	right : number,
	top : number
}

// 联合字面量类型（替代 enum）
export type NAVIGATE_TYPE = "navigate" | "redirect" | "switchTab" | "reLaunch" | "navigateBack"
export type xTweenStatus = 1 | 2 | 3 | 4 | 5 | 6

// 函数类型
export type CALLFUN_T_BOOLEAN = (val : any) => boolean
export type xTweenCallbackFunType = (x : number) => number
```

### 禁止内联类型

```uts
// 错误：函数参数/返回值中内联对象类型
const getInfo = () : { name : string, age : number } => { ... }

// 正确：先定义 type 再引用
type InfoResult = { name : string, age : number }
const getInfo = () : InfoResult => { ... }
```

### 禁止嵌套类型，必须平铺

```uts
// 错误：类型嵌套
type UserInfo = {
	name : string,
	address : { city : string, zip : string }
}

// 正确：拆分为独立 type 后引用
type AddressInfo = {
	city : string,
	zip : string
}
type UserInfo = {
	name : string,
	address : AddressInfo
}
```

### 联合类型规则

联合类型只能是**字面量联合**或 **Type | null**。不支持复杂联合，不可用 `===`。

```uts
// 正确：字面量联合
type SkinType = 'default' | 'outline' | 'text' | 'thin'
type StatusCode = 0 | 1 | 2 | 3
type SizeType = 'mini' | 'small' | 'normal' | 'large'

// 正确：Type | null（null 可以和任何类型联合）
type MaybeUser = UserInfo | null
const info = ref<UserInfo | null>(null)
const task = null as RequestTask | null

// 正确：基础类型联合
type ValueType = string | number

// 错误：复杂联合类型（多个自定义类型联合）
type Result = SuccessResult | ErrorResult | PendingResult  // 不可以

// 错误：用 === 比较（UTS 中用 ==）
if (props.skin === 'default') { ... }  // 错误
if (props.skin == 'default') { ... }   // 正确
```

### 函数参数默认值规则（重要）

> **核心规则**：UTS 中**箭头函数 / 匿名函数禁止为参数声明默认值**，只有使用 `function` 关键字声明的命名函数支持形参默认值。
>
> 编译期报错：`error: Anonymous functions cannot specify default values for their parameters.`

```uts
// ❌ 错误：箭头函数带参数默认值（即使默认值是 null / true / 字面量都不行）
const fitView = (shrinkOnly : boolean = true) => { ... }
const doWork = (callback : (() => void) | null = null) => { ... }
const request = (opts : xRequestOptions | null = null) : Promise<Result> => { ... }

// ✅ 正确：用 function 关键字声明（支持默认参数）
function fitView(shrinkOnly : boolean = true) { ... }
function doWork(callback : (() => void) | null = null) { ... }
function request(opts : xRequestOptions | null = null) : Promise<Result> { ... }

// ✅ 正确：箭头函数如需"可选参数"效果，不写默认值，由调用方显式传 null / 传值
const fitView = (shrinkOnly : boolean) => { ... }   // 调用: fitView(true)
const doWork = (callback : (() => void) | null) => { ... }  // 调用: doWork(null)
```

#### 在 `withDefaults(defineProps<T>(), { ... })` 中

`defineProps` 的 default 是"工厂函数"，不是"有默认参数的函数"，所以可以继续用箭头函数：

```uts
// ✅ 工厂函数（无参、只返回值）可以是箭头函数
const props = withDefaults(defineProps<PropsType>(), {
	list: () : string[] => [] as string[],             // 返回类型的箭头函数 OK
	content: () : SNACKBAR_INFO => ({                   // 工厂函数 OK
		id: -1, content: ""
	} as SNACKBAR_INFO),
	autoFit: true                                       // 字面量直接写
})
```

#### 类方法允许默认参数

类方法（`class` 内的成员函数）可以正常声明默认参数：

```uts
export class OrgChartRenderer {
	// ✅ 类方法支持默认参数
	public calculateFitScale(shrinkOnly : boolean = true) : number { ... }
}
```

#### 速查表

| 声明形式 | 是否支持参数默认值 |
|---|---|
| `function name(x : T = v) { ... }` | ✅ 支持 |
| `class.method(x : T = v) { ... }` | ✅ 支持 |
| `const fn = (x : T = v) => { ... }` | ❌ **禁止**（编译报错） |
| `setTimeout(() => { ... }, 100)` 内的箭头函数 | ❌ 禁止声明默认参数 |

### 函数标识符不能作为回调直接传递（重要）

> **核心规则**：UTS 里把**函数名**当作"函数引用"传给接受 `Function0<Unit>` / `() => void` 等函数类型的参数（比如 `setTimeout` / 数组 `.forEach` 等），编译器会把它解读为**即时调用**而不是引用，导致类型不匹配报错。
>
> 必须用**箭头函数包一层**手动调用。

编译期报错（示例）：
```
error: 参数类型不匹配：实际类型为 'Unit'，预期类型为 'Function0<Unit>'
error: Function invocation 'tick()' expected.
```

```uts
function tick() { ... }

// ❌ 错误：UTS 会把 tick 解读为即时调用 tick()，返回 Unit，类型不匹配
setTimeout(tick, 16)
arr.forEach(handler)                      // handler 同样被当成立即调用
render.value!.onListen(onRenderClick)      // 如果 onListen 参数是函数类型也会踩坑

// ✅ 正确：用箭头函数包一层
setTimeout(() => { tick() }, 16)
arr.forEach((item : T) => { handler(item) })
render.value!.onListen((item : XXX) => { onRenderClick(item) })

// ✅ 另一种方式：直接写成 const 箭头函数变量，不用 function 声明
const tick = () => { ... }
setTimeout(tick, 16)    // 这样 tick 本身就是函数引用，能直接传
```

**踩坑点**：
- `function foo()` 形式声明的**命名函数**不能作为值传递 → 必须包一层箭头
- `const foo = () => {...}` 形式的**箭头函数变量**可以直接传（因为它本身就是"值"）
- 递归场景（函数内部需要引用自己调度下一次）只能用 `function` 声明然后包箭头，因为 `const` 自引用会报"未定义"

```uts
// 递归场景的正确写法
function tick() {
	// ... 业务逻辑
	setTimeout(() => { tick() }, 16)  // ✅ 箭头包一层
}
tick()
```

### 局部 type 写在 import 之后、其他代码之前

```uts
// 1. import
import { ref } from "vue"
// 2. 局部 type（紧跟 import）
type MyResult = { ok : boolean, msg : string }
// 3. 其他代码
const result = ref<MyResult | null>(null)
```

### 使用 `class`（有状态/实例的实现）

```uts
export class xDate { ... }
export class useTool {
	public static instanceReal: useTool | null = null;
	public static getInstance(): useTool { ... }
}
```

### 命名规范

- 全大写下划线：`XCONFIG`、`NODE_INFO`、`XGRID_ITEM_INFO`、`FORM_RULE`
- 小驼峰 + 后缀：`xDateDayInfoType`、`xRequestOptions`
- 组件 Props：`xGridPropsType`、`xModalPropsType`、`XTabsProps`

### 可选属性

```uts
export type xRequestOptions = {
	url: string,
	method?: xRequestMethond,
	header?: UTSJSONObject,
	timeout?: number
}
```

## 2. 导入导出

### 导入路径

```uts
// 相对路径
import { getDefaultColor } from "../../core/util/xCoreColorUtil.uts"

// @/ 别名
import { TABBAR_ITEM_INFO } from "@/uni_modules/tmx-ui/interface.uts"
```

**注意：** 导入 `.uts` 文件时需要写完整扩展名 `.uts`。

### 导入方式

```uts
// 命名导入
import { xConfig, setConfig } from "../../config/xConfig.uts"

// 命名空间导入
import * as xColor from "./core/util/xCoreColorUtil.uts"

// 类型与值一起导入
import { TABBAR_ITEM_INFO, XCONFIG } from "../interface.uts"
```

### 导出方式

```uts
// 内联导出
export type XCONFIG = { ... }
export const xConfig = reactive({ ... } as XCONFIG)
export const getThemePrimary = (): string => { ... }

// 先定义后批量导出
function rpx2px(n: number): number { ... }
function checkIsCssUnit(str: string, unit: string): string { ... }
export { rpx2px, checkIsCssUnit }

// 默认导出
export default xui

// 聚合入口（index.uts）
import * as xStorex from "./config/xConfig.uts"
import { xTween } from "./core/util/xTween.uts"
export { xTween, xStorex }
export class xStore { static xConfig = ...; static setConfig = ...; }
```

## 3. UTS 特有语法

### UTSJSONObject

UTS 中替代普通对象的类型，通过 getter/setter 访问：

```uts
let rgb = hexToRgb(sColor)  // 返回 UTSJSONObject
rgb.getNumber("r")
rgb.set('l', 50)
```

### 类型断言

```uts
} as XCONFIG
} as UTSJSONObject
m as 'dark' | 'auto' | 'light'
[] as XACTION_MENU_ITEM_INFO[]
```

### 模板 v-model 绑定复杂对象属性时必须加类型断言

当 ref 是复杂对象类型时，v-model 绑定其属性需要 `as` 类型断言，否则编译报错：

```uvue
<script lang="ts" setup>
	type FormData = {
		name : string,
		age : number,
		score : number
	}
	const form = ref<FormData>({ name: '', age: 0, score: 0 } as FormData)
</script>
<template>
	<!-- 正确：v-model 加类型断言 -->
	<input v-model="(form.name as string)" />
	<x-input v-model="(form.age as number)" />
	<slider v-model="(form.score as number)" />

	<!-- 错误：缺少类型断言 -->
	<input v-model="form.name" />
</template>
```

### 非空断言

```uts
cfg.getString('color')!
cfg.getNumber('designSize')!
xani.value!.stop()
form.value!.submit()
```

### 容器（Map / Array / Set）泛型空安全

> **核心规则**：`Map<K, V>` / `Array<V>` / `Set<V>` 的 `V` 如果是**非空类型**，则调用 `set`/`push`/`add` 时传入的值也必须是非空。若表达式可能为 null（如函数声明返回 `T | null`），**必须先判空**再传入。
>
> 编译期报错（示例）：`error: 参数类型不匹配：实际类型为 'Function1<..., Number>?'，预期类型为 'Function1<..., Number>'`

```uts
// 场景：bezier(x1, y1, x2, y2) 在参数越界时会返回 null
function bezier(x1 : number, y1 : number, x2 : number, y2 : number) : ((x : number) => number) | null { ... }

// ❌ 错误：fn 类型是 `Function | null`，不能直接塞进非空 Map
const cache : Map<string, (x : number) => number> = new Map()
const fn = bezier(0.42, 0.38, 0.15, 0.93)
cache.set(key, fn)                  // ← 编译报错：实际类型 `Function?`

// ✅ 正确：判空 + 非空断言
const fn = bezier(0.42, 0.38, 0.15, 0.93)
if (fn != null) {
    cache.set(key, fn!)
    return fn!
}
// 处理 null 分支，例如 fallback 或返回 null

// ✅ 正确：声明 V 为可空类型，set 可空值就不报错
const cache : Map<string, ((x : number) => number) | null> = new Map()
cache.set(key, fn)                  // OK
```

同理适用 Array 与 Set：

```uts
// ❌ 错误：push 可空元素到非空数组
const list = [] as string[]
const maybe : string | null = getName()
list.push(maybe)                    // ← 编译报错

// ✅ 正确：判空后 push 或声明数组允许 null
if (maybe != null) list.push(maybe!)
// 或
const list = [] as Array<string | null>
list.push(maybe)
```

**边界场景**：即使你在判空分支内已经能逻辑推导出非空，编译器通常仍要求显式 `!` 断言才能通过，尤其当：
- 可空值来自**外部函数返回值**（编译器不会跨函数边界缩窄）
- 可空值穿过 `await` / `setTimeout` 等异步边界

### reactive 与类型

```uts
export const xConfig = reactive({
	color: "#0091FF",
	dark: 'auto',
	unit: 'px',
	// ...
} as XCONFIG)
```

### computed / watch / 回调示例（参见第 0 节）

```uts
const _bgColor = computed((): string => getDefaultColor(props.bgColor))
const _col = computed((): number => props.col)
const _show = computed((): boolean => props.show)
const _info = computed((): XGRID_ITEM_INFO => { return { ... } as XGRID_ITEM_INFO })
watch((): boolean => props.disabled, () => { getNodes() })
```

### definePlugin

```uts
const xui = definePlugin({
	install(app: VueApp, config: Tmui4xOptions | null) {
		setConfig(config)
		app.config.globalProperties.$i18n = xConfig.i18n
		app.mixin({ ... })
	}
})
export default xui
```

### 泛型

```uts
function splitArray<T>(target: Array<T>, value: number): Array<Array<T>>
```

### Map 使用

```uts
const colors = new Map<string, string>([
	['primary', '#0088FF'],
	['success', '#34C759'],
])
xConfig.theme = new Map<string, string>([])
const styleMap = new Map<string, any>()
```

## 4. 条件编译

### Script 中

```uts
// #ifdef APP
ix = Math.floor(...).toString().substring(0, length as Int);
// #endif

// #ifdef H5
r = getLayoutRatio() * n
// #endif

// #ifdef APP-ANDROID && uniVersion >=4.31
maxmindiff = maxmindiff.toDouble()
// #endif
```

### 平台原生导入

```uts
// #ifdef APP-ANDROID
import Context from 'android.content.Context'
import LinearLayout from 'android.widget.LinearLayout'
// #endif
```

## 5. 工具函数规范

### 防抖 / 节流

```uts
// 使用 useTool 类
useTool.debounce('key', () => { ... }, 300)
useTool.throttle('key', () => { ... }, 300)

// 使用函数式
const debouncedFn = debounce(func, wait, immediate)
```

### ID 生成

```uts
import { getUid } from "../../core/util/xCoreUtil.uts"
const id = ref<string>("xModal" + getUid())
```

### 数组工具

```uts
splitArray<T>(target, chunkSize)       // 按大小分组
splitArrayByGroup<T>(target, groupNum) // 按组数分组
toFillMarginAr(val: number[]): number[] // 边距填充
```

## 6. 全局配置结构（xConfig）

| 分类 | 配置项 |
|------|--------|
| 主题 | `color`、`dark`、`theme`、`fontColor`、`fontDarkColor` |
| 尺寸 | `designSize`、`maximumCalculatedSize`、`unit`、`fontScale`、`fontSize` |
| 圆角 | `inputRadius`、`buttonRadius`、`tagRadius`、`cellRadius`、`modalRadius`、`drawerRadius` |
| 暗黑 | `sheetDarkColor`、`inputDarkColor`、`borderDarkColor` |
| 导航栏 | `navigationBarTextStyleDark/Light`、`navigationBarBackgroundColorDark/Light` |
| 动画 | `animationFun` |
| 国际化 | `i18n`、`language` |

### 配置 API

```uts
setConfig(configs: Tmui4xOptions | null)
setThemePrimary(scolor: string): string
getThemePrimary(): string
setDarkModel(value: 'dark' | 'auto' | 'light')
getDarkModel(): 'dark' | 'auto' | 'light'
getI18n(): Tmui4xI18nTml
```
