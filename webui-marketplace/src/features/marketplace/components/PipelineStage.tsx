import {
  Bot,
  CheckCircle2,
  ScanLine,
  ShieldCheck,
  Wallet,
  XCircle,
} from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { cn } from '../../../lib/cn';
import type {
  PipelineStageKey,
  PipelineStageStatus,
} from '../../../types/domain';
import {
  formatDateTime,
  STAGE_DESCRIPTION,
  STAGE_LABEL,
  STAGE_STATUS_LABEL,
  STAGE_STATUS_VARIANT,
} from '../utils';

const STAGE_ICON: Record<PipelineStageKey, typeof Bot> = {
  AI_GENERATION: Bot,
  AI_REVIEW: ShieldCheck,
  COMPLIANCE_SCAN: ScanLine,
  SETTLEMENT: Wallet,
};

const STATUS_RING: Record<PipelineStageStatus, string> = {
  PENDING: 'border-slate-200 text-slate-400',
  RUNNING: 'border-indigo-300 text-indigo-600',
  DONE: 'border-emerald-300 text-emerald-600',
  FAILED: 'border-rose-300 text-rose-600',
};

export interface PipelineStageProps {
  keyName: PipelineStageKey;
  status: PipelineStageStatus;
  at?: string;
  by?: string;
  summary?: string;
  index: number;
  isLast: boolean;
}

export function PipelineStage({
  keyName,
  status,
  at,
  by,
  summary,
  index,
  isLast,
}: PipelineStageProps) {
  const Icon = STAGE_ICON[keyName];
  const StatusIcon =
    status === 'DONE' ? CheckCircle2 : status === 'FAILED' ? XCircle : null;

  return (
    <div className="relative">
      {!isLast && (
        <span
          aria-hidden="true"
          className={cn(
            'absolute left-[calc(50%+1.5rem)] top-5 hidden h-px w-[calc(100%-3rem)] lg:block',
            status === 'DONE' ? 'bg-emerald-300' : 'bg-slate-200',
          )}
        />
      )}
      <div className="flex items-start gap-3 lg:flex-col lg:items-center lg:text-center">
        <span
          className={cn(
            'relative z-10 flex h-10 w-10 shrink-0 items-center justify-center rounded-full border-2 bg-white',
            STATUS_RING[status],
            status === 'RUNNING' && 'animate-pulse',
          )}
        >
          <Icon className="h-4 w-4" />
        </span>
        <div className="min-w-0 space-y-1 lg:flex lg:flex-col lg:items-center">
          <div className="flex flex-wrap items-center gap-1.5 lg:justify-center">
            <span className="text-[11px] font-medium uppercase tracking-wide text-slate-400">
              STEP {index + 1}
            </span>
            <span className="text-sm font-semibold text-slate-900">
              {STAGE_LABEL[keyName]}
            </span>
          </div>
          <Badge variant={STAGE_STATUS_VARIANT[status]} className="w-fit">
            {StatusIcon && <StatusIcon className="h-3 w-3" />}
            {STAGE_STATUS_LABEL[status]}
          </Badge>
          <p className="max-w-[16rem] text-xs leading-5 text-slate-500">
            {summary ?? STAGE_DESCRIPTION[keyName]}
          </p>
          <p className="text-[11px] text-slate-400">
            {at ? formatDateTime(at) : '尚未开始'}
          </p>
          {by && (
            <p className="max-w-[16rem] truncate text-[11px] text-slate-400" title={by}>
              {by}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
