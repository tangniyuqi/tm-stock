# 单选框组 xRadioGroup
-------
<ViewMobile url="" />

## 介绍

使用时,从1.1.2开始允许是非直接xRadio子节点布局,但考虑到性能建议是直接子节点.

## 平台兼容

| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |



## 文件路径

``` ts

@/uni_modules/tmx-ui/components/x-radio-group/x-radio-group.uvue
```

```mermaid

    flowchart LR
    uni_modules --> tmx-ui --> components --> x-radio-group --> x-radio-group.uvue
```

## 使用

``` ts

<x-radio-group></x-radio-group>
```

## Props 属性

| 名称 | 说明 | 类型 | 默认值 |
| ------ | ---- | ---- | ---- |
| modelValue | `当前选中的值。null表示未选中。`<br> | `union` | `""` |
| direction | `对齐方式`<br> | `union` | `"row"` |


## Events 事件

| 名称 | 参数 | 说明 |
| ------ | ---- | ---- |
| change | `val: union` | 选项变化时触发。 |
| update:modelValue | `-` | - |


## Slots 插槽

| 名称 | 说明 | 数据 |
| ------ | ---- | ---- |
| default | `从1.1.2开始允许是非直接xRadioGroup子节点布局,但考虑到性能建议是直接子节点.` | - |


## Ref 方法

| 名称 | 参数 | 返回值 | 说明 |
| ------ | ---- | ---- | ---- |
| addItem | - | `-` | 添加单选项 * |
| getAllSelecteds | - | `-` | 获取所有选中的值 * |


## 示例文件路径

``` json


```

```mermaid

    flowchart LR
    根目录 
```

## 示例源码

::: details uvue



:::

