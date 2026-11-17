import {
  CalendarClock,
  CircleCheck,
  Lock,
  Star,
  UserRound,
} from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { Card } from '../../../components/ui/Card';
import type { EscrowStatus, TaskDetail } from '../../../types/domain';
import {
  categoryLabel,
  formatDateTime,
  formatMoney,
  skillLabel,
  STATUS_BADGE_VARIANT,
  STATUS_LABEL,
} from '../utils';

const ESCROW_LABEL: Record<EscrowStatus, string> = {
  NONE: '未缴纳赏金押金',
  FROZEN: '赏金押金已冻结',
  RELEASED: '赏金押金已释放',
  REFUNDED: '赏金押金已退回',
};

export interface TaskSummaryProps {
  task: TaskDetail;
  className?: string;
}

export function TaskSummary({ task, className }: TaskSummaryProps) {
  return (
    <Card className={className}>
      <div className="space-y-4 p-5">
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant={STATUS_BADGE_VARIANT[task.status]}>
              {STATUS_LABEL[task.status]}
            </Badge>
            <Badge variant="outline">{categoryLabel(task.category)}</Badge>
            <Badge variant={task.escrow.state === 'FROZEN' ? 'warning' : 'default'}>
              <Lock className="h-3 w-3" />
              {ESCROW_LABEL[task.escrow.state]}
            </Badge>
          </div>
          <h1 className="text-xl font-semibold tracking-tight text-slate-900">
            {task.title}
          </h1>
          <p className="text-sm leading-6 text-slate-600">{task.description}</p>
        </div>

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div className="rounded-lg border border-gold/25 bg-gold/5 p-3">
            <p className="text-xs text-gold/80">金币赏金</p>
            <p className="mt-0.5 font-mono text-lg font-semibold text-gold">
              {formatMoney(task.reward.amount)}
            </p>
          </div>
          <div className="rounded-lg bg-slate-50 p-3">
            <p className="flex items-center gap-1 text-xs text-slate-500">
              <CalendarClock className="h-3.5 w-3.5" />
              截止时间
            </p>
            <p className="mt-0.5 text-sm font-medium text-slate-800">
              {formatDateTime(task.deadline)}
            </p>
          </div>
          <div className="rounded-lg bg-slate-50 p-3">
            <p className="flex items-center gap-1 text-xs text-slate-500">
              <Star className="h-3.5 w-3.5 text-amber-500" />
              发布者
            </p>
            <p className="mt-0.5 truncate text-sm font-medium text-slate-800">
              {task.publisher.displayName} ·{' '}
              {task.publisher.reputationScore.toFixed(1)}
            </p>
          </div>
          <div className="rounded-lg bg-slate-50 p-3">
            <p className="flex items-center gap-1 text-xs text-slate-500">
              <UserRound className="h-3.5 w-3.5" />
              AI 指挥官
            </p>
            <p className="mt-0.5 truncate text-sm font-medium text-slate-800">
              {task.assignee
                ? `${task.assignee.displayName} · ${task.assignee.reputationScore.toFixed(1)}`
                : '待接受挑战'}
            </p>
          </div>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-1.5">
            <h2 className="text-xs font-semibold uppercase tracking-wide text-slate-500">
              通关条件
            </h2>
            <ul className="space-y-1">
              {task.acceptanceCriteria.length === 0 && (
                <li className="text-xs text-slate-400">未填写通关条件</li>
              )}
              {task.acceptanceCriteria.map((item) => (
                <li
                  key={item}
                  className="flex items-start gap-1.5 text-xs leading-5 text-slate-600"
                >
                  <CircleCheck className="mt-0.5 h-3.5 w-3.5 shrink-0 text-emerald-500" />
                  {item}
                </li>
              ))}
            </ul>
          </div>
          <div className="space-y-1.5">
            <h2 className="text-xs font-semibold uppercase tracking-wide text-slate-500">
              出战技能
            </h2>
            <div className="flex flex-wrap gap-1">
              {task.requiredSkills.length === 0 && (
                <span className="text-xs text-slate-400">未指定技能</span>
              )}
              {task.requiredSkills.map((skill) => (
                <Badge key={skill} variant="default" className="font-normal">
                  {skillLabel(skill)}
                </Badge>
              ))}
            </div>
            <p className="pt-2 text-xs text-slate-400">
              副本编号 {task.id} · 发布于 {formatDateTime(task.createdAt)}
            </p>
          </div>
        </div>
      </div>
    </Card>
  );
}
