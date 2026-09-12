import service from '@/utils/request';

// 分页获取短信日志列表
export const getSmsLogList = params => {
  return service({
    url: '/member/smsLog/getSmsLogList',
    method: 'get',
    params,
  });
};

// 删除短信日志
export const deleteSmsLog = params => {
  return service({
    url: '/member/smsLog/deleteSmsLog',
    method: 'delete',
    params,
  });
};

// 批量删除短信日志
export const deleteSmsLogByIds = params => {
  return service({
    url: '/member/smsLog/deleteSmsLogByIds',
    method: 'delete',
    params,
  });
};
