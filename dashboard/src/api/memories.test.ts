import { getMemory, listMemories, listMemoryKinds, updateMemoryState } from './memories'

test('listMemories builds page search and kind query params', async () => {
  const calls: string[] = []

  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    calls.push(String(input))
    return new Response(
      JSON.stringify({
        items: [],
        page: 2,
        page_size: 20,
        total: 0,
        total_pages: 0,
        has_more: false,
      }),
      {
        headers: { 'Content-Type': 'application/json' },
      },
    )
  }) as typeof fetch

  await listMemories({ page: 2, pageSize: 20, search: 'alice', kind: 'profile', state: 'archived' })

  expect(calls[0]).toContain('/api/v1/memories?page=2&page_size=20&search=alice&kind=profile&state=archived')
})

test('listMemoryKinds requests the top 10 kinds', async () => {
  const calls: string[] = []

  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    calls.push(String(input))
    return new Response(JSON.stringify({ items: [{ kind: 'profile', count: 3 }] }), {
      headers: { 'Content-Type': 'application/json' },
    })
  }) as typeof fetch

  await listMemoryKinds()

  expect(calls[0]).toContain('/api/v1/memories/kinds?limit=10')
})

test('getMemory requests the selected memory detail', async () => {
  const calls: string[] = []

  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    calls.push(String(input))
    return new Response(JSON.stringify({ id: 'mem-1', content: 'hello', state: 'active', scope: 'user' }), {
      headers: { 'Content-Type': 'application/json' },
    })
  }) as typeof fetch

  await getMemory('mem-1')

  expect(calls[0]).toContain('/api/v1/memories/mem-1')
})

test('updateMemoryState sends the requested memory state', async () => {
  const calls: Array<{ url: string; init?: RequestInit }> = []

  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init })
    return new Response(JSON.stringify({ id: 'mem-1', state: 'archived', scope: 'user', content: 'hello' }), {
      headers: { 'Content-Type': 'application/json' },
    })
  }) as typeof fetch

  await updateMemoryState('mem-1', 'active')

  expect(calls[0].url).toContain('/api/v1/memories/mem-1')
  expect(calls[0].init?.method).toBe('PUT')
  expect(calls[0].init?.body).toBe(JSON.stringify({ state: 'active' }))
})
