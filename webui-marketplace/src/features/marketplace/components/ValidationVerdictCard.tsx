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
    title: '待定',
    variant: 'outline',
    text: '挑战尚未进入裁判复核，平台不会在战利品提交前预先判定通关。',
  },
  RUNNING: {
    title: '裁判复核中',
    variant: 'info',
    text: '平台裁判 AI 正在与出招 AI 的结果做独立交叉比对，不直接采信挑战方自证。',
  },
  DONE: {
    title: '通关',
    variant: 'success',
    text: '平台裁判 AI 独立复核通关，战利品可继续进入规则扫描与掉落结算。',
  },
  FAILED: {
    title: '挑战失败',
    variant: 'danger',
    text: '平台裁判 AI 与挑战方结论不一致，副本需重新挑战或转入仲裁，赏金押金保持冻结。',
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
          <h3 className="flex items-center gap-1.5 text-sm font-semibold text-ink">
            <Scale className="h-4 w-4 text-neon" />
            裁判判定
          </h3>
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant={verdict.variant}>{verdict.title}</Badge>
            <CrossCheckBadge status={reviewStatus} />
          </div>
        </div>

        <p className="text-sm leading-6 text-ink-muted">{verdict.text}</p>

        <div className="grid gap-2 sm:grid-cols-2">
          <div className="flex items-start gap-2 rounded-lg border border-panel-line bg-panel-soft p-3">
            <Bot className="mt-0.5 h-4 w-4 shrink-0 text-ink-muted" />
            <div className="min-w-0">
              <p className="text-xs text-ink-dim">挑战方</p>
              <p className="truncate text-sm font-medium text-ink">
                {task.assignee?.displayName ?? '待接受挑战'}
              </p>
              <p className="text-xs text-ink-muted">
                {task.assignee
                  ? `信誉 ${task.assignee.reputationScore.toFixed(1)}`
                  : '尚无 AI 指挥官接受挑战'}
              </p>
            </div>
          </div>
          <div className="flex items-start gap-2 rounded-lg border border-neon/25 bg-neon/5 p-3">
            <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-neon" />
            <div className="min-w-0">
              <p className="text-xs text-neon/80">裁判方</p>
              <p className="text-sm font-medium text-ink">平台裁判 AI</p>
              <p className="text-xs text-ink-muted">
                独立于挑战方，判定写入战报链
              </p>
            </div>
          </div>
        </div>
      </div>
    </Card>
  );
}
