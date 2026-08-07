import assert from "node:assert/strict";
import fs from "node:fs";

const storageModule = await import("../src/lib/reader-options-storage.ts");
const {
  READER_OPTIONS_BY_WORK_KEY,
  resolveReaderOptions,
  splitReaderOptionsUpdate,
} = storageModule;

assert.equal(READER_OPTIONS_BY_WORK_KEY, "reader-options-by-work-v1");

const defaults = {
  mode: "single",
  direction: "ltr",
  doubleCoverAlone: false,
  doublePageNoGap: true,
  infiniteScroll: false,
  continuousReading: false,
  fitMode: "container",
  containerWidth: "",
  preloadCount: 2,
  headerVisible: true,
};

const resolved = resolveReaderOptions(
  defaults,
  { mode: "double", preloadCount: 15, headerVisible: false },
  {
    "work:demo": {
      mode: "webtoon",
      direction: "ttb",
      continuousReading: true,
    },
  },
  "work:demo",
);

assert.deepEqual(resolved, {
  ...defaults,
  mode: "webtoon",
  direction: "ttb",
  continuousReading: true,
  preloadCount: 15,
  headerVisible: false,
});

const partitioned = splitReaderOptionsUpdate({
  mode: "double",
  direction: "rtl",
  preloadCount: 20,
  headerVisible: false,
});

assert.deepEqual(partitioned.scoped, {
  mode: "double",
  direction: "rtl",
});
assert.deepEqual(partitioned.global, {
  preloadCount: 20,
  headerVisible: false,
});

const navbarSource = fs.readFileSync(
  new URL("../src/components/Navbar.tsx", import.meta.url),
  "utf8",
);
assert.match(
  navbarSource,
  /fixed top-0 left-0 right-0[^"]*lg:left-\[220px\][^"]*xl:left-\[240px\]/,
);
assert.match(
  navbarSource,
  /href="\/"[\s\S]{0,160}className="[^"]*lg:hidden/,
);

console.log("reader option scoping and desktop navbar contract: ok");
