import type { MemorySummary } from './types'

export function getMemoryKinds(memory: Pick<MemorySummary, 'kind' | 'kinds'>): string[] {
  if (memory.kinds && memory.kinds.length > 0) {
    return memory.kinds
  }

  if (memory.kind) {
    return [memory.kind]
  }

  return []
}

export function formatTimestamp(value?: string): string {
  if (!value) {
    return 'Unknown update time'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return new Intl.DateTimeFormat('en', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}
