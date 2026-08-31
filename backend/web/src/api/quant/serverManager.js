export const startServer = (server) => {
  if (window.pywebview && window.pywebview.api) {
    return window.pywebview.api.quant_startClient(JSON.stringify(server))
  }
  return Promise.reject(new Error('非客户端环境无法调用'))
}

export const stopServer = (server) => {
  if (window.pywebview && window.pywebview.api) {
    return window.pywebview.api.quant_stopClient(JSON.stringify(server))
  }
  return Promise.reject(new Error('非客户端环境无法调用'))
}

export const checkServerStatus = (server) => {
  if (window.pywebview && window.pywebview.api) {
    return window.pywebview.api.quant_checkClientStatus(JSON.stringify(server))
  }
  return Promise.resolve({ code: 200, data: { running: false } })
}
