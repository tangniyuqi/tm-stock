// 大模型厂商及模型选项（与 config.yaml 的 ai.providers 保持一致）
export const providerOptions = [
  {
    value: 'deepseek',
    label: 'DeepSeek（深度求索）',
    disabled: false,
    models: [
      { value: 'deepseek-v4-flash', label: 'DeepSeek V4 Flash' },
      { value: 'deepseek-v4-pro', label: 'DeepSeek V4 Pro' },
    ],
  },
  {
    value: 'siliconflow',
    label: 'SiliconFlow（硅基流动）',
    disabled: false,
    models: [
      { value: 'deepseek-ai/DeepSeek-V4-Flash', label: 'DeepSeek V4 Flash' },
      { value: 'deepseek-ai/DeepSeek-V4-Pro', label: 'DeepSeek V4 Pro' },
    ],
  },
  {
    value: 'doubao',
    label: 'Doubao（豆包）',
    models: [
      { value: 'doubao-seed-2-0-mini-260428', label: 'Doubao Seed 2.0 Mini' },
      { value: 'doubao-seed-2-0-lite-260428', label: 'Doubao Seed 2.0 Lite' },
      { value: 'doubao-seed-2-0-pro-260428', label: 'Doubao Seed 2.0 Pro' },
      { value: 'doubao-seed-2-1-turbo-260628', label: 'Doubao Seed 2.1 Turbo' },
      { value: 'doubao-seed-2-1-pro-260628', label: 'Doubao Seed 2.1 Pro' },
    ],
  },
  {
    value: 'qwen',
    label: 'Qwen（阿里千问）',
    disabled: false,
    models: [
      { value: 'qwen3.7-flash', label: 'Qwen3.7 Flash' },
      { value: 'qwen3.7-plus', label: 'Qwen3.7 Plus' },
      { value: 'qwen3.7-max', label: 'Qwen3.7 Max' },
      { value: 'qwen3.8-max', label: 'Qwen3.8 Max' },
      { value: 'deepseek-v4-flash-0731', label: 'DeepSeek V4 Flash 2026-07-31' },
      { value: 'deepseek-v4-pro-0813', label: 'DeepSeek V4 Pro 2026-08-13' },
    ],
  },
  {
    value: 'kimi',
    label: 'Kimi（月之暗面）',
    disabled: true,
    models: [
      { value: 'kimi-k3', label: 'Kimi K3' },
      { value: 'kimi-k2.7-code', label: 'Kimi K2.7 Code' },
      { value: 'kimi-k2.6', label: 'Kimi K2.6' },
    ],
  },
  {
    value: 'zhipu',
    label: 'Zhipu（智谱）',
    disabled: true,
    models: [
      { value: 'glm-5.2', label: 'GLM-5.2' },
      { value: 'glm-5.1', label: 'GLM-5.1' },
      { value: 'glm-5', label: 'GLM-5' },
    ],
  },
  {
    value: 'minimax',
    label: 'MiniMax（稀宇）',
    disabled: true,
    models: [
      { value: 'MiniMax-M3', label: 'MiniMax-M3' },
      { value: 'MiniMax-M2.7', label: 'MiniMax-M2.7' },
      { value: 'MiniMax-M2.7-highspeed', label: 'MiniMax-M2.7 HighSpeed' },
    ],
  },
  {
    value: 'claude',
    label: 'Claude（Anthropic）',
    disabled: true,
    models: [
      { value: 'claude-opus-5', label: 'Claude Opus 5' },
      { value: 'claude-sonnet-5', label: 'Claude Sonnet 5' },
      { value: 'claude-haiku-4-5', label: 'Claude Haiku 4.5' },
    ],
  },
  {
    value: 'gpt',
    label: 'GPT（OpenAI）',
    disabled: true,
    models: [
      { value: 'gpt-5.5', label: 'GPT-5.5' },
      { value: 'gpt-5.4', label: 'GPT-5.4' },
      { value: 'gpt-5.4-mini', label: 'GPT-5.4 Mini' },
    ],
  },
  {
    value: 'gemini',
    label: 'Gemini（Google）',
    disabled: true,
    models: [
      { value: 'gemini-3.6-flash', label: 'Gemini 3.6 Flash' },
      { value: 'gemini-3.5-flash-lite', label: 'Gemini 3.5 Flash Lite' },
      { value: 'gemini-3.1-flash-lite', label: 'Gemini 3.1 Flash Lite' },
    ],
  },
];
