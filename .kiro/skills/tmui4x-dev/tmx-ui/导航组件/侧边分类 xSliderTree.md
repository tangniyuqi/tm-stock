# 侧边分类 xSliderTree
-------
<ViewMobile url="/pages/daohang/slider-tree" />

## 介绍

侧边分类选择，可多选，单选模式。

## 平台兼容

| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |



## 文件路径

``` ts

@/uni_modules/tmx-ui/components/x-slider-tree/x-slider-tree.uvue
```

```mermaid

    flowchart LR
    uni_modules --> tmx-ui --> components --> x-slider-tree --> x-slider-tree.uvue
```

## 使用

``` ts

<x-slider-tree></x-slider-tree>
```

## Props 属性

| 名称 | 说明 | 类型 | 默认值 |
| ------ | ---- | ---- | ---- |
| width | `宽`<br> | `string` | `"auto"` |
| height | `高是必填，不可为auto。`<br> | `string` | `"100%"` |
| activeTextColor | `侧边选中的文字颜色，空值取全局主题`<br> | `string` | `""` |
| textColor | `侧边未选中时的文字颜色`<br> | `string` | `"#888888"` |
| itemTextColor | `选项项目未选中的文字颜色`<br> | `string` | `"#333333"` |
| itemActiveColor | `选项项目选中的文字颜色，空值取全局主题`<br> | `string` | `""` |
| sliderBgColor | `左侧边栏背景颜色`<br> | `string` | `"#f5f5f5"` |
| darkSliderBgColor | `左侧边栏暗黑背景颜色`<br>`如果不提供，自动读取全局的backgroundColorContentDark背景色`<br> | `string` | `""` |
| sliderContentBgColor | `右内容区域背景颜色`<br> | `string` | `"white"` |
| darkSliderContentBgColor | `右内容区域暗黑背景颜色`<br>`如果不提供读取sheet窗口的暗黑背景`<br> | `string` | `""` |
| sliderWidth | `侧边栏宽`<br> | `string` | `"100"` |
| list | `数据列表`<br> | `Array` | `[] as SLIDER_TREE_ITEM_INFO[]` |
| modelValue | `当前选中项的id数组`<br> | `Array` | `[] as string[]` |
| multiple | `每级是否允许多选`<br> | `boolean` | `false` |
| fontSize | `文字大小`<br> | `string` | `"16"` |


## Events 事件

| 名称 | 参数 | 说明 |
| ------ | ---- | ---- |
| change | `ids: string[]` | 选中变换时触发 |
| update:modelValue | `-` | 等同v-model:modelValue |


## Slots 插槽

| 名称 | 说明 | 数据 |
| ------ | ---- | ---- |
| default | `-` | - |


## Ref 方法

| 名称 | 参数 | 返回值 | 说明 |
| ------ | ---- | ---- | ---- |
| getSelected | - | `-` | 获取当前选中的项 * |
| setSelected | - | `-` | 设置选中的项 * |


## 示例文件路径

``` json

/pages/daohang/slider-tree
```

```mermaid

    flowchart LR
    根目录  --> pages --> daohang --> slider-tree
```

## 示例源码

::: details uvue

<<< ../../../../pages/daohang/slider-tree.uvue{vue}

:::

