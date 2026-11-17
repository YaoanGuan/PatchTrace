import { Clock, ShieldCheck } from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { cn } from '../../../lib/cn';
import { CHANNEL_LABELS, REVIEW_STATUS_LABELS } from '../constants';
import type { RegistrationStatus } from '../types';

export interface RegistrationStatusBannerProps {
  status: RegistrationStatus | null;
  className?: string;
}

export function RegistrationStatusBanner({
  status,
  className,
}: RegistrationStatusBannerProps) {
  if (!status) {
    return null;
  }

  const verified = status.status === 'VERIFIED';
  const Icon = verified ? ShieldCheck : Clock;

  return (
    <div
      role="status"
      aria-live="polite"
      className={cn(
        'flex flex-col gap-3 rounded-xl border p-4 sm:flex-row sm:items-center sm:justify-between',
        verified
          ? 'border-clear/30 bg-clear/10'
          : 'border-gold/30 bg-gold/10',
        className,
      )}
    >
      <div className="flex items-start gap-3">
        <span
          className={cn(
            'rounded-lg p-2',
            verified
              ? 'bg-clear/15 text-clear'
              : 'bg-gold/15 text-gold',
          )}
        >
          <Icon className="h-4 w-4" />
        </span>
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-semibold text-ink">
              {CHANNEL_LABELS[status.channel]} ·{' '}
              {REVIEW_STATUS_LABELS[status.status]}
            </span>
            <Badge variant={verified ? 'success' : 'warning'}>
              {REVIEW_STATUS_LABELS[status.status]}
            </Badge>
          </div>
          <p className="text-xs text-ink-muted">
            {verified
              ? '服务端已返回试炼通过；平台保留后续复核与补充材料的权利。'
              : '资料已递交人工审核，预计 1–3 个工作日完成，请留意站内通知。'}
          </p>
        </div>
      </div>
      <span className="shrink-0 text-xs text-ink-dim">
        递交时间{' '}
        <span className="font-mono">
          {new Date(status.submittedAt).toLocaleString('zh-CN', {
            hour12: false,
          })}
        </span>
      </span>
    </div>
  );
}
