# 图片上传 xUploadMedia
-------
<ViewMobile url="/pages/biaodan/upload-media" />

## 介绍

可以上传视频或者图片,注意不要混搭.要么上传视频,要么上传图片.当用户自行添加默认文件时可以给你的文件对象加status:0表示待上传,要确保你的上传文件路径是正确的
否则安卓会引发io错误,并且uni.upload是无法捕捉到fail事件中,会导致整个程序不可用

## 平台兼容

| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |



## 文件路径

``` ts

@/uni_modules/tmx-ui/components/x-upload-media/x-upload-media.uvue
```

```mermaid

    flowchart LR
    uni_modules --> tmx-ui --> components --> x-upload-media --> x-upload-media.uvue
```

## 使用

``` ts

<x-upload-media></x-upload-media>
```

## Props 属性

| 名称 | 说明 | 类型 | 默认值 |
| ------ | ---- | ---- | ---- |
| mode | `上传的类型,video,photo,默认是上传图片`<br> | `string` | `'photo'` |
| videoOps | `当mode=video时,选择视频上传的参数,`<br>`些参数为utsjson,所有字段为可选,具体字段为uni.chooseVideo(options)中的部分有效字段`<br>`pageOrientation,albumMode,compressed,compressed,maxDuration,camera`<br> | `UTSJSONObject` | `() : UTSJSONObject => {     return {         pageOrientation: 'auto',         albumMode: 'system',         sourceType: ['album', 'camera'] as string[],         compressed: true,         maxDuration: 60,         camera: 'back'     } as UTSJSONObject }` |
| maxCount | `最大的可上传数量`<br> | `number` | `9` |
| url | `上传地址`<br> | `string` | `"https://mockapi.eolink.com/LRViGGZ8e6c1e8b4a636cd82bca1eb15d2635ed8c74e774/admin/upload_pic/"` |
| name | `上传到服务器的名称字段`<br> | `string` | `"file"` |
| header | `上传到服务器的头文件`<br> | `UTSJSONObject` | `() : UTSJSONObject => { return {} as UTSJSONObject }` |
| formData | `额外的表单数据。`<br> | `union` | `() : UTSJSONObject => { return {} as UTSJSONObject }` |
| imgHeight | `图片高,此处不可使用%单位`<br> | `string` | `"80"` |
| column | `一行显示几列`<br> | `number` | `5` |
| okFileIsDelete | `上传成功的文件是否允许删除`<br> | `boolean` | `true` |
| uploadingFileIsDelete | `上传中的文件是否允许删除`<br> | `boolean` | `true` |
| modelValue | `等同v-model`<br> | `Array` | `() : XUPLOADFILE_FILE_VALUE[] => [] as XUPLOADFILE_FILE_VALUE[]` |
| beforeDel | `图片被删除时触发`<br>`如果返回Promise<false>删除失败否则成功`<br> | `funcalldel` | `getNOOP_DEL()` |
| beforeComplete | `你需要原路返回参数提供的item`<br>`item可以自行修改响应内容，响应类型这样可以自己根据服务的内容判断`<br>`是成功还是失败或者没有权限。示例见demo使用。`<br> | `funbeforeCompelte` | `getNOOP_COMPLETE()` |
| beforeUpload | `上传前的最后一步执行,异步回调,函数返回一个item:XUPLOADFILE_FILE_INFO`<br>`返回时要原样类型返回Promise<XUPLOADFILE_FILE_INFO>,你可以在这里修改文件参数跳过上传或者`<br>`进行最后的头参数设置.demo有参考.`<br> | `beforeUploadType` | `getNOOP_UPLOAD()` |
| autoStart | `是否自动上传`<br> | `boolean` | `true` |
| sourceType | `图片来源同官方的sourceType：'album','camera'`<br> | `Array` | `() : string[] => ['album', 'camera'] as string[]` |
| compress | `是否压缩`<br> | `boolean` | `true` |
| compressedHeight | `压缩后的缩放高，0表示不压缩高`<br> | `number` | `0` |
| compressedWidth | `压缩后的缩放高，0表示不压缩宽`<br> | `number` | `0` |
| quality | `压缩质量`<br> | `number` | `80` |
| addPos | `添加图片的位置 'before'出现在前面 'after'出现在后面`<br> | `string` | `"after"` |
| align | `子项对齐方式，left左对齐默认，right:右对齐`<br> | `string` | `"left"` |


## Events 事件

| 名称 | 参数 | 说明 |
| ------ | ---- | ---- |
| complete | `XUPLOADFILE_FILE_VALUE[]` | 每次全部上传完时触发 |
| change | `XUPLOADFILE_FILE_VALUE[]` | 变化时触发 |
| delete | `XUPLOADFILE_FILE_VALUE[]` | 图片被删除时触发 |
| update:modelValue | `-` | - |


## Slots 插槽

| 名称 | 说明 | 数据 |
| ------ | ---- | ---- |
| default | `上传图片按钮插槽。` | - |


## Ref 方法

| 名称 | 参数 | 返回值 | 说明 |
| ------ | ---- | ---- | ---- |
| upload | - | `-` | - |
| chooseFile | - | `-` | - |


## 示例文件路径

``` json

/pages/biaodan/upload-media
```

```mermaid

    flowchart LR
    根目录  --> pages --> biaodan --> upload-media
```

## 示例源码

::: details uvue

<<< ../../../../pages/biaodan/upload-media.uvue{vue}

:::

