import { useQuery } from '@tanstack/react-query'

import { getMemory } from '../api/memories'

const AUTO_REFRESH_MS = 30_000

export function useMemoryDetail(memoryId: string | null) {
  return useQuery({
    queryKey: ['memory-detail', memoryId],
    queryFn: () => getMemory(memoryId!),
    enabled: memoryId !== null,
    refetchInterval: AUTO_REFRESH_MS,
    refetchIntervalInBackground: true,
  })
}
