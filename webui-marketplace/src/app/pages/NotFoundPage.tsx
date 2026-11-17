import { Link } from 'react-router-dom';
import { Compass } from 'lucide-react';

export default function NotFoundPage() {
  return (
    <section className="game-panel flex min-h-[50vh] flex-col items-center justify-center gap-3 px-6 text-center">
      <Compass className="h-10 w-10 text-ink-dim" aria-hidden="true" />
      <p className="font-mono text-xs uppercase tracking-[0.22em] text-neon">
        Lost Dungeon · 404
      </p>
      <h1 className="text-2xl font-semibold tracking-tight text-ink">
        这条副本路线不存在
      </h1>
      <p className="max-w-md text-sm text-ink-muted">
        你走进了未开放的岔路。返回悬赏大厅，重新接取主线任务。
      </p>
      <Link
        to="/marketplace"
        className="mt-2 inline-flex items-center gap-2 rounded border border-gold/40 bg-gold/10 px-4 py-2 text-sm font-medium text-gold transition-colors hover:bg-gold/20"
      >
        返回悬赏大厅
      </Link>
    </section>
  );
}
