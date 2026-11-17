import { useMemo, useState } from 'react';
import { LayoutGrid } from 'lucide-react';
import { Tabs } from '../../../components/ui/Tabs';
import { EmptyState } from '../../../components/ui/EmptyState';
import { cn } from '../../../lib/cn';
import type {
  AiToolCategory,
  AiToolGrade,
  AiToolIntegration,
} from '../../../types/domain';
import {
  AI_TOOL_CATEGORY_LABEL,
  AI_TOOL_CATEGORY_ORDER,
  AI_TOOL_GRADE_LABEL,
  AI_TOOL_GRADES,
} from '../labels';
import { ToolCard } from './ToolCard';

export interface AccessMatrixProps {
  tools?: AiToolIntegration[] | null;
  className?: string;
}

type CategoryFilter = AiToolCategory | 'ALL';
type GradeFilter = AiToolGrade | 'ALL';

/**
 * 战友召唤图鉴：召唤通道 + 稀有度双重筛选，网格展示所有已缔结契约 / 待召唤的 AI 战友。
 */
export function AccessMatrix({ tools, className }: AccessMatrixProps) {
  const list = useMemo(() => tools ?? [], [tools]);
  const [category, setCategory] = useState<CategoryFilter>('ALL');
  const [grade, setGrade] = useState<GradeFilter>('ALL');

  const categoryItems = useMemo(
    () => [
      { key: 'ALL', label: `全部 ${list.length}` },
      ...AI_TOOL_CATEGORY_ORDER.map((key) => ({
        key,
        label: `${AI_TOOL_CATEGORY_LABEL[key]} ${
          list.filter((tool) => tool.category === key).length
        }`,
      })),
    ],
    [list],
  );

  const gradeItems = useMemo(
    () => [
      { key: 'ALL', label: '全部等级' },
      ...AI_TOOL_GRADES.map((key) => ({
        key,
        label: AI_TOOL_GRADE_LABEL[key],
      })),
    ],
    [],
  );

  const filtered = useMemo(
    () =>
      list.filter(
        (tool) =>
          (category === 'ALL' || tool.category === category) &&
          (grade === 'ALL' || tool.grade === grade),
      ),
    [category, grade, list],
  );

  const connectedCount = list.filter(
    (tool) => tool.status === 'CONNECTED',
  ).length;

  return (
    <section className={cn('space-y-3', className)}>
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div>
          <h2 className="flex items-center gap-1.5 text-base font-semibold text-ink">
            <LayoutGrid className="h-4 w-4 text-arcane" />
            战友召唤图鉴
          </h2>
          <p className="mt-0.5 text-xs text-ink-muted">
            云座驾（云托管计算机）/ 命令行召唤（AI CLI）/ 锻造台（AI IDE）/
            传声筒（聊天客户端）四类召唤通道全覆盖 · 共{' '}
            <span className="font-mono text-ink">{list.length}</span> 位战友，
            <span className="font-mono text-clear">{connectedCount}</span>{' '}
            位已缔结契约
          </p>
        </div>
        <p className="text-xs text-ink-dim">
          当前筛选：<span className="font-mono text-ink">{filtered.length}</span> 位
        </p>
      </div>

      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs font-medium text-ink-muted">召唤类型</span>
          <Tabs
            items={categoryItems}
            value={category}
            onChange={(key) => setCategory(key as CategoryFilter)}
          />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs font-medium text-ink-muted">稀有度</span>
          <Tabs
            items={gradeItems}
            value={grade}
            onChange={(key) => setGrade(key as GradeFilter)}
          />
        </div>
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          title="当前筛选下暂无已召唤战友"
          description="尝试切换召唤类型或稀有度；战友缔结契约后会在这里展示召唤协议、权限与规则标签。"
        />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {filtered.map((tool) => (
            <ToolCard key={tool.id} tool={tool} />
          ))}
        </div>
      )}
    </section>
  );
}
