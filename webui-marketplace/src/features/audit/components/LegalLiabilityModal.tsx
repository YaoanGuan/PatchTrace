import { useEffect, useState } from 'react';
import { AlertTriangle, ShieldCheck } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { Modal } from '../../../components/ui/Modal';
import { useUiStore } from '../../../app/store/uiStore';
import { signContract } from '../../../lib/api/client';
import type { SignContractResponse } from '../../../types/api';
import type { UserRole } from '../../../types/domain';
import type { LiabilityClause } from '../types';
import { demoSignatureHash } from '../utils';
import { ContractSignaturePanel } from './ContractSignaturePanel';
import { LiabilityChecklist } from './LiabilityChecklist';
import { SignatureHash } from './SignatureHash';

export const LIABILITY_CLAUSES: LiabilityClause[] = [
  {
    id: 'clause_employer_scope',
    party: 'EMPLOYER',
    title: '任务目标与授权范围',
    detail:
      '雇主确认任务目标合法、资产授权完整，并对测试范围、禁止事项与验收标准承担最终责任。',
  },
  {
    id: 'clause_employer_dispute',
    party: 'EMPLOYER',
    title: '验收与争议依据',
    detail:
      '雇主同意以本页哈希链证据与平台仲裁结论作为验收与争议责任划分依据，不得要求超出授权范围的操作。',
  },
  {
    id: 'clause_commander_evidence',
    party: 'COMMANDER',
    title: '执行过程与证据真实性',
    detail:
      'AI 指挥官对 Agent 编排、工具调用与产出证据的真实性负责，承诺全程留存可审计的证据链节点。',
  },
  {
    id: 'clause_commander_approval',
    party: 'COMMANDER',
    title: '风险动作审批',
    detail:
      '执行过程如触发高风险动作，指挥官须先获得平台或雇主授权，不得绕过审批直接执行。',
  },
  {
    id: 'clause_platform_scope',
    party: 'PLATFORM',
    title: '平台角色边界',
    detail:
      '平台仅提供任务封装、撮合、资金托管、哈希链审计与争议仲裁流程，不参与任务执行，不对执行结果作实质背书。',
  },
  {
    id: 'clause_platform_audit',
    party: 'PLATFORM',
    title: '审计与仲裁职责',
    detail:
      '平台负责维护证据链完整性、校验签名回执并组织争议仲裁；平台不对不可归因于平台的第三方行为承担责任。',
  },
];

const PARTY_ROLE_LABEL: Record<'EMPLOYER' | 'COMMANDER', string> = {
  EMPLOYER: '雇主',
  COMMANDER: 'AI 指挥官',
};

export interface LegalLiabilityModalProps {
  open: boolean;
  taskId: string;
  hasTrace: boolean;
  defaultSignerRole: UserRole;
  signerName: string;
  onClose(): void;
}

export function LegalLiabilityModal({
  open,
  taskId,
  hasTrace,
  defaultSignerRole,
  signerName,
  onClose,
}: LegalLiabilityModalProps) {
  const pushToast = useUiStore((state) => state.pushToast);
  const [accepted, setAccepted] = useState<boolean[]>(() =>
    LIABILITY_CLAUSES.map(() => false),
  );
  const [signerRole, setSignerRole] = useState<UserRole>(defaultSignerRole);
  const [signed, setSigned] = useState<SignContractResponse | null>(null);
  const [signing, setSigning] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    setAccepted(LIABILITY_CLAUSES.map(() => false));
    setSignerRole(defaultSignerRole);
    setSigned(null);
    setSigning(false);
    setError(null);
    // 仅在弹窗打开或任务切换时重置；签署角色变化不应清空当前勾选。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, taskId]);

  const acceptedCount = accepted.filter(Boolean).length;
  const allAccepted = acceptedCount === LIABILITY_CLAUSES.length;

  const handleToggle = (index: number) => {
    if (signing || signed) {
      return;
    }
    setAccepted((previous) =>
      previous.map((value, current) => (current === index ? !value : value)),
    );
  };

  const handleSign = async () => {
    if (!hasTrace) {
      setError('该任务暂无证据链节点，无法签署免责契约。');
      return;
    }
    if (!allAccepted || signing || signed) {
      return;
    }

    setSigning(true);
    setError(null);
    try {
      const response = await signContract({
        taskId,
        signerRole,
        clausesAccepted: accepted,
      });
      // 演示环境：使用确定性本地哈希，保证同一任务 / 角色 / 勾选组合可复现；
      // 真实接入时应直接展示后端签名回执哈希。
      const demoHash = demoSignatureHash(taskId, signerRole, accepted);
      setSigned({ ...response, signatureHash: demoHash });
      pushToast({
        id: `contract-sign-${taskId}-${Date.now()}`,
        title: '免责契约已签署',
        description: '签名哈希已生成（演示环境，不具备真实法律效力）。',
        variant: 'success',
      });
    } catch (caught) {
      const message =
        caught instanceof Error ? caught.message : '签署失败，请稍后重试';
      setError(message);
      pushToast({
        id: `contract-error-${taskId}-${Date.now()}`,
        title: '签署失败',
        description: message,
        variant: 'error',
      });
    } finally {
      setSigning(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="lg"
      title="免责与三方责任边界契约"
      description={`任务 ${taskId} · 雇主 / AI 指挥官 / 平台`}
      footer={
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span className="text-[11px] text-slate-400">
            演示契约：签名哈希由本地确定性算法生成，不构成真实电子签名。
          </span>
          <Button variant="secondary" size="sm" onClick={onClose}>
            关闭
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        {!hasTrace && (
          <div className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5 text-xs text-amber-700">
            <AlertTriangle
              className="mt-0.5 h-4 w-4 shrink-0"
              aria-hidden="true"
            />
            <span>
              该任务暂无证据链节点，签署入口已禁用。请先产生可审计执行记录后再签署免责契约。
            </span>
          </div>
        )}

        {error && (
          <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-700">
            {error}
          </div>
        )}

        <LiabilityChecklist
          clauses={LIABILITY_CLAUSES}
          accepted={accepted}
          disabled={signing || signed !== null}
          onToggle={handleToggle}
        />

        <ContractSignaturePanel
          signerRole={signerRole}
          signerName={signerName}
          acceptedCount={acceptedCount}
          totalClauses={LIABILITY_CLAUSES.length}
          allAccepted={allAccepted}
          disabled={!hasTrace}
          disabledReason="暂无证据链，无法生成可验证的签名回执。"
          signing={signing}
          signed={signed}
          onSignerRoleChange={setSignerRole}
          onSign={() => {
            void handleSign();
          }}
        />

        {signed && (
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs font-medium text-emerald-700">
              <ShieldCheck className="h-4 w-4" aria-hidden="true" />
              契约签署完成，以下为签名回执
            </div>
            <SignatureHash
              hash={signed.signatureHash}
              signedAt={signed.signedAt}
              signerRoleLabel={
                PARTY_ROLE_LABEL[signed.signerRole as 'EMPLOYER' | 'COMMANDER']
              }
            />
          </div>
        )}
      </div>
    </Modal>
  );
}
