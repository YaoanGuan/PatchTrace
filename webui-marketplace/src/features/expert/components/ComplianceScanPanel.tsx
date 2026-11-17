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
  PASS: '通关',
  WARN: '警戒',
  BLOCK: '关卡封锁',
};

const STATUS_CLASS: Record<string, string> = {
  PASS: 'bg-clear/10 text-clear ring-1 ring-inset ring-clear/30',
  WARN: 'bg-amber-500/10 text-amber-300 ring-1 ring-inset ring-amber-500/30',
  BLOCK: 'bg-boss/15 text-boss ring-1 ring-inset ring-boss/40',
};

function severityLabel(severity: ComplianceSeverity | undefined): string {
  return (severity && SEVERITY_LABEL[severity]) || '未标注';
}

function severityClass(severity: ComplianceSeverity | undefined): string {
  return (severity && SEVERITY_CLASS[severity]) || 'text-ink-muted bg-panel-soft';
}

/**
 * PCPD 基线副本规则扫描：规则 / 严重级别 / 状态 / 说明。
 * BLOCK 行以 boss 色高亮并标注「关卡封锁」。
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
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink">
            <ScanLine className="h-4 w-4 text-neon" />
            副本规则扫描
          </h2>
          <p className="mt-0.5 text-xs text-ink-muted">
            PCPD 基线规则逐条检查，BLOCK 命中会直接触发关卡封锁，任务不进入结算。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-1.5 text-[11px]">
          <span className="rounded-full bg-clear/10 px-2 py-0.5 font-medium text-clear ring-1 ring-inset ring-clear/30">
            通关 <span className="font-mono">{passCount}</span>
          </span>
          <span className="rounded-full bg-amber-500/10 px-2 py-0.5 font-medium text-amber-300 ring-1 ring-inset ring-amber-500/30">
            警戒 <span className="font-mono">{warnCount}</span>
          </span>
          <span className="rounded-full bg-boss/10 px-2 py-0.5 font-medium text-boss ring-1 ring-inset ring-boss/30">
            关卡封锁 <span className="font-mono">{blockCount}</span>
          </span>
        </div>
      </div>

      {list.length === 0 ? (
        <EmptyState
          className="mt-3"
          title="暂无副本规则扫描结果"
          description="任务进入副本规则扫描阶段后，命中的规则会显示在这里。"
        />
      ) : (
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[560px] border-collapse text-left text-xs">
            <thead>
              <tr className="border-b border-panel-line text-[11px] uppercase tracking-wide text-ink-muted">
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
                  'bg-panel-soft text-ink-muted ring-1 ring-inset ring-panel-line';
                return (
                  <tr
                    key={finding.id}
                    className={cn(
                      'border-b border-panel-line last:border-0',
                      blocked && 'border-l-2 border-l-boss bg-boss/10',
                    )}
                  >
                    <td className="py-2.5 pr-3">
                      <span
                        className={cn(
                          'font-medium',
                          blocked ? 'text-boss' : 'text-ink',
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
                        blocked ? 'text-boss' : 'text-ink-muted',
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
        <p className="mt-3 flex items-start gap-1.5 rounded-lg bg-boss/10 px-3 py-2 text-xs text-boss ring-1 ring-inset ring-boss/30">
          <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          关卡封锁：命中 <span className="font-mono">{blockCount}</span>{' '}
          条 BLOCK 级规则，任务在人工复核前不会进入结算。
        </p>
      )}
    </Card>
  );
}
