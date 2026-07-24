package uts.sdk.modules.xSseUtsModule

import okhttp3.MediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.Response
import okhttp3.sse.EventSource
import okhttp3.sse.EventSourceListener
import okhttp3.sse.EventSources
import java.util.concurrent.TimeUnit

class SSEClient(
    private val url: String,
    private val headers: Map<String, String> = mapOf(),
    method: String = "GET",
    private val bodyString: String? = null,
    private val bodyBytes: ByteArray? = null,
    private val bodyContentType: String? = null
) {
    private val httpMethod: String = if (method.uppercase() == "POST") "POST" else "GET"
    private var eventSource: EventSource? = null
    private var isConnected = false
    private var cancelledByUser = false
    private var listener: SSEListener? = null

    // 连接状态回调接口
    interface SSEListener {
        fun onOpen()
        fun onMessage(event: String?, data: String)
        fun onError(throwable: Throwable)
        fun onClosed()
    }

    private val client = OkHttpClient.Builder()
        .readTimeout(0, TimeUnit.SECONDS)  // SSE 需要禁用读取超时
        .retryOnConnectionFailure(true)
        .build()

    fun setListener(listener: SSEListener) {
        this.listener = listener
    }

    private fun buildPostBody(): RequestBody {
        val mediaType = MediaType.parse(
            bodyContentType ?: "application/json; charset=utf-8"
        )
        return when {
            bodyBytes != null -> RequestBody.create(mediaType, bodyBytes)
            !bodyString.isNullOrEmpty() -> RequestBody.create(
                mediaType,
                bodyString!!.toByteArray(Charsets.UTF_8)
            )
            else -> RequestBody.create(null, ByteArray(0))
        }
    }

    fun connect() {
        if (isConnected) return
        cancelledByUser = false

        val requestBuilder = Request.Builder()
            .url(url)
            .header("Accept", "text/event-stream")
            .header("Cache-Control", "no-cache")

        headers.forEach { (key, value) ->
            requestBuilder.header(key, value)
        }

        if (httpMethod == "POST") {
            requestBuilder.post(buildPostBody())
        }

        val builtRequest = requestBuilder.build()

        eventSource = EventSources.createFactory(client)
            .newEventSource(builtRequest, object : EventSourceListener() {
                override fun onOpen(eventSource: EventSource, response: Response) {
                    isConnected = true
                    listener?.onOpen()
                }

                override fun onEvent(
                    eventSource: EventSource,
                    id: String?,
                    type: String?,
                    data: String
                ) {
                    listener?.onMessage(type, data)
                }

                override fun onClosed(eventSource: EventSource) {
                    isConnected = false
                    listener?.onClosed()
                }

                override fun onFailure(
                    eventSource: EventSource,
                    t: Throwable?,
                    response: Response?
                ) {
                    isConnected = false
                    // cancel() 会走 onFailure 而非 onClosed，与 iOS NSURLErrorCancelled 同理需忽略
                    if (cancelledByUser) {
                        cancelledByUser = false
                        return
                    }
                    t?.let { listener?.onError(it) }
                }
            })
    }

    fun disconnect() {
        if (eventSource == null) return
        cancelledByUser = true
        listener = null
        val source = eventSource
        eventSource = null
        isConnected = false
        source?.cancel()
    }

    fun isConnected(): Boolean = isConnected
}