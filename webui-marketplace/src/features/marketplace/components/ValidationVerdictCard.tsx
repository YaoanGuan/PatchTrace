import { Bot, Scale, ShieldCheck } from 'lucide-react';
import { Badge, type BadgeVariant } from '../../../components/ui/Badge';
import { Card } from '../../../components/ui/Card';
import type { PipelineStageStatus, TaskDetail } from '../../../types/domain';
import { deriveStageStatuses } from '../utils';
import { CrossCheckBadge } from './CrossCheckBadge';

const VERDICT: Record<
  PipelineStageStatus,
  { title: string; variant: BadgeVariant; text: string }
> = {
  PENDING: {
    title: '待平台 AI 复核',
    variant: 'outline',
    text: '任务尚未进入 AI 复核阶段，平台不会在交付前预扣结论。',
  },
  RUNNING: {
    title: '平台 AI 复核中',
    variant: 'info',
    text: '平台审核 AI 正在与执行 AI 的结果做独立交叉比对，不直接采信执行方自证。',
  },
  DONE: {
    title: '交叉复核通过',
    variant: 'success',
    text: '平台审核 AI 独立复核通过，交付物可继续进入合规扫描与资金结算。',
  },
  FAILED: {
    title: '交叉复核未通过',
    variant: 'danger',
    text: '平台审核 AI 与执行方结论不一致，任务需返工或转入争议仲裁，托管资金保持冻结。',
  },
};

export interface ValidationVerdictCardProps {
  task: TaskDetail;
  className?: string;
}

export function ValidationVerdictCard({
  task,
  className,
}: ValidationVerdictCardProps) {
  const reviewStatus = deriveStageStatuses(task).AI_REVIEW;
  const verdict = VERDICT[reviewStatus];

  return (
    <Card className={className}>
      <div className="space-y-3 p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
            <Scale className="h-4 w-4 text-indigo-600" />
            审核结论
          </h3>
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant={verdict.variant}>{verdict.title}</Badge>
            <CrossCheckBadge status={reviewStatus} />
          </div>
        </div>

        <p className="text-sm leading-6 text-slate-600">{verdict.text}</p>

        <div className="grid gap-2 sm:grid-cols-2">
          <div className="flex items-start gap-2 rounded-lg bg-slate-50 p-3">
            <Bot className="mt-0.5 h-4 w-4 shrink-0 text-slate-500" />
            <div className="min-w-0">
              <p className="text-xs text-slate-400">执行方</p>
              <p className="truncate text-sm font-medium text-slate-800">
                {task.assignee?.displayName ?? '待接单'}
              </p>
              <p className="text-xs text-slate-500">
                {task.assignee
                  ? `信誉 ${task.assignee.reputationScore.toFixed(1)}`
                  : '尚无 AI 指挥官领取任务'}
              </p>
            </div>
          </div>
          <div className="flex items-start gap-2 rounded-lg bg-indigo-50/60 p-3">
            <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-indigo-600" />
            <div className="min-w-0">
              <p className="text-xs text-indigo-500">审核方</p>
              <p className="text-sm font-medium text-indigo-900">平台审核 AI</p>
              <p className="text-xs text-indigo-700/80">
                独立于执行方，结果写入哈希链
              </p>
            </div>
          </div>
        </div>
      </div>
    </Card>
  );
}
