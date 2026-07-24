# 瀑布流子组件 xWaterfallItem
-------
<ViewMobile url="" />

## 介绍

注意事项：内容不可异步，不可直接修改本节点style上的margin,padding你可以通过在节点内套一个view来改变间隙。如果你放置了异步图片你应该写死图片的高。总之不可异步改变内容高，否则排版会失效和错乱。

## 平台兼容

| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |



## 文件路径

``` ts

@/uni_modules/tmx-ui/components/x-waterfall-item/x-waterfall-item.uvue
```

```mermaid

    flowchart LR
    uni_modules --> tmx-ui --> components --> x-waterfall-item --> x-waterfall-item.uvue
```

## 使用

``` ts

<x-waterfall-item></x-waterfall-item>
```

## Props 属性

| 名称 | 说明 | 类型 | 默认值 |
| ------ | ---- | ---- | ---- |
| order | `循环时，请填写key同等index即可`<br>`暂时无作用。`<br> | `number` | `-1` |


## Events 事件

| 名称 | 参数 | 说明 |
| ------ | ---- | ---- |


## Slots 插槽

| 名称 | 说明 | 数据 |
| ------ | ---- | ---- |
| default | `默认内容插槽，不要有动态高或者异步加载内容的行为。 ` | col: number<br> |


## Ref 方法

| 名称 | 参数 | 返回值 | 说明 |
| ------ | ---- | ---- | ---- |


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

