import Foundation

class SSEClient: NSObject {
    private var url: URL
    private var headers: [String: String]
    private var httpMethod: String
    private var httpBody: Data?
    private var eventSourceTask: URLSessionDataTask?
    
    // 回调
    private var onMessageCallback: ((String) -> Void)?
    private var onOpenCallback: (() -> Void)?
    private var onErrorCallback: ((Error?) -> Void)?
    
    // 【关键修复】添加缓冲区，用于拼接不完整的 TCP 数据包
    private var buffer = ""
    
    init(url: URL, headers: [String: String] = [:], method: String = "GET", httpBody: Data? = nil) {
        self.url = url
        self.headers = headers
        self.httpMethod = method.uppercased() == "POST" ? "POST" : "GET"
        self.httpBody = httpBody
        super.init()
    }

    func onMessage(_ callback: @escaping (String) -> Void) {
        onMessageCallback = callback
    }

    func onOpen(_ callback: @escaping () -> Void) {
        onOpenCallback = callback
    }

    func onError(_ callback: @escaping (Error?) -> Void) {
        onErrorCallback = callback
    }

    func start() {
        // 建议同时加上之前的超时配置，以防万一
        let configuration = URLSessionConfiguration.default
        configuration.timeoutIntervalForRequest = 300
        configuration.timeoutIntervalForResource = TimeInterval.greatestFiniteMagnitude
        
        let session = URLSession(configuration: configuration, delegate: self, delegateQueue: nil)
        
        var request = URLRequest(url: url)
        request.httpMethod = httpMethod
        request.addValue("text/event-stream", forHTTPHeaderField: "Accept")
        request.addValue("no-cache", forHTTPHeaderField: "Cache-Control")
        // 某些服务器需要指定 Last-Event-ID 或 Connection，视情况而定
        // request.addValue("keep-alive", forHTTPHeaderField: "Connection")
        
        for (key, value) in headers {
            request.addValue(value, forHTTPHeaderField: key)
        }

        if httpMethod == "POST", let body = httpBody {
            request.httpBody = body
        }
        
        eventSourceTask = session.dataTask(with: request)
        eventSourceTask?.resume()
    }

    func stop() {
        eventSourceTask?.cancel()
        eventSourceTask = nil
        buffer = "" // 清空缓冲
    }
    
    // 【关键修复】解析缓冲区的完整消息
    private func processBuffer() {
        // SSE 消息以 "\n\n" 结尾
        let separator = "\n\n"
        
        while buffer.contains(separator) {
            // 找到第一个完整消息的结束位置
            if let range = buffer.range(of: separator) {
                let messageBlock = String(buffer[..<range.lowerBound])
                // 移除已处理的消息块（包括分隔符）
                buffer.removeSubrange(..<range.upperBound)
                
                parseAndEmit(messageBlock: messageBlock)
            } else {
                break
            }
        }
    }
    
    // 解析单个消息块 (可能包含多行，如 id:, event:, data:)
    private func parseAndEmit(messageBlock: String) {
        let lines = messageBlock.components(separatedBy: .newlines)
        var finalData = ""
        
        for line in lines {
            // 跳过空行或注释
            if line.isEmpty || line.hasPrefix(":") {
                continue
            }
            
            if line.hasPrefix("data: ") {
                let content = String(line.dropFirst(6)) // 去掉 "data: "
                // 如果有多行 data，SSE 规定要用换行符连接
                if finalData.isEmpty {
                    finalData = content
                } else {
                    finalData += "\n" + content
                }
            } else if line.hasPrefix("data:") {
                // 处理 "data:" 后面直接跟内容没有空格的情况 (虽然标准推荐有空格)
                let content = String(line.dropFirst(5))
                if finalData.isEmpty {
                    finalData = content
                } else {
                    finalData += "\n" + content
                }
            }
            // 这里可以扩展解析 event: 或 id:
        }
        
        if !finalData.isEmpty {
            onMessageCallback?(finalData)
        }
    }
}

extension SSEClient: URLSessionDataDelegate {
    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
        guard let string = String(data: data, encoding: .utf8) else {
            return
        }
        
        // 将新收到的数据追加到缓冲区
        buffer += string
        
        // 尝试处理缓冲区中的完整消息
        processBuffer()
    }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive response: URLResponse, completionHandler: @escaping (URLSession.ResponseDisposition) -> Void) {
        // 确认是 SSE 流
        if let mimeType = response.mimeType, mimeType == "text/event-stream" {
            completionHandler(.allow)
            onOpenCallback?()
        } else {
            // 如果服务器返回的不是 SSE，可能需要报错或允许（视兼容性需求）
            completionHandler(.allow)
            onOpenCallback?()
        }
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
        if let error = error {
            // 如果是取消操作，不视为错误
            if let nsError = error as? NSError, nsError.code == NSURLErrorCancelled {
                return
            }
            onErrorCallback?(error)
        }
        // 连接断开，清空缓冲
        buffer = ""
    }
}