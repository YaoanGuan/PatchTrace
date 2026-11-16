import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from './components/ui/card';
import { Badge } from './components/ui/badge';
import { Button } from './components/ui/button';
import { ScrollArea } from './components/ui/scroll-area';

// GraphPanel renders the read-only provenance graph exposed by the graph lane.
// It never writes: every request is a GET, and the only controls are refresh
// and project selection.

type GraphProject = {
    id: string;
    title: string;
    origin: string;
    goal: string;
    status: string;
    bootstrap_enabled?: boolean;
    created_at?: string;
};

type GraphNode = {
    id: string;
    project_id?: string;
    kind: string;
    type: string;
    content: string;
    state?: string;
    from?: string[] | null;
    to?: string | null;
    creator?: string;
    worker?: string;
    created_at?: string | null;
    concluded_at?: string | null;
};

type GraphEdge = {
    from: string;
    to: string;
    kind: string;
};

type GraphSnapshot = {
    project: GraphProject;
    nodes: GraphNode[];
    edges: GraphEdge[];
    timeline: string;
};

type GraphPanelProps = {
    baseURL: string;
    authHeader: string;
};

type BadgeVariant = 'default' | 'primary' | 'secondary' | 'success' | 'warning' | 'destructive';

function factBadgeVariant(type: string): BadgeVariant {
    switch ((type || '').toLowerCase()) {
        case 'verdict':
            return 'destructive';
        case 'patch_profile':
            return 'primary';
        case 'exploit_idea':
            return 'warning';
        case 'observation':
            return 'secondary';
        default:
            return 'default';
    }
}

function intentBadgeVariant(state: string): BadgeVariant {
    switch ((state || '').toLowerCase()) {
        case 'open':
            return 'warning';
        case 'claimed':
        case 'running':
            return 'primary';
        case 'concluded':
            return 'success';
        default:
            return 'default';
    }
}

function formatTime(value?: string | null): string {
    if (!value) return '';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString();
}

function isJsonResponse(resp: Response): boolean {
    const contentType = resp.headers.get('content-type') || '';
    return contentType.toLowerCase().includes('application/json');
}

function contentClampStyle(lines: number): CSSProperties {
    return {
        display: '-webkit-box',
        WebkitLineClamp: lines,
        WebkitBoxOrient: 'vertical',
        overflow: 'hidden',
        wordBreak: 'break-word',
    };
}

function SectionTitle({ children, count }: { children: ReactNode; count?: number }) {
    return (
        <div className="flex items-center gap-2 mb-2">
            <span className="text-sm font-semibold tracking-wide">{children}</span>
            {typeof count === 'number' ? <Badge variant="secondary">{count}</Badge> : null}
        </div>
    );
}

export default function GraphPanel({ baseURL, authHeader }: GraphPanelProps) {
    const [projects, setProjects] = useState<GraphProject[]>([]);
    const [projectsLoading, setProjectsLoading] = useState<boolean>(false);
    const [laneUnavailable, setLaneUnavailable] = useState<boolean>(false);
    const [projectsError, setProjectsError] = useState<string>('');
    const [selectedId, setSelectedId] = useState<string>('');

    const [snapshot, setSnapshot] = useState<GraphSnapshot | null>(null);
    const [snapshotLoading, setSnapshotLoading] = useState<boolean>(false);
    const [snapshotError, setSnapshotError] = useState<string>('');
    const [reloadKey, setReloadKey] = useState<number>(0);

    // The graph protocol's GET /api/graph/projects returns a bare array, while
    // the snapshot endpoint uses the {success,result} envelope. Accept both so
    // the panel works against the existing protocol unchanged.
    useEffect(() => {
        if (!authHeader) return;
        const controller = new AbortController();
        let cancelled = false;

        async function loadProjects() {
            setProjectsLoading(true);
            setProjectsError('');
            try {
                const resp = await fetch(`${baseURL}/api/graph/projects`, {
                    headers: { Authorization: authHeader },
                    signal: controller.signal,
                });
                if (!isJsonResponse(resp)) {
                    if (!cancelled) setLaneUnavailable(true);
                    return;
                }
                let payload: any;
                try {
                    payload = await resp.json();
                } catch {
                    if (!cancelled) setLaneUnavailable(true);
                    return;
                }
                let list: GraphProject[] | null = null;
                if (Array.isArray(payload)) {
                    list = payload;
                } else if (payload && payload.success === true && Array.isArray(payload.result)) {
                    list = payload.result;
                }
                if (!list) {
                    if (!cancelled) setLaneUnavailable(true);
                    return;
                }
                if (!cancelled) {
                    setLaneUnavailable(false);
                    setProjects(list);
                    setSelectedId(prev => {
                        if (prev && list!.some(p => p.id === prev)) return prev;
                        return list![0]?.id || '';
                    });
                }
            } catch (e: any) {
                if (e?.name === 'AbortError' || cancelled) return;
                setLaneUnavailable(true);
                setProjectsError(String(e?.message || e));
            } finally {
                if (!cancelled) setProjectsLoading(false);
            }
        }

        void loadProjects();
        return () => {
            cancelled = true;
            controller.abort();
        };
    }, [baseURL, authHeader, reloadKey]);

    useEffect(() => {
        if (!authHeader || !selectedId) {
            setSnapshot(null);
            return;
        }
        const controller = new AbortController();
        let cancelled = false;

        async function loadSnapshot() {
            setSnapshotLoading(true);
            setSnapshotError('');
            try {
                const resp = await fetch(`${baseURL}/api/graph/projects/${encodeURIComponent(selectedId)}/snapshot`, {
                    headers: { Authorization: authHeader },
                    signal: controller.signal,
                });
                if (!isJsonResponse(resp)) {
                    throw new Error(`响应不是 JSON（HTTP ${resp.status}）`);
                }
                const payload: any = await resp.json();
                if (!payload || payload.success !== true || !payload.result) {
                    throw new Error(payload?.error || '快照加载失败');
                }
                if (!cancelled) setSnapshot(payload.result as GraphSnapshot);
            } catch (e: any) {
                if (e?.name === 'AbortError' || cancelled) return;
                setSnapshot(null);
                setSnapshotError(String(e?.message || e));
            } finally {
                if (!cancelled) setSnapshotLoading(false);
            }
        }

        void loadSnapshot();
        return () => {
            cancelled = true;
            controller.abort();
        };
    }, [baseURL, authHeader, selectedId, reloadKey]);

    const facts = useMemo(() => (snapshot?.nodes || []).filter(n => n.kind === 'fact'), [snapshot]);
    const intents = useMemo(() => (snapshot?.nodes || []).filter(n => n.kind === 'intent'), [snapshot]);
    const hints = useMemo(() => (snapshot?.nodes || []).filter(n => n.kind === 'hint'), [snapshot]);
    const edges = snapshot?.edges || [];

    const emptyState = (
        <div className="rounded-lg border border-border bg-background/20 p-6 text-center">
            <div className="text-sm font-semibold">图车道未启用或未配置</div>
            <div className="mt-2 text-xs text-muted-foreground">
                在设置中将 misc.graph_lane_enabled 设为 true 并重启
            </div>
            {projectsError ? (
                <div className="mt-2 text-xs text-destructive break-all">请求失败：{projectsError}</div>
            ) : null}
        </div>
    );

    return (
        <Card className="col-span-full">
            <CardHeader>
                <div className="flex flex-col sm:flex-row sm:items-center justify-between w-full gap-3">
                    <div>
                        <CardTitle>溯源图</CardTitle>
                        <div className="mt-1 text-xs text-muted-foreground">
                            只读视图 · 事实 / 意图 / 提示 / 边 / 时间线
                        </div>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                        <Badge variant="primary">{projects.length} 个项目</Badge>
                        <Button size="sm" variant="outline" onClick={() => setReloadKey(k => k + 1)} disabled={projectsLoading || snapshotLoading}>
                            {projectsLoading || snapshotLoading ? '加载中…' : '刷新'}
                        </Button>
                    </div>
                </div>
            </CardHeader>
            <CardContent>
                {laneUnavailable ? emptyState : (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 16, alignItems: 'flex-start' }}>
                        <div style={{ flex: '0 0 240px', minWidth: 200, maxWidth: '100%' }}>
                            <SectionTitle count={projects.length}>项目</SectionTitle>
                            <ScrollArea className="h-[60vh] rounded-lg border border-border bg-background/20">
                                <div className="p-2 space-y-2">
                                    {projects.map(p => {
                                        const active = p.id === selectedId;
                                        return (
                                            <button
                                                key={p.id}
                                                className={`w-full text-left rounded-lg border px-3 py-2 transition-colors ${active ? 'border-primary bg-primary/10' : 'border-border bg-background/10 hover:bg-muted/30'}`}
                                                onClick={() => setSelectedId(p.id)}
                                                title={`${p.id} ${p.title || ''}`}
                                            >
                                                <div className="text-sm font-semibold truncate">{p.title || p.id}</div>
                                                <div className="mt-1 flex items-center gap-2 flex-wrap">
                                                    <Badge variant={p.status === 'open' ? 'warning' : 'secondary'}>{p.status || 'unknown'}</Badge>
                                                    <span className="text-[11px] text-muted-foreground font-mono truncate">{p.id}</span>
                                                </div>
                                            </button>
                                        );
                                    })}
                                    {projects.length === 0 ? (
                                        <div className="text-sm text-muted-foreground p-2">暂无项目</div>
                                    ) : null}
                                </div>
                            </ScrollArea>
                        </div>

                        <div style={{ flex: '1 1 420px', minWidth: 0 }}>
                            {snapshotLoading ? (
                                <div className="text-sm text-muted-foreground p-2">加载中…</div>
                            ) : snapshotError ? (
                                <div className="rounded-lg border border-border bg-background/20 p-4 text-sm text-destructive break-all">
                                    快照加载失败：{snapshotError}
                                </div>
                            ) : !snapshot ? (
                                <div className="rounded-lg border border-border bg-background/20 p-4 text-sm text-muted-foreground">
                                    请选择左侧项目查看溯源图
                                </div>
                            ) : (
                                <div className="space-y-4">
                                    <div className="rounded-lg border border-border bg-background/20 p-4">
                                        <div className="flex items-center gap-2 flex-wrap">
                                            <span className="text-base font-semibold break-all">{snapshot.project.title || snapshot.project.id}</span>
                                            <Badge variant="primary">{snapshot.project.status || 'unknown'}</Badge>
                                        </div>
                                        <div className="mt-1 text-[11px] font-mono text-muted-foreground break-all">{snapshot.project.id}</div>
                                        <div className="mt-2 text-xs text-muted-foreground break-all">
                                            <span className="font-semibold">来源：</span>{snapshot.project.origin || '—'}
                                        </div>
                                        <div className="mt-1 text-xs text-muted-foreground break-all" style={{ whiteSpace: 'pre-wrap' }}>
                                            <span className="font-semibold">目标：</span>{snapshot.project.goal || '—'}
                                        </div>
                                        <div className="mt-2 flex items-center gap-2 flex-wrap">
                                            <Badge variant="secondary">事实 {facts.length}</Badge>
                                            <Badge variant="secondary">意图 {intents.length}</Badge>
                                            <Badge variant="secondary">提示 {hints.length}</Badge>
                                            <Badge variant="secondary">边 {edges.length}</Badge>
                                        </div>
                                    </div>

                                    <div className="rounded-lg border border-border bg-background/20 p-4">
                                        <SectionTitle count={facts.length}>事实</SectionTitle>
                                        <div className="space-y-2">
                                            {facts.map(f => (
                                                <div key={f.id} className="rounded-md border border-border/70 bg-background/20 px-3 py-2">
                                                    <div className="flex items-center gap-2 flex-wrap">
                                                        <Badge variant={factBadgeVariant(f.type)}>{f.type || 'fact'}</Badge>
                                                        <span className="text-[11px] font-mono text-muted-foreground break-all">{f.id}</span>
                                                        <span className="text-[11px] text-muted-foreground">{formatTime(f.created_at)}</span>
                                                    </div>
                                                    <div className="mt-1 text-xs" style={contentClampStyle(3)} title={f.content}>{f.content || '—'}</div>
                                                </div>
                                            ))}
                                            {facts.length === 0 ? <div className="text-xs text-muted-foreground">暂无事实</div> : null}
                                        </div>
                                    </div>

                                    <div className="rounded-lg border border-border bg-background/20 p-4">
                                        <SectionTitle count={intents.length}>意图</SectionTitle>
                                        <div className="space-y-2">
                                            {intents.map(it => (
                                                <div key={it.id} className="rounded-md border border-border/70 bg-background/20 px-3 py-2">
                                                    <div className="flex items-center gap-2 flex-wrap">
                                                        <Badge variant={intentBadgeVariant(it.state || '')}>{it.state || 'unknown'}</Badge>
                                                        <Badge>{it.type || 'intent'}</Badge>
                                                        <span className="text-[11px] font-mono text-muted-foreground break-all">{it.id}</span>
                                                    </div>
                                                    <div className="mt-1 text-xs" style={contentClampStyle(3)} title={it.content}>{it.content || '—'}</div>
                                                    <div className="mt-1 text-[11px] text-muted-foreground break-all" style={{ whiteSpace: 'pre-wrap' }}>
                                                        {(it.from || []).join(', ') || '—'} → {it.to || '—'}
                                                        {it.worker ? ` · worker: ${it.worker}` : ''}
                                                    </div>
                                                </div>
                                            ))}
                                            {intents.length === 0 ? <div className="text-xs text-muted-foreground">暂无意图</div> : null}
                                        </div>
                                    </div>

                                    <div className="rounded-lg border border-border bg-background/20 p-4">
                                        <SectionTitle count={hints.length}>提示</SectionTitle>
                                        <div className="space-y-2">
                                            {hints.map(h => (
                                                <div key={h.id} className="rounded-md border border-border/70 bg-background/20 px-3 py-2">
                                                    <div className="flex items-center gap-2 flex-wrap">
                                                        <Badge variant="secondary">{h.creator || 'unknown'}</Badge>
                                                        <span className="text-[11px] font-mono text-muted-foreground break-all">{h.id}</span>
                                                        <span className="text-[11px] text-muted-foreground">{formatTime(h.created_at)}</span>
                                                    </div>
                                                    <div className="mt-1 text-xs" style={contentClampStyle(3)} title={h.content}>{h.content || '—'}</div>
                                                </div>
                                            ))}
                                            {hints.length === 0 ? <div className="text-xs text-muted-foreground">暂无提示</div> : null}
                                        </div>
                                    </div>

                                    <div className="rounded-lg border border-border bg-background/20 p-4">
                                        <SectionTitle count={edges.length}>边</SectionTitle>
                                        <div className="space-y-1">
                                            {edges.map((e, idx) => (
                                                <div key={`${e.from}-${e.to}-${e.kind}-${idx}`} className="text-xs font-mono break-all text-muted-foreground">
                                                    {e.from} → {e.to} <span className="text-foreground">({e.kind})</span>
                                                </div>
                                            ))}
                                            {edges.length === 0 ? <div className="text-xs text-muted-foreground">暂无边</div> : null}
                                        </div>
                                    </div>

                                    <div className="rounded-lg border border-border bg-background/20 p-4">
                                        <SectionTitle>时间线</SectionTitle>
                                        <pre
                                            style={{
                                                margin: 0,
                                                maxHeight: 360,
                                                overflow: 'auto',
                                                whiteSpace: 'pre-wrap',
                                                wordBreak: 'break-word',
                                                fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
                                                fontSize: 12,
                                                lineHeight: 1.6,
                                            }}
                                        >
                                            {snapshot.timeline || '（空）'}
                                        </pre>
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                )}
            </CardContent>
        </Card>
    );
}
