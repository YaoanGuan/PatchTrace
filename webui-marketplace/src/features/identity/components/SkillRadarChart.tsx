import type { SkillDimension, SkillScore } from '../../../types/domain';
import { useSessionStore } from '../../../app/store/sessionStore';
import { EmptyState } from '../../../components/ui/EmptyState';
import { Progress } from '../../../components/ui/Progress';
import { cn } from '../../../lib/cn';

export interface SkillRadarChartProps {
  skills?: SkillScore[];
  className?: string;
}

const DIMENSION_ORDER: { key: SkillDimension; label: string }[] = [
  { key: 'INSTRUCTION_PRECISION', label: 'AI指令精准度' },
  { key: 'TASK_CLOSURE', label: '副本闭环能力' },
  { key: 'COST_EFFICIENCY', label: '魔力效率' },
  { key: 'RISK_CONTROL', label: '风险预判' },
  { key: 'EVIDENCE_QUALITY', label: '战利品质量' },
];

interface ResolvedSkill {
  key: SkillDimension;
  label: string;
  score: number;
}

/**
 * 把 store 中的 skills 对齐到固定五维顺序。
 * 缺失维度记 0 分，非法分值夹取到 0–100，避免雷达图形变或页面崩溃。
 */
function resolveSkills(skills: SkillScore[] | undefined): ResolvedSkill[] {
  return DIMENSION_ORDER.map(({ key, label }) => {
    const found = skills?.find((item) => item.key === key);
    const raw = found?.score;
    const score =
      typeof raw === 'number' && Number.isFinite(raw)
        ? Math.max(0, Math.min(100, raw))
        : 0;
    return {
      key,
      label: found?.label?.trim() || label,
      score,
    };
  });
}

const VIEW_SIZE = 320;
const CENTER = { x: 160, y: 158 };
const RADIUS = 88;
const RINGS = [0.2, 0.4, 0.6, 0.8, 1];

const angleAt = (index: number): number =>
  (Math.PI * 2 * index) / DIMENSION_ORDER.length - Math.PI / 2;

const pointAt = (index: number, ratio: number): { x: number; y: number } => ({
  x: CENTER.x + Math.cos(angleAt(index)) * RADIUS * ratio,
  y: CENTER.y + Math.sin(angleAt(index)) * RADIUS * ratio,
});

const toPolygon = (ratios: number[]): string =>
  ratios
    .map((ratio, index) => {
      const point = pointAt(index, ratio);
      return `${point.x.toFixed(1)},${point.y.toFixed(1)}`;
    })
    .join(' ');

/**
 * 纯 SVG 手写五维雷达图，不引入图表库。
 * 数据缺失时渲染空态；正常时与 SkillLegend 共用同一套五维对齐逻辑。
 */
export function SkillRadarChart({
  skills,
  className,
}: SkillRadarChartProps) {
  const storeSkills = useSessionStore((state) => state.currentUser?.skills);
  const source = skills ?? storeSkills;
  const resolved = resolveSkills(source);

  if (!source || source.length === 0) {
    return (
      <EmptyState
        className={className}
        title="暂无技能树雷达数据"
        description="通关真实副本并通过平台复核后，这里会聚合出五维技能评分。"
      />
    );
  }

  return (
    <div className={cn('flex w-full items-center justify-center', className)}>
      <svg
        viewBox={`0 0 ${VIEW_SIZE} ${VIEW_SIZE}`}
        role="img"
        aria-label="冒险者五维技能树雷达图"
        className="h-auto w-full max-w-[360px]"
      >
        {/* 同心五边形网格 */}
        {RINGS.map((ring) => (
          <polygon
            key={`ring-${ring}`}
            points={toPolygon(DIMENSION_ORDER.map(() => ring))}
            fill="none"
            stroke="#1e2a48"
            strokeWidth={1}
          />
        ))}

        {/* 五条轴线 */}
        {DIMENSION_ORDER.map((dimension, index) => {
          const outer = pointAt(index, 1);
          return (
            <line
              key={`axis-${dimension.key}`}
              x1={CENTER.x}
              y1={CENTER.y}
              x2={outer.x}
              y2={outer.y}
              stroke="#1e2a48"
              strokeWidth={1}
            />
          );
        })}

        {/* 数据区域 */}
        <polygon
          points={toPolygon(resolved.map((item) => item.score / 100))}
          fill="rgba(167, 139, 250, 0.18)"
          stroke="#a78bfa"
          strokeWidth={2}
          strokeLinejoin="round"
        />

        {/* 顶点圆点 */}
        {resolved.map((item, index) => {
          const point = pointAt(index, item.score / 100);
          return (
            <circle
              key={`dot-${item.key}`}
              cx={point.x}
              cy={point.y}
              r={3}
              fill="#a78bfa"
            />
          );
        })}

        {/* 维度标签与分值 */}
        {resolved.map((item, index) => {
          const labelPoint = pointAt(index, 1.28);
          return (
            <g key={`label-${item.key}`}>
              <text
                x={labelPoint.x}
                y={labelPoint.y}
                textAnchor="middle"
                className="fill-ink-muted"
                fontSize={11}
              >
                {item.label}
              </text>
              <text
                x={labelPoint.x}
                y={labelPoint.y + 13}
                textAnchor="middle"
                className="fill-arcane font-mono"
                fontSize={11}
                fontWeight={600}
              >
                {Math.round(item.score)}
              </text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}

export interface SkillLegendProps {
  skills?: SkillScore[];
  className?: string;
}

/** 雷达图配套图例：逐维展示分值，便于精确读数。 */
export function SkillLegend({ skills, className }: SkillLegendProps) {
  const storeSkills = useSessionStore((state) => state.currentUser?.skills);
  const source = skills ?? storeSkills;
  const resolved = resolveSkills(source);

  if (!source || source.length === 0) {
    return (
      <p className={cn('text-xs text-ink-dim', className)}>
        暂无技能树维度数据。
      </p>
    );
  }

  return (
    <ul className={cn('space-y-2.5', className)}>
      {resolved.map((item) => (
        <li key={item.key} className="space-y-1">
          <div className="flex items-center justify-between text-xs">
            <span className="text-ink-muted">{item.label}</span>
            <span className="font-mono font-semibold text-arcane">
              {Math.round(item.score)}
            </span>
          </div>
          <Progress value={item.score} />
        </li>
      ))}
    </ul>
  );
}
