import * as Dialog from '@radix-ui/react-dialog'

import { formatTimestamp, getMemoryKinds } from '../lib/format'
import { useMemoryDetail } from '../hooks/use-memory-detail'
import { useMemoryStateMutation } from '../hooks/use-memory-state-mutation'

type MemoryDetailDrawerProps = {
  memoryId: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function MemoryDetailDrawer({ memoryId, open, onOpenChange }: MemoryDetailDrawerProps) {
  const detailQuery = useMemoryDetail(open ? memoryId : null)
  const stateMutation = useMemoryStateMutation(memoryId)
  const kinds = detailQuery.data ? getMemoryKinds(detailQuery.data) : []
  const isArchived = detailQuery.data?.state === 'archived'
  const nextState = isArchived ? 'active' : 'archived'
  const actionLabel = isArchived ? 'Mark active' : 'Archive memory'
  const pendingLabel = isArchived ? 'Marking active...' : 'Archiving...'
  const successLabel = stateMutation.variables === 'active' ? 'Memory marked active.' : 'Memory archived.'

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-slate-950/70 backdrop-blur-sm" />
        <Dialog.Content className="fixed inset-x-0 bottom-0 z-50 max-h-[88vh] rounded-t-[28px] border border-white/10 bg-slate-950/96 p-6 text-slate-100 shadow-2xl shadow-black/40 outline-none sm:inset-y-0 sm:right-0 sm:left-auto sm:h-full sm:w-[30rem] sm:max-w-[90vw] sm:rounded-none sm:rounded-l-[32px]">
          <div className="flex items-start justify-between gap-4">
            <div>
              <Dialog.Title className="text-2xl font-semibold tracking-tight">Memory Detail</Dialog.Title>
              <Dialog.Description className="mt-2 text-sm text-slate-400">
                Inspect the full memory payload and archive it when it is no longer active.
              </Dialog.Description>
            </div>
            <Dialog.Close className="rounded-full border border-white/10 px-3 py-1 text-sm text-slate-300 transition hover:border-white/20 hover:text-white">
              Close
            </Dialog.Close>
          </div>

          {detailQuery.isPending ? <p className="mt-8 text-sm text-slate-400">Loading detail...</p> : null}

          {detailQuery.isError ? (
            <div className="mt-8 rounded-2xl border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-200">
              <p>Failed to load memory.</p>
              <button
                type="button"
                className="mt-3 rounded-full border border-red-300/30 px-3 py-1 text-xs font-medium uppercase tracking-[0.18em]"
                onClick={() => void detailQuery.refetch()}
              >
                Retry
              </button>
            </div>
          ) : null}

          {detailQuery.data ? (
            <div className="mt-8 space-y-6 overflow-y-auto pb-8">
              <div className="rounded-[24px] border border-white/10 bg-white/5 p-5">
                <p className="text-base leading-7 text-white">{detailQuery.data.content}</p>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <DetailItem label="Type" value={detailQuery.data.type ?? 'unknown'} />
                <DetailItem label="State" value={detailQuery.data.state} />
                <DetailItem label="Updated" value={formatTimestamp(detailQuery.data.updated_at)} />
                <DetailItem label="Store Count" value={String(detailQuery.data.store_count ?? 0)} />
                <DetailItem label="Use Count" value={String(detailQuery.data.use_count ?? 0)} />
              </div>

              <div>
                <p className="text-xs font-medium uppercase tracking-[0.24em] text-slate-400">Kinds</p>
                <div className="mt-3 flex flex-wrap gap-2">
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
                    <span className="text-sm text-slate-400">No kinds</span>
                  )}
                </div>
              </div>

              {detailQuery.data.metadata ? (
                <div>
                  <p className="text-xs font-medium uppercase tracking-[0.24em] text-slate-400">Metadata</p>
                  <pre className="mt-3 overflow-x-auto rounded-[24px] border border-white/10 bg-slate-950/80 p-4 text-xs leading-6 text-slate-300">
                    {JSON.stringify(detailQuery.data.metadata, null, 2)}
                  </pre>
                </div>
              ) : null}

              <div className="flex items-center gap-3">
               <button
                  type="button"
                  className="rounded-full bg-sky-400 px-4 py-2 text-sm font-medium text-slate-950 transition hover:bg-sky-300 disabled:cursor-not-allowed disabled:bg-slate-700 disabled:text-slate-300"
                  disabled={stateMutation.isPending}
                  onClick={() => stateMutation.mutate(nextState)}
                >
                  {stateMutation.isPending ? pendingLabel : actionLabel}
                </button>
                {stateMutation.isSuccess ? <p className="text-sm text-emerald-300">{successLabel}</p> : null}
              </div>

              {stateMutation.isError ? <p className="text-sm text-red-300">State update failed.</p> : null}
            </div>
          ) : null}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[20px] border border-white/10 bg-white/5 p-4">
      <p className="text-xs font-medium uppercase tracking-[0.24em] text-slate-400">{label}</p>
      <p className="mt-2 text-sm text-white">{value}</p>
    </div>
  )
}
