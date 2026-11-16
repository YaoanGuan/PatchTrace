import { useCallback, useEffect, useMemo, useState } from 'react';
import { getTasks, getTrace } from '../../../lib/api/client';
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
        caught instanceof Error ? caught.message : '审计数据加载失败，请重试',
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
      <div className="space-y-2">
        <p className="text-xs font-medium uppercase tracking-wide text-indigo-600">
          M3 · 责任隔离与防篡改审计
        </p>
        <h1 className="text-2xl font-semibold tracking-tight text-slate-900">
          审计证据
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-slate-500">
          以任务为单位查看不可篡改的执行证据链。列表对已加载任务逐条执行
          prevHash 校验，异常链路会标记并可进入详情定位断点；默认演示任务
          task_1001 拥有完整的 6 类节点证据链。
        </p>
      </div>

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
