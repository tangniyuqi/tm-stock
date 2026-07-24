# 时间区间选择 xBetweenTime
-------
<ViewMobile url="/pages/biaodan/between-time" />

## 介绍

快速的时间区间选择器，方便时间选择自动判断前后时间大小并校正。

## 平台兼容

| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |



## 文件路径

``` ts

@/uni_modules/tmx-ui/components/x-between-time/x-between-time.uvue
```

```mermaid

    flowchart LR
    uni_modules --> tmx-ui --> components --> x-between-time --> x-between-time.uvue
```

## 使用

``` ts

<x-between-time></x-between-time>
```

## Props 属性

| 名称 | 说明 | 类型 | 默认值 |
| ------ | ---- | ---- | ---- |
| modelValue | `当前时间,与modelStr不同，此提供的值必须是正常的时间格式`<br>`否则报错，无法运行。`<br> | `Array` | `() : string[] => [] as string[]` |
| modelStr | `当前时间经过format格式化后输出的值。`<br>`此值不会处理输入，只输出显示。`<br> | `string` | `""` |
| modelShow | `当前打开的状态。`<br>`等同v-model:model-show`<br> | `boolean` | `false` |
| title | `顶部标题`<br>`空默认为：请选择时间（根据语言不同也不同）`<br>`如果你提供了值，组件内的多语言失效以你设定的为准`<br> | `string` | `""` |
| start | `开始时间,提供时，请提供完整到秒的格式`<br> | `string` | `""` |
| end | `结束时间,提供时，请提供完整到秒的格式`<br> | `string` | `""` |
| type | `精确到的级别,这里只是展示，具体的返回值还是完整的值。`<br>`year:年`<br>`month:年月`<br>`day:年月日`<br>`hour:年月日小时`<br>`minute:年月日小时分钟`<br>`second:年月日小时分钟秒`<br> | `ModelType` | `"day"` |
| format | `输出时间格式，只对v-model:modelStr及输入框展示有效YYYY-MM-DD，为空时由当前语文决定`<br>`因此它可能不是一个标准时间，比如YY SS ,所以不能作为modelValue使用`<br>`有效格式：`<br>`YYYY年`<br>`MM月`<br>`DD日`<br>`hh小时`<br>`mm分钟`<br>`ss秒`<br> | `string` | `""` |
| cellUnits | `上方的单位名称`<br>`默认为：['年', '月', '日', '时', '分', '秒']`<br> | `Array` | `() : string[] => [] as string[]` |
| quickDate | `快速时间区间选择，如果直接填写数字字符，会以你提供的数字最近多少来天来算。`<br>`d:本日`<br>`w:本周`<br>`m:本月`<br>`y:本年`<br>`q:本季度`<br>`7:最近7天，后面的依此类推，数字的就是最近xx天。`<br>`px:前x年，p+[x]数字依此类推，表示前x年,如：p1,p2...`<br>`如果提供以下json结构，则以你自定的为准。`<br>`UTSJSONObject:{title:'本学年',start:'2025-1-1',end:'2025-12-31'} as UTSJSONObject`<br> | `Array` | `() : Array<any> => ['d', 'w', 'm', 'y', 'q'] as Array<any>` |
| lazyContent | `是否懒加载内部内容。`<br>`当前你的列表内容非常多，且影响打开的动画性能时，请务必`<br>`设置此项为true，以获得流畅视觉效果。如果选择数据较少没有必要打开`<br>`要兼容微信,必须设置为true,非微信可以为false`<br> | `boolean` | `true` |
| drawerSize | `如果你的快捷选择较多可能会让高度不足，需要自行设置下高。`<br> | `string` | `'540px'` |
| disabledClear | `是否禁用清除按钮，默认不禁用，允许用户清空选择。点确认，以清空选项数据`<br> | `boolean` | `false` |
| disabled | `是否禁用弹出`<br> | `boolean` | `false` |
| widthCoverCenter | `宽屏时是否让内容剧中显示`<br>`并限制其宽为屏幕宽，只展示中间内容以适应宽屏。`<br> | `boolean` | `true` |


## Events 事件

| 名称 | 参数 | 说明 |
| ------ | ---- | ---- |
| cancel | `-` | 取消时触发 |
| confirm | `date: string` | 确认触发 |
| change | `date: string` | 滑动变换时触发 |
| dateClick | `item: UTSJSONObject<{text:string,value:string[]}>` | 快速日期被选中时触发 |
| update:modelShow | `-` | 变量控制打开状态<br>等同v-model:model-show |
| update:modelStr | `-` | 经格式化后的值。等同v-model:model-str |
| update:modelValue | `-` | - |


## Slots 插槽

| 名称 | 说明 | 数据 |
| ------ | ---- | ---- |
| default | `插槽,默认触发打开选择器。你的默认布局可以放置在这里。` | show: boolean<br>show: boolean<br>show: boolean<br>startVal: boolean<br>startVal: boolean<br>startVal: boolean<br>endVal: boolean<br>endVal: boolean<br>endVal: boolean<br> |


## Ref 方法

| 名称 | 参数 | 返回值 | 说明 |
| ------ | ---- | ---- | ---- |
| open | - | `-` | 打开选择器 * |
| close | - | `-` | 关闭选择器 * |
| clear | - | `-` | 清空选择 * |


## 示例文件路径

``` json

/pages/biaodan/between-time
```

```mermaid

    flowchart LR
    根目录  --> pages --> biaodan --> between-time
```

## 示例源码

::: details uvue

<<< ../../../../pages/biaodan/between-time.uvue{vue}

:::

