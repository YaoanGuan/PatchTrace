import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from './components/ui/card';
import { Badge } from './components/ui/badge';
import { Button } from './components/ui/button';
import { Input } from './components/ui/input';
import { Textarea } from './components/ui/textarea';
import { ScrollArea } from './components/ui/scroll-area';

// GraphPanel drives the whole graph protocol exposed by the graph lane under
// /api/graph: projects, hints, intents (claim/heartbeat/release/conclude),
// reason leases, completion/reopen, export and provenance. Reads and writes
// use the same bearer/basic auth header as the rest of the app.

type GraphReason = {
    worker: string;
    trigger: string;
    started_at: string;
    last_heartbeat_at: string;
};

type GraphProject = {
    id: string;
    title: string;
    origin: string;
    goal: string;
    status: string;
    bootstrap_enabled?: boolean;
    created_at?: string;
    reason?: GraphReason | null;
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
    lease_until?: string | null;
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

type JSONResult = { ok: true; data: any } | { ok: false; error: string };

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
        case 'resolved':
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

function FieldLabel({ children }: { children: ReactNode }) {
    return <div className="mb-1 text-xs font-medium text-muted-foreground">{children}</div>;
}

// requestJSON normalizes protocol replies: graph handlers return bare objects
// on success and {error} on failure; the snapshot handler returns an envelope.
async function requestJSON(url: string, init?: RequestInit): Promise<JSONResult> {
    try {
        const resp = await fetch(url, init);
        const text = await resp.text();
        let payload: any = null;
        if (text) {
            try {
                payload = JSON.parse(text);
            } catch {
                payload = null;
            }
        }
        if (!resp.ok) {
            const message = payload && typeof payload.error === 'string'
                ? payload.error
                : `HTTP ${resp.status}`;
            return { ok: false, error: message };
        }
        if (payload && payload.success === false) {
            return { ok: false, error: String(payload.error || '请求失败') };
        }
        return { ok: true, data: payload };
    } catch (e: any) {
        return { ok: false, error: String(e?.message || e) };
    }
}

function toggleValue(values: string[], value: string): string[] {
    return values.includes(value) ? values.filter(v => v !== value) : [...values, value];
}

function FactPicker({
    facts,
    selected,
    onToggle,
}: {
    facts: GraphNode[];
    selected: string[];
    onToggle: (id: string) => void;
}) {
    if (facts.length === 0) {
        return <div className="text-xs text-muted-foreground">暂无可用事实（需要先写入事实）</div>;
    }
    return (
        <div className="max-h-32 space-y-1 overflow-auto rounded-md border border-border/70 bg-background/20 p-2">
            {facts.map(f => (
                <label key={f.id} className="flex items-start gap-2 text-xs">
                    <input
                        type="checkbox"
                        className="mt-0.5"
                        checked={selected.includes(f.id)}
                        onChange={() => onToggle(f.id)}
                    />
                    <span className="font-mono text-muted-foreground">{f.id}</span>
                    <span className="truncate" title={f.content}>{f.type} · {f.content}</span>
                </label>
            ))}
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

    const [actionBusy, setActionBusy] = useState<string>('');
    const [actionError, setActionError] = useState<string>('');
    const [actionNote, setActionNote] = useState<string>('');

    // Shared operator fields: creators write facts/hints, workers hold leases.
    const [creator, setCreator] = useState<string>('ui');
    const [worker, setWorker] = useState<string>('ui');
    const [leaseSeconds, setLeaseSeconds] = useState<string>('300');

    const [showNewProject, setShowNewProject] = useState<boolean>(false);
    const [newProject, setNewProject] = useState({ id: '', title: '', origin: '', goal: '', bootstrap: false });

    const [hintText, setHintText] = useState<string>('');
    const [intentType, setIntentType] = useState<string>('gap');
    const [intentText, setIntentText] = useState<string>('');
    const [intentFrom, setIntentFrom] = useState<string[]>([]);

    const [showComplete, setShowComplete] = useState<boolean>(false);
    const [completeText, setCompleteText] = useState<string>('');
    const [completeFrom, setCompleteFrom] = useState<string[]>(['origin']);
    const [showReopen, setShowReopen] = useState<boolean>(false);
    const [reopenText, setReopenText] = useState<string>('');

    const [reasonTrigger, setReasonTrigger] = useState<string>('manual');

    const [concludeFor, setConcludeFor] = useState<string>('');
    const [concludeText, setConcludeText] = useState<string>('');
    const [concludeType, setConcludeType] = useState<string>('observation');

    const [chain, setChain] = useState<GraphNode[] | null>(null);
    const [chainError, setChainError] = useState<string>('');

    const graphURL = useMemo(() => (path: string) => `${baseURL}/api/graph${path}`, [baseURL]);
    const jsonHeaders = useMemo(
        () => ({ Authorization: authHeader, 'Content-Type': 'application/json' }),
        [authHeader],
    );
    const getHeaders = useMemo(() => ({ Authorization: authHeader }), [authHeader]);

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
                if (resp.status === 401 || resp.status === 403) {
                    if (!cancelled) {
                        setProjectsError(`鉴权失败（HTTP ${resp.status}）`);
                        setProjects([]);
                    }
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
                    setProjectsError('');
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
            // Reset the in-flight flags too: the previous request's cleanup
            // skips them once aborted, so leaving them set would pin the pane
            // on "加载中…" after the project list empties.
            setSnapshot(null);
            setSnapshotLoading(false);
            setSnapshotError('');
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

    const facts = useMemo(
        () => (snapshot?.nodes || []).filter(n => n.kind === 'fact' && n.type !== 'goal'),
        [snapshot],
    );
    const intents = useMemo(() => (snapshot?.nodes || []).filter(n => n.kind === 'intent'), [snapshot]);
    const hints = useMemo(() => (snapshot?.nodes || []).filter(n => n.kind === 'hint'), [snapshot]);
    const edges = snapshot?.edges || [];

    function afterMutation(note: string) {
        setActionNote(note);
        setReloadKey(k => k + 1);
    }

    async function createProject() {
        const title = newProject.title.trim();
        const origin = newProject.origin.trim();
        const goal = newProject.goal.trim();
        if (!title || !origin || !goal) {
            setActionError('新建项目需要填写标题、来源和目标');
            return;
        }
        setActionBusy('create');
        setActionError('');
        setActionNote('');
        const body: Record<string, any> = { title, origin, goal, bootstrap_enabled: newProject.bootstrap };
        if (newProject.id.trim()) body.id = newProject.id.trim();
        const res = await requestJSON(graphURL('/projects'), {
            method: 'POST',
            headers: jsonHeaders,
            body: JSON.stringify(body),
        });
        setActionBusy('');
        if (!res.ok) {
            setActionError(res.error);
            return;
        }
        const created = res.data as GraphProject | null;
        setShowNewProject(false);
        setNewProject({ id: '', title: '', origin: '', goal: '', bootstrap: false });
        if (created?.id) setSelectedId(created.id);
        afterMutation(`项目已创建：${created?.id || ''}`);
    }

    async function setProjectStatus(status: 'active' | 'stopped') {
        if (!selectedId) return;
        setActionBusy('status');
        setActionError('');
        setActionNote('');
        const res = await requestJSON(graphURL(`/projects/${encodeURIComponent(selectedId)}/status`), {
            method: 'PUT',
            headers: jsonHeaders,
            body: JSON.stringify({ status }),
        });
        setActionBusy('');
        if (!res.ok) {
            setActionError(res.error);
            return;
        }
        afterMutation(`项目状态已更新为 ${status}`);
    }

    async function reopenProject() {
        if (!selectedId) return;
        const description = reopenText.trim();
        const who = creator.trim();
        if (!description || !who) {
            setActionError('重开项目需要填写操作者和说明');
            return;
        }
        setActionBusy('reopen');
        setActionError('');
        setActionNote('');
        const res = await requestJSON(graphURL(`/projects/${encodeURIComponent(selectedId)}/reopen`), {
            method: 'POST',
            headers: jsonHeaders,
            body: JSON.stringify({ creator: who, description }),
        });
        setActionBusy('');
        if (!res.ok) {
            setActionError(res.error);
            return;
        }
        setShowReopen(false);
        setReopenText('');
        afterMutation('项目已重开');
    }

    async function completeProject() {
        if (!selectedId) return;
        const description = completeText.trim();
        const who = worker.trim();
        if (!description || !who) {
            setActionError('完成项目需要填写操作者和结论说明');
            return;
        }
        if (completeFrom.length === 0) {
            setActionError('完成项目至少选择一个依据事实');
            return;
        }
        setActionBusy('complete');
        setActionError('');
        setActionNote('');
        const res = await requestJSON(graphURL(`/projects/${encodeURIComponent(selectedId)}/complete`), {
            method: 'POST',
            headers: jsonHeaders,
            body: JSON.stringify({ from: completeFrom, description, worker: who }),
        });
        setActionBusy('');
        if (!res.ok) {
            setActionError(res.error);
            return;
        }
        setShowComplete(false);
        setCompleteText('');
        afterMutation('项目已完成');
    }

    async function addHint() {
        if (!selectedId) return;
        const content = hintText.trim();
        const who = creator.trim();
        if (!content || !who) {
            setActionError('写入提示需要填写内容和创建者');
            return;
        }
        setActionBusy('hint');
        setActionError('');
        setActionNote('');
        const res = await requestJSON(graphURL(`/projects/${encodeURIComponent(selectedId)}/hints`), {
            method: 'POST',
            headers: jsonHeaders,
            body: JSON.stringify({ content, creator: who }),
        });
        setActionBusy('');
        if (!res.ok) {
            setActionError(res.error);
            return;
        }
        setHintText('');
        afterMutation('提示已写入');
    }

    async function declareIntent() {
        if (!selectedId) return;
        const content = intentText.trim();
        const who = creator.trim();
        if (!content || !who) {
            setActionError('声明意图需要填写内容和创建者');
            return;
        }
        if (intentFrom.length === 0) {
            setActionError('声明意图至少选择一个依据事实');
            return;
        }
        setActionBusy('intent');
        setActionError('');
        setActionNote('');
        const res = await requestJSON(graphURL(`/projects/${encodeURIComponent(selectedId)}/intents`), {
            method: 'POST',
            headers: jsonHeaders,
            body: JSON.stringify({ type: intentType || 'gap', content, from: intentFrom, creator: who }),
        });
        setActionBusy('');
        if (!res.ok) {
            setActionError(res.error);
            return;
        }
        setIntentText('');
        setIntentFrom([]);
        afterMutation('意图已声明');
    }

    async function intentLease(intentId: string, action: 'heartbeat' | 'release') {
        if (!selectedId) return;
        const who = worker.trim();
        if (!who) {
            setActionError('需要填写 worker');
            return;
        }
        setActionBusy(`intent-${action}-${intentId}`);
        setActionError('');
        setActionNote('');
        const path = `/projects/${encodeURIComponent(selectedId)}/intents/${encodeURIComponent(intentId)}/${action}`;
        const body = action === 'heartbeat'
            ? { worker: who, lease_seconds: Number(leaseSeconds) || 0 }
            : { worker: who };
        const res = await requestJSON(graphURL(path), {
            method: 'POST',
            headers: jsonHeaders,
            body: JSON.stringify(body),
        });
        setActionBusy('');
        if (!res.ok) {
            setActionError(res.error);
            return;
        }
        afterMutation(action === 'heartbeat' ? `意图 ${intentId} 已领取/续约` : `意图 ${intentId} 已释放`);
    }

    async function concludeIntent(intentId: string) {
        if (!selectedId) return;
        const description = concludeText.trim();
        const who = worker.trim();
        if (!description || !who) {
            setActionError('总结意图需要填写 worker 和结论说明');
            return;
        }
        setActionBusy(`intent-conclude-${intentId}`);
        setActionError('');
        setActionNote('');
        const res = await requestJSON(graphURL(`/projects/${encodeURIComponent(selectedId)}/intents/${encodeURIComponent(intentId)}/conclude`), {
            method: 'POST',
            headers: jsonHeaders,
            body: JSON.stringify({ worker: who, description, fact_type: concludeType || 'observation' }),
        });
        setActionBusy('');
        if (!res.ok) {
            setActionError(res.error);
            return;
        }
        setConcludeFor('');
        setConcludeText('');
        afterMutation(`意图 ${intentId} 已总结`);
    }

    async function reasonAction(action: 'claim' | 'heartbeat' | 'release') {
        if (!selectedId) return;
        const who = worker.trim();
        if (!who) {
            setActionError('需要填写 worker');
            return;
        }
        const body: Record<string, any> = { worker: who };
        if (action === 'claim') {
            const trigger = reasonTrigger.trim();
            if (!trigger) {
                setActionError('领取总脑需要填写 trigger');
                return;
            }
            body.trigger = trigger;
            body.lease_seconds = Number(leaseSeconds) || 0;
        }
        setActionBusy(`reason-${action}`);
        setActionError('');
        setActionNote('');
        const res = await requestJSON(graphURL(`/projects/${encodeURIComponent(selectedId)}/reason/${action}`), {
            method: 'POST',
            headers: jsonHeaders,
            body: JSON.stringify(body),
        });
        setActionBusy('');
        if (!res.ok) {
            setActionError(res.error);
            return;
        }
        afterMutation(action === 'claim' ? '总脑租约已领取' : action === 'heartbeat' ? '总脑租约已续约' : '总脑租约已释放');
    }

    async function loadChain(nodeId: string) {
        if (!selectedId) return;
        setChainError('');
        setChain(null);
        const res = await requestJSON(
            `${graphURL(`/projects/${encodeURIComponent(selectedId)}/provenance`)}?node=${encodeURIComponent(nodeId)}`,
            { headers: getHeaders },
        );
        if (!res.ok) {
            setChainError(res.error);
            return;
        }
        setChain(Array.isArray(res.data) ? res.data : []);
    }

    async function exportProject(format: 'yaml' | 'timeline') {
        if (!selectedId) return;
        setActionBusy(`export-${format}`);
        setActionError('');
        setActionNote('');
        try {
            const resp = await fetch(graphURL(`/projects/${encodeURIComponent(selectedId)}/export?format=${format}`), {
                headers: { Authorization: authHeader },
            });
            if (!resp.ok) {
                const text = await resp.text();
                throw new Error(text.slice(0, 300) || `HTTP ${resp.status}`);
            }
            const blob = await resp.blob();
            const href = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = href;
            a.download = format === 'yaml' ? `${selectedId}.yaml` : `${selectedId}-timeline.txt`;
            document.body.appendChild(a);
            a.click();
            a.remove();
            setTimeout(() => URL.revokeObjectURL(href), 1000);
            setActionNote(`已导出 ${format === 'yaml' ? 'YAML' : '时间线'}`);
        } catch (e: any) {
            setActionError(String(e?.message || e));
        } finally {
            setActionBusy('');
        }
    }

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

    const busy = actionBusy !== '';

    return (
        <Card className="col-span-full">
            <CardHeader>
                <div className="flex flex-col sm:flex-row sm:items-center justify-between w-full gap-3">
                    <div>
                        <CardTitle>溯源图</CardTitle>
                        <div className="mt-1 text-xs text-muted-foreground">
                            图协议面板 · 事实 / 意图 / 提示 / 边 / 时间线
                        </div>
                    </div>
                    <div className="flex items-center gap-2 shrink-0 flex-wrap">
                        <Badge variant={laneUnavailable ? 'destructive' : 'success'}>
                            {laneUnavailable ? '图 API 不可用' : '图 API 可用'}
                        </Badge>
                        <Badge variant="primary">{projects.length} 个项目</Badge>
                        <Button size="sm" variant="outline" disabled={busy || !selectedId} onClick={() => void exportProject('yaml')}>
                            导出 YAML
                        </Button>
                        <Button size="sm" variant="outline" disabled={busy || !selectedId} onClick={() => void exportProject('timeline')}>
                            导出时间线
                        </Button>
                        <Button size="sm" variant="outline" onClick={() => setReloadKey(k => k + 1)} disabled={projectsLoading || snapshotLoading}>
                            {projectsLoading || snapshotLoading ? '加载中…' : '刷新'}
                        </Button>
                    </div>
                </div>
            </CardHeader>
            <CardContent>
                {actionError ? (
                    <div className="mb-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive break-all">
                        操作失败：{actionError}
                    </div>
                ) : null}
                {actionNote ? (
                    <div className="mb-3 rounded-lg border border-border bg-background/30 px-3 py-2 text-xs text-muted-foreground break-all">
                        {actionNote}
                    </div>
                ) : null}
                {laneUnavailable ? emptyState : (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 16, alignItems: 'flex-start' }}>
                        <div style={{ flex: '0 0 260px', minWidth: 220, maxWidth: '100%' }}>
                            <div className="mb-2 flex items-center justify-between gap-2">
                                <SectionTitle count={projects.length}>项目</SectionTitle>
                                <Button size="sm" variant="outline" onClick={() => setShowNewProject(v => !v)}>
                                    {showNewProject ? '取消' : '新建'}
                                </Button>
                            </div>
                            {showNewProject ? (
                                <div className="mb-3 space-y-2 rounded-lg border border-border bg-background/20 p-3">
                                    <div>
                                        <FieldLabel>项目 ID（留空自动生成）</FieldLabel>
                                        <Input className="h-8 text-xs" value={newProject.id}
                                            onChange={e => setNewProject(v => ({ ...v, id: e.target.value }))} />
                                    </div>
                                    <div>
                                        <FieldLabel>标题 *</FieldLabel>
                                        <Input className="h-8 text-xs" value={newProject.title}
                                            onChange={e => setNewProject(v => ({ ...v, title: e.target.value }))} />
                                    </div>
                                    <div>
                                        <FieldLabel>来源（commit/补丁）*</FieldLabel>
                                        <Input className="h-8 text-xs" value={newProject.origin}
                                            onChange={e => setNewProject(v => ({ ...v, origin: e.target.value }))} />
                                    </div>
                                    <div>
                                        <FieldLabel>目标 *</FieldLabel>
                                        <Textarea className="min-h-[64px] text-xs" value={newProject.goal}
                                            onChange={e => setNewProject(v => ({ ...v, goal: e.target.value }))} />
                                    </div>
                                    <label className="flex items-center gap-2 text-xs">
                                        <input type="checkbox" checked={newProject.bootstrap}
                                            onChange={e => setNewProject(v => ({ ...v, bootstrap: e.target.checked }))} />
                                        创建后触发 bootstrap（需配置模型）
                                    </label>
                                    <Button size="sm" disabled={busy} onClick={() => void createProject()}>
                                        {actionBusy === 'create' ? '创建中…' : '创建项目'}
                                    </Button>
                                </div>
                            ) : null}
                            <div className="mb-2 grid grid-cols-2 gap-2">
                                <div>
                                    <FieldLabel>操作者（creator）</FieldLabel>
                                    <Input className="h-8 text-xs" value={creator} onChange={e => setCreator(e.target.value)} />
                                </div>
                                <div>
                                    <FieldLabel>worker</FieldLabel>
                                    <Input className="h-8 text-xs" value={worker} onChange={e => setWorker(e.target.value)} />
                                </div>
                            </div>
                            <ScrollArea className="h-[52vh] rounded-lg border border-border bg-background/20">
                                <div className="p-2 space-y-2">
                                    {projects.map(p => {
                                        const active = p.id === selectedId;
                                        return (
                                            <button
                                                key={p.id}
                                                className={`w-full text-left rounded-lg border px-3 py-2 transition-colors ${active ? 'border-primary bg-primary/10' : 'border-border bg-background/10 hover:bg-muted/30'}`}
                                                onClick={() => { setSelectedId(p.id); setChain(null); setChainError(''); }}
                                                title={`${p.id} ${p.title || ''}`}
                                            >
                                                <div className="text-sm font-semibold truncate">{p.title || p.id}</div>
                                                <div className="mt-1 flex items-center gap-2 flex-wrap">
                                                    <Badge variant={p.status === 'active' ? 'warning' : p.status === 'completed' ? 'success' : 'secondary'}>{p.status || 'unknown'}</Badge>
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

                        <div style={{ flex: '1 1 520px', minWidth: 0 }}>
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
                                    {chainError ? (
                                        <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-xs text-destructive break-all">
                                            溯源链加载失败：{chainError}
                                        </div>
                                    ) : null}
                                    {chain ? (
                                        <div className="rounded-lg border border-primary/40 bg-primary/5 p-4">
                                            <div className="mb-2 flex items-center justify-between gap-2">
                                                <SectionTitle count={chain.length}>溯源链</SectionTitle>
                                                <Button size="sm" variant="ghost" onClick={() => setChain(null)}>关闭</Button>
                                            </div>
                                            <div className="space-y-1">
                                                {chain.map((n, idx) => (
                                                    <div key={`${n.id}-${idx}`} className="text-xs">
                                                        <span className="font-mono text-muted-foreground">{n.id}</span>
                                                        {' '}
                                                        <Badge variant={n.kind === 'hint' ? 'secondary' : factBadgeVariant(n.type)}>{n.type || n.kind}</Badge>
                                                        {' '}
                                                        <span className="break-all">{n.content}</span>
                                                    </div>
                                                ))}
                                                {chain.length === 0 ? <div className="text-xs text-muted-foreground">链为空</div> : null}
                                            </div>
                                        </div>
                                    ) : null}

                                    <div className="rounded-lg border border-border bg-background/20 p-4">
                                        <div className="flex items-center gap-2 flex-wrap">
                                            <span className="text-base font-semibold break-all">{snapshot.project.title || snapshot.project.id}</span>
                                            <Badge variant={snapshot.project.status === 'active' ? 'warning' : snapshot.project.status === 'completed' ? 'success' : 'secondary'}>
                                                {snapshot.project.status || 'unknown'}
                                            </Badge>
                                            <Badge variant={snapshot.project.bootstrap_enabled ? 'primary' : 'secondary'}>
                                                bootstrap {snapshot.project.bootstrap_enabled ? '开启' : '关闭'}
                                            </Badge>
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
                                        <div className="mt-3 flex flex-wrap items-center gap-2">
                                            {snapshot.project.status !== 'completed' ? (
                                                <Button
                                                    size="sm"
                                                    variant={snapshot.project.status === 'active' ? 'outline' : 'default'}
                                                    disabled={busy}
                                                    onClick={() => void setProjectStatus(snapshot.project.status === 'active' ? 'stopped' : 'active')}
                                                >
                                                    {snapshot.project.status === 'active' ? '暂停项目' : '恢复项目'}
                                                </Button>
                                            ) : null}
                                            {snapshot.project.status === 'active' ? (
                                                <Button size="sm" variant="outline" onClick={() => setShowComplete(v => !v)}>
                                                    {showComplete ? '取消完成' : '完成项目'}
                                                </Button>
                                            ) : null}
                                            {snapshot.project.status === 'completed' ? (
                                                <Button size="sm" variant="outline" onClick={() => setShowReopen(v => !v)}>
                                                    {showReopen ? '取消重开' : '重开项目'}
                                                </Button>
                                            ) : null}
                                            <div className="flex items-center gap-1">
                                                <span className="text-xs text-muted-foreground">租约(秒)</span>
                                                <Input className="h-8 w-20 text-xs" value={leaseSeconds}
                                                    onChange={e => setLeaseSeconds(e.target.value)} />
                                            </div>
                                        </div>

                                        {showComplete ? (
                                            <div className="mt-3 space-y-2 rounded-md border border-border/70 bg-background/20 p-3">
                                                <FieldLabel>完成结论（说明）*</FieldLabel>
                                                <Textarea className="min-h-[64px] text-xs" value={completeText}
                                                    onChange={e => setCompleteText(e.target.value)} />
                                                <FieldLabel>依据事实 *</FieldLabel>
                                                <FactPicker facts={facts} selected={completeFrom}
                                                    onToggle={id => setCompleteFrom(v => toggleValue(v, id))} />
                                                <Button size="sm" disabled={busy} onClick={() => void completeProject()}>
                                                    {actionBusy === 'complete' ? '提交中…' : '提交完成'}
                                                </Button>
                                            </div>
                                        ) : null}

                                        {showReopen ? (
                                            <div className="mt-3 space-y-2 rounded-md border border-border/70 bg-background/20 p-3">
                                                <FieldLabel>重开说明 *（操作者取左侧 creator）</FieldLabel>
                                                <Textarea className="min-h-[64px] text-xs" value={reopenText}
                                                    onChange={e => setReopenText(e.target.value)} />
                                                <Button size="sm" disabled={busy} onClick={() => void reopenProject()}>
                                                    {actionBusy === 'reopen' ? '提交中…' : '提交重开'}
                                                </Button>
                                            </div>
                                        ) : null}
                                    </div>

                                    <div className="rounded-lg border border-border bg-background/20 p-4">
                                        <SectionTitle>总脑租约</SectionTitle>
                                        {snapshot.project.reason ? (
                                            <div className="space-y-2">
                                                <div className="text-xs text-muted-foreground break-all">
                                                    worker <span className="font-mono">{snapshot.project.reason.worker}</span>
                                                    {' · '}trigger <span className="font-mono">{snapshot.project.reason.trigger}</span>
                                                </div>
                                                <div className="text-[11px] text-muted-foreground">
                                                    开始 {formatTime(snapshot.project.reason.started_at)} · 最近心跳 {formatTime(snapshot.project.reason.last_heartbeat_at)}
                                                </div>
                                                <div className="flex gap-2">
                                                    <Button size="sm" variant="outline" disabled={busy}
                                                        onClick={() => void reasonAction('heartbeat')}>心跳</Button>
                                                    <Button size="sm" variant="outline" disabled={busy}
                                                        onClick={() => void reasonAction('release')}>释放</Button>
                                                </div>
                                            </div>
                                        ) : (
                                            <div className="flex flex-wrap items-end gap-2">
                                                <div>
                                                    <FieldLabel>trigger</FieldLabel>
                                                    <Input className="h-8 w-40 text-xs" value={reasonTrigger}
                                                        onChange={e => setReasonTrigger(e.target.value)} />
                                                </div>
                                                <Button size="sm" disabled={busy} onClick={() => void reasonAction('claim')}>
                                                    {actionBusy === 'reason-claim' ? '领取中…' : '领取总脑'}
                                                </Button>
                                                <span className="text-[11px] text-muted-foreground">worker 取左侧输入</span>
                                            </div>
                                        )}
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
                                                        <Button size="sm" variant="ghost" className="ml-auto"
                                                            onClick={() => void loadChain(f.id)}>溯源</Button>
                                                    </div>
                                                    <div className="mt-1 text-xs" style={contentClampStyle(3)} title={f.content}>{f.content || '—'}</div>
                                                    {f.from && f.from.length > 0 ? (
                                                        <div className="mt-1 text-[11px] font-mono text-muted-foreground break-all">
                                                            from: {f.from.join(', ')}
                                                        </div>
                                                    ) : null}
                                                </div>
                                            ))}
                                            {facts.length === 0 ? <div className="text-xs text-muted-foreground">暂无事实</div> : null}
                                        </div>
                                        <div className="mt-4 space-y-2 rounded-md border border-border/70 bg-background/20 p-3">
                                            <FieldLabel>写入提示（Hint）</FieldLabel>
                                            <Textarea className="min-h-[64px] text-xs" value={hintText}
                                                onChange={e => setHintText(e.target.value)} placeholder="线索、补丁观察、后续方向…" />
                                            <Button size="sm" disabled={busy} onClick={() => void addHint()}>
                                                {actionBusy === 'hint' ? '写入中…' : '写入提示'}
                                            </Button>
                                        </div>
                                    </div>

                                    <div className="rounded-lg border border-border bg-background/20 p-4">
                                        <SectionTitle count={intents.length}>意图</SectionTitle>
                                        <div className="space-y-2">
                                            {intents.map(it => {
                                                const claimed = (it.state || '') === 'claimed' || (it.state || '') === 'running';
                                                const open = (it.state || '') === 'open';
                                                const resolved = (it.state || '') === 'resolved' || (it.state || '') === 'concluded';
                                                return (
                                                    <div key={it.id} className="rounded-md border border-border/70 bg-background/20 px-3 py-2">
                                                        <div className="flex items-center gap-2 flex-wrap">
                                                            <Badge variant={intentBadgeVariant(it.state || '')}>{it.state || 'unknown'}</Badge>
                                                            <Badge>{it.type || 'intent'}</Badge>
                                                            <span className="text-[11px] font-mono text-muted-foreground break-all">{it.id}</span>
                                                            {it.worker ? <span className="text-[11px] text-muted-foreground">worker: {it.worker}</span> : null}
                                                            {it.lease_until ? <span className="text-[11px] text-muted-foreground">租约至 {formatTime(it.lease_until)}</span> : null}
                                                        </div>
                                                        <div className="mt-1 text-xs" style={contentClampStyle(3)} title={it.content}>{it.content || '—'}</div>
                                                        <div className="mt-1 text-[11px] text-muted-foreground break-all" style={{ whiteSpace: 'pre-wrap' }}>
                                                            {(it.from || []).join(', ') || '—'} → {it.to || '—'}
                                                        </div>
                                                        <div className="mt-2 flex flex-wrap items-center gap-2">
                                                            {open ? (
                                                                <Button size="sm" variant="outline" disabled={busy}
                                                                    onClick={() => void intentLease(it.id, 'heartbeat')}>
                                                                    领取（heartbeat）
                                                                </Button>
                                                            ) : null}
                                                            {claimed ? (
                                                                <>
                                                                    <Button size="sm" variant="outline" disabled={busy}
                                                                        onClick={() => void intentLease(it.id, 'heartbeat')}>心跳</Button>
                                                                    <Button size="sm" variant="outline" disabled={busy}
                                                                        onClick={() => void intentLease(it.id, 'release')}>释放</Button>
                                                                    <Button size="sm" disabled={busy}
                                                                        onClick={() => setConcludeFor(v => v === it.id ? '' : it.id)}>
                                                                        {concludeFor === it.id ? '取消总结' : '总结'}
                                                                    </Button>
                                                                </>
                                                            ) : null}
                                                            {resolved ? (
                                                                <span className="text-[11px] text-muted-foreground">已总结 → {it.to || '—'}</span>
                                                            ) : null}
                                                        </div>
                                                        {concludeFor === it.id ? (
                                                            <div className="mt-2 space-y-2 rounded-md border border-border/70 bg-background/20 p-2">
                                                                <FieldLabel>结论（将写入新事实）*</FieldLabel>
                                                                <Textarea className="min-h-[64px] text-xs" value={concludeText}
                                                                    onChange={e => setConcludeText(e.target.value)} />
                                                                <div className="flex items-end gap-2">
                                                                    <div>
                                                                        <FieldLabel>事实类型</FieldLabel>
                                                                        <select
                                                                            className="h-8 rounded-md border border-border bg-background/40 px-2 text-xs"
                                                                            value={concludeType}
                                                                            onChange={e => setConcludeType(e.target.value)}
                                                                        >
                                                                            <option value="observation">observation</option>
                                                                            <option value="patch_profile">patch_profile</option>
                                                                            <option value="exploit_idea">exploit_idea</option>
                                                                        </select>
                                                                    </div>
                                                                    <Button size="sm" disabled={busy}
                                                                        onClick={() => void concludeIntent(it.id)}>
                                                                        {actionBusy === `intent-conclude-${it.id}` ? '提交中…' : '提交总结'}
                                                                    </Button>
                                                                </div>
                                                            </div>
                                                        ) : null}
                                                    </div>
                                                );
                                            })}
                                            {intents.length === 0 ? <div className="text-xs text-muted-foreground">暂无意图</div> : null}
                                        </div>

                                        <div className="mt-4 space-y-2 rounded-md border border-border/70 bg-background/20 p-3">
                                            <FieldLabel>声明意图（声明人取左侧 creator）*</FieldLabel>
                                            <div className="flex flex-wrap items-center gap-2">
                                                <select
                                                    className="h-8 rounded-md border border-border bg-background/40 px-2 text-xs"
                                                    value={intentType}
                                                    onChange={e => setIntentType(e.target.value)}
                                                >
                                                    <option value="gap">gap</option>
                                                    <option value="exploit_idea">exploit_idea</option>
                                                    <option value="verify">verify</option>
                                                    <option value="patch_profile">patch_profile</option>
                                                </select>
                                                <Input className="h-8 flex-1 min-w-[200px] text-xs" value={intentText}
                                                    onChange={e => setIntentText(e.target.value)} placeholder="意图内容…" />
                                            </div>
                                            <FieldLabel>依据事实（至少一个）*</FieldLabel>
                                            <FactPicker facts={facts} selected={intentFrom}
                                                onToggle={id => setIntentFrom(v => toggleValue(v, id))} />
                                            <Button size="sm" disabled={busy} onClick={() => void declareIntent()}>
                                                {actionBusy === 'intent' ? '声明中…' : '声明意图'}
                                            </Button>
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
