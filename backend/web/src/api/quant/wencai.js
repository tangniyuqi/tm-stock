import service from '@/utils/request';

export const queryWencai = async (params, signal) => {
  if (window.pywebview && window.pywebview.api) {
    try {
      // pywebview 环境下，使用 Promise.race 实现取消
      const queryPromise = window.pywebview.api.quant_queryWencai(params);
      
      if (signal) {
        return await Promise.race([
          queryPromise,
          new Promise((_, reject) => {
            signal.addEventListener('abort', () => {
              reject(new DOMException('查询已取消', 'AbortError'));
            });
          })
        ]);
      }
      
      return await queryPromise;
    } catch (error) {
      console.error('pywebview error:', error);
      if (error.name === 'AbortError') {
        throw error;
      }
      return { code: 500, msg: error.message || '查询失败' };
    }
  }

  return service({
    url: '/wencai/query',
    method: 'get',
    params,
    signal,
    timeout: 120000, // 单独配置超时时间为 120 秒
    donNotShowLoading: true, // 禁用全局 loading，避免强制关闭
  });
};
