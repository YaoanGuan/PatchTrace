import type { ReactNode } from 'react';
import { Inbox } from 'lucide-react';
import { cn } from '../../lib/cn';

export interface EmptyStateProps {
  title: string;
  description?: string;
  icon?: ReactNode;
  action?: ReactNode;
  className?: string;
}

export function EmptyState({
  title,
  description,
  icon,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-slate-300 bg-slate-50 px-6 py-12 text-center',
        className,
      )}
    >
      {icon ?? <Inbox className="h-8 w-8 text-slate-400" />}
      <h3 className="text-sm font-medium text-slate-700">{title}</h3>
      {description && (
        <p className="max-w-sm text-xs text-slate-500">{description}</p>
      )}
      {action}
    </div>
  );
}
