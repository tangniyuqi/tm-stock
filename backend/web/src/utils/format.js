import { formatTimeToStr } from '@/utils/date'
import { getDict } from '@/utils/dictionary'
import { ref } from 'vue'
import { getUrl } from './image'

export const formatBoolean = (bool) => {
  if (bool !== null) {
    return bool ? '是' : '否'
  } else {
    return ''
  }
}
export const formatDate = (time) => {
  if (time !== null && time !== '') {
    var date = new Date(time)
    return formatTimeToStr(date, 'yyyy-MM-dd hh:mm:ss')
  } else {
    return ''
  }
}

export const filterDict = (value, options) => {
  // 递归查找函数
  const findInOptions = (opts, targetValue) => {
    if (!opts || !Array.isArray(opts)) return null

    for (const item of opts) {
      if (item.value === targetValue) {
        return item
      }

      if (item.children && Array.isArray(item.children)) {
        const found = findInOptions(item.children, targetValue)
        if (found) return found
      }
    }

    return null
  }

  const rowLabel = findInOptions(options, value)
  return rowLabel && rowLabel.label
}

export const filterDataSource = (dataSource, value) => {
  // 递归查找函数
  const findInDataSource = (data, targetValue) => {
    if (!data || !Array.isArray(data)) return null

    for (const item of data) {
      // 检查当前项是否匹配
      if (item.value === targetValue) {
        return item
      }

      // 如果有children属性，递归查找
      if (item.children && Array.isArray(item.children)) {
        const found = findInDataSource(item.children, targetValue)
        if (found) return found
      }
    }

    return null
  }

  if (Array.isArray(value)) {
    return value.map((item) => {
      const rowLabel = findInDataSource(dataSource, item)
      return rowLabel?.label
    })
  }

  const rowLabel = findInDataSource(dataSource, value)
  return rowLabel?.label
}

export const getDictFunc = async (type) => {
  const dicts = await getDict(type)
  return dicts
}

export const ReturnArrImg = (arr) => {
  const imgArr = []
  if (arr instanceof Array) {
    // 如果是数组类型
    for (const arrKey in arr) {
        imgArr.push(getUrl(arr[arrKey]))
    }
  } else {
    imgArr.push(getUrl(arr))
  }
  return imgArr
}

export const returnArrImg = ReturnArrImg

export const onDownloadFile = (url) => {
  window.open(getUrl(url))
}

const baseUrl = ref(import.meta.env.VITE_BASE_API)

export const getBaseUrl = () => {
  return baseUrl.value === '/' ? '' : baseUrl.value
}

export const CreateUUID = () => {
  let d = new Date().getTime()
  if (window.performance && typeof window.performance.now === 'function') {
    d += performance.now()
  }
  return '00000000-0000-0000-0000-000000000000'.replace(/0/g, (c) => {
    const r = (d + Math.random() * 16) % 16 | 0 // d是随机种子
    d = Math.floor(d / 16)
    return (c === '0' ? r : (r & 0x3) | 0x8).toString(16)
  })
}

export const formatSmartNumber = (val) => {
  if (val === null || val === undefined) return '--';

  return Number(val).toLocaleString('zh-CN', {
    // 如果是 1000000.00 -> 显示 1,000,000
    // 如果是 1000000.01 -> 显示 1,000,000.01
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
    useGrouping: true // 是否显示千分位逗号
  });
};

/**
 * 计算目标时间距离当前时间的天数
 * @param {string} targetDateStr - 目标时间字符串（ISO 8601 格式，如 "2025-01-26T23:41:05.469+08:00"）
 * @returns {number} - 返回距离目标时间的天数（整数，可能为负数）
 */
export const getDaysDiff = (dateStr) => {
  const targetDate = new Date(dateStr);
  const currentDate = new Date();
  // 计算时间差（毫秒）
  const timeDiff = currentDate - targetDate;
  // 将时间差转换为天数
  const daysDiff = Math.floor(timeDiff / (1000 * 60 * 60 * 24));

  return daysDiff;
}
