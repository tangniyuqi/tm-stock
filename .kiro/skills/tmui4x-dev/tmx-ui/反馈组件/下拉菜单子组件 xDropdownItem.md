# 下拉菜单子组件 xDropdownItem
-------
<ViewMobile url="" />

## 介绍

注意只能放置在父组件x-dropdown-menu中

## 平台兼容

| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |



## 文件路径

``` ts

@/uni_modules/tmx-ui/components/x-dropdown-item/x-dropdown-item.uvue
```

```mermaid

    flowchart LR
    uni_modules --> tmx-ui --> components --> x-dropdown-item --> x-dropdown-item.uvue
```

## 使用

``` ts

<x-dropdown-item></x-dropdown-item>
```

## Props 属性

| 名称 | 说明 | 类型 | 默认值 |
| ------ | ---- | ---- | ---- |
| title | `菜单标题`<br> | `string` | `"标题"` |
| keyName | `标识，变换或者点击时，会通过事件传回。`<br> | `string` | `""` |
| icon | `未选中时的图标`<br> | `string` | `"arrow-down-s-fill"` |
| activeIcon | `激活时的图标`<br> | `string` | `"arrow-up-s-fill"` |
| fontColor | `默认的文字及图标颜色`<br> | `string` | `"#333333"` |
| darkFontColor | `暗黑时的默认的文字及图标颜色，`<br>`空取时白色`<br> | `string` | `""` |
| fontSize | `文字及图标大小`<br> | `string` | `"16"` |
| activeFontColor | `激活的文字及图标颜色`<br>`空值时取全局统一的主题色。`<br> | `string` | `""` |
| isBtn | `是否是按钮选项。`<br> | `boolean` | `false` |
| color | `内容背景颜色`<br> | `string` | `"white"` |
| darkColor | `暗黑时的内容背景颜色,空值取sheetDarkColor`<br> | `string` | `""` |
| render | `渲染,如果设置为true,内部使用vif切换渲染`<br>`注意它不会影响其它菜单只影响本菜单.vif切换会导致内容重绘,但`<br>`可以解决sdk的一些异常组件嵌套的问题.`<br>`当您嵌套的组件在此内出现异常,闪退,报错时请设置为true,否则不用理会本属性`<br>`本属性存在就是为了绕开sdk bug.`<br> | `boolean` | `false` |


## Events 事件

| 名称 | 参数 | 说明 |
| ------ | ---- | ---- |


## Slots 插槽

| 名称 | 说明 | 数据 |
| ------ | ---- | ---- |
| default | `默认插槽，弹层内容。` | - |


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

