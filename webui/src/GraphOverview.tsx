import { useEffect, useMemo, useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from './components/ui/card';
import { Badge } from './components/ui/badge';
import { Button } from './components/ui/button';
import { normalizeProjects, recentProjects, type GraphProject } from './graphOrder';

// GraphOverview is the post-iteration homepage: a read-only, numbered walkthrough
// of the graph lane (patch inversion -> provenance graph -> reason -> dispatch ->
// sandbox evidence -> rules -> completion). It never replaces the legacy lab
// entries; App.tsx keeps those below this card.

type GraphOverviewProps = {
    baseURL: string;
    authHeader: string;
    labTotal: number;
    labRunning: number;
    onOpenGraph: () => void;
    onOpenLabs: () => void;
};

type GraphNode = {
    id: string;
    project_id?: string;
    kind?: string;
    type?: string;
    content?: string;
    state?: string;
};

type GraphEdge = {
    from?: string;
    to?: string;
    kind?: string;
};

type LoadedProject = {
    project: GraphProject;
    nodes: GraphNode[];
    edges: GraphEdge[];
};

type Metric = {
    label: string;
    value: number | string;
    hint: string;
};

type DemoModule = {
    no: string;
    title: string;
    req: string;
    metric: string;
    desc: string;
};

type RequirementRow = {
    id: string;
    title: string;
    surface: string;
    status: string;
    tone: 'success' | 'warning' | 'secondary';
};

// R1–R16 coverage: every backend iteration item is mapped to the frontend
// surface that exposes it, or explicitly marked as having no endpoint yet.
const REQUIREMENTS: RequirementRow[] = [
    { id: 'R1', title: '可回退基线', surface: 'Git 基线 / 构建产物', status: '工程侧', tone: 'secondary' },
    { id: 'R2', title: 'Cairn Skills', surface: '技能页（旧入口）', status: '已展示', tone: 'success' },
    { id: 'R3', title: '溯源图模型与存储', surface: '溯源图事实 / 意图 / 边', status: '已展示', tone: 'success' },
    { id: 'R4', title: '图协议 API', surface: '溯源图面板全部操作', status: '已展示', tone: 'success' },
    { id: 'R5', title: '调度核心', surface: '模块 04 + reason 租约徽标', status: '部分展示', tone: 'warning' },
    { id: 'R6', title: 'Worker driver', surface: 'bootstrap / reason / explore 事实', status: '部分展示', tone: 'warning' },
    { id: 'R7', title: '补丁反演 bootstrap', surface: '模块 01 + patch_profile 事实', status: '已展示', tone: 'success' },
    { id: 'R8', title: '溯源图推理 reason', surface: '模块 03 + 领取总脑', status: '已展示', tone: 'success' },
    { id: 'R9', title: '沙箱实证 explore', surface: '模块 05 + observation 事实', status: '已展示', tone: 'success' },
    { id: 'R10', title: '证据判定与反证', surface: '模块 05/07 + 完成 / 重开', status: '已展示', tone: 'success' },
    { id: 'R11', title: '规则演化回流', surface: '模块 06 + rule_hit 事实', status: '部分展示', tone: 'warning' },
    { id: 'R12', title: '人类意图注入', surface: '写入提示（Hint）', status: '已展示', tone: 'success' },
    { id: 'R13', title: 'Web 面板与报告', surface: '图视图已交付；补丁面板 / Finding 报告后置', status: '部分展示', tone: 'warning' },
    { id: 'R14', title: '对照评测', surface: 'patcheval NDJSON；无 HTTP 端点', status: '待接入', tone: 'secondary' },
    { id: 'R15', title: '端到端验收', surface: '导出 YAML / 时间线 + CVE 演示项目', status: '部分展示', tone: 'warning' },
    { id: 'R16', title: '兼容与灰度', surface: '图 API 可用徽标 + 旧实验室入口', status: '已展示', tone: 'success' },
];

function isJsonResponse(resp: Response): boolean {
    const contentType = resp.headers.get('content-type') || '';
    return contentType.toLowerCase().includes('application/json');
}

function unwrap(payload: any): any {
    if (payload && typeof payload === 'object' && payload.success === true && 'result' in payload) {
        return payload.result;
    }
    if (payload && typeof payload === 'object' && 'Data' in payload) {
        return payload.Data;
    }
    return payload;
}

function projectStatusVariant(status?: string): 'success' | 'warning' | 'secondary' {
    if (status === 'completed') return 'success';
    if (status === 'active') return 'warning';
    return 'secondary';
}

function shortOrigin(origin?: string): string {
    const raw = String(origin || '').trim();
    if (!raw) return '未填写来源';
    const parts = raw.split(/[\\/]/).filter(Boolean);
    return parts.length > 0 ? parts[parts.length - 1] : raw;
}

export default function GraphOverview({
    baseURL,
    authHeader,
    labTotal,
    labRunning,
    onOpenGraph,
    onOpenLabs,
}: GraphOverviewProps) {
    const [projects, setProjects] = useState<GraphProject[]>([]);
    const [loaded, setLoaded] = useState<LoadedProject[]>([]);
    const [loading, setLoading] = useState<boolean>(false);
    const [laneUnavailable, setLaneUnavailable] = useState<boolean>(false);
    const [error, setError] = useState<string>('');
    const [reloadKey, setReloadKey] = useState<number>(0);

    useEffect(() => {
        if (!authHeader) {
            setLoading(false);
            setLaneUnavailable(false);
            setProjects([]);
            setLoaded([]);
            setError('');
            return;
        }
        const controller = new AbortController();
        let cancelled = false;

        async function load() {
            setLoading(true);
            setError('');
            try {
                const resp = await fetch(`${baseURL}/api/graph/projects`, {
                    headers: { Authorization: authHeader },
                    signal: controller.signal,
                });
                if (!isJsonResponse(resp)) {
                    if (!cancelled) {
                        setLaneUnavailable(true);
                        setProjects([]);
                        setLoaded([]);
                    }
                    return;
                }
                const payload = await resp.json().catch(() => null);
                if (resp.status === 401 || resp.status === 403) {
                    if (!cancelled) {
                        setLaneUnavailable(true);
                        setError(`鉴权失败（HTTP ${resp.status}），请重新登录`);
                        setProjects([]);
                        setLoaded([]);
                    }
                    return;
                }
                const list = Array.isArray(payload)
                    ? payload
                    : (payload && payload.success === true && Array.isArray(payload.result) ? payload.result : null);
                if (!list) {
                    if (!cancelled) {
                        setLaneUnavailable(true);
                        setProjects([]);
                        setLoaded([]);
                    }
                    return;
                }
                if (cancelled) return;
                const normalized = normalizeProjects(list);
                setLaneUnavailable(false);
                setProjects(normalized);

                const targets = recentProjects(normalized, 5);
                const snapshots = await Promise.all(targets.map(async (p: GraphProject): Promise<LoadedProject> => {
                    try {
                        const snapResp = await fetch(
                            `${baseURL}/api/graph/projects/${encodeURIComponent(p.id)}/snapshot`,
                            { headers: { Authorization: authHeader }, signal: controller.signal },
                        );
                        if (!snapResp.ok || !isJsonResponse(snapResp)) {
                            return { project: p, nodes: [], edges: [] };
                        }
                        const snapPayload = unwrap(await snapResp.json());
                        return {
                            project: (snapPayload && snapPayload.project) || p,
                            nodes: Array.isArray(snapPayload?.nodes) ? snapPayload.nodes : [],
                            edges: Array.isArray(snapPayload?.edges) ? snapPayload.edges : [],
                        };
                    } catch {
                        return { project: p, nodes: [], edges: [] };
                    }
                }));
                if (!cancelled) setLoaded(snapshots);
            } catch (e: any) {
                if (e?.name === 'AbortError' || cancelled) return;
                setLaneUnavailable(true);
                setError(String(e?.message || e));
            } finally {
                if (!cancelled) setLoading(false);
            }
        }

        void load();
        return () => {
            cancelled = true;
            controller.abort();
        };
    }, [baseURL, authHeader, reloadKey]);

    const aggregate = useMemo(() => {
        const nodes: GraphNode[] = [];
        const edges: GraphEdge[] = [];
        for (const item of loaded) {
            nodes.push(...item.nodes);
            edges.push(...item.edges);
        }
        const facts = nodes.filter(n => n.kind === 'fact');
        const intents = nodes.filter(n => n.kind === 'intent');
        const hints = nodes.filter(n => n.kind === 'hint');
        const countType = (type: string) => facts.filter(n => (n.type || '').toLowerCase() === type).length;
        const openIntents = intents.filter(n => (n.state || 'open') === 'open').length;
        const claimedIntents = intents.filter(n => n.state === 'claimed').length;
        return {
            nodes,
            edges,
            facts,
            intents,
            hints,
            openIntents,
            claimedIntents,
            patchProfiles: countType('patch_profile'),
            exploitIdeas: countType('exploit_idea'),
            constraints: countType('constraint'),
            observations: countType('observation'),
            verdicts: countType('verdict'),
            ruleHits: countType('rule_hit'),
            findings: countType('finding'),
            activeProjects: projects.filter(p => p.status === 'active').length,
            completedProjects: projects.filter(p => p.status === 'completed').length,
            stoppedProjects: projects.filter(p => p.status === 'stopped').length,
            reasonLeases: projects.filter(p => !!p.reason).length,
            bootstrapProjects: projects.filter(p => !!p.bootstrap_enabled).length,
        };
    }, [loaded, projects]);

    const metrics: Metric[] = [
        { label: '图项目', value: projects.length, hint: `${aggregate.activeProjects} 运行中 · ${aggregate.completedProjects} 已完成 · ${aggregate.stoppedProjects} 已停止` },
        { label: '事实 Fact', value: aggregate.facts.length, hint: `${aggregate.patchProfiles} 补丁档案` },
        { label: '意图 Intent', value: aggregate.intents.length, hint: `${aggregate.openIntents} 待认领` },
        { label: '提示 Hint', value: aggregate.hints.length, hint: '人类判断注入' },
        { label: '证据边 Edge', value: aggregate.edges.length, hint: `${aggregate.verdicts} 条判决` },
        { label: '实验室', value: labTotal, hint: `${labRunning} 个沙箱运行中` },
    ];

    const modules: DemoModule[] = [
        {
            no: '01',
            title: '补丁摄入与反演',
            req: 'R7',
            metric: `${aggregate.patchProfiles} 个补丁档案 · ${aggregate.exploitIdeas} 条利用假设`,
            desc: 'diff 经 bootstrap 解析为 patch_profile / exploit_idea 事实；来源文件：' +
                shortOrigin(projects.find(p => p.bootstrap_enabled)?.origin || projects[0]?.origin),
        },
        {
            no: '02',
            title: '溯源图事实 / 意图 / 提示',
            req: 'R3 · R4 · R12',
            metric: `${aggregate.facts.length} 事实 · ${aggregate.intents.length} 意图 · ${aggregate.hints.length} 提示`,
            desc: 'Fact / Intent / Hint 与 derive / support / refute / block / produce 边构成唯一知识载体。',
        },
        {
            no: '03',
            title: '推理与缺口检测',
            req: 'R8',
            metric: `${aggregate.openIntents} 个 open 意图 · ${aggregate.reasonLeases} 个项目持有 reason 租约`,
            desc: 'reason 读全图做覆盖检查与缺口排序，把下一步探索声明为可认领意图。',
        },
        {
            no: '04',
            title: 'Worker 调度与租约',
            req: 'R5 · R6',
            metric: `${aggregate.claimedIntents} 个意图已认领 · ${aggregate.bootstrapProjects} 个项目启用 bootstrap`,
            desc: 'Dispatcher 是唯一协议写入者；Worker 领取/心跳/释放，超时自动回收。',
        },
        {
            no: '05',
            title: '沙箱实证与证据判决',
            req: 'R9 · R10',
            metric: `${aggregate.observations} 条观测 · ${aggregate.verdicts} 条判决`,
            desc: 'explore 在 Docker 沙箱执行验证并回写 observation；verdict 必须带 support/refute 边。',
        },
        {
            no: '06',
            title: '规则回流与评测',
            req: 'R11 · R14',
            metric: `${aggregate.ruleHits} 条规则命中 · 规则库 SQLite`,
            desc: 'CE4 将终态回流 RuleBase；评测口径：反演成功率 / 试错轮次 / 单位 finding token 成本（NDJSON，当前无 HTTP 端点）。',
        },
        {
            no: '07',
            title: '完成、重开与导出',
            req: 'R10 · R15',
            metric: `${aggregate.completedProjects} 个完成项目 · ${aggregate.edges.length} 条可导出边`,
            desc: 'complete 建立 from → goal 完成边；reopen 写入纠错事实；YAML / timeline 可回放。',
        },
    ];

    return (
        <Card className="col-span-full overflow-hidden">
            <CardHeader>
                <div className="min-w-0">
                    <CardTitle className="text-base">图驱动总览</CardTitle>
                    <div className="mt-1 text-xs text-muted-foreground">
                        补丁是起点，溯源图是唯一知识与推理载体；按 01 → 07 顺序展开，每个模块都能进入溯源图查看细节。
                        指标取最近 5 个图项目的快照聚合。
                    </div>
                </div>
                <div className="flex shrink-0 flex-wrap items-center gap-2">
                    <Badge variant={loading ? 'secondary' : laneUnavailable ? 'destructive' : 'success'}>
                        {loading ? '加载中…' : laneUnavailable ? '图 API 不可用' : '图 API 可用'}
                    </Badge>
                    <Badge variant="primary">{projects.length} 个图项目</Badge>
                    <Button size="sm" variant="outline" disabled={loading} onClick={() => setReloadKey(k => k + 1)}>
                        {loading ? '加载中…' : '刷新'}
                    </Button>
                </div>
            </CardHeader>
            <CardContent>
                {error ? (
                    <div className="mb-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive break-all">
                        {error}
                    </div>
                ) : null}
                {laneUnavailable ? (
                    <div className="mb-3 rounded-lg border border-warning/40 bg-warning/10 px-3 py-2 text-xs text-warning break-words">
                        图车道未启用或不可达。确认配置 <code>misc.graph_lane_enabled=true</code> 后重启服务；旧实验室入口不受影响。
                    </div>
                ) : null}

                <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-6">
                    {metrics.map(m => (
                        <div key={m.label} className="rounded-lg border border-border bg-background/20 px-3 py-2">
                            <div className="text-[11px] text-muted-foreground">{m.label}</div>
                            <div className="mt-0.5 text-xl font-semibold tabular-nums">{m.value}</div>
                            <div className="mt-0.5 text-[10px] text-muted-foreground truncate">{m.hint}</div>
                        </div>
                    ))}
                </div>

                <div className="mt-4 flex flex-wrap items-end justify-between gap-2">
                    <div>
                        <div className="text-xs font-semibold tracking-wide">模块演示序列</div>
                        <div className="mt-0.5 text-[11px] text-muted-foreground">
                            从补丁到证据的完整推导路径；点击任一模块进入溯源图。
                        </div>
                    </div>
                    <div className="flex flex-wrap gap-1 text-[10px] text-muted-foreground">
                        {modules.map(m => (
                            <span key={m.no} className="rounded-full border border-border px-2 py-0.5">{m.no}</span>
                        ))}
                    </div>
                </div>

                <div className="mt-2 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                    {modules.map(m => (
                        <button
                            key={m.no}
                            type="button"
                            onClick={onOpenGraph}
                            className="group rounded-lg border border-border bg-background/20 p-3 text-left transition-colors hover:border-primary/60 hover:bg-primary/5"
                            title={`进入溯源图：${m.title}`}
                        >
                            <div className="flex items-center justify-between gap-2">
                                <div className="flex items-center gap-2 min-w-0">
                                    <span className="rounded-md border border-primary/40 bg-primary/10 px-1.5 py-0.5 text-[11px] font-mono text-primary">{m.no}</span>
                                    <span className="truncate text-sm font-semibold">{m.title}</span>
                                </div>
                                <Badge variant="secondary" className="shrink-0">{m.req}</Badge>
                            </div>
                            <div className="mt-2 text-xs text-primary/90 break-words">{m.metric}</div>
                            <div className="mt-1 break-words text-[11px] leading-relaxed text-muted-foreground">{m.desc}</div>
                            <div className="mt-2 text-[11px] font-medium text-primary opacity-0 transition-opacity group-hover:opacity-100">
                                打开溯源图 →
                            </div>
                        </button>
                    ))}
                </div>

                <details className="mt-4 rounded-lg border border-border bg-background/20">
                    <summary className="flex cursor-pointer flex-wrap items-center justify-between gap-2 px-3 py-2">
                        <span className="text-xs font-semibold">迭代组件覆盖（R1–R16 逐项对照）</span>
                        <span className="text-[11px] text-muted-foreground">
                            已展示 {REQUIREMENTS.filter(r => r.status === '已展示').length} · 部分展示 {REQUIREMENTS.filter(r => r.status === '部分展示').length} · 待接入 / 工程侧 {REQUIREMENTS.filter(r => r.status === '待接入' || r.status === '工程侧').length}
                        </span>
                    </summary>
                    <div className="grid gap-2 border-t border-border px-3 py-3 md:grid-cols-2">
                        {REQUIREMENTS.map(r => (
                            <div key={r.id} className="flex items-start justify-between gap-2 rounded-md border border-border/70 bg-background/30 px-2.5 py-1.5">
                                <div className="min-w-0">
                                    <div className="text-xs font-medium">
                                        <span className="mr-1.5 font-mono text-[11px] text-muted-foreground">{r.id}</span>
                                        {r.title}
                                    </div>
                                    <div className="mt-0.5 break-words text-[10px] text-muted-foreground">{r.surface}</div>
                                </div>
                                <Badge variant={r.tone} className="shrink-0">{r.status}</Badge>
                            </div>
                        ))}
                    </div>
                </details>

                <div className="mt-4 rounded-lg border border-border bg-background/20">
                    <div className="flex items-center justify-between gap-2 border-b border-border px-3 py-2">
                        <div className="text-xs font-semibold">图项目速览</div>
                        <Button size="sm" variant="ghost" onClick={onOpenGraph}>全部项目 →</Button>
                    </div>
                    <div className="divide-y divide-border/60">
                        {recentProjects(projects, 4).map(p => (
                            <button
                                key={p.id}
                                type="button"
                                onClick={onOpenGraph}
                                className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left hover:bg-muted/20"
                            >
                                <div className="min-w-0">
                                    <div className="truncate text-xs font-semibold">{p.title || p.id}</div>
                                    <div className="mt-0.5 truncate text-[10px] text-muted-foreground font-mono">{shortOrigin(p.origin)}</div>
                                </div>
                                <div className="flex shrink-0 items-center gap-1">
                                    {p.bootstrap_enabled ? <Badge variant="primary">bootstrap</Badge> : null}
                                    {p.reason?.worker ? <Badge variant="warning">reason {p.reason.worker}</Badge> : null}
                                    <Badge variant={projectStatusVariant(p.status)}>{p.status || 'unknown'}</Badge>
                                </div>
                            </button>
                        ))}
                        {!loading && !laneUnavailable && !error && projects.length === 0 ? (
                            <div className="flex flex-wrap items-center justify-between gap-2 px-3 py-3 text-xs text-muted-foreground">
                                <span>暂无图项目。新建时来源填写补丁 diff 绝对路径并勾选 bootstrap。</span>
                                <Button size="sm" variant="outline" onClick={onOpenGraph}>进入溯源图新建项目 →</Button>
                            </div>
                        ) : null}
                    </div>
                </div>

                <div className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-dashed border-border bg-background/10 px-3 py-2">
                    <div className="text-[11px] text-muted-foreground">
                        旧入口保留：多来源实验室创建、启动 / 停止、沙箱自动初始化与执行仍在下方。
                    </div>
                    <Button size="sm" variant="outline" onClick={onOpenLabs}>前往实验室入口</Button>
                </div>
            </CardContent>
        </Card>
    );
}
