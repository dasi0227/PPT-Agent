# 统一资源元信息与文件存储

日期：2026-09-25。状态：已实现；仅做静态复核，测试、构建与交互验收待用户执行。

本设计覆盖此前仓库、提示词库、标签启停与 seed 初始化文档中的存储和接口约定。资源正文的视觉契约、现有输入框触发字符与匹配顺序继续沿用。`DESIGN.md` 不变。

## 数据所有权

| 表 | 用途 |
| --- | --- |
| `resources` | 四类资源的身份、名称、描述、启停和应用内修改时间 |
| `tags` | 按资源类型区分的标签定义 |
| `resource_tags` | 资源与标签的多对多关联 |

`resources` 字段为 `type、id、name、normalized_name、description、disabled、created_at、updated_at`，联合主键 `(type,id)`。类型仅为 `theme、component、skill、snippet`。服务对名称执行 trim 和小写化，生成 `normalized_name`；只有 snippet 的规范化名称具有唯一索引。其他类型允许同名，组件按名称加载仍会报告重名歧义。

`resource_tags(resource_type,resource_id)` 通过联合外键引用资源，删除资源时级联删除标签关联；标签定义不随资源删除。标签必须属于对应资源类型，最多两个，不允许重复。名称、描述、标签与启停在一次数据库事务中修改。

迁移 `0022_resources.sql` 删除 `prompts`、`resource_states`，重建资源关联，将标签 scope 切换为 snippet。按开发期约定不转换旧资源内容、不双读、不保留旧接口；验收使用新工作目录。历史迁移保留，保证结构迁移链可从空库执行。

## 正文文件

| 类型 | 根据类型和 ID 推导的路径 |
| --- | --- |
| theme | `assets/themes/<ID>/theme.css` |
| component | `assets/components/<ID>/index.html` |
| skill | `assets/skills/<ID>/SKILL.md` |
| snippet | `assets/snippets/<ID>/snippet.txt` |

文件只承载正文，不再解析或写入名称、描述前缀。数据库不存绝对路径。ID 不允许路径分隔符或目录穿越，资源路径拒绝符号链接。主题正文继续校验主题 token 契约。正文必须是非空 UTF-8；主题/组件上限 64 KiB，Skill 256 KiB，短语 16 KiB。

外部编辑仅改变正文，文件 mtime 不更新数据库 `updated_at`。名称、描述修改不触碰文件字节，主题正文 hash 不受元信息影响。Skill 中的示例 frontmatter、Markdown 分隔线属于正文，完整保留。

所有管理列表从数据库取资源身份；按文件检查结果返回 `content_state: ready | missing | invalid`，不可用时附 `content_error`。主题不可用时 `appearance: null`。文件缺失、正文无效的资源仍可修改元信息或删除。目录存在但未登记的资源不会自动出现。

主题 CSS、组件正文及 Skill 正文经各自读取接口按需加载。短语列表携带正文以支持原有全文匹配。候选与 Agent 加载仅使用正文可用且已启用的资源；已选项目仍可渲染已关闭但正文有效的主题。

## 接口

| 方法与路径 | 请求或用途 |
| --- | --- |
| `POST /api/v1/resources` | 登记既有有效文件：`type,id,name,description,tags,disabled?` |
| `PATCH /api/v1/resources/:type/:id` | 部分修改 `name,description,tags,disabled`；响应资源元信息 |
| `DELETE /api/v1/resources/:type/:id` | 删除文件目录、资源记录及标签关联 |
| `GET /api/v1/themes`、`/themes/:id` | 主题管理列表及详情 |
| `GET /api/v1/components`、`/components/:id` | 组件管理列表及详情 |
| `GET /api/v1/skills`、`/skills/:id` | Skill 管理列表及详情 |
| `GET /api/v1/snippets`、`/snippets/:id` | 短语管理列表及详情，字段为 `description,content` |
| `POST /api/v1/snippets` | `name,description,content,tags,disabled?`，生成 ID 后写文件并登记 |
| `PUT /api/v1/snippets/:id/content` | 仅修改 `content`，原子替换正文文件 |

旧 `/prompts` 及各类型专用 PATCH/DELETE 路由被移除。重复身份或短语重名返回 409，未知资源返回 404，非法类型/ID/标签返回 400，无效正文/元信息/路径返回 422；数据库及 I/O 故障返回 500。PATCH 不接受正文、ID、类型或时间戳变更。客户端先 PATCH 元信息，再读取对应类型详情，获得当前文件状态。

元信息示例：

```json
{
  "type": "snippet",
  "id": "review-note",
  "name": "检查叙事",
  "description": "检查每页是否服务于核心结论。",
  "tags": ["review"]
}
```

提交登记前，正文须已存在于 `assets/snippets/review-note/snippet.txt`。登记接口不创建文件或扫描目录。

## 写入与删除故障处理

应用内资源写操作串行执行。短语创建先写临时目录，文件同步后重命名为正式目录，再插入数据库；数据库插入失败清理新目录。正文修改在同目录写临时文件并原子重命名，再更新数据库时间；数据库失败恢复原字节，原先文件缺失则删除新文件。

删除顺序：

1. 若目录存在，移动到 `.resource-trash/<type>/<id>`；文件或目录缺失不阻止删除记录。
2. 删除数据库记录，关联由外键级联清理。
3. 数据库失败时恢复原目录；提交成功后清理临时删除目录。
4. 启动及显式初始化先恢复未完成删除：记录仍在则恢复目录，记录已不存在则清理残留目录。恢复目标已存在时报告冲突，避免覆盖外部文件。

临时删除目录清理失败会保留现场并报告错误，下一次启动继续清理。应用内文件写入与数据库之间采用失败补偿；除删除日志外，不承诺跨文件系统与数据库的断电事务。外部同时修改文件不属于应用内串行锁的范围。

## 初始化与维护

`seed/resources.json` 集中登记预置名称、描述、标签；只作为初始化输入，运行时不读取此清单。`seed/assets/` 保存三个主题、五个组件、两个 Skill、六条短语的纯正文。

初始化通过 `ResourceService.InitializeResources` 复制缺失文件并登记；已登记资源直接跳过，包括已修改或文件已缺失的资源。未登记但已有文件时保留已有正文并校验登记。显式重复初始化不会覆盖已登记资源的用户修改。

```sh
# 从仓库根目录执行，指定新的验收工作目录
./scripts/init-workroot.sh /tmp/ppt-resource-acceptance
```

普通后端启动只执行结构迁移和未完成删除恢复，不初始化预置。`restart.sh` 仅在数据库不存在或显式重置后调用初始化。已有工作目录的 `--no-reset` 不补回删除的预置资源。用户主动执行初始化命令仍会重新登记已删除的预置。

预置替换是单独的维护操作，应先停止后端：

```sh
python3 scripts/replace-theme-presets.py --work-root /tmp/ppt-resource-acceptance
python3 scripts/replace-theme-presets.py --work-root /tmp/ppt-resource-acceptance --components-only
```

替换命令调用同一 Go 资源服务，替换预置正文及元信息、标签，保留已有启停状态；完整替换还删除已登记的退役主题。未登记目录不属于资源库，留在磁盘。组件专用模式不修改主题、Skill 或短语。批处理逐资源执行，失败时停止，可重新执行；不承诺整批回滚。

## 前端

资源路由切换为 `/warehouse/snippet`，类型、API、Store、快捷键 ID 切换为 snippet，界面称为“短语”。保留 `%` / `％` 及可配置触发行为；匹配顺序仍为名称、标签、描述、正文，再按更新时间和名称排序。`@` 汇总候选也排除不可用短语。

短语插入使用文件正文，提交给 Run 的仍是纯文本；名称、描述、ID 不混入正文。增加与其他仓库一致的外部文件查看入口。不新增创建或导入 UI。系统提示词目录 `backend/prompts/` 和通用 Composer 命名不在资源重命名范围内。

历史 Run 中 Skill 是执行时快照，与当前资源的可用状态分开建模；删除当前资源不抹除历史快照。

## 手动验收

以下命令未由 Agent 执行，由用户手动运行：

```sh
cd backend
go test ./internal/service ./internal/store/sqlite ./internal/httpapi ./internal/contextengine ./internal/workflow ./migrations
cd ../frontend
pnpm test src/stores/snippetStore.test.ts src/stores/componentStore.test.ts src/features/agent/promptMatching.test.ts src/features/agent/PromptComposerEditor.test.tsx src/features/agent/SkillSelector.test.tsx src/features/repository/SnippetRepositoryPage.test.tsx src/features/repository/RepositoryPages.test.tsx src/features/viewer/ThemeSelector.test.tsx
pnpm build
```

聚焦用例覆盖四类登记/查询/元信息/启停/删除、重复 ID、短语唯一名称、非法标签事务回滚、文件字节与主题 hash 稳定、纯正文加载、缺失/无效资源管理、文件写入失败、数据库失败补偿、删除中断恢复、重复初始化及纯文本插入。

交互验收使用新工作目录：登记四类资源，修改名称和描述，禁用、恢复和删除；外部删掉或清空正文后刷新，确认管理页保留资源且候选消失；短语查看文件和 `%` 插入保持正文；重复初始化保留用户修改，普通重启不恢复已删除资源。

## 2026-09-25 标签字典消费补充

标签名称、排序和系统标记统一通过 `GET /api/v1/tags?scope=theme|component|skill|snippet` 从数据库读取；不传 scope 返回全部定义，非法 scope 返回 400。资源管理页和输入框消费相同字典，前端不另存标签名称和排序。

`seed/tags.json` 是显式初始化输入；`InitializeResources` 和预置替换 CLI 先登记缺少的标签，已有定义不覆盖。普通资源查询不执行初始化。本次不提供标签编辑界面。整体数据库切换仍按 [持久化重构进度](2026-09-25-session-storage-refactor-design.md#17-实施进度2026-09-25) 继续实施。
