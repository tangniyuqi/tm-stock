# uvue CSS 规范（与 Web CSS 的差异）

> 来源：[官方文档](https://doc.dcloud.net.cn/uni-app-x/css/)
> uni-app x 在 App 端实现了 Web CSS 的子集（ucss）。编译到 Web/小程序时支持全部 CSS。
> **以下规则主要针对 App 端**，Web 端无这些限制。

## 一、三大核心差异（必记）

### 1. 仅 Flex 布局

App 端 `display` 只支持 `flex`（默认值），**不支持** `block`、`inline`、`grid`、`table` 等。

flex 默认方向为**纵向**（`column`），与 W3C 默认横向（`row`）不同。横向排列需显式写 `flex-direction: row`。

### 2. 仅 class 选择器

App 端**只能用 class 选择器**，不支持：
- 标签选择器（`div`、`view`、`text`）
- ID 选择器（`#id`）
- 属性选择器（`[type="text"]`）
- 伪类/伪元素（`:hover`、`::before`）— Web 端条件编译内可用
- `*` 通配符
- class 名只能含 `A-Z`、`a-z`、`0-9`、`_`、`-`

### 3. 样式不继承

父元素样式**不会影响**子元素。文字样式必须写在 `<text>` 组件上，不能写在父 `<view>` 上。

```html
<!-- 错误：App 端文字颜色不会变 -->
<view style="color:red">
    <text>123</text>
</view>

<!-- 正确 -->
<view>
    <text style="color:red">123</text>
</view>
```

`inherit`、`unset` 关键字在 App 端不支持。

## 二、CSS 默认值重置

以下默认值与 W3C 标准不同：

| 属性 | uni-app x（App） | W3C 标准 |
|---|---|---|
| `display` | `flex` | `inline` |
| `flex-direction` | `column` | `row` |
| `flex-shrink` | `0` | `1` |
| `box-sizing` | `border-box` | `content-box` |
| `position` | `relative` | `static` |
| `overflow` | `hidden` | `visible` |
| `font-size` | `16px` | `medium` |
| `z-index` | `0` | `auto` |
| `color` | `#000000` | `canvastext` |
| `border-color` | `#000000` | `currentcolor` |
| `align-items` | `stretch` | `normal` |
| `align-content` | `stretch` | `normal` |
| `text-align` | `left` | `start` |

## 三、支持的 CSS 属性（完整清单）

**以下属性可以使用**，不在此列表中的属性 App 端不支持：

布局：`display`、`flex`、`flex-direction`、`flex-wrap`、`flex-flow`、`flex-grow`、`flex-shrink`、`flex-basis`、`align-items`、`align-self`、`align-content`、`justify-content`

尺寸：`width`、`height`、`min-width`、`min-height`、`max-width`、`max-height`

间距：`margin`、`margin-top/right/bottom/left`、`padding`、`padding-top/right/bottom/left`

定位：`position`、`top`、`right`、`bottom`、`left`、`z-index`

边框：`border`、`border-width`、`border-style`、`border-color`、`border-radius`、`border-top/right/bottom/left`（及其子属性）

背景：`background-color`、`background-image`、`background-clip`（**禁止用 `background` 简写**，必须分开写 `background-color` 和 `background-image`）

文本：`color`、`font-size`、`font-weight`、`font-style`、`font-family`、`line-height`、`letter-spacing`、`text-align`、`text-overflow`、`text-decoration`（及其子属性）、`text-shadow`、`white-space`、`lines`（非标准，限制行数）

视觉：`opacity`、`visibility`、`box-shadow`、`box-sizing`、`overflow`、`pointer-events`

变换：`transform`、`transform-origin`

过渡：`transition`、`transition-property`、`transition-duration`、`transition-delay`、`transition-timing-function`

## 四、不支持的常见 Web CSS（禁用）

| 禁用属性/特性 | 说明 |
|---|---|
| `background: xxx` 简写 | **禁止**。必须分开写 `background-color` 和 `background-image` |
| `gap` / `row-gap` / `column-gap` | 不支持，用 `margin` 或嵌套 `view` 加 `padding`/`margin` 实现间距 |
| `display: block/inline/grid/table` | 仅支持 flex |
| `float` | 不支持 |
| `clear` | 不支持 |
| `vertical-align` | 不支持（用 flex align-items） |
| `text-indent` | 不支持 |
| `word-break` / `word-wrap` / `overflow-wrap` | 不支持 |
| `cursor` | 不支持 |
| `outline` | 不支持 |
| `list-style` | 不支持 |
| `animation` / `@keyframes` | 不支持（用 transition 或 x-animation 组件） |
| `calc()` / `min()` / `max()` / `clamp()` | 不支持 |
| `media query` (`@media`) | 不支持 |
| `grid` 相关属性 | 不支持 |
| `:hover` / `:focus` / `:active` | 不支持（Web 条件编译内可用） |
| `::before` / `::after` | 不支持 |
| `inherit` / `unset` / `initial` | 不支持 |
| 标签/ID/属性选择器 | 不支持 |

## 五、其它注意事项

### z-index

仅对**同层兄弟节点**生效，不支持脱离 DOM 树任意调层级。

### !important

仅支持在 **class 选择器**中使用（可覆盖内联 style），style 内联属性不支持 `!important`。

### 样式优先级

- `style` 属性 > `class` 选择器
- 多个 class 时，按 class 属性中的**书写顺序**确定优先级（后面的优先级高）

### CSS 方法

仅支持：`url()`、`rgb()`、`rgba()`、`var()`、`env()`

### 长度单位

支持：`px`、`rpx`、`%`、`vh`、`vw`

### 页面级滚动

App 端**页面本身不滚动**，需要滚动必须放 `scroll-view`。推荐页面根节点用条件编译：

```html
<!-- #ifdef APP -->
<scroll-view style="flex:1">
<!-- #endif -->
    <!-- 页面内容 -->
<!-- #ifdef APP -->
</scroll-view>
<!-- #endif -->
```

### 渐变色（background-image）

App 端**不支持背景图片**（url），仅支持 `linear-gradient` 渐变。且有严格限制：

```css
/* 正确写法 */
background-image: linear-gradient(to right, #5d6dff, #4cc8ff);

/* 错误：用 background 简写 */
background: linear-gradient(135deg, #5d6dff, #4cc8ff);

/* 错误：用角度 */
background-image: linear-gradient(135deg, red, yellow);

/* 错误：超过 2 个颜色 */
background-image: linear-gradient(to right, red, yellow, green);
```

**App 端限制：**
- **必须用 `background-image`**，不能用 `background` 简写
- **方向只能用 `to xxx`**：`to right`、`to left`、`to top`、`to bottom`、`to bottom left`、`to bottom right`、`to top left`、`to top right`
- **不支持角度**（`45deg`、`135deg` 等）
- **只支持 2 个颜色**（起始色 + 终止色），不支持 3 个及以上
- **不支持 `url()` 背景图片**
- `background-image` 优先级高于 `background-color`，同时设置时 `background-color` 被覆盖

### 文字必须用 text 组件

直接写在 `<view>` 中的文字会被编译器自动套 `<text>`，但无法设置样式。必须显式使用 `<text>` 组件。
