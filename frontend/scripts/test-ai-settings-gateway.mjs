import assert from "node:assert/strict";
import fs from "node:fs";

const source = fs.readFileSync(
  new URL("../src/components/AISettingsPanel.tsx", import.meta.url),
  "utf8",
);
const detailPanel = fs.readFileSync(
  new URL("../src/components/scraper/DetailPanel.tsx", import.meta.url),
  "utf8",
);

assert.match(
  source,
  /fetch\(apiPath\("\/api\/ai\/test"\),\s*\{[\s\S]*?body:\s*JSON\.stringify\(config\)/,
  "connection testing must send the current unsaved form configuration",
);
assert.doesNotMatch(
  source,
  /handleTestConnection[\s\S]{0,900}method:\s*"PUT"/,
  "connection testing must not save configuration as a hidden side effect",
);
assert.match(
  source,
  /fetch\(apiPath\("\/api\/ai\/models"\),\s*\{[\s\S]*?method:\s*"POST"[\s\S]*?body:\s*JSON\.stringify\(config\)/,
  "model discovery must submit the current configuration without putting the API key in the URL",
);
assert.match(source, /status\?:\s*string/, "fetched model objects must preserve provider status");
assert.match(source, /requestId/, "connection errors must expose provider request IDs");
assert.match(source, /statusCode/, "connection errors must expose provider HTTP status");
assert.match(source, /errorType/, "connection errors must expose their classified type");
assert.match(source, /pre-offline/, "the UI must visibly label models that are going offline");
assert.match(
  source,
  /Math\.min\(2,\s*parseInt\(e\.target\.value\)/,
  "retry input must be capped at two retries",
);
assert.match(
  detailPanel,
  /engine !== undefined \? engine : translateEngine/,
  "selecting the automatic translation engine must not fall back to the previous engine",
);
assert.match(
  detailPanel,
  /data-testid="metadata-translate-error"/,
  "metadata translation errors must remain visible outside edit mode",
);

console.log("AI settings gateway UI tests passed.");
