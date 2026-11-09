import { MAX_SLIDES_BYTES } from "@/shared/slides";

/**
 * What the file row shows right after a file is chosen (UI-05). A synchronous subset of
 * `precheckSlides` in the same order: size, empty, extension, filename. Magic bytes are
 * checked on submit, and the server stays the final judge (PRD §6.8). Never renames (OI-21).
 */
export type FileStatus =
  | { kind: "none" }
  | { kind: "error"; code: "FILE_TOO_LARGE" | "EMPTY_FILE" | "UNSUPPORTED_FILE_TYPE" }
  | { kind: "mismatch"; expected: string }
  | { kind: "match" }
  | { kind: "unknown" };

export function fileStatus(file: File | undefined, expectedBase: string | null): FileStatus {
  if (!file) return { kind: "none" };
  if (file.size > MAX_SLIDES_BYTES) return { kind: "error", code: "FILE_TOO_LARGE" };
  if (file.size === 0) return { kind: "error", code: "EMPTY_FILE" };
  const name = file.name.normalize("NFC");
  const dot = name.lastIndexOf(".");
  const ext = dot < 0 ? "" : name.slice(dot + 1).toLowerCase();
  if (ext !== "pdf" && ext !== "pptx") return { kind: "error", code: "UNSUPPORTED_FILE_TYPE" };
  // Without a valid category, org and name there is nothing to compare with yet.
  if (expectedBase === null) return { kind: "unknown" };
  const expected = expectedBase.normalize("NFC");
  return name.slice(0, dot) === expected ? { kind: "match" } : { kind: "mismatch", expected: `${expected}.${ext}` };
}

/** `size` in KB below 1 MiB, else in MB with one decimal (binary units, as the 5 MiB limit). */
export function formatSize(size: number, locale: string): string {
  const fmt = (n: number, digits: number) =>
    new Intl.NumberFormat(locale, { maximumFractionDigits: digits, minimumFractionDigits: digits }).format(n);
  return size < 1024 * 1024 ? `${fmt(Math.max(1, Math.ceil(size / 1024)), 0)} KB` : `${fmt(size / (1024 * 1024), 1)} MB`;
}

/** The limit, from the same constant as the size check. */
export const MAX_SLIDES_MIB = MAX_SLIDES_BYTES / (1024 * 1024);
