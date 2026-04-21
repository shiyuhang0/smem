export type MemorySummary = {
  id: string
  content: string
  type?: string
  kind?: string
  kinds?: string[]
  scope: string
  state: string
  metadata?: Record<string, unknown>
  store_count?: number
  use_count?: number
  created_at?: string
  updated_at?: string
}

export type MemoryDetail = MemorySummary

export type MemoryListResponse = {
  items: MemorySummary[]
  page: number
  page_size: number
  total: number
  total_pages: number
  has_more: boolean
}

export type MemoryKindCount = {
  kind: string
  count: number
}

export type MemoryKindListResponse = {
  items: MemoryKindCount[]
}

export type MemoryListParams = {
  page: number
  pageSize: number
  search: string
  kind: string
  state: string
}
