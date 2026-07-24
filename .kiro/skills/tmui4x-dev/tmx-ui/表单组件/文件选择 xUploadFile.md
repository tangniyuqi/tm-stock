# 文件选择 xUploadFile
-------
<ViewMobile url="/pages/biaodan/upload-file" />

## 介绍

使用时请注意区别：安卓端和IOS端没有区别。web端是我模拟的格式有点区别，上传文件是File对象。
没有提供双向绑定，因为可能的数据格式化比较麻烦，请根据change事件来保存数据,微信端请保证你的小程序隐私政策中勾选了相关权限使用.

## 平台兼容

| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |



## 文件路径

``` ts

@/uni_modules/tmx-ui/components/x-upload-file/x-upload-file.uvue
```

```mermaid

    flowchart LR
    uni_modules --> tmx-ui --> components --> x-upload-file --> x-upload-file.uvue
```

## 使用

``` ts

<x-upload-file></x-upload-file>
```

## Props 属性

| 名称 | 说明 | 类型 | 默认值 |
| ------ | ---- | ---- | ---- |
| defaultList | `提供的默认上传的文件列表`<br>`请确保要有以下三个字段：`<br>`1.name`<br>`2.type`<br>`3.request 你服务器自己构造的成功数据，例：{code,data,msg}`<br>`小心的是，如果你变动了此值，内部会把默认当前的列表数据清空，直接用此列表替换。这样你在表单加载数据更新编辑时`<br>`会很有用。`<br> | `union` | `():UTSJSONObject[] null =>{     return null }` |
| header | `头部数据`<br> | `union` | `():UTSJSONObject null =>{     return null }` |
| name | `文件上传的字段名称`<br> | `string` | `"file"` |
| formData | `表单数据`<br> | `union` | `():UTSJSONObject null =>{     return null }` |
| url | `服务器上传地址`<br> | `string` | `"https://mockapi.eolink.com/LRViGGZ8e6c1e8b4a636cd82bca1eb15d2635ed8c74e774/admin/upload_pic/"` |
| maxCount | `最大上传数量`<br>`-1表示不限制`<br> | `number` | `-1` |
| multiple | `是否多选`<br> | `boolean` | `true` |
| maxFileSize | `最大文件大小,单位字节`<br>`超过此大小的文件不会被上传。30*1024*1024`<br> | `number` | `30 * 1024 * 1024` |
| loading | `当前是否在上传中`<br>`请使用v-model:loading来`<br>`不可更改此值。`<br> | `boolean` | `true` |
| autoStart | `选择文件后自动上传`<br> | `boolean` | `true` |
| statusCode | `服务正常返回的状态码如果为此值表示失败。`<br> | `number` | `200` |
| disabled | `禁用上传功能`<br> | `boolean` | `false` |
| uploadFileDisabledDel | `已上传的文件是否禁用删除`<br>`这个是某些特殊的erp系统中的需求需要。`<br> | `boolean` | `false` |
| filter | `文件过滤器,不要动态修改。`<br>`空数组时为所有文件类型,类型请注意使用如下规范:`<br>`如:["jpg","docx","doc","webp"]`<br>`在ios需要14+支持类型过滤，14以下此属性无效,`<br>`web是过滤指定类型.`<br>`在安卓端事实上系统差异,暂时无法多文件类型过滤,安卓只会取类型文件中的最后一项,如上述只会取webp`<br>`但在安卓是我映射的类型,因为如果比如我要jpg,gif格式,只取gif肯定不行,因此你写jpg,gif,png其实在`<br>`安卓端都是同一种类型文件对应image*所有图图片类型,比如你写pdf,zip,josn格式,都是统一为application*`<br>`因此在安卓它是对应一种文件类的类型不是指代具体的后缀.`<br> | `Array` | `() : string[] => [] as string[]` |
| borderBottomColor | `底部线条颜色`<br> | `string` | `"#f5f5f5"` |
| beforeUpload | `上传前执行的函数勾子,如果不会使用见demo`<br>`类型:(lsit:xFileSListType[]) => Promise<xFileSListType[]>`<br>`这里可以过滤不需要上传的文件.`<br> | `XFILESUPLOAD_BEFORE_CALLBACK` | `(list : xFileSListType[]) : Promise<xFileSListType[]> => {     return Promise.resolve(list) }` |


## Events 事件

| 名称 | 参数 | 说明 |
| ------ | ---- | ---- |
| change | `-` | 只有删除和上传完成时才会触发<br>并返回当前的文件列表。 |


## Slots 插槽

| 名称 | 说明 | 数据 |
| ------ | ---- | ---- |
| default | `触发文件选择的区域插槽` | - |
| list | `-` | - |


## Ref 方法

| 名称 | 参数 | 返回值 | 说明 |
| ------ | ---- | ---- | ---- |
| start | - | `-` | 上传文件列表 |
| clearCacheFiles | - | `-` | 安卓专用清缓存函数 |


## 示例文件路径

``` json

/pages/biaodan/upload-file
```

```mermaid

    flowchart LR
    根目录  --> pages --> biaodan --> upload-file
```

## 示例源码

::: details uvue

<<< ../../../../pages/biaodan/upload-file.uvue{vue}

:::

