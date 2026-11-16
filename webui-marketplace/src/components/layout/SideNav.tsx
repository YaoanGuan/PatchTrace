import { NavLink } from 'react-router-dom';
import { GraduationCap, ShieldCheck, Store, UserRound } from 'lucide-react';
import { cn } from '../../lib/cn';

interface NavItem {
  to: string;
  label: string;
  hint: string;
  icon: typeof UserRound;
}

const navItems: NavItem[] = [
  { to: '/identity', label: '个人中枢', hint: 'M1', icon: UserRound },
  { to: '/marketplace', label: '任务大厅', hint: 'M2', icon: Store },
  { to: '/audit', label: '审计证据', hint: 'M3', icon: ShieldCheck },
  { to: '/onboarding', label: '准入信誉', hint: 'M4', icon: GraduationCap },
];

export function SideNav() {
  return (
    <aside className="w-full shrink-0 border-b border-slate-200 bg-white sm:w-56 sm:border-b-0 sm:border-r">
      <nav className="flex gap-1 overflow-x-auto p-2 sm:flex-col sm:p-3">
        {navItems.map(({ to, label, hint, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              cn(
                'flex shrink-0 items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-indigo-50 text-indigo-700'
                  : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900',
              )
            }
          >
            <Icon className="h-4 w-4" />
            <span>{label}</span>
            <span className="ml-auto hidden rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-normal text-slate-400 sm:inline">
              {hint}
            </span>
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
