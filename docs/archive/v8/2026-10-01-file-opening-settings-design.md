# 文件打开设置

日期：2026-10-01。状态：已实施，待用户手动验收。

## 界面与应用管理

设置导航及板块标题统一为「文件打开」。上方「文件打开方式」按默认方式、JSON 文件、HTML 文件排列，字段标题不显示图标；下方「打开应用」显示应用图标与名称。复用公共 Select、IconButton、设置行布局和全局交互 token。

访达、文本编辑、VS Code 为固定预设，不提供删除入口，也不保存为自定义应用副本。「添加应用」直接使用已有 macOS 系统应用选择器，不展示候选列表。取消选择与重复选择不保存。其它应用可删除。

七个应用图标放在 `frontend/public/file-app-icons/`，下载来源记录在同目录 `sources.json`。Obsidian、Typora、WebStorm、Cursor 仅预备图标，用户添加对应应用后再匹配；IDEA 是原型的未知图标示例，不预置在正式应用列表。系统默认应用、未知应用及图片加载失败统一使用 Lucide Box。列表图标为 30px，Select 内图标为 20px，保持原始比例。

## 前后端协议

`GET /api/v1/settings/files` 和 `PUT /api/v1/settings/files` 直接使用新结构：

```json
{
  "default": { "open_with": "system", "custom_app_path": "" },
  "json": { "open_with": "inherit", "custom_app_path": "" },
  "html": { "open_with": "inherit", "custom_app_path": "" },
  "custom_apps": [],
  "revision": 0
}
```

读取及保存响应额外携带 `supported`；保存请求不包含该字段。`open_with` 为 `system`、`finder`、`textedit`、`vscode`、`custom`；JSON 与 HTML 额外允许 `inherit`，默认方式禁止该值。只有 `custom` 使用非空的 `custom_app_path`，且必须引用已注册的自定义应用。自定义应用仍由实际应用包路径生成名称、去重及校验。

系统选择接口的应用对象包含 `name`、`path`，选中固定预设时额外包含 `builtin`，取值为 `finder`、`textedit` 或 `vscode`。前端忽略这类重复添加；后端也阻止将固定预设保存成可删除副本。

三种打开方式和应用列表在一次 revision 校验中原子保存。删除被引用的应用时，默认方式恢复 `system`，JSON / HTML 恢复 `inherit`；其它配置保留。不接受未注册的新引用，也不因保存失败覆盖当前设置。

## 实际打开与存储

`POST /api/v1/files/open` 由后端按请求文件扩展名选择配置，`.json` 使用 JSON 配置，`.html` / `.htm` 使用 HTML 配置，其余使用默认方式；扩展名不区分大小写。`inherit` 解析到当前默认方式。前端按钮提示使用同样的文件类型规则。保留本机同源检查、文件范围校验、字面参数调用和失败不重试规则。

SQLite `file_settings` 将原单一方式切换为 `default_open`、`json_open`、`html_open` 三个 Method JSON 列，并保留 `custom_apps` 与 `revision`。开发数据库格式从 3 升到 4，直接切换，没有旧协议、双读双写或历史迁移方案；已有旧格式数据库需重新初始化。本次仅修改项目代码，没有覆盖运行中的数据库。

## 手动验收

遵循项目约定，未运行测试、构建或浏览器自动验证；更新现有回归用例，覆盖独立路由、继承和删除引用。

- 默认、JSON、HTML 分别选择不同应用，重载设置后仍保持，实际 JSON / HTML / 其它文件按对应配置打开。
- JSON 或 HTML 选择「跟随默认」，更换默认方式后立即使用新的默认应用。
- 点击添加应用直接显示系统选择器；取消、重复或选择固定预设不新增条目。
- 自定义应用可删除，三个固定预设没有删除入口；删除被多个配置引用的应用时所有引用一并恢复。
- 未知应用使用同尺寸 Box；已准备的品牌应用显示对应图标，图片失效时显示 Box。
- 并发窗口保存冲突、保存失败和启动应用失败保留原状态及现有错误反馈。
