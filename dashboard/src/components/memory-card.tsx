import { formatTimestamp, getMemoryKinds } from '../lib/format'
import type { MemorySummary } from '../lib/types'

type MemoryCardProps = {
  memory: MemorySummary
  selected: boolean
  onSelect: (id: string) => void
}

export function MemoryCard({ memory, selected, onSelect }: MemoryCardProps) {
  const kinds = getMemoryKinds(memory)

  return (
    <button
      type="button"
      className={[
        'group w-full rounded-[24px] border p-5 text-left transition',
        selected
          ? 'border-sky-400/70 bg-sky-400/10 shadow-lg shadow-sky-950/30'
          : 'border-white/8 bg-white/5 hover:border-white/18 hover:bg-white/8',
      ].join(' ')}
      aria-pressed={selected}
      onClick={() => onSelect(memory.id)}
    >
      <div className="flex flex-wrap items-center gap-2 text-xs font-medium uppercase tracking-[0.22em] text-slate-400">
        <span>{memory.type ?? 'unknown type'}</span>
        <span className="rounded-full border border-white/10 px-2 py-1 text-[11px] tracking-[0.18em] text-slate-300">
          {memory.state}
        </span>
      </div>
      <div className="mt-4 text-base font-medium leading-7 text-white">{memory.content}</div>
      <div className="mt-4 flex flex-wrap gap-2">
        {kinds.length > 0 ? (
          kinds.map((kind) => (
            <span
              key={kind}
              className="rounded-full border border-emerald-400/20 bg-emerald-400/10 px-3 py-1 text-xs font-medium text-emerald-200"
            >
              {kind}
            </span>
          ))
        ) : (
          <span className="rounded-full border border-white/10 px-3 py-1 text-xs text-slate-400">
            no kind
          </span>
        )}
      </div>
      <div className="mt-4 text-sm text-slate-400">Updated {formatTimestamp(memory.updated_at)}</div>
    </button>
  )
}
