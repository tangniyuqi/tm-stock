import axios from 'axios';
import { ElMessage } from 'element-plus';

const pythonService = axios.create({
  timeout: 5000
});

pythonService.interceptors.response.use(
  response => {
    return response.data;
  },
  error => {
    ElMessage({
      message: error.message || 'Error',
      type: 'error',
      duration: 5 * 1000
    });
    return Promise.reject(error);
  }
);

export const getTasks = (baseUrl) => {
  return pythonService({
    url: `${baseUrl}/tasks`,
    method: 'get'
  });
};

export const startTask = (baseUrl, data) => {
  return pythonService({
    url: `${baseUrl}/task/start`,
    method: 'post',
    data
  });
};

export const stopTask = (baseUrl, taskId) => {
  return pythonService({
    url: `${baseUrl}/task/stop`,
    method: 'post',
    data: {
      task_id: taskId
    }
  });
};

export const getBalance = (baseUrl, taskId) => {
  return pythonService({
    url: `${baseUrl}/task/${taskId}/balance`,
    method: 'get'
  });
};

export const getPosition = (baseUrl, taskId) => {
  return pythonService({
    url: `${baseUrl}/task/${taskId}/position`,
    method: 'get'
  });
};
