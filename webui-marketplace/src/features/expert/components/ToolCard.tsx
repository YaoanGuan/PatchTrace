import { PlugZap, ShieldCheck } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import type {
  AiToolGrade,
  AiToolIntegration,
  AiToolStatus,
} from '../../../types/domain';
import {
  AI_TOOL_CATEGORY_LABEL,
  AI_TOOL_CATEGORY_ORIGIN_LABEL,
  AI_TOOL_GRADE_CLASS,
  AI_TOOL_GRADE_LABEL,
  AI_TOOL_STATUS_CLASS,
  AI_TOOL_STATUS_LABEL,
} from '../labels';

export interface ToolCardProps {
  tool: AiToolIntegration;
  className?: string;
}

const GRADE_FALLBACK_CLASS =
  'bg-panel-soft text-ink-muted ring-1 ring-inset ring-panel-line';
const STATUS_FALLBACK_CLASS =
  'bg-panel-soft text-ink-muted ring-1 ring-inset ring-panel-line';

function gradeClass(grade: AiToolGrade | undefined): string {
  return (grade && AI_TOOL_GRADE_CLASS[grade]) || GRADE_FALLBACK_CLASS;
}

function gradeLabel(grade: AiToolGrade | undefined): string {
  return (grade && AI_TOOL_GRADE_LABEL[grade]) || '未分级';
}

function statusClass(status: AiToolStatus | undefined): string {
  return (status && AI_TOOL_STATUS_CLASS[status]) || STATUS_FALLBACK_CLASS;
}

function statusLabel(status: AiToolStatus | undefined): string {
  return (status && AI_TOOL_STATUS_LABEL[status]) || '状态未知';
}

/**
 * 单个 AI 战友卡：名称、来源、召唤协议、稀有度、契约状态、公会权限与规则标签。
 * 所有数组/字段都做了空值兜底，缺失时展示占位文案而不是白屏。
 */
export function ToolCard({ tool, className }: ToolCardProps) {
  const roles = tool.roles ?? [];
  const permissions = tool.permissions ?? [];
  const compliance = tool.compliance ?? [];
  const canEmployerPublish = roles.includes('EMPLOYER');
  const canCommanderExecute = roles.includes('COMMANDER');

  return (
    <Card
      className={cn(
        'flex h-full flex-col p-4 transition-shadow hover:shadow-md',
        className,
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold text-ink">
            {tool.name || '未命名战友'}
          </h3>
          <p className="mt-0.5 truncate text-xs text-ink-muted">
            {tool.vendor || '来源未标注'}
          </p>
        </div>
        <span
          className={cn(
            'inline-flex shrink-0 items-center rounded-md px-2 py-0.5 text-xs font-bold',
            gradeClass(tool.grade),
          )}
          title={`稀有度 ${gradeLabel(tool.grade)}`}
        >
          {gradeLabel(tool.grade)}
        </span>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-1.5">
        <span className="inline-flex items-center gap-1 rounded-full bg-panel-soft px-2 py-0.5 text-[11px] font-medium text-ink-muted ring-1 ring-inset ring-panel-line">
          <PlugZap className="h-3 w-3" />
          {tool.protocol || '协议未标注'}
        </span>
        <span
          className={cn(
            'inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium',
            statusClass(tool.status),
          )}
        >
          {statusLabel(tool.status)}
        </span>
        <span
          className="inline-flex items-center rounded-full bg-arcane/10 px-2 py-0.5 text-[11px] font-medium text-arcane ring-1 ring-inset ring-arcane/30"
          title={
            tool.category
              ? AI_TOOL_CATEGORY_ORIGIN_LABEL[tool.category] ?? tool.category
              : undefined
          }
        >
          {tool.category
            ? AI_TOOL_CATEGORY_LABEL[tool.category] ?? tool.category
            : '分类未标注'}
        </span>
      </div>

      <div className="mt-3 space-y-1.5 text-xs">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-ink-muted">公会权限</span>
          {canEmployerPublish || canCommanderExecute ? (
            <>
              {canEmployerPublish && (
                <span className="rounded bg-clear/10 px-1.5 py-0.5 font-medium text-clear">
                  雇主公会可发布
                </span>
              )}
              {canCommanderExecute && (
                <span className="rounded bg-arcane/10 px-1.5 py-0.5 font-medium text-arcane">
                  指挥官战队可执行
                </span>
              )}
            </>
          ) : (
            <span className="rounded bg-panel-soft px-1.5 py-0.5 text-ink-dim">
              未授权角色
            </span>
          )}
        </div>
        {permissions.length > 0 && (
          <div className="flex flex-wrap items-center gap-1">
            {permissions.map((permission) => (
              <span
                key={permission}
                className="rounded border border-panel-line px-1.5 py-0.5 font-mono text-[10px] text-ink-muted"
              >
                {permission}
              </span>
            ))}
          </div>
        )}
      </div>

      <div className="mt-auto flex flex-wrap items-center gap-1.5 pt-3">
        <ShieldCheck className="h-3.5 w-3.5 text-neon" />
        {compliance.length > 0 ? (
          compliance.map((tag) => (
            <span
              key={tag}
              className="rounded-full bg-neon/10 px-2 py-0.5 text-[10px] font-medium text-neon ring-1 ring-inset ring-neon/30"
            >
              {tag}
            </span>
          ))
        ) : (
          <span className="text-[11px] text-ink-dim">暂无规则标签</span>
        )}
      </div>
    </Card>
  );
}
