# Git Hooks（L2 提交门）

## 安装

```bash
bash scripts/git-hooks/install.sh
```

每个新 clone / 新 worktree 都要跑一次（git hooks 不随仓库自动分发）。

## 分层原则

| 钩子 | 放什么 | 为什么 |
|-----|-------|-------|
| `pre-commit` | **秒级**：密钥扫描、格式检查 | 快才不会被绕过 |
| `pre-push` | **重量级**：编译、测试 | 推送前兜底，慢一点可接受 |

**踩坑固化**：慢钩子放 pre-commit → 开发者必然用 `--no-verify` → 门禁形同虚设。

## 红线

- `--no-verify` 属于**异常操作**，不要常态化。
- 基线文件（`secret-scan-baseline.txt`）**只减不增**，禁止为让新代码过门禁而扩基线。
