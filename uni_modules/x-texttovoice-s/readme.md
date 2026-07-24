# x-texttovoice-s

### 开发文档

将文字转为语音播报。

### 兼容性

**注意：Harmony需要使用模拟器6.0+(api 20+)才支持在模拟器上使用本插件,低于此版本需要真机调试。**

| iOS	| Harmony	| 小程序 | Andriod	| WEB	|
| ---	| ---	| ---	| ---	| --- |
| 12+	| 12+	| x	| 5.0+ | 全部	|

### 方法说明

使用
```ts

import { XTtsSpeek, xTtsImplIns } from '@/uni_modules/x-texttovoice-s';
const word = ref("我是待播放的语音文本。")
const onclick = () => {
	XTtsSpeek({
		fail(res) {
			console.error(res)
		},
		onDone() {
			console.log('播放完毕')
		},
		onStop() {
			console.log('stop')
		},
		success(res) {
			// 初始成功后，调用播放功能。
			xTtsImplIns.play(word.value)
		}
	})
}

```

### XTtsSpeek(opts:XuiTextToVoiceOpts)
引擎初始方法
```ts

export type XuiTextToVoiceResult = {

}

export interface XuiTextToVoiceFail extends IUniError {
  errCode : number
};

export type XuiTextToVoiceOpts = {
  onStart ?:() => void
  onStop ?:() => void
  onError ?:(res : XuiTextToVoiceFail) => void
  /** 播放完成 */
  onDone ?:() => void
  /** 函数执行初始化成功，然后就可以播放，停止等操作了。 */
  success ?: (res : XTTSImpl) => void
  /** 函数初始化失败 */
  fail ?: (res : XuiTextToVoiceFail) => void
  /** 不管初始失败还是成功都执行。 */
  complete ?: (res : XuiTextToVoiceResult|null) => void
}





```

### xTtsImplIns
语音播放静态类方法。

#### isPlaying(): boolean; 获取当前状态
#### play(c:string); 播播文本语音
#### stop(); 停止播报

```ts
import { XTtsSpeek, xTtsImplIns } from '@/uni_modules/x-texttovoice-s';
// xTtsImplIns就是实例 XTTSImpl，含有以下方法。
export  interface XTTSImpl {
	isPlaying(): boolean;
	play(c:string):void
	stop():void
}

```