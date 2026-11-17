import { Loader2, Star } from 'lucide-react';
import { Badge, type BadgeVariant } from '../../../components/ui/Badge';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import { cn } from '../../../lib/cn';
import type { Rating, RatingDirection } from '../../../types/domain';
import { RATING_DIRECTION_LABELS } from '../constants';

export interface RatingHistoryProps {
  ratings: Rating[];
  loading?: boolean;
  className?: string;
}

const directionVariants: Record<RatingDirection, BadgeVariant> = {
  EMPLOYER_TO_COMMANDER: 'info',
  COMMANDER_TO_EMPLOYER: 'default',
};

export function RatingHistory({
  ratings,
  loading,
  className,
}: RatingHistoryProps) {
  const ordered = [...ratings].sort((a, b) =>
    b.createdAt.localeCompare(a.createdAt),
  );

  return (
    <Card className={cn('p-5', className)}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-ink">战绩评分历史</h3>
        <Badge variant="outline">{ordered.length} 条战报</Badge>
      </div>

      {loading ? (
        <div className="mt-4 flex items-center justify-center gap-2 py-8 text-sm text-ink-muted">
          <Loader2 className="h-4 w-4 animate-spin" />
          战报历史加载中…
        </div>
      ) : ordered.length === 0 ? (
        <EmptyState
          className="mt-4 border-none bg-transparent px-0 py-8"
          title="暂无战报点评"
          description="完成一次副本协作并提交声望互评后，战报历史会展示在这里。"
        />
      ) : (
        <ul className="mt-3 divide-y divide-panel-line">
          {ordered.map((rating) => (
            <li key={rating.id} className="space-y-2 py-4 first:pt-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={directionVariants[rating.direction]}>
                    {RATING_DIRECTION_LABELS[rating.direction]}
                  </Badge>
                  <span className="inline-flex items-center gap-1 font-mono text-sm font-medium text-ink">
                    <Star className="h-4 w-4 fill-gold text-gold" />
                    {rating.score.toFixed(1)}
                    <span className="font-mono text-xs font-normal text-ink-dim">
                      / 5.0
                    </span>
                  </span>
                </div>
                <div className="flex items-center gap-3 text-xs text-ink-dim">
                  <span className="font-mono">{rating.taskId}</span>
                  <span>{rating.createdAt.slice(0, 10)}</span>
                </div>
              </div>

              {rating.tags.length > 0 && (
                <div className="flex flex-wrap gap-1.5">
                  {rating.tags.map((tag) => (
                    <span
                      key={tag}
                      className="rounded-full bg-panel-soft px-2 py-0.5 text-[11px] text-ink-muted"
                    >
                      {tag}
                    </span>
                  ))}
                </div>
              )}

              {rating.comment && (
                <p className="text-sm leading-relaxed text-ink-muted">
                  {rating.comment}
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
