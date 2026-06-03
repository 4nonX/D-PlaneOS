/**
 * Format a date string or timestamp using the user's locale.
 * Replaces hardcoded 'de-DE' locale across all pages.
 */
export function fmtDate(raw: string | number | Date, opts?: Intl.DateTimeFormatOptions): string {
  try {
    return new Date(raw).toLocaleDateString(undefined, opts)
  } catch {
    return String(raw)
  }
}

export function fmtDateTime(raw: string | number | Date): string {
  try {
    return new Date(raw).toLocaleString(undefined)
  } catch {
    return String(raw)
  }
}
