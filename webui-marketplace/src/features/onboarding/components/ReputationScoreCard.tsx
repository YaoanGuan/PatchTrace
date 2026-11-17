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
          'flex items-center justify-center gap-2 p-8 text-sm text-ink-muted',
          className,
        )}
      >
        <Loader2 className="h-4 w-4 animate-spin" />
        正在加载声望数据…
      </Card>
    );
  }

  if (typeof score !== 'number' || !Number.isFinite(score)) {
    return (
      <EmptyState
        className={className}
        title="暂无声望数据"
        description="完成首个副本并获得战报点评后，这里会展示声望总分与维度。"
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
        <div className="rounded-xl border border-arcane/30 bg-gradient-to-br from-arcane/25 via-panel-soft to-panel-soft p-5 text-ink">
          <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">
            声望总分
          </p>
          <div className="mt-2 flex items-end gap-2">
            <span className="font-mono text-4xl font-semibold tabular-nums text-gold">
              {score.toFixed(1)}
            </span>
            <span className="pb-1 text-sm text-ink-muted">/ 5.0</span>
          </div>
          <div className="mt-2 flex items-center gap-1 text-gold">
            {[1, 2, 3, 4, 5].map((value) => (
              <Star
                key={value}
                aria-hidden="true"
                className={cn(
                  'h-4 w-4',
                  value <= roundedScore
                    ? 'fill-current'
                    : 'text-ink-dim/50',
                )}
              />
            ))}
            <span className="ml-1 font-mono text-xs text-ink-muted">
              {ratingCount} 条战绩评分
            </span>
          </div>

          <dl className="mt-4 grid grid-cols-3 gap-2 border-t border-panel-line pt-3 text-center">
            <div>
              <dt className="text-[11px] text-ink-dim">通关副本</dt>
              <dd className="font-mono text-sm font-semibold tabular-nums">
                {completedTasks}
              </dd>
            </div>
            <div>
              <dt className="text-[11px] text-ink-dim">准时率</dt>
              <dd className="font-mono text-sm font-semibold tabular-nums">
                {onTimeRate}
              </dd>
            </div>
            <div>
              <dt className="text-[11px] text-ink-dim">争议战报</dt>
              <dd className="font-mono text-sm font-semibold tabular-nums">
                {disputes}
              </dd>
            </div>
          </dl>
        </div>

        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-sm font-semibold text-ink">声望维度</h3>
            <div className="flex items-center gap-2">
              {usedFallback && <Badge variant="info">角色概览兜底</Badge>}
              {loading && (
                <span className="inline-flex items-center gap-1 text-xs text-ink-dim">
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
                    <span className="text-ink-muted">{dimension.label}</span>
                    <span className="font-mono font-medium tabular-nums text-ink">
                      {dimension.score}
                    </span>
                  </div>
                  <Progress value={dimension.score} />
                </div>
              ))}
            </div>
          ) : (
            <p className="text-xs text-ink-dim">暂无维度数据。</p>
          )}

          {usedFallback && (
            <p className="text-xs text-ink-dim">
              实时声望接口暂无可用维度，当前展示 getMe() 角色概览与能力维度兜底。
            </p>
          )}
        </div>
      </div>
    </Card>
  );
}
