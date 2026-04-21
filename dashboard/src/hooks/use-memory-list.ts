import { useInfiniteQuery } from '@tanstack/react-query'

import { listMemories } from '../api/memories'

const AUTO_REFRESH_MS = 30_000

export function useMemoryList(search: string, kind: string, state: string) {
  return useInfiniteQuery({
    queryKey: ['memories', search, kind, state],
    initialPageParam: 1,
    queryFn: ({ pageParam }) =>
      listMemories({
        page: pageParam,
        pageSize: 10,
        search,
        kind,
        state,
      }),
    getNextPageParam: (lastPage) => (lastPage.has_more ? lastPage.page + 1 : undefined),
    refetchInterval: AUTO_REFRESH_MS,
    refetchIntervalInBackground: true,
  })
}
