# 题材宝典 tm-stock

为投资者提供**题材 / 行业信息查询与通俗解读**的移动端应用。
一期定位：**免费快讯引流 + 付费题材查询**（只到题材/行业/产业链环节层）。

## 🔴 开工前必读

**[`.claude/agents/compliance-redline.md`](.claude/agents/compliance-redline.md) — 合规红线**

> 不评价 · 不预测 · 不荐买卖 · 个股只做「客观事实」层（分类归属 + 涨跌幅，**依据可溯源**）·
> 一期不做个股详情 · 用户自主判断。
> **分界线是「事实 vs 我方观点」，不是「有没有出现个股」**（见 [ADR-0003](docs/adr/0003-个股客观事实层可做.md)）。
> 违反 = 公司级法律风险（无牌证券投资咨询）。

AI 协作入口：**[CLAUDE.md](CLAUDE.md)**；研发流程：[WORKFLOW.md](WORKFLOW.md)。

## 技术栈

| 端 | 技术 |
|---|------|
| 前端 | **uni-app x**（`.uvue` / `.uts`）+ tmx-ui，HBuilderX 构建；一期只发 **H5** |
| 后端 | **Go + Gin**（`server/`） |
| 存储 | MySQL / Redis（按需）；全文检索预留 Meilisearch |

## 目录

```
├── App.uvue · main.uts · pages.json    # 前端入口（仓库根即前端工程）
├── pages/         index · news(快讯) · segment(题材) · member · express
├── uni_modules/   组件依赖（含 tmx-ui）
├── .kiro/skills/tmui4x-dev/            # ★ tmui4x 开发技能库（写页面前先查）
├── .claude/       AI 协作：settings · hooks · agents(规范)
├── server/        Go 后端
├── scripts/       质量门禁 + git hooks
├── docs/          GOALS · adr · specs
└── tasks/         半自主任务队列
```

## 快速开始

```bash
# 1. 安装 git hooks（每个 clone 都要跑一次）
bash scripts/git-hooks/install.sh

# 2. 前端：用 HBuilderX 打开本目录，运行到 H5

# 3. 后端
cd server && go mod tidy && go run ./cmd/api
```

## 质量门禁

```bash
bash scripts/harness-checks.sh          # 合规词 + 密钥 + Go 编译测试 + pages.json
bash scripts/check-compliance-words.sh --all   # 单独跑合规词检查
```

**铁律：Agent 自述不算数，门禁绿灯才算数。**

## 当前阶段

一期以**无开发付费验证**为先（内容 + 私域测付费意愿），暂不追求正式上线。
详见 [docs/GOALS.md](docs/GOALS.md)。
