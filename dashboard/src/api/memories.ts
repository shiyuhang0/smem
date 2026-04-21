import { apiFetch } from './client'
import type {
  MemoryDetail,
  MemoryKindListResponse,
  MemoryListParams,
  MemoryListResponse,
} from '../lib/types'

export function listMemories(input: MemoryListParams) {
  const params = new URLSearchParams({
    page: String(input.page),
    page_size: String(input.pageSize),
  })

  if (input.search) {
    params.set('search', input.search)
  }

  if (input.kind) {
    params.set('kind', input.kind)
  }

  if (input.state) {
    params.set('state', input.state)
  }

  return apiFetch<MemoryListResponse>(`/api/v1/memories?${params.toString()}`)
}

export function listMemoryKinds() {
  return apiFetch<MemoryKindListResponse>('/api/v1/memories/kinds?limit=10')
}

export function getMemory(id: string) {
  return apiFetch<MemoryDetail>(`/api/v1/memories/${id}`)
}

export function updateMemoryState(id: string, state: string) {
  return apiFetch<MemoryDetail>(`/api/v1/memories/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ state }),
  })
}
