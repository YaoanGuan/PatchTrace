import { useEffect, type ReactNode } from 'react';
import { X } from 'lucide-react';
import { cn } from '../../lib/cn';

export type ModalSize = 'sm' | 'md' | 'lg';

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  description?: string;
  children?: ReactNode;
  footer?: ReactNode;
  size?: ModalSize;
  className?: string;
}

const sizeClasses: Record<ModalSize, string> = {
  sm: 'max-w-sm',
  md: 'max-w-lg',
  lg: 'max-w-3xl',
};

export function Modal({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  size = 'md',
  className,
}: ModalProps) {
  useEffect(() => {
    if (!open) {
      return undefined;
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [open, onClose]);

  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div
        className="absolute inset-0 bg-black/75 backdrop-blur-sm"
        aria-hidden="true"
        onClick={onClose}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={cn(
          'game-panel relative flex max-h-[90vh] w-full flex-col overflow-hidden',
          sizeClasses[size],
          className,
        )}
      >
        {(title || description) && (
          <div className="flex items-start justify-between gap-4 border-b border-panel-line px-5 py-4">
            <div className="space-y-1">
              {title && (
                <h2 className="text-base font-semibold tracking-wide text-ink">
                  {title}
                </h2>
              )}
              {description && (
                <p className="text-xs text-ink-muted">{description}</p>
              )}
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label="关闭"
              className="rounded p-1 text-ink-dim transition-colors hover:bg-panel-soft hover:text-gold"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        )}
        <div className="overflow-y-auto px-5 py-4">{children}</div>
        {footer && (
          <div className="border-t border-panel-line bg-panel-soft/60 px-5 py-3">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}
