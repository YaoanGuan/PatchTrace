import type { HTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

export type BadgeVariant =
  | 'default'
  | 'success'
  | 'warning'
  | 'danger'
  | 'info'
  | 'outline';

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant;
}

const variantClasses: Record<BadgeVariant, string> = {
  default: 'border-panel-line bg-panel-soft text-ink-muted',
  success: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300',
  warning: 'border-amber-500/30 bg-amber-500/10 text-amber-300',
  danger: 'border-rose-500/30 bg-rose-500/10 text-rose-300',
  info: 'border-neon/30 bg-neon/10 text-neon',
  outline: 'border-ink-dim/40 bg-transparent text-ink-muted',
};

export function Badge({
  variant = 'default',
  className,
  ...props
}: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded border px-2 py-0.5 text-[11px] font-medium tracking-wide',
        variantClasses[variant],
        className,
      )}
      {...props}
    />
  );
}
