import { Link } from 'react-router-dom';
import { CalendarClock, ShieldCheck, Star, Users } from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import type { TaskSummary as TaskSummaryModel } from '../../../types/domain';
import {
  categoryLabel,
  formatDateTime,
  formatMoney,
  skillLabel,
  STATUS_BADGE_VARIANT,
  STATUS_LABEL,
} from '../utils';

export interface TaskCardProps {
  task: TaskSummaryModel;
  className?: string;
}

export function TaskCard({ task, className }: TaskCardProps) {
  return (
    <Link
      to={`/marketplace/tasks/${task.id}`}
      className={cn('group block focus:outline-none', className)}
    >
      <Card className="h-full space-y-3 p-4 transition-all group-hover:-translate-y-0.5 group-hover:border-indigo-200 group-hover:shadow-md group-focus-visible:ring-2 group-focus-visible:ring-indigo-500/40">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-1.5">
              <Badge variant={STATUS_BADGE_VARIANT[task.status]}>
                {STATUS_LABEL[task.status]}
              </Badge>
              <Badge variant="outline">{categoryLabel(task.category)}</Badge>
            </div>
            <h3 className="line-clamp-2 text-sm font-semibold leading-5 text-slate-900 group-hover:text-indigo-700">
              {task.title}
            </h3>
          </div>
          <p className="shrink-0 text-right font-mono text-base font-semibold text-gold">
            {formatMoney(task.reward.amount)}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-500">
          <span className="inline-flex items-center gap-1">
            <Star className="h-3 w-3 text-amber-500" />
            {task.publisher.displayName}
            <span className="font-medium text-slate-700">
              {task.publisher.reputationScore.toFixed(1)}
            </span>
          </span>
          <span className="inline-flex items-center gap-1">
            <CalendarClock className="h-3 w-3" />
            截止 {formatDateTime(task.deadline)}
          </span>
          <span className="inline-flex items-center gap-1">
            <Users className="h-3 w-3" />
            {task.acceptCount} 人关注
          </span>
          {task.validationScore > 0 && (
            <span className="inline-flex items-center gap-1 text-emerald-600">
              <ShieldCheck className="h-3 w-3" />
              验证分 {task.validationScore}
            </span>
          )}
        </div>

        {task.requiredSkills.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {task.requiredSkills.map((skill) => (
              <Badge key={skill} variant="default" className="font-normal">
                {skillLabel(skill)}
              </Badge>
            ))}
          </div>
        )}
      </Card>
    </Link>
  );
}
