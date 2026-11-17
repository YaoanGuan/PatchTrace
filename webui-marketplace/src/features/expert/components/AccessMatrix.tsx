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
 * 接入矩阵：分类 + 信任等级双重筛选，网格展示所有已接入 / 待接入的 AI 工具。
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
        label: `${key} 级`,
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
          <h2 className="flex items-center gap-1.5 text-base font-semibold text-slate-900">
            <LayoutGrid className="h-4 w-4 text-indigo-600" />
            接入矩阵
          </h2>
          <p className="mt-0.5 text-xs text-slate-500">
            云托管计算机 / AI CLI / AI IDE / 聊天客户端四类全覆盖 · 共 {list.length}{' '}
            个工具，{connectedCount} 个已连接
          </p>
        </div>
        <p className="text-xs text-slate-400">
          当前筛选：{filtered.length} 个
        </p>
      </div>

      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs font-medium text-slate-500">分类</span>
          <Tabs
            items={categoryItems}
            value={category}
            onChange={(key) => setCategory(key as CategoryFilter)}
          />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs font-medium text-slate-500">等级</span>
          <Tabs
            items={gradeItems}
            value={grade}
            onChange={(key) => setGrade(key as GradeFilter)}
          />
        </div>
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          title="当前筛选下暂无接入工具"
          description="尝试切换分类或信任等级；工具接入后会在这里展示协议、权限与合规标签。"
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
