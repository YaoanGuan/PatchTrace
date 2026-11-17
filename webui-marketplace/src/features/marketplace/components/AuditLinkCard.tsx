import { Link } from 'react-router-dom';
import { ArrowRight, FileSearch } from 'lucide-react';
import { Card } from '../../../components/ui/Card';

export interface AuditLinkCardProps {
  taskId: string;
  className?: string;
}

export function AuditLinkCard({ taskId, className }: AuditLinkCardProps) {
  return (
    <Card className={className}>
      <Link
        to={`/audit/${taskId}`}
        className="flex items-center gap-3 p-4 transition-colors hover:bg-neon/5"
      >
        <span className="rounded-lg bg-neon/10 p-2 text-neon">
          <FileSearch className="h-5 w-5" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-semibold text-ink">
            战报回放
          </span>
          <span className="block text-xs leading-5 text-ink-muted">
            需求、通关条件、模型调用、战斗动作与裁判结论均写入战报链。
          </span>
        </span>
        <ArrowRight className="h-4 w-4 shrink-0 text-ink-dim" />
      </Link>
    </Card>
  );
}
