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
  AI_TOOL_GRADE_CLASS,
  AI_TOOL_GRADE_LABEL,
  AI_TOOL_STATUS_CLASS,
  AI_TOOL_STATUS_LABEL,
} from '../labels';

export interface ToolCardProps {
  tool: AiToolIntegration;
  className?: string;
}

const GRADE_FALLBACK_CLASS = 'bg-slate-500 text-white';
const STATUS_FALLBACK_CLASS = 'bg-slate-100 text-slate-600';

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
 * 单个 AI 工具接入卡：名称、厂商、协议、信任等级、状态、角色权限与合规标签。
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
          <h3 className="truncate text-sm font-semibold text-slate-900">
            {tool.name || '未命名工具'}
          </h3>
          <p className="mt-0.5 truncate text-xs text-slate-500">
            {tool.vendor || '厂商未标注'}
          </p>
        </div>
        <span
          className={cn(
            'inline-flex shrink-0 items-center rounded-md px-2 py-0.5 text-xs font-bold',
            gradeClass(tool.grade),
          )}
          title={`信任等级 ${gradeLabel(tool.grade)}`}
        >
          {gradeLabel(tool.grade)}
        </span>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-1.5">
        <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600">
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
        <span className="inline-flex items-center rounded-full bg-indigo-50 px-2 py-0.5 text-[11px] font-medium text-indigo-700 ring-1 ring-inset ring-indigo-600/20">
          {tool.category
            ? AI_TOOL_CATEGORY_LABEL[tool.category] ?? tool.category
            : '分类未标注'}
        </span>
      </div>

      <div className="mt-3 space-y-1.5 text-xs">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-slate-500">角色权限</span>
          {canEmployerPublish || canCommanderExecute ? (
            <>
              {canEmployerPublish && (
                <span className="rounded bg-emerald-50 px-1.5 py-0.5 font-medium text-emerald-700">
                  雇主可发布
                </span>
              )}
              {canCommanderExecute && (
                <span className="rounded bg-sky-50 px-1.5 py-0.5 font-medium text-sky-700">
                  指挥官可执行
                </span>
              )}
            </>
          ) : (
            <span className="rounded bg-slate-100 px-1.5 py-0.5 text-slate-500">
              未授权角色
            </span>
          )}
        </div>
        {permissions.length > 0 && (
          <div className="flex flex-wrap items-center gap-1">
            {permissions.map((permission) => (
              <span
                key={permission}
                className="rounded border border-slate-200 px-1.5 py-0.5 font-mono text-[10px] text-slate-500"
              >
                {permission}
              </span>
            ))}
          </div>
        )}
      </div>

      <div className="mt-auto flex flex-wrap items-center gap-1.5 pt-3">
        <ShieldCheck className="h-3.5 w-3.5 text-slate-400" />
        {compliance.length > 0 ? (
          compliance.map((tag) => (
            <span
              key={tag}
              className="rounded-full bg-slate-50 px-2 py-0.5 text-[10px] font-medium text-slate-600 ring-1 ring-inset ring-slate-200"
            >
              {tag}
            </span>
          ))
        ) : (
          <span className="text-[11px] text-slate-400">暂无合规标签</span>
        )}
      </div>
    </Card>
  );
}
