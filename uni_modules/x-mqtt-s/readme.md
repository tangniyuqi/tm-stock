# x-mqtt-s
### 开发文档
[TMUI4.0文档](https://xui.tmui.design/)
[TMUI4.0组件库](https://ext.dcloud.net.cn/plugin?id=16369)

用于mqtt连接，推送，订阅消息的插件。

安卓：适于用tcp,ssl,ws,wssl等几乎所有协议
ios:仅适配了webstock mqtt,ws协议
web:适配webstock mqtt,ws,wss协议
微信端:wss协议，且服务器不能为mqtt服务需要是ws服务，因此可能需要你在服务器将Mqtt转换为ws

### 微信使用说明
微信不支持websocket连接以Mqtt为服务的服务器，你需要在服务器上转换


### 说明
这是tmui4.0|XUI的原生插件附赠插件

### 兼容性

| Harmony | IOS | Android | WEB | 小程序 |
| --- | --- | --- | --- | --- |
| 支持 | 支持 | 支持 | 支持 | 支持 |

### 类型参考

```vue
<template>
	<x-sheet :margin="['0']">
		<view class="mb-12"><x-text>收到的消息:{{message}}</x-text></view>
		<x-text>
			{{connecting==0?'待连接':''}}
			{{connecting==1?'连接中':''}}
			{{connecting==3?'断开连接':''}}
			{{connecting==2?'连接成功':''}}
		</x-text>
		<x-button class="mb-12 mt-12" :disabled="isCreate" @click="create" :block="true">创建服务对象</x-button>
		<x-button class="mb-12" :disabled="(connecting==1||connecting==2)||!isCreate" @click="connect" :block="true">连接服务器</x-button>
		<x-button class="mb-12" :disabled="connecting!=2" @click="subscribe" :block="true">订阅消息</x-button>
		<x-button class="mb-12" :disabled="connecting!=2" @click="publish" :block="true">推送消息</x-button>
		<x-button :disabled="connecting!=2" @click="disconnect" :block="true">断开</x-button>
	</x-sheet>
</template>

<script setup>
	import {
		xMqtt,
		CONNECT_STATUS,
		MQTT_EVENT_TYPE,
		MQTT_EVENT_CALL,
		MQTT_EVENT_PUBLISH,
		MQTT_SUBSCRIBE,
		MQTT_PUBLISH_TOPIC,
		MQTT_CONNECT_OPTS
	} from '@/uni_modules/x-mqtt-s';
	const connecting = ref(0)
	const message = ref("")
	const mqtt = new xMqtt()
	const isCreate = ref(false)
	const create = () => {
		mqtt.create({
			// web，ios端不会起作用，由软件根据useSSL来判断是ws,wss,app需要指定是ws://,wss://,ssl://等
			protocol: 'wss://',
			// 连接的路径，如果没有就空值，有就填写比如:'/mqtt'
			path: '/mqtt',
			// 客户端id
			clientId: 'tmui4x-'+Date.now().toString(),
			// 服务器地址
			server: "i90e06e9.ala.cn-hangzhou.emqxsl.cn",
			// 服务器端口
			port: 8084,
			// 用户名，没有为空
			userName: 'tmui4x',
			// 用户密码，没有为空
			passWord: '123456',
			// 是否使用加密连接，web端true时为wss,否则为ws协议
			useSSL: true,
			// 保持消息跳动的间隔
			keepAliveInterval: 60,
			// 连接超时时间
			timeout: 30,
			// 是否自动重连
			reconnect: true
		})
		
		mqtt.addEventListener('open', (type : MQTT_EVENT_TYPE, topic : string | null, str : string) => {
			connecting.value = 2;
		})
		mqtt.addEventListener('message', (type : MQTT_EVENT_TYPE, topic : string | null, str : string) => {
			console.log(topic, str)
			message.value = `主题:${topic},内容:${str}`
		})
		mqtt.addEventListener('dissconnect', (type : MQTT_EVENT_TYPE, topic : string | null, str : string) => {
			connecting.value = 3;
		})
		mqtt.addEventListener('error', (type : MQTT_EVENT_TYPE, topic : string | null, str : string) => {
			connecting.value = 3;
		})
		
		isCreate.value = true;
	}
	const connect = ()=>{
		if(connecting.value!=3&&connecting.value!=0) return;
		connecting.value = 1;
		mqtt.connect()
	}
	const subscribe = ()=>{
		mqtt.subscribe([{
			topic: "xui/hi",
			qos: 1
		} as MQTT_SUBSCRIBE] as MQTT_SUBSCRIBE[])
	}
	const publish = ()=>{
		mqtt.publish({
				topic: "xui/hi",
				qos: 0,
				message: (Math.random()*100).toFixed(2)+"-消息内容" ,
				retained: false
			} as MQTT_PUBLISH_TOPIC, (_ : boolean) => { })
	}
	const disconnect = ()=>{
		connecting.value = 3
		mqtt.disconnect()
	}
</script>



```
