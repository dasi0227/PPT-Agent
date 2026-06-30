---
id: DEV-RULES
title: 开发规则
status: approved
owner: shared
depends_on: [OVERVIEW-SCOPE]
verifies: []
---

# 开发规则

本规则对人与 Coding Agent 同等生效。措辞遵循 RFC 2119（MUST / MUST NOT / SHOULD / MAY）。

## 总则

- **R1**：实现 MUST 以本 `docs/` 为事实源；与文档冲突时，先改文档达成一致，再写代码。
- **R2**：动手前 MUST 阅读对应功能的 `feat-*.md` 验收标准与相关 schema/契约。
- **R3**：MUST NOT 超出 [scope](../00-overview/scope.md) 范围实现 backlog 项；仅按预留接口占位。
- **R4**：每个改动 MUST 可追溯到某个 SPEC-ID / AC；无归属的「顺手改」MUST 拆分并记录。

## 范围与克制（YAGNI）

- **R5**：MUST NOT 增加任务未要求的特性、抽象、配置开关。
- **R6**：MUST NOT 为不可能发生的场景加防御/兜底；仅在系统边界（用户输入、外部 API）校验。
- **R7**：三行相似代码优于过早抽象；重复达到痛点再抽象。

## 状态与并发（来自项目约定）

- **R8**：共享状态（`state.json`、SQLite 写）MUST 串行化访问，防竞态（[ARCH-SYS-005](../20-architecture/system-overview.md)）。
- **R9**：用单一状态游标（`current_state`）表达进度，MUST NOT 引入冗余标志位造成「双重真相」。
- **R10**：`state.json` MUST 位于 work_dir 根。

## scope 隔离（产品核心约束）

- **R11**：page scope MUST 只改目标页文件；overview scope MUST 只改公共样式层。违反即缺陷。
- **R12**：任何可编辑产物变更 MUST 产生可回滚版本。

## LLM 与安全

- **R13**：API Key MUST 仅来自环境变量，MUST NOT 落库/落日志。
- **R14**：用户输入 MUST 经注入边界处理；system 层约束 MUST NOT 被用户指令覆盖。
- **R15**：prompt MUST 经 [prompt-templates](../50-agent/prompt-templates.md) 装配，MUST NOT 散落硬编码。

## 产出质量

- **R16**：slide 产出 MUST 通过 [html-output-spec](../60-design-system/html-output-spec.md) 校验后才落盘。
- **R17**：注释克制：默认不写；仅在「为什么」非显而易见处写一行（隐藏约束/坑/反直觉）。MUST NOT 写解释「做什么」的冗余注释。
- **R18**：MUST NOT 为移除的代码留「兼容性占位」（空 `_var`、`// removed` 注释、无用 re-export）。确认无用即删。

## 提交与协作

- **R19**：提交信息聚焦「为什么」，关联 SPEC-ID/AC。
- **R20**：高风险/不可逆操作（删分支、force push、改 CI、删库表）MUST 先与用户确认。
- **R21**：MUST NOT 跳过 hook（`--no-verify` 等），除非用户明确要求。

## 文档同步

- **R22**：枚举类（layout/chart/动效/错误码/token 必需集）变更 MUST 同步更新文档与对应校验脚本。
- **R23**：新增需求 MUST 先在功能文档登记 SPEC-ID 与 AC，再实现。

## 验收标准（Given-When-Then）

- **AC-RULES-R11**（R11）
  - GIVEN 任意 page/overview 编辑
  - WHEN hash diff 受影响文件集合
  - THEN 与 scope 约束完全一致（无越界）

## 校验方式

- 通过各功能 AC 的 hash diff / lint / test 间接强制；R 系列在 code review 与 CI 中核查。

## 依赖

- [OVERVIEW-SCOPE](../00-overview/scope.md)
