import { cn } from '../../lib/cn';

export interface ProgressProps {
  value: number;
  max?: number;
  label?: string;
  className?: string;
  barClassName?: string;
}

/** XP 风格进度条：金色到青色，带流光。 */
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
        <div className="mb-1 flex items-center justify-between text-[11px] text-ink-muted">
          <span className="tracking-wide">{label}</span>
          <span className="font-mono text-gold">{Math.round(percent)}%</span>
        </div>
      )}
      <div
        role="progressbar"
        aria-valuenow={Math.round(percent)}
        aria-valuemin={0}
        aria-valuemax={100}
        className="h-2.5 w-full overflow-hidden rounded-full border border-panel-line bg-abyss"
      >
        <div
          className={cn(
            'relative h-full overflow-hidden rounded-full bg-gradient-to-r from-gold via-gold to-neon transition-all',
            barClassName,
          )}
          style={{ width: `${percent}%` }}
        >
          {percent > 8 && (
            <span className="absolute inset-y-0 left-0 w-1/3 animate-shine bg-white/35 blur-[5px]" />
          )}
        </div>
      </div>
    </div>
  );
}
