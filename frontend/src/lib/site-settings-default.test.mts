// @ts-nocheck
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const testDir = path.dirname(fileURLToPath(import.meta.url));
const srcDir = path.resolve(testDir, "..");

const hookSource = fs.readFileSync(
  path.join(srcDir, "hooks", "useSiteSettings.ts"),
  "utf8",
);
const scraperPageSource = fs.readFileSync(
  path.join(srcDir, "app", "scraper", "page.tsx"),
  "utf8",
);
const settingsPanelSource = fs.readFileSync(
  path.join(srcDir, "components", "SiteSettingsPanel.tsx"),
  "utf8",
);

assert.match(
  hookSource,
  /const defaultSettings:[\s\S]*?scraperEnabled:\s*true,/,
  "the global site-settings fallback should enable scraping",
);
assert.match(
  hookSource,
  /scraperEnabled:\s*data\.scraperEnabled\s*\?\?\s*true,/,
  "a missing scraperEnabled response should fall back to true",
);
assert.match(
  scraperPageSource,
  /setScraperEnabled\(data\.scraperEnabled\s*\?\?\s*true\)/,
  "the scraper page should treat a missing setting as enabled",
);
assert.match(
  scraperPageSource,
  /\.catch\(\(\)\s*=>\s*setScraperEnabled\(true\)\)/,
  "the scraper page request fallback should remain enabled",
);
assert.match(
  settingsPanelSource,
  /setConfig\(\{[\s\S]*?scraperEnabled:\s*true,[\s\S]*?\.\.\.data,/,
  "the settings panel should default a missing setting to true before applying saved data",
);

const resolveScraperEnabled = (value: boolean | null | undefined) => value ?? true;
assert.equal(resolveScraperEnabled(undefined), true);
assert.equal(resolveScraperEnabled(null), true);
assert.equal(
  resolveScraperEnabled(false),
  false,
  "an explicitly saved false value must not be overridden",
);

console.log("site-settings default tests passed");
