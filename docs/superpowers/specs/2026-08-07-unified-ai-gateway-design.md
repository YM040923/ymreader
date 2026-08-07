# YMReader 统一 AI 网关设计

## 背景

当前云端 AI 功能分散在多个 handler 和 service 中。连接测试、元数据翻译、AI 刮削、简介生成、标签建议、分类、文件名解析、封面分析等功能虽然共享部分配置，却没有共享完整的请求构造、端点解析、错误分类和响应解析规则。

正式环境已经出现可稳定复现的故障：

- 配置地址为完整 Responses API 地址 `.../v1/responses`。
- 现有 OpenAI-compatible 调用无条件追加 `/chat/completions`。
- 最终请求变成 `.../v1/responses/chat/completions` 并返回 404。
- 当前模型列表接口能列出 `hy3-preview`，但实际推理接口返回“模型或服务 ID 不存在”。
- 连接测试与实际功能曾经使用不同的调用路径，导致“测试成功、实际失败”。
- 4xx 配置错误可能被重试并最终包装成模糊的 5xx。

## 目标

建立唯一的云端 AI 请求网关，使所有 AI 功能使用相同的配置解析、模型验证、端点适配、重试、用量统计和错误信息，并同时兼容：

1. OpenAI Chat Completions 风格接口。
2. OpenAI Responses 风格接口。
3. 用户输入基础 URL。
4. 用户输入完整 endpoint URL。
5. 现有 Anthropic 与 Gemini 专用协议。

本次不重做现有页面视觉，不改变漫画、书库、阅读和元数据的数据模型。

## 方案选择

### 方案 A：只修复 URL 重复拼接

优点是改动最小。缺点是连接测试、模型验证、Responses 解析、重试和其他 AI 功能仍然不一致，后续会继续出现同类问题。

### 方案 B：统一 AI 网关（采用）

建立协议无关的统一调用入口，并将现有功能迁移到该入口。工作量适中，但能从根本上解决当前问题，并为自定义兼容接口提供稳定行为。

### 方案 C：引入第三方多供应商 SDK

可以减少部分协议代码，但会引入新的依赖、兼容限制和升级风险；对当前 Go 项目不是必要条件。

## 核心架构

### Endpoint Resolver

负责把配置地址解析为明确协议和最终 URL：

- `https://host/v1` → Chat Completions，最终 URL 为 `.../v1/chat/completions`。
- `https://host/v1/` → 同上。
- `https://host/v1/chat/completions` → 原样使用。
- `https://host/v1/responses` → Responses，原样使用。
- 其他包含 `/responses` 或 `/chat/completions` 的完整地址不得再次追加路径。
- 地址缺少 scheme、为空或无法解析时，在发起请求前返回明确配置错误。

端点协议可以自动识别；后续如页面增加显式协议选择，自动识别仍作为兼容回退。

### Unified Cloud Client

新增统一请求接口，接收：

- system prompt
- user prompt
- 图片
- 最大输出 token
- temperature
- 是否要求 JSON
- 是否关闭思考
- 场景名称

输出：

- 文本结果
- prompt/output/total token
- provider、model、protocol
- finish reason
- request ID（供应商返回时）

Chat Completions 与 Responses 仅在 adapter 内部存在差异，业务层不再直接构造 HTTP 请求。

### Responses Adapter

使用 Responses 请求结构：

- `model`
- `instructions`
- `input`
- `max_output_tokens`
- 适用时的结构化 JSON 配置

解析顺序：

1. 顶层 `output_text`。
2. `output[].content[]` 中 `type=output_text` 的 `text`。
3. 对兼容供应商的常见文本字段进行有限回退。

同时解析 `input_tokens`、`output_tokens`、`total_tokens` 和完成状态。

### Chat Completions Adapter

保留现有 messages 协议，但：

- 使用 Endpoint Resolver 生成最终 URL。
- 不再直接进行字符串拼接。
- 统一解析错误体、请求 ID、finish reason 和 token。
- 结构化输出、关闭思考等扩展参数仅在对应 provider 支持时发送。

### 模型发现与验证

模型列表不能等同于模型可用：

- `GET /models` 用于候选列表。
- “测试连接”必须使用当前页面中的 provider、URL、API Key、model 发起一次最小真实推理。
- 测试请求与正式业务调用使用相同网关。
- 模型列表中处于 `pre-offline`、`offline` 等状态的模型在 UI 中标记状态。
- 推理返回“模型或服务 ID 不存在”时直接显示供应商原始中文信息，不自动换模型。

### 重试策略

只重试瞬时故障：

- 网络超时、连接重置。
- HTTP 408、409、425、429。
- HTTP 500、502、503、504。

不重试：

- 400 参数或模型错误。
- 401/403 鉴权错误。
- 404 URL 或模型路由错误。
- 其他确定性 4xx。

重试使用有上限的指数退避，并尊重 `Retry-After`。配置中的重试次数统一限制在安全范围内，避免因为误填 `5` 等旧值造成大量重复额度消耗。

### 错误模型

底层返回可分类错误：

- `invalid_config`
- `authentication`
- `endpoint_not_found`
- `model_unavailable`
- `rate_limited`
- `timeout`
- `provider_error`
- `invalid_response`
- `truncated`

错误包含 HTTP 状态、供应商消息、request ID、是否可重试。handler 根据错误类别返回合理 HTTP 状态和中文消息，不再把所有失败都包装成含糊的 500/502。

### 结构化输出与翻译

元数据翻译继续采用一次请求翻译整部作品的多个字段：

- 翻译默认关闭推理/思考能力。
- 采用较低 temperature。
- 根据输入长度计算合理输出上限，不直接使用全局 20000 token。
- 优先结构化 JSON。
- JSON 仅允许一次轻量纠错重试。
- 响应被截断时给出明确错误，不写入半截数据。

### 用量统计

所有云端 AI 功能从统一网关记录：

- 场景。
- provider/model/protocol。
- prompt、output、total token。
- 成功/失败。
- 耗时。
- 错误类别。

失败且供应商未返回 usage 时不得伪造 token。重试的每一次实际请求都计入用量，但 UI 同时显示逻辑操作次数，便于解释额度消耗。

## 业务迁移范围

以下功能必须统一迁移：

- AI 连接测试。
- 元数据字段翻译和批量翻译。
- AI 智能刮削。
- AI 搜索与候选识别。
- AI 简介生成和补全。
- 标签建议与分类。
- 文件名/标题解析。
- 封面和图片分析。
- AI 聊天、提示词和其他管理页面功能。
- 流式输出使用与统一 resolver 相同的端点规则和错误分类。

Anthropic 与 Gemini 保留专用 adapter，但也纳入统一错误、重试和用量模型。

## 前端行为

- 设置页面测试当前表单值，而不是只能测试已经保存的旧配置。
- 测试成功后显示实际协议、模型和简短响应。
- 测试失败显示准确中文原因、状态码和 request ID，不泄露 API Key。
- 模型列表显示供应商返回状态。
- AI 业务按钮显示后端返回的真实错误，不再静默恢复原状。
- 保存配置时规范化空白，但保留用户填写的完整 endpoint。

## 测试策略

后端使用 `httptest.Server` 覆盖：

- 基础 URL 和尾斜杠。
- 完整 `/chat/completions`。
- 完整 `/responses`。
- Chat 与 Responses 文本、JSON、usage 解析。
- 400/401/403/404 不重试。
- 429/5xx/超时按规则重试。
- `Retry-After`。
- 截断响应。
- 空响应和畸形 JSON。
- 模型不可用错误分类。
- 连接测试和正式翻译调用使用同一路径。

前端脚本测试覆盖：

- 测试请求携带当前未保存的表单值。
- 模型状态展示。
- 中文错误展示。
- 不吞掉翻译、刮削等 AI 操作失败。

正式部署后使用现有配置进行最小真实测试；不得在日志和终端输出 API Key。

## 部署和回滚

- 使用完整 Git HEAD 构建，不再局部上传源文件。
- 构建前备份数据库、缓存配置和 Compose。
- 新镜像使用递增版本标签。
- 替换正式容器后验证健康、静态资源、设置页面、翻译和至少一个其他 AI 功能。
- 保留上一正式镜像，确认稳定后再清理。

