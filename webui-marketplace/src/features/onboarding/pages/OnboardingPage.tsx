import { useState } from 'react';
import { QuestHeading } from '../../../components/game/QuestHeading';
import { TrustBadgeWall } from '../../../components/shared/TrustBadgeWall';
import { Badge } from '../../../components/ui/Badge';
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
      <QuestHeading
        chapter="STARTER VILLAGE · M5"
        title="新手村与转职试炼"
        description="从学院试炼、公会担保与游侠自荐三条通道建立初始信任，再用雇主与 AI 指挥官的声望互评让声望持续积累。"
        badge={
          <Badge
            variant="outline"
            className="font-mono tracking-widest !text-gold"
          >
            TRANSFER TRIAL
          </Badge>
        }
      />

      <RegistrationStatusBanner status={registration} />

      <MultiChannelRegistration onRegistered={setRegistration} />

      <section className="space-y-3">
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-ink">成就徽章墙</h2>
          <p className="text-sm text-ink-muted">
            学院试炼、公会担保与副本通关认证会沉淀为可验证的成就凭证。
          </p>
        </div>
        <TrustBadgeWall />
      </section>

      <section className="space-y-3">
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-ink">声望互评</h2>
          <p className="text-sm text-ink-muted">
            雇主与 AI 指挥官互评，战绩评分、维度与战报历史共同构成后续撮合的信任依据。
          </p>
        </div>
        <ReputationSystem />
      </section>
    </section>
  );
}
