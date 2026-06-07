import { useState, useRef, useCallback, useEffect, useMemo } from 'react'

/**
 * useFrozenLayout - separates structural layout from live metrics.
 *
 * The problem: polling with refetchInterval causes full re-renders on every tick.
 * When the returned data contains the same items with updated values, React still
 * diffs the whole tree and the browser can shift card positions, trigger layout
 * recalculations, and lose scroll position mid-read.
 *
 * The solution: maintain two views of the same data:
 *   snapshot  - which items exist and in what order. Updated only when items are
 *               added/removed, or when refreshKey changes. Use this for the list
 *               structure (the map() call, key prop, position in DOM).
 *   liveById  - current values by ID. Always up-to-date on every poll tick. Use
 *               this to read per-item metrics inside each rendered card.
 *
 * Usage:
 *   const { snapshot, liveById, forceRefresh } = useFrozenLayout(
 *     pools,          // T[] | undefined - live data from useQuery
 *     p => p.name,    // keyFn: stable unique ID for each item
 *     refreshKey,     // number - increment to force structural update (post-mutation)
 *   )
 *
 *   // Render structure from snapshot (stable), metrics from liveById (live):
 *   {(snapshot ?? pools)?.map(pool => (
 *     <PoolCard key={pool.name} pool={liveById.get(pool.name) ?? pool} />
 *   ))}
 *
 * Structural update triggers:
 *   - First data load
 *   - Any item ID appears in data that is not in the current snapshot
 *   - Any item ID in the snapshot no longer appears in data
 *   - refreshKey argument changes (caller signals a known structural change)
 *   - forceRefresh() is called (e.g., explicit refresh button)
 *
 * Non-structural updates (snapshot unchanged, liveById updated silently):
 *   - Same set of item IDs, different field values (health, capacity, state, etc.)
 */
export function useFrozenLayout<T>(
  data: T[] | undefined,
  keyFn: (item: T) => string,
  refreshKey = 0,
): {
  snapshot: T[] | undefined
  liveById: Map<string, T>
  forceRefresh: () => void
} {
  const [snapshot, setSnapshot] = useState<T[] | undefined>(undefined)
  const snapshotKeysRef  = useRef<Set<string>>(new Set())
  const initializedRef   = useRef(false)
  const lastRefreshKeyRef = useRef(refreshKey)

  // Always-current lookup map - never causes re-renders on its own
  const liveById = useMemo<Map<string, T>>(() => {
    const m = new Map<string, T>()
    data?.forEach(item => m.set(keyFn(item), item))
    return m
  }, [data, keyFn])

  useEffect(() => {
    if (data === undefined) return

    const refreshForced = refreshKey !== lastRefreshKeyRef.current
    lastRefreshKeyRef.current = refreshKey

    const newKeys = data.map(keyFn)
    const newKeySet = new Set(newKeys)

    // First load or forced refresh: always update snapshot
    if (!initializedRef.current || refreshForced) {
      setSnapshot([...data])
      snapshotKeysRef.current = newKeySet
      initializedRef.current = true
      return
    }

    // Structural change detection: added or removed items
    const hasAdded   = newKeys.some(k => !snapshotKeysRef.current.has(k))
    const hasRemoved = [...snapshotKeysRef.current].some(k => !newKeySet.has(k))

    if (hasAdded || hasRemoved) {
      setSnapshot([...data])
      snapshotKeysRef.current = newKeySet
      return
    }

    // Only metrics changed: liveById updated silently, snapshot stays frozen
  }, [data, keyFn, refreshKey])

  const forceRefresh = useCallback(() => {
    if (data !== undefined) {
      setSnapshot([...data])
      snapshotKeysRef.current = new Set(data.map(keyFn))
    }
  }, [data, keyFn])

  return { snapshot, liveById, forceRefresh }
}
