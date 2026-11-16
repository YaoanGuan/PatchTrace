import { Fingerprint, ShieldAlert } from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { formatDateTime } from '../utils';
import { HashText } from './HashText';

export interface SignatureHashProps {
  hash: string;
  signedAt?: string;
  signerRoleLabel?: string;
}

export function SignatureHash({
  hash,
  signedAt,
  signerRoleLabel,
}: SignatureHashProps) {
  return (
    <div className="rounded-lg border border-indigo-100 bg-indigo-50/50 px-3 py-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-indigo-700">
          <Fingerprint className="h-4 w-4" aria-hidden="true" />
          签名哈希
        </span>
        <Badge variant="warning">
          <ShieldAlert className="h-3 w-3" aria-hidden="true" />
          演示签名
        </Badge>
      </div>

      <div className="mt-2">
        <HashText
          hash={hash}
          className="bg-white text-[12px] text-indigo-700 ring-1 ring-inset ring-indigo-100"
        />
      </div>

      <p className="mt-2 text-[11px] leading-5 text-indigo-600/80">
        由任务、签署角色与条款勾选组合确定性生成，仅用于演示，不构成真实电子签名或法律效力。
      </p>

      {(signedAt || signerRoleLabel) && (
        <p className="mt-1 text-[11px] text-slate-500">
          {signerRoleLabel ? `签署角色：${signerRoleLabel}` : null}
          {signerRoleLabel && signedAt ? ' · ' : null}
          {signedAt ? `签署时间：${formatDateTime(signedAt)}` : null}
        </p>
      )}
    </div>
  );
}
