---
name: tmui4x-dev
description: 使用 UVue、UTS、CSS 进行本项目所有前端开发的通用编码规范。适用于三类场景：(1) 在 uni_modules/tmx-ui/components/ 下新增或修改 x- 前缀的 tmx-ui 组件；(2) 在 pages/ 目录下编写 .uvue 页面（使用 tmx-ui 组件 + 官方组件）；(3) 在 pages/ 或任意目录下创建自定义 .uvue 组件（非 tmx-ui）。涵盖 UTS 严格类型系统、Vue 组合式 API (script setup)、CSS Flexbox 布局、暗黑模式、多端条件编译、性能优化等规范。编写任何 .uvue 文件时均应使用此 skill。
---

# TMUI4X 开发规范

使用 UVue + UTS + CSS 进行三类开发，规范分开遵循：

| 场景 | 位置 | 说明 |
|---|---|---|
| tmx-ui 组件开发 | `uni_modules/tmx-ui/components/x-xxx/` | 开发/维护 tmx-ui 组件库 |
| 页面开发 | `pages/` | 使用 tmx-ui + 官方组件写业务页面 |
| 自定义组件开发 | `pages/`、项目任意目录 | 非 tmx-ui 的 .uvue 组件（业务封装） |

---

## 元规则：错误自进化（强制）

> **AI 必须遵守的学习闭环**：当用户或编译器指出代码存在**语法错误 / 类型错误 / 平台兼容性错误**，且该错误属于 UTS / UVue / 本项目特有规则范畴时，在修复代码后，必须**立即**把该规则归纳到本 skill 的文档中。这是本 skill 的核心进化机制，不执行视为未完成任务。

### 执行步骤

1. **修复当前出错的代码**（首要任务，不能仅改文档不改代码）
2. **定位规则归属文档**（按主题选择落地位置）：
   - UTS 语法/类型/语言特性 → `uts-patterns.md`
   - Vue 组合式 API / script setup → `vue-conventions.md`
   - CSS / 布局 / 样式 → `css-system.md`
   - 官方组件 / uni API → `system-components.md`
   - 页面结构 / 分包 / Store → `page-dev-guide.md`
   - 跨领域或纲领性规则 → `SKILL.md`
3. **搜索已有规则**（`rg '相关关键字' .cursor/skills/tmui4x-dev/`）：
   - 若已有规则但**措辞不准确/示例错误**：更新措辞、修正示例、补充反例
   - 若完全缺失：新增一节，写明"错误信息 / 错误示例 / 正确示例 / 速查表"
4. **编写格式要求**：
   - 必须给出**报错原文**（让未来搜索可命中）
   - 必须给出 **❌ 错误示例** 与 **✅ 正确示例** 对照
   - 尽量补充**速查表**或**适用边界**（哪些场景触发、哪些场景不触发）
5. **不要**只在回答里口述规则，必须落盘到文档
6. **不要**创建新的散落 md 文件，优先改已有文档

### 典型触发场景

| 用户话术 | 应识别为需归纳的错误 |
|---|---|
| "编译失败 / 编译报错" + 代码片段 | ✅ 归纳 |
| "XX 不支持 / 不能这样写 / 要改成 YY" | ✅ 归纳 |
| "这个错误你经常犯 / 要写到 skill / 记住下次别这样" | ✅ 归纳（用户显式要求） |
| "换个方案 / 优化一下" | ❌ 非语法错误，不归纳 |
| "加个功能 / 改下样式" | ❌ 需求变更，不归纳 |

### 示例：本次修正过程

- **错误信息**：`error: Anonymous functions cannot specify default values for their parameters.`
- **错误代码**：`const fitView = (shrinkOnly : boolean = true) => { ... }`
- **修正代码**：`function fitView(shrinkOnly : boolean = true) { ... }`
- **归纳到**：`uts-patterns.md` 的「函数参数默认值规则」节已更新为精确措辞 + 速查表

## 项目结构

```
tmui4x/
├── pages/                          # 页面（主包 + 分包）
├── uni_modules/tmx-ui/
│   ├── components/x-xxx/x-xxx.uvue # 组件（easycom 自动注册）
│   ├── core/util/                   # 工具函数（.uts）
│   ├── config/xConfig.uts           # 全局配置
│   ├── interface.uts                # 类型定义
│   ├── scss/uvue.scss               # 工具类（rpx）— 仅页面使用
│   ├── scss/uvuePx.scss             # 工具类（px）— 仅页面使用
│   └── index.uts                    # 主入口
├── scripts/cssPlus/                 # CSS 按需提取插件
└── manifest.json                    # uni-app-x 配置
```

---

## 一、tmx-ui 组件开发规范

> 适用范围：`uni_modules/tmx-ui/components/` 下的所有 `x-` 前缀组件

### 组件模板

```uvue
<script lang="ts" setup>
	import { computed } from "vue"
	import { getDefaultColor } from "../../core/util/xCoreColorUtil.uts"
	import { checkIsCssUnit } from "../../core/util/xCoreUtil.uts"
	import { xConfig } from "../../config/xConfig.uts"

	/**
	 * @name 组件名 xExample
	 * @description 组件描述
	 * @page /pages/index/example
	 * @category 分类
	 * @constant 平台兼容
	 *	| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
		| --- | --- | --- | --- | --- | --- | --- | --- |
		| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |
	 */
	defineOptions({ name: "xExample" })

	type xExamplePropsType = {
		/** 背景颜色 */
		bgColor: string,
		/** 圆角 */
		round: string,
	}

	const props = withDefaults(defineProps<xExamplePropsType>(), {
		bgColor: 'white',
		round: '0',
	})

	const emits = defineEmits<{
		click: [e: UniPointerEvent]
	}>()

	const _bgColor = computed((): string => {
		if (xConfig.dark == 'dark') {
			return getDefaultColor(xConfig.sheetDarkColor)
		}
		return getDefaultColor(props.bgColor)
	})
	const _round = computed((): string => checkIsCssUnit(props.round, xConfig.unit))
</script>
<template>
	<view :style="{ backgroundColor: _bgColor, borderRadius: _round }">
		<slot></slot>
	</view>
</template>
<style scoped>
</style>
```

### 组件编码约定

| 项目 | 约定 |
|------|------|
| 组件命名 | `defineOptions({ name: "xXxx" })`，PascalCase，`x` 前缀 |
| Props 类型 | `type xXxxPropsType = { ... }` + `withDefaults(defineProps<T>(), {...})` |
| 类型标注 | **必须**显式标注（见「UTS 类型系统规则」）：`ref<T>()`、`computed((): T =>`、`watch((): T =>`、函数返回类型（void 除外） |
| 类型定义 | 只能用 `type`（禁止 `interface`），禁止内联/嵌套类型，必须平铺定义 |
| 联合类型 | 只允许字面量联合（`'a' \| 'b'`）或 `Type \| null`，不支持复杂联合 |
| 比较运算 | 用 `==` 不用 `===`（UTS 不支持 `===`） |
| v-model | 复杂对象属性需加类型断言：`v-model="(form.name as string)"` |
| 颜色处理 | 始终用 `getDefaultColor()` 包裹 |
| 单位处理 | 始终用 `checkIsCssUnit(value, xConfig.unit)` |
| 暗黑模式 | `xConfig.dark == 'dark'` 判断，优先组件 dark 属性，回退全局配置 |
| 条件编译 | `<!-- #ifdef H5 -->`、`// #ifdef APP`、`/* #ifdef WEB */` |
| 弹层组件 | H5 用 `<teleport>`，微信用 `<root-portal>` |

### 组件性能优化规则（重要）

> 以下规则基于对 x-button、x-tag、x-sheet、x-grid、x-grid-item、x-row、x-col 等高频组件的优化经验。
> 每个 `ref` / `computed` 都有响应式依赖追踪的开销，高频组件（一个页面可能几十个实例）必须最小化响应式原语数量。

#### 1. 禁止创建无意义的 computed 包装器

直接返回 `props.xxx` 的 computed 是纯开销。模板中可以直接使用 prop 名。

```uvue
<!-- 错误：无意义的 computed -->
<script lang="ts" setup>
	const _disabled = computed(() : boolean => props.disabled)
	const _icon = computed(() : string => props.icon)
	const _loading = computed(() : boolean => props.loading)
</script>
<template>
	<view v-if="_loading">...</view>
</template>

<!-- 正确：模板直接用 prop 名 -->
<template>
	<view v-if="loading">...</view>
</template>
```

**例外**：如果 prop 值需要 provide 给子组件，provide 需要 reactive 值，此时 computed 是必要的，但应内联到 provide 中：

```uvue
<!-- 推荐：内联到 provide -->
provide('xGridCol', computed(() : number => props.col))

<!-- 不推荐：多余的命名变量 -->
const _col = computed(() : number => props.col)
provide('xGridCol', _col)
```

#### 2. 用 computed 替代 ref + watch + customStyles 模式

当颜色/样式依赖多个 props 和全局配置时，**禁止**使用 refs + watch + 手动函数的模式。用 computed 让 Vue 自动追踪依赖。

```uvue
<!-- 错误：ref + watch + 手动触发 -->
<script lang="ts" setup>
	const _borderColor = ref<string>('...')
	const _bgColor = ref<string>('...')
	const _fontColor = ref<string>('...')

	function customStyles(hover : boolean) {
		_borderColor.value = ...
		_bgColor.value = ...
		_fontColor.value = ...
	}

	watch([() : any => props.color, () : any => props.skin, () : any => xConfig.dark], () => {
		customStyles(false)
	})
	onMounted(() => { customStyles(false) })
</script>

<!-- 正确：单个 computed 自动追踪 -->
<script lang="ts" setup>
	type _ColorResult = { border : string, background : string, fontColor : string }

	const _resolvedColors = computed(() : _ColorResult => {
		let hover = _isHover.value  // hover 也作为依赖
		// ... 颜色计算逻辑 ...
		return { border, background, fontColor } as _ColorResult
	})
</script>
```

优势：消除 N 个 ref + 1 个 watch + 1 个 onMounted + 1 个函数 → 1 个 computed。

#### 3. computed 必须是纯函数，禁止副作用

```uvue
<!-- 错误：computed 中修改其他 ref -->
const _disabled = computed(() : boolean => {
	if (props.disabled) { _opacity.value = '0.7' }  // 副作用！
	return props.disabled
})

<!-- 正确：opacity 在同一个 computed 中一起计算 -->
const _resolvedStyles = computed(() : StyleResult => {
	let opacity = (props.disabled || props.loading) ? '0.7' : '1'
	return { ..., opacity } as StyleResult
})
```

#### 4. 移除所有未使用的 import

每个 import 都有模块解析和初始化开销。布局组件（x-row、x-col）经常有未使用的 `getUid`、`getDefaultColor` 等 import，必须清理。

#### 5. 合并相似的 hover/normal 颜色 computed

不要为 normal 和 hover 各创建一个 computed。合并为一个依赖 `_isHover` ref 的 computed，hover 时才执行高开销的颜色变换（如 `colorAddDeepen`）：

```uvue
<!-- 错误：两个 computed，hover 颜色总是预计算 -->
const _bgColor = computed(() : string => ...)
const _hoverBgColor = computed(() : string => colorAddDeepen(...))  // 总是执行

<!-- 正确：一个 computed，条件执行 -->
const _currentBgColor = computed(() : string => {
	if (isHover.value && (props.isLink || props.url != '')) {
		return colorAddDeepen(...)  // 仅 hover 时执行
	}
	return ...
})
```

#### 6. 微信 MP 跨组件通信限制

微信 MP 中 `<script setup>` + `defineExpose` 暴露的方法，**无法**通过 `$callMethod` 或直接属性访问调用（Options API 的 `methods` 则正常）。解决方案：

- **方案 A**：子组件将自身的 reactive ref 通过 provide/inject 或 CHILDREN_INFO 传递给父组件，父组件直接修改 ref 值（x-drag 模式）
- **方案 B**：使用 Options API（`export default`）的组件可正常跨组件调用方法（x-form 模式）
- 注意 Vue reactive 代理会**自动解包嵌套 ref**：存储在 reactive 容器中的 ref 通过 `refs.xxx = value`（不加 `.value`）赋值

### 组件 CSS 与布局规则

- **布局模型**：uni-app-x 采用 **Flexbox 盒子模型**，所有布局基于 flex。App 端 `display` 只支持 `flex`（默认值），不支持 `block`/`inline`/`grid`。flex 默认方向为**纵向**（`column`），需要横向布局时显式写 `flex-direction: row`
- **基础组件优先**：布局用官方原生组件（`view`、`scroll-view`、`list-view`、`text`、`image`、`input` 等），扩展 UI 用 tmx-ui 的 `x-` 组件
- **禁止**使用 `uvuePx.scss` / `uvue.scss` 的工具类（`pa-12`、`flex-row`、`round-12` 等），工具类仅供页面使用
- 样式通过 `<style scoped>` 编写原生 CSS，或通过 `:style` 动态绑定
- 遵循官方 ucss 规范，详见 [css/README.md](css/README.md)
- App 端限制：仅 class 选择器、样式不继承、不支持 `*` 和标签选择器

### 组件 script 代码顺序

1. imports
2. **局部 type 定义**（`type XxxType = { ... }`，必须紧跟 import，在其他代码之前）
3. JSDoc 组件文档注释
4. `defineOptions`
5. Props type 定义（`type xXxxPropsType`）+ `defineProps` + `withDefaults`
6. `defineEmits`
7. `defineSlots`
8. 模板 ref（`const elRef = ref<UniElement | null>(null)`）
9. 响应式数据（`ref`）
10. 非响应式内部变量（`let tid = 0` 等计时器/内部状态）
11. `computed`（被后面函数调用的 computed 必须在前）
12. 方法函数（被其他函数调用的工具函数写在前面）
13. `watch`（尽量用 computed 替代）
14. 生命周期（`onMounted`、`onBeforeUnmount`）
15. `provide`
16. `defineExpose`

### UTS 类型系统规则（重要）

> **UTS 是严格类型语言。类型只能用 `type`（不用 `interface`），不允许内联类型和嵌套类型，必须平铺定义。**

#### 只能使用 `type`，禁止 `interface`

```uts
// 正确
type UserInfo = {
	name : string,
	age : number
}

// 错误
interface UserInfo {
	name : string
	age : number
}
```

#### 禁止内联类型和嵌套类型

函数参数、返回值、变量声明中不可使用内联对象类型。必须先 `type` 定义再引用。

```uts
// 错误：内联类型
const getData = () : { name : string, age : number } => { ... }
const info = ref<{ name : string }>({ name: '' })

// 错误：嵌套类型
type UserInfo = {
	name : string,
	address : { city : string, zip : string }  // 嵌套！
}

// 正确：平铺后引用
type AddressInfo = {
	city : string,
	zip : string
}
type UserInfo = {
	name : string,
	address : AddressInfo
}
const getData = () : UserInfo => { ... }
const info = ref<UserInfo | null>(null)
```

#### 局部 type 必须写在 import 之后、其他代码之前

```uvue
<script lang="ts" setup>
	// 1. imports
	import { ref, computed } from "vue"
	import { xConfig } from "../../config/xConfig.uts"

	// 2. 局部 type 定义（紧跟 import）
	type ColorResult = {
		border : string,
		background : string
	}
	type SizeType = 'mini' | 'normal' | 'large'

	// 3. defineOptions / defineProps / ...
	// 4. ref / computed / 函数 / 生命周期 ...
</script>
```

#### 所有数据、函数必须显式声明类型（void 除外）

```uvue
<!-- 正确 -->
<script lang="ts" setup>
	const count = ref<number>(0)
	const list = ref<string[]>([])
	const info = ref<UserInfo | null>(null)
	const getLabel = () : string => 'hello'
	const _show = computed(() : boolean => props.show)
	watch(() : boolean => props.disabled, () => { getNodes() })
	watch(() : string => props.color, (val : string) => { update(val) })
</script>

<!-- 错误：缺少类型标注 -->
<script lang="ts" setup>
	const count = ref(0)             // ✗ 缺少泛型
	const list = ref([])             // ✗ 缺少泛型
	const getLabel = () => 'hello'   // ✗ 缺少返回类型
	const _show = computed(() => props.show)      // ✗ 缺少返回类型
	watch(() => props.disabled, () => { getNodes() }) // ✗ getter 缺少返回类型
</script>
```

**唯一例外**：返回值为 `void` 的函数可以省略返回类型标注：
```uts
const handleClick = (e : UniPointerEvent) => { emits('click', e) }  // void 可省略
onMounted(() => { init() })  // void 回调可省略
```

### 声明顺序规则（重要）

> **UTS 要求：被调用的函数、变量、类型、属性必须在调用处之前定义，顺序不能反。没有任何提升（hoisting）。**

- `<script setup>` 中代码**自上而下**执行
- 如果函数 A 调用了函数 B，则 B 必须写在 A **之前**
- `computed`、`watch`、生命周期中引用的函数/变量，必须在它们之前定义
- `ref` / `let` / `const` 声明必须在被引用之前
- `type` 定义必须在使用该类型的 ref/computed/函数之前

```uvue
<!-- 正确：被调用函数在前 -->
<script lang="ts" setup>
	const getColor = () : string => { return 'red' }
	const _bgColor = computed(() : string => getColor())  // getColor 已在上方定义
</script>

<!-- 错误：调用在定义之前 -->
<script lang="ts" setup>
	const _bgColor = computed(() : string => getColor())  // 报错：getColor 未定义
	const getColor = () : string => { return 'red' }
</script>
```

方法函数内部的依赖关系也要遵循此规则：

```uvue
<!-- 正确：工具函数 → 业务函数 → 组合函数 -->
<script lang="ts" setup>
	// 1. 底层工具
	const formatValue = (v : string) : string => checkIsCssUnit(v, xConfig.unit)
	// 2. 依赖工具的业务函数
	const calcPosition = (x : number) : number => { /* 可调用 formatValue */ }
	// 3. 依赖业务函数的组合函数
	const handleMove = (evt : UniTouchEvent) => { /* 可调用 calcPosition */ }
</script>
```

### 组件开发详细参考

| 文档 | 内容 |
|---|---|
| [component-catalog.md](component-catalog.md) | tmx-ui 组件索引（含文档路径），**使用组件前必查** |
| [tmx-ui/](tmx-ui/) | 125 个组件的详细文档（Props/Events/Slots/示例） |
| [css-system.md](css-system.md) | uvue CSS 与 Web CSS 的差异、支持/禁用属性清单 |
| [uts-patterns.md](uts-patterns.md) | UTS 严格类型、语法规范、平台差异 |
| [vue-conventions.md](vue-conventions.md) | Vue 组合式 API、Props/Emits/Slots 规范 |
| [system-components.md](system-components.md) | 官方内置组件、uni API 用法 |
| [page-dev-guide.md](page-dev-guide.md) | 页面结构模板、Store/请求封装、组件使用示例 |

---

## 二、页面开发规范

> 适用范围：`pages/` 目录下的所有 `.uvue` 页面和页面级组件

### 布局模型

> uni-app-x 采用 **Flexbox 盒子模型**布局。App 端 `display` 只支持 `flex`（默认值），不支持 `block`/`inline`/`grid`。
> flex 默认方向为**纵向**（`column`），横向布局需显式 `flex-direction: row` 或工具类 `flex-row`。

### 新建页面必须注册到 pages.json（强制）

创建新页面后，**必须**在 `pages.json` 中注册路径，否则页面无法访问。

**项目使用分包结构**，主包和分包的注册方式不同：

```json
{
  "pages": [
    // 主包页面（pages/ 根目录下）
    { "path": "pages/index/index", "style": { "navigationBarTitleText": "首页", "navigationStyle": "custom" } }
  ],
  "subPackages": [
    // 分包：pages/biaodan/（表单相关页面）
    {
      "root": "pages/biaodan",
      "pages": [
        { "path": "form", "style": { "navigationBarTitleText": "表单 xForm" } },
        { "path": "input", "style": { "navigationBarTitleText": "输入框 xInput" } }
      ]
    }
  ]
}
```

**注册规则：**

| 场景 | 注册位置 | path 写法 |
|---|---|---|
| 主包页面 | `pages` 数组 | `"pages/目录/文件名"` （完整路径） |
| 分包页面 | `subPackages[n].pages` 数组 | `"文件名"` （相对于 root 的路径，不含 root 前缀） |

新页面应根据功能归入已有分包，或在 `pages/` 下新建目录并添加新的 `subPackages` 条目。创建前先读取 `pages.json` 确认现有分包结构。

**style 常用配置：**

```json
{
  "path": "pageName",
  "style": {
    "navigationBarTitleText": "页面标题",
    "navigationStyle": "custom",          // 使用 x-navbar 自定义导航栏时设置
    "enablePullDownRefresh": false         // 是否开启下拉刷新
  }
}
```

使用 `x-navbar` 自定义导航栏的页面必须设置 `"navigationStyle": "custom"`。

### 页面开发优先级

编写页面时，按以下优先级选择方案：

1. **官方原生组件做基础布局** — `view`（flex 容器）、`scroll-view`（滚动）、`text`、`image`、`input` 等是布局基石
2. **tmx-ui 组件做 UI 扩展** — 按钮用 `x-button`，容器用 `x-sheet`，栅格用 `x-row`/`x-col`，输入用 `x-input`，弹窗用 `x-modal`/`x-drawer`，导航用 `x-navbar`/`x-tabs` 等。tmx-ui 提供主题、暗黑模式、多端适配
3. **工具类辅助** — 使用 `uvuePx.scss` 的工具类（`flex-row`、`pa-12`、`round-8` 等）做间距和样式微调

### 页面 CSS 规则

**优先级：tmx-ui 组件 → CSS 工具类 → 自写 CSS**

开发页面或自定义组件时，优先使用 CSS 工具类布局，95% 场景无需手写样式表。工具类来自 `uvuePx.scss`（px 单位）/ `uvue.scss`（rpx 单位），兼容全平台。

完整 CSS 工具类规则见 **[css-system.md](css-system.md)**，以下为速查：

#### Flex 布局

| 类名 | 作用 |
|---|---|
| `flex-row` | 横向排列 |
| `flex-col` | 纵向排列 |
| `flex-center` | 居中对齐 |
| `flex-between` | 两边对齐 |
| `flex-around` | 均分 |
| `flex-wrap` | 自动换行 |
| `flex-1` ~ `flex-12` | flex 占比 |
| `flex-shrink` | 不被挤压 |

**Flex-row 精确定位**（配合 `flex-row`）：

| 上 | 中 | 下 |
|---|---|---|
| `flex-row-top-start/center/end` | `flex-row-center-start/center/end` | `flex-row-bottom-start/center/end` |

`flex-row-center-between`：上下居中、两边对齐

**Flex-col 精确定位**（配合 `flex-col`）：同理 `flex-col-top-start` 等 9 种组合，`flex-col-full` 使子元素宽 100%

#### 间距

| 规则 | 方向 | 值范围 | 示例 |
|---|---|---|---|
| `pa-{x}` | 内边距四周 | 0-50 | `pa-12` |
| `p{方向}-{x}` | 指定方向内边距 | 0-50 | `px-12`、`pt-8`、`py-16` |
| `ma-{x}` | 外边距四周 | 0-50 | `ma-12` |
| `m{方向}-{x}` | 指定方向外边距 | 0-50 | `ml-8`、`mt-16` |
| `m{方向}-n{x}` | 大间距（x*2） | n1-n25 | `ma-n10` = 20px |
| `mt--{x}` | 负外边距 | 0-50 | `mt--12` = -12px |

方向：`a`(四周) `l`(左) `r`(右) `t`(上) `b`(下) `x`(水平) `y`(垂直)

#### 圆角

| 规则 | 示例 |
|---|---|
| `round-{0-25}` | `round-12` |
| `round-{位置}-{0-25}` | `round-tl-8`（左上）、`round-t-12`（顶部两角） |

位置：`tl`(左上) `tr`(右上) `bl`(左下) `br`(右下) `t`(顶) `b`(底) `l`(左) `r`(右) `a`(全部)

#### 文本

| 类名 | 作用 |
|---|---|
| `text-size-{xxs/xs/s/n/g/lg/xl}` | 文本大小 |
| `text-weight-{s/n/b}` | 细/正常/加粗 |
| `text-align-{left/center/right}` | 对齐 |
| `text-overflow` | 单行省略 |
| `text-overflow-{1-5}` | 多行省略 |
| `text-delete` | 删除线 |
| `text-underline` | 下划线 |

#### 颜色

| 文本色 | 背景色 | 说明 |
|---|---|---|
| `text-primary` | `primary` | 主色 |
| `text-success` | `success` | 成功 |
| `text-warn` | `warn` | 警告 |
| `text-error` | `error` | 错误 |
| `text-info` | `info` | 信息 |
| `text-red/pink/purple/blue/green/orange...` | 同名 | 具名颜色 |

#### 其它

| 类名 | 作用 |
|---|---|
| `relative` / `absolute` / `fixed` | 定位 |
| `fulled` / `fulled-height` | 宽/高 100% |
| `overflow` / `overflow-x` / `overflow-y` | 溢出控制 |
| `opacity-{0-9}` | 透明度（0=全透明，9=90%） |
| `zIndex-{0-25}` | z-index 层级 |
| `t-{0-50}` / `l-{0-50}` / `r-{0-50}` / `b-{0-50}` | 定位偏移 |
| `t--{0-50}` | 负偏移 |

#### 额外约束

- **tmx-ui 容器组件禁止多层嵌套**（`x-sheet`、`x-row`、`x-col` 等），外层套一层后，内部用 `view` 等官方基础组件布局
- tmx-ui 未覆盖的场景，再使用原生 CSS 和官方组件
- 遵循官方文档规范（[css/](css/)、[uts/](uts/)、[api/](api/)、[vue/](vue/)）

### 页面示例

```uvue
<script lang="ts" setup>
	const list = ref<string[]>(['项目1', '项目2', '项目3'])
	const onConfirm = () => {
		uni.showToast({ title: '确认' })
	}
</script>
<template>
	<!-- #ifdef APP -->
	<scroll-view style="flex:1">
	<!-- #endif -->

	<x-navbar title="示例页面"></x-navbar>

	<x-sheet :round="8" :margin="[12,12,12,12]">
		<x-cell v-for="(item, index) in list" :key="index" :title="item"></x-cell>
	</x-sheet>

	<view class="pa-12">
		<x-button color="primary" :block="true" @click="onConfirm">确认</x-button>
	</view>

	<view class="flex-row flex-center pa-12">
		<x-tag color="success">标签1</x-tag>
		<x-tag color="warn">标签2</x-tag>
	</view>

	<!-- #ifdef APP -->
	</scroll-view>
	<!-- #endif -->
</template>
<style scoped>
</style>
```

### tmx-ui 组件文档检索规则（强制）

> **使用任何 `<x-xxx>` 组件前，必须按以下步骤操作：**
>
> 1. 打开 **[component-catalog.md](component-catalog.md)** 查找组件对应的**文档路径**（第三列）
> 2. **读取该文档**，了解完整 Props、Events、Slots、Ref 方法
> 3. 参考文档中的**示例源码**编写代码
>
> 文档按分类存放在 [tmx-ui/](tmx-ui/) 下：常用组件/、表单组件/、导航组件/、展示组件/、反馈组件/、其它组件/
>
> **禁止凭记忆猜测组件属性，必须查文档确认。**

### tmx-ui 组件快速索引

> **[component-catalog.md](component-catalog.md)** — 120+ 组件的分类速查清单（一行一个组件），快速定位需要的组件。

### 页面开发详细规范

> **[page-dev-guide.md](page-dev-guide.md)** — 页面开发参考，包含：
> - 页面基础结构模板（APP/微信条件编译）
> - Store 创建规范（reactive + 类型定义 + 修改函数）
> - API 请求封装（xRequest + 统一响应处理）
> - 常用 tmx-ui 组件的使用代码示例
> - 常用类型导入清单 / 页面开发 Checklist

### 页面开发参考

同「组件开发详细参考」表格中的文档，页面开发重点关注：
- [component-catalog.md](component-catalog.md) — 选组件
- [page-dev-guide.md](page-dev-guide.md) — 页面模板和示例
- [css-system.md](css-system.md) — CSS 差异和禁用属性

---

## 三、自定义组件开发规范

> 适用范围：非 tmx-ui 的 .uvue 组件（页面级封装、业务组件、通用组件）

### 与 tmx-ui 组件的区别

| | tmx-ui 组件 | 自定义组件 |
|---|---|---|
| 位置 | `uni_modules/tmx-ui/components/` | `pages/` 子目录或项目任意位置 |
| 命名 | `x-` 前缀，easycom 自动注册 | 自定义名称，手动 import 或 easycom |
| CSS | 禁止用 `uvuePx.scss` 工具类 | 可以用工具类 |
| 暗黑模式 | 必须适配 `xConfig.dark` | 按需适配 |
| 颜色/单位 | 必须用 `getDefaultColor` / `checkIsCssUnit` | 可直接写 CSS 值 |

### 自定义组件遵循的规则（与 tmx-ui 相同）

以下规则是 UTS/UVue 的通用约束，**所有 .uvue 文件都必须遵循**：

1. **`<script lang="ts" setup>`** — 统一使用组合式 API
2. **类型只能用 `type`** — 禁止 `interface`，禁止内联/嵌套类型
3. **显式类型标注** — `ref<T>()`、`computed((): T =>`、函数返回类型（void 除外）
4. **声明在前，调用在后** — UTS 无 hoisting
5. **`==` 不用 `===`** — UTS 不支持 `===`
6. **Flexbox 布局** — `display` 只支持 `flex`（App 端）
7. **条件编译** — `#ifdef APP`、`#ifdef MP-WEIXIN` 等
8. **代码顺序** — imports → type → defineOptions → defineProps → defineEmits → ref → computed → 函数 → watch → 生命周期 → provide → defineExpose

### 自定义组件模板

```uvue
<script lang="ts" setup>
    type PropsType = {
        /** 标题 */
        title : string,
        /** 是否显示 */
        visible : boolean
    }

    const props = withDefaults(defineProps<PropsType>(), {
        title: '',
        visible: false
    })

    const emit = defineEmits<{
        (e : 'close') : void,
        (e : 'confirm', value : string) : void
    }>()

    const _show = computed(() : boolean => props.visible)

    function handleConfirm() {
        emit('confirm', props.title)
    }

    defineExpose({
        open() { /* ... */ },
        close() { /* ... */ }
    })
</script>
<template>
    <view v-if="_show" class="my-dialog">
        <text>{{ title }}</text>
        <x-button color="primary" @click="handleConfirm">确认</x-button>
    </view>
</template>
<style scoped>
    .my-dialog {
        padding: 24px;
        background-color: #ffffff;
        border-radius: 12px;
    }
</style>
```

### 自定义组件中使用 tmx-ui

自定义组件内可以直接使用 tmx-ui 组件（easycom 自动注册），与页面中使用方式一致：

```uvue
<template>
    <x-sheet :round="8">
        <x-cell title="设置项" :showArrow="true" @click="onClick"></x-cell>
    </x-sheet>
</template>
```

