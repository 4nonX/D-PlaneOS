import { useState } from 'react'

/**
 * useState backed by localStorage. Reads the stored value on first render,
 * writes back on every update. Falls back to defaultValue when nothing is stored,
 * when the stored value can't be parsed, or when it isn't in the allowed set.
 *
 * @param key          localStorage key
 * @param defaultValue value used when nothing is stored (also determines type: string vs number)
 * @param allowed      optional set of valid values - guards against stale/renamed keys
 */
export function usePersistedState<T extends string | number>(
  key: string,
  defaultValue: T,
  allowed?: readonly T[]
): [T, (v: T) => void] {
  const [state, setRaw] = useState<T>(() => {
    const raw = localStorage.getItem(key)
    if (raw === null) return defaultValue
    const parsed = (typeof defaultValue === 'number' ? Number(raw) : raw) as T
    if (typeof parsed === 'number' && Number.isNaN(parsed)) return defaultValue
    if (allowed && !(allowed as readonly (string | number)[]).includes(parsed)) return defaultValue
    return parsed
  })

  function set(v: T) {
    localStorage.setItem(key, String(v))
    setRaw(v)
  }

  return [state, set]
}
