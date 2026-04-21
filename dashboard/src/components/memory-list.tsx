import { useEffect, useRef } from 'react'

import type { MemorySummary } from '../lib/types'
import { MemoryCard } from './memory-card'

type MemoryListProps = {
  memories: MemorySummary[]
  selectedId: string | null
  hasNextPage: boolean
  isFetchingNextPage: boolean
  onLoadMore: () => void
  onSelect: (id: string) => void
}

export function MemoryList({
  memories,
  selectedId,
  hasNextPage,
  isFetchingNextPage,
  onLoadMore,
  onSelect,
}: MemoryListProps) {
  const sentinelRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!hasNextPage || !sentinelRef.current || typeof IntersectionObserver === 'undefined') {
      return
    }

    const observer = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting && !isFetchingNextPage) {
        onLoadMore()
      }
    })

    observer.observe(sentinelRef.current)
    return () => observer.disconnect()
  }, [hasNextPage, isFetchingNextPage, onLoadMore])

  return (
    <section className="space-y-4">
      {memories.map((memory) => (
        <MemoryCard
          key={memory.id}
          memory={memory}
          selected={selectedId === memory.id}
          onSelect={onSelect}
        />
      ))}
      {hasNextPage ? <div ref={sentinelRef} className="h-4" aria-hidden="true" /> : null}
      {isFetchingNextPage ? <p className="pb-6 text-sm text-slate-400">Loading more memories...</p> : null}
    </section>
  )
}
