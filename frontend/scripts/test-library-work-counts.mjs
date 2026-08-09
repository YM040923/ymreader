import fs from "node:fs";
import path from "node:path";

const source = fs.readFileSync(
  path.resolve("src/api/libraries.ts"),
  "utf8",
);

if (source.includes('fetch(apiPath("/api/series"))')) {
  throw new Error("accessible library counts must not be reduced by legacy series data");
}
if (source.includes("groupedSavings")) {
  throw new Error("legacy groupedSavings adjustment is still present");
}
if (!source.includes("library.workCount ?? library.comicCount")) {
  throw new Error("accessible library count must prefer server workCount");
}

console.log("library Work count contract passed");
