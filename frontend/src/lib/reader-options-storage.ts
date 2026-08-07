import type { ReaderOptions } from "../types/reader.ts";

export const READER_OPTIONS_BY_WORK_KEY = "reader-options-by-work-v1";

export const WORK_SCOPED_READER_OPTION_KEYS = [
  "mode",
  "direction",
  "doubleCoverAlone",
  "doublePageNoGap",
  "infiniteScroll",
  "continuousReading",
  "fitMode",
  "containerWidth",
] as const satisfies readonly (keyof ReaderOptions)[];

const scopedKeySet = new Set<keyof ReaderOptions>(
  WORK_SCOPED_READER_OPTION_KEYS,
);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function resolveReaderOptions<T extends ReaderOptions>(
  defaults: T,
  globalStored: unknown,
  scopedStored: unknown,
  scopeId?: string,
): T {
  const globalOptions = isRecord(globalStored) ? globalStored : {};
  const scopedMap = isRecord(scopedStored) ? scopedStored : {};
  const rawScopedOptions =
    scopeId && isRecord(scopedMap[scopeId]) ? scopedMap[scopeId] : {};
  const scopedOptions: Partial<ReaderOptions> = {};

  for (const key of WORK_SCOPED_READER_OPTION_KEYS) {
    if (key in rawScopedOptions) {
      Object.assign(scopedOptions, { [key]: rawScopedOptions[key] });
    }
  }

  return {
    ...defaults,
    ...globalOptions,
    ...scopedOptions,
  } as T;
}

export function splitReaderOptionsUpdate(
  partial: Partial<ReaderOptions>,
): {
  scoped: Partial<ReaderOptions>;
  global: Partial<ReaderOptions>;
} {
  const scoped: Partial<ReaderOptions> = {};
  const global: Partial<ReaderOptions> = {};

  for (const [rawKey, value] of Object.entries(partial)) {
    const key = rawKey as keyof ReaderOptions;
    Object.assign(scopedKeySet.has(key) ? scoped : global, { [key]: value });
  }

  return { scoped, global };
}
