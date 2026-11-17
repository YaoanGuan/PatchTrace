import { useCallback, useEffect, useState } from 'react';
import {
  RefreshCw,
  ScanLine,
  ShieldAlert,
  ShieldCheck,
  Sparkles,
} from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { getExpertOverview } from '../../../lib/api/client';
import type { ExpertOverview } from '../../../types/domain';
import { AccessMatrix } from '../components/AccessMatrix';
import { ComplianceScanPanel } from '../components/ComplianceScanPanel';
import { CrossValidationPanel } from '../components/CrossValidationPanel';
import { RiskGauge } from '../components/RiskGauge';
import { RoleBridge } from '../components/RoleBridge';
import { TrustPricing } from '../components/TrustPricing';

type Phase = 'loading' | 'ready' | 'error';

function OverviewSkeleton() {
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <div className="h-5 w-28 animate-pulse rounded bg-slate-200/70" />
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 6 }).map((_, index) => (
            <Card key={index} className="h-40 animate-pulse bg-slate-50" />
          ))}
        </div>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <Card className="h-48 animate-pulse bg-slate-50" />
        <Card className="h-48 animate-pulse bg-slate-50" />
      </div>
    </div>
  );
}

/**
 * M2 · AI 专家（信任收费站 / Trust Layer）。
 * 信任服务是产品本身：接入矩阵 → 角色桥 → 交叉验证 → 合规扫描 → 风险阻断。
 * overview 拉取失败时保留 Hero 与静态角色桥，并给出重试入口，不白屏。
 */
export default function ExpertPage() {
  const [overview, setOverview] = useState<ExpertOverview | null>(null);
  const [phase, setPhase] = useState<Phase>('loading');

  const load = useCallback(async () => {
    setPhase('loading');
    try {
      const data = await getExpertOverview();
      setOverview(data);
      setPhase('ready');
    } catch {
      setOverview(null);
      setPhase('error');
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-6">
      <section className="grid gap-4 lg:grid-cols-3">
        <div className="space-y-3 lg:col-span-2">
          <p className="text-xs font-medium uppercase tracking-wide text-indigo-600">
            M2 · AI 专家
          </p>
          <h1 className="text-3xl font-semibold tracking-tight text-slate-900">
            信任收费站
          </h1>
          <p className="max-w-2xl text-sm leading-6 text-slate-600">
            纯 API 代理无定价权——模型调用谁都能转售。我们把交付前的信任做成可计价产品：
            <span className="font-medium text-slate-800">
              多 Agent 交叉验证 + 自动化合规扫描 + 风险阻断
            </span>
            ，任一项不通过，任务不进结算。
          </p>
          <div className="flex flex-wrap gap-1.5">
            <span className="inline-flex items-center gap-1 rounded-full bg-indigo-50 px-2.5 py-1 text-[11px] font-medium text-indigo-700 ring-1 ring-inset ring-indigo-600/20">
              <ShieldCheck className="h-3 w-3" />
              交叉验证
            </span>
            <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-1 text-[11px] font-medium text-emerald-700 ring-1 ring-inset ring-emerald-600/20">
              <ScanLine className="h-3 w-3" />
              合规扫描
            </span>
            <span className="inline-flex items-center gap-1 rounded-full bg-rose-50 px-2.5 py-1 text-[11px] font-medium text-rose-700 ring-1 ring-inset ring-rose-600/20">
              <ShieldAlert className="h-3 w-3" />
              风险阻断
            </span>
            <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2.5 py-1 text-[11px] font-medium text-slate-600">
              <Sparkles className="h-3 w-3" />
              按任务计费 · 不含上游 API 成本
            </span>
          </div>
        </div>

        <TrustPricing pricing={overview?.pricing} />
      </section>

      {phase === 'error' && (
        <Card className="flex flex-wrap items-center justify-between gap-3 border-rose-200 bg-rose-50/60 p-4">
          <div>
            <h2 className="text-sm font-semibold text-rose-800">
              专家数据暂不可用
            </h2>
            <p className="mt-0.5 text-xs text-rose-700">
              接入矩阵、交叉验证与风险数据加载失败。下方角色接入说明仍可查阅，可稍后重试。
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={() => void load()}>
            <RefreshCw className="h-3.5 w-3.5" />
            重试
          </Button>
        </Card>
      )}

      {phase === 'loading' && !overview ? (
        <OverviewSkeleton />
      ) : (
        <>
          {overview && (
            <AccessMatrix tools={overview.tools} />
          )}

          <RoleBridge />

          {overview && (
            <>
              <CrossValidationPanel validation={overview.validation} />

              <div className="grid gap-4 lg:grid-cols-5">
                <ComplianceScanPanel
                  findings={overview.compliance}
                  className="lg:col-span-3"
                />
                <RiskGauge risk={overview.risk} className="lg:col-span-2" />
              </div>
            </>
          )}
        </>
      )}
    </div>
  );
}
