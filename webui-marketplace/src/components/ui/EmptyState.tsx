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
        'flex flex-col items-center justify-center gap-2 rounded-md border border-dashed border-panel-line bg-panel/40 px-6 py-12 text-center',
        className,
      )}
    >
      {icon ?? <Inbox className="h-8 w-8 text-ink-dim" />}
      <h3 className="text-sm font-medium text-ink">{title}</h3>
      {description && (
        <p className="max-w-sm text-xs text-ink-muted">{description}</p>
      )}
      {action}
    </div>
  );
}
