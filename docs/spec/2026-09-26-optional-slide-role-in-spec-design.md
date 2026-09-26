# 页面角色归属 Spec 并改为可选

日期：2026-09-26。状态：已实施，待用户手动验收。

## 决定与数据职责

按用户确认，将页面 `role` 从 Outline 移至 Slide Spec，改为可选字段。Outline 只维护章节、页面成员、稳定 ID、标题、归属和顺序；Spec 维护可选角色、核心信息、内容元素和可选布局建议。

`.outline.json` 中的页面节点只包含 `slide_id` 和 `title`。`outline.init`、`outline.insert` 与 `outline.update` 不再接受角色。创建页面不生成占位 Spec，也不默认设置 `content`。

`.spec.json` 继续以稳定页面 ID 为对象键。每页 `key_message`、`elements` 必填，`role`、`layout` 可选。角色沿用现有 12 种枚举值，缺省表示未设置；清空时删除字段，不写入空字符串、null 或额外的 `none` 枚举。

```json
{
  "sli_example": {
    "role": "evidence",
    "key_message": "企业客户贡献主要增量",
    "elements": [{ "type": "chart", "intent": "对比不同客群的收入增量" }]
  }
}
```

## 编辑、展示与生成

- 目录管理移除页面角色输入，新增页面和修改标题只操作 Outline。
- 设计稿管理增加「页面角色（选填）」选择器，提供「未设置」选项；保存仅更新该页 Spec。
- 预览、设计稿展示、Agent 页面摘要和运行时 Frame 均从 Spec 读取角色。Spec 未生成或未设置角色时不推断为内容页。
- `slide.spec.write` 可以设置或省略角色；`slide.spec.patch` 支持对 `/role` 添加、替换和删除，沿用单页内容 hash 与修改权限。
- 角色添加、变更和删除进入现有 Spec 生成参考差异。仅修改角色不自动修改 HTML、不推进生成参考基线；Agent 根据用户任务和差异决定是否重新创作。

## 实施边界

前后端、Schema、工具、提示词、渲染和导出一次性切换，不保留 Outline 角色兼容、双读或双写，不迁移或清空历史开发数据。不修改 `DESIGN.md`。

本文替代既有设计文档中「Outline 拥有页面角色」的约定。目录结构、Spec 集合存储和 HTML 生成参考快照的其他规则保持不变。

## 手动验收

1. 新建项目并创建大纲、添加页面，角色不是必填项，页面标题仍可编辑。
2. 创建或编辑设计稿，选择角色并保存，预览与设计稿展示一致；再选择「未设置」保存，角色可清空。
3. 仅修改某页角色时，其他页 Spec、Outline 和现有 HTML 不被改写。
4. 该页已有 HTML 生成参考基线时，Agent 能收到 `/role` 的净变化；角色恢复为原值后差异消失。

## 验证记录

- 后端全部测试包编译检查通过；`spec`、`pptmutation`、`schemas`、`prompt` 包测试通过，覆盖角色枚举校验、添加／替换／删除、非法值拒绝及生成参考净变化。
- Agent 上下文与工具合同、生成参考、HTTP 目录修改和导出快照定向测试通过。
- 前端目录、Spec 编辑与清空、页面投影、Frame 和语义标签共 22 项定向测试通过；生产构建通过，保留现有大分包提示。
- 静态设计检查及 Git diff 空白检查通过；未运行浏览器或计算机自动化，交互和视觉验收由用户完成。
- 扩展检查并非全绿：`contextengine` 的 `TestTranscriptNormalizesExistingRenderCallsOnReplayAndSave` 使用旧日志格式，触发 `identity or format mismatch`；前端测试脚本的参数未按预期限制范围，额外报告了命令生命周期、Git 提交时间线、iframe 缓存等测试失败。本次未修改这些功能，未再次运行全套测试。新增 Spec 表单测试的 jsdom 适配问题已修正，随后定向检查通过。
