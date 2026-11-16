import { Loader2, Star } from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import { Progress } from '../../../components/ui/Progress';
import { cn } from '../../../lib/cn';
import type { ReputationSummary, User } from '../../../types/domain';

export interface ReputationScoreCardProps {
  summary: ReputationSummary | null;
  fallbackUser: User | null;
  usedFallback?: boolean;
  ratingCount?: number;
  loading?: boolean;
  className?: string;
}

export function ReputationScoreCard({
  summary,
  fallbackUser,
  usedFallback,
  ratingCount = 0,
  loading,
  className,
}: ReputationScoreCardProps) {
  const score = summary?.score ?? fallbackUser?.reputation.score;
  const dimensions =
    summary?.dimensions && summary.dimensions.length > 0
      ? summary.dimensions
      : (fallbackUser?.skills ?? []);
  const fallbackReputation = fallbackUser?.reputation;

  if (loading && !summary && !fallbackUser) {
    return (
      <Card
        className={cn(
          'flex items-center justify-center gap-2 p-8 text-sm text-slate-500',
          className,
        )}
      >
        <Loader2 className="h-4 w-4 animate-spin" />
        正在加载信誉数据…
      </Card>
    );
  }

  if (typeof score !== 'number' || !Number.isFinite(score)) {
    return (
      <EmptyState
        className={className}
        title="暂无信誉数据"
        description="完成首个任务并获得评价后，这里会展示双向信誉总分与维度。"
      />
    );
  }

  const onTimeRate =
    typeof fallbackReputation?.onTimeRate === 'number'
      ? `${Math.round(fallbackReputation.onTimeRate * 100)}%`
      : '—';
  const completedTasks = fallbackReputation?.completedTasks ?? '—';
  const disputes = fallbackReputation?.disputes ?? '—';
  const roundedScore = Math.max(0, Math.min(5, Math.round(score)));

  return (
    <Card className={cn('p-5 sm:p-6', className)}>
      <div className="grid gap-5 lg:grid-cols-[minmax(0,260px)_1fr]">
        <div className="rounded-xl bg-gradient-to-br from-indigo-600 to-indigo-500 p-5 text-white">
          <p className="text-xs font-medium uppercase tracking-wide text-indigo-100">
            双向信誉总分
          </p>
          <div className="mt-2 flex items-end gap-2">
            <span className="text-4xl font-semibold tabular-nums">
              {score.toFixed(1)}
            </span>
            <span className="pb-1 text-sm text-indigo-100">/ 5.0</span>
          </div>
          <div className="mt-2 flex items-center gap-1 text-amber-300">
            {[1, 2, 3, 4, 5].map((value) => (
              <Star
                key={value}
                aria-hidden="true"
                className={cn(
                  'h-4 w-4',
                  value <= roundedScore
                    ? 'fill-current'
                    : 'text-indigo-300/60',
                )}
              />
            ))}
            <span className="ml-1 text-xs text-indigo-100">
              {ratingCount} 条评分
            </span>
          </div>

          <dl className="mt-4 grid grid-cols-3 gap-2 border-t border-white/20 pt-3 text-center">
            <div>
              <dt className="text-[11px] text-indigo-100">完成任务</dt>
              <dd className="text-sm font-semibold tabular-nums">
                {completedTasks}
              </dd>
            </div>
            <div>
              <dt className="text-[11px] text-indigo-100">准时率</dt>
              <dd className="text-sm font-semibold tabular-nums">
                {onTimeRate}
              </dd>
            </div>
            <div>
              <dt className="text-[11px] text-indigo-100">争议次数</dt>
              <dd className="text-sm font-semibold tabular-nums">{disputes}</dd>
            </div>
          </dl>
        </div>

        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-sm font-semibold text-slate-900">信誉维度</h3>
            <div className="flex items-center gap-2">
              {usedFallback && <Badge variant="info">账号概览兜底</Badge>}
              {loading && (
                <span className="inline-flex items-center gap-1 text-xs text-slate-400">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  同步中
                </span>
              )}
            </div>
          </div>

          {dimensions.length > 0 ? (
            <div className="space-y-3">
              {dimensions.map((dimension) => (
                <div key={dimension.key} className="space-y-1">
                  <div className="flex items-center justify-between text-xs">
                    <span className="text-slate-600">{dimension.label}</span>
                    <span className="font-medium tabular-nums text-slate-900">
                      {dimension.score}
                    </span>
                  </div>
                  <Progress value={dimension.score} />
                </div>
              ))}
            </div>
          ) : (
            <p className="text-xs text-slate-400">暂无维度数据。</p>
          )}

          {usedFallback && (
            <p className="text-xs text-slate-400">
              实时信誉接口暂无可用维度，当前展示 getMe() 账号概览与能力维度兜底。
            </p>
          )}
        </div>
      </div>
    </Card>
  );
}
