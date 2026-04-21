import { Select } from './ui/select'

const ALL_KINDS_VALUE = '__all_kinds__'

type MemorySearchBarProps = {
  search: string
  kind: string
  state: string
  kinds: string[]
  kindsDisabled?: boolean
  onSearchChange: (value: string) => void
  onKindChange: (value: string) => void
  onStateChange: (value: string) => void
}

export function MemorySearchBar({
  search,
  kind,
  state,
  kinds,
  kindsDisabled = false,
  onSearchChange,
  onKindChange,
  onStateChange,
}: MemorySearchBarProps) {
  return (
    <section className="rounded-[28px] border border-white/10 bg-white/6 p-4 shadow-xl shadow-sky-950/20 backdrop-blur-md">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end">
        <label className="flex-1">
          <span className="mb-2 block text-xs font-medium uppercase tracking-[0.24em] text-slate-400">
            Search
          </span>
          <input
            className="w-full rounded-2xl border border-white/10 bg-slate-950/70 px-4 py-3 text-sm text-white outline-none transition focus:border-sky-400/60"
            value={search}
            placeholder="Search memories"
            onChange={(event) => onSearchChange(event.target.value)}
          />
        </label>
        <label className="lg:w-56">
          <span className="mb-2 block text-xs font-medium uppercase tracking-[0.24em] text-slate-400">
            Kind
          </span>
          <Select
            ariaLabel="Kind"
            value={kind || ALL_KINDS_VALUE}
            placeholder="All kinds"
            disabled={kindsDisabled}
            onValueChange={(value) => onKindChange(value === ALL_KINDS_VALUE ? '' : value)}
            options={[
              { label: 'All kinds', value: ALL_KINDS_VALUE },
              ...kinds.map((item) => ({ label: item, value: item })),
            ]}
          />
        </label>
        <fieldset className="flex flex-col">
          <span className="mb-2 block text-xs font-medium uppercase tracking-[0.24em] text-slate-400">
            State
          </span>
          <div className="flex overflow-hidden rounded-2xl border border-white/10">
            <button
              type="button"
              className={[
                'px-5 py-3 text-sm font-medium uppercase tracking-[0.12em] transition',
                state === 'active'
                  ? 'bg-sky-400/20 text-sky-100'
                  : 'bg-slate-950/70 text-slate-400 hover:text-white',
              ].join(' ')}
              onClick={() => onStateChange('active')}
            >
              Active
            </button>
            <button
              type="button"
              className={[
                'px-5 py-3 text-sm font-medium uppercase tracking-[0.12em] transition',
                state === 'archived'
                  ? 'bg-sky-400/20 text-sky-100'
                  : 'bg-slate-950/70 text-slate-400 hover:text-white',
              ].join(' ')}
              onClick={() => onStateChange('archived')}
            >
              Archived
            </button>
          </div>
        </fieldset>
      </div>
    </section>
  )
}
