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
          ? 'border-emerald-200 bg-emerald-50'
          : 'border-amber-200 bg-amber-50',
        className,
      )}
    >
      <div className="flex items-start gap-3">
        <span
          className={cn(
            'rounded-lg p-2',
            verified
              ? 'bg-emerald-100 text-emerald-700'
              : 'bg-amber-100 text-amber-700',
          )}
        >
          <Icon className="h-4 w-4" />
        </span>
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-semibold text-slate-900">
              {CHANNEL_LABELS[status.channel]} ·{' '}
              {REVIEW_STATUS_LABELS[status.status]}
            </span>
            <Badge variant={verified ? 'success' : 'warning'}>
              {REVIEW_STATUS_LABELS[status.status]}
            </Badge>
          </div>
          <p className="text-xs text-slate-600">
            {verified
              ? '服务端已返回认证通过；平台保留后续复核与补充材料的权利。'
              : '资料已提交人工审核，预计 1–3 个工作日完成，请留意站内通知。'}
          </p>
        </div>
      </div>
      <span className="shrink-0 text-xs text-slate-500">
        提交时间{' '}
        {new Date(status.submittedAt).toLocaleString('zh-CN', {
          hour12: false,
        })}
      </span>
    </div>
  );
}
