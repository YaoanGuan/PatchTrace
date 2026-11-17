import { NavLink } from 'react-router-dom';
import {
  CircuitBoard,
  GraduationCap,
  ShieldCheck,
  Store,
  UserRound,
} from 'lucide-react';
import { cn } from '../../lib/cn';

interface NavItem {
  to: string;
  label: string;
  hint: string;
  chapter: string;
  icon: typeof UserRound;
}

const navItems: NavItem[] = [
  {
    to: '/identity',
    label: '角色圣殿',
    hint: 'M1',
    chapter: '01',
    icon: UserRound,
  },
  {
    to: '/expert',
    label: '专家祭坛',
    hint: 'M2',
    chapter: '02',
    icon: CircuitBoard,
  },
  {
    to: '/marketplace',
    label: '悬赏大厅',
    hint: 'M3',
    chapter: '03',
    icon: Store,
  },
  {
    to: '/audit',
    label: '战报档案馆',
    hint: 'M4',
    chapter: '04',
    icon: ShieldCheck,
  },
  {
    to: '/onboarding',
    label: '新手村',
    hint: 'M5',
    chapter: '05',
    icon: GraduationCap,
  },
];

export function SideNav() {
  return (
    <aside className="w-full shrink-0 border-b border-panel-line bg-panel/60 sm:w-60 sm:border-b-0 sm:border-r">
      <div className="hidden items-center justify-between border-b border-panel-line/70 px-4 py-3 sm:flex">
        <span className="font-mono text-[10px] uppercase tracking-[0.22em] text-ink-dim">
          Quest Log
        </span>
        <span className="font-mono text-[10px] text-gold">5 CHAPTERS</span>
      </div>
      <nav className="flex gap-1 overflow-x-auto p-2 sm:flex-col sm:p-3">
        {navItems.map(({ to, label, hint, chapter, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              cn(
                'group flex shrink-0 items-center gap-2.5 rounded border px-3 py-2 text-sm font-medium transition-all',
                isActive
                  ? 'border-gold/40 bg-gold/10 text-gold shadow-[inset_0_0_18px_rgba(246,196,83,.08)]'
                  : 'border-transparent text-ink-muted hover:bg-panel-soft hover:text-ink',
              )
            }
          >
            <span
              className={cn(
                'font-mono text-[10px]',
                'text-ink-dim group-hover:text-neon',
              )}
            >
              {chapter}
            </span>
            <Icon className="h-4 w-4" aria-hidden="true" />
            <span>{label}</span>
            <span className="ml-auto hidden rounded border border-panel-line px-1.5 py-0.5 font-mono text-[9px] font-normal text-ink-dim sm:inline">
              {hint}
            </span>
          </NavLink>
        ))}
      </nav>
      <div className="hidden px-4 py-4 sm:block">
        <div className="game-rule mb-3" />
        <p className="font-mono text-[10px] uppercase tracking-[0.18em] text-ink-dim">
          Next Boss
        </p>
        <p className="mt-1 text-xs text-ink-muted">
          大Boss · 未验证交付
        </p>
      </div>
    </aside>
  );
}
