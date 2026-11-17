import { useEffect, useMemo, useState } from 'react';
import { Bot, CalendarClock, CheckCircle2, Handshake, Star } from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import { useTaskStore } from '../../../app/store/taskStore';
import { useUiStore } from '../../../app/store/uiStore';
import type { TaskDetail, TaskSummary } from '../../../types/domain';
import {
  errorMessage,
  formatDateTime,
  formatMoney,
  skillLabel,
  STATUS_BADGE_VARIANT,
  STATUS_LABEL,
} from '../utils';

/** 与 mock POST /api/tasks/:id/accept 返回的租约时长保持一致。 */
const LEASE_DURATION_MS = 2 * 60 * 60 * 1000;

/** 列表摘要与详情共享的接单所需字段。 */
type AcceptableTask = TaskSummary | TaskDetail;

export interface AcceptTaskPanelProps {
  /** 单任务模式（详情页）。 */
  task?: AcceptableTask | null;
  /** 多任务模式（大厅）：从待接任务中选择。 */
  tasks?: AcceptableTask[];
  onAccepted?: (taskId: string) => void;
  className?: string;
}

export function AcceptTaskPanel({
  task,
  tasks,
  onAccepted,
  className,
}: AcceptTaskPanelProps) {
  const acceptTask = useTaskStore((state) => state.acceptTask);
  const pushToast = useUiStore((state) => state.pushToast);

  const options = useMemo<AcceptableTask[]>(() => {
    if (tasks && tasks.length > 0) {
      return tasks;
    }
    return task ? [task] : [];
  }, [tasks, task]);

  const firstOpenId =
    options.find((item) => item.status === 'PUBLISHED')?.id ?? options[0]?.id ?? '';

  const [selectedId, setSelectedId] = useState(firstOpenId);
  const [accepting, setAccepting] = useState(false);
  const [lease, setLease] = useState<{ taskId: string; expiresAt: string } | null>(
    null,
  );

  useEffect(() => {
    if (selectedId && options.some((item) => item.id === selectedId)) {
      return;
    }
    setSelectedId(firstOpenId);
  }, [firstOpenId, options, selectedId]);

  const selected = options.find((item) => item.id === selectedId) ?? null;
  const isOpen = selected?.status === 'PUBLISHED';
  const leaseShown = lease && selected && lease.taskId === selected.id;

  const handleAccept = async () => {
    if (!selected || !isOpen) {
      return;
    }
    setAccepting(true);
    try {
      await acceptTask(selected.id);
      const expiresAt = new Date(Date.now() + LEASE_DURATION_MS).toISOString();
      setLease({ taskId: selected.id, expiresAt });
      pushToast({
        id: `accept-${selected.id}`,
        title: '已接受挑战',
        description: `${selected.title} · 租约到期 ${formatDateTime(expiresAt)}`,
        variant: 'success',
      });
      onAccepted?.(selected.id);
    } catch (error) {
      pushToast({
        id: `accept-error-${selected.id}-${Date.now()}`,
        title: '接受挑战失败',
        description: errorMessage(error),
        variant: 'error',
      });
      console.error('[marketplace] acceptTask failed', error);
    } finally {
      setAccepting(false);
    }
  };

  return (
    <div className={className}>
      <Card className="space-y-3 p-4">
        <div>
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink">
            <Handshake className="h-4 w-4 text-neon" />
            接受挑战面板
          </h2>
          <p className="mt-0.5 text-xs text-ink-muted">
            AI 指挥官视角：接受挑战后获得限时租约，需在到期前提交可验证的战利品。
          </p>
        </div>

        {options.length === 0 ? (
          <EmptyState
            title="暂无可挑战副本"
            description="当前筛选条件下没有悬赏，等待雇主发布新悬赏。"
          />
        ) : (
          <>
            {options.length > 1 && (
              <label className="block space-y-1">
                <span className="text-xs font-medium text-slate-600">
                  选择副本
                </span>
                <select
                  value={selectedId}
                  onChange={(event) => setSelectedId(event.target.value)}
                  className="h-9 w-full rounded-lg border border-slate-300 bg-panel px-3 text-sm text-slate-900 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
                >
                  {options.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.title}（{STATUS_LABEL[item.status]}）
                    </option>
                  ))}
                </select>
              </label>
            )}

            {selected && (
              <div className="space-y-2 rounded-lg border border-panel-line bg-panel-soft p-3">
                <div className="flex items-start justify-between gap-2">
                  <p className="line-clamp-2 text-sm font-medium text-ink">
                    {selected.title}
                  </p>
                  <Badge
                    variant={STATUS_BADGE_VARIANT[selected.status]}
                    className="shrink-0 whitespace-normal text-right"
                  >
                    {STATUS_LABEL[selected.status]}
                  </Badge>
                </div>
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-muted">
                  <span className="font-mono font-semibold text-gold">
                    {formatMoney(selected.reward.amount)}
                  </span>
                  <span className="inline-flex items-center gap-1">
                    <Star className="h-3 w-3 text-amber-500" />
                    {selected.publisher.displayName} ·{' '}
                    {selected.publisher.reputationScore.toFixed(1)}
                  </span>
                  <span className="inline-flex items-center gap-1">
                    <CalendarClock className="h-3 w-3" />
                    截止 {formatDateTime(selected.deadline)}
                  </span>
                </div>
                {selected.requiredSkills.length > 0 && (
                  <div className="flex flex-wrap gap-1">
                    {selected.requiredSkills.map((skill) => (
                      <Badge key={skill} variant="outline" className="font-normal">
                        {skillLabel(skill)}
                      </Badge>
                    ))}
                  </div>
                )}
              </div>
            )}

            {leaseShown && lease ? (
              <div className="flex items-start gap-2 rounded-lg border border-clear/30 bg-clear/10 p-3 text-xs text-clear">
                <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />
                <span>
                  <span className="block font-medium">已接受挑战，租约生效</span>
                  租约到期时间：{formatDateTime(lease.expiresAt)}
                  ，到期未提交将释放副本。
                </span>
              </div>
            ) : isOpen ? (
              <Button
                className="w-full"
                onClick={() => void handleAccept()}
                disabled={accepting}
              >
                <Bot className="h-4 w-4" />
                {accepting ? '接受挑战中…' : '接受挑战并领取租约'}
              </Button>
            ) : (
              <p className="rounded-lg border border-panel-line bg-panel-soft p-3 text-xs text-ink-muted">
                该副本当前状态为「{selected ? STATUS_LABEL[selected.status] : '未知'}」，
                不可接受挑战。请选择状态为「悬赏中」的副本。
              </p>
            )}
          </>
        )}
      </Card>
    </div>
  );
}
