---
id: ADR-0002
title: SQLite + 文件系统混合持久化
status: accepted
owner: backend
depends_on: [ADR-0001]
verifies: []
---

# ADR-0002：SQLite + 文件系统混合持久化

## 背景

无登录注册、单用户单机。需保存项目、Deck、Slide、版本历史与个人插件仓库。slide 是大文本 HTML，需可直接渲染、git 友好；元数据需可查询、事务安全。

## 决策

**混合持久化**：元数据（项目/Deck/Slide 索引/版本记录/Run/插件索引）入 **SQLite**；slide html/css/js、公共样式层、slide-json、插件资源等大文本入**文件系统**（work_dir）。两者经路径字段关联。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| SQLite + 文件系统（选中） | 查询能力 + 大文本可直接渲染/版本对比/git 友好 | 需维护二者一致性 |
| 纯 SQLite | 部署最简、单文件 | 大 HTML blob 不便直查/版本对比 |
| 纯文件系统 | 极简、git 友好 | 跨项目查询弱、需自建索引 |
| 嵌入式 KV（Bolt） | 无外部依赖 | 缺关系查询，分析需自建索引 |

## 后果

- 数据模型分两处定义：SQLite schema（[sqlite-schema.sql](../30-data-model/sqlite-schema.sql)）+ 文件布局（[filesystem-layout](../30-data-model/filesystem-layout.md)）。
- 公共样式层与单页 html 物理分离，天然支撑 `/page` 与 `/overview` 隔离。
- 版本以快照文件 + SQLite 记录实现，回滚直观。
- 需保证写一致性：写文件与写 SQLite 的顺序与失败处理需明确（先写文件后登记，或事务边界约定）。

## 状态

accepted（2026-06-30）。
