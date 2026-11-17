import { cn } from '../../lib/cn';

export interface XpBarProps {
  value: number;
  label?: string;
  compact?: boolean;
  className?: string;
}

/** 升级经验条：金色到青色渐变 + 流光。 */
export function XpBar({ value, label, compact = false, className }: XpBarProps) {
  const percent = Math.max(0, Math.min(100, Math.round(value)));

  return (
    <div className={cn('w-full', className)}>
      {label && (
        <div className="mb-1 flex items-center justify-between font-mono text-[10px] uppercase tracking-wider text-ink-dim">
          <span>{label}</span>
          <span className="text-gold">{percent}%</span>
        </div>
      )}
      <div
        role="progressbar"
        aria-valuenow={percent}
        aria-valuemin={0}
        aria-valuemax={100}
        className={cn(
          'w-full overflow-hidden rounded-full border border-panel-line bg-abyss',
          compact ? 'h-1.5' : 'h-2.5',
        )}
      >
        <div
          className="relative h-full overflow-hidden rounded-full bg-gradient-to-r from-gold via-gold to-neon transition-all"
          style={{ width: `${percent}%` }}
        >
          {percent > 8 && (
            <span className="absolute inset-y-0 left-0 w-1/3 animate-shine bg-white/35 blur-[4px]" />
          )}
        </div>
      </div>
    </div>
  );
}
