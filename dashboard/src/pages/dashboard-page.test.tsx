import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { App } from '../app/app'

beforeEach(() => {
  window.history.replaceState(null, '', '/')
})

test('renders a memory card and opens the detail drawer when selected', async () => {
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)

    if (url.includes('/api/v1/memories/kinds?limit=10')) {
      return new Response(JSON.stringify({ items: [{ kind: 'preference', count: 1 }] }), {
        headers: { 'Content-Type': 'application/json' },
      })
    }

    if (url.includes('/api/v1/memories?page=1&page_size=10&state=active')) {
      return new Response(
        JSON.stringify({
          items: [
            {
              id: 'mem-1',
              content: 'Alice likes tea',
              type: 'fact',
              kind: 'preference',
              scope: 'user',
              state: 'active',
              updated_at: '2026-04-21T12:00:00Z',
            },
          ],
          page: 1,
          page_size: 10,
          total: 1,
          total_pages: 1,
          has_more: false,
        }),
        {
          headers: { 'Content-Type': 'application/json' },
        },
      )
    }

    if (url.includes('/api/v1/memories/mem-1') && !init?.method) {
      return new Response(
        JSON.stringify({
          id: 'mem-1',
          content: 'Alice likes tea',
          type: 'fact',
          kind: 'preference',
          scope: 'user',
          state: 'active',
          store_count: 2,
          updated_at: '2026-04-21T12:00:00Z',
          metadata: { source: 'test' },
        }),
        {
          headers: { 'Content-Type': 'application/json' },
        },
      )
    }

    throw new Error(`Unhandled fetch: ${url}`)
  }) as typeof fetch

  render(<App />)

  const card = await screen.findByRole('button', { name: /alice likes tea/i })

  expect(card).toBeVisible()
  expect(screen.getAllByText('preference')[0]).toBeVisible()

  await userEvent.click(card)

  expect(screen.getByRole('heading', { name: 'Memory Detail' })).toBeVisible()
})

test('archives the selected memory and refreshes the drawer state', async () => {
  const user = userEvent.setup()
  let state = 'active'

  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)

    if (url.includes('/api/v1/memories/kinds?limit=10')) {
      return new Response(JSON.stringify({ items: [{ kind: 'preference', count: 1 }] }), {
        headers: { 'Content-Type': 'application/json' },
      })
    }

    if (url.includes('/api/v1/memories?page=1&page_size=10&state=active')) {
      return new Response(
        JSON.stringify({
          items: [
            {
              id: 'mem-1',
              content: 'Alice likes tea',
              type: 'fact',
              kind: 'preference',
              scope: 'user',
              state,
              updated_at: '2026-04-21T12:00:00Z',
            },
          ],
          page: 1,
          page_size: 10,
          total: 1,
          total_pages: 1,
          has_more: false,
        }),
        {
          headers: { 'Content-Type': 'application/json' },
        },
      )
    }

    if (url.includes('/api/v1/memories/mem-1') && init?.method === 'PUT') {
      state = 'archived'
      return new Response(
        JSON.stringify({
          id: 'mem-1',
          content: 'Alice likes tea',
          type: 'fact',
          kind: 'preference',
          scope: 'user',
          state,
          updated_at: '2026-04-21T12:00:00Z',
        }),
        {
          headers: { 'Content-Type': 'application/json' },
        },
      )
    }

    if (url.includes('/api/v1/memories/mem-1')) {
      return new Response(
        JSON.stringify({
          id: 'mem-1',
          content: 'Alice likes tea',
          type: 'fact',
          kind: 'preference',
          scope: 'user',
          state,
          store_count: 2,
          updated_at: '2026-04-21T12:00:00Z',
        }),
        {
          headers: { 'Content-Type': 'application/json' },
        },
      )
    }

    throw new Error(`Unhandled fetch: ${url}`)
  }) as typeof fetch

  render(<App />)

  await user.click(await screen.findByRole('button', { name: /alice likes tea/i }))
  await user.click(await screen.findByRole('button', { name: /archive memory/i }))

  expect((await screen.findAllByText('archived')).length).toBeGreaterThan(0)
})

test('filters the list to archived memories and can mark one active again', async () => {
  const user = userEvent.setup()
  let state = 'archived'

  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)

    if (url.includes('/api/v1/memories/kinds?limit=10')) {
      return new Response(JSON.stringify({ items: [{ kind: 'preference', count: 1 }] }), {
        headers: { 'Content-Type': 'application/json' },
      })
    }

    if (url.includes('/api/v1/memories?page=1&page_size=10') && url.includes('state=archived')) {
      return new Response(
        JSON.stringify({
          items: [
            {
              id: 'mem-1',
              content: 'Alice likes tea',
              type: 'fact',
              kind: 'preference',
              scope: 'user',
              state,
              updated_at: '2026-04-21T12:00:00Z',
            },
          ],
          page: 1,
          page_size: 10,
          total: 1,
          total_pages: 1,
          has_more: false,
        }),
        { headers: { 'Content-Type': 'application/json' } },
      )
    }

    if (url.includes('/api/v1/memories?page=1&page_size=10')) {
      return new Response(
          JSON.stringify({ items: [], page: 1, page_size: 10, total: 0, total_pages: 0, has_more: false }),
        { headers: { 'Content-Type': 'application/json' } },
      )
    }

    if (url.includes('/api/v1/memories/mem-1') && init?.method === 'PUT') {
      state = 'active'
      return new Response(
        JSON.stringify({
          id: 'mem-1',
          content: 'Alice likes tea',
          type: 'fact',
          kind: 'preference',
          scope: 'user',
          state,
          updated_at: '2026-04-21T12:00:00Z',
        }),
        { headers: { 'Content-Type': 'application/json' } },
      )
    }

    if (url.includes('/api/v1/memories/mem-1')) {
      return new Response(
        JSON.stringify({
          id: 'mem-1',
          content: 'Alice likes tea',
          type: 'fact',
          kind: 'preference',
          scope: 'user',
          state,
          store_count: 2,
          updated_at: '2026-04-21T12:00:00Z',
        }),
        { headers: { 'Content-Type': 'application/json' } },
      )
    }

    throw new Error(`Unhandled fetch: ${url}`)
  }) as typeof fetch

  render(<App />)

  await user.click(screen.getByRole('button', { name: 'Archived' }))
  await user.click(await screen.findByRole('button', { name: /alice likes tea/i }))
  await user.click(await screen.findByRole('button', { name: /mark active/i }))

  expect(await screen.findByText('Memory marked active.')).toBeVisible()
})

test('shows an empty state when the current filters have no matches', async () => {
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)

    if (url.includes('/api/v1/memories/kinds?limit=10')) {
      return new Response(JSON.stringify({ items: [] }), {
        headers: { 'Content-Type': 'application/json' },
      })
    }

    if (url.includes('/api/v1/memories?page=1&page_size=10')) {
      return new Response(
        JSON.stringify({
          items: [],
          page: 1,
          page_size: 10,
          total: 0,
          total_pages: 0,
          has_more: false,
        }),
        {
          headers: { 'Content-Type': 'application/json' },
        },
      )
    }

    throw new Error(`Unhandled fetch: ${url}`)
  }) as typeof fetch

  render(<App />)

  expect(await screen.findByText('No memories found')).toBeVisible()
})

test('restores filters and loaded page count from the URL after refresh', async () => {
  const calls: string[] = []

  window.history.replaceState(null, '', '/?search=tea&state=archived&pages=2')

  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    calls.push(url)

    if (url.includes('/api/v1/memories/kinds?limit=10')) {
      return new Response(JSON.stringify({ items: [{ kind: 'preference', count: 2 }] }), {
        headers: { 'Content-Type': 'application/json' },
      })
    }

    if (url.includes('/api/v1/memories?page=1&page_size=10&search=tea&state=archived')) {
      return new Response(
        JSON.stringify({
          items: [
            {
              id: 'mem-1',
              content: 'Alice likes tea',
              type: 'fact',
              kind: 'preference',
              scope: 'user',
              state: 'archived',
              updated_at: '2026-04-21T12:00:00Z',
            },
          ],
          page: 1,
          page_size: 10,
          total: 2,
          total_pages: 2,
          has_more: true,
        }),
        { headers: { 'Content-Type': 'application/json' } },
      )
    }

    if (url.includes('/api/v1/memories?page=2&page_size=10&search=tea&state=archived')) {
      return new Response(
        JSON.stringify({
          items: [
            {
              id: 'mem-2',
              content: 'Tea archive note',
              type: 'fact',
              kind: 'preference',
              scope: 'user',
              state: 'archived',
              updated_at: '2026-04-20T12:00:00Z',
            },
          ],
          page: 2,
          page_size: 10,
          total: 2,
          total_pages: 2,
          has_more: false,
        }),
        { headers: { 'Content-Type': 'application/json' } },
      )
    }

    throw new Error(`Unhandled fetch: ${url}`)
  }) as typeof fetch

  render(<App />)

  expect(await screen.findByRole('button', { name: /alice likes tea/i })).toBeVisible()
  expect(await screen.findByRole('button', { name: /tea archive note/i })).toBeVisible()
  expect(calls.some((url) => url.includes('page=2&page_size=10&search=tea&state=archived'))).toBe(true)
  expect(window.location.search).toContain('search=tea')
  expect(window.location.search).toContain('state=archived')
  expect(window.location.search).toContain('pages=2')
})
