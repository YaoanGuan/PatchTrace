import { cn } from '../../lib/cn';

export interface TabItem {
  key: string;
  label: string;
  disabled?: boolean;
}

export interface TabsProps {
  items: TabItem[];
  value: string;
  onChange: (key: string) => void;
  className?: string;
}

export function Tabs({ items, value, onChange, className }: TabsProps) {
  return (
    <div
      role="tablist"
      className={cn(
        'inline-flex flex-wrap items-center gap-1 rounded-md border border-panel-line bg-panel-soft/70 p-1',
        className,
      )}
    >
      {items.map((item) => {
        const active = item.key === value;
        return (
          <button
            key={item.key}
            type="button"
            role="tab"
            aria-selected={active}
            disabled={item.disabled}
            onClick={() => onChange(item.key)}
            className={cn(
              'rounded px-3 py-1.5 text-xs font-medium tracking-wide transition-all disabled:cursor-not-allowed disabled:opacity-50',
              active
                ? 'bg-panel text-gold shadow-glow'
                : 'text-ink-muted hover:bg-panel hover:text-ink',
            )}
          >
            {item.label}
          </button>
        );
      })}
    </div>
  );
}
