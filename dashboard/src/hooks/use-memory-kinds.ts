import { useQuery } from '@tanstack/react-query'

import { listMemoryKinds } from '../api/memories'

const AUTO_REFRESH_MS = 60_000

export function useMemoryKinds() {
  return useQuery({
    queryKey: ['memory-kinds'],
    queryFn: listMemoryKinds,
    refetchInterval: AUTO_REFRESH_MS,
    refetchIntervalInBackground: true,
  })
}
