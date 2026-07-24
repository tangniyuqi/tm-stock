# 富文本编辑器 xEditor
-------
<ViewMobile url="/pages/biaodan/editor" />

## 介绍

传递正常markdown或者html内容即可,传递markdown时会自动转换为html,如果直接传递html不会转换直接赋值.
值得注意的是:在微信小程序端它是没有样式高亮显示的.其它平台有样式指示,这是因为受限于微信官方本身就不支持.
另外我测试发现HBX4.53 sdk ios端的输入框焦点有问题，导致无法设置样式，待官方修复。

## 平台兼容

| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |



## 文件路径

``` ts

@/uni_modules/tmx-ui/components/x-editor/x-editor.uvue
```

```mermaid

    flowchart LR
    uni_modules --> tmx-ui --> components --> x-editor --> x-editor.uvue
```

## 使用

``` ts

<x-editor></x-editor>
```

## Props 属性

| 名称 | 说明 | 类型 | 默认值 |
| ------ | ---- | ---- | ---- |
| width | `窗口宽`<br> | `string` | `'auto'` |
| height | `窗口高,可以传递所支持的任意单位高.`<br> | `string` | `'350'` |
| value | `需要渲染的markdow或者html内容。`<br> | `string` | `''` |
| isHtml | `是否启用纯html渲染。如果你的内容含有特殊字符比如:%,^&%这种不要出现在里面`<br>`此时你启用isHtml会经过数据处理直接跳过插件,直接赋值内容到html.就不要启用Markdown了.`<br> | `boolean` | `false` |
| nodeStyle | `富文本的style样式,不可以动态更改.`<br> | `string` | `'line-height:1.6;color:#000'` |
| nodeDarkStyle | `同上，暗黑时的样式.`<br> | `string` | `'line-height:1.6;color:#fff'` |
| color | `默认的按钮背景色`<br> | `string` | `'#f5f5f5'` |
| activeColor | `激活时的选中背景色，空值取全局`<br> | `string` | `''` |
| customColos | `自定义默认的文字/背景颜色`<br> | `Array` | `() : string[] => ['#ff0000', '#ff00ff', '#00ff00', '#00ffff', '#ffff00', '#ffffff', '#000000'] as string[]` |


## Events 事件

| 名称 | 参数 | 说明 |
| ------ | ---- | ---- |
| tagClick | `undefined` | 特定的a,img标签被点击触发,小程序不支持,其它平台支持. |
| init | `-` | 是否初始化成功 |
| getValue | `-` | 需要通过ref函数调用getHtml才会触发此函数 |


## Slots 插槽

| 名称 | 说明 | 数据 |
| ------ | ---- | ---- |


## Ref 方法

| 名称 | 参数 | 返回值 | 说明 |
| ------ | ---- | ---- | ---- |
| getHtml | - | `-` | 获取html内容 * |
| getSelected | - | `-` | 获取选区 * |


## 示例文件路径

``` json

/pages/biaodan/editor
```

```mermaid

    flowchart LR
    根目录  --> pages --> biaodan --> editor
```

## 示例源码

::: details uvue

<<< ../../../../pages/biaodan/editor.uvue{vue}

:::

