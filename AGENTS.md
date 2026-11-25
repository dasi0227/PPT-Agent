# AGENTS

## 项目目标

本项目旨在构建一**个模拟 Microsoft Office PowerPoint 使用体验的创作平台**，以及一个**能理解需求、组织内容、规划设计并生成 HTML 格式的演示文稿的专用 Agent/Harness**。

## 参考文件

- [DESIGN.md](docs/note/DESIGN.md)：全局 UI 设计规范，统一布局、配色、字体、间距、交互状态及公共组件用法。
- [ICON.html](docs/note/ICON.html)：界面对象与图标的对应规范，统一各模块的图标选择与语义。
- [API.html](docs/note/API.html) 汇总接口和字段，[API_SPEC.md](docs/note/API_SPEC.md) 规定请求处理、历史查询与实时通知行为。
- [DATABASE.html](docs/note/DATABASE.html) 定义表和字段，[DATABASE_SPEC.md](docs/note/DATABASE_SPEC.md) 规定持久化、事务与一致性约束。
- [STRUCTURE.html](docs/note/STRUCTURE.html) 列出模块和目录，[STRUCTURE_SPEC.md](docs/note/STRUCTURE_SPEC.md) 规定职责、文件提交与导出生命周期。
- [WORKFLOW.html](docs/note/WORKFLOW.html) 展示流程图与类职责，[WORKFLOW_SPEC.md](docs/note/WORKFLOW_SPEC.md) 规定 Run、Hook、工具提交、命令与恢复的执行规则。
- [ASSEMBLE.html](docs/note/ASSEMBLE.html)：Agent 提示词拼装规范，明确各模式与任务的提示词模块、上下文组成及可用工具。

## 行为约束

- 若非用户直接要求执行，在大规模改动前，必须用通俗易懂的无序列表列出所有计划变更，并等待我确认。
- 当要求交付任务给其他会话或子代理时，发送的提示词内容需要至少包含「参考文件、设计理由、需求说明、目标要求」。
- 当完成代码开发后，只需要用无序列表输出必要的变更总结即可，不需要长篇大论的阐述和解释。
- 若非用户直接要求，否则禁止使用 `Computer Use`、`Browser` 等自动化工具执行任何验证操作，交给用户进行验证即可。
- 若非用户直接要求，否则禁止使用 `SubAgent`、`MultiAgent` 等子代理能力开发工作，全部任务必须独立完成。

## 开发规范

### 文档规范

- `docs/archive/` 存放历史设计文档，仅用于帮助理解项目的演进，禁止作为开发依据，禁止做任何更改。
- `docs/spec/` 存放后端 Spec Markdown 设计文档，并使用 `yyyy-mm-dd-<需求>.md` 格式命名。
- `docs/demo/` 存放前端 Demo HTML 原型文件，并使用 `yyyy-mm-dd-<需求>.html` 格式命名。
- `docs/note` 存放用户已经确定的内容，必须严格遵守，具有最强约束力，任何与其中不相符的内容都需要明确跟用户提出。
- `docs/tmp/` 存放无需协作或提交的本地临时数据，禁止存放需要纳入版本控制的内容。

### UI/UX 规范

- 必须与产品整体 UI 对齐，让布局、配色、字体、间距、图标、交互反馈在各页面间保持视觉与体验一致。
- 以高效、高级、易用为原则，避免冗余装饰、模板化布局、无意义卡片堆叠和装饰性渐变。
- 优先复用已有公共组件与设计模式，必要时将可复用的新模式沉淀为公共组件。
- 让布局代替文本说话，禁止添加任何提示/解释/说明/翻译文案。

### 重构规范

**最小必要实现**：只有存在真实职责边界才引入接口与抽象，合并无意义拆分和纯转发包装，不为凑齐分层、套用设计模式、方便测试或未来扩展增加代码。

**单元测试**：若非用户直接要求，否则禁止创建任何测试文件，直接根据代码逻辑判断正确即可。

**数据演进**：若要切换数据结构，必须直接切换到新结构，可以直接覆盖并统一，禁止为旧结构做任何兼容、迁移、双读、双写或降级。

**重构范围**：每批重构仅限用户指定范围，明确涉及的入口、数据和调用方，不借关联关系扩大改动，发现范围外的问题先报告，不顺手重构其他模块。

**功能保留**：以当前实现和用户最新决定为基准，保留已有业务规则、功能和外部行为，允许改变内部实现，禁止以精简为由删减功能或擅自新增产品行为。

**字段设计**：每个字段必须有明确的当前用途和实际消费方，禁止保留无必要的重复语义、未来预留、低成本推导及仅为日志、trace 或测试存在的字段。

**日志约束**：日志只保留定位实际问题所需的信息，不额外建设 trace 链、诊断快照、指标平台或纯观测状态，也不为此增加接口、回调、配置或依赖。

### Java 规范

1. 需要写日志的类直接使用 `@Slf4j` 自动生成的 `log`，禁止注入或手动创建 `Logger`。
2. 枚举类统一使用 `@Getter` 和 `@AllArgsConstructor`，并且在 `value` 字段上标注 `@JsonValue`，在 `fromValue(String)` 上标注 `@JsonCreator`。
3. Controller、Service、Repository 等业务 Bean 将注入依赖声明为 `final`，并且通过 `@RequiredArgsConstructor` 生成构造器完成注入。
4. 对象创建统一通过 `Xxx.builder()` 按字段名赋值，并调用 `build()` 完成构建，禁止使用全参构造器。
5. 工具类的方法声明为 `final static`，并且统一使用 `@NoArgsConstructor(access = AccessLevel.PRIVATE)` 生成私有构造器。
6. 仅 Builder 构建链和 Stream 流水线要求逐步换行，同层调用的 `.` 对齐。若参数本身也是 Builder 构建链或 Stream 流水线，首个表达式紧跟左括号，后续调用再缩进一级，内层链结束后恢复外层缩进。简单字符串处理可写在一行，如 `name.trim().isEmpty()`。
7. 参数无效时使用 `invalid()`，资源缺失时使用 `notFound(code)`，状态或版本冲突时使用 `conflict(code)`，按需传入 `message` 覆盖 `defaultMessage`。
8. 按 ID 局部更新允许只传 ID 时，若所有 `SET` 项都是可选字段，Mapper 统一用 `id = id` 保证 SQL 有效；Service 按需根据影响行数判断资源是否存在。



