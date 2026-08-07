# Unified AI Gateway Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every YMReader cloud-AI feature use one reliable request path that supports both Chat Completions and Responses endpoints, validates real model availability, avoids wasteful retries, and reports actionable Chinese errors.

**Architecture:** Add a protocol-aware endpoint resolver and typed provider error layer beneath the existing `CallCloudLLM` API. Keep existing business services intact while routing non-streaming and streaming OpenAI-compatible calls through shared URL, retry, response, and usage rules. Update the settings API and UI so connection tests use the current form values and the exact same gateway as production AI features.

**Tech Stack:** Go 1.23, `net/http`, Gin, `httptest`, React/TypeScript, existing frontend script tests, Docker.

---

## File Structure

- Create `internal/service/ai_gateway.go`: endpoint resolution, protocol enum, typed provider errors, retry classification, request-ID parsing.
- Create `internal/service/ai_gateway_test.go`: URL, protocol, error and retry tests.
- Modify `internal/service/ai_llm.go`: use gateway for Chat Completions and Responses requests.
- Modify `internal/service/ai_stream.go`: use the same endpoint resolver and fail clearly for unsupported Responses streaming.
- Modify `internal/service/ai_usage.go`: record protocol and error category without inventing usage.
- Modify `internal/service/ai_config.go`: sanitize retry limits and configuration whitespace.
- Modify `internal/handler/ai_config_handler.go`: test current submitted configuration and return structured diagnostics.
- Create `internal/handler/ai_config_handler_test.go`: handler-level test parity and error mapping.
- Modify `frontend/src/components/AISettingsPanel.tsx`: submit current form values, show model status and exact errors.
- Create `frontend/scripts/test-ai-settings-gateway.mjs`: source-level UI regression checks.

### Task 1: Endpoint Resolution and Typed Errors

**Files:**
- Create: `internal/service/ai_gateway.go`
- Create: `internal/service/ai_gateway_test.go`

- [ ] **Step 1: Write failing endpoint-resolution tests**

```go
func TestResolveOpenAIEndpoint(t *testing.T) {
    cases := []struct {
        raw      string
        wantURL  string
        protocol AIProtocol
    }{
        {"https://example.test/v1", "https://example.test/v1/chat/completions", AIProtocolChatCompletions},
        {"https://example.test/v1/", "https://example.test/v1/chat/completions", AIProtocolChatCompletions},
        {"https://example.test/v1/chat/completions", "https://example.test/v1/chat/completions", AIProtocolChatCompletions},
        {"https://example.test/v1/responses", "https://example.test/v1/responses", AIProtocolResponses},
    }
    for _, tc := range cases {
        got, err := resolveOpenAIEndpoint(tc.raw)
        if err != nil {
            t.Fatalf("resolveOpenAIEndpoint(%q): %v", tc.raw, err)
        }
        if got.URL != tc.wantURL || got.Protocol != tc.protocol {
            t.Fatalf("got %#v, want url=%q protocol=%q", got, tc.wantURL, tc.protocol)
        }
    }
}
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```powershell
go test ./internal/service -run 'TestResolveOpenAIEndpoint' -count=1
```

Expected: compile failure because `AIProtocol` and `resolveOpenAIEndpoint` do not exist.

- [ ] **Step 3: Implement the minimal resolver and typed error**

```go
type AIProtocol string

const (
    AIProtocolChatCompletions AIProtocol = "chat_completions"
    AIProtocolResponses       AIProtocol = "responses"
)

type ResolvedAIEndpoint struct {
    URL      string
    Protocol AIProtocol
}

type AIProviderError struct {
    Kind       string
    StatusCode int
    Message    string
    RequestID  string
    Retryable  bool
}
```

`resolveOpenAIEndpoint` must trim whitespace and trailing slashes, validate `http`/`https`, preserve complete endpoints, and append `/chat/completions` only to base URLs.

- [ ] **Step 4: Add failing error-classification tests**

Cover 400 model errors, 401, 404, 429, 500, timeout and `Retry-After`.

- [ ] **Step 5: Implement error parsing and retry classification**

Add helpers:

```go
func parseAIProviderError(resp *http.Response, body []byte) *AIProviderError
func shouldRetryAIError(err error) bool
func providerRequestID(resp *http.Response, body []byte) string
```

- [ ] **Step 6: Run focused tests**

```powershell
go test ./internal/service -run 'TestResolveOpenAIEndpoint|TestAIProviderError|TestShouldRetryAIError' -count=1
```

- [ ] **Step 7: Commit**

```powershell
git add internal/service/ai_gateway.go internal/service/ai_gateway_test.go
git commit -m "feat: add protocol-aware AI gateway primitives"
```

### Task 2: Chat Completions and Responses Adapters

**Files:**
- Modify: `internal/service/ai_llm.go`
- Modify: `internal/service/ai_metadata_reliability_test.go`
- Modify: `internal/service/ai_gateway_test.go`

- [ ] **Step 1: Write a failing Responses adapter test**

Use `httptest.Server` to assert the request contains `model`, `instructions`, `input`, and `max_output_tokens`, then return:

```json
{
  "status": "completed",
  "output": [{
    "type": "message",
    "content": [{"type": "output_text", "text": "{\"title\":\"药屋少女的呢喃\"}"}]
  }],
  "usage": {"input_tokens": 20, "output_tokens": 8, "total_tokens": 28}
}
```

Assert text and all usage fields are parsed.

- [ ] **Step 2: Run the Responses test and verify RED**

```powershell
go test ./internal/service -run 'TestCallOpenAICompatibleResponses' -count=1
```

Expected: request reaches the wrong `/chat/completions` path or cannot parse `output`.

- [ ] **Step 3: Split protocol adapters**

Add:

```go
func callOpenAIChatCompletions(
    cfg AIConfig,
    endpoint ResolvedAIEndpoint,
    systemPrompt, userPrompt string,
    maxTokens int,
    temperature float64,
    images []ImageContent,
    opts *LLMCallOptions,
) (string, tokenUsage, error)

func callOpenAIResponses(
    cfg AIConfig,
    endpoint ResolvedAIEndpoint,
    systemPrompt, userPrompt string,
    maxTokens int,
    temperature float64,
    images []ImageContent,
    opts *LLMCallOptions,
) (string, tokenUsage, error)
```

Make `callOpenAICompatibleWithOptions` resolve once and dispatch by protocol.

- [ ] **Step 4: Add response edge-case tests**

Test top-level `output_text`, nested `output_text`, empty output, malformed JSON, failed status, incomplete status and truncated output.

- [ ] **Step 5: Preserve Chat Completions behavior**

Run existing structured translation and truncation tests. Update request assertions only where the unified resolver changes the URL.

- [ ] **Step 6: Run focused tests**

```powershell
go test ./internal/service -run 'OpenAICompatible|Responses|TranslateMetadataFields' -count=1
```

- [ ] **Step 7: Commit**

```powershell
git add internal/service/ai_llm.go internal/service/ai_gateway_test.go internal/service/ai_metadata_reliability_test.go
git commit -m "feat: support chat completions and responses AI APIs"
```

### Task 3: Correct Retries, Token Limits and Usage

**Files:**
- Modify: `internal/service/ai_llm.go`
- Modify: `internal/service/ai_usage.go`
- Modify: `internal/service/ai_config.go`
- Modify: `internal/service/ai_gateway_test.go`

- [ ] **Step 1: Write failing retry tests**

Create table tests proving:

- 400, 401, 403 and 404 produce exactly one request.
- 429 and 503 retry up to configured limit.
- timeout retries.
- a successful second request returns normally.

- [ ] **Step 2: Verify RED**

```powershell
go test ./internal/service -run 'TestCloudLLMRetry' -count=1
```

- [ ] **Step 3: Replace unconditional retries**

In `CallCloudLLM`, retry only when `shouldRetryAIError(err)` is true. Cap retries to `0..2` when loading/saving configuration. Respect `Retry-After` when present; otherwise use bounded exponential backoff.

- [ ] **Step 4: Add scenario token budgets**

Keep caller overrides, but use small defaults for translation and connection tests. Translation requests must not inherit a global `20000` output-token value when a few hundred tokens are enough.

- [ ] **Step 5: Record accurate usage**

Extend usage records with protocol and error category. Record actual provider usage from every attempted request; do not invent tokens for failures without usage.

- [ ] **Step 6: Run tests**

```powershell
go test ./internal/service -run 'Retry|Usage|TranslateMetadataFields' -count=1
```

- [ ] **Step 7: Commit**

```powershell
git add internal/service/ai_llm.go internal/service/ai_usage.go internal/service/ai_config.go internal/service/ai_gateway_test.go
git commit -m "fix: make AI retries and usage accounting predictable"
```

### Task 4: Streaming Endpoint Consistency

**Files:**
- Modify: `internal/service/ai_stream.go`
- Create or modify: `internal/service/ai_stream_test.go`

- [ ] **Step 1: Write failing stream URL tests**

Assert a base URL produces `/chat/completions`, a full Chat endpoint is preserved, and a Responses endpoint returns a clear `unsupported_stream_protocol` error instead of appending another path.

- [ ] **Step 2: Verify RED**

```powershell
go test ./internal/service -run 'TestCloudLLMStreamEndpoint' -count=1
```

- [ ] **Step 3: Reuse the resolver**

Call `resolveOpenAIEndpoint` from `CallCloudLLMStream`. Route Chat Completions to existing SSE parsing. Until a verified Responses streaming adapter exists, return a typed non-retryable error explaining that the configured Responses endpoint supports non-streaming features but not the current streaming reader assistant.

- [ ] **Step 4: Run streaming tests**

```powershell
go test ./internal/service -run 'Stream' -count=1
```

- [ ] **Step 5: Commit**

```powershell
git add internal/service/ai_stream.go internal/service/ai_stream_test.go
git commit -m "fix: share AI endpoint rules with streaming calls"
```

### Task 5: Real Connection Test and Model Diagnostics

**Files:**
- Modify: `internal/handler/ai_config_handler.go`
- Create: `internal/handler/ai_config_handler_test.go`
- Modify: `internal/service/ai_config.go`

- [ ] **Step 1: Write failing handler tests**

POST a JSON body containing unsaved `cloudProvider`, `cloudApiUrl`, masked-or-new key, `cloudModel`, `maxTokens`, and `maxRetries`. Assert the handler uses this submitted config rather than only `LoadAIConfig()`.

- [ ] **Step 2: Verify RED**

```powershell
go test ./internal/handler -run 'TestAIConnection' -count=1
```

- [ ] **Step 3: Accept current form configuration**

Use:

```go
type AITestRequest struct {
    service.AIConfig
}
```

If the request body is empty, retain backward compatibility by loading saved config. If the key is masked, substitute the saved key. Invoke the same `service.TranslateMetadataFields` path used by actual translation.

- [ ] **Step 4: Return structured diagnostics**

Success response includes:

```json
{
  "success": true,
  "reply": "药屋少女的呢喃",
  "protocol": "responses",
  "model": "example-model"
}
```

Failure response includes safe fields:

```json
{
  "success": false,
  "error": "模型或服务 ID 不存在",
  "errorType": "model_unavailable",
  "statusCode": 400,
  "requestId": "..."
}
```

- [ ] **Step 5: Expose model status**

When the provider model endpoint returns objects, preserve `id`, `name`, and `status` instead of flattening everything to strings.

- [ ] **Step 6: Run handler tests**

```powershell
go test ./internal/handler -run 'AI.*Connection|AI.*Models' -count=1
```

- [ ] **Step 7: Commit**

```powershell
git add internal/handler/ai_config_handler.go internal/handler/ai_config_handler_test.go internal/service/ai_config.go
git commit -m "fix: make AI connection tests match real calls"
```

### Task 6: AI Settings UI and Error Visibility

**Files:**
- Modify: `frontend/src/components/AISettingsPanel.tsx`
- Create: `frontend/scripts/test-ai-settings-gateway.mjs`
- Modify: `frontend/package.json` only if the existing test command needs registration

- [ ] **Step 1: Write a failing frontend regression script**

Assert source behavior includes:

- POST `/api/ai/test` with the current config JSON.
- rendering `error`, `errorType`, `statusCode`, and `requestId`.
- rendering model status such as `online` or `pre-offline`.
- no mandatory save-before-test request.

- [ ] **Step 2: Run and verify RED**

```powershell
node frontend/scripts/test-ai-settings-gateway.mjs
```

- [ ] **Step 3: Update connection testing**

Send:

```ts
body: JSON.stringify(config)
```

Do not silently save before testing. Show the returned protocol and model on success.

- [ ] **Step 4: Improve model and error rendering**

Display provider model status beside the model name. Render exact server errors in Chinese and keep the request ID copyable without exposing credentials.

- [ ] **Step 5: Run frontend checks**

```powershell
node frontend/scripts/test-ai-settings-gateway.mjs
Set-Location frontend
npx tsc -b
npm run build
```

- [ ] **Step 6: Commit**

```powershell
git add frontend/src/components/AISettingsPanel.tsx frontend/scripts/test-ai-settings-gateway.mjs frontend/package.json
git commit -m "fix: make AI settings tests and errors trustworthy"
```

### Task 7: Full Regression and Deployment

**Files:**
- Modify version/build metadata files used by the current Docker build.
- No changes to manga files, libraries or the production database schema.

- [ ] **Step 1: Run the complete backend suite**

```powershell
go test ./... -count=1
```

Expected: zero failures.

- [ ] **Step 2: Run complete frontend checks**

```powershell
Set-Location frontend
npx tsc -b
npm run build
```

Expected: successful typecheck and production build.

- [ ] **Step 3: Build a complete source archive**

Archive the complete Git HEAD, excluding `.git`, build output, caches and `node_modules`. Do not upload only changed files.

- [ ] **Step 4: Back up production configuration**

On the NAS, create a timestamped backup of:

- `/vol3/1000/docker/nowen-reader/data`
- `/vol3/1000/docker/nowen-reader/cache/ai-config.json`
- `/vol1/1000/compose/NowenReader/docker-compose.yml`

- [ ] **Step 5: Build a new NAS image**

Replace `/vol3/1000/ymreader-unified-work-clean-src` with the complete source archive and build an incremented image tag.

- [ ] **Step 6: Replace only the formal `nowen-reader` container**

Preserve all existing mounts, environment, account data and libraries. Do not start old test containers and do not create manga directories.

- [ ] **Step 7: Verify health and static assets**

Check Docker health, root HTTP 200, settings page load, and absence of new asset 404s, SQLite locks or panics.

### Task 8: Production Functional Smoke Test

**Files:**
- No source changes unless the smoke test exposes a reproducible defect.

- [ ] **Step 1: Test the configured model**

Use the real settings page connection test. If the configured model is rejected, show the real provider error and select a user-authorized valid model; do not silently substitute one.

- [ ] **Step 2: Test metadata translation**

Translate one work with English metadata. Confirm the request succeeds, the translated fields appear, and the operation uses a bounded token count.

- [ ] **Step 3: Test a second AI function**

Run one AI description, tag, category or filename operation and confirm it uses the same gateway and error behavior.

- [ ] **Step 4: Re-test clear metadata and cover replacement**

Verify the previously repaired non-AI operations still work after the new build.

- [ ] **Step 5: Inspect logs**

Confirm there are no double-appended URLs, repeated deterministic 4xx calls, API-key leakage, asset 404s, panics or database locks.

- [ ] **Step 6: Tag and push**

```powershell
git tag ymreader-v2026.08.07.<next>
git push fork ymreader-final
git push fork ymreader-v2026.08.07.<next>
```

