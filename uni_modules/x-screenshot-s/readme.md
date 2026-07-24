# x-screenshot-s
### 开发文档

1. 可以对窗口进行静默截图
2. 可以对某一个view节点进行截图

### 应用场景

1. 对产品端,给用户反馈保存时,可以对当前反馈的页面进行截图反馈,比如界面异常上传等
2. 对局部节点截图保存可以进行对局部的分享保存,比如页面的排版海报
3. 对整个页面分享保存

### 兼容性

| Harmony | IOS | Android | WEB | 小程序 |
| --- | --- | --- | --- | --- |
| 支持 | 支持 | 支持 | x | x |

### 注意事项

使用前一定要打基座才可用，一定要在页面上先引用，再去打基座。
如果你mac开发。ios可以不用打基座，能直接使用（但前提是你要配置好原生开发环境，否则一样要打包）
如果你是开始安卓，不管是mac,win电脑都要打包基座才能使用。

### API说明

#### getRootShotImage
```ts
type XScreenshotResult = { path: string }
type XScreenshotRootOpts = {
  /** 保存格式，默认 png，可选 jpg */
  format?: 'png' | 'jpg'
  success?: (res: XScreenshotResult) => void
  fail?: (res: XScreenshotFail) => void
  complete?: (res: XScreenshotResult | null) => void
}
function getRootShotImage(opts: XScreenshotRootOpts): void
```

**参数说明：**
- opts.format：图片格式，`png`（默认；**节点截图**仅绘制目标 View，临时清除目标节点自身 background 以保留透明底；子节点 background 仍按其自身渲染）或 `jpg`（无透明通道，节点会合成父级/页面背景，更适合需要保留容器自身背景的场景）
- opts.success：成功回调，`res.path` 为截图保存后的本地文件路径
- opts.fail：失败回调，返回标准 `IUniError` 错误对象
- opts.complete：无论成功失败都会执行

#### getElementShotImage
```ts
type XScreenshotElementOpts = {
  ele?: UniElement | null
  /** 保存格式，默认 png，可选 jpg */
  format?: 'png' | 'jpg'
  success?: (res: XScreenshotResult) => void
  fail?: (res: XScreenshotFail) => void
  complete?: (res: XScreenshotResult | null) => void
}
function getElementShotImage(opts: XScreenshotElementOpts): void
```

**参数说明：**
- opts.ele：要截图的目标元素；为空时 `fail` 错误码 `1003`
- opts.success / opts.fail / opts.complete：同窗口截图


### 示例代码 

```vue
<template>
  <x-sheet id="screentIds">
    <x-button class="mb-16" :block="true" @click="getScreenimg">保存屏幕图片</x-button>
    <x-button :block="true" @click="getSEleimg">保存节点截图</x-button>
  </x-sheet>
</template>

<script setup lang="ts">
import {getRootShotImage, getElementShotImage} from "@/uni_modules/x-screenshot-s"

const getScreenimg = () => {
  getRootShotImage({
    format: 'png',
    success: (res) => {
      uni.previewImage({
        current: res.path,
        urls: [res.path] as string[]
      })
    },
    fail: (err) => {
      console.error(err)
    }
  })
}

const getSEleimg = () => {
  let ele = uni.getElementById("screentIds") as UniElement|null
  getElementShotImage({
    ele: ele,
    success: (res) => {
      uni.previewImage({
        current: res.path,
        urls: [res.path] as string[]
      })
    },
    fail: (err) => {
      console.error(err)
    }
  })
}
</script>
```

### 常见问题

1. 截图黑屏或无内容
   - 确保已正确打包基座
   - 检查目标View是否已完成渲染
   - Android端确保View的宽高不为0
   - 节点内含 `<video>`、TextureView 等视频层时，请升级至 1.0.3+；Android 7.0+ 使用 PixelCopy 合成画面，低版本回退 TextureView 叠加绘制

2. 截图文件路径无效
   - 检查应用是否有存储权限
   - 确保缓存目录可写入

3. 性能注意事项
   - 大尺寸View截图可能占用较多内存
   - 建议在截图后及时释放不需要的图片资源