import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  AlertTriangle,
  ArrowLeft,
  GitBranch,
  RotateCw,
  ShieldCheck,
} from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { useSessionStore } from '../../../app/store/sessionStore';
import { getTrace } from '../../../lib/api/client';
import type { GetTraceResponse } from '../../../types/api';
import { ChainStateBadge } from '../components/ChainStateBadge';
import { ComplianceScanReport } from '../components/ComplianceScanReport';
import { HashChainStatus } from '../components/HashChainStatus';
import { HashText } from '../components/HashText';
import { LegalLiabilityModal } from '../components/LegalLiabilityModal';
import { TraceabilityTimeline } from '../components/TraceabilityTimeline';
import { verifyTrace } from '../utils';

export function AuditDetailPage() {
  const { taskId } = useParams<{ taskId: string }>();
  const navigate = useNavigate();
  const currentUser = useSessionStore((state) => state.currentUser);
  const activeRole = useSessionStore((state) => state.activeRole);

  const [trace, setTrace] = useState<GetTraceResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [legalModalOpen, setLegalModalOpen] = useState(false);

  const load = useCallback(async () => {
    if (!taskId) {
      setTrace(null);
      setError('缺少副本标识，无法加载战报。');
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const response = await getTrace(taskId);
      setTrace(response);
    } catch (caught) {
      setTrace(null);
      setError(
        caught instanceof Error ? caught.message : '战报加载失败，请重试',
      );
    } finally {
      setLoading(false);
    }
  }, [taskId]);

  useEffect(() => {
    void load();
  }, [load]);

  const nodes = trace?.nodes ?? [];
  const verification = useMemo(() => verifyTrace(trace), [trace]);
  const hasTrace = nodes.length > 0;

  if (loading) {
    return (
      <section className="space-y-6" aria-busy="true">
        <div className="space-y-3">
          <div className="h-3 w-24 animate-pulse rounded bg-slate-200" />
          <div className="h-8 w-80 max-w-full animate-pulse rounded bg-slate-200" />
          <div className="h-4 w-96 max-w-full animate-pulse rounded bg-slate-100" />
        </div>
        <div className="grid gap-6 lg:grid-cols-3">
          <div className="h-96 animate-pulse rounded-xl border border-slate-200 bg-slate-100 lg:col-span-2" />
          <div className="space-y-6">
            <div className="h-56 animate-pulse rounded-xl border border-slate-200 bg-slate-100" />
            <div className="h-56 animate-pulse rounded-xl border border-slate-200 bg-slate-100" />
          </div>
        </div>
      </section>
    );
  }

  if (error) {
    return (
      <section className="space-y-6">
        <Button variant="ghost" size="sm" onClick={() => navigate('/audit')}>
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          返回战报档案
        </Button>
        <Card className="flex flex-col items-center gap-3 border-boss/30 bg-boss/10 px-6 py-12 text-center">
          <AlertTriangle className="h-8 w-8 text-boss" aria-hidden="true" />
          <div className="space-y-1">
            <h1 className="text-base font-semibold text-boss">
              战报加载失败
            </h1>
            <p className="text-xs text-boss/80">{error}</p>
            {taskId && (
              <p className="font-mono text-[11px] text-boss/70">
                taskId: {taskId}
              </p>
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={() => void load()}>
              <RotateCw className="h-3.5 w-3.5" aria-hidden="true" />
              重新加载
            </Button>
            <Button variant="secondary" size="sm" onClick={() => navigate('/audit')}>
              返回列表
            </Button>
          </div>
        </Card>
      </section>
    );
  }

  return (
    <section className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-2">
          <Button
            variant="ghost"
            size="sm"
            className="-ml-2"
            onClick={() => navigate('/audit')}
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            返回战报档案
          </Button>
          <p className="text-xs font-medium uppercase tracking-wide text-neon">
            M4 · 单副本战报 + 战报契约
          </p>
          <h1 className="text-2xl font-semibold tracking-tight text-ink">
            战报回放 · {taskId}
          </h1>
          <div className="flex flex-wrap items-center gap-3 text-xs text-ink-muted">
            <ChainStateBadge state={verification.state} />
            <span className="inline-flex items-center gap-1">
              <GitBranch className="h-3.5 w-3.5 text-ink-dim" aria-hidden="true" />
              {nodes.length} 个战报节点
            </span>
            {verification.headHash && (
              <span className="inline-flex items-center gap-1">
                封印链头
                <HashText hash={verification.headHash} />
              </span>
            )}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" onClick={() => navigate('/audit')}>
            返回战报档案
          </Button>
          <Button
            onClick={() => setLegalModalOpen(true)}
            disabled={!hasTrace}
            title={
              hasTrace
                ? '打开三方责任边界与签名面板'
                : '该副本暂无战报，无法签署战报契约'
            }
          >
            <ShieldCheck className="h-4 w-4" aria-hidden="true" />
            签署战报契约
          </Button>
        </div>
      </div>

      {!hasTrace && (
        <div className="flex items-start gap-2 rounded-lg border border-boss/30 bg-boss/10 px-3 py-2.5 text-xs text-boss">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span>
            该副本暂无战报节点。战斗回放时间线与封印链校验将降级展示，战报契约签署入口保持禁用。
          </span>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="space-y-4 p-5 lg:col-span-2">
          <div className="space-y-1">
            <h2 className="text-sm font-semibold text-ink">
              战斗回放时间线
            </h2>
            <p className="text-xs text-ink-muted">
              按需求、验收、模型调用、执行动作、产物输出、复核验证六类战报节点还原执行全过程。
            </p>
          </div>
          <TraceabilityTimeline nodes={nodes} verification={verification} />
        </Card>

        <div className="space-y-6">
          <HashChainStatus trace={trace} verification={verification} />
          <ComplianceScanReport trace={trace} verification={verification} />
        </div>
      </div>

      <LegalLiabilityModal
        open={legalModalOpen}
        taskId={taskId ?? ''}
        hasTrace={hasTrace}
        defaultSignerRole={activeRole}
        signerName={currentUser?.displayName ?? '当前用户'}
        onClose={() => setLegalModalOpen(false)}
      />
    </section>
  );
}

export default AuditDetailPage;
