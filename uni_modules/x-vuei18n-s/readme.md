# x-vuei18n-s

一个为 UniApp-X 设计的国际化(i18n)插件，提供完整的多语言支持功能。
本库为tmui4x提供多语言支持，同时也是开源免费的，不需要tmui4x授权，您即可导入和下载使用
使用时请按照文档安装和使用。
使用方式几乎与源Vu8I18n一致基本没有变化，所以你看我下面的文档和看官方的文档都可以,但部分函数还是有差异的
[源官方文档链接](https://vue-i18n.intlify.dev/guide/essentials/started.html)

## 特性

- 🌍 多语言翻译支持
- 🔢 复数形式处理
- 📅 日期时间格式化
- 💰 数字格式化
- ⏰ 相对时间格式化
- 🎯 插值和参数替换
- 🔄 语言回退机制
- 📱 跨平台兼容

| Harmony | IOS | Android | WEB | 小程序 |
| --- | --- | --- | --- | --- |
| 支持 | 支持 | 支持 | 支持 | 支持 |

## 安装

将插件复制到项目的 `uni_modules` 目录下即可。
地区语言代码标准见[打开链接](https://xnxy.github.io/2024/06/11/%E5%9B%BD%E9%99%85%E5%8C%96%E4%B8%AD%E5%B8%B8%E7%94%A8BCP-47%20Code%E5%92%8C%E8%AF%AD%E8%A8%80%E5%AF%B9%E7%85%A7%E8%A1%A8/)

## 在TMUI4x项目中使用

### 前置条件
你已经使用并安装tmui4x组件库，那么会自带集成，但需要你复制本插件到您的项目中，目录
```您的项目 > uni_modules > x-vuei18n-s```
接着安装语言包
```uts
import App from './App'
import {xui} from "@/uni_modules/tmx-ui/index.uts"
import {Tmui4xOptions} from "@/uni_modules/tmx-ui/interface.uts"

import en from "@/uni_modules/tmx-ui/localLanuage/en.json"
import zhHans from "@/uni_modules/tmx-ui/localLanuage/zh-Hans.json"
import zhHant from "@/uni_modules/tmx-ui/localLanuage/zh-Hant.json"
import ko from "@/uni_modules/tmx-ui/localLanuage/ko.json"
import ja from "@/uni_modules/tmx-ui/localLanuage/ja.json"
// 下方是组件的语言包，以及你自己定义的语言包与组件合并即可。共用一个实例。
const messages : UTSJSONObject = {
	"en":{
		...en,
		"hellow":"Hi~"
	},
	"zh-Hans":{
		...zhHans,
		"hellow":"哈喽"
	},
	"zh-Hant":zhHant,
	"ko":ko,
	"ja":ja
}
import { createSSRApp } from 'vue'
export function createApp() {
	const app = createSSRApp(App)
	// 配置语言包
	app.use(xui,{i18nOptions:{locale:"zh-Hans",messages}} as Tmui4xOptions)
	return {
		app
	}
}

```

### 在Uvue内使用

#### 组合式模板内
```uts
import {xStore} from "@/uni_modules/tmx-ui/index.uts"
const i18n = xStore.xConfig.i18n
console.log(i18n.t('xx.x'))
```
uts内导入后可以在模板内使用
```vue
<x-text>{{i18n.t('xx.x')}}</x-text>
```

#### 选项式模板内
它已经集成了,不需要向组合式那样再导入.
```uts
export default {
	methods:{
		test():string{
			return this!.i18n.t('xx.x')
		}
	}
}
```
它已经集成了,不需要向组合式那样再导入.
```vue
<x-text>{{i18n!.t('xx.x')}}</x-text>
```

### 在任意Uts中使用

```uts
import {xStore} from "@/uni_modules/tmx-ui/index.uts"
const i18n = xStore.xConfig.i18n
console.log(i18n.t('xx.x'))
```


## 作为插件集成到您应用中（非tmui4x项目）

### 前置条件
你需要在main.uts中安装插件
```uts
import { tmxI18n} from "@/uni_modules/x-vuei18n-s/index.uts"
import { I18nOptions} from "@/uni_modules/x-vuei18n-s/interface.uts"

const config = {locale:'zh'} as I18nOptions
app.use(tmxI18n,config)

```

**在下面的2和3取得的i18n均为多语言实例，你可以调用前面的实例方法动态添加语言或者切语言等**

### 1.在Vue模板内使用

```vue
	<x-text>{{i18n.t('cnacel')}}</x-text>
```
### 2.在组合式代码内使用

```ts
	import {getCurrentInstance} from "vue"
	const i18n = getCurrentInstance()?.proxy?.$i18n()
	//可选
	console.log(i18n?.t('confirm'))
	//断言
	console.log(i18n!.t('confirm'))
```

### 3.在选项式代码内使用

```ts
	export default {
		methods:{
			test():string{
				return this!.i18n.t('confirm')
			}
		}
	}
	
```

### 4.在任意文件中使用
这里说的是你脱离了vue模板，比如在非setup和选项式中使用时，前面的方式就不行了，因为前面是依赖于vue实例。
此种方式是只使用插件模式不需要在main.uts中app.use安装本插件，即可在任意位置导入使用。包括在vue模板内（不分组合式和选项都能用）
如果你想在模板内使用本方式
**不要再使用i18n变量名了**
因为本身在模板内就有全局的i18n。

任意uts代码中
```uts
import { $i18n } from "@/uni_modules/x-vuei18n-s/index.uts"
const la = $i18n()
// uts文件中就可以使用了。
console.log(la.t('xx.x.x.'))
//模板内

```
vue模板内
```uts
import { $i18n } from "@/uni_modules/x-vuei18n-s/index.uts"
const la = $i18n()
```
```vue
<x-text>{{la.t('xx.x.x.')}}</x-text>
```

## 快速开始

### 基本使用

```typescript
import { $i18n} from "@/uni_modules/x-vuei18n-s/index.uts"
const i18n = $i18n();
```


```typescript
// 设置中文消息
const zhMessages: UTSJSONObject = {
  'hello': '你好',
  'welcome': '欢迎使用UniApp-X',
  'user': {
    'name': '用户名',
    'age': '年龄'
  },
  'items': {
    'apple': '苹果 | 苹果们 | {count}个苹果'
  },
  'greeting': '你好，{name}！'
}

// 设置英文消息
const enMessages: UTSJSONObject = {
  'hello': 'Hello',
  'welcome': 'Welcome to UniApp-X',
  'user': {
    'name': 'Username',
    'age': 'Age'
  },
  'items': {
    'apple': 'apple | apples | {count} of apple'
  },
  'greeting': 'Hello, {name}!'
}
i18n.mergeLocaleMessage('zh',zhMessages)
i18n.mergeLocaleMessage('en',enMessages)
// 设置当前语言
i18n.setLocale('zh')
```

## API 文档

### 创建实例

如果是在uvue模板内，可以参照文档尾部的使用方法。

```typescript
import { $i18n} from "@/uni_modules/x-vuei18n-s/index.uts"
const i18n = $i18n();
```

### 翻译方法 t()

基本翻译：
```typescript
i18n.t('hello') // '你好'
i18n.t('user.name') // '用户名'
```

带参数的翻译：
```typescript
i18n.t('greeting', { name: '张三' }) // '你好，张三！'
```

复数形式：
```typescript
i18n.t('items.apple', 1) // '苹果'
i18n.t('items.apple', 2) // '苹果们'
i18n.t('items.apple', 5, { count: 5 }) // '5个苹果'
```

### 数字格式化 n()

```typescript
// 基本数字格式化
i18n.n(1234.56) // '1234.56'

// 货币格式化
i18n.n(1234.56, null, {
  style: 'currency',
  currency: 'CNY'
}) // '¥1,234.56'

// 百分比格式化
i18n.n(0.85, null, {
  style: 'percent'
}) // '85%'
```

### 日期时间格式化 d()

```typescript
const date = new Date()

// 基本日期格式化
i18n.d(date) // '2024/1/15'

// 自定义格式
i18n.d(date, null, {
  year: 'numeric',
  month: 'long',
  day: 'numeric'
}) // '2024年1月15日'

// 时间格式化
i18n.d(date, null, {
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit'
}) // '14:30:25'
```

### 相对时间格式化 rt()

```typescript
// 相对于当前时间
i18n.rt(new Date()) // '刚刚'

// 指定时间字符串
i18n.rt('2025-7-8 20:4:50', 'second') // '6个月后'

// 时间戳
i18n.rt(Date.now() + 3600000, 'minute') // '1小时后'

// 自动单位进阶
i18n.rt(Date.now() + 90000, 'second') // '1分钟后' (90秒自动转为分钟)
```

### 翻译键存在性检测 te()

```typescript
// 检测翻译键是否存在
i18n.te('hello') // true
i18n.te('user.name') // true
i18n.te('nonexistent.key') // false

// 检测指定语言中的翻译键
i18n.te('hello', 'zh') // true
i18n.te('hello', 'en') // true
i18n.te('hello', 'fr') // false (如果没有法语语言包)

// 检测嵌套键
i18n.te('user.settings.theme') // true
i18n.te('user.nonexistent') // false
```

### 语言管理

```typescript
// 获取当前语言
const currentLocale = i18n.getLocale() // 'zh'

// 设置语言
i18n.setLocale('en')

// 获取可用语言列表
const locales = i18n.availableLocales() // ['zh', 'en']

// 动态添加语言内容
i18n.mergeLocaleMessage('zh', {
  'newKey': '新内容'
})
```

## 高级功能

### 复数规则自定义

```typescript
// pluralRule
const i18n.ops = (choice: number, choicesLength: number) => {
    // 自定义复数规则
    if (choicesLength === 2) {
      return choice === 1 ? 1 : 0
    }
    return choice === 0 ? 0 : choice === 1 ? 1 : 2
  }
}
```

### 回退语言设置

```typescript
i18n.setLocale('zh')
i18n.setFallbackLocale('en')
```

### 预定义格式

```typescript
// 数字格式预定义
i18n.ops.numberFormats.set('zh', new Map([
  ['currency', {
    style: 'currency',
    currency: 'CNY',
    currencyDisplay: 'symbol'
  }]
]))

// 使用预定义格式
i18n.n(1234.56, 'currency') // '¥1,234.56'
```


## 注意事项

1. 消息对象必须使用 `UTSJSONObject` 类型
2. 复数形式使用 `|` 分隔不同的形式
3. 插值参数使用 `{key}` 格式
4. 相对时间会自动选择最合适的单位
5. 支持嵌套的消息键（如 `user.name`）


## 许可证

请不要随意更改源码，你仅有使用权。
