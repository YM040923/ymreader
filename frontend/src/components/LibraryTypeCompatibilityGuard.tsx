"use client";

import { useEffect } from "react";

const LEGACY_MIXED_OPTION_SELECTOR = 'select option[value="mixed"]';

/**
 * Removes the retired mixed-library option from legacy forms that may still be
 * present in an older component chunk. The database migration converts stored
 * mixed libraries before the UI loads, while this guard keeps both current and
 * temporarily cached frontend bundles limited to comic and novel choices.
 */
export function LibraryTypeCompatibilityGuard() {
  useEffect(() => {
    const removeLegacyOptions = () => {
      document
        .querySelectorAll<HTMLOptionElement>(LEGACY_MIXED_OPTION_SELECTOR)
        .forEach((option) => option.remove());
    };

    removeLegacyOptions();
    const observer = new MutationObserver(removeLegacyOptions);
    observer.observe(document.body, { childList: true, subtree: true });

    return () => observer.disconnect();
  }, []);

  return null;
}
