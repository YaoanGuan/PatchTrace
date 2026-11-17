import { GitBranch } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import { cn } from '../../../lib/cn';
import type { PipelineStage as PipelineStageModel, TaskDetail } from '../../../types/domain';
import {
  deriveStageStatuses,
  STAGE_LABEL,
  STAGE_ORDER,
} from '../utils';
import { CrossCheckBadge } from './CrossCheckBadge';
import { PipelineStage } from './PipelineStage';
import { ValidationVerdictCard } from './ValidationVerdictCard';

export interface AIValidationPipelineProps {
  task: TaskDetail | null;
  loading?: boolean;
  className?: string;
  showVerdict?: boolean;
}

export function AIValidationPipeline({
  task,
  loading,
  className,
  showVerdict = true,
}: AIValidationPipelineProps) {
  if (loading && !task) {
    return (
      <Card className={className}>
        <div className="space-y-3 p-4">
          <div className="h-5 w-40 animate-pulse rounded bg-slate-100" />
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {Array.from({ length: 4 }).map((_, index) => (
              <div
                key={index}
                className="h-28 animate-pulse rounded-lg bg-slate-50"
              />
            ))}
          </div>
        </div>
      </Card>
    );
  }

  if (!task) {
    return (
      <EmptyState
        className={className}
        title="暂无可展示的关卡进度"
        description="选择一个副本后，可查看 AI 出招、裁判复核、规则扫描与掉落结算四个关卡。"
      />
    );
  }

  const derived = deriveStageStatuses(task);
  const recorded = new Map<PipelineStageModel['key'], PipelineStageModel>(
    task.pipeline.stages.map((stage) => [stage.key, stage]),
  );
  const stages = STAGE_ORDER.map((key) => {
    const stage = recorded.get(key);
    return {
      key,
      status: derived[key],
      at: stage?.at,
      by: stage?.by,
      summary: stage?.summary,
    };
  });

  const currentStageKey =
    STAGE_ORDER.find((key) => derived[key] === 'RUNNING') ??
    STAGE_ORDER.find((key) => derived[key] === 'FAILED') ??
    [...STAGE_ORDER].reverse().find((key) => derived[key] === 'DONE') ??
    STAGE_ORDER[0];

  return (
    <div className={cn('space-y-3', className)}>
      <Card>
        <div className="space-y-4 p-4">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div>
              <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink">
                <GitBranch className="h-4 w-4 text-neon" />
                关卡进度
              </h2>
              <p className="mt-0.5 text-xs text-ink-muted">
                当前关卡：{STAGE_LABEL[currentStageKey]} · 副本状态随关卡推进同步更新
              </p>
            </div>
            <CrossCheckBadge status={derived.AI_REVIEW} />
          </div>

          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {stages.map((stage, index) => (
              <PipelineStage
                key={stage.key}
                keyName={stage.key}
                status={stage.status}
                at={stage.at}
                by={stage.by}
                summary={stage.summary}
                index={index}
                isLast={index === stages.length - 1}
              />
            ))}
          </div>

          <p className="rounded-lg border border-panel-line bg-panel-soft px-3 py-2 text-xs leading-5 text-ink-muted">
            平台不直接信任单个 AI：出招 AI 与平台裁判 AI 相互隔离，
            战利品结论需连携验证一致后，才进入规则扫描与掉落结算。
          </p>
        </div>
      </Card>

      {showVerdict && <ValidationVerdictCard task={task} />}
    </div>
  );
}
