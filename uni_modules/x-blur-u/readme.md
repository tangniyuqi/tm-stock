# x-blur-u

### 开发文档
[TMUI4.0文档](https://xui.tmui.design/)
[TMUI4.0组件库](https://ext.dcloud.net.cn/plugin?id=16369)


### 说明

全平台可用的背景毛玻璃背景模糊插件，使用如view一样简单。

### 兼容性

| Harmony | IOS | Android | WEB | 小程序 |
| --- | --- | --- | --- | --- |
| 支持 | 支持 | 支持 | 支持 | 支持 |

### 使用前准备

### 开发环境配置
- 如果是uniapp应用需要自定基座
- uniappx不需要定义基座，但win开发ios时需要
- 如果是mac开发uniappx不需要基座

### Props 属性

| 名称 | 说明 | 类型 | 默认值 |
| ------ | ---- | ---- | ---- |
| customStyle | `自定内部样式`<br> | `string` | `''` |
| blur | `背景模糊程度`<br> | `number` | `'20'` |
| bgColor | `背景色`<br> | `string` | `"rgba(255,255,255,0.85)"` |

### 使用示例

```vue
<view class="testClassStyle">
	<x-blur-u style="height: 80px;width:100%">
		<text style="font-size: 20px;font-weight: bold;color:rgba(0,0,0,0.6);text-align: center;line-height: 4;">我是内容区域,请滚动页面。</text>
	</x-blur-u>
</view>

<style>
.testClassStyle{
	position: fixed;
	// #ifdef WEB
	top:44px;
	// #endif
	// #ifndef WEB
	top:0;
	// #endif
	left:0;
	width:100%;
	height:80px;
	z-index: 50;
}
</style>
```
