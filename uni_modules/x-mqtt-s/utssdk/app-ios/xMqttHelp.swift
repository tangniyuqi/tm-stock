import Foundation
import Security
import CocoaMQTT
import DCloudUTSFoundation

// 证书加载工具预计由同目录的 xMqttCerHelp.swift 提供
// getClientCertFromP12File(resourcePath: String, certPassword: String) -> [String: Any]?
// getClientCertFromCrtFile(resourcePath: String) -> [String: Any]?

@objc public class XMqttHelp: NSObject {

	@objc public enum ConnectStatus: Int {
		case wait
		case opening
		case open
		case error
		case dissconnect
		case message
	}

	public typealias EventType = String // 'open' | 'error' | 'dissconnect' | 'message'
	public typealias MQTTEventCallback = @convention(block) (_ type: EventType, _ topic: String?, _ payload: String) -> Void
	public typealias VoidCallback = @convention(block) () -> Void

	private var mqtt: CocoaMQTT?
	@objc public private(set) var connectStatus: ConnectStatus = .wait
	private var anyStateCallback: VoidCallback?

	private struct EventItem {
		let type: EventType
		let callback: MQTTEventCallback
	}
	private var events = [String: EventItem]()
	private var pendingSubs = [(topic: String, qos: CocoaMQTTQoS)]()

	@objc public override init() {
		super.init()
	}

	// opts keys 对齐 index.uts: protocol, server, port, path, clientId, userName, passWord, keepAliveInterval, reconnect, useSSL, certName, certPassword
	@objc public func create(_ opts: NSDictionary) -> XMqttHelp {
		let proto = ((opts["protocol"] as? String) ?? "wss").lowercased() // ws | wss
		let server = (opts["server"] as? String) ?? ""
		let port = (opts["port"] as? NSNumber)?.uint16Value ?? 0
		let path = (opts["path"] as? String) ?? "/mqtt"
		let clientId = (opts["clientId"] as? String) ?? ("iOS-" + UUID().uuidString)
		let userName = opts["userName"] as? String
		let passWord = opts["passWord"] as? String
		let keepAlive = (opts["keepAliveInterval"] as? NSNumber)?.uint16Value ?? 60
		let reconnect = (opts["reconnect"] as? NSNumber)?.boolValue ?? true
		var useSSL = (opts["useSSL"] as? NSNumber)?.boolValue ?? (proto == "wss")
		let certName = opts["certName"] as? String
		let certPassword = opts["certPassword"] as? String
		let allowUntrust = (opts["allowUntrustCACertificate"] as? NSNumber)?.boolValue ?? false
		var headers = opts["headers"] as? [String: String] ?? [:]
		let subProtocols = opts["protocols"] as? [String] ?? ["mqtt"]
		let connectTimeoutMs = (opts["connectTimeoutMs"] as? NSNumber)?.intValue ?? 30000

		let websocket = CocoaMQTTWebSocket(uri: path)
		// CocoaMQTTWebSocket 会自己根据 enableSSL 拼接 ws/wss URL。
		// 只设置 mqtt.enableSSL 不足以让 WebSocket 切到 wss。
		websocket.enableSSL = useSSL
		// 保留调用方透传的自定义 headers，避免覆盖系统自动生成的握手头
		// （Host / Upgrade / Connection / Sec-WebSocket-*），否则可能导致 wss 握手卡住。
		if !subProtocols.isEmpty {
			console.log("[XMqttHelp] websocket protocols => \(subProtocols)")
		}
		websocket.headers = headers
		let mqtt = CocoaMQTT(clientID: clientId, host: server, port: port, socket: websocket)
		mqtt.username = userName
		mqtt.password = passWord
		mqtt.keepAlive = keepAlive
		mqtt.enableSSL = useSSL
		mqtt.autoReconnect = reconnect
		mqtt.cleanSession = true
		mqtt.willMessage = CocoaMQTTMessage(topic: "/will", string: "dieout")

		console.log("[XMqttHelp] connect url => \(proto)://\(server):\(port)\(path), mqttSSL=\(useSSL), wsSSL=\(websocket.enableSSL), headers=\(headers)")

		// 证书策略（可选）
		if useSSL, let name = certName, !name.isEmpty {
			if name.lowercased().hasSuffix(".p12") {
				if let pwd = certPassword, !pwd.isEmpty {
					let base = (name as NSString).deletingPathExtension
					if let path = Bundle.main.path(forResource: base, ofType: "p12"),
					   let settings = getClientCertFromP12File(resourcePath: path, certPassword: pwd) {
						mqtt.sslSettings = settings
					} else {
						console.log("[XMqttHelp] p12 证书无效或找不到: \(name)")
					}
				} else {
					console.log("[XMqttHelp] p12 证书未提供密码")
				}
			} else if name.lowercased().hasSuffix(".crt") {
				let base = (name as NSString).deletingPathExtension
				if let path = Bundle.main.path(forResource: base, ofType: "crt"),
				   let settings = getClientCertFromCrtFile(resourcePath: path) {
					mqtt.sslSettings = settings
				} else {
					console.log("[XMqttHelp] crt 证书无效或找不到: \(name)")
				}
			}
		}

		// 事件回调
		weak var weakSelf = self

		// 自签证书信任（调试可开，生产建议禁用或使用证书绑定）
		mqtt.allowUntrustCACertificate = allowUntrust
		mqtt.didReceiveTrust = { [weak weakSelf] _, trust, completionHandler in
			guard let self = weakSelf else {
				completionHandler(false)
				return
			}
			let trusted = self.evaluateServerTrust(trust, host: server, allowUntrust: allowUntrust)
			console.log("[XMqttHelp] didReceiveTrust => host=\(server), allowUntrust=\(allowUntrust), trusted=\(trusted)")
			completionHandler(trusted)
		}

		// 指定 SNI 主机名，避免非标准端口下的握手失败
		if useSSL {
			var ssl: [String: NSObject] = [:]
			if let existing = mqtt.sslSettings as? [String: NSObject] {
				ssl = existing
			}
			ssl[kCFStreamSSLPeerName as String] = server as NSString
			mqtt.sslSettings = ssl
		}
		mqtt.didDisconnect = { [weak weakSelf] _, error in
			guard let self = weakSelf else { return }
			self.connectStatus = .error
			self.notifyStateChange()
			let errText = error.map { String(describing: $0) } ?? "连接已关闭"
			console.log("[XMqttHelp] didDisconnect error => \(errText)")
			self.buildCallEvents(type: "error", topic: nil, payload: errText)
		}
		mqtt.didConnectAck = { [weak weakSelf] _, _ in
			guard let self = weakSelf else { return }
			self.connectStatus = .open
			self.notifyStateChange()
			self.buildCallEvents(type: "open", topic: nil, payload: "已连接")
			// 补发未完成的订阅
			if !self.pendingSubs.isEmpty {
				for item in self.pendingSubs {
					self.mqtt?.subscribe(item.topic, qos: item.qos)
					console.log("[XMqttHelp] re-subscribe => \(item.topic) qos=\(item.qos.rawValue)")
				}
				self.pendingSubs.removeAll()
			}
		}
		mqtt.didChangeState = { [weak weakSelf] _, state in
			guard let self = weakSelf else { return }
			switch state {
			case .connecting:
				console.log("[XMqttHelp] 连接中")
			case .connected:
				console.log("[XMqttHelp] 连接成功")
			case .disconnected:
				self.connectStatus = .dissconnect
				self.notifyStateChange()
				self.buildCallEvents(type: "dissconnect", topic: nil, payload: "已断开连接")
			default:
				break
			}
		}
		mqtt.didReceiveMessage = { [weak weakSelf] _, message, _ in
			guard let self = weakSelf else { return }
			self.connectStatus = .message
			self.notifyStateChange()
			let payload = message.string ?? (String(data: Data(message.payload), encoding: .utf8) ?? "")
			self.buildCallEvents(type: "message", topic: message.topic, payload: payload)
		}

		self.mqtt = mqtt
		self.connectStatus = .wait
		self.notifyStateChange()

		// 连接超时保护
		if connectTimeoutMs > 0 {
			let deadline = DispatchTime.now() + .milliseconds(connectTimeoutMs)
			DispatchQueue.main.asyncAfter(deadline: deadline) { [weak self] in
				guard let self = self else { return }
				if self.connectStatus == .opening {
					console.log("[XMqttHelp] 连接超时: \(connectTimeoutMs)ms")
					_ = self.disconnect()
					self.connectStatus = .error
					self.notifyStateChange()
					self.buildCallEvents(type: "error", topic: nil, payload: "连接超时")
				}
			}
		}
		return self
	}

	@objc public func connect() -> XMqttHelp {
		guard let mqtt = self.mqtt else { return self }
		self.connectStatus = .opening
		self.notifyStateChange()
		mqtt.connect()
		return self
	}

	@objc public func publish(_ topic: String, message: String, qos: Int, retained: Bool, completion: @escaping @convention(block) (_ ok: Bool) -> Void) -> XMqttHelp {
		guard let mqtt = self.mqtt else { completion(false); return self }
		let q: CocoaMQTTQoS = (qos == 2) ? .qos2 : ((qos == 1) ? .qos1 : .qos0)
		let msg = CocoaMQTTMessage(topic: topic, string: message, qos: q, retained: retained)
		mqtt.publish(msg)
		completion(true)
		return self
	}

	@objc public func subscribe(_ topics: [NSDictionary]) -> XMqttHelp {
		guard let mqtt = self.mqtt else { return self }
		for item in topics {
			guard let topic = item["topic"] as? String else { continue }
			let qosVal = (item["qos"] as? NSNumber)?.intValue ?? 0
			let q: CocoaMQTTQoS = (qosVal == 2) ? .qos2 : ((qosVal == 1) ? .qos1 : .qos0)
			console.log(self.connectStatus == .open,"opeing status")
			if self.connectStatus == .open {
				mqtt.subscribe(topic, qos: q)
				console.log("[XMqttHelp] subscribe => \(topic) qos=\(q.rawValue)")
			} else {
				self.pendingSubs.append((topic, q))
				console.log("[XMqttHelp] cache subscribe (not open) => \(topic) qos=\(q.rawValue)")
			}
		}
		return self
	}

	@objc public func unsubscribe(_ topics: [String]) -> XMqttHelp {
		guard let mqtt = self.mqtt else { return self }
		for t in topics { mqtt.unsubscribe(t) }
		return self
	}

	@objc public func disconnect() -> XMqttHelp {
		guard let mqtt = self.mqtt else { return self }
		mqtt.disconnect()
		return self
	}

	// 注册任意状态变化的回调（无参数）
	@objc public func setCallBack(_ cb: @escaping VoidCallback) {
		self.anyStateCallback = cb
	}


	// 规范命名，提供同等功能（可选使用）
	@objc public func getStatus() -> String {
		return self.statusString(self.connectStatus)
	}

	// 事件系统：与 index.uts 的 addEventListener/removeEventListener 对齐
	@objc @discardableResult
	public func addEventListener(_ type: EventType, callback: @escaping MQTTEventCallback) -> String {
		let id = String(format: "%.0f-%d", Date().timeIntervalSince1970 * 1000, Int.random(in: 0...9999))
		events[id] = EventItem(type: type, callback: callback)
		return id
	}

	@objc @discardableResult
	public func removeEventListener(_ id: String) -> XMqttHelp {
		events.removeValue(forKey: id)
		return self
	}

	private func buildCallEvents(type: EventType, topic: String?, payload: String) {
		for (_, item) in events where item.type == type {
			item.callback(type, topic, payload)
		}
	}

	private func notifyStateChange() {
		anyStateCallback?()
	}

	private func evaluateServerTrust(_ trust: SecTrust, host: String, allowUntrust: Bool) -> Bool {
		if allowUntrust {
			return true
		}

		if #available(iOS 13.0, *) {
			var error: CFError?
			let trusted = SecTrustEvaluateWithError(trust, &error)
			if let error = error {
				console.log("[XMqttHelp] SecTrustEvaluateWithError => \(error)")
			}
			return trusted
		}

		var result = SecTrustResultType.invalid
		let status = SecTrustEvaluate(trust, &result)
		if status != errSecSuccess {
			console.log("[XMqttHelp] SecTrustEvaluate status => \(status)")
			return false
		}

		switch result {
		case .unspecified, .proceed:
			return true
		default:
			return false
		}
	}

	private func statusString(_ status: ConnectStatus) -> String {
		switch status {
		case .wait:
			return "wait"
		case .opening:
			return "opening"
		case .open:
			return "open"
		case .error:
			return "error"
		case .dissconnect:
			return "dissconnect"
		case .message:
			return "message"
		}
	}
}


