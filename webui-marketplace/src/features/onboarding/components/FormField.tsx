import type { ReactNode } from 'react';
import { cn } from '../../../lib/cn';

export const inputClassName =
  'w-full rounded-lg border border-slate-300 bg-panel px-3 py-2 text-sm text-ink shadow-sm outline-none transition-colors placeholder:text-ink-dim focus:border-neon focus:ring-2 focus:ring-neon/20 disabled:cursor-not-allowed disabled:bg-panel-soft';

export const inputErrorClassName =
  'border-boss focus:border-boss focus:ring-boss/20';

export interface FormFieldProps {
  id: string;
  label: string;
  required?: boolean;
  error?: string;
  hint?: string;
  className?: string;
  children: ReactNode;
}

export function FormField({
  id,
  label,
  required,
  error,
  hint,
  className,
  children,
}: FormFieldProps) {
  return (
    <div className={cn('space-y-1.5', className)}>
      <label
        htmlFor={id}
        className="flex items-center gap-1 text-sm font-medium text-ink"
      >
        {label}
        {required && (
          <span aria-hidden="true" className="text-gold">
            *
          </span>
        )}
      </label>
      {children}
      {error ? (
        <p role="alert" className="text-xs text-boss">
          {error}
        </p>
      ) : hint ? (
        <p className="text-xs text-ink-dim">{hint}</p>
      ) : null}
    </div>
  );
}
