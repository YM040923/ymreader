import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const panel = readFileSync(
  new URL("../src/components/SiteSettingsPanel.tsx", import.meta.url),
  "utf8",
);

assert.match(
  panel,
  /async function consumeBatchSSE\(/,
  "SiteSettingsPanel must use one strict SSE consumer for both batch tasks",
);
assert.match(
  panel,
  /if \(!response\.ok\)\s*\{[\s\S]*throw new Error/,
  "HTTP non-2xx responses must fail before the UI can report success",
);
assert.match(
  panel,
  /terminalType:\s*"complete"\s*\|\s*"done"/,
  "the consumer must distinguish standard scrape complete from translation done",
);
assert.match(
  panel,
  /event\.type === terminalType/,
  "the configured terminal event must be required",
);
assert.match(
  panel,
  /throw new Error\([^)]*(?:中断|完成事件)/,
  "an SSE stream ending before its terminal event must be reported as an error",
);
assert.doesNotMatch(
  panel,
  /catch\s*\{\s*\/\*\s*skip\s*\*\/\s*\}/,
  "malformed SSE events must not be silently ignored",
);
assert.match(
  panel,
  /setBatchDone\(await consumeBatchSSE\(res,\s*"complete"/,
  "standard metadata scraping must finish on the backend complete event",
);
assert.match(
  panel,
  /const result = await consumeBatchSSE\(res,\s*"done"/,
  "metadata translation must finish on the backend done event",
);
assert.match(
  panel,
  /success:\s*result\.translated\s*\?\?\s*0/,
  "translation results must expose translated as the UI success count",
);
assert.match(
  panel,
  /batchProgress\.current/,
  "standard scrape progress must use the backend current field",
);
assert.match(
  panel,
  /translateProgress\.current/,
  "translation progress must use the backend current field",
);
assert.match(
  panel,
  /batchError\s*&&[\s\S]*text-red-/,
  "standard scrape transport/protocol errors must be rendered as errors",
);
assert.match(
  panel,
  /translateError\s*&&[\s\S]*text-red-/,
  "translation transport/protocol errors must be rendered as errors",
);

console.log("Site settings SSE contract tests passed.");
