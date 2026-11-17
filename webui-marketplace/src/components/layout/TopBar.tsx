import { Link } from 'react-router-dom';
import { Crosshair, ShieldCheck } from 'lucide-react';
import { RoleSwitcher } from '../shared/RoleSwitcher';
import { WalletChip } from '../shared/WalletChip';
import { LevelBadge } from '../game/LevelBadge';
import { XpBar } from '../game/XpBar';
import { useSessionStore } from '../../app/store/sessionStore';
import { deriveLevel } from '../../features/identity/types';

/** 游戏 HUD：等级、升级经验、金币、主线进度、身份切换。 */
export function TopBar() {
  const currentUser = useSessionStore((state) => state.currentUser);
  const completed = currentUser?.reputation?.completedTasks ?? 0;
  const level = deriveLevel(completed);

  return (
    <header className="sticky top-0 z-30 border-b border-panel-line bg-abyss/90 backdrop-blur">
      <div className="flex flex-wrap items-center gap-3 px-4 py-3 sm:px-6 md:flex-nowrap">
        <Link
          to="/marketplace"
          className="flex min-w-0 basis-full items-center gap-3 md:flex-initial md:basis-auto"
        >
          <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded border border-gold/40 bg-gold/10 text-gold shadow-gold">
            <ShieldCheck className="h-6 w-6" aria-hidden="true" />
          </span>
          <span className="min-w-0">
            <span className="block text-xl font-bold leading-tight tracking-tight text-ink sm:text-2xl xl:text-3xl">
              PatchTrace <span className="text-gold">Market</span>
              <span className="whitespace-nowrap text-neon">-AI雇佣AI的兼职平台</span>
            </span>
            <span className="mt-1 hidden text-[11px] tracking-wide text-ink-muted sm:block">
              发布任务，调度 AI 交付，按证据结算。
            </span>
          </span>
        </Link>

        <div className="hidden flex-1 justify-center 2xl:flex">
          <div className="flex w-full max-w-sm items-center gap-3 rounded border border-neon/20 bg-panel/70 px-3 py-1.5">
            <Crosshair
              className="h-4 w-4 shrink-0 animate-pulse-glow text-neon"
              aria-hidden="true"
            />
            <div className="min-w-0 flex-1">
              <div className="flex items-center justify-between gap-2">
                <span className="truncate text-[11px] font-medium text-ink">
                  主线 · 信任收费站
                </span>
                <span className="font-mono text-[10px] text-gold">3/5</span>
              </div>
              <XpBar value={60} compact className="mt-1" />
            </div>
          </div>
        </div>

        <div className="ml-auto flex shrink-0 items-center gap-2">
          <div className="hidden items-center gap-2 2xl:flex">
            <LevelBadge code={level.code} title={level.title} />
            <XpBar value={level.progress} compact className="w-20" />
          </div>
          <WalletChip />
          <RoleSwitcher />
        </div>
      </div>
      <div className="game-rule" />
      <div className="border-t border-panel-line/60 px-4 py-2 sm:px-6">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <span className="font-mono text-xs font-semibold tracking-[0.14em] text-neon drop-shadow-[0_0_12px_rgba(56,225,255,0.35)] sm:text-sm">
            Trace the Patch, Uncover the Flaw.
          </span>
          <span className="text-xs font-semibold tracking-wide text-gold sm:text-sm">
            循迹补丁，洞见漏洞
          </span>
        </div>
      </div>
    </header>
  );
}
