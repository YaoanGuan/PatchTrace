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
        className="flex items-center gap-3 p-4 transition-colors hover:bg-slate-50"
      >
        <span className="rounded-lg bg-indigo-50 p-2 text-indigo-600">
          <FileSearch className="h-5 w-5" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-semibold text-slate-900">
            查看防篡改审计链
          </span>
          <span className="block text-xs leading-5 text-slate-500">
            需求、验收、模型调用、执行动作与复核结论均写入哈希链。
          </span>
        </span>
        <ArrowRight className="h-4 w-4 shrink-0 text-slate-400" />
      </Link>
    </Card>
  );
}
