# Manifest 资源与变更总结设计

## 资源边界

`deck` 只表示整份演示文稿的聚合作用域，演示内容资源统一命名为 `manifest`：

- `deck:manifest`：演示内容
- `deck:outline`：目录结构
- `deck:design`：视觉设计
- `slide:<id>:spec`：单页设计稿
- `slide:<id>:html`：单页幻灯片

项目根目录使用 `manifest.json`、`outline.json`、`design.json`。Manifest 保存标题、目标、受众、语言、要求、禁忌、画布和页码规则。资源 Schema、Mutation、版本类型、快照目录、公共事件、项目内容 API 和 Agent 上下文使用同一命名。

项目处于开发阶段，只接受 `manifest.json`、`manifest` 版本目标和 `deck:manifest` 公共目标。前后端不读取、归一化或双写旧 `deck.json`、`deck` 版本目标和 `deck:deck` 事件。

`scope.level: deck` 保持不变，因为它表示操作范围是整份演示文稿，而不是 Manifest 文件。

## 变更总结

折叠标题统一使用文件口径：

```text
{唯一变更目标数量} 个文件已更改
```

展开列表去重后按以下顺序展示：

1. 演示内容
2. 目录结构
3. 视觉设计
4. 设计稿，按当前目录页序升序
5. 幻灯片，按当前目录页序升序

页面顺序必须通过 `slide_id` 查询当前 `outline` 的 ordinal，不解析展示文案，也不按 `slide_id` 字符串排序。
