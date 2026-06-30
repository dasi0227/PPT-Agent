---
id: DEV-DOD
title: 完成定义（DoD）
status: approved
owner: shared
depends_on: [DEV-TEST, DEV-RULES]
verifies: []
---

# 完成定义（Definition of Done）

一项工作（功能/里程碑/PR）视为「完成」，当且仅当**全部**满足以下清单。
Coding Agent 在声称「完成/修复/通过」前 MUST 逐项核对并附证据（命令输出），而非断言。

## 通用 DoD 清单

- [ ] 关联了明确的 SPEC-ID / AC（可追溯）。
- [ ] 该功能全部 **P0 验收场景**的校验命令通过（附输出）。
- [ ] 未超出 [scope](../00-overview/scope.md)；backlog 仅占位未实现。
- [ ] 后端：`gofmt` 干净、`go vet` 无告警、`go test -race` 通过。
- [ ] 前端：`lint` + `tsc --noEmit` + 单测 通过。
- [ ] 涉及数据：`sqlite-schema.sql` 可执行；相关 JSON Schema 合法。
- [ ] 涉及 API：`openapi.yaml` 通过 lint，且实现与契约一致。
- [ ] 涉及 slide 产出：`lint-slide` 全过。
- [ ] 涉及枚举变更：文档与 schema/脚本已同步（layout/chart/token/错误码）。
- [ ] scope 隔离类改动：hash diff 证明无越界（page/overview）。
- [ ] 无遗留：无 TODO 占位、无「removed」死注释、无未用兼容 shim。
- [ ] 文档已与实现对齐（如有偏差，文档先行更新）。

## 功能级附加

- [ ] Happy path + 关键边界场景均验证。
- [ ] 错误路径返回正确错误码（错误码表）。
- [ ] 涉及 UI：在浏览器实际操作验证（翻页/总览/编辑），或明确说明无法测试的部分。

## 里程碑级附加

- [ ] 该里程碑 `verifies:` 列出的全部场景通过。
- [ ] CI 必经检查全绿（见 [test-strategy](test-strategy.md)）。
- [ ] 端到端主流程可跑通（到 M4 起逐步要求）。

## 证据要求

- 声称通过 MUST 附**实际命令与输出**（哪怕摘要），不接受「应该通过」「理论上 OK」。
- UI 正确性 MUST 经浏览器验证；类型检查/测试只证明代码正确，不证明功能正确。

## 依赖

- [DEV-TEST](test-strategy.md)、[DEV-RULES](dev-rules.md)
