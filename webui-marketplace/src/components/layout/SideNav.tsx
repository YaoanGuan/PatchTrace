import { NavLink } from 'react-router-dom';
import {
  CircuitBoard,
  Crosshair,
  ExternalLink,
  GraduationCap,
  PanelLeftClose,
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

const PATCHTRACE_HOME_URL =
  typeof window === 'undefined'
    ? 'http://localhost:9999/'
    : `${window.location.protocol}//${window.location.hostname}:9999/`;

export interface SideNavProps {
  onCollapse: () => void;
}

export function SideNav({ onCollapse }: SideNavProps) {
  return (
    <aside
      id="site-sidebar"
      className="w-full shrink-0 border-b border-panel-line bg-panel/60 lg:w-60 lg:border-b-0 lg:border-r"
    >
      <div className="flex items-center justify-between border-b border-panel-line/70 px-4 py-3">
        <span className="font-mono text-[10px] uppercase tracking-[0.22em] text-ink-dim">
          Quest Log <span className="text-gold">5 CHAPTERS</span>
        </span>
        <button
          type="button"
          onClick={onCollapse}
          aria-label="收起侧栏"
          aria-controls="site-sidebar"
          aria-expanded="true"
          title="收起侧栏"
          className="inline-flex h-7 w-7 items-center justify-center rounded border border-panel-line text-ink-muted transition-colors hover:border-neon/40 hover:text-neon focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-neon"
        >
          <PanelLeftClose className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      <nav className="flex gap-1 overflow-x-auto p-2 lg:flex-col lg:p-3">
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
            <span className="ml-auto hidden rounded border border-panel-line px-1.5 py-0.5 font-mono text-[9px] font-normal text-ink-dim lg:inline">
              {hint}
            </span>
          </NavLink>
        ))}
      </nav>

      <div className="px-2 pb-2 lg:px-3 lg:pb-3">
        <a
          href={PATCHTRACE_HOME_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-2 rounded border border-neon/30 bg-neon/10 px-3 py-2 text-xs font-semibold text-neon transition-colors hover:border-neon/60 hover:bg-neon/15 hover:text-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-neon"
        >
          <Crosshair className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="flex-1">渗透找外援就上循迹</span>
          <ExternalLink className="h-3.5 w-3.5 shrink-0 opacity-70" aria-hidden="true" />
        </a>
      </div>

      <div className="hidden px-4 py-4 lg:block">
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
