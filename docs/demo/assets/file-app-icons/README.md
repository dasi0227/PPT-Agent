# 文件打开应用图标

供 `2026-10-01-file-opening-settings-demo.html` 原型使用，不接入实际前端。

下载地址、来源页面和原始文件大小记录在 [sources.json](sources.json)。图标保持原始样式，版权及商标属于各应用或素材作者；TextEdit 图标来自 Apple 的 macOS Sequoia 使用手册，Finder 图标来自 Wikimedia Commons，其余来自应用官方网站。

初始列表包含三个固定应用，以及用于展示通用图标的 IntelliJ IDEA；Obsidian、Typora、WebStorm、Cursor 图标已预备，系统选择器选中对应应用后再匹配。系统默认应用、未知应用与图片加载失败时使用原型内的 Lucide Box 通用图标。

「添加应用」直接复用项目的系统应用选择接口，不展示候选应用列表。通过本地 HTTP 原型访问该接口，添加结果只保存在原型内存中，不写入实际文件设置。
