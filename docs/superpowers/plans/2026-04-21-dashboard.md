# Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone dashboard app under `dashboard/` that lists memories with infinite scroll, supports keyword search and top-kind filtering, opens a right-side detail drawer, and archives memories through the existing server API.

**Architecture:** Create a new Vite + React 19 + TypeScript frontend app in `dashboard/`. Keep server state in TanStack Query, UI state in the page layer, and split the UI into a small set of focused components: search bar, list, card, and detail drawer. Use the existing server API directly, adding no server changes unless implementation proves the list endpoint cannot provide newest-first results.

**Tech Stack:** Vite, React 19, TypeScript, Tailwind CSS 4, shadcn/ui, TanStack Query, Vitest, Testing Library.

---

## File Map

- Create: `dashboard/package.json`
- Create: `dashboard/tsconfig.json`
- Create: `dashboard/tsconfig.node.json`
- Create: `dashboard/vite.config.ts`
- Create: `dashboard/index.html`
- Create: `dashboard/src/main.tsx`
- Create: `dashboard/src/app/app.tsx`
- Create: `dashboard/src/app/providers.tsx`
- Create: `dashboard/src/app/styles.css`
- Create: `dashboard/src/pages/dashboard-page.tsx`
- Create: `dashboard/src/components/memory-search-bar.tsx`
- Create: `dashboard/src/components/memory-list.tsx`
- Create: `dashboard/src/components/memory-card.tsx`
- Create: `dashboard/src/components/memory-detail-drawer.tsx`
- Create: `dashboard/src/components/ui/*` via shadcn where needed
- Create: `dashboard/src/api/client.ts`
- Create: `dashboard/src/api/memories.ts`
- Create: `dashboard/src/hooks/use-memory-list.ts`
- Create: `dashboard/src/hooks/use-memory-kinds.ts`
- Create: `dashboard/src/hooks/use-memory-detail.ts`
- Create: `dashboard/src/hooks/use-archive-memory.ts`
- Create: `dashboard/src/lib/format.ts`
- Create: `dashboard/src/lib/types.ts`
- Create: `dashboard/src/test/setup.ts`
- Create: `dashboard/src/api/memories.test.ts`
- Create: `dashboard/src/pages/dashboard-page.test.tsx`
- Modify: `README.md`

### Task 1: Scaffold The Dashboard App

**Files:**
- Create: `dashboard/package.json`
- Create: `dashboard/tsconfig.json`
- Create: `dashboard/tsconfig.node.json`
- Create: `dashboard/vite.config.ts`
- Create: `dashboard/index.html`
- Create: `dashboard/src/main.tsx`
- Create: `dashboard/src/app/app.tsx`
- Create: `dashboard/src/app/providers.tsx`
- Create: `dashboard/src/app/styles.css`

- [ ] **Step 1: Write the failing smoke test for app boot**

```tsx
import { render, screen } from '@testing-library/react'
import { App } from './app/app'

test('renders dashboard shell title', () => {
  render(<App />)
  expect(screen.getByText('Memory Console')).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npm test -- --run src/app/app.test.tsx`
Expected: FAIL because `dashboard/` app files and test setup do not exist yet.

- [ ] **Step 3: Initialize the Vite app and dependencies**

```json
{
  "name": "smem-dashboard",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "lint": "tsc --noEmit"
  },
  "dependencies": {
    "@radix-ui/react-dialog": "^1.1.1",
    "@radix-ui/react-select": "^2.1.1",
    "@tanstack/react-query": "^5.59.0",
    "@tanstack/react-query-devtools": "^5.59.0",
    "clsx": "^2.1.1",
    "lucide-react": "^0.469.0",
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "tailwind-merge": "^2.5.4"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.6.3",
    "@testing-library/react": "^16.1.0",
    "@testing-library/user-event": "^14.5.2",
    "@types/react": "^19.0.1",
    "@types/react-dom": "^19.0.1",
    "@vitejs/plugin-react": "^4.3.4",
    "jsdom": "^25.0.1",
    "tailwindcss": "^4.0.0",
    "typescript": "^5.6.3",
    "vite": "^6.0.1",
    "vitest": "^2.1.4"
  }
}
```

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './app/app'
import './app/styles.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

```tsx
import { Providers } from './providers'
import { DashboardPage } from '../pages/dashboard-page'

export function App() {
  return (
    <Providers>
      <DashboardPage />
    </Providers>
  )
}
```

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactNode } from 'react'

const queryClient = new QueryClient()

export function Providers({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}
```

```tsx
export function DashboardPage() {
  return <main>Memory Console</main>
}
```

- [ ] **Step 4: Add Vitest setup and the smoke test file**

```ts
import '@testing-library/jest-dom'
```

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
})
```

```tsx
import { render, screen } from '@testing-library/react'
import { App } from './app'

test('renders dashboard shell title', () => {
  render(<App />)
  expect(screen.getByText('Memory Console')).toBeInTheDocument()
})
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `npm test -- --run src/app/app.test.tsx`
Expected: PASS

- [ ] **Step 6: Install dependencies and verify the scaffold builds**

Run: `npm install && npm run build`
Expected: install succeeds and Vite build completes.

- [ ] **Step 7: Commit the scaffold**

```bash
git add dashboard README.md docs/superpowers/plans/2026-04-21-dashboard.md
git commit -m "feat: scaffold dashboard app"
```

### Task 2: Add Typed API Helpers And Minimal Request Tests

**Files:**
- Create: `dashboard/src/api/client.ts`
- Create: `dashboard/src/api/memories.ts`
- Create: `dashboard/src/lib/types.ts`
- Create: `dashboard/src/api/memories.test.ts`

- [ ] **Step 1: Write the failing API request-shape tests**

```ts
import { archiveMemory, listMemories, listMemoryKinds } from './memories'

test('listMemories builds page search and kind query params', async () => {
  const calls: string[] = []
  global.fetch = vi.fn(async (input: RequestInfo | URL) => {
    calls.push(String(input))
    return new Response(JSON.stringify({ items: [], next_page: null }), {
      headers: { 'Content-Type': 'application/json' },
    })
  }) as typeof fetch

  await listMemories({ page: 2, pageSize: 20, search: 'alice', kind: 'profile' })

  expect(calls[0]).toContain('/api/v1/memories?page=2&page_size=20&search=alice&kind=profile')
})

test('listMemoryKinds requests the top 10 kinds', async () => {
  const calls: string[] = []
  global.fetch = vi.fn(async (input: RequestInfo | URL) => {
    calls.push(String(input))
    return new Response(JSON.stringify({ items: [] }), {
      headers: { 'Content-Type': 'application/json' },
    })
  }) as typeof fetch

  await listMemoryKinds()

  expect(calls[0]).toContain('/api/v1/memories/kinds?limit=10')
})

test('archiveMemory sends archived state', async () => {
  const calls: Array<{ url: string; init?: RequestInit }> = []
  global.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init })
    return new Response(JSON.stringify({ id: 'mem-1', state: 'archived' }), {
      headers: { 'Content-Type': 'application/json' },
    })
  }) as typeof fetch

  await archiveMemory('mem-1')

  expect(calls[0].url).toContain('/api/v1/memories/mem-1')
  expect(calls[0].init?.method).toBe('PUT')
  expect(calls[0].init?.body).toBe(JSON.stringify({ state: 'archived' }))
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npm test -- --run src/api/memories.test.ts`
Expected: FAIL because the API helpers and types do not exist yet.

- [ ] **Step 3: Add typed API primitives and memory request helpers**

```ts
export type MemorySummary = {
  id: string
  content: string
  type?: string
  state?: string
  kinds?: string[]
  updated_at?: string
  store_count?: number
}

export type MemoryDetail = MemorySummary & {
  metadata?: Record<string, unknown>
}

export type MemoryListResponse = {
  items: MemorySummary[]
  next_page: number | null
}

export type MemoryKindsResponse = {
  items: string[]
}
```

```ts
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  })

  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}
```

```ts
import { apiFetch } from './client'
import type { MemoryDetail, MemoryKindsResponse, MemoryListResponse } from '../lib/types'

export function listMemories(input: {
  page: number
  pageSize: number
  search: string
  kind: string
}) {
  const params = new URLSearchParams()
  params.set('page', String(input.page))
  params.set('page_size', String(input.pageSize))
  if (input.search) params.set('search', input.search)
  if (input.kind) params.set('kind', input.kind)
  return apiFetch<MemoryListResponse>(`/api/v1/memories?${params.toString()}`)
}

export function listMemoryKinds() {
  return apiFetch<MemoryKindsResponse>('/api/v1/memories/kinds?limit=10')
}

export function getMemory(id: string) {
  return apiFetch<MemoryDetail>(`/api/v1/memories/${id}`)
}

export function archiveMemory(id: string) {
  return apiFetch<MemoryDetail>(`/api/v1/memories/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ state: 'archived' }),
  })
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npm test -- --run src/api/memories.test.ts`
Expected: PASS

- [ ] **Step 5: Commit the API layer**

```bash
git add dashboard/src/api dashboard/src/lib/types.ts dashboard/src/api/memories.test.ts
git commit -m "feat: add dashboard memory api layer"
```

### Task 3: Build The Search Bar, Memory List, And Card Layout

**Files:**
- Create: `dashboard/src/pages/dashboard-page.tsx`
- Create: `dashboard/src/components/memory-search-bar.tsx`
- Create: `dashboard/src/components/memory-list.tsx`
- Create: `dashboard/src/components/memory-card.tsx`
- Create: `dashboard/src/hooks/use-memory-list.ts`
- Create: `dashboard/src/hooks/use-memory-kinds.ts`
- Create: `dashboard/src/lib/format.ts`

- [ ] **Step 1: Write the failing list interaction test**

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { App } from '../app/app'

test('shows a memory card with kind metadata and opens selection state', async () => {
  const user = userEvent.setup()

  render(<App />)

  const card = await screen.findByRole('button', { name: /Alice likes tea/i })
  expect(screen.getByText('profile')).toBeInTheDocument()

  await user.click(card)

  expect(await screen.findByText('Memory Detail')).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npm test -- --run src/pages/dashboard-page.test.tsx`
Expected: FAIL because the page still renders only the shell title.

- [ ] **Step 3: Add the list and kind hooks**

```ts
import { useInfiniteQuery } from '@tanstack/react-query'
import { listMemories } from '../api/memories'

export function useMemoryList(search: string, kind: string) {
  return useInfiniteQuery({
    queryKey: ['memories', search, kind],
    initialPageParam: 1,
    queryFn: ({ pageParam }) =>
      listMemories({ page: pageParam, pageSize: 20, search, kind }),
    getNextPageParam: (lastPage) => lastPage.next_page,
  })
}
```

```ts
import { useQuery } from '@tanstack/react-query'
import { listMemoryKinds } from '../api/memories'

export function useMemoryKinds() {
  return useQuery({
    queryKey: ['memory-kinds'],
    queryFn: listMemoryKinds,
  })
}
```

- [ ] **Step 4: Implement the search bar, card, and list components**

```tsx
export function MemorySearchBar(props: {
  search: string
  kind: string
  kinds: string[]
  onSearchChange: (value: string) => void
  onKindChange: (value: string) => void
}) {
  return (
    <section>
      <input
        value={props.search}
        placeholder="Search memories"
        onChange={(event) => props.onSearchChange(event.target.value)}
      />
      <select value={props.kind} onChange={(event) => props.onKindChange(event.target.value)}>
        <option value="">All kinds</option>
        {props.kinds.map((kind) => (
          <option key={kind} value={kind}>
            {kind}
          </option>
        ))}
      </select>
    </section>
  )
}
```

```tsx
import type { MemorySummary } from '../lib/types'

export function MemoryCard(props: {
  memory: MemorySummary
  selected: boolean
  onSelect: (id: string) => void
}) {
  return (
    <button onClick={() => props.onSelect(props.memory.id)} aria-pressed={props.selected}>
      <div>{props.memory.content}</div>
      <div>{props.memory.kinds?.join(', ')}</div>
      <div>{props.memory.type}</div>
      <div>{props.memory.state}</div>
    </button>
  )
}
```

```tsx
import type { MemorySummary } from '../lib/types'
import { MemoryCard } from './memory-card'

export function MemoryList(props: {
  memories: MemorySummary[]
  selectedId: string | null
  hasNextPage: boolean
  isFetchingNextPage: boolean
  onLoadMore: () => void
  onSelect: (id: string) => void
}) {
  return (
    <section>
      {props.memories.map((memory) => (
        <MemoryCard
          key={memory.id}
          memory={memory}
          selected={props.selectedId === memory.id}
          onSelect={props.onSelect}
        />
      ))}
      {props.hasNextPage ? (
        <button onClick={props.onLoadMore} disabled={props.isFetchingNextPage}>
          {props.isFetchingNextPage ? 'Loading…' : 'Load more'}
        </button>
      ) : null}
    </section>
  )
}
```

- [ ] **Step 5: Compose the dashboard page around those hooks and components**

```tsx
import { useMemo, useState } from 'react'
import { MemoryList } from '../components/memory-list'
import { MemorySearchBar } from '../components/memory-search-bar'
import { MemoryDetailDrawer } from '../components/memory-detail-drawer'
import { useMemoryKinds } from '../hooks/use-memory-kinds'
import { useMemoryList } from '../hooks/use-memory-list'

export function DashboardPage() {
  const [search, setSearch] = useState('')
  const [kind, setKind] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const kindsQuery = useMemoryKinds()
  const listQuery = useMemoryList(search, kind)

  const memories = useMemo(
    () => listQuery.data?.pages.flatMap((page) => page.items) ?? [],
    [listQuery.data],
  )

  return (
    <main>
      <h1>Memory Console</h1>
      <MemorySearchBar
        search={search}
        kind={kind}
        kinds={kindsQuery.data?.items ?? []}
        onSearchChange={setSearch}
        onKindChange={setKind}
      />
      <MemoryList
        memories={memories}
        selectedId={selectedId}
        hasNextPage={Boolean(listQuery.hasNextPage)}
        isFetchingNextPage={listQuery.isFetchingNextPage}
        onLoadMore={() => void listQuery.fetchNextPage()}
        onSelect={setSelectedId}
      />
      <MemoryDetailDrawer memoryId={selectedId} open={selectedId !== null} onOpenChange={() => setSelectedId(null)} />
    </main>
  )
}
```

- [ ] **Step 6: Run the interaction test to verify it passes**

Run: `npm test -- --run src/pages/dashboard-page.test.tsx`
Expected: PASS

- [ ] **Step 7: Commit the list experience**

```bash
git add dashboard/src/pages dashboard/src/components dashboard/src/hooks dashboard/src/lib
git commit -m "feat: add dashboard memory list ui"
```

### Task 4: Implement The Detail Drawer And Archive Mutation

**Files:**
- Create: `dashboard/src/components/memory-detail-drawer.tsx`
- Create: `dashboard/src/hooks/use-memory-detail.ts`
- Create: `dashboard/src/hooks/use-archive-memory.ts`
- Modify: `dashboard/src/pages/dashboard-page.tsx`

- [ ] **Step 1: Write the failing archive invalidation test**

```tsx
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { App } from '../app/app'

test('archives the selected memory and refreshes the drawer state', async () => {
  const user = userEvent.setup()

  render(<App />)

  await user.click(await screen.findByRole('button', { name: /Alice likes tea/i }))
  await user.click(await screen.findByRole('button', { name: /Archive memory/i }))

  await waitFor(() => {
    expect(screen.getByText('archived')).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npm test -- --run src/pages/dashboard-page.test.tsx`
Expected: FAIL because the drawer and archive mutation do not exist yet.

- [ ] **Step 3: Add the detail and archive hooks**

```ts
import { useQuery } from '@tanstack/react-query'
import { getMemory } from '../api/memories'

export function useMemoryDetail(memoryId: string | null) {
  return useQuery({
    queryKey: ['memory-detail', memoryId],
    queryFn: () => getMemory(memoryId!),
    enabled: Boolean(memoryId),
  })
}
```

```ts
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { archiveMemory } from '../api/memories'

export function useArchiveMemory(memoryId: string | null) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () => archiveMemory(memoryId!),
    onSuccess: async (_, id = memoryId) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['memories'] }),
        queryClient.invalidateQueries({ queryKey: ['memory-kinds'] }),
        queryClient.invalidateQueries({ queryKey: ['memory-detail', id] }),
      ])
    },
  })
}
```

- [ ] **Step 4: Implement the drawer UI and archive action**

```tsx
import { useArchiveMemory } from '../hooks/use-archive-memory'
import { useMemoryDetail } from '../hooks/use-memory-detail'

export function MemoryDetailDrawer(props: {
  memoryId: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const detailQuery = useMemoryDetail(props.memoryId)
  const archiveMutation = useArchiveMemory(props.memoryId)

  if (!props.open) {
    return null
  }

  return (
    <aside>
      <button onClick={() => props.onOpenChange(false)}>Close</button>
      <h2>Memory Detail</h2>
      {detailQuery.isLoading ? <p>Loading detail…</p> : null}
      {detailQuery.isError ? <p>Failed to load memory.</p> : null}
      {detailQuery.data ? (
        <div>
          <p>{detailQuery.data.content}</p>
          <p>{detailQuery.data.type}</p>
          <p>{detailQuery.data.state}</p>
          <p>{detailQuery.data.kinds?.join(', ')}</p>
          <button disabled={archiveMutation.isPending} onClick={() => archiveMutation.mutate()}>
            {archiveMutation.isPending ? 'Archiving…' : 'Archive memory'}
          </button>
          {archiveMutation.isError ? <p>Archive failed.</p> : null}
        </div>
      ) : null}
    </aside>
  )
}
```

- [ ] **Step 5: Run the archive test to verify it passes**

Run: `npm test -- --run src/pages/dashboard-page.test.tsx`
Expected: PASS

- [ ] **Step 6: Commit the detail flow**

```bash
git add dashboard/src/components/memory-detail-drawer.tsx dashboard/src/hooks/use-memory-detail.ts dashboard/src/hooks/use-archive-memory.ts dashboard/src/pages/dashboard-page.tsx dashboard/src/pages/dashboard-page.test.tsx
git commit -m "feat: add dashboard memory detail drawer"
```

### Task 5: Polish UI States, Infinite Scroll Trigger, And Manual Verification Notes

**Files:**
- Modify: `dashboard/src/pages/dashboard-page.tsx`
- Modify: `dashboard/src/components/memory-search-bar.tsx`
- Modify: `dashboard/src/components/memory-list.tsx`
- Modify: `dashboard/src/components/memory-card.tsx`
- Modify: `dashboard/src/components/memory-detail-drawer.tsx`
- Modify: `dashboard/src/app/styles.css`
- Modify: `README.md`

- [ ] **Step 1: Write the failing empty-state test**

```tsx
import { render, screen } from '@testing-library/react'
import { App } from '../app/app'

test('shows an empty state when the current filters have no matches', async () => {
  render(<App />)

  expect(await screen.findByText('No memories found')).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npm test -- --run src/pages/dashboard-page.test.tsx`
Expected: FAIL because the page does not yet render a dedicated empty state.

- [ ] **Step 3: Add the final UI states and visual polish**

```tsx
{listQuery.isLoading ? <p>Loading memories…</p> : null}
{!listQuery.isLoading && memories.length === 0 ? <p>No memories found</p> : null}
{listQuery.isError ? <button onClick={() => void listQuery.refetch()}>Retry</button> : null}
```

```tsx
<main className="min-h-screen bg-neutral-950 text-neutral-50">
  <section className="mx-auto max-w-7xl px-4 py-8">
    <header className="mb-6 rounded-3xl border border-white/10 bg-white/5 p-4 shadow-2xl shadow-cyan-950/30 backdrop-blur">
      <h1 className="text-3xl font-semibold tracking-tight">Memory Console</h1>
      <p className="mt-2 text-sm text-neutral-300">Search, inspect, and archive long-term memories.</p>
    </header>
  </section>
</main>
```

```css
:root {
  color-scheme: dark;
  font-family: Inter, ui-sans-serif, system-ui, sans-serif;
  background: #09090b;
}

body {
  margin: 0;
  min-width: 320px;
  background:
    radial-gradient(circle at top, rgba(34, 211, 238, 0.12), transparent 30%),
    linear-gradient(180deg, #0a0a0f 0%, #09090b 100%);
}
```

- [ ] **Step 4: Replace the fallback load-more button with an intersection-observer trigger**

```tsx
const sentinelRef = useRef<HTMLDivElement | null>(null)

useEffect(() => {
  if (!props.hasNextPage || !sentinelRef.current) return

  const observer = new IntersectionObserver((entries) => {
    if (entries[0]?.isIntersecting && !props.isFetchingNextPage) {
      props.onLoadMore()
    }
  })

  observer.observe(sentinelRef.current)
  return () => observer.disconnect()
}, [props.hasNextPage, props.isFetchingNextPage, props.onLoadMore])
```

- [ ] **Step 5: Document manual verification in the README**

```md
## Dashboard

Run the dashboard locally:

```bash
cd dashboard
npm install
npm run dev
```

Manual verification checklist:

1. Open the dashboard and confirm the first page of memories loads.
2. Search by keyword and confirm results update.
3. Filter by kind and confirm cards update.
4. Scroll to the bottom and confirm the next page loads.
5. Open one memory and confirm the detail drawer shows full content.
6. Archive one memory and confirm the state refreshes to `archived`.
7. Temporarily stop the server and confirm request failures show retry/error states.
```

- [ ] **Step 6: Run the full frontend verification suite**

Run: `npm test && npm run build && npm run lint`
Expected: all tests pass, TypeScript check passes, and production build succeeds.

- [ ] **Step 7: Perform manual verification against the running server**

Run: `npm run dev`
Expected: dashboard opens locally and the manual checklist above can be exercised against `http://localhost:8080` or `VITE_API_BASE_URL`.

- [ ] **Step 8: Commit the polished dashboard**

```bash
git add dashboard README.md
git commit -m "feat: finish dashboard memory console"
```

## Self-Review

- Spec coverage: this plan covers search, top-kind filter, infinite scroll, card list with visible `kind`, detail drawer, archive action, responsive-friendly overlay behavior, minimal automated tests, and manual verification notes.
- Placeholder scan: no `TODO`, `TBD`, or deferred implementation markers remain.
- Type consistency: the plan uses one stable naming set across tasks: `MemorySummary`, `MemoryDetail`, `MemoryListResponse`, `MemoryDetailDrawer`, `useMemoryList`, `useMemoryKinds`, `useMemoryDetail`, and `useArchiveMemory`.
