import { useState } from 'react';
import {
  CheckCircle2,
  PlayCircle,
  RotateCcw,
  Send,
  ShieldCheck,
} from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { useTaskStore } from '../../../app/store/taskStore';
import { useUiStore } from '../../../app/store/uiStore';
import type { TaskDetail, TaskStatus } from '../../../types/domain';
import { errorMessage, STATUS_BADGE_VARIANT, STATUS_LABEL } from '../utils';

interface NextAction {
  label: string;
  next: TaskStatus;
  hint: string;
  icon: typeof Send;
  requireNote?: boolean;
}

function resolveAction(status: TaskStatus): NextAction | null {
  switch (status) {
    case 'ACCEPTED':
      return {
        label: '进入副本',
        next: 'IN_PROGRESS',
        hint: '启动 Agent 进入副本，副本进行中。',
        icon: PlayCircle,
      };
    case 'IN_PROGRESS':
      return {
        label: '提交战利品',
        next: 'SUBMITTED',
        hint: '提交可复现脚本、执行日志与结论报告。',
        icon: Send,
        requireNote: true,
      };
    case 'SUBMITTED':
      return {
        label: '送交裁判复核',
        next: 'AI_VALIDATING',
        hint: '平台裁判 AI 将独立复核战利品。',
        icon: ShieldCheck,
      };
    case 'AI_VALIDATING':
      return {
        label: '裁判通关，进入规则扫描',
        next: 'COMPLIANCE_SCAN',
        hint: '交叉复核一致后，继续校验授权与规则边界。',
        icon: ShieldCheck,
      };
    case 'VALIDATION_FAILED':
      return {
        label: '重新挑战',
        next: 'IN_PROGRESS',
        hint: '挑战失败，回到副本进行中补充证据。',
        icon: RotateCcw,
      };
    case 'COMPLIANCE_SCAN':
      return {
        label: '规则通过，进入掉落结算',
        next: 'SETTLING',
        hint: '进入赏金押金释放流程。',
        icon: CheckCircle2,
      };
    case 'SETTLING':
      return {
        label: '确认掉落结算',
        next: 'COMPLETED',
        hint: '释放赏金押金，副本闭环通关。',
        icon: CheckCircle2,
      };
    default:
      return null;
  }
}

export interface SubmitWorkPanelProps {
  task: TaskDetail;
  className?: string;
}

export function SubmitWorkPanel({ task, className }: SubmitWorkPanelProps) {
  const advanceStage = useTaskStore((state) => state.advanceStage);
  const pushToast = useUiStore((state) => state.pushToast);
  const [note, setNote] = useState('');
  const [advancing, setAdvancing] = useState(false);

  const action = resolveAction(task.status);

  const handleAdvance = () => {
    if (!action) {
      return;
    }
    if (action.requireNote && note.trim().length < 8) {
      pushToast({
        id: `submit-note-${task.id}`,
        title: '请填写战利品说明',
        description: '战利品说明至少 8 个字，便于战报链记录。',
        variant: 'warning',
      });
      return;
    }

    setAdvancing(true);
    try {
      advanceStage(task.id, action.next);
      pushToast({
        id: `advance-${task.id}-${action.next}`,
        title: `关卡已推进：${STATUS_LABEL[action.next]}`,
        description: action.hint,
        variant: 'success',
      });
      if (action.next === 'SUBMITTED') {
        setNote('');
      }
    } catch (error) {
      pushToast({
        id: `advance-error-${task.id}-${Date.now()}`,
        title: '关卡推进失败',
        description: errorMessage(error),
        variant: 'error',
      });
      console.error('[marketplace] advanceStage failed', error);
    } finally {
      setAdvancing(false);
    }
  };

  return (
    <Card className={className ?? 'p-4'}>
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="text-sm font-semibold text-ink">战利品与推进</h2>
            <p className="mt-0.5 text-xs text-ink-muted">
              指挥官视角：所有关卡迁移都经过状态机校验，非法迁移会被拒绝。
            </p>
          </div>
          <Badge variant={STATUS_BADGE_VARIANT[task.status]}>
            {STATUS_LABEL[task.status]}
          </Badge>
        </div>

        {task.status === 'IN_PROGRESS' && (
          <label className="block space-y-1">
            <span className="text-xs font-medium text-slate-600">
              战利品说明（写入战报链）
            </span>
            <textarea
              rows={3}
              value={note}
              onChange={(event) => setNote(event.target.value)}
              placeholder="例如：已完成 12 轮变异测试，附崩溃样本、复现脚本与执行日志。"
              className="w-full rounded-lg border border-slate-300 bg-panel px-3 py-2 text-sm text-slate-900 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
            />
          </label>
        )}

        {action ? (
          <Button
            className="w-full"
            onClick={handleAdvance}
            disabled={advancing}
          >
            <action.icon className="h-4 w-4" />
            {advancing ? '推进中…' : action.label}
          </Button>
        ) : (
          <p className="rounded-lg border border-panel-line bg-panel-soft p-3 text-xs text-ink-muted">
            {task.status === 'COMPLETED'
              ? '副本已通关，赏金押金已释放。'
              : task.status === 'DISPUTED'
                ? '副本处于仲裁中，请前往战报回放查看战斗记录并等待仲裁。'
                : '当前状态下暂无可执行的推进动作。'}
          </p>
        )}

        {action && (
          <p className="text-xs text-ink-dim">{action.hint}</p>
        )}
      </div>
    </Card>
  );
}
