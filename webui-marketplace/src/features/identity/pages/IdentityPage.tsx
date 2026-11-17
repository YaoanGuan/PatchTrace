import { useEffect, useState } from 'react';
import { Upload } from 'lucide-react';
import { useSessionStore } from '../../../app/store/sessionStore';
import { RoleSwitcher } from '../../../components/shared/RoleSwitcher';
import { TrustBadgeWall } from '../../../components/shared/TrustBadgeWall';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { QuestHeading } from '../../../components/game/QuestHeading';
import { ProfileHeader } from '../components/ProfileHeader';
import { WalletPanel } from '../components/WalletPanel';
import { ApiGatewayPanel } from '../components/ApiGatewayPanel';
import { SkillLegend, SkillRadarChart } from '../components/SkillRadarChart';
import { ExpertiseRepository } from '../components/ExpertiseRepository';
import { CredentialUploadDialog } from '../components/CredentialUploadDialog';

/**
 * M1 · 冒险者圣殿。
 * 组合角色档案、金币袋、技能补给站、视角切换、技能树雷达、技能图鉴与成就徽章；
 * 数据来自 sessionStore / walletStore 与 API 层，缺字段时各子组件自行降级为空态。
 */
export default function IdentityPage() {
  const currentUser = useSessionStore((state) => state.currentUser);
  const [uploadOpen, setUploadOpen] = useState(false);

  useEffect(() => {
    if (!currentUser) {
      void useSessionStore
        .getState()
        .loadProfile()
        .catch(() => undefined);
    }
  }, [currentUser]);

  const openUpload = () => setUploadOpen(true);

  return (
    <section className="space-y-6">
      <QuestHeading
        chapter="CHARACTER HALL · M1"
        title="冒险者圣殿"
        description="等级、金币、技能树与召唤契约都在这里——用真实副本战绩堆出你的角色档案，切换雇主与 AI 指挥官身份，点亮每一枚成就徽章。"
      />

      <ProfileHeader />

      <WalletPanel />

      <ApiGatewayPanel />

      <div className="grid gap-4 lg:grid-cols-5">
        <Card className="p-5 lg:col-span-3">
          <div className="mb-4 space-y-1">
            <h2 className="text-sm font-semibold text-ink">
              技能树雷达
            </h2>
            <p className="text-xs text-ink-muted">
              五维技能评分由真实副本执行证据聚合生成
            </p>
          </div>
          <div className="grid items-center gap-4 md:grid-cols-2">
            <SkillRadarChart />
            <SkillLegend />
          </div>
        </Card>

        <Card className="p-5 lg:col-span-2">
          <div className="mb-3 space-y-1">
            <h2 className="text-sm font-semibold text-ink">出战身份</h2>
            <p className="text-xs text-ink-muted">
              同一个人可以同时是雇主与 AI 指挥官，切换身份不改变角色归属。
            </p>
          </div>
          <RoleSwitcher variant="panel" />
        </Card>
      </div>

      <Card className="p-5">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <h2 className="text-sm font-semibold text-ink">技能图鉴</h2>
            <p className="text-xs text-ink-muted">
              战友技能、战利品数据与咒语模板的技能沉淀
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={openUpload}>
            <Upload className="h-4 w-4" aria-hidden="true" />
            上传成就凭证
          </Button>
        </div>
        <ExpertiseRepository onUploadCredential={openUpload} />
      </Card>

      <Card className="p-5">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <h2 className="text-sm font-semibold text-ink">成就徽章</h2>
            <p className="text-xs text-ink-muted">
              学院试炼、公会担保与副本通关认证
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={openUpload}>
            <Upload className="h-4 w-4" aria-hidden="true" />
            上传徽章凭证
          </Button>
        </div>
        <TrustBadgeWall />
      </Card>

      <CredentialUploadDialog
        open={uploadOpen}
        onClose={() => setUploadOpen(false)}
      />
    </section>
  );
}
