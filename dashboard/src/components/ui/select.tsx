import * as SelectPrimitive from '@radix-ui/react-select'
import { Check, ChevronDown } from 'lucide-react'

type SelectOption = {
  label: string
  value: string
}

type SelectProps = {
  ariaLabel: string
  value: string
  placeholder: string
  disabled?: boolean
  options: SelectOption[]
  onValueChange: (value: string) => void
}

export function Select({ ariaLabel, value, placeholder, disabled = false, options, onValueChange }: SelectProps) {
  return (
    <SelectPrimitive.Root value={value} disabled={disabled} onValueChange={onValueChange}>
      <SelectPrimitive.Trigger
        aria-label={ariaLabel}
        className="flex w-full items-center justify-between rounded-2xl border border-white/10 bg-slate-950/70 px-4 py-3 text-sm text-white outline-none transition hover:border-white/20 focus:border-sky-400/60 disabled:cursor-not-allowed disabled:opacity-60"
      >
        <SelectPrimitive.Value placeholder={placeholder} />
        <SelectPrimitive.Icon>
          <ChevronDown className="h-4 w-4 text-slate-400" />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>

      <SelectPrimitive.Portal>
        <SelectPrimitive.Content
          position="popper"
          sideOffset={8}
          className="z-50 min-w-[var(--radix-select-trigger-width)] overflow-hidden rounded-2xl border border-white/10 bg-slate-950/98 p-1 shadow-2xl shadow-black/40"
        >
          <SelectPrimitive.Viewport className="p-1">
            {options.map((option) => (
              <SelectPrimitive.Item
                key={option.value}
                value={option.value}
                className="relative flex cursor-default select-none items-center rounded-xl py-2.5 pr-8 pl-3 text-sm text-slate-200 outline-none data-[highlighted]:bg-sky-400/15 data-[highlighted]:text-white"
              >
                <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
                <SelectPrimitive.ItemIndicator className="absolute right-3 inline-flex items-center text-sky-300">
                  <Check className="h-4 w-4" />
                </SelectPrimitive.ItemIndicator>
              </SelectPrimitive.Item>
            ))}
          </SelectPrimitive.Viewport>
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  )
}
