---
id: 0001-example-task
title: 一句话任务标题
status: pending          # pending | in_progress | done | failed | blocked
priority: 1              # 数字越小越先执行
retries: 0               # 执行器自增，勿手填
max_retry: 3
max_runtime_sec: 5400    # 90 分钟
branch: auto/0001-example-task
task_type: backend       # backend | web | docs | script | mixed
verify_level: test       # build | test | harness | web | custom | none
verify_command: ""       # verify_level=custom 时必填
deps: []                 # 依赖任务 id
owner: 后端开发 Agent
---

## 目标

一句话说清要达成的**业务结果**（不是"改某个文件"，而是"用户能做到什么"）。

## 上下文

涉及哪个模块/接口/页面；相关文件路径；已知约束与坑。

## 验收标准（必须可验证）

- [ ] 编译通过（`go build ./...`）
- [ ] 单测绿（`go test ./... -race`）
- [ ] <具体业务断言，如：GET /api/v1/xxx 返回毫秒时间戳字段 createdAt>

## 边界（明确不要动什么）

- 只改 `server/internal/xxx/` 下文件
- 不动数据库 schema
- 不改公共中间件
