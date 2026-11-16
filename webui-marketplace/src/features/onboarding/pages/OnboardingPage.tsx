import { useState } from 'react';
import { TrustBadgeWall } from '../../../components/shared/TrustBadgeWall';
import { MultiChannelRegistration } from '../components/MultiChannelRegistration';
import { RegistrationStatusBanner } from '../components/RegistrationStatusBanner';
import { ReputationSystem } from '../components/ReputationSystem';
import type { RegistrationStatus } from '../types';

export default function OnboardingPage() {
  const [registration, setRegistration] = useState<RegistrationStatus | null>(
    null,
  );

  return (
    <section className="space-y-8">
      <header className="space-y-2">
        <p className="text-xs font-medium uppercase tracking-wide text-indigo-600">
          M4 · 准入机制与生态信誉
        </p>
        <h1 className="text-2xl font-semibold tracking-tight text-slate-900">
          准入与信誉
        </h1>
        <p className="max-w-3xl text-sm text-slate-500">
          从高校学生、企业白名单与自由职业者三通道建立初始信任，
          再用雇主与 AI 指挥官的双向评分让信誉可持续积累。
        </p>
      </header>

      <RegistrationStatusBanner status={registration} />

      <MultiChannelRegistration onRegistered={setRegistration} />

      <section className="space-y-3">
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-slate-900">信任徽章墙</h2>
          <p className="text-sm text-slate-500">
            高校、企业与任务通关认证会沉淀为可验证的信任凭证。
          </p>
        </div>
        <TrustBadgeWall />
      </section>

      <section className="space-y-3">
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-slate-900">双向信誉体系</h2>
          <p className="text-sm text-slate-500">
            雇主与 AI 指挥官互评，评分、维度与历史记录共同构成后续撮合的信任依据。
          </p>
        </div>
        <ReputationSystem />
      </section>
    </section>
  );
}
