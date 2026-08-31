// 联网搜索引擎选项（chat/completions 接口模式下 AI 联网检索可选；与 config.yaml 的 web-search 配置节保持一致）
// baidu 为默认引擎（未指定时服务端默认使用百度搜索，需在 config.yaml 的 web-search.baidu 配置 api-key）
export const webSearchOptions = [
  {
    value: 'baidu',
    label: '百度',
    disabled: false,
  },
  {
    value: 'bocha',
    label: '博查AI',
    disabled: false,
  },
  {
    value: 'azure',
    label: 'Azure',
    disabled: true,
  },
  {
    value: 'tavily',
    label: 'Tavily',
    disabled: true,
  },
  {
    value: 'serper',
    label: 'Serper',
    disabled: true,
  },
];
