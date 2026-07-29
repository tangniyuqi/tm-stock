# uni-app x 前端开发规范（tm-stock）

## ★ 先查已有技能库

仓库内已有成熟的 **tmui4x / tmx-ui 开发技能库**：

```
.kiro/skills/tmui4x-dev/
├── SKILL.md              # 入口
├── component-catalog.md  # 组件目录
├── css-system.md         # CSS 系统
├── page-dev-guide.md     # 页面开发指南
├── system-components.md
└── tmx-ui/<分类>/<组件>.md   # 各组件详细文档
```

**写页面前先查它**，不要凭空写组件用法、不要重复造轮子、不要另立一套组件规范。

## 技术栈事实（实地核实）

- **uni-app x**：页面用 `.uvue`，逻辑用 `.uts`（不是 Vue SFC + TS）
- 构建走 **HBuilderX**（仓库无 package.json / 无 pnpm / 无 ESLint CLI）
- 仓库根目录**就是前端工程**（不是 web/ 子目录）
- UI 组件库：**tmx-ui**（uni_modules 内）

> ⚠️ 因为没有 CLI lint，前端质量**更依赖人和 AI 自觉 + 合规词门禁**。写完务必在 HBuilderX 里跑一次。

## 目录与命名

| 对象 | 规则 | 示例 |
|-----|------|------|
| 页面目录 | kebab-case，按业务分 | `pages/segment/` |
| 页面文件 | 与目录同名或 index | `pages/news/news.uvue` |
| 组件 | PascalCase | `ThemeCard.uvue` |
| 方法/变量 | camelCase | `getThemeDetail()` |

现有业务页面：`index`（首页）· `news`（快讯）· `segment`（题材）· `member`（会员）· `express`。
**新页面必须在 `pages.json` 注册**。

## uts 类型要点（踩坑预防）

- uts 是**强类型**，不是 TypeScript：类型不匹配会编译报错而非运行时容错。
- **接口返回的时间字段统一为毫秒时间戳（number）**，前端用统一工具函数格式化，
  ❌ 不要在 model 里声明成 string（会因 int→string 直接崩，这是同类项目踩过的坑）。
- 可空类型显式声明（`type?` / `| null`），不要用 any 兜底。
- 数组/对象操作注意 uts 与 JS 的差异（部分 JS 方法不可用），以 tmui4x 技能库示例为准。

## 页面三态（强制）

每个数据页面必须有：
1. **加载态**：骨架/loading，不要白屏
2. **空态**：区分"暂无数据"与"筛选无结果"，给引导文案
3. **错误态**：网络失败可重试；无权限（未订阅）引导订阅

## 移动端要点

- 按 375px 设计，向上适配；禁止横向滚动
- 点击区域 ≥ 44×44px；注意 iOS 安全区
- 列表长滚动做分页/虚拟列表，避免卡顿

## 接口调用

- 统一封装请求（拦截器处理 token、错误提示、loading）
- **catch 里禁止重复弹错误提示**（拦截器已统一处理）
- 涨跌幅等行情展示位**必须标注数据来源与延时**（见合规红线 + 数据真实铁律）

## 🔴 合规（前端是文案重灾区）

- 页面文案、按钮、标题、AI 展示内容**逐字对照** `.claude/agents/compliance-redline.md` 禁用词表
- **一期页面不得出现个股列表/代码/个股涨跌幅**
- 全站固定免责："本平台为公开信息聚合与历史数据统计，不构成任何投资建议。"
- ❌ 禁止硬编码假数据冒充真实数据；mock 必须显著标注且上线前清除
