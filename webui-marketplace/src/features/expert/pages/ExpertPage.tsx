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
import { QuestHeading } from '../../../components/game/QuestHeading';
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
        <div className="h-5 w-28 animate-pulse rounded bg-panel-soft" />
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 6 }).map((_, index) => (
            <Card key={index} className="h-40 animate-pulse bg-panel-soft" />
          ))}
        </div>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <Card className="h-48 animate-pulse bg-panel-soft" />
        <Card className="h-48 animate-pulse bg-panel-soft" />
      </div>
    </div>
  );
}

/**
 * M2 · 专家祭坛（信任收费站 / 副本裁判所）。
 * 信任服务是产品本身：战友召唤图鉴 → 雇主公会/指挥官战队 → 双人连携验证 →
 * 副本规则扫描 → 大Boss 威胁关卡封锁。overview 拉取失败时保留 Hero 与静态
 * 角色说明，并给出重试入口，不白屏。
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
          <QuestHeading
            chapter="EXPERT ALTAR · M2"
            title="信任收费站 · 副本裁判所"
            description="纯 API 代理无定价权——模型调用谁都能转售；信任才是产品本身。我们把交付前的信任做成可计价的副本裁判服务：多 Agent 双人连携验证 + 自动副本规则扫描 + 大Boss 威胁关卡封锁，任一环节未通关，任务不进入结算。"
          />
          <div className="flex flex-wrap gap-1.5">
            <span className="inline-flex items-center gap-1 rounded-full bg-arcane/10 px-2.5 py-1 text-[11px] font-medium text-arcane ring-1 ring-inset ring-arcane/30">
              <ShieldCheck className="h-3 w-3" />
              双人连携验证
            </span>
            <span className="inline-flex items-center gap-1 rounded-full bg-neon/10 px-2.5 py-1 text-[11px] font-medium text-neon ring-1 ring-inset ring-neon/30">
              <ScanLine className="h-3 w-3" />
              副本规则扫描
            </span>
            <span className="inline-flex items-center gap-1 rounded-full bg-boss/10 px-2.5 py-1 text-[11px] font-medium text-boss ring-1 ring-inset ring-boss/30">
              <ShieldAlert className="h-3 w-3" />
              大Boss 威胁封锁
            </span>
            <span className="inline-flex items-center gap-1 rounded-full bg-panel-soft px-2.5 py-1 text-[11px] font-medium text-ink-muted ring-1 ring-inset ring-panel-line">
              <Sparkles className="h-3 w-3" />
              按副本计费 · 不含上游 API 成本
            </span>
          </div>
        </div>

        <TrustPricing pricing={overview?.pricing} />
      </section>

      {phase === 'error' && (
        <Card className="flex flex-wrap items-center justify-between gap-3 border-boss/30 bg-boss/10 p-4">
          <div>
            <h2 className="text-sm font-semibold text-boss">
              祭坛数据暂不可用
            </h2>
            <p className="mt-0.5 text-xs text-ink-muted">
              战友召唤图鉴、双人连携验证与大Boss 威胁数据加载失败。下方公会接入说明仍可查阅，可稍后重试。
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
