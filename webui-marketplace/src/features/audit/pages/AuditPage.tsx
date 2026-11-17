import { useCallback, useEffect, useMemo, useState } from 'react';
import { getTasks, getTrace } from '../../../lib/api/client';
import { Badge } from '../../../components/ui/Badge';
import { QuestHeading } from '../../../components/game/QuestHeading';
import { AuditFilterBar } from '../components/AuditFilterBar';
import { AuditTaskList } from '../components/AuditTaskList';
import { HashChainSummary } from '../components/HashChainSummary';
import type { AuditTaskEntry, ChainFilter } from '../types';
import { latestNodeAt, verifyTrace } from '../utils';

export function AuditPage() {
  const [entries, setEntries] = useState<AuditTaskEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [keyword, setKeyword] = useState('');
  const [chainFilter, setChainFilter] = useState<ChainFilter>('ALL');

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await getTasks();
      // 详情页与列表页共用同一校验函数；当前 mock 数据量小，逐条并发校验。
      // 接真实后端时应改为批量接口或仅对当前页前若干条采样校验。
      const traces = await Promise.all(
        response.items.map((task) => getTrace(task.id).catch(() => null)),
      );
      const nextEntries: AuditTaskEntry[] = response.items.map(
        (task, index) => {
          const trace = traces[index] ?? null;
          return {
            task,
            trace,
            chain: verifyTrace(trace),
            lastNodeAt: trace ? latestNodeAt(trace.nodes) : null,
          };
        },
      );
      setEntries(nextEntries);
    } catch (caught) {
      setEntries([]);
      setError(
        caught instanceof Error ? caught.message : '战报数据加载失败，请重试',
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const counts = useMemo(
    () =>
      entries.reduce(
        (accumulator, entry) => {
          accumulator.total += 1;
          if (entry.chain.state === 'VALID') {
            accumulator.valid += 1;
          } else if (entry.chain.state === 'INVALID') {
            accumulator.invalid += 1;
          } else {
            accumulator.unverified += 1;
          }
          return accumulator;
        },
        { total: 0, valid: 0, invalid: 0, unverified: 0 },
      ),
    [entries],
  );

  const filteredEntries = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    return entries.filter((entry) => {
      const matchesKeyword =
        normalizedKeyword.length === 0 ||
        `${entry.task.id} ${entry.task.title} ${entry.task.category} ${entry.task.publisher.displayName}`
          .toLowerCase()
          .includes(normalizedKeyword);
      const matchesChain =
        chainFilter === 'ALL' ||
        (chainFilter === 'VALID' && entry.chain.state === 'VALID') ||
        (chainFilter === 'INVALID' && entry.chain.state === 'INVALID');
      return matchesKeyword && matchesChain;
    });
  }, [entries, keyword, chainFilter]);

  return (
    <section className="space-y-6">
      <QuestHeading
        chapter="WAR ARCHIVE · M4"
        title="战报回放"
        description="以副本为单位查看不可篡改的执行战报。列表对已加载副本逐条执行 prevHash 封印校验，封印破损的副本会标记并可进入详情定位断点；默认演示副本 task_1001 拥有完整的 6 类战报节点。"
        badge={
          <Badge variant="outline" className="font-mono">
            副本 {counts.total}
          </Badge>
        }
      />

      <AuditFilterBar
        keyword={keyword}
        chainFilter={chainFilter}
        counts={counts}
        onKeywordChange={setKeyword}
        onChainFilterChange={setChainFilter}
      />

      <AuditTaskList
        entries={filteredEntries}
        loading={loading}
        error={error}
        onRetry={() => {
          void load();
        }}
      />

      {!loading && !error && <HashChainSummary entries={entries} />}
    </section>
  );
}

export default AuditPage;
