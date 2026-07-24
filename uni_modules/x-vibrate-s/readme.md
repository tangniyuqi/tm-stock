# x-vibrate-s

### 兼容性

| Harmony | IOS | Android | WEB | 小程序 |
| --- | --- | --- | --- | --- |
| 支持 | 支持 | 支持 | 支持 | 支持 |

### 开发文档

鸿蒙使用了：ohos.permission.VIBRATE 权限

调用：
```ts
import { vibrator } from "@/uni_modules/x-vibrate-s"
// 使用,ios13+支持指定频率震动,ios12及以下不支持.
vibrator(50)

```

参数配置：


```uts
/**
 * 震动
 * @param {number} duriation 震动时间单位ms,ios(12及以下)无效
 */
vibrator(duriation : number):boolean 

```
