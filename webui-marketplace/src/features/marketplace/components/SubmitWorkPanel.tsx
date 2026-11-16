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
        label: '开始执行',
        next: 'IN_PROGRESS',
        hint: '启动 Agent 执行任务，进入执行中状态。',
        icon: PlayCircle,
      };
    case 'IN_PROGRESS':
      return {
        label: '提交交付',
        next: 'SUBMITTED',
        hint: '提交可复现脚本、执行日志与结论报告。',
        icon: Send,
        requireNote: true,
      };
    case 'SUBMITTED':
      return {
        label: '送交平台 AI 复核',
        next: 'AI_VALIDATING',
        hint: '平台审核 AI 将独立交叉复核交付物。',
        icon: ShieldCheck,
      };
    case 'AI_VALIDATING':
      return {
        label: '复核通过，进入合规扫描',
        next: 'COMPLIANCE_SCAN',
        hint: '交叉复核一致后，继续校验授权与合规边界。',
        icon: ShieldCheck,
      };
    case 'VALIDATION_FAILED':
      return {
        label: '返工重做',
        next: 'IN_PROGRESS',
        hint: '平台复核未通过，回到执行中补充证据。',
        icon: RotateCcw,
      };
    case 'COMPLIANCE_SCAN':
      return {
        label: '合规通过，进入结算',
        next: 'SETTLING',
        hint: '进入托管资金释放流程。',
        icon: CheckCircle2,
      };
    case 'SETTLING':
      return {
        label: '确认结算完成',
        next: 'COMPLETED',
        hint: '释放托管赏金，任务闭环完成。',
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
        title: '请填写交付说明',
        description: '交付说明至少 8 个字，便于审计链记录。',
        variant: 'warning',
      });
      return;
    }

    setAdvancing(true);
    try {
      advanceStage(task.id, action.next);
      pushToast({
        id: `advance-${task.id}-${action.next}`,
        title: `状态已推进：${STATUS_LABEL[action.next]}`,
        description: action.hint,
        variant: 'success',
      });
      if (action.next === 'SUBMITTED') {
        setNote('');
      }
    } catch (error) {
      pushToast({
        id: `advance-error-${task.id}-${Date.now()}`,
        title: '状态推进失败',
        description: errorMessage(error),
        variant: 'error',
      });
      console.error('[marketplace] advanceStage failed', error);
    } finally {
      setAdvancing(false);
    }
  };

  return (
    <div
      className={
        className ??
        'rounded-xl border border-slate-200 bg-white p-4 shadow-sm'
      }
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="text-sm font-semibold text-slate-900">交付与推进</h2>
            <p className="mt-0.5 text-xs text-slate-500">
              指挥官视角：所有状态迁移都经过状态机校验，非法迁移会被拒绝。
            </p>
          </div>
          <Badge variant={STATUS_BADGE_VARIANT[task.status]}>
            {STATUS_LABEL[task.status]}
          </Badge>
        </div>

        {task.status === 'IN_PROGRESS' && (
          <label className="block space-y-1">
            <span className="text-xs font-medium text-slate-600">
              交付说明（写入审计链）
            </span>
            <textarea
              rows={3}
              value={note}
              onChange={(event) => setNote(event.target.value)}
              placeholder="例如：已完成 12 轮变异测试，附崩溃样本、复现脚本与执行日志。"
              className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
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
          <p className="rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs text-slate-500">
            {task.status === 'COMPLETED'
              ? '任务已完成，托管赏金已释放。'
              : task.status === 'DISPUTED'
                ? '任务处于争议中，请前往审计模块查看证据链并等待仲裁。'
                : '当前状态下暂无可执行的推进动作。'}
          </p>
        )}

        {action && (
          <p className="text-xs text-slate-400">{action.hint}</p>
        )}
      </div>
    </div>
  );
}
