---
id: PLUGIN-AUTHORING
title: 插件编写与挂载
status: approved
owner: shared
depends_on: [PLUGIN-PROTOCOL]
verifies: []
---

# 插件编写与挂载

## 编写一个插件（示例：粒子特效 fx）

### 目录

```
_plugins/particle-burst/
├── manifest.json
└── effect.js
```

### manifest.json

```json
{
  "name": "particle-burst",
  "version": "1.0.0",
  "kind": "fx",
  "description": "封面/强调页的粒子迸发特效，随主题强调色着色",
  "tags": ["cover", "celebrate", "canvas"],
  "mount": { "target": "slide-root", "position": "append" },
  "params": {
    "density": { "type": "number", "default": 120, "description": "粒子数" },
    "color": { "type": "color", "default": "var(--color-accent)" }
  },
  "assets": { "js": "effect.js" },
  "cleanup": true
}
```

### effect.js（约定）

- 暴露初始化入口，接受挂载容器与 params。
- 监听页进入启动、页离开清理（满足 [DS-ANIM-003](../60-design-system/animations.md)）。

## 收藏流程（用户视角）

1. 用户选中一段满意的 html/css/js（或描述意图）。
2. 前端调用 `POST /plugins`（含 manifest + files）。
3. 后端校验 manifest schema → 落 `_plugins/<id>/` + 索引入库。

## AI 移植流程（Agent 视角）

```
用户：「给封面加上我收藏的粒子特效」
  │
  ▼
检索 plugins（按 description/tags 匹配 → particle-burst）
  │
  ▼
读 manifest → 校验 → 解析 mount(append to slide-root) + params(默认)
  │
  ▼
注入：在封面 slide 末尾挂载 effect.js 初始化调用，填入 params
  │
  ▼
lint-slide 校验 → 落盘 + 版本
```

## 参数填充规则

| ID | 约束 |
|---|---|
| `PLUGIN-AUTH-001` | 缺参 MUST 用 manifest 默认值；`ask` 模式可就关键参数提问 |
| `PLUGIN-AUTH-002` | `color` 类参数 SHOULD 默认引用 token，保持换肤一致 |
| `PLUGIN-AUTH-003` | 移植 MUST 按 `mount.position` 精确注入，不破坏既有结构 |

## 验收标准（Given-When-Then）

- **AC-PLUGIN-AUTH-001**（`PLUGIN-AUTH-001/003`）
  - GIVEN particle-burst 插件 + 「给封面加粒子」
  - WHEN 移植
  - THEN 封面末尾出现该 fx 初始化，使用默认 params，lint-slide 通过

## 校验方式

```bash
go test ./internal/plugin -run TestMountAndParams
```

## 依赖

- [PLUGIN-PROTOCOL](plugin-protocol.md)
