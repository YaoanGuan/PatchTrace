import { useEffect, useState } from 'react';
import { Upload } from 'lucide-react';
import { useSessionStore } from '../../../app/store/sessionStore';
import { RoleSwitcher } from '../../../components/shared/RoleSwitcher';
import { TrustBadgeWall } from '../../../components/shared/TrustBadgeWall';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { ProfileHeader } from '../components/ProfileHeader';
import { SkillLegend, SkillRadarChart } from '../components/SkillRadarChart';
import { ExpertiseRepository } from '../components/ExpertiseRepository';
import { CredentialUploadDialog } from '../components/CredentialUploadDialog';

/**
 * M1 · 个人中枢与能力认证。
 * 组合个人档案、视角切换、五维能力雷达、专家库与信任凭证；
 * 数据全部来自 sessionStore.currentUser（mock API），缺字段时各子组件自行降级为空态。
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
      <header className="space-y-1">
        <p className="text-xs font-medium uppercase tracking-wide text-indigo-600">
          M1 · 个人中枢与能力认证
        </p>
        <h1 className="text-2xl font-semibold tracking-tight text-slate-900">
          个人中枢
        </h1>
        <p className="max-w-2xl text-sm text-slate-500">
          在一个地方管理身份、切换雇主 / AI 指挥官视角，展示由真实任务沉淀的
          AI 调度能力与信任凭证。
        </p>
      </header>

      <ProfileHeader />

      <div className="grid gap-4 lg:grid-cols-5">
        <Card className="p-5 lg:col-span-3">
          <div className="mb-4 space-y-1">
            <h2 className="text-sm font-semibold text-slate-900">
              AI 调度能力雷达
            </h2>
            <p className="text-xs text-slate-500">
              五维评分由真实任务执行证据聚合生成
            </p>
          </div>
          <div className="grid items-center gap-4 md:grid-cols-2">
            <SkillRadarChart />
            <SkillLegend />
          </div>
        </Card>

        <Card className="p-5 lg:col-span-2">
          <div className="mb-3 space-y-1">
            <h2 className="text-sm font-semibold text-slate-900">当前视角</h2>
            <p className="text-xs text-slate-500">
              同一个人可以同时是雇主与 AI 指挥官，切换视角不改变账号身份。
            </p>
          </div>
          <RoleSwitcher variant="panel" />
        </Card>
      </div>

      <Card className="p-5">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <h2 className="text-sm font-semibold text-slate-900">专家库</h2>
            <p className="text-xs text-slate-500">
              Agent 能力、数据资产与提示词模板的能力沉淀
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={openUpload}>
            <Upload className="h-4 w-4" aria-hidden="true" />
            上传认证凭证
          </Button>
        </div>
        <ExpertiseRepository onUploadCredential={openUpload} />
      </Card>

      <Card className="p-5">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <h2 className="text-sm font-semibold text-slate-900">信任凭证</h2>
            <p className="text-xs text-slate-500">
              高校学生、企业白名单与任务通关认证
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={openUpload}>
            <Upload className="h-4 w-4" aria-hidden="true" />
            上传凭证
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
