# x-sse-s
### 开发文档
用于连接SSE服务器

### 兼容性

| Harmony | IOS | Android | WEB | 小程序 |
| --- | --- | --- | --- | --- |
| 支持 | 支持 | 支持 | 支持 | x |

### 注意事项

使用前一定要打基座才可用，一定要在页面上先引用，再去打基座。
如果你mac开发。ios可以不用打基座，能直接使用（但前提是你要配置好原生开发环境，否则一样要打包）
如果你是开始安卓，不管是mac,win电脑都要打包基座才能使用。

### SSEClientApp 实例

鸿蒙需要：ohos.permission.INTERNET权限

```ts
// 群主提供一个测试服务器连接,随时关闭,仅用于测试连接.
// https://tmui.design:8000/sse
// header是可选的
// method可选，默认为get，也可以设置为post
const sse:SSEClientApp = new SSEClientApp({url:"你的sse服务器",header:{},method:'post,get'});
const sse:SSEClientApp = new SSEClientApp({url:"你的sse服务器"});

```

### SSEClientApp 事件
**打开时触发**
onOpen(callback : () => void)
**出错(安卓和ios关闭时也会触发)触发**
onError(callback : () => void)
**关闭时触发**
onClosed(callback : () => void)
**打开时触发**
onMessage(callback : (data:string) => void)

### SSEClientApp 方法
**连接**
connect()
**关闭**
disconnect()
**返回当前连接状态**
isConnected()
**当请求类型为post时可以通过本方法在连接前设置请求体数据发送给服务器**
setBody(bodyStr:string|ArrayBuffer|UTSJSONObject)
### 代码示例

```ts
import {SSEClientApp}  from '@/uni_modules/x-sse-s';
const sse = new SSEClientApp({
	method:'post', // post,get,此参数1.0.3开始支持,此版本前默认是get
	url:"https://testsse.element-ui-x.com/api/sse",
	header:{"token":"token","password":"password"}
})

//收到消息
sse.onMessage((msg:string)=>{
	console.log("SSE", `收到消息:  data=${msg}`)
})

//连接
const concet = ()=>{
	// 如果为method = post时可以执行设置body体
	// sse.setBody({} as UTSJSONObject)
	sse.connect()
}
//断开
const disconcet = ()=>{
	sse.disconnect()
}


```