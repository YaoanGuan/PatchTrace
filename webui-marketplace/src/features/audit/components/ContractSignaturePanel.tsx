import { CheckCircle2, Loader2, PenLine, UserRoundCheck } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { Tabs } from '../../../components/ui/Tabs';
import type { SignContractResponse } from '../../../types/api';
import type { UserRole } from '../../../types/domain';
import { formatDateTime } from '../utils';

export interface ContractSignaturePanelProps {
  signerRole: UserRole;
  signerName: string;
  acceptedCount: number;
  totalClauses: number;
  allAccepted: boolean;
  disabled: boolean;
  disabledReason?: string;
  signing: boolean;
  signed: SignContractResponse | null;
  onSignerRoleChange(role: UserRole): void;
  onSign(): void;
}

const ROLE_LABEL: Record<UserRole, string> = {
  EMPLOYER: '雇主',
  COMMANDER: 'AI 指挥官',
};

export function ContractSignaturePanel({
  signerRole,
  signerName,
  acceptedCount,
  totalClauses,
  allAccepted,
  disabled,
  disabledReason,
  signing,
  signed,
  onSignerRoleChange,
  onSign,
}: ContractSignaturePanelProps) {
  const signDisabled = disabled || !allAccepted || signing || signed !== null;

  return (
    <div className="space-y-3 rounded-lg border border-slate-200 bg-panel px-3 py-3">
      <div className="flex items-center gap-2">
        <UserRoundCheck className="h-4 w-4 text-indigo-500" aria-hidden="true" />
        <h3 className="text-sm font-semibold text-ink">签署信息</h3>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <span className="text-xs text-slate-500">签署角色</span>
        <Tabs
          items={[
            { key: 'EMPLOYER', label: ROLE_LABEL.EMPLOYER },
            { key: 'COMMANDER', label: ROLE_LABEL.COMMANDER },
          ]}
          value={signerRole}
          onChange={(key) => onSignerRoleChange(key as UserRole)}
        />
      </div>

      <div className="grid gap-2 sm:grid-cols-2">
        <div className="rounded-md bg-slate-50 px-3 py-2">
          <p className="text-[11px] font-medium text-slate-400">签署人</p>
          <p className="mt-0.5 text-xs text-slate-700">{signerName}</p>
        </div>
        <div className="rounded-md bg-slate-50 px-3 py-2">
          <p className="text-[11px] font-medium text-slate-400">条款确认</p>
          <p className="mt-0.5 text-xs text-slate-700">
            {acceptedCount}/{totalClauses} 条已勾选
          </p>
        </div>
      </div>

      {disabled && disabledReason && (
        <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] text-amber-700">
          {disabledReason}
        </p>
      )}

      {!disabled && !allAccepted && (
        <p className="text-[11px] text-amber-600">
          请先勾选全部责任边界条款，签署按钮才会解锁。
        </p>
      )}

      {signed && (
        <div className="flex items-start gap-2 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-700">
          <CheckCircle2
            className="mt-0.5 h-4 w-4 shrink-0"
            aria-hidden="true"
          />
          <span>
            战报契约已签署（演示）· {formatDateTime(signed.signedAt)}
          </span>
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-slate-100 pt-3">
        <p className="text-[11px] text-slate-400">
          演示环境：不产生真实法律效力，签名哈希见下方回执。
        </p>
        <Button
          onClick={onSign}
          disabled={signDisabled}
          title={
            disabled
              ? disabledReason
              : !allAccepted
                ? '请先勾选全部条款'
                : undefined
          }
        >
          {signing ? (
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
          ) : signed ? (
            <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
          ) : (
            <PenLine className="h-4 w-4" aria-hidden="true" />
          )}
          {signing ? '签署中…' : signed ? '已签署' : '勾选完成，生成签名'}
        </Button>
      </div>
    </div>
  );
}
