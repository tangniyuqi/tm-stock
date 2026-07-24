# x-ocr-s

### 开发文档

先在页面上引用使用函数，再去打包基座后，回来再编译到真机上或者模拟器（鸿蒙必须真机）

### 功能特色

支持以下特殊的ocr文本识别。**离线识别，不需要联网**

- 支持中文
- 支持英文
- 支持日文

### 兼容性

| Harmony | IOS | Andriod | WEB |
| --- | --- | --- | --- |
| 鸿蒙Next 5.1+ | 15.50+ | 支持6.0+ | 支持 |


### 方法

**参数opts为类型XOcrOpts，见下面类型注释**

xOcrPare(opts:XOcrOpts)

```ts

export type XOcrResult = {
  /** 文本块，含定位 */
  textBlock : string[],
  /** 文本不含定位 */
  text : string[]
}

export interface XOcrFail extends IUniError {
  errCode : number
};

export type XOcrOpts = {
  /** 图片路径 */
  path : string,
  /** zh,ja */
  langs : string,
  /** 仅安卓支持0-1,默认为0.5，即识别的可信度大于0.5时就为正确识别文本。 */
  zhixingdu ?:number,
  success ?: (res : XOcrResult) => void
  fail ?: (res : XOcrFail) => void
  complete ?: (res : XOcrResult|null) => void
}


```


```vue
	<view class="content">
		<button @click="choosePhoto">相机选择图片</button>
	</view>
		import {xOcrPare} from "@/uni_modules/x-ocr-s"
		const choosePhoto = ()=>{
			uni.chooseImage({
				count:1,
				success(res){
					if(res.tempFilePaths.length==0) return;
					let path = res.tempFilePaths[0]
					ocrPare({
						path,
						langs:'zh',
						success(res) {
							console.log(res)
						},
						fail() {
							
						}
					})
				}
			})
		}



```