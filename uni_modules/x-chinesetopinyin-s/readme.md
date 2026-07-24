# x-chinesetopinyin-s

### 开发文档

将中文转为拼音,带注音和不注音

### 兼容性

| iOS	| Harmony	| 小程序 | Andriod	| WEB	|
| ---	| ---	| ---	| ---	| --- |
| 9+	| 12+	| 全部	| 5.0+ | 全部	|

### 方法说明

使用
```ts

import { toPinYin } from '@/uni_modules/x-chinesetopinyin-s';
toPinYin({
	char: "你好",
	success(res) {
		// 输出：{detail: "ni hao", zhuyin: "nĭ hăo"}
		console.log(res)
	}
})

```

### toPinYin(opts:XuiChineseToPinYinOpts)

```ts

export type XuiChineseToPinYinResult = {
	detail : string,
	zhuyin : string
}

export interface XuiChineseToPinYinFail extends IUniError {
	errCode : number
};

export type XuiChineseToPinYinOpts = {
	/** 待转换的字符串 */
	char : string
	success ?: (res : XuiChineseToPinYinResult) => void
	fail ?: (res : XuiChineseToPinYinFail) => void
	complete ?: (res : XuiChineseToPinYinResult|null) => void
}


```