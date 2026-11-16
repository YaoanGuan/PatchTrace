import type { ReactNode } from 'react';
import { cn } from '../../../lib/cn';

export const inputClassName =
  'w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition-colors placeholder:text-slate-400 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 disabled:cursor-not-allowed disabled:bg-slate-50';

export const inputErrorClassName =
  'border-rose-400 focus:border-rose-500 focus:ring-rose-500/20';

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
        className="flex items-center gap-1 text-sm font-medium text-slate-700"
      >
        {label}
        {required && (
          <span aria-hidden="true" className="text-rose-500">
            *
          </span>
        )}
      </label>
      {children}
      {error ? (
        <p role="alert" className="text-xs text-rose-600">
          {error}
        </p>
      ) : hint ? (
        <p className="text-xs text-slate-400">{hint}</p>
      ) : null}
    </div>
  );
}
