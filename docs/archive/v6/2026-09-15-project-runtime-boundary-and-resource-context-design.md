# 项目运行时边界与资源上下文设计

> 2026-09-25 持久化切换说明：本文中的旧数据库表、独立 user/model JSONL、命令专用执行接口及事件流描述已由[会话日志、命令与数据库重构总结](2026-09-25-session-storage-refactor-design.md)替代。其他产品行为仍按最新用户决定执行；新实现仅做静态复核，手动验收见新文档第 17 节。

## 决策

PPT 项目的私有运行数据统一放在：

```text
.dasi/ppt/projects/<project_id>/
├── artifacts/
├── threads/
│   └── <thread_id>/
│       ├── user.jsonl
│       └── model.jsonl
└── checkpoints/
```

`Project.WorkDir` 指向 `artifacts/`。Agent、Git、PPT 规范文件、页面 HTML、附件和运行时渲染均以该目录为工作目录。`threads/` 与 `checkpoints/` 属于项目私有运行数据，不进入 Agent 工作目录，也不进入 artifacts 的 Git 仓库。

本次直接切换到新结构，不读取旧的 `projects/<id>/threads`、`project-history/`、`threads/<id>.transcript.jsonl` 或 `memory.json`，也不提供迁移与兼容分支。

## 双历史文件

- `user.jsonl` 保存用户界面需要展示的任务历史、工具活动与状态事件。
- `model.jsonl` 保存送入模型的消息、工具调用与工具观察，用于连续对话和上下文压缩。

两者生命周期相同但消费方与内容结构不同，因此不得合并。线程创建时同时创建两个空文件，线程删除时同时删除。

`memory.json` 当前没有稳定的数据契约和写入规则，全部读写与上下文注入先移除。需要长期记忆时再以独立设计重新引入。

## Checkpoints

项目历史快照、状态与恢复 journal 存放在 `checkpoints/`。快照只包含 `artifacts/`、`threads/` 和项目数据库清单，绝不递归包含 `checkpoints/` 自身。

删除项目时，先删除数据库行、`artifacts/` 与 `threads/`，保留 checkpoints 直到历史中间件确认操作成功；确认后再清理整个项目容器。若确认前进程中断，启动恢复可用 journal 和快照还原项目。

## 空项目 Target

项目内容加载完成且没有页面时，默认 Target 为“全部页 · 全局资源”。Target 按钮保持可点击；设计稿、幻灯片、演示文稿、当前页、自选页与自选章显示为禁用，全局资源与全部页可选。

项目已有页面且用户没有主动选过 Target 时，默认使用“当前页 · 演示文稿”。本地缓存中的旧 Target 不得覆盖空项目的默认值。

## 组件与技能上下文

上下文组装器从真实组件仓库与技能仓库加载所有启用资源的目录，向模型明确注入稳定 ID、名称、描述与标签。模型只能使用目录中给出的 ID 调用 `load_component` 与 `load_skill`；工具调用成功后返回仓库中的真实 HTML 或技能正文。

资源目录参与上下文预算。当目录加载失败时记录明确 warning；不得用虚构 ID 或占位内容替代。

## finish 与模型历史

`finish` 的 `summary` 是用户最终看到的助手回复。工具校验通过后，运行时必须把该 summary 追加为普通 assistant 消息，再保存 `model.jsonl` 和终态 checkpoint。后续任务由此获得与用户界面一致的上一轮最终回复。
