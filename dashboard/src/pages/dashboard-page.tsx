import { useEffect, useMemo, useState } from 'react'

import { MemoryDetailDrawer } from '../components/memory-detail-drawer'
import { MemoryList } from '../components/memory-list'
import { MemorySearchBar } from '../components/memory-search-bar'
import { useMemoryKinds } from '../hooks/use-memory-kinds'
import { useMemoryList } from '../hooks/use-memory-list'

export function DashboardPage() {
  const initialState = useMemo(() => readDashboardParams(), [])
  const [search, setSearch] = useState(initialState.search)
  const [kind, setKind] = useState(initialState.kind)
  const [state, setState] = useState(initialState.state || 'active')
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const kindsQuery = useMemoryKinds()
  const listQuery = useMemoryList(search, kind, state)
  const memories = listQuery.data?.pages.flatMap((page) => page.items) ?? []
  const loadedPageCount = listQuery.data?.pages.length ?? 0

  useEffect(() => {
    setSelectedId(null)
  }, [search, kind, state])

  useEffect(() => {
    if (!listQuery.data) {
      return
    }

    if (loadedPageCount >= initialState.pages || !listQuery.hasNextPage || listQuery.isFetchingNextPage) {
      return
    }

    void listQuery.fetchNextPage()
  }, [initialState.pages, listQuery, loadedPageCount])

  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }

    const params = new URLSearchParams()
    if (search) {
      params.set('search', search)
    }
    if (kind) {
      params.set('kind', kind)
    }
    if (state) {
      params.set('state', state)
    }
    if (loadedPageCount > 1) {
      params.set('pages', String(loadedPageCount))
    }

    const query = params.toString()
    const nextUrl = query ? `${window.location.pathname}?${query}` : window.location.pathname
    window.history.replaceState(null, '', nextUrl)
  }, [kind, loadedPageCount, search, state])

  return (
    <main className="min-h-screen px-4 py-10 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-7xl">
        <section className="rounded-[28px] border border-white/10 bg-white/6 p-6 shadow-2xl shadow-sky-950/40 backdrop-blur-md">
          <p className="text-sm uppercase tracking-[0.24em] text-sky-300/80">SMEM</p>
          <h1 className="mt-3 text-4xl font-semibold tracking-tight text-white">Memory Console</h1>
          <p className="mt-3 max-w-2xl text-sm text-slate-300">
            Search, inspect, and archive your long-term memories from one dashboard.
          </p>
        </section>

        <div className="mt-6 space-y-6">
          <div className="sticky top-4 z-10">
            <MemorySearchBar
              search={search}
              kind={kind}
              state={state}
              kinds={(kindsQuery.data?.items ?? []).map((item) => item.kind)}
              kindsDisabled={kindsQuery.isError}
              onSearchChange={setSearch}
              onKindChange={setKind}
              onStateChange={setState}
            />
          </div>

          {listQuery.isPending ? <p className="text-sm text-slate-400">Loading memories...</p> : null}

          {listQuery.isError ? (
            <div className="rounded-[24px] border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-200">
              <p>Failed to load memories.</p>
              <button
                type="button"
                className="mt-3 rounded-full border border-red-300/30 px-3 py-1 text-xs font-medium uppercase tracking-[0.18em]"
                onClick={() => void listQuery.refetch()}
              >
                Retry
              </button>
            </div>
          ) : null}

          {!listQuery.isPending && !listQuery.isError && memories.length === 0 ? (
            <div className="rounded-[24px] border border-dashed border-white/12 bg-white/4 p-10 text-center text-slate-300">
              <p className="text-lg font-medium text-white">No memories found</p>
              <p className="mt-2 text-sm text-slate-400">Try a broader keyword or clear the current state filter.</p>
            </div>
          ) : null}

          {memories.length > 0 ? (
            <MemoryList
              memories={memories}
              selectedId={selectedId}
              hasNextPage={Boolean(listQuery.hasNextPage)}
              isFetchingNextPage={listQuery.isFetchingNextPage}
              onLoadMore={() => void listQuery.fetchNextPage()}
              onSelect={setSelectedId}
            />
          ) : null}
        </div>
      </div>

      <MemoryDetailDrawer memoryId={selectedId} open={selectedId !== null} onOpenChange={(open) => !open && setSelectedId(null)} />
    </main>
  )
}

function readDashboardParams() {
  if (typeof window === 'undefined') {
    return { search: '', kind: '', state: 'active', pages: 1 }
  }

  const params = new URLSearchParams(window.location.search)
  const pages = Number(params.get('pages') ?? '1')

  return {
    search: params.get('search') ?? '',
    kind: params.get('kind') ?? '',
    state: params.get('state') ?? 'active',
    pages: Number.isFinite(pages) && pages > 0 ? Math.floor(pages) : 1,
  }
}
