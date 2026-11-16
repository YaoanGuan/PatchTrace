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
  default: 'bg-slate-100 text-slate-700',
  success: 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-600/20',
  warning: 'bg-amber-50 text-amber-700 ring-1 ring-inset ring-amber-600/20',
  danger: 'bg-rose-50 text-rose-700 ring-1 ring-inset ring-rose-600/20',
  info: 'bg-indigo-50 text-indigo-700 ring-1 ring-inset ring-indigo-600/20',
  outline: 'border border-slate-300 bg-transparent text-slate-600',
};

export function Badge({
  variant = 'default',
  className,
  ...props
}: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium',
        variantClasses[variant],
        className,
      )}
      {...props}
    />
  );
}
