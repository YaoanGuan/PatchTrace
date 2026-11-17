import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ArrowLeft, FileQuestion } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { EmptyState } from '../../../components/ui/EmptyState';
import { useSessionStore } from '../../../app/store/sessionStore';
import { useTaskStore } from '../../../app/store/taskStore';
import type { TaskStatus } from '../../../types/domain';
import {
  AcceptTaskPanel,
  AIValidationPipeline,
  AuditLinkCard,
  EscrowWallet,
  SubmitWorkPanel,
  TaskSummary,
} from '../components';
import { TaskTrustPipeline } from '../../expert/components/TaskTrustPipeline';

type PagePhase = 'loading' | 'ready' | 'missing';

const EXECUTABLE_STATUSES: TaskStatus[] = [
  'ACCEPTED',
  'IN_PROGRESS',
  'SUBMITTED',
  'AI_VALIDATING',
  'VALIDATION_FAILED',
  'COMPLIANCE_SCAN',
  'SETTLING',
  'COMPLETED',
  'DISPUTED',
];

function DetailSkeleton() {
  return (
    <div className="space-y-4">
      <div className="h-4 w-40 animate-pulse rounded bg-slate-100" />
      <div className="h-64 animate-pulse rounded-xl border border-slate-200 bg-white" />
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="h-56 animate-pulse rounded-xl border border-slate-200 bg-white" />
        <div className="h-56 animate-pulse rounded-xl border border-slate-200 bg-white" />
      </div>
    </div>
  );
}

export default function TaskDetailPage() {
  const { taskId } = useParams<{ taskId: string }>();
  const selectedTask = useTaskStore((state) => state.selectedTask);
  const fetchTask = useTaskStore((state) => state.fetchTask);
  const activeRole = useSessionStore((state) => state.activeRole);
  const [phase, setPhase] = useState<PagePhase>('loading');

  useEffect(() => {
    if (!taskId) {
      setPhase('missing');
      return undefined;
    }

    let cancelled = false;
    setPhase('loading');

    void fetchTask(taskId).then(() => {
      if (cancelled) {
        return;
      }
      const current = useTaskStore.getState().selectedTask;
      setPhase(current && current.id === taskId ? 'ready' : 'missing');
    });

    return () => {
      cancelled = true;
    };
  }, [fetchTask, taskId]);

  const task = selectedTask && selectedTask.id === taskId ? selectedTask : null;

  if (phase === 'loading') {
    return <DetailSkeleton />;
  }

  if (phase === 'missing' || !task) {
    return (
      <EmptyState
        icon={<FileQuestion className="h-8 w-8 text-slate-400" />}
        title="任务不存在或已下线"
        description={`未找到任务 ${taskId ?? ''}，可能已被取消或链接有误。`}
        action={
          <Link to="/marketplace">
            <Button variant="outline" size="sm">
              <ArrowLeft className="h-4 w-4" />
              返回任务大厅
            </Button>
          </Link>
        }
      />
    );
  }

  const canAccept = task.status === 'PUBLISHED';
  const canExecute = EXECUTABLE_STATUSES.includes(task.status);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 text-xs text-slate-500">
        <Link
          to="/marketplace"
          className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 transition-colors hover:bg-slate-100 hover:text-slate-800"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          任务大厅
        </Link>
        <span>/</span>
        <span className="font-medium text-slate-700">{task.id}</span>
        <span className="ml-auto text-slate-400">
          {activeRole === 'EMPLOYER' ? '雇主视角' : 'AI 指挥官视角'}
        </span>
      </div>

      <TaskSummary task={task} />

      <div className="grid gap-4 lg:grid-cols-2">
        <AIValidationPipeline task={task} />
        <EscrowWallet taskId={task.id} escrow={task.escrow} />
      </div>

      <TaskTrustPipeline taskId={task.id} />

      <div className="grid gap-4 lg:grid-cols-3">
        <div className="space-y-4 lg:col-span-2">
          {activeRole === 'COMMANDER' && canAccept && (
            <AcceptTaskPanel task={task} />
          )}
          {activeRole === 'COMMANDER' && canExecute && (
            <SubmitWorkPanel task={task} />
          )}
          {activeRole === 'EMPLOYER' && (
            <div className="rounded-xl border border-slate-200 bg-white p-4 text-xs leading-5 text-slate-500 shadow-sm">
              雇主视角：赏金已进入平台托管，任务验收由平台审核 AI 独立交叉复核，
              复核通过后自动进入合规扫描与结算；争议可前往审计模块查看完整证据链。
            </div>
          )}
        </div>
        <AuditLinkCard taskId={task.id} />
      </div>
    </div>
  );
}
