import type { MaintenanceStatus, ResourceInfo, Weekday } from '@/types'

export const WEEKDAYS: { key: Weekday; label: string }[] = [
  { key: 'mon', label: 'Mon' },
  { key: 'tue', label: 'Tue' },
  { key: 'wed', label: 'Wed' },
  { key: 'thu', label: 'Thu' },
  { key: 'fri', label: 'Fri' },
  { key: 'sat', label: 'Sat' },
  { key: 'sun', label: 'Sun' },
]

/**
 * One-line status, e.g. "Open until Wed 02:00" or "Closed · next Sat 06:00".
 * The server pre-formats the times in its own timezone.
 */
export function maintenanceSummary(status?: MaintenanceStatus): string {
  if (!status) return ''
  if (status.open) return status.currentEndText ? `Open until ${status.currentEndText}` : 'Open'
  return status.nextStartText ? `Closed · next ${status.nextStartText}` : 'Closed'
}

/** "Mon, Tue" in weekday order. */
export function daysLabel(days: Weekday[]): string {
  return WEEKDAYS.filter(d => days.includes(d.key)).map(d => d.label).join(', ')
}

/**
 * Mirrors the server's Window.Matches selector part: every key=value must be
 * present in the cluster's live Omni labels or its Git template labels.
 */
export function matchesSelector(c: Pick<ResourceInfo, 'labels' | 'templateLabels'>, selector: Record<string, string>): boolean {
  const keys = Object.keys(selector)
  if (!keys.length) return false
  return keys.every(k =>
    (c.labels && k in c.labels && c.labels[k] === selector[k]) ||
    (c.templateLabels && k in c.templateLabels && c.templateLabels[k] === selector[k]),
  )
}

/**
 * Explains why a selector key=value matches no cluster, e.g. when "dev" was
 * entered as the key although clusters have env=dev. Returns '' when the pair
 * matches at least one cluster or the key is empty.
 */
export function selectorHint(
  clusters: Pick<ResourceInfo, 'labels' | 'templateLabels'>[],
  key: string,
  value: string,
): string {
  if (!key) return ''
  const all = clusters.flatMap(c => [...Object.entries(c.labels ?? {}), ...Object.entries(c.templateLabels ?? {})])
  if (all.some(([k, v]) => k === key && v === value)) return ''
  const sameKey = [...new Set(all.filter(([k]) => k === key).map(([, v]) => v))].sort()
  if (sameKey.length) {
    return `No cluster has ${key} = ${value || '""'}. Values in use: ${sameKey.map(v => v || '""').join(', ')}.`
  }
  const asValue = [...new Set(all.filter(([, v]) => v === key).map(([k]) => k))].sort()
  if (asValue.length) return `No cluster has a label named "${key}". Did you mean ${asValue.map(k => `${k} = ${key}`).join(' or ')}?`
  return `No cluster has a label named "${key}" yet.`
}

