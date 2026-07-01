---
id: ADR-0010
title: 三层隔离模型（Project / Thread / Run）
status: accepted
owner: backend
depends_on: [ADR-0002, ADR-0004]
verifies: []
---

# ADR-0010：三层隔离模型（Project / Thread / Run），对齐 Codex

## 背景

用户问：启动后如何创建不同工作目录代表不同 PPT，如何像 Codex 那样区分 thread/project 实现隔离。
原设计只有 Project（work_dir）与 Run（单次执行），缺少"跨多次执行的可恢复对话"这一层，
导致 `/recap`、编辑连续性、"关掉再打开接着聊"无处承载。

## 决策

引入 **三层隔离**，对齐 Codex 的 project/thread/turn：

```
Project（PPT = work_dir，产物文件） —— 每 project 一把执行锁
  └── Thread（可恢复对话历史，可多条并行）—— 共享 project 产物（Codex 语义）
        └── Run（一次执行/turn，跑 harness 循环）
```

- **Project**：一个 PPT 一个 `work_dir`；新建 PPT=`POST /projects`；切换=换 `project_id`。物理隔离。
- **Thread**：一个 project 可多条 thread；thread 只隔离对话历史（`threads/<id>.jsonl`），**共享产物文件**（非分叉）。
- **Run**：挂在 thread 下（`POST /threads/{id}/runs`），执行前载入该 thread 历史。
- **并发**：每 project 一把执行锁——同 project 串行（含跨 thread），不同 project 并行。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| 三层 + thread 共享产物（选中） | 对齐 Codex、支持多对话/可恢复、并发安全、复杂度可控 | 多 thread 不能各自独立产物快照（用版本回滚替代） |
| 只两层（Project + Run，无 thread） | 最简单 | 无对话连续性/恢复，/recap 只能硬凑 |
| thread 分叉产物（copy-on-write，像 git 分支） | thread 间产物隔离 | 复杂度高、与"每 project 锁"冲突、单机收益低 |

## 后果

- 数据层新增 `threads` 表；`runs` 加 `thread_id`（+ 冗余 `project_id` 便于加锁）。
- 文件布局新增 `threads/<id>.jsonl`；多项目在 `PPT_WORK_ROOT/<project_id>/` 各自独立；`_assets/` 全局共享。
- API 新增 thread 层：`/projects/{id}/threads`、`/threads/{id}`、`/threads/{id}/history`；Run 创建改为 `/threads/{id}/runs`。
- Run 引擎新增"每 project 执行锁"（`ARCH-RUN-LOCK-*`）。
- context-assembly 与 `/recap` 以 thread 历史为主要连续性来源。

## 状态

accepted（2026-07-01）。
