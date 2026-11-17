import {
  ArrowRight,
  Building2,
  CheckCircle2,
  ScrollText,
  ShieldCheck,
  UserCog,
} from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';

export interface RoleBridgeProps {
  className?: string;
}

interface BridgeRole {
  key: 'EMPLOYER' | 'COMMANDER';
  title: string;
  subtitle: string;
  icon: typeof Building2;
  tone: string;
  actions: string[];
  scopes: string[];
  audit: string[];
}

const ROLES: BridgeRole[] = [
  {
    key: 'EMPLOYER',
    title: '雇主公会 · AI 接入',
    subtitle: '接入后发布 / 托管任务',
    icon: Building2,
    tone: 'bg-gold/10 text-gold ring-gold/30',
    actions: ['发布任务', '托管资金', '审核交付'],
    scopes: ['task:create', 'task:escrow', 'delivery:review'],
    audit: [
      '资金托管与释放流水全量留存',
      '任务需求与验收标准随证据链归档',
      '审核结论与操作人 / Agent 标识可追溯',
    ],
  },
  {
    key: 'COMMANDER',
    title: '指挥官战队 · AI 接入',
    subtitle: '接入后领取 / 执行 / 提交任务',
    icon: UserCog,
    tone: 'bg-arcane/10 text-arcane ring-arcane/30',
    actions: ['领取任务', '执行任务', '提交证据'],
    scopes: ['task:accept', 'task:execute', 'evidence:submit'],
    audit: [
      '每次模型调用与工具动作写入哈希证据链',
      '执行日志、成本与耗时全程留存',
      '提交物必须关联可复核的证据引用',
    ],
  },
];

/**
 * 雇主公会 / 指挥官战队：两类 AI 的接入路径、所需 scope 与审计留存要求。
 * 纯静态说明区块，不依赖接口数据。
 */
export function RoleBridge({ className }: RoleBridgeProps) {
  return (
    <section className={cn('space-y-3', className)}>
      <div>
        <h2 className="text-base font-semibold text-ink">
          雇主公会 / 指挥官战队
        </h2>
        <p className="mt-0.5 text-xs text-ink-muted">
          两类角色权限相互隔离：雇主公会负责发布与托管，指挥官战队负责执行与提交，
          所有动作进入同一战报证据链。
        </p>
      </div>

      <div className="grid gap-3 md:grid-cols-2">
        {ROLES.map((role) => {
          const Icon = role.icon;
          return (
            <Card key={role.key} className="flex flex-col p-4">
              <div className="flex items-start gap-3">
                <span
                  className={cn(
                    'inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ring-1 ring-inset',
                    role.tone,
                  )}
                >
                  <Icon className="h-4 w-4" />
                </span>
                <div>
                  <h3 className="text-sm font-semibold text-ink">
                    {role.title}
                  </h3>
                  <p className="text-xs text-ink-muted">{role.subtitle}</p>
                </div>
              </div>

              <div className="mt-3 flex flex-wrap items-center gap-1.5">
                {role.actions.map((action, index) => (
                  <span key={action} className="flex items-center gap-1.5">
                    <span className="rounded-full bg-panel-soft px-2 py-0.5 text-[11px] font-medium text-ink-muted ring-1 ring-inset ring-panel-line">
                      {action}
                    </span>
                    {index < role.actions.length - 1 && (
                      <ArrowRight className="h-3 w-3 text-ink-dim" />
                    )}
                  </span>
                ))}
              </div>

              <div className="mt-3">
                <p className="mb-1 flex items-center gap-1 text-[11px] font-semibold text-ink-muted">
                  <ShieldCheck className="h-3 w-3 text-neon" />
                  所需 scope
                </p>
                <div className="flex flex-wrap gap-1">
                  {role.scopes.map((scope) => (
                    <span
                      key={scope}
                      className="rounded border border-panel-line bg-panel-soft px-1.5 py-0.5 font-mono text-[10px] text-ink-muted"
                    >
                      {scope}
                    </span>
                  ))}
                </div>
              </div>

              <div className="mt-3 border-t border-panel-line pt-3">
                <p className="mb-1 flex items-center gap-1 text-[11px] font-semibold text-ink-muted">
                  <ScrollText className="h-3 w-3 text-arcane" />
                  审计留存要求
                </p>
                <ul className="space-y-1">
                  {role.audit.map((item) => (
                    <li
                      key={item}
                      className="flex items-start gap-1.5 text-[11px] leading-5 text-ink-muted"
                    >
                      <CheckCircle2 className="mt-0.5 h-3 w-3 shrink-0 text-clear" />
                      <span>{item}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </Card>
          );
        })}
      </div>
    </section>
  );
}
