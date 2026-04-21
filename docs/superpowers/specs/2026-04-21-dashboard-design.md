# Dashboard Design

## Goal

Build a standalone dashboard for `smem` that lets a user browse memories, search by keyword, filter by `kind`, inspect one memory in a right-side detail drawer, and archive memories, while keeping the UI more polished than a basic admin table.

## Non-Goals

- Editing memory content or metadata.
- Creating or deleting memories from the dashboard.
- Adding new server APIs unless the existing API contract is insufficient for the approved dashboard behavior.
- Building a generalized design system beyond what this dashboard needs.

## Scope

This design applies to a new frontend app under `dashboard/` only, with one possible exception: if the existing list API does not actually return newest-first results by default, the server may need a minimal sorting capability so the dashboard can match the approved default behavior.

### In Scope

- A Vite-based React dashboard app.
- Keyword search.
- `kind` filtering using the top 10 kinds.
- Infinite-scroll memory listing.
- Memory detail drawer.
- Archive action from the detail drawer.
- Focused error, loading, and empty states.
- Minimal automated tests, with manual verification as the primary acceptance path.

### Out Of Scope

- Memory editing flows.
- Multi-filter advanced search beyond keyword + kind.
- Bulk archive or bulk operations.
- Authentication, permissions, or multi-user concerns.

## Product Shape

The dashboard is a single-page memory console rather than a traditional CRUD table.

The page has two main areas:

1. A fixed search bar at the top with a keyword input and a `kind` select.
2. A scrolling list of memory cards below it, with a right-side drawer that opens when one card is selected.

The list is the primary navigation surface. Users should be able to skim recent memories quickly without opening detail for every item. Because of that, each memory card must show enough metadata to be useful on its own, including `kind`.

## UX Requirements

### Visual Direction

- Modern, polished, and slightly fancy.
- Not a plain enterprise table.
- Strong visual hierarchy for content, type, state, and timestamps.
- Comfortable on both desktop and mobile, with the drawer adapting to narrow screens.

### Search And Filter

- The search input sits at the top of the page for immediate access.
- The `kind` filter appears beside the search input.
- The `kind` list is populated from the server's top-kinds endpoint with `limit=10`.
- Search and filter changes reset the list back to the first page.

### List Behavior

- The memory list defaults to time-descending order.
- Pagination is implemented as load-more-on-scroll, not page-number navigation.
- Each card shows the minimum useful summary set:
  - memory content preview
  - `kind` or `kinds`
  - `type`
  - `state`
  - last updated time
- Cards should clearly indicate the selected item when the detail drawer is open.

### Detail Drawer

- Clicking a memory card opens a right-side drawer.
- The drawer fetches the full memory record on demand using the memory id.
- The drawer displays at least:
  - full content
  - `type`
  - `state`
  - `kinds`
  - updated time
  - usage or store count if the API exposes it
  - other available metadata that is already present in the server response

### Archive Flow

- The drawer contains an archive action.
- Archiving updates the memory state to `archived` through the existing update endpoint.
- The action must show a pending state while the request is in flight.
- On success, the UI shows confirmation feedback and refreshes any stale list, detail, and kind data.
- On failure, the drawer stays open and the error is shown to the user.

## API Usage

The dashboard should consume the existing server API surface under `server/api/openapi.yaml`.

### List Memories

`GET /api/v1/memories`

Used for the scrolling list with these query parameters:

- `page`
- `page_size`
- `search`
- `kind`

The dashboard should not depend on any undocumented query parameters.

### List Kinds

`GET /api/v1/memories/kinds?limit=10`

Used to populate the `kind` filter.

### Get Memory

`GET /api/v1/memories/{id}`

Used when opening the detail drawer so the list endpoint can stay lightweight.

### Archive Memory

`PUT /api/v1/memories/{id}` with:

```json
{
  "state": "archived"
}
```

Used only from the detail drawer in this iteration.

## Frontend Architecture

The dashboard should be implemented as a small, focused frontend app with a thin component tree and explicit API/query boundaries.

### Stack

- Vite
- React 19
- TypeScript
- Tailwind CSS 4
- shadcn/ui
- TanStack Query

### Recommended File Shape

- `dashboard/src/app/*` for app shell and providers.
- `dashboard/src/pages/DashboardPage.tsx` for top-level page composition.
- `dashboard/src/components/memory-search-bar.tsx` for keyword and `kind` controls.
- `dashboard/src/components/memory-list.tsx` for list rendering and infinite-scroll behavior.
- `dashboard/src/components/memory-card.tsx` for individual list items.
- `dashboard/src/components/memory-detail-drawer.tsx` for the selected-memory panel.
- `dashboard/src/api/memories.ts` for request helpers.
- `dashboard/src/hooks/*` for query and mutation wrappers.
- `dashboard/src/lib/*` for small UI utilities such as formatting helpers.

The exact filenames can change during implementation, but the separation of responsibilities should stay roughly this small and explicit.

## Data Flow

### Initial Load

1. Load top kinds for the filter.
2. Load the first memory page using the default query state.
3. Render the list and keep the drawer closed until a user selects a memory.

### Search Or Filter Change

1. Update local query state.
2. Reset list pagination.
3. Request page 1 for the new query.
4. Replace prior list results instead of appending.

### Load More

1. Detect the scroll threshold near the end of the list.
2. Request the next page using the current search/filter state.
3. Append the new page to the existing list.
4. Stop requesting once the API indicates there is no next page.

### Open Detail

1. Record the selected memory id.
2. Open the right-side drawer.
3. Fetch the full memory payload for that id.
4. Render loading, success, or error state inside the drawer.

### Archive

1. Send `PUT /api/v1/memories/{id}` with `state=archived`.
2. Disable the archive action while pending.
3. On success, invalidate the list query, detail query, and top-kinds query.
4. Re-render the selected memory using fresh data.

## State Management

TanStack Query should own server-state concerns:

- list query
- kinds query
- selected-memory detail query
- archive mutation

Local component state should stay limited to UI state such as:

- current search text
- selected `kind`
- selected memory id
- drawer open/closed state

This keeps the page easy to reason about and avoids embedding network behavior directly inside presentational components.

## Loading, Empty, And Error States

Each surface should handle its own state cleanly.

### List

- Initial loading skeleton.
- Search-empty state when no memories match the current query.
- Inline retry for fetch failures.
- Bottom loading indicator when fetching the next page.

### Kinds Filter

- Disabled or fallback UI if kinds fail to load.
- The dashboard still works without kind filtering if this endpoint fails.

### Detail Drawer

- Loading skeleton when fetching a selected memory.
- Error state inside the drawer if fetch fails.
- Drawer remains open so the user can retry or close it intentionally.

### Archive Action

- Pending button state while archiving.
- Success toast or equivalent lightweight confirmation.
- Inline error if archive fails.

## Responsive Behavior

- Desktop: right-side drawer behaves as a classic detail panel.
- Mobile: the same detail surface may slide from the bottom or occupy most of the screen, as long as it remains clearly a detail overlay and does not navigate away from the list.
- The search bar should remain easy to use on narrow screens, even if controls stack vertically.

## Testing Strategy

The user explicitly prefers minimal automated tests and will rely primarily on manual testing. The implementation should respect that preference.

### Automated Test Scope

Keep automated tests narrow and only cover the most failure-prone integration points:

- query parameter construction for list/search/filter requests
- archive mutation invalidation behavior
- one key interaction path proving that selecting a card opens the drawer for the correct id

Avoid broad snapshot coverage, exhaustive component tests, or deep visual test suites in this iteration.

### Manual Verification Scope

Manual testing is the primary acceptance path and should verify:

1. initial list load
2. keyword search
3. `kind` filtering
4. infinite scroll
5. opening detail drawer
6. archive success path
7. archive failure path
8. empty results state
9. mobile layout sanity

## Open Constraint

This design assumes the existing memory list endpoint already returns newest-first results by default. If implementation confirms that it does not, the minimal acceptable server change is to add an explicit sort order compatible with time-descending listing. No broader API redesign should be introduced for that.

## Implementation Notes

- Follow existing repo minimalism: small files, limited abstractions, only enough structure to keep queries and UI readable.
- Prefer a refined card-based interface over a data grid.
- Keep the app narrowly focused on memory browsing and archiving so it stays higher quality than a demo without becoming a large admin surface.
