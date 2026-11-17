import { useCallback, useEffect, useState } from 'react';
import { RefreshCw, ShieldAlert, ShieldCheck } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import { getTaskTrust } from '../../../lib/api/client';
import type { TaskTrust } from '../../../types/domain';
import { PipelineStatus } from './PipelineStatus';
import { RiskGauge } from './RiskGauge';

export interface TaskTrustPipelineProps {
  taskId: string;
  className?: string;
}

type Phase = 'loading' | 'ready' | 'error';

function TrustSkeleton({ className }: { className?: string }) {
  return (
    <Card className={cn('space-y-3 p-4', className)}>
      <div className="flex items-center justify-between">
        <h2 className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
          <ShieldCheck className="h-4 w-4 text-indigo-600" />
          信任流水线
        </h2>
        <span className="text-[11px] text-slate-400">加载中…</span>
      </div>
      <div className="h-5 w-32 animate-pulse rounded bg-slate-100" />
      <div className="flex gap-2">
        <div className="h-20 flex-1 animate-pulse rounded-lg bg-slate-100" />
        <div className="h-20 flex-1 animate-pulse rounded-lg bg-slate-100" />
        <div className="h-20 flex-1 animate-pulse rounded-lg bg-slate-100" />
      </div>
      <div className="h-16 animate-pulse rounded-lg bg-slate-50" />
    </Card>
  );
}

/**
 * 任务详情页复用的信任流水线卡。
 * 挂载时请求 GET /api/tasks/:id/trust；加载中显示骨架，失败显示降级说明与重试，
 * 不会抛出未捕获错误，也不阻塞任务详情页其他内容。
 */
export function TaskTrustPipeline({
  taskId,
  className,
}: TaskTrustPipelineProps) {
  const [trust, setTrust] = useState<TaskTrust | null>(null);
  const [phase, setPhase] = useState<Phase>('loading');
  const [reloadToken, setReloadToken] = useState(0);

  useEffect(() => {
    if (!taskId) {
      setTrust(null);
      setPhase('error');
      return undefined;
    }

    let cancelled = false;
    setPhase('loading');

    getTaskTrust(taskId)
      .then((data) => {
        if (cancelled) {
          return;
        }
        setTrust(data);
        setPhase('ready');
      })
      .catch(() => {
        if (cancelled) {
          return;
        }
        setTrust(null);
        setPhase('error');
      });

    return () => {
      cancelled = true;
    };
  }, [taskId, reloadToken]);

  const retry = useCallback(() => {
    setReloadToken((token) => token + 1);
  }, []);

  if (phase === 'loading') {
    return <TrustSkeleton className={className} />;
  }

  if (phase === 'error' || !trust) {
    return (
      <Card className={cn('p-4', className)}>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
              <ShieldAlert className="h-4 w-4 text-amber-500" />
              信任流水线
            </h2>
            <p className="mt-1 text-xs text-slate-500">信任数据暂不可用</p>
            <p className="mt-0.5 text-[11px] text-slate-400">
              任务主流程不受影响，可稍后重试获取交叉验证、合规扫描与风险定级。
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={retry}>
            <RefreshCw className="h-3.5 w-3.5" />
            重试
          </Button>
        </div>
      </Card>
    );
  }

  const findings = trust.compliance ?? [];
  const hitCount = findings.filter((item) => item.status !== 'PASS').length;
  const blockCount = findings.filter((item) => item.status === 'BLOCK').length;

  return (
    <Card className={cn('p-4', className)}>
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
            <ShieldCheck className="h-4 w-4 text-indigo-600" />
            信任流水线
          </h2>
          <p className="mt-0.5 text-xs text-slate-500">
            交叉验证 → 合规扫描 → 风险阻断，全部通过才允许结算。
          </p>
        </div>
        <span className="rounded-full bg-slate-100 px-2 py-0.5 font-mono text-[10px] text-slate-500">
          {trust.taskId || taskId}
        </span>
      </div>

      <div className="mt-3">
        <PipelineStatus validation={trust.validation} compact />
      </div>

      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <RiskGauge
          risk={trust.risk}
          compact
          className={
            trust.risk ? 'border-0 bg-transparent p-0 shadow-none' : undefined
          }
        />
        <div className="space-y-2">
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
            <p className="text-[11px] text-slate-500">合规命中</p>
            <p className="mt-0.5 text-2xl font-bold tabular-nums text-slate-900">
              {hitCount}
              <span className="ml-1 text-xs font-normal text-slate-400">
                条 / 共 {findings.length} 条规则
              </span>
            </p>
            <p className="mt-1 text-[11px] text-slate-500">
              {blockCount > 0
                ? `其中 ${blockCount} 条 BLOCK 级规则，任务已阻断。`
                : '当前没有 BLOCK 级命中。'}
            </p>
          </div>
          <div className="rounded-lg bg-slate-50 px-3 py-2 text-[11px] leading-5 text-slate-500">
            信任数据仅用于本任务结算；证据链与判定结果可在审计模块追溯。
          </div>
        </div>
      </div>
    </Card>
  );
}
