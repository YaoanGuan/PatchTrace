import { cn } from '../../lib/cn';

export interface ProgressProps {
  value: number;
  max?: number;
  label?: string;
  className?: string;
  barClassName?: string;
}

export function Progress({
  value,
  max = 100,
  label,
  className,
  barClassName,
}: ProgressProps) {
  const percent =
    max > 0 ? Math.max(0, Math.min(100, (value / max) * 100)) : 0;

  return (
    <div className={cn('w-full', className)}>
      {label && (
        <div className="mb-1 flex items-center justify-between text-xs text-slate-500">
          <span>{label}</span>
          <span>{Math.round(percent)}%</span>
        </div>
      )}
      <div
        role="progressbar"
        aria-valuenow={Math.round(percent)}
        aria-valuemin={0}
        aria-valuemax={100}
        className="h-2 w-full overflow-hidden rounded-full bg-slate-100"
      >
        <div
          className={cn(
            'h-full rounded-full bg-indigo-600 transition-all',
            barClassName,
          )}
          style={{ width: `${percent}%` }}
        />
      </div>
    </div>
  );
}
