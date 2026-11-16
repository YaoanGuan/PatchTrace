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
        'inline-flex flex-wrap items-center gap-1 rounded-lg bg-slate-100 p-1',
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
              'rounded-md px-3 py-1.5 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50',
              active
                ? 'bg-white text-slate-900 shadow-sm'
                : 'text-slate-500 hover:text-slate-800',
            )}
          >
            {item.label}
          </button>
        );
      })}
    </div>
  );
}
