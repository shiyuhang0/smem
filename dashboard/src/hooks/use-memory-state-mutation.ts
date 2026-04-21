import { useMutation, useQueryClient } from '@tanstack/react-query'

import { updateMemoryState } from '../api/memories'

export function useMemoryStateMutation(memoryId: string | null) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (state: string) => {
      if (!memoryId) {
        throw new Error('memory id is required')
      }

      return updateMemoryState(memoryId, state)
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['memories'] }),
        queryClient.invalidateQueries({ queryKey: ['memory-kinds'] }),
        queryClient.invalidateQueries({ queryKey: ['memory-detail', memoryId] }),
      ])
    },
  })
}
