import moment from 'moment';

// 默认的日期时间格式
const DATE_TIME_FORMAT = 'YYYY-MM-DD HH:mm:ss';

/**
 * 获取 moment 对象
 * @param {string|Date} date 输入的日期，默认为当前时间
 * @returns {moment.Moment} moment 对象
 */
function getMoment(date = new Date()) {
  return moment(date);
}

/**
 * 将日期格式化为指定的字符串格式
 * @param {string|Date|number} date 输入的日期，默认为当前时间
 * @param {string} format 输出的日期格式，默认为 'YYYY-MM-DD HH:mm:ss'
 * @returns {string} 格式化后的日期字符串
 */
export function formatToDateTime(date = new Date(), format = DATE_TIME_FORMAT) {
  // 如果输入为 0 或无效值，直接返回''
  if (!date || date == 0) {
    return '';
  }

  // 如果是数字类型的时间戳，判断是秒还是毫秒
  // 秒级时间戳通常小于 10000000000 (对应 2286-11-20)
  // 毫秒级时间戳通常大于 10000000000
  if (typeof date === 'number' && date < 10000000000) {
    // 秒级时间戳，转换为毫秒
    date = date * 1000;
  }

  return getMoment(date).format(format);
}

/**
 * 返回输入日期的时间戳（以秒为单位）
 * @param {string|Date} date 输入的日期，默认为当前时间
 * @returns {number} 时间戳（秒数）
 */
export function getTimestampInSeconds(date = new Date()) {
  return Math.floor(getMoment(date).valueOf() / 1000);
}

/**
 * 返回当天开始的时间戳（以秒为单位）
 * @returns {number} 当天开始的时间戳（秒数）
 */
export function getStartOfDayTimestamp() {
  return Math.floor(getMoment().startOf('day').valueOf() / 1000);
}

/**
 * 计算两个日期之间的差值
 * @param {string|Date} date1 第一个日期
 * @param {string|Date} date2 第二个日期
 * @param {string} unit 差值的单位（如 'days', 'seconds' 等），默认为 'days'
 * @returns {number} 两个日期之间的差值
 */
export function getDateDifference(date1, date2, unit = 'days') {
  return getMoment(date1).diff(getMoment(date2), unit);
}

/**
 * 计算指定时间的倒计时剩余天数
 * @param {string|Date} targetDate 指定的目标时间
 * @param {number} countdownDays 倒计时的总天数，默认为 15 天
 * @returns {number} 剩余天数（如果目标时间已过，则返回 0）
 */
export function getCountdownDays(targetDate, countdownDays = 15) {
  // 获取当前时间
  const now = moment();
  // 获取目标时间
  const target = moment(targetDate);
  // 计算目标时间与当前时间的天数差
  const diffDays = target.diff(now, 'days');

  // 如果目标时间已过，返回 0
  if (diffDays < 0) {
    return 0;
  }

  // 计算剩余天数
  const remainingDays = Math.max(countdownDays - diffDays, 0);
  return remainingDays;
}

/**
 * 计算两个日期之间的秒数差
 * @param {string|Date} endDate 结束日期
 * @param {string|Date} startDate 开始日期，默认为当前时间
 * @returns {number} 两个日期之间的秒数差，如果结束日期早于开始日期，则返回 0
 */
export function getSecondsDifference(endDate, startDate = new Date()) {
  const startTimestamp = getTimestampInSeconds(startDate);
  const endTimestamp = getTimestampInSeconds(endDate);
  return Math.max(endTimestamp - startTimestamp, 0);
}

// 导出 moment 对象，方便在其他地方使用 moment.js 的功能
export const datetime = moment;
