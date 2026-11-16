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
          className="flex items-center gap-2 text-sm font-semibold tracking-tight text-slate-900"
        >
          <ShieldCheck className="h-5 w-5 text-indigo-600" />
          <span>
            PatchTrace <span className="text-slate-400">Marketplace</span>
          </span>
        </Link>
        <div className="flex items-center gap-2 sm:gap-3">
          <RoleSwitcher />
          <WalletChip />
        </div>
      </div>
    </header>
  );
}
