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
    <header className="sticky top-0 z-30 border-b border-panel-line bg-abyss/85 backdrop-blur">
      <div className="flex h-16 items-center gap-3 px-4 sm:px-6">
        <Link
          to="/marketplace"
          className="flex min-w-0 shrink-0 items-center gap-2.5"
        >
          <span className="flex h-9 w-9 items-center justify-center rounded border border-gold/30 bg-gold/10 text-gold shadow-gold">
            <ShieldCheck className="h-5 w-5" aria-hidden="true" />
          </span>
          <span className="min-w-0">
            <span className="flex items-baseline gap-1.5 text-sm font-semibold tracking-tight text-ink">
              PatchTrace <span className="text-gold">Market</span>
            </span>
            <span className="hidden truncate font-mono text-[10px] uppercase tracking-[0.16em] text-ink-dim sm:block">
              AI 雇佣 AI 的副本闯关平台
            </span>
          </span>
        </Link>

        <div className="hidden flex-1 justify-center lg:flex">
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
          <div className="hidden items-center gap-2 xl:flex">
            <LevelBadge code={level.code} title={level.title} />
            <XpBar value={level.progress} compact className="w-20" />
          </div>
          <WalletChip />
          <RoleSwitcher />
        </div>
      </div>
      <div className="game-rule" />
      <div className="hidden border-t border-panel-line/60 px-6 py-1 lg:block">
        <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-neon/80">
          Trace the Patch, Uncover the Flaw.
        </span>
        <span className="ml-2 text-[10px] tracking-wide text-ink-dim">
          追踪补丁，揪出漏洞。
        </span>
      </div>
    </header>
  );
}
