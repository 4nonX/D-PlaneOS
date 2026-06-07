/**
 * lib/listFilter.ts - opacity-based list filtering utility.
 *
 * Instead of removing non-matching items from the DOM (which causes layout
 * reflow and loses spatial context), items that don't match the query are
 * rendered at reduced opacity and become non-interactive. Items stay in
 * their original positions so operators can see what they searched past.
 *
 * Usage:
 *   const opacity = itemOpacity(query, item.name, item.path)
 *   <Row style={{ opacity, transition: 'opacity 0.12s', pointerEvents: opacity < 1 ? 'none' : undefined }} />
 *
 * Pass multiple text candidates: the highest-scoring one wins.
 */

/** Returns 1.0 for a matching item, 0.15 for a non-matching item. */
export function itemOpacity(query: string, ...candidates: string[]): number {
  if (!query.trim()) return 1
  const q = query.trim().toLowerCase()
  return candidates.some(t => t.toLowerCase().includes(q)) ? 1 : 0.15
}

/** True when an item matches the query (all candidates checked). */
export function itemMatches(query: string, ...candidates: string[]): boolean {
  if (!query.trim()) return true
  const q = query.trim().toLowerCase()
  return candidates.some(t => t.toLowerCase().includes(q))
}

/** Count matching items without filtering the array. */
export function matchCount<T>(items: T[], query: string, textFn: (item: T) => string[]): number {
  if (!query.trim()) return items.length
  const q = query.trim().toLowerCase()
  return items.filter(item => textFn(item).some(t => t.toLowerCase().includes(q))).length
}
