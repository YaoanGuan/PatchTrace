import { ShieldCheck } from 'lucide-react';

export interface ComplianceAckProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}

export function ComplianceAck({
  checked,
  onChange,
  disabled,
}: ComplianceAckProps) {
  return (
    <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-amber-200 bg-amber-50/70 p-3">
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        className="mt-0.5 h-4 w-4 shrink-0 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
      />
      <span className="space-y-1">
        <span className="flex items-center gap-1.5 text-sm font-medium text-amber-900">
          <ShieldCheck className="h-4 w-4" />
          合规承诺
        </span>
        <span className="block text-xs leading-5 text-amber-800">
          我承诺任务目标合法合规，不包含未授权的攻击目标或敏感数据；
          已阅读平台责任边界，理解雇主、指挥官与平台三方责任划分。
        </span>
      </span>
    </label>
  );
}
