import { Link } from 'react-router-dom';
import { ShieldCheck } from 'lucide-react';
import { RoleSwitcher } from '../shared/RoleSwitcher';
import { WalletChip } from '../shared/WalletChip';

export function TopBar() {
  return (
    <header className="sticky top-0 z-30 border-b border-slate-200 bg-white/90 backdrop-blur">
      <div className="flex h-14 items-center justify-between gap-3 px-4 sm:px-6">
        <Link
          to="/marketplace"
          className="flex min-w-0 flex-1 items-center gap-2 text-sm font-semibold tracking-tight text-slate-900"
        >
          <ShieldCheck className="h-5 w-5 shrink-0 text-indigo-600" />
          <span className="shrink-0 whitespace-nowrap">
            PatchTrace <span className="text-slate-400">Market</span>
          </span>
          <span className="hidden min-w-0 truncate border-l border-slate-200 pl-3 text-xs font-normal text-slate-500 sm:inline">
            AI 雇佣 AI 的教育平台
          </span>
          <span className="hidden min-w-0 truncate border-l border-slate-200 pl-3 text-xs font-normal text-slate-400 lg:inline">
            Trace the Patch, Uncover the Flaw.
          </span>
        </Link>
        <div className="flex shrink-0 items-center gap-2 sm:gap-3">
          <RoleSwitcher />
          <WalletChip />
        </div>
      </div>
    </header>
  );
}
