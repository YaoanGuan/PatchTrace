import { ScanLine, ShieldAlert } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import { cn } from '../../../lib/cn';
import type {
  ComplianceFinding,
  ComplianceSeverity,
} from '../../../types/domain';
import { SEVERITY_CLASS, SEVERITY_LABEL } from '../labels';

export interface ComplianceScanPanelProps {
  findings?: ComplianceFinding[] | null;
  className?: string;
}

const STATUS_LABEL: Record<string, string> = {
  PASS: '通过',
  WARN: '告警',
  BLOCK: '已阻断',
};

const STATUS_CLASS: Record<string, string> = {
  PASS: 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-600/20',
  WARN: 'bg-amber-50 text-amber-700 ring-1 ring-inset ring-amber-600/20',
  BLOCK: 'bg-rose-600 text-white',
};

function severityLabel(severity: ComplianceSeverity | undefined): string {
  return (severity && SEVERITY_LABEL[severity]) || '未标注';
}

function severityClass(severity: ComplianceSeverity | undefined): string {
  return (severity && SEVERITY_CLASS[severity]) || 'text-slate-600 bg-slate-100';
}

/**
 * PCPD 基线合规扫描：规则 / 严重级别 / 状态 / 说明。
 * BLOCK 行以红色高亮并标注「已阻断」。
 */
export function ComplianceScanPanel({
  findings,
  className,
}: ComplianceScanPanelProps) {
  const list = findings ?? [];
  const blockCount = list.filter((item) => item.status === 'BLOCK').length;
  const warnCount = list.filter((item) => item.status === 'WARN').length;
  const passCount = list.filter((item) => item.status === 'PASS').length;

  return (
    <Card className={cn('p-4', className)}>
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
            <ScanLine className="h-4 w-4 text-indigo-600" />
            自动化合规扫描
          </h2>
          <p className="mt-0.5 text-xs text-slate-500">
            PCPD 基线规则逐条检查，BLOCK 命中会直接阻断任务结算。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-1.5 text-[11px]">
          <span className="rounded-full bg-emerald-50 px-2 py-0.5 font-medium text-emerald-700">
            通过 {passCount}
          </span>
          <span className="rounded-full bg-amber-50 px-2 py-0.5 font-medium text-amber-700">
            告警 {warnCount}
          </span>
          <span className="rounded-full bg-rose-50 px-2 py-0.5 font-medium text-rose-700">
            阻断 {blockCount}
          </span>
        </div>
      </div>

      {list.length === 0 ? (
        <EmptyState
          className="mt-3"
          title="暂无合规扫描结果"
          description="任务进入合规扫描阶段后，命中规则会显示在这里。"
        />
      ) : (
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[560px] border-collapse text-left text-xs">
            <thead>
              <tr className="border-b border-slate-200 text-[11px] uppercase tracking-wide text-slate-500">
                <th className="py-2 pr-3 font-medium">规则</th>
                <th className="py-2 pr-3 font-medium">严重级别</th>
                <th className="py-2 pr-3 font-medium">状态</th>
                <th className="py-2 font-medium">说明</th>
              </tr>
            </thead>
            <tbody>
              {list.map((finding) => {
                const blocked = finding.status === 'BLOCK';
                const statusLabel = STATUS_LABEL[finding.status] ?? finding.status;
                const statusClass =
                  STATUS_CLASS[finding.status] ??
                  'bg-slate-100 text-slate-600 ring-1 ring-inset ring-slate-500/20';
                return (
                  <tr
                    key={finding.id}
                    className={cn(
                      'border-b border-slate-100 last:border-0',
                      blocked && 'border-l-2 border-l-rose-500 bg-rose-50/60',
                    )}
                  >
                    <td className="py-2.5 pr-3">
                      <span
                        className={cn(
                          'font-medium',
                          blocked ? 'text-rose-700' : 'text-slate-800',
                        )}
                      >
                        {finding.rule || '未命名规则'}
                      </span>
                    </td>
                    <td className="py-2.5 pr-3">
                      <span
                        className={cn(
                          'inline-flex rounded px-1.5 py-0.5 text-[10px] font-semibold',
                          severityClass(finding.severity),
                        )}
                      >
                        {severityLabel(finding.severity)}
                      </span>
                    </td>
                    <td className="py-2.5 pr-3">
                      <span
                        className={cn(
                          'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold',
                          statusClass,
                        )}
                      >
                        {blocked && <ShieldAlert className="h-3 w-3" />}
                        {statusLabel || '状态未知'}
                      </span>
                    </td>
                    <td
                      className={cn(
                        'py-2.5 leading-5',
                        blocked ? 'text-rose-700' : 'text-slate-600',
                      )}
                    >
                      {finding.detail || '无补充说明'}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {blockCount > 0 && (
        <p className="mt-3 flex items-start gap-1.5 rounded-lg bg-rose-50 px-3 py-2 text-xs text-rose-700">
          <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          已阻断：命中 {blockCount} 条 BLOCK 级规则，任务在人工复核前不会进入结算。
        </p>
      )}
    </Card>
  );
}
