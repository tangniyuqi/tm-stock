# 轮播子组件 xSwiperItem
-------
<ViewMobile url="" />

## 介绍

注意：本组件非官方的swiper轮播的封装，而是作者设计并开发的轮播，因此有些使用上的区别，请仔细看文档。
之所以要重新设计轮播，是为了后期的功能扩展。当前的一些功能已经比官方的还更好。
比如支持阻尼动效的设计，动效函数的设置。临界位置回弹的设置。指示点位置的偏移设置（这点非常有用，然官方不支持，而我的由于是自行开发的可以随意设置）
如果你有更多需求，你只需要阅读源码后自行扩展更多的功能。
只能放置在父组件x-swiper中，不可单独使用。
click 事件请绑定在 swiper 上

## 平台兼容

| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |



## 文件路径

``` ts

@/uni_modules/tmx-ui/components/x-swiper-item/x-swiper-item.uvue
```

```mermaid

    flowchart LR
    uni_modules --> tmx-ui --> components --> x-swiper-item --> x-swiper-item.uvue
```

## 使用

``` ts

<x-swiper-item></x-swiper-item>
```

## Props 属性

| 名称 | 说明 | 类型 | 默认值 |
| ------ | ---- | ---- | ---- |
| order | `请务必正确填写轮播顺序`<br>`这将影响播放模式的正确性`<br>`循环时直接填写 index 顺序就可以了。`<br> | `number` | `-1` |
| round | ``<br> | `string` | `""` |


## Events 事件

| 名称 | 参数 | 说明 |
| ------ | ---- | ---- |
| click | `-` | 组件被点击 |


## Slots 插槽

| 名称 | 说明 | 数据 |
| ------ | ---- | ---- |
| default | `只允许放其父节点x-swiper里面。` | - |


## Ref 方法

| 名称 | 参数 | 返回值 | 说明 |
| ------ | ---- | ---- | ---- |
| setConfig | - | `-` | - |
| setList | - | `-` | - |
| setNextOrNext | - | `-` | - |
| setNextOrPrev | - | `-` | - |


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

