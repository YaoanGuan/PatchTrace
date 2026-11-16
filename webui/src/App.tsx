import { useEffect, useMemo, useRef, useState } from 'react';
import './App.css';
import { GetAPIBaseURL, GetBasicAuthPassword, GetBasicAuthUser } from './web_api';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from './components/ui/dialog';
import { Card, CardContent, CardHeader, CardTitle } from './components/ui/card';
import { Button } from './components/ui/button';
import { Badge } from './components/ui/badge';
import { ScrollArea } from './components/ui/scroll-area';
import GraphPanel from './GraphPanel';
import { Input } from './components/ui/input';
import { Textarea } from './components/ui/textarea';
import ReactMarkdown from 'react-markdown';

type ApiResp<T> = {
    success?: boolean;
    result?: T;
    error?: any;
    Data?: T;
    data?: T;
};

function getRespData<T>(r: ApiResp<T>): T | undefined {
    return (r.result ?? r.Data ?? r.data) as any;
}

function generateUUID(): string {
    const randomUUID = globalThis.crypto?.randomUUID;
    if (typeof randomUUID === 'function') {
        return randomUUID.call(globalThis.crypto);
    }

    const getRandomValues = globalThis.crypto?.getRandomValues?.bind(globalThis.crypto);
    if (typeof getRandomValues === 'function') {
        const bytes = new Uint8Array(16);
        getRandomValues(bytes);
        bytes[6] = (bytes[6] & 0x0f) | 0x40;
        bytes[8] = (bytes[8] & 0x3f) | 0x80;
        const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0'));
        return `${hex.slice(0, 4).join('')}-${hex.slice(4, 6).join('')}-${hex.slice(6, 8).join('')}-${hex.slice(8, 10).join('')}-${hex.slice(10, 16).join('')}`;
    }

    return `uuid-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
}

const avatarUrlCache = new Map<string, string>();
const avatarPreloadCache = new Set<string>();

type ProjectListItem = {
	projectID: string;
    projectName: string;
    labName?: string;
    status?: string;
    rawStatus?: string;
    isRunning?: boolean;
    isStarting?: boolean;
    isQueued?: boolean;
    queuePosition?: number;
    brainFinished?: boolean;
    groupCount?: number;
    startTime?: string;
    endTime?: string;
};

type LabChatMessage = {
    role: 'user' | 'system';
    text: string;
    ts: string;
    persona_name?: string;
    avatar_file?: string;
    channel_id?: string;
    channel_name?: string;
    group_id?: string;
    group_name?: string;
    sender_type?: string;
};

type HostCommandApproval = {
    id: string;
    kind?: string;
    title?: string;
    target?: string;
    action?: string;
    impact?: string;
    safer_alternative?: string;
    status: string;
    command: string;
    reason: string;
    workdir?: string;
    timeout_sec?: number;
    max_output_chars?: number;
    created_at?: string;
    updated_at?: string;
    decision_at?: string;
    decision_note?: string;
    reviewer?: string;
    group_id?: string;
    group_name?: string;
    requester_source?: string;
    agent_tool_name?: string;
    digital_human_id?: string;
    persona_name?: string;
    avatar_file?: string;
    exit_code?: number;
    output_preview?: string;
    error?: string;
    output_truncated?: boolean;
    timed_out?: boolean;
};

type BackgroundProcess = {
    session_id: string;
    source: string;
    command: string;
    reason?: string;
    workdir?: string;
    status: string;
    running: boolean;
    pid?: number;
    started_at?: string;
    updated_at?: string;
    ended_at?: string;
    running_for_sec?: number;
    exit_code?: number;
    timed_out?: boolean;
    terminated_by?: string;
    termination_reason?: string;
    error?: string;
    output_preview?: string;
    request_id?: string;
    agent_tool_name?: string;
    agent_name?: string;
    digital_human_id?: string;
    persona_name?: string;
    avatar_file?: string;
    group_id?: string;
    group_name?: string;
    requester_source?: string;
};

type MemoryDebugTarget = {
    target_id: string;
    group_id?: string;
    group_name?: string;
    target_type: string;
    label: string;
    persona_name?: string;
    agent_tool_name?: string;
    digital_human_id?: string;
    agent_id?: string;
    state?: string;
    available: boolean;
    reason?: string;
};

type MCPDigitalHumanLite = {
    id: string;
    persona_name: string;
    avatar_file?: string;
};

type MCPAgentTarget = {
    agent_type: string;
    agent_name: string;
    description?: string;
    digital_humans: MCPDigitalHumanLite[];
    digital_count?: number;
};

type MCPConfigRow = {
    id: string;
    name: string;
    enabled: boolean;
    transport: 'stdio' | 'streamable' | 'sse';
    command?: string;
    args?: string[];
    env?: Record<string, string>;
    workdir?: string;
    url?: string;
    headers?: Record<string, string>;
    tool_allowlist?: string[];
    tool_denylist?: string[];
    tool_name_prefix?: string;
    preserve_schema?: boolean;
    connect_timeout_sec?: number;
    call_timeout_sec?: number;
    agent_types?: string[];
    agent_bindings?: MCPAgentBinding[];
};

type MCPAgentBinding = {
    agent_type: string;
    tool_allowlist?: string[];
    tool_denylist?: string[];
};

type MCPTestResult = {
    ok?: boolean;
    message?: string;
    tool_count?: number;
    tool_names?: string[];
    server_name?: string;
    transport?: string;
    display_name?: string;
};

type LLMTestResult = {
    ok?: boolean;
    backend?: string;
    reply?: string;
    message?: string;
    model?: string;
    section?: string;
};

type SkillSummary = {
    name: string;
    description?: string;
    root_path?: string;
    skill_file_path?: string;
    has_scripts?: boolean;
    has_assets?: boolean;
    has_references?: boolean;
    editable?: boolean;
};

type SkillSettingsResponse = {
    search_paths?: string;
    resolved_roots?: string[];
    managed_root?: string;
    skills?: SkillSummary[];
};

type SkillFileEntry = {
    path: string;
    type: 'file' | 'dir';
    size?: number;
    depth?: number;
};

type SkillFileList = {
    skill_name?: string;
    base_path?: string;
    max_depth?: number;
    truncated?: boolean;
    entry_count?: number;
    entries?: SkillFileEntry[];
};

type SkillFileDocument = {
    skill_name?: string;
    relative_path?: string;
    editable?: boolean;
    content?: string;
    total_chars?: number;
};

function normalizeStringArray(input: any): string[] {
    if (!Array.isArray(input)) return [];
    return Array.from(new Set(input.map(v => String(v ?? '').trim()).filter(Boolean))).sort();
}

function normalizeOrderedStringArray(input: any): string[] {
    if (!Array.isArray(input)) return [];
    return input.map(v => String(v ?? '').trim()).filter(Boolean);
}

function normalizeStringMap(input: any): Record<string, string> {
    if (!input || typeof input !== 'object' || Array.isArray(input)) return {};
    const out: Record<string, string> = {};
    Object.entries(input).forEach(([k, v]) => {
        const key = String(k ?? '').trim();
        if (!key) return;
        out[key] = String(v ?? '').trim();
    });
    return out;
}

function normalizeMCPAgentBindings(input: any, legacyAgentTypes?: any): MCPAgentBinding[] {
    const merged = new Map<string, MCPAgentBinding>();
    const list = Array.isArray(input) ? input : [];
    list.forEach((raw: any) => {
        const agentType = String(raw?.agent_type ?? '').trim();
        if (!agentType) return;
        const prev = merged.get(agentType) || { agent_type: agentType, tool_allowlist: [], tool_denylist: [] };
        const next: MCPAgentBinding = {
            agent_type: agentType,
            tool_allowlist: normalizeStringArray([...(prev.tool_allowlist || []), ...(raw?.tool_allowlist || [])]),
            tool_denylist: normalizeStringArray([...(prev.tool_denylist || []), ...(raw?.tool_denylist || [])]),
        };
        merged.set(agentType, next);
    });
    normalizeStringArray(legacyAgentTypes).forEach(agentType => {
        if (!merged.has(agentType)) {
            merged.set(agentType, { agent_type: agentType, tool_allowlist: [], tool_denylist: [] });
        }
    });
    return Array.from(merged.values()).sort((a, b) => a.agent_type.localeCompare(b.agent_type));
}

function agentTypesFromBindings(bindings?: MCPAgentBinding[]): string[] {
    return normalizeStringArray((bindings || []).map(item => item?.agent_type));
}

function emptyMCPConfig(): MCPConfigRow {
    return {
        id: generateUUID(),
        name: '',
        enabled: true,
        transport: 'stdio',
        command: '',
        args: [],
        env: {},
        workdir: '',
        url: '',
        headers: {},
        tool_allowlist: [],
        tool_denylist: [],
        tool_name_prefix: '',
        preserve_schema: false,
        connect_timeout_sec: 20,
        call_timeout_sec: 120,
        agent_types: [],
        agent_bindings: [],
    };
}

function normalizeMCPConfigRow(input: any): MCPConfigRow {
    const rawTransport = String(input?.transport ?? 'stdio').trim().toLowerCase();
    const transport = rawTransport === 'http' ? 'streamable' : rawTransport;
    const agentBindings = normalizeMCPAgentBindings(input?.agent_bindings, input?.agent_types);
    return {
        id: String(input?.id ?? generateUUID()).trim() || generateUUID(),
        name: String(input?.name ?? '').trim(),
        enabled: !!input?.enabled,
        transport: (['stdio', 'streamable', 'sse'].includes(transport) ? transport : 'stdio') as MCPConfigRow['transport'],
        command: String(input?.command ?? '').trim(),
        args: normalizeOrderedStringArray(input?.args),
        env: normalizeStringMap(input?.env),
        workdir: String(input?.workdir ?? '').trim(),
        url: String(input?.url ?? '').trim(),
        headers: normalizeStringMap(input?.headers),
        tool_allowlist: normalizeStringArray(input?.tool_allowlist),
        tool_denylist: normalizeStringArray(input?.tool_denylist),
        tool_name_prefix: String(input?.tool_name_prefix ?? '').trim(),
        preserve_schema: !!input?.preserve_schema,
        connect_timeout_sec: Math.max(1, Number(input?.connect_timeout_sec || 20) || 20),
        call_timeout_sec: Math.max(1, Number(input?.call_timeout_sec || 120) || 120),
        agent_types: agentTypesFromBindings(agentBindings),
        agent_bindings: agentBindings,
    };
}

function linesToList(input: string): string[] {
    return normalizeStringArray(String(input || '').split('\n'));
}

function listToLines(input?: string[]): string {
    return normalizeStringArray(input).join('\n');
}

function kvTextToMap(input: string): Record<string, string> {
    const out: Record<string, string> = {};
    String(input || '').split('\n').forEach(line => {
        const raw = line.trim();
        if (!raw) return;
        const eq = raw.indexOf('=');
        const colon = raw.indexOf(':');
        let idx = eq;
        if (idx < 0 || (colon >= 0 && colon < idx)) idx = colon;
        if (idx < 0) {
            out[raw] = '';
            return;
        }
        const key = raw.slice(0, idx).trim();
        const value = raw.slice(idx + 1).trim();
        if (!key) return;
        out[key] = value;
    });
    return out;
}

function mapToKVText(input?: Record<string, string>): string {
    const map = normalizeStringMap(input);
    return Object.keys(map).sort().map(k => `${k}=${map[k]}`).join('\n');
}

function chatMessageIdentity(input: any): string {
    return [
        String(input?.role ?? ''),
        String(input?.text ?? input?.message ?? ''),
        String(input?.ts ?? ''),
        String(input?.persona_name ?? ''),
        String(input?.channel_id ?? input?.channelId ?? ''),
        String(input?.sender_type ?? input?.senderType ?? ''),
    ].join('\u0000');
}

function appendUniqueChatMessage(list: LabChatMessage[], msg: LabChatMessage): LabChatMessage[] {
    const key = chatMessageIdentity(msg);
    if (list.some(item => chatMessageIdentity(item) === key)) {
        return list;
    }
    return [...list, msg];
}

function App() {
    const [baseURL, setBaseURL] = useState<string>('');
    const [auth, setAuth] = useState<{ user: string; pass: string } | null>(null);
    const [initError, setInitError] = useState<string>('');
    const [token, setToken] = useState<string>(() => localStorage.getItem('aix_token') || '');
    const [loginUser, setLoginUser] = useState<string>('');
    const [loginPass, setLoginPass] = useState<string>('');
    const [loginError, setLoginError] = useState<string>('');
    const [loginLoading, setLoginLoading] = useState<boolean>(false);
    const [projects, setProjects] = useState<ProjectListItem[]>([]);
    const [events, setEvents] = useState<any[]>([]);
    const [labGroups, setLabGroups] = useState<any[]>([]);
    const [groupDetails, setGroupDetails] = useState<Record<string, any>>({});
    const [activeGroupId, setActiveGroupId] = useState<string>('');
    const [controlBrain, setControlBrain] = useState<any>(null);
    const [sharedResources, setSharedResources] = useState<any>(null);
    const [projectStatus, setProjectStatus] = useState<string>('');
    const [projectIsRunning, setProjectIsRunning] = useState<boolean>(false);
    const [brainFinished, setBrainFinished] = useState<boolean>(false);
    const [view, setView] = useState<'home' | 'detail' | 'settings' | 'mcp' | 'skills' | 'digital_humans' | 'report_templates' | 'graph'>('home');
    const [detailProject, setDetailProject] = useState<string>('');
    const [detailProjectName, setDetailProjectName] = useState<string>('');
    const [detailOpen, setDetailOpen] = useState<boolean>(false);
    const [detailTitle, setDetailTitle] = useState<string>('');
    const [detailKind, setDetailKind] = useState<'agent' | 'exploitIdea' | 'exploitChain' | 'container' | 'report' | 'json' | 'digitalHumanProfile'>('json');
    const [detailObj, setDetailObj] = useState<any>(null);
    const [projectName, setProjectName] = useState<string>('');
    const [taskContent, setTaskContent] = useState<string>('尽可能的挖掘项目中的漏洞，侧重挖掘未授权情况下能完成的高危攻击链');
    const [sourceType, setSourceType] = useState<'file' | 'git' | 'url' | 'empty'>('file');
    const [gitUrl, setGitUrl] = useState<string>('');
    const [fileUrl, setFileUrl] = useState<string>('');
    const fileRef = useRef<HTMLInputElement | null>(null);
    const wsRef = useRef<WebSocket | null>(null);
    const [chatMessages, setChatMessages] = useState<LabChatMessage[]>([]);
    const [chatChannelId, setChatChannelId] = useState<string>('decision_makers');
    const [chatInput, setChatInput] = useState<string>('');
    const [chatSending, setChatSending] = useState<boolean>(false);
    const chatEndRef = useRef<HTMLDivElement | null>(null);
    const chatEndRefFull = useRef<HTMLDivElement | null>(null);
    const [chatFullscreen, setChatFullscreen] = useState<boolean>(false);
    const [hostCommandApprovals, setHostCommandApprovals] = useState<HostCommandApproval[]>([]);
    const [backgroundProcesses, setBackgroundProcesses] = useState<BackgroundProcess[]>([]);
    const [debugMode, setDebugMode] = useState<boolean>(() => {
        try {
            return localStorage.getItem('aix_debug_mode') === '1';
        } catch {
            return false;
        }
    });
    const [memoryDebugTargets, setMemoryDebugTargets] = useState<MemoryDebugTarget[]>([]);
    const [hostCommandDecisionNotes, setHostCommandDecisionNotes] = useState<Record<string, string>>({});
    const [hostCommandDecisionLoading, setHostCommandDecisionLoading] = useState<Record<string, boolean>>({});
    const [decisionPanelExpanded, setDecisionPanelExpanded] = useState<boolean>(() => {
        try {
            const stored = localStorage.getItem('aix_decision_panel_expanded');
            return stored === null ? false : stored === '1';
        } catch {
            return false;
        }
    });
    const [recentDecisionsExpanded, setRecentDecisionsExpanded] = useState<boolean>(() => {
        try {
            return localStorage.getItem('aix_recent_decisions_expanded') === '1';
        } catch {
            return false;
        }
    });
    const [backgroundKillLoading, setBackgroundKillLoading] = useState<Record<string, boolean>>({});
    const [memoryDebugExporting, setMemoryDebugExporting] = useState<Record<string, boolean>>({});
    const [reportSummaryGenerating, setReportSummaryGenerating] = useState<boolean>(false);
    const [toast, setToast] = useState<{ msg: string; type: 'ok' | 'err' } | null>(null);
    const [btnLoading, setBtnLoading] = useState<Record<string, boolean>>({});
    const [tokenUsage, setTokenUsage] = useState<{ prompt_tokens: number; completion_tokens: number; total_tokens: number; agents?: { label: string; prompt_tokens: number; completion_tokens: number; total_tokens: number }[] }>({ prompt_tokens: 0, completion_tokens: 0, total_tokens: 0, agents: [] });
    const [contextBreakdown, setContextBreakdown] = useState<{ sections: { name: string; tokens: number }[]; total: number; max_context: number }>({ sections: [], total: 0, max_context: 0 });
    const [configData, setConfigData] = useState<Record<string, Record<string, string>>>({});
    const [configDraft, setConfigDraft] = useState<Record<string, Record<string, string>>>({});
    const [configLoading, setConfigLoading] = useState<boolean>(false);
    const [configSaving, setConfigSaving] = useState<boolean>(false);
    const [mcpConfigs, setMCPConfigs] = useState<MCPConfigRow[]>([]);
    const [mcpAgentTargets, setMCPAgentTargets] = useState<MCPAgentTarget[]>([]);
    const [mcpLoading, setMCPLoading] = useState<boolean>(false);
    const [mcpSaving, setMCPSaving] = useState<boolean>(false);
    const [mcpTesting, setMCPTesting] = useState<Record<string, boolean>>({});
    const [mcpTestResults, setMCPTestResults] = useState<Record<string, MCPTestResult | null>>({});
    const [mcpExpanded, setMCPExpanded] = useState<Record<string, boolean>>({});
    const [mcpArgDrafts, setMCPArgDrafts] = useState<Record<string, string>>({});
    const [skillsLoading, setSkillsLoading] = useState<boolean>(false);
    const [skillsSaving, setSkillsSaving] = useState<boolean>(false);
    const [skillsCreating, setSkillsCreating] = useState<boolean>(false);
    const [skillsDeleting, setSkillsDeleting] = useState<boolean>(false);
    const [skillFileLoading, setSkillFileLoading] = useState<boolean>(false);
    const [skillFileSaving, setSkillFileSaving] = useState<boolean>(false);
    const [skillSearchPathsDraft, setSkillSearchPathsDraft] = useState<string>('');
    const [skillResolvedRoots, setSkillResolvedRoots] = useState<string[]>([]);
    const [skillManagedRoot, setSkillManagedRoot] = useState<string>('');
    const [skillItems, setSkillItems] = useState<SkillSummary[]>([]);
    const [selectedSkillName, setSelectedSkillName] = useState<string>('');
    const [skillFiles, setSkillFiles] = useState<SkillFileEntry[]>([]);
    const [selectedSkillPath, setSelectedSkillPath] = useState<string>('');
    const [skillFileContent, setSkillFileContent] = useState<string>('');
    const [skillFileDirty, setSkillFileDirty] = useState<boolean>(false);
    const [newSkillName, setNewSkillName] = useState<string>('');
    const [newSkillDescription, setNewSkillDescription] = useState<string>('');
    const [newSkillFilePath, setNewSkillFilePath] = useState<string>('');
    const [newSectionName, setNewSectionName] = useState<string>('');
    const [newKeyInputs, setNewKeyInputs] = useState<Record<string, { key: string; value: string }>>({});
    const [modelOptions, setModelOptions] = useState<Record<string, string[]>>({});
    const [modelLoading, setModelLoading] = useState<Record<string, boolean>>({});
    const [llmTestLoading, setLLMTestLoading] = useState<Record<string, boolean>>({});
    const [llmTestResults, setLLMTestResults] = useState<Record<string, LLMTestResult | null>>({});
    const [digitalHumans, setDigitalHumans] = useState<any[]>([]);
    const [dhLoading, setDhLoading] = useState<boolean>(false);
    const [dhEditing, setDhEditing] = useState<any | null>(null);
    const [reportTemplates, setReportTemplates] = useState<Record<string, string>>({});
    const [reportTemplatesDraft, setReportTemplatesDraft] = useState<Record<string, string>>({});
    const [rtLoading, setRtLoading] = useState<boolean>(false);
    const [rtSaving, setRtSaving] = useState<boolean>(false);
    const [initChecked, setInitChecked] = useState<boolean>(false);
    const [needsInit, setNeedsInit] = useState<boolean>(false);
    const [dockerImages, setDockerImages] = useState<Record<string, boolean>>({});
    const [initStep, setInitStep] = useState<'user' | 'docker' | 'done'>('user');
    const [initUsername, setInitUsername] = useState<string>('');
    const [initPassword, setInitPassword] = useState<string>('');
    const [initPassword2, setInitPassword2] = useState<string>('');
    const [setupError, setSetupError] = useState<string>('');
    const [initLoading, setInitLoading] = useState<boolean>(false);
    const [dockerBuilding, setDockerBuilding] = useState<Record<string, boolean>>({});
    const [dockerBuildOutput, setDockerBuildOutput] = useState<Record<string, string>>({});
    const [customRegistry, setCustomRegistry] = useState<string>('');

    function formatTokens(n: number): string {
        if (n >= 1_000_000) return (n / 1_000_000).toFixed(2) + 'M';
        if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
        return String(n);
    }

    function getAvatarUrl(file: string): string {
        const f = file && file !== '-' ? file : 'system.png';
        const cacheKey = `${baseURL || ''}|${f}`;
        const cached = avatarUrlCache.get(cacheKey);
        if (cached) return cached;
        let url = '';
        try { url = new URL(`./assets/avatar/${f}`, import.meta.url).toString(); }
        catch {
            url = f === 'system.png'
                ? new URL(`./assets/avatar/system.png`, import.meta.url).toString()
                : `${baseURL}/avatar/${encodeURIComponent(f)}`;
        }
        avatarUrlCache.set(cacheKey, url);
        if (typeof window !== 'undefined' && !avatarPreloadCache.has(url)) {
            avatarPreloadCache.add(url);
            const img = new Image();
            img.decoding = 'async';
            img.src = url;
        }
        return url;
    }

    function getRunPersonaName(run: any): string {
        return String(run?.Profile?.persona_name ?? run?.persona_name ?? run?.PersonaName ?? getRoleLabel(run?.ToolName ?? run?.AgentToolName ?? '') ?? '运行中数字人');
    }

    function getRunAvatarFile(run: any): string {
        return String(run?.Profile?.avatar_file ?? run?.AvatarFile ?? run?.avatar_file ?? 'system.png');
    }

    function getRunActivity(run: any): string {
        const explicit = String(run?.Activity ?? run?.activity ?? '').trim();
        if (explicit) return explicit;
        const state = String(run?.RunState ?? run?.state ?? '').toLowerCase();
        if (state.includes('coordinat')) return '组内研判中';
        if (state.includes('done') || state.includes('complete') || state.includes('success')) return '已完成';
        if (state.includes('fail') || state.includes('error') || state.includes('cancel')) return '已结束';
        if (state.includes('standby') || state.includes('idle') || state.includes('stop')) return '待命';
        return '思考中';
    }

    function isTemporaryAgentRun(run: any): boolean {
        const haystack = [run?.ToolName, run?.agentToolName, run?.AgentToolName, run?.AgentID, run?.PersonaName, run?.persona_name]
            .map((v) => String(v ?? '').toLowerCase()).join(' ');
        return haystack.includes('pocknowledgeagent') || haystack.includes('toolusageguideagent') || haystack.includes('doclookup') || haystack.includes('poclookup');
    }

    function isRunActive(run: any): boolean {
        if (!run || isTemporaryAgentRun(run)) return false;
        const state = String(run?.RunState ?? run?.state ?? '').toLowerCase();
        return !(state.includes('done') || state.includes('complete') || state.includes('success') || state.includes('fail') || state.includes('error') || state.includes('cancel') || state.includes('standby') || state.includes('idle') || state.includes('stop'));
    }

    function getRuntimeGroupIdFromProjectName(projectName: any): string {
        const runtimeProject = String(projectName ?? '').trim();
        if (runtimeProject.endsWith('_binary_security')) return 'binary_security';
        if (runtimeProject.endsWith('_pentest_security')) return 'pentest_security';
        return 'web_security';
    }

    function findRosterMemberForRun(roster: Record<string, any[]> | undefined, run: any): any | null {
        const runAgentId = String(run?.AgentID ?? run?.agent_id ?? '').trim();
        const runPersona = getRunPersonaName(run);
        for (const list of Object.values(roster || {})) {
            if (!Array.isArray(list)) continue;
            for (const member of list) {
                const memberAgentId = String(member?.agent_id ?? member?.AgentID ?? '').trim();
                if (runAgentId && memberAgentId && runAgentId === memberAgentId) return member;
                if (!runAgentId && String(member?.persona_name ?? '').trim() === runPersona) return member;
            }
        }
        return null;
    }

    function runIdentityKey(run: any, groupId: string): string {
        const profile = run?.Profile || {};
        const digitalHumanId = String(profile?.digital_human_id ?? run?.digital_human_id ?? '').trim();
        if (digitalHumanId) return `${groupId}:dh:${digitalHumanId}`;
        const agentId = String(run?.AgentID ?? run?.agent_id ?? '').trim();
        if (agentId) return `${groupId}:agent:${agentId}`;
        const personaName = getRunPersonaName(run).trim();
        const toolName = String(run?.ToolName ?? run?.AgentToolName ?? run?.agentToolName ?? '').trim();
        return `${groupId}:persona:${toolName}:${personaName}`;
    }

    function runStartedAtMs(run: any): number {
        const raw = String(run?.StartedAt ?? run?.started_at ?? run?.startedAt ?? '').trim();
        if (!raw) return 0;
        const normalized = raw.includes('T') ? raw : raw.replace(' ', 'T');
        const ts = Date.parse(normalized);
        return Number.isFinite(ts) ? ts : 0;
    }

    function dedupeActiveRuns(items: { run: any; member: any; groupId: string; groupName: string }[]) {
        const byIdentity = new Map<string, { run: any; member: any; groupId: string; groupName: string }>();
        for (const item of items) {
            const key = runIdentityKey(item.run, item.groupId);
            const previous = byIdentity.get(key);
            if (!previous || runStartedAtMs(item.run) >= runStartedAtMs(previous.run)) {
                byIdentity.set(key, item);
            }
        }
        return Array.from(byIdentity.values());
    }

    function showToast(msg: string, type: 'ok' | 'err' = 'ok') {
        setToast({ msg, type });
        setTimeout(() => setToast(null), 2500);
    }

    const authHeader = useMemo(() => {
        if (auth && auth.user && auth.pass) {
            return 'Basic ' + btoa(`${auth.user}:${auth.pass}`);
        }
        if (token) return 'Bearer ' + token;
        return '';
    }, [auth, token]);

    const selectedSkill = useMemo(() => {
        return skillItems.find(item => item.name === selectedSkillName) || null;
    }, [skillItems, selectedSkillName]);

    useEffect(() => {
        try {
            localStorage.setItem('aix_debug_mode', debugMode ? '1' : '0');
        } catch {
            // ignore
        }
    }, [debugMode]);

    useEffect(() => {
        try {
            localStorage.setItem('aix_decision_panel_expanded', decisionPanelExpanded ? '1' : '0');
        } catch {
            // ignore
        }
    }, [decisionPanelExpanded]);

    useEffect(() => {
        try {
            localStorage.setItem('aix_recent_decisions_expanded', recentDecisionsExpanded ? '1' : '0');
        } catch {
            // ignore
        }
    }, [recentDecisionsExpanded]);

    function handleLogout() {
        setToken('');
        localStorage.removeItem('aix_token');
    }

    async function handleLogin() {
        if (!loginUser.trim() || !loginPass.trim()) return;
        setLoginLoading(true);
        setLoginError('');
        try {
            let loginBase = baseURL;
            if (!loginBase) {
                const origin = (typeof window !== 'undefined' && window.location?.origin) || '';
                if (origin && origin !== 'null') {
                    const u = new URL(origin);
                    const host = String(u.hostname || '').toLowerCase();
                    const port = String(u.port || '');
                    if ((host === 'localhost' || host === '127.0.0.1') && port && port !== '9999') {
                        loginBase = 'http://127.0.0.1:9999';
                    }
                }
            }
            const resp = await fetch(`${loginBase}/login`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username: loginUser.trim(), password: loginPass.trim() }),
            });
            const js = await resp.json();
            if (!resp.ok || !js?.success) {
                setLoginError(js?.error || '登录失败');
                return;
            }
            const t = js.token as string;
            setToken(t);
            localStorage.setItem('aix_token', t);
            setLoginError('');
        } catch (e: any) {
            setLoginError(e?.message || '网络错误');
        } finally {
            setLoginLoading(false);
        }
    }

    const detailLiveRef = useRef<{ open: boolean; kind: string; obj: any }>({ open: false, kind: 'json', obj: null });

    useEffect(() => {
        detailLiveRef.current = { open: detailOpen, kind: detailKind, obj: detailObj };
    }, [detailOpen, detailKind, detailObj]);


    function cleanupDetailState() {
        wsRef.current?.close();
        setDetailProject('');
        setDetailProjectName('');
        setLabGroups([]);
        setGroupDetails({});
        setActiveGroupId('');
        setControlBrain(null);
        setSharedResources(null);
        setProjectStatus('');
        setProjectIsRunning(false);
        setBrainFinished(false);
        setChatChannelId('decision_makers');
        setHostCommandApprovals([]);
        setBackgroundProcesses([]);
        setMemoryDebugTargets([]);
        setHostCommandDecisionNotes({});
        setHostCommandDecisionLoading({});
        setBackgroundKillLoading({});
        setMemoryDebugExporting({});
        setReportSummaryGenerating(false);
        setEvents([]);
    }

    function applyRouteFromHash() {
        const h = String(window.location?.hash ?? '');
        const raw = h.startsWith('#') ? h.slice(1) : h;
        const p = raw.startsWith('/') ? raw : '/';

        // Routes:
        // - #/                 => home
        // - #/project/:name    => detail
        if (p === '/' || p === '') {
            cleanupDetailState();
            setView('home');
            return;
        }

        if (p === '/settings') {
            cleanupDetailState();
            setView('settings');
            return;
        }

        if (p === '/mcp') {
            cleanupDetailState();
            setView('mcp');
            return;
        }

        if (p === '/skills') {
            cleanupDetailState();
            setView('skills');
            return;
        }

        if (p === '/digital_humans') {
            cleanupDetailState();
            setView('digital_humans');
            return;
        }

        if (p === '/report_templates') {
            cleanupDetailState();
            setView('report_templates');
            return;
        }

        if (p === '/graph') {
            cleanupDetailState();
            setView('graph');
            return;
        }

        const m = p.match(/^\/project\/(.+)$/);
        if (m) {
            const projectID = decodeURIComponent(m[1]);
            setDetailProject(projectID);
            setView('detail');
            return;
        }

        // Unknown hash => home
        cleanupDetailState();
        setView('home');
    }

    useEffect(() => {
        // Hash routing: always load at '/', use hash to switch views.
        applyRouteFromHash();
        const onHash = () => applyRouteFromHash();
        window.addEventListener('hashchange', onHash);
        return () => window.removeEventListener('hashchange', onHash);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    // Auto-fetch data when view changes and auth is ready.
    useEffect(() => {
        if (!authHeader || !initChecked) return;
        if (view === 'settings') {
            fetchConfig();
        }
        if (view === 'mcp') fetchMCPSettings();
        if (view === 'skills') fetchSkillsSettings();
        if (view === 'digital_humans') fetchDigitalHumans();
        if (view === 'report_templates') fetchReportTemplates();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [view, authHeader, initChecked]);

    useEffect(() => {
        (async () => {
            let resolvedBase = '';
            try {
                const url = await GetAPIBaseURL();
                const user = await GetBasicAuthUser();
                const pass = await GetBasicAuthPassword();
                setBaseURL(url);
                resolvedBase = url;
                setAuth({ user, pass });
                setInitError('');
            } catch (e: any) {
                const origin = (typeof window !== 'undefined' && window.location && window.location.origin) ? window.location.origin : '';
                let useLocalDevAPI = false;
                try {
                    if (origin && origin !== 'null') {
                        const u = new URL(origin);
                        const host = String(u.hostname || '').toLowerCase();
                        const port = String(u.port || '');
                        if ((host === 'localhost' || host === '127.0.0.1') && port && port !== '9999') {
                            useLocalDevAPI = true;
                        }
                    }
                } catch {
                    // ignore
                }
                if (useLocalDevAPI) {
                    resolvedBase = 'http://127.0.0.1:9999';
                    setBaseURL(resolvedBase);
                } else {
                    setBaseURL('');
                }
                setInitError(String(e?.message ?? e ?? 'init failed'));
            }
            // Check init status (first-run detection).
            try {
                const initBase = resolvedBase || '';
                const resp = await fetch(`${initBase}/init_status`);
                const js = await resp.json();
                const dimgs = js.docker_images || {};
                setDockerImages(dimgs);
                if (js.needs_user) {
                    setNeedsInit(true);
                    setInitStep('user');
                } else if (!dimgs['aisandbox'] || !dimgs['java_env']) {
                    setNeedsInit(true);
                    setInitStep('docker');
                }
            } catch {
                // Server not reachable yet, skip init check.
            }
            // Validate stored token: tokenSecret regenerates on server restart,
            // so a stale localStorage token must be cleared to show the login page.
            const storedToken = localStorage.getItem('aix_token');
            if (storedToken) {
                try {
                    const hResp = await fetch(`${resolvedBase || ''}/healthz`, {
                        headers: { Authorization: 'Bearer ' + storedToken },
                    });
                    if (hResp.status === 401) {
                        localStorage.removeItem('aix_token');
                        setToken('');
                    }
                } catch {
                    // Server not reachable, keep token for now.
                }
            }
            setInitChecked(true);
        })();
    }, []);

    function badgeVariantForExploitState(s: any): any {
        const v = String(s ?? '');
        if (!v) return 'default';
        if (v.includes('可利用')) return 'success';
        if (v.includes('正在验证')) return 'warning';
        if (v.includes('验证失败') || v.includes('审核失败') || v.toLowerCase().includes('fail')) return 'destructive';
        return 'secondary';
    }

    function badgeVariantForHostApprovalStatus(s: any): any {
        const v = String(s ?? '').trim().toLowerCase();
        if (!v) return 'secondary';
        if (v.includes('pending')) return 'warning';
        if (v === 'approved' || v === 'starting' || v === 'running' || v === 'terminating') return 'warning';
        if (v === 'completed') return 'success';
        if (v === 'failed' || v === 'rejected' || v === 'cancelled' || v === 'terminated') return 'destructive';
        return 'secondary';
    }

    function badgeVariantForBackgroundStatus(s: any): any {
        const v = String(s ?? '').trim().toLowerCase();
        if (!v) return 'secondary';
        if (v === 'starting' || v === 'running' || v === 'terminating') return 'warning';
        if (v === 'completed') return 'success';
        if (v === 'failed' || v === 'terminated' || v === 'cancelled') return 'destructive';
        return 'secondary';
    }

    function formatRuntimeSeconds(input: any): string {
        const sec = Math.max(0, Number(input ?? 0) || 0);
        if (sec < 60) return `${sec}s`;
        const min = Math.floor(sec / 60);
        const rem = sec % 60;
        if (min < 60) return rem > 0 ? `${min}m ${rem}s` : `${min}m`;
        const hour = Math.floor(min / 60);
        const minRem = min % 60;
        return minRem > 0 ? `${hour}h ${minRem}m` : `${hour}h`;
    }

    function getRoleLabel(toolName: string): string {
        const t = String(toolName || '');
        if (t.includes('Agent-General-')) return '通用杂项专家';
        if (t.includes('Agent-Ops-OpsEnvScoutAgent')) return '远程环境运维工程师';
        if (t.includes('Agent-Ops-OpsCommonAgent')) return '环境运维专家';
        if (t.includes('Agent-Analyze-')) return '代码审计专家';
        if (t.includes('Agent-Verifier-')) return '漏洞验证专家';
        if (t.includes('Agent-Report-')) return '报告编写专员';
        if (t.includes('Pentest-DecisionBrain')) return '渗透测试组决策大脑';
        if (t.includes('Binary-DecisionBrain')) return '二进制组决策大脑';
        if (t.includes('Agent-Pentest-PentestCommonAgent')) return '渗透突破专家';
        if (t.includes('Agent-Pentest-SrcCommonAgent')) return '黑盒漏洞研究员';
        if (t.includes('Agent-Pentest-PostExploitationCommonAgent')) return '后渗透研究员';
        if (t.includes('Agent-Binary-Reverse')) return '逆向分析研究员';
        if (t.includes('Agent-Binary-Android')) return 'Android逆向研究员';
        if (t.includes('Agent-Binary-Report')) return '二进制报告研究员';
        if (t.includes('Agent-Binary-Verify')) return '样本验证研究员';
        if (t.includes('Agent-')) return '研究员';
        return t || '研究员';
    }

    function normalizeChatMessage(m: any): LabChatMessage {
        const channelId = String(m?.channel_id ?? m?.channelId ?? (m?.persona_name === '总脑' ? 'decision_makers' : 'group:web_security') ?? 'decision_makers');
        return {
            role: (m?.role === 'user' ? 'user' : 'system') as 'user' | 'system',
            text: String(m?.text ?? m?.message ?? ''),
            ts: String(m?.ts ?? ''),
            persona_name: m?.persona_name || undefined,
            avatar_file: m?.avatar_file || undefined,
            channel_id: channelId,
            channel_name: String(m?.channel_name ?? m?.channelName ?? (channelId === 'decision_makers' ? '决策者群' : '组内群')),
            group_id: m?.group_id || m?.groupId || undefined,
            group_name: m?.group_name || m?.groupName || undefined,
            sender_type: m?.sender_type || m?.senderType || undefined,
        };
    }

    function normalizeHostCommandApproval(item: any): HostCommandApproval {
        return {
            id: String(item?.id ?? ''),
            kind: item?.kind ? String(item.kind) : undefined,
            title: item?.title ? String(item.title) : undefined,
            target: item?.target ? String(item.target) : undefined,
            action: item?.action ? String(item.action) : undefined,
            impact: item?.impact ? String(item.impact) : undefined,
            safer_alternative: item?.safer_alternative ? String(item.safer_alternative) : undefined,
            status: String(item?.status ?? ''),
            command: String(item?.command ?? ''),
            reason: String(item?.reason ?? ''),
            workdir: item?.workdir ? String(item.workdir) : undefined,
            timeout_sec: Number(item?.timeout_sec ?? 0) || undefined,
            max_output_chars: Number(item?.max_output_chars ?? 0) || undefined,
            created_at: item?.created_at ? String(item.created_at) : undefined,
            updated_at: item?.updated_at ? String(item.updated_at) : undefined,
            decision_at: item?.decision_at ? String(item.decision_at) : undefined,
            decision_note: item?.decision_note ? String(item.decision_note) : undefined,
            reviewer: item?.reviewer ? String(item.reviewer) : undefined,
            group_id: item?.group_id ? String(item.group_id) : undefined,
            group_name: item?.group_name ? String(item.group_name) : undefined,
            requester_source: item?.requester_source ? String(item.requester_source) : undefined,
            agent_tool_name: item?.agent_tool_name ? String(item.agent_tool_name) : undefined,
            digital_human_id: item?.digital_human_id ? String(item.digital_human_id) : undefined,
            persona_name: item?.persona_name ? String(item.persona_name) : undefined,
            avatar_file: item?.avatar_file ? String(item.avatar_file) : undefined,
            exit_code: Number(item?.exit_code ?? 0) || undefined,
            output_preview: item?.output_preview ? String(item.output_preview) : undefined,
            error: item?.error ? String(item.error) : undefined,
            output_truncated: !!item?.output_truncated,
            timed_out: !!item?.timed_out,
        };
    }

    function normalizeBackgroundProcess(item: any): BackgroundProcess {
        return {
            session_id: String(item?.session_id ?? ''),
            source: String(item?.source ?? ''),
            command: String(item?.command ?? ''),
            reason: item?.reason ? String(item.reason) : undefined,
            workdir: item?.workdir ? String(item.workdir) : undefined,
            status: String(item?.status ?? ''),
            running: !!item?.running,
            pid: Number(item?.pid ?? 0) || undefined,
            started_at: item?.started_at ? String(item.started_at) : undefined,
            updated_at: item?.updated_at ? String(item.updated_at) : undefined,
            ended_at: item?.ended_at ? String(item.ended_at) : undefined,
            running_for_sec: Number(item?.running_for_sec ?? 0) || 0,
            exit_code: Number(item?.exit_code ?? 0),
            timed_out: !!item?.timed_out,
            terminated_by: item?.terminated_by ? String(item.terminated_by) : undefined,
            termination_reason: item?.termination_reason ? String(item.termination_reason) : undefined,
            error: item?.error ? String(item.error) : undefined,
            output_preview: item?.output_preview ? String(item.output_preview) : undefined,
            request_id: item?.request_id ? String(item.request_id) : undefined,
            agent_tool_name: item?.agent_tool_name ? String(item.agent_tool_name) : undefined,
            agent_name: item?.agent_name ? String(item.agent_name) : undefined,
            digital_human_id: item?.digital_human_id ? String(item.digital_human_id) : undefined,
            persona_name: item?.persona_name ? String(item.persona_name) : undefined,
            avatar_file: item?.avatar_file ? String(item.avatar_file) : undefined,
            group_id: item?.group_id ? String(item.group_id) : undefined,
            group_name: item?.group_name ? String(item.group_name) : undefined,
            requester_source: item?.requester_source ? String(item.requester_source) : undefined,
        };
    }

    function normalizeMemoryDebugTarget(item: any): MemoryDebugTarget {
        return {
            target_id: String(item?.target_id ?? ''),
            group_id: item?.group_id ? String(item.group_id) : undefined,
            group_name: item?.group_name ? String(item.group_name) : undefined,
            target_type: String(item?.target_type ?? ''),
            label: String(item?.label ?? ''),
            persona_name: item?.persona_name ? String(item.persona_name) : undefined,
            agent_tool_name: item?.agent_tool_name ? String(item.agent_tool_name) : undefined,
            digital_human_id: item?.digital_human_id ? String(item.digital_human_id) : undefined,
            agent_id: item?.agent_id ? String(item.agent_id) : undefined,
            state: item?.state ? String(item.state) : undefined,
            available: !!item?.available,
            reason: item?.reason ? String(item.reason) : undefined,
        };
    }

    function getChatChannels(groups: any[], controlBrainObj: any) {
        const out: { id: string; name: string; subtitle: string }[] = [];
        out.push({
            id: 'decision_makers',
            name: '决策者群',
            subtitle: controlBrainObj ? '总脑 + 各组决策大脑' : '总脑',
        });
        (groups || []).forEach((group: any) => {
            const gid = String(group?.groupId ?? '');
            const gname = String(group?.groupName ?? '');
            if (!gid || !gname) return;
            out.push({
                id: `group:${gid}`,
                name: `${gname}·组内群`,
                subtitle: String(group?.description ?? ''),
            });
        });
        return out;
    }

    function getDecisionBrainName(group: any): string {
        return String(group?.decisionBrain?.name ?? `${String(group?.groupName ?? '')}·决策大脑`);
    }

    function getGroupId(group: any): string {
        return String(group?.groupId ?? group?.group_id ?? '');
    }

    function getGroupDetailRecord(details: Record<string, any>, groupId: string): any {
        if (!groupId) return null;
        const direct = details?.[groupId];
        return direct && typeof direct === 'object' ? direct : null;
    }

    function countRosterMembers(roster: any): number {
        if (!roster || typeof roster !== 'object') return 0;
        return Object.values(roster).reduce<number>((total, list) => total + (Array.isArray(list) ? list.length : 0), 0);
    }

    function sanitizeGroupDetail(detail: any): any {
        if (!detail || typeof detail !== 'object') return detail;
        const next = { ...detail };
        if ('reports' in next) {
            delete next.reports;
        }
        return next;
    }

    function getChannelParticipants(channelId: string, groups: any[], groupDetailMap: Record<string, any>, controlBrainObj: any) {
        if (channelId === 'decision_makers') {
            const participants: { name: string; avatar: string; state: string; role: string }[] = [];
            if (controlBrainObj) {
                participants.push({ name: '总脑', avatar: 'system.png', state: String(controlBrainObj?.status ?? '待命'), role: '实验室总控' });
            }
            (groups || []).forEach((group: any) => {
                participants.push({
                    name: getDecisionBrainName(group),
                    avatar: 'system.png',
                    state: String(group?.status ?? '待命'),
                    role: '组决策大脑',
                });
            });
            return participants;
        }
        const targetGroupId = String(channelId || '').replace(/^group:/, '');
        const group = (groups || []).find((g: any) => getGroupId(g) === targetGroupId);
        const groupDetail = getGroupDetailRecord(groupDetailMap, targetGroupId);
        if (!group) return [];
        const participants: { name: string; avatar: string; state: string; role: string }[] = [
            {
                name: getDecisionBrainName(group),
                avatar: 'system.png',
                state: String(group?.status ?? '待命'),
                role: '组决策大脑',
            }
        ];
        const roster = groupDetail?.digitalHumans || group?.digitalHumans || {};
        Object.entries(roster).forEach(([toolName, list]) => {
            if (!Array.isArray(list)) return;
            list.forEach((h: any) => {
                const personaName = String(h?.persona_name ?? '');
                if (!personaName) return;
                participants.push({
                    name: personaName,
                    avatar: String(h?.avatar_file ?? 'system.png'),
                    state: String(h?.state ?? 'idle'),
                    role: getRoleLabel(String(toolName)),
                });
            });
        });
        return participants;
    }

    function buildChatShortcutTargets(channelId: string, groups: any[], groupDetailMap: Record<string, any>, controlBrainObj: any) {
        if (channelId === 'decision_makers') {
            const targets = [{ label: '总脑', value: '', description: '默认发给总脑' }];
            (groups || []).forEach((group: any) => {
                targets.push({
                    label: getDecisionBrainName(group),
                    value: `@${getDecisionBrainName(group)} `,
                    description: '在决策者群中定向给组决策大脑',
                });
            });
            return targets;
        }
        const targetGroupId = String(channelId || '').replace(/^group:/, '');
        const group = (groups || []).find((g: any) => getGroupId(g) === targetGroupId);
        const groupDetail = getGroupDetailRecord(groupDetailMap, targetGroupId);
        if (!group) return [];
        const targets = [
            { label: '组决策脑', value: `@${getDecisionBrainName(group)} `, description: '定向给本组决策大脑' },
            { label: '@all', value: '@all ', description: '广播给本组成员' },
        ];
        const roster = groupDetail?.digitalHumans || group?.digitalHumans || {};
        Object.values(roster).forEach((list: any) => {
            if (!Array.isArray(list)) return;
            list.forEach((h: any) => {
                const personaName = String(h?.persona_name ?? '');
                if (!personaName) return;
                targets.push({
                    label: `@${personaName}`,
                    value: `@${personaName} `,
                    description: '定向给本组成员',
                });
            });
        });
        return targets;
    }

    function upsertGroupDetail(groupId: string, updater: (current: any) => any) {
        if (!groupId) return;
        setGroupDetails((prev) => {
            const current = prev?.[groupId] && typeof prev[groupId] === 'object' ? prev[groupId] : { groupId };
            const nextValue = updater(current) ?? current;
            return { ...prev, [groupId]: { ...current, ...nextValue, groupId } };
        });
    }

    function mergeGroupSummariesIntoDetails(groups: any[]) {
        setGroupDetails((prev) => {
            const next = { ...prev };
            (groups || []).forEach((group: any) => {
                const gid = getGroupId(group);
                if (!gid) return;
                const current = next?.[gid] && typeof next[gid] === 'object' ? next[gid] : { groupId: gid };
                next[gid] = sanitizeGroupDetail({
                    ...current,
                    groupId: gid,
                    groupName: String(group?.groupName ?? current?.groupName ?? ''),
                    groupKind: String(group?.groupKind ?? current?.groupKind ?? ''),
                    status: String(group?.status ?? current?.status ?? ''),
                    isRunning: Boolean(group?.isRunning ?? current?.isRunning),
                    brainFinished: Boolean(group?.brainFinished ?? current?.brainFinished),
                    isDemo: Boolean(group?.isDemo ?? current?.isDemo),
                    decisionBrain: group?.decisionBrain ?? current?.decisionBrain,
                    digitalHumans: group?.digitalHumans ?? current?.digitalHumans,
                    agentRuns: Array.isArray(group?.agentRuns) ? group.agentRuns : current?.agentRuns,
                });
            });
            return next;
        });
    }

    function shortText(x: any, max = 240): string {
        const s = String(x ?? '');
        if (s.length <= max) return s;
        return s.slice(0, max) + '…';
    }

    function isTruncated(x: any, max = 240): boolean {
        const s = String(x ?? '');
        return s.length > max;
    }

    function normalizeProjectList(data: any): ProjectListItem[] {
        if (!Array.isArray(data)) return [];
        const out: ProjectListItem[] = [];
        for (const item of data) {
            if (typeof item === 'string') {
                const name = item.trim();
                if (name) out.push({ projectID: name, projectName: name });
                continue;
            }
            if (!item || typeof item !== 'object') continue;
            const name = String(item.projectName ?? item.project_name ?? item.name ?? '').trim();
            if (!name) continue;
			const id = String(item.projectID ?? item.project_id ?? item.projectUUID ?? item.project_uuid ?? name).trim();
            out.push({
                projectID: id || name,
                projectName: name,
                labName: String(item.labName ?? item.lab_name ?? name),
                status: String(item.status ?? ''),
                rawStatus: String(item.rawStatus ?? item.raw_status ?? ''),
                isRunning: !!item.isRunning,
                isStarting: !!item.isStarting,
                isQueued: !!item.isQueued,
                queuePosition: Number(item.queuePosition ?? 0) || 0,
                brainFinished: !!item.brainFinished,
                groupCount: Number(item.groupCount ?? item.group_count ?? 0) || 0,
                startTime: String(item.startTime ?? ''),
                endTime: String(item.endTime ?? ''),
            });
        }
        return out;
    }

    async function apiPost<T>(path: string, body: any): Promise<T | undefined> {
        let useBase = baseURL;
        if (!useBase) {
            try {
                const u = await GetAPIBaseURL();
                if (u) { useBase = u; setBaseURL(u); }
            } catch { /* ignore */ }
        }
        const url = `${useBase}${path}`;
        const resp = await fetch(url, {
            method: 'POST',
            headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        });
        if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
        if (!resp.ok) throw new Error(`apiPost failed: ${resp.status}`);
        const js = (await resp.json()) as ApiResp<T>;
        return getRespData(js);
    }

    async function apiGet<T>(path: string): Promise<T | undefined> {
        let useBase = baseURL;
        if (!useBase) {
            try {
                const u = await GetAPIBaseURL();
                if (u) {
                    useBase = u;
                    setBaseURL(u);
                }
            } catch {
                // ignore
            }
        }

        const url = `${useBase}${path}`;
        let resp: Response;
        try {
            resp = await fetch(url, {
                headers: {
                    Authorization: authHeader,
                },
            });
        } catch (e: any) {
            console.error('apiGet network error', { url, error: e });
            throw e;
        }

        if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
        if (!resp.ok) {
            let text = '';
            try {
                text = await resp.text();
            } catch {
                // ignore
            }
            console.error('apiGet non-2xx', { url, status: resp.status, body: text });
            throw new Error(`apiGet failed: ${resp.status}`);
        }

        const js = (await resp.json()) as ApiResp<T>;
        return getRespData(js);
    }

    async function refreshProjects() {
        if (!authHeader) return;

        const data = await apiGet<any>('/projects');
        setProjects(normalizeProjectList(data));
    }

    async function refreshChatMessages(projectName: string) {
        const trimmed = String(projectName ?? '').trim();
        if (!trimmed) {
            setChatMessages([]);
            return;
        }
        const name = encodeURIComponent(trimmed);
        const chatMsgs = await apiGet<any[]>(`/projects/${name}/chat/messages`);
        if (Array.isArray(chatMsgs)) {
            setChatMessages(chatMsgs.map((m: any) => normalizeChatMessage(m)));
            return;
        }
        setChatMessages([]);
    }

    useEffect(() => {
        refreshProjects();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [authHeader]);

    useEffect(() => {
        if (!authHeader) return;
        const hasPendingStarts = projects.some((p) => !!p.isQueued || !!p.isStarting);
        if (!hasPendingStarts) return;
        const timer = window.setInterval(() => {
            refreshProjects().catch(() => { /* ignore */ });
        }, 2000);
        return () => window.clearInterval(timer);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [authHeader, projects]);

    async function createProject() {
        if (!authHeader) return;
        if (sourceType === 'file' && !fileRef.current?.files?.[0]) { showToast('请选择源码压缩包', 'err'); return; }
        if (sourceType === 'git' && !gitUrl.trim()) { showToast('请输入 Git 仓库地址', 'err'); return; }
        if (sourceType === 'url' && !fileUrl.trim()) { showToast('请输入压缩包下载链接', 'err'); return; }
        setBtnLoading(prev => ({ ...prev, create: true }));
        try {
            const fd = new FormData();
            fd.append('source_type', sourceType);
            fd.append('taskContent', taskContent);
            if (sourceType === 'file') {
                fd.append('file', fileRef.current!.files![0]);
            } else if (sourceType === 'git') {
                fd.append('git_url', gitUrl.trim());
            } else if (sourceType === 'url') {
                fd.append('file_url', fileUrl.trim());
            }
            const name = projectName.trim();
            const q = name ? `?projectName=${encodeURIComponent(name)}` : '';
            const resp = await fetch(`${baseURL}/projects/create${q}`, {
                method: 'POST',
                headers: { Authorization: authHeader },
                body: fd,
            });
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast(js?.error || '创建失败', 'err');
                return;
            }
            await refreshProjects();
            showToast('实验室创建成功');
        } catch (e: any) {
            showToast(e?.message || '创建失败', 'err');
        } finally {
            setBtnLoading(prev => ({ ...prev, create: false }));
        }
    }

	async function enterProject(projectID: string) {
		window.location.hash = `#/project/${encodeURIComponent(projectID)}`;
	}

    function leaveProject() {
        window.location.hash = '#/';
    }

    function sanitizeConfigForUI(input: Record<string, Record<string, string>>): Record<string, Record<string, string>> {
        const hiddenKeys = new Set([
            'MCP_SERVERS',
            'GITHUB_COPILOT_MCP_ENABLED',
            'GITHUB_COPILOT_MCP_URL',
            'GITHUB_COPILOT_MCP_HEADERS',
            'GITHUB_COPILOT_MCP_TOOL_PREFIX',
            'GITHUB_COPILOT_MCP_AUTHORIZATION',
        ]);
        const out: Record<string, Record<string, string>> = {};
        Object.entries(input || {}).forEach(([section, kv]) => {
            const sec: Record<string, string> = {};
            Object.entries(kv || {}).forEach(([key, value]) => {
                if (hiddenKeys.has(key)) return;
                sec[key] = value;
            });
            out[section] = sec;
        });
        return out;
    }

    async function fetchConfig() {
        setConfigLoading(true);
        try {
            const resp = await fetch(`${baseURL}/config`, { headers: { Authorization: authHeader } });
            const js = await resp.json();
            const d = getRespData<Record<string, Record<string, string>>>(js) ?? {};
            const sanitized = sanitizeConfigForUI(d);
            setConfigData(JSON.parse(JSON.stringify(sanitized)));
            setConfigDraft(JSON.parse(JSON.stringify(sanitized)));
            setLLMTestResults({});
        } catch (e: any) {
            showToast('加载配置失败: ' + (e?.message || ''), 'err');
        } finally {
            setConfigLoading(false);
        }
    }

    async function saveConfig() {
        setConfigSaving(true);
        try {
            const sanitized = sanitizeConfigForUI(configDraft);
            const resp = await fetch(`${baseURL}/config`, {
                method: 'PUT',
                headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
                body: JSON.stringify(sanitized),
            });
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast('保存失败: ' + (js?.error || ''), 'err');
                return;
            }
            showToast(getRespData<string>(js) || '保存成功');
            setConfigData(JSON.parse(JSON.stringify(sanitized)));
            setConfigDraft(JSON.parse(JSON.stringify(sanitized)));
            setLLMTestResults({});
        } catch (e: any) {
            showToast('保存失败: ' + (e?.message || ''), 'err');
        } finally {
            setConfigSaving(false);
        }
    }

    async function fetchMCPSettings() {
        setMCPLoading(true);
        try {
            const resp = await fetch(`${baseURL}/mcp/settings`, { headers: { Authorization: authHeader } });
            const js = await resp.json();
            const data = getRespData<{ configs?: MCPConfigRow[]; agent_targets?: MCPAgentTarget[] }>(js) || {};
            const configs = (data.configs || []).map(normalizeMCPConfigRow);
            setMCPConfigs(configs);
            setMCPAgentTargets(Array.isArray(data.agent_targets) ? data.agent_targets : []);
            setMCPArgDrafts(prev => {
                const next: Record<string, string> = {};
                configs.forEach(item => {
                    if (prev[item.id]) next[item.id] = prev[item.id];
                });
                return next;
            });
            setMCPExpanded(prev => {
                const next: Record<string, boolean> = {};
                configs.forEach((item, index) => {
                    next[item.id] = prev[item.id] ?? index === 0;
                });
                return next;
            });
        } catch (e: any) {
            showToast('加载 MCP 设置失败: ' + (e?.message || ''), 'err');
        } finally {
            setMCPLoading(false);
        }
    }

    async function saveMCPSettings() {
        setMCPSaving(true);
        try {
            const payload = { configs: mcpConfigs.map(normalizeMCPConfigRow) };
            const resp = await fetch(`${baseURL}/mcp/settings`, {
                method: 'PUT',
                headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast('MCP 保存失败: ' + (js?.error || ''), 'err');
                return;
            }
            showToast(getRespData<string>(js) || 'MCP 配置已保存');
            await fetchMCPSettings();
        } catch (e: any) {
            showToast('MCP 保存失败: ' + (e?.message || ''), 'err');
        } finally {
            setMCPSaving(false);
        }
    }

    async function testMCPConfigRow(row: MCPConfigRow) {
        const normalized = normalizeMCPConfigRow(row);
        setMCPTesting(prev => ({ ...prev, [normalized.id]: true }));
        try {
            const resp = await fetch(`${baseURL}/mcp/test`, {
                method: 'POST',
                headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
                body: JSON.stringify(normalized),
            });
            const js = await resp.json();
            const result = getRespData<MCPTestResult>(js);
            if (!resp.ok || !result) {
                showToast('MCP 测试失败: ' + (js?.error || ''), 'err');
                return;
            }
            setMCPTestResults(prev => ({ ...prev, [normalized.id]: result }));
            showToast(result.ok ? 'MCP 连接测试成功' : `MCP 连接失败: ${result.message || '未知错误'}`, result.ok ? 'ok' : 'err');
        } catch (e: any) {
            showToast('MCP 测试失败: ' + (e?.message || ''), 'err');
        } finally {
            setMCPTesting(prev => ({ ...prev, [normalized.id]: false }));
        }
    }

    function updateMCPConfig(id: string, patch: Partial<MCPConfigRow>) {
        setMCPConfigs(prev => prev.map(item => item.id === id ? normalizeMCPConfigRow({ ...item, ...patch }) : item));
    }

    function updateMCPArg(id: string, index: number, value: string) {
        setMCPConfigs(prev => prev.map(item => {
            if (item.id !== id) return item;
            const args = normalizeOrderedStringArray(item.args);
            if (index < 0 || index >= args.length) return item;
            const trimmed = value.trim();
            if (!trimmed) {
                args.splice(index, 1);
            } else {
                args[index] = trimmed;
            }
            return normalizeMCPConfigRow({ ...item, args });
        }));
    }

    function removeMCPArg(id: string, index: number) {
        setMCPConfigs(prev => prev.map(item => {
            if (item.id !== id) return item;
            const args = normalizeOrderedStringArray(item.args);
            if (index < 0 || index >= args.length) return item;
            args.splice(index, 1);
            return normalizeMCPConfigRow({ ...item, args });
        }));
    }

    function updateMCPArgDraft(id: string, value: string) {
        setMCPArgDrafts(prev => ({ ...prev, [id]: value }));
    }

    function commitMCPArgDraft(id: string) {
        const draft = String(mcpArgDrafts[id] || '').trim();
        if (!draft) return;
        setMCPConfigs(prev => prev.map(item => {
            if (item.id !== id) return item;
            const args = [...normalizeOrderedStringArray(item.args), draft];
            return normalizeMCPConfigRow({ ...item, args });
        }));
        setMCPArgDrafts(prev => ({ ...prev, [id]: '' }));
    }

    function addMCPConfig() {
        const row = emptyMCPConfig();
        setMCPConfigs(prev => [...prev, row]);
        setMCPExpanded(prev => ({ ...prev, [row.id]: true }));
    }

    function removeMCPConfig(id: string) {
        setMCPConfigs(prev => prev.filter(item => item.id !== id));
        setMCPTestResults(prev => {
            const next = { ...prev };
            delete next[id];
            return next;
        });
        setMCPExpanded(prev => {
            const next = { ...prev };
            delete next[id];
            return next;
        });
        setMCPArgDrafts(prev => {
            const next = { ...prev };
            delete next[id];
            return next;
        });
    }

    function toggleMCPAgentType(id: string, agentType: string) {
        setMCPConfigs(prev => prev.map(item => {
            if (item.id !== id) return item;
            const bindings = normalizeMCPAgentBindings(item.agent_bindings, item.agent_types);
            const exists = bindings.some(binding => binding.agent_type === agentType);
            const next = exists
                ? bindings.filter(binding => binding.agent_type !== agentType)
                : [...bindings, { agent_type: agentType, tool_allowlist: [], tool_denylist: [] }];
            return normalizeMCPConfigRow({
                ...item,
                agent_bindings: next,
                agent_types: agentTypesFromBindings(next),
            });
        }));
    }

    function updateMCPAgentBinding(id: string, agentType: string, patch: Partial<MCPAgentBinding>) {
        setMCPConfigs(prev => prev.map(item => {
            if (item.id !== id) return item;
            const bindings = normalizeMCPAgentBindings(item.agent_bindings, item.agent_types);
            let found = false;
            const next = bindings.map(binding => {
                if (binding.agent_type !== agentType) return binding;
                found = true;
                return {
                    ...binding,
                    ...patch,
                    agent_type: agentType,
                    tool_allowlist: normalizeStringArray(patch.tool_allowlist ?? binding.tool_allowlist),
                    tool_denylist: normalizeStringArray(patch.tool_denylist ?? binding.tool_denylist),
                };
            });
            if (!found) {
                next.push({
                    agent_type: agentType,
                    tool_allowlist: normalizeStringArray(patch.tool_allowlist),
                    tool_denylist: normalizeStringArray(patch.tool_denylist),
                });
            }
            return normalizeMCPConfigRow({
                ...item,
                agent_bindings: next,
                agent_types: agentTypesFromBindings(next),
            });
        }));
    }

    function toggleMCPExpanded(id: string) {
        setMCPExpanded(prev => ({ ...prev, [id]: !prev[id] }));
    }

    async function fetchSkillsSettings(preferredSkillName?: string, preferredFilePath?: string) {
        setSkillsLoading(true);
        try {
            const resp = await fetch(`${baseURL}/skills/settings`, { headers: { Authorization: authHeader } });
            if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
            const js = await resp.json();
            const data = getRespData<SkillSettingsResponse>(js) || {};
            if (!resp.ok || js?.error) {
                showToast('加载 Skills 失败: ' + (js?.error || ''), 'err');
                return;
            }
            const items = Array.isArray(data.skills) ? [...data.skills].sort((a, b) => String(a.name || '').localeCompare(String(b.name || ''))) : [];
            setSkillSearchPathsDraft(String(data.search_paths || ''));
            setSkillResolvedRoots(Array.isArray(data.resolved_roots) ? data.resolved_roots.map(item => String(item || '')) : []);
            setSkillManagedRoot(String(data.managed_root || ''));
            setSkillItems(items);
            const fallbackSelected = preferredSkillName || selectedSkillName;
            const nextSelected = items.some(item => item.name === fallbackSelected) ? fallbackSelected : (items[0]?.name || '');
            setSelectedSkillName(nextSelected);
            if (!nextSelected) {
                setSkillFiles([]);
                setSelectedSkillPath('');
                setSkillFileContent('');
                setSkillFileDirty(false);
                return;
            }
            await loadSkillFiles(nextSelected, preferredFilePath);
        } catch (e: any) {
            showToast('加载 Skills 失败: ' + (e?.message || ''), 'err');
        } finally {
            setSkillsLoading(false);
        }
    }

    async function saveSkillSettings() {
        setSkillsSaving(true);
        try {
            const resp = await fetch(`${baseURL}/skills/settings`, {
                method: 'PUT',
                headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
                body: JSON.stringify({ search_paths: skillSearchPathsDraft }),
            });
            if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast('保存 Skills 设置失败: ' + (js?.error || ''), 'err');
                return;
            }
            showToast(getRespData<string>(js) || 'Skills 设置已保存');
            await fetchSkillsSettings(selectedSkillName, selectedSkillPath);
        } catch (e: any) {
            showToast('保存 Skills 设置失败: ' + (e?.message || ''), 'err');
        } finally {
            setSkillsSaving(false);
        }
    }

    async function loadSkillFiles(skillName: string, preferredFilePath?: string) {
        if (!skillName) return;
        try {
            const resp = await fetch(`${baseURL}/skills/${encodeURIComponent(skillName)}/files?path=${encodeURIComponent('.')}`, {
                headers: { Authorization: authHeader },
            });
            if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
            const js = await resp.json();
            const data = getRespData<SkillFileList>(js);
            if (!resp.ok || !data) {
                showToast('加载 Skill 文件失败: ' + (js?.error || ''), 'err');
                return;
            }
            const entries = Array.isArray(data.entries) ? data.entries : [];
            setSkillFiles(entries);
            const fileEntries = entries.filter(item => item.type === 'file');
            const nextPath = fileEntries.some(item => item.path === preferredFilePath)
                ? String(preferredFilePath)
                : (fileEntries.find(item => item.path === 'SKILL.md')?.path || fileEntries[0]?.path || '');
            if (!nextPath) {
                setSelectedSkillPath('');
                setSkillFileContent('');
                setSkillFileDirty(false);
                return;
            }
            await openSkillFile(skillName, nextPath, true);
        } catch (e: any) {
            showToast('加载 Skill 文件失败: ' + (e?.message || ''), 'err');
        }
    }

    async function openSkillFile(skillName: string, relativePath: string, skipDirtyConfirm = false) {
        if (!skillName || !relativePath) return;
        if (!skipDirtyConfirm && skillFileDirty && selectedSkillName === skillName && selectedSkillPath !== relativePath) {
            if (!confirm('当前文件有未保存修改，确认切换并丢弃这些修改？')) return;
        }
        setSkillFileLoading(true);
        try {
            setSelectedSkillName(skillName);
            setSelectedSkillPath(relativePath);
            const resp = await fetch(`${baseURL}/skills/${encodeURIComponent(skillName)}/file?path=${encodeURIComponent(relativePath)}`, {
                headers: { Authorization: authHeader },
            });
            if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
            const js = await resp.json();
            const data = getRespData<SkillFileDocument>(js);
            if (!resp.ok || !data) {
                showToast('读取 Skill 文件失败: ' + (js?.error || ''), 'err');
                return;
            }
            setSkillFileContent(String(data.content || ''));
            setSkillFileDirty(false);
        } catch (e: any) {
            showToast('读取 Skill 文件失败: ' + (e?.message || ''), 'err');
        } finally {
            setSkillFileLoading(false);
        }
    }

    async function createSkillAPI() {
        const name = newSkillName.trim();
        if (!name) return;
        setSkillsCreating(true);
        try {
            const resp = await fetch(`${baseURL}/skills`, {
                method: 'POST',
                headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, description: newSkillDescription }),
            });
            if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast('创建 Skill 失败: ' + (js?.error || ''), 'err');
                return;
            }
            showToast('Skill 已创建');
            setNewSkillName('');
            setNewSkillDescription('');
            await fetchSkillsSettings(name, 'SKILL.md');
        } catch (e: any) {
            showToast('创建 Skill 失败: ' + (e?.message || ''), 'err');
        } finally {
            setSkillsCreating(false);
        }
    }

    async function saveSkillFile() {
        if (!selectedSkillName || !selectedSkillPath) return;
        setSkillFileSaving(true);
        try {
            const resp = await fetch(`${baseURL}/skills/${encodeURIComponent(selectedSkillName)}/file`, {
                method: 'PUT',
                headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
                body: JSON.stringify({ relative_path: selectedSkillPath, content: skillFileContent }),
            });
            if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast('保存 Skill 文件失败: ' + (js?.error || ''), 'err');
                return;
            }
            setSkillFileDirty(false);
            showToast('Skill 文件已保存');
            await loadSkillFiles(selectedSkillName, selectedSkillPath);
        } catch (e: any) {
            showToast('保存 Skill 文件失败: ' + (e?.message || ''), 'err');
        } finally {
            setSkillFileSaving(false);
        }
    }

    async function createSkillFile() {
        const path = newSkillFilePath.trim();
        if (!selectedSkillName || !path) return;
        if (skillFileDirty && !confirm('当前文件有未保存修改，确认继续创建新文件并覆盖编辑区内容？')) return;
        setSkillFileSaving(true);
        try {
            const resp = await fetch(`${baseURL}/skills/${encodeURIComponent(selectedSkillName)}/file`, {
                method: 'PUT',
                headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
                body: JSON.stringify({ relative_path: path, content: '' }),
            });
            if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast('创建 Skill 文件失败: ' + (js?.error || ''), 'err');
                return;
            }
            setNewSkillFilePath('');
            showToast('Skill 文件已创建');
            await loadSkillFiles(selectedSkillName, path);
        } catch (e: any) {
            showToast('创建 Skill 文件失败: ' + (e?.message || ''), 'err');
        } finally {
            setSkillFileSaving(false);
        }
    }

    async function deleteSelectedSkillFile() {
        if (!selectedSkillName || !selectedSkillPath) return;
        if (selectedSkillPath === 'SKILL.md') {
            showToast('SKILL.md 不能单独删除，请直接删除整个 Skill', 'err');
            return;
        }
        if (!confirm(`确定删除文件「${selectedSkillPath}」？`)) return;
        setSkillFileSaving(true);
        try {
            const resp = await fetch(`${baseURL}/skills/${encodeURIComponent(selectedSkillName)}/file?path=${encodeURIComponent(selectedSkillPath)}`, {
                method: 'DELETE',
                headers: { Authorization: authHeader },
            });
            if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast('删除 Skill 文件失败: ' + (js?.error || ''), 'err');
                return;
            }
            showToast('Skill 文件已删除');
            setSkillFileDirty(false);
            await loadSkillFiles(selectedSkillName);
        } catch (e: any) {
            showToast('删除 Skill 文件失败: ' + (e?.message || ''), 'err');
        } finally {
            setSkillFileSaving(false);
        }
    }

    async function deleteSkillAPI(name: string) {
        if (!name) return;
        if (!confirm(`确定删除 Skill「${name}」及其全部文件？`)) return;
        setSkillsDeleting(true);
        try {
            const resp = await fetch(`${baseURL}/skills/${encodeURIComponent(name)}`, {
                method: 'DELETE',
                headers: { Authorization: authHeader },
            });
            if (resp.status === 401) { handleLogout(); throw new Error('登录已过期，请重新登录'); }
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast('删除 Skill 失败: ' + (js?.error || ''), 'err');
                return;
            }
            showToast('Skill 已删除');
            if (selectedSkillName === name) {
                setSelectedSkillName('');
                setSelectedSkillPath('');
                setSkillFileContent('');
                setSkillFileDirty(false);
            }
            await fetchSkillsSettings();
        } catch (e: any) {
            showToast('删除 Skill 失败: ' + (e?.message || ''), 'err');
        } finally {
            setSkillsDeleting(false);
        }
    }

    function invalidateLLMTestResults(section: string) {
        setLLMTestResults(prev => {
            if (section === 'main_setting') return {};
            return { ...prev, [section]: null };
        });
    }

    function updateConfigDraft(section: string, key: string, value: string) {
        invalidateLLMTestResults(section);
        setConfigDraft(prev => ({
            ...prev,
            [section]: { ...(prev[section] || {}), [key]: value },
        }));
    }

    function deleteConfigKey(section: string, key: string) {
        invalidateLLMTestResults(section);
        setConfigDraft(prev => {
            const sec = { ...(prev[section] || {}) };
            delete sec[key];
            if (Object.keys(sec).length === 0) {
                const next = { ...prev };
                delete next[section];
                return next;
            }
            return { ...prev, [section]: sec };
        });
    }

    function deleteConfigSection(section: string) {
        invalidateLLMTestResults(section);
        setConfigDraft(prev => {
            const next = { ...prev };
            delete next[section];
            return next;
        });
    }

    function addConfigSection(name: string) {
        if (!name.trim()) return;
        setConfigDraft(prev => ({ ...prev, [name.trim()]: { ...(prev[name.trim()] || {}) } }));
        setNewSectionName('');
    }

    function addConfigKey(section: string) {
        const input = newKeyInputs[section];
        if (!input?.key?.trim()) return;
        updateConfigDraft(section, input.key.trim(), input.value || '');
        setNewKeyInputs(prev => ({ ...prev, [section]: { key: '', value: '' } }));
    }

    async function fetchModels(section: string) {
        const draft = configDraft[section];
        if (!draft) return;
        const baseUrl = getEffectiveConfigValue(section, 'BASE_URL');
        const apiKey = getEffectiveConfigValue(section, 'OPENAI_API_KEY');
        const apiMode = getEffectiveConfigValue(section, 'API_MODE');
        if (!baseUrl || !apiKey) return;
        setModelLoading(prev => ({ ...prev, [section]: true }));
        try {
            const resp = await fetch(`${baseURL}/models?base_url=${encodeURIComponent(baseUrl)}&api_key=${encodeURIComponent(apiKey)}&api_mode=${encodeURIComponent(apiMode)}`, {
                headers: { Authorization: authHeader },
            });
            const js = await resp.json();
            const list = getRespData<string[]>(js) ?? [];
            if (list.length > 0) {
                setModelOptions(prev => ({ ...prev, [section]: list.sort() }));
            } else {
                setModelOptions(prev => ({ ...prev, [section]: [] }));
            }
        } catch {
            setModelOptions(prev => ({ ...prev, [section]: [] }));
        } finally {
            setModelLoading(prev => ({ ...prev, [section]: false }));
        }
    }

    function getEffectiveConfigValue(section: string, key: string): string {
        const directValue = (configDraft[section]?.[key] || '').trim();
        if (directValue) return directValue;
        if (section === 'main_setting') return '';
        return (configDraft['main_setting']?.[key] || '').trim();
    }

    async function testLLM(section: string) {
        const baseUrl = getEffectiveConfigValue(section, 'BASE_URL');
        const apiKey = getEffectiveConfigValue(section, 'OPENAI_API_KEY');
        const apiMode = getEffectiveConfigValue(section, 'API_MODE');
        const userAgent = getEffectiveConfigValue(section, 'USER_AGENT');
        const model = getEffectiveConfigValue(section, 'MODEL');

        if (!baseUrl || !apiKey || !model) {
            setLLMTestResults(prev => ({
                ...prev,
                [section]: {
                    ok: false,
                    message: `缺少必要配置：${[
                        !baseUrl ? 'BASE_URL' : '',
                        !apiKey ? 'OPENAI_API_KEY' : '',
                        !model ? 'MODEL' : '',
                    ].filter(Boolean).join(', ')}`,
                },
            }));
            return;
        }

        setLLMTestLoading(prev => ({ ...prev, [section]: true }));
        setLLMTestResults(prev => ({ ...prev, [section]: null }));
        try {
            const resp = await fetch(`${baseURL}/llm/test`, {
                method: 'POST',
                headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    section,
                    base_url: baseUrl,
                    api_key: apiKey,
                    api_mode: apiMode,
                    user_agent: userAgent,
                    model,
                }),
            });
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                setLLMTestResults(prev => ({
                    ...prev,
                    [section]: {
                        ok: false,
                        message: js?.error || `HTTP ${resp.status}`,
                        backend: getRespData<LLMTestResult>(js)?.backend,
                    },
                }));
                return;
            }
            const result = getRespData<LLMTestResult>(js);
            setLLMTestResults(prev => ({
                ...prev,
                [section]: result || { ok: false, message: '测试返回为空' },
            }));
        } catch (e: any) {
            setLLMTestResults(prev => ({
                ...prev,
                [section]: {
                    ok: false,
                    message: e?.message || '网络错误',
                },
            }));
        } finally {
            setLLMTestLoading(prev => ({ ...prev, [section]: false }));
        }
    }

    async function fetchDigitalHumans() {
        setDhLoading(true);
        try {
            const resp = await fetch(`${baseURL}/digital_humans`, { headers: { Authorization: authHeader } });
            const js = await resp.json();
            setDigitalHumans(getRespData<any[]>(js) ?? []);
        } catch (e: any) {
            showToast('加载数字人失败: ' + (e?.message || ''), 'err');
        } finally {
            setDhLoading(false);
        }
    }

    async function saveDigitalHumanAPI(dh: any) {
        try {
            const resp = await fetch(`${baseURL}/digital_humans`, {
                method: 'POST',
                headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
                body: JSON.stringify(dh),
            });
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast('保存失败: ' + (js?.error || ''), 'err');
                return;
            }
            showToast(getRespData<string>(js) || '保存成功');
            fetchDigitalHumans();
            setDhEditing(null);
        } catch (e: any) {
            showToast('保存失败: ' + (e?.message || ''), 'err');
        }
    }

    async function deleteDigitalHumanAPI(id: string) {
        try {
            const resp = await fetch(`${baseURL}/digital_humans/${encodeURIComponent(id)}`, {
                method: 'DELETE',
                headers: { Authorization: authHeader },
            });
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast('删除失败: ' + (js?.error || ''), 'err');
                return;
            }
            showToast(getRespData<string>(js) || '删除成功');
            fetchDigitalHumans();
        } catch (e: any) {
            showToast('删除失败: ' + (e?.message || ''), 'err');
        }
    }

    async function fetchReportTemplates() {
        setRtLoading(true);
        try {
            const resp = await fetch(`${baseURL}/report_templates`, { headers: { Authorization: authHeader } });
            const js = await resp.json();
            const data = getRespData<Record<string, string>>(js) ?? {};
            setReportTemplates(data);
            setReportTemplatesDraft(data);
        } catch (e: any) {
            showToast('加载报告模板失败: ' + (e?.message || ''), 'err');
        } finally {
            setRtLoading(false);
        }
    }

    async function saveReportTemplates() {
        setRtSaving(true);
        try {
            const resp = await fetch(`${baseURL}/report_templates`, {
                method: 'PUT',
                headers: { Authorization: authHeader, 'Content-Type': 'application/json' },
                body: JSON.stringify(reportTemplatesDraft),
            });
            const js = await resp.json();
            if (!resp.ok || js?.error) {
                showToast(js?.error || '保存失败', 'err');
                return;
            }
            showToast(getRespData<string>(js) || '保存成功');
            setReportTemplates({ ...reportTemplatesDraft });
        } catch (e: any) {
            showToast('保存失败: ' + (e?.message || ''), 'err');
        } finally {
            setRtSaving(false);
        }
    }

    function openDetail(kind: 'agent' | 'exploitIdea' | 'exploitChain' | 'container' | 'report' | 'json' | 'digitalHumanProfile', title: string, obj: any) {
        setDetailTitle(title);
        setDetailKind(kind);
        setDetailObj(obj);
        setDetailOpen(true);
    }

    function openReportPreview(id: string, filename: string, content: string, downloadId?: string) {
        setDetailTitle(filename || id);
        setDetailKind('report');
        setDetailObj({ id, filename, content, downloadId: downloadId || id });
        setDetailOpen(true);
    }

    function closeDetail() {
        setDetailOpen(false);
    }

    async function refreshActiveGroupDetail(projectName?: string, groupId?: string) {
        const trimmedProject = String(projectName ?? detailProject ?? '').trim();
        const trimmedGroupId = String(groupId ?? activeGroupId ?? '').trim();
        if (!trimmedProject || !trimmedGroupId) return;
        const name = encodeURIComponent(trimmedProject);
        const gid = encodeURIComponent(trimmedGroupId);
        const detail = await apiGet<any>(`/projects/${name}/group_details/${gid}`);
        if (!detail || typeof detail !== 'object' || Array.isArray(detail)) return;
        setGroupDetails((prev) => ({
            ...prev,
            [trimmedGroupId]: sanitizeGroupDetail(detail),
        }));
    }

    async function refreshAllGroupDetails(projectName?: string, groups?: any[]) {
        const trimmedProject = String(projectName ?? detailProject ?? '').trim();
        const sourceGroups = Array.isArray(groups) ? groups : labGroups;
        const ids = Array.from(new Set((sourceGroups || []).map((g: any) => getGroupId(g)).filter(Boolean)));
        if (!trimmedProject || ids.length === 0) return;
        const name = encodeURIComponent(trimmedProject);
        const results = await Promise.all(ids.map(async (gid) => {
            try {
                const detail = await apiGet<any>(`/projects/${name}/group_details/${encodeURIComponent(gid)}`);
                if (!detail || typeof detail !== 'object' || Array.isArray(detail)) return null;
                return [gid, sanitizeGroupDetail(detail)] as const;
            } catch { return null; }
        }));
        const entries = results.filter(Boolean) as ReadonlyArray<readonly [string, any]>;
        if (entries.length === 0) return;
        setGroupDetails((prev) => {
            const next = { ...prev };
            entries.forEach(([gid, detail]) => { next[gid] = detail; });
            return next;
        });
    }

    async function refreshMemoryDebugTargets(projectName?: string) {
        const trimmedProject = String(projectName ?? detailProject ?? '').trim();
        if (!trimmedProject) return;
        const name = encodeURIComponent(trimmedProject);
        const targets = await apiGet<any[]>(`/projects/${name}/debug/memory_targets`);
        if (Array.isArray(targets)) {
            setMemoryDebugTargets(targets.map((item: any) => normalizeMemoryDebugTarget(item)));
            return;
        }
        setMemoryDebugTargets([]);
    }

    async function refreshProjectDetail() {
        if (!detailProject) return;
		const name = encodeURIComponent(detailProject);
		const proj = await apiGet<any>(`/projects/${name}`);
		const loadedProjectID = String(proj?.projectID ?? proj?.project_id ?? proj?.projectUUID ?? proj?.project_uuid ?? detailProject).trim();
		const loadedProjectName = String(proj?.projectName ?? proj?.project_name ?? proj?.labName ?? proj?.lab_name ?? loadedProjectID).trim();
		if (loadedProjectID && loadedProjectID !== detailProject) {
			setDetailProject(loadedProjectID);
		}
		setDetailProjectName(loadedProjectName || loadedProjectID || detailProject);

        const groups = proj?.groups;
        if (Array.isArray(groups)) {
            setLabGroups(groups);
            mergeGroupSummariesIntoDetails(groups);
        }

        const availableGroupIds = Array.isArray(groups) ? groups.map((g: any) => getGroupId(g)).filter(Boolean) : [];
        setActiveGroupId((current) => {
            const trimmedCurrent = String(current || '').trim();
            if (trimmedCurrent && availableGroupIds.includes(trimmedCurrent)) return trimmedCurrent;
            return availableGroupIds.includes('web_security') ? 'web_security' : (availableGroupIds[0] || '');
        });

        if (proj?.controlBrain && typeof proj.controlBrain === 'object') {
            setControlBrain(proj.controlBrain);
        }

        if (proj?.sharedResources && typeof proj.sharedResources === 'object') {
            setSharedResources(proj.sharedResources);
        } else {
            setSharedResources(null);
        }

        if (Array.isArray(proj?.hostCommandApprovals)) {
            setHostCommandApprovals(proj.hostCommandApprovals.map((item: any) => normalizeHostCommandApproval(item)));
        } else {
            setHostCommandApprovals([]);
        }
        if (Array.isArray(proj?.backgroundProcesses)) {
            setBackgroundProcesses(proj.backgroundProcesses.map((item: any) => normalizeBackgroundProcess(item)));
        } else {
            setBackgroundProcesses([]);
        }

        setProjectStatus(String(proj?.status ?? ''));
        setProjectIsRunning(Boolean(proj?.isRunning));
        setBrainFinished(Boolean(proj?.brainFinished));

        // Load token usage.
        try {
            const tu = await apiGet<any>(`/projects/${name}/token_usage`);
            if (tu && typeof tu === 'object') {
                setTokenUsage({
                    prompt_tokens: Number(tu?.prompt_tokens ?? 0),
                    completion_tokens: Number(tu?.completion_tokens ?? 0),
                    total_tokens: Number(tu?.total_tokens ?? 0),
                    agents: Array.isArray(tu?.agents) ? tu.agents : [],
                });
            }
        } catch { /* ignore */ }

        // Load context breakdown.
        try {
            const cb = await apiGet<any>(`/projects/${name}/context_breakdown`);
            if (cb && typeof cb === 'object') {
                setContextBreakdown({
                    sections: Array.isArray(cb?.sections) ? cb.sections : [],
                    total: Number(cb?.total ?? 0),
                    max_context: Number(cb?.max_context ?? 0),
                });
            }
        } catch { /* ignore */ }

        try {
            await refreshAllGroupDetails(detailProject, Array.isArray(groups) ? groups : labGroups);
        } catch { /* ignore */ }

        // Load persisted chat messages.
        try {
            await refreshChatMessages(detailProject);
        } catch { /* ignore */ }
    }

    async function fetchReportText(project: string, id: string): Promise<string> {
        const name = encodeURIComponent(project);
        const rid = encodeURIComponent(id);
        const resp = await fetch(`${baseURL}/projects/${name}/reports/download/${rid}`, {
            headers: {
                Authorization: authHeader,
            },
        });
        if (!resp.ok) {
            throw new Error(`fetch report failed: ${resp.status}`);
        }
        return await resp.text();
    }

    async function generateReportSummary() {
        if (!detailProject || reportSummaryGenerating || sharedReportCatalog.length === 0) return;
        setReportSummaryGenerating(true);
        try {
            const name = encodeURIComponent(detailProject);
            const result = await apiPost<any>(`/projects/${name}/reports/generate_summary`, {});
            await refreshProjectDetail();
            if (result && typeof result === 'object') {
                const reportId = String(result?.reportId ?? '').trim();
                const downloadId = String(result?.downloadId ?? reportId).trim();
                const rawFileName = result?.fileName ?? reportId ?? 'lab-summary.md';
                const fileName = String(rawFileName).trim() || 'lab-summary.md';
                const content = String(result?.content ?? '').trim();
                if (reportId && content) {
                    openReportPreview(reportId, fileName, content, downloadId);
                }
            }
            showToast('已生成实验室总结');
        } catch (e: any) {
            showToast(e?.message || '生成总结失败', 'err');
        } finally {
            setReportSummaryGenerating(false);
        }
    }

    async function downloadFileWithAuth(url: string, filename: string) {
        const resp = await fetch(url, {
            headers: {
                Authorization: authHeader,
            },
        });
        if (!resp.ok) {
            throw new Error(`download failed: ${resp.status}`);
        }
        const blob = await resp.blob();
        const objUrl = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = objUrl;
        a.download = filename || 'download';
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(objUrl);
    }

    async function decideHostCommandApproval(id: string, approved: boolean) {
        const approvalId = String(id || '').trim();
        if (!detailProject || !approvalId) return;
        setHostCommandDecisionLoading(prev => ({ ...prev, [approvalId]: true }));
        try {
            const project = encodeURIComponent(detailProject);
            await apiPost<string>(`/projects/${project}/host_command_requests/${encodeURIComponent(approvalId)}/decision`, {
                approved,
                note: String(hostCommandDecisionNotes[approvalId] || '').trim(),
            });
            if (!approved) {
                setHostCommandDecisionNotes(prev => ({ ...prev, [approvalId]: '' }));
            }
            const targetItem = hostCommandApprovals.find(item => String(item.id || '') === approvalId);
            const isAbandonIdea = String(targetItem?.kind || '') === 'abandon_idea';
            showToast(isAbandonIdea ? (approved ? '已同意放弃该思路' : '已要求继续该思路') : (approved ? '已批准该决策事项' : '已拒绝该决策事项'));
            await refreshProjectDetail();
        } catch (e: any) {
            showToast(e?.message || (approved ? '批准失败' : '拒绝失败'), 'err');
        } finally {
            setHostCommandDecisionLoading(prev => ({ ...prev, [approvalId]: false }));
        }
    }

    async function killBackgroundProcess(sessionId: string) {
        const id = String(sessionId || '').trim();
        if (!detailProject || !id) return;
        setBackgroundKillLoading(prev => ({ ...prev, [id]: true }));
        try {
            const project = encodeURIComponent(detailProject);
            await apiPost<any>(`/projects/${project}/background_processes/${encodeURIComponent(id)}/kill`, {});
            showToast('已发送终止请求');
            await refreshProjectDetail();
        } catch (e: any) {
            showToast(e?.message || '终止进程失败', 'err');
        } finally {
            setBackgroundKillLoading(prev => ({ ...prev, [id]: false }));
        }
    }

    async function exportMemoryDebug(target: MemoryDebugTarget) {
        const targetId = String(target?.target_id || '').trim();
        if (!detailProject || !targetId) return;
        setMemoryDebugExporting(prev => ({ ...prev, [targetId]: true }));
        try {
            const name = encodeURIComponent(detailProject);
            const url = `${baseURL}/projects/${name}/debug/memory_export?target_id=${encodeURIComponent(targetId)}`;
            const filenameParts = [
                String(detailProject || '').trim(),
                String(target.group_id || '').trim(),
                String(target.target_type || '').trim(),
                String(target.label || target.persona_name || 'memory').trim(),
            ].filter(Boolean);
            const filename = filenameParts.join('-').replace(/[\\/:*?"<>| ]+/g, '_') + '.txt';
            await downloadFileWithAuth(url, filename);
            showToast('记忆导出成功');
        } catch (e: any) {
            showToast(e?.message || '记忆导出失败', 'err');
        } finally {
            setMemoryDebugExporting(prev => ({ ...prev, [targetId]: false }));
        }
    }

    const rowStyle: any = {
        display: 'grid',
        gridTemplateColumns: '160px 1fr',
        gap: 12,
        padding: '10px 0',
        borderBottom: '1px solid rgba(255,255,255,0.08)',
        minWidth: 0,
    };
    const labelStyle: any = { opacity: 0.75, fontWeight: 700 };
    const valStyle: any = { opacity: 0.95, wordBreak: 'break-word', overflow: 'hidden', minWidth: 0 };

    function Section({ title, children }: { title: string; children: any }) {
        return (
            <div style={{ marginTop: 14 }}>
                <div style={{ fontWeight: 800, letterSpacing: 0.2, marginBottom: 8 }}>{title}</div>
                <div>{children}</div>
            </div>
        );
    }

    function FieldRow({ label, value }: { label: string; value: any }) {
        if (value === undefined || value === null || value === '') return null;
        const isLong = typeof value === 'string' && value.length > 180;
        return (
            <div style={rowStyle}>
                <div style={labelStyle}>{label}</div>
                <div style={valStyle}>
                    {isLong ? (
                        <pre className="aix-pre" style={{ maxHeight: 260, overflow: 'auto', margin: 0 }}>
                            {String(value)}
                        </pre>
                    ) : (
                        <div>{typeof value === 'string' ? value : JSON.stringify(value)}</div>
                    )}
                </div>
            </div>
        );
    }

    function get(obj: any, keys: string[]) {
        for (const k of keys) {
            const v = obj?.[k];
            if (v !== undefined) return v;
        }
        return undefined;
    }

    function renderDetailContent() {
        const kind = detailKind;
        const obj = detailObj;

        if (!obj) return <div className="aix-empty">无</div>;

        if (kind === 'agent') {
            return (
                <div style={{ paddingTop: 6 }}>
                    <Section title="基础信息">
                        <FieldRow label="运行状态" value={get(obj, ['RunState', 'runState'])} />
                    </Section>
                    <Section title="RUNTask">
                        <FieldRow label="RUNTask" value={get(obj, ['RUNTask', 'runTask'])} />
                    </Section>
                    <Section title="RUNSummary">
                        <FieldRow label="RUNSummary" value={get(obj, ['RUNSummary', 'runSummary'])} />
                    </Section>
                    <Section title="RUNError">
                        <FieldRow label="RUNError" value={get(obj, ['RUNError', 'runError'])} />
                    </Section>
                </div>
            );
        }

        if (kind === 'report') {
            const content = String(obj?.content ?? '');
            return (
                <div className="space-y-3">
                    <div className="flex items-center justify-between gap-2">
                        <div className="text-xs text-muted-foreground">{String(obj?.id ?? '')}</div>
                        <Button size="sm" variant="secondary" onClick={() => {
                            const name = encodeURIComponent(detailProject);
                            const rid = encodeURIComponent(String(obj?.downloadId ?? obj?.id ?? ''));
                            const fn = String(obj?.filename ?? 'report.md');
                            downloadFileWithAuth(`${baseURL}/projects/${name}/reports/download/${rid}`, fn);
                        }}>下载</Button>
                    </div>
                    <div className="rounded-lg border border-border bg-background/20 p-3">
                        <div className="aix-markdown aix-markdown--report">
                            <ReactMarkdown>{content}</ReactMarkdown>
                        </div>
                    </div>
                </div>
            );
        }

        if (detailKind === 'container') {
            return (
                <div style={{ paddingTop: 6 }}>
                    <Section title="基础信息">
                        <FieldRow label="containerId" value={get(obj, ['containerId', 'ContainerId'])} />
                        <FieldRow label="image" value={get(obj, ['image', 'Image'])} />
                        <FieldRow label="containerIP" value={get(obj, ['containerIP', 'ContainerIP'])} />
                        <FieldRow label="webPort" value={(() => { const wp = obj?.webPort ?? obj?.WebPort; return Array.isArray(wp) && wp.length > 0 ? wp.join(', ') : '-'; })()} />
                        <FieldRow label="type" value={get(obj, ['type', 'Type'])} />
                    </Section>
                </div>
            );
        }

        if (detailKind === 'exploitIdea') {
            const ep = get(obj, ['exploit_point', 'ExploitPoint']);
            return (
                <div style={{ paddingTop: 6 }}>
                    <Section title="基本信息">
                        <FieldRow label="ExploitIdeaId" value={get(obj, ['exploitIdeaId', 'ExploitIdeaId'])} />
                        <FieldRow label="状态" value={get(obj, ['state', 'State'])} />
                        <FieldRow label="危害" value={get(obj, ['harm', 'Harm'])} />
                        <FieldRow label="扩大化思路" value={get(obj, ['extend_idea', 'ExtendIdea'])} />
                        <FieldRow label="利用条件" value={get(obj, ['condition', 'Condition'])} />
                    </Section>

                    <Section title="ExploitPoint">
                        <FieldRow label="标题" value={get(ep, ['title', 'Title'])} />
                        <FieldRow label="类型" value={get(ep, ['type', 'Type'])} />
                        <FieldRow label="路由/端点" value={get(ep, ['route_or_endpoint', 'RouteOrEndpoint'])} />
                        <FieldRow label="文件" value={get(ep, ['file', 'File'])} />
                        <FieldRow label="函数/方法" value={get(ep, ['function_or_method', 'FunctionOrMethod'])} />
                        <FieldRow label="参数" value={get(ep, ['params', 'Params'])} />
                        <FieldRow label="Payload 思路" value={get(ep, ['payload_idea', 'PayloadIdea'])} />
                        <FieldRow label="预期影响" value={get(ep, ['expected_impact', 'ExpectedImpact'])} />
                        <FieldRow label="置信度" value={get(ep, ['confidence', 'Confidence'])} />
                        <FieldRow label="ExploitId" value={get(ep, ['exploit_id', 'ExploitId'])} />
                    </Section>

                    <Section title="证据与 PoC">
                        <FieldRow label="Evidence" value={get(obj, ['evidence', 'Evidence'])} />
                        <FieldRow label="PoC" value={get(obj, ['poc', 'Poc'])} />
                    </Section>
                </div>
            );
        }

        if (detailKind === 'digitalHumanProfile') {
            const avatarSrc = String(obj?.avatar_src ?? '');
            const personaName = String(obj?.persona_name ?? '-');
            const gender = String(obj?.gender ?? '-');
            const personality = String(obj?.personality ?? '-');
            const age = Number(obj?.age ?? 0);
            const role = String(obj?.role ?? '-');
            const dhId = String(obj?.digital_human_id ?? '-');
            const state = String(obj?.state ?? '-');
            const agentId = String(obj?.agent_id ?? '');
            const runState = obj?.RunState;
            const runTask = obj?.RUNTask;
            const runSummary = obj?.RUNSummary;
            const runError = obj?.RUNError;
            const lastSummary = String(obj?.last_summary ?? '');
            const lastTask = String(obj?.last_task ?? '');
            const lastError = String(obj?.last_error ?? '');
            const summaryToShow = runSummary ? String(runSummary) : lastSummary;
            const taskToShow = runTask ? String(runTask) : lastTask;
            const errorToShow = runError ? String(runError) : lastError;
            return (
                <div style={{ paddingTop: 6 }}>
                    <div className="flex items-center gap-4 mb-4">
                        {avatarSrc && (
                            <img
                                src={avatarSrc}
                                alt={personaName}
                                className="h-16 w-16 rounded-full border-2 border-border object-cover"
                            />
                        )}
                        <div>
                            <div className="text-lg font-bold">{personaName}</div>
                            <div className="text-sm text-muted-foreground">{role}</div>
                        </div>
                    </div>
                    <Section title="基本信息">
                        <FieldRow label="姓名" value={personaName} />
                        <FieldRow label="职业" value={role} />
                        <FieldRow label="性格" value={personality} />
                        <FieldRow label="数字人ID" value={dhId} />
                    </Section>
                    <Section title="状态">
                        <FieldRow label="当前状态" value={state === 'busy' ? '忙碌' : '空闲'} />
                        {runState ? <FieldRow label="运行状态" value={String(runState)} /> : null}
                        {errorToShow ? <FieldRow label="失败原因" value={errorToShow} /> : null}
                    </Section>
                    {taskToShow ? (
                        <Section title={runTask ? '当前任务' : '上次任务'}>
                            <div className="text-sm text-foreground whitespace-pre-wrap break-words">{taskToShow}</div>
                        </Section>
                    ) : null}
                    {summaryToShow ? (
                        <Section title={runSummary ? '执行摘要' : '上次任务总结'}>
                            <div className="prose prose-invert prose-sm max-w-none break-words text-sm">
                                <ReactMarkdown>{summaryToShow}</ReactMarkdown>
                            </div>
                        </Section>
                    ) : null}
                </div>
            );
        }

        if (detailKind === 'exploitChain') {
            const eideas = get(obj, ['exploit_idea', 'ExploitIdea']);
            return (
                <div style={{ paddingTop: 6 }}>
                    <Section title="基本信息">
                        <FieldRow label="ExploitChainId" value={get(obj, ['exploit_chain_id', 'ExploitChainId'])} />
                        <FieldRow label="状态" value={get(obj, ['state', 'State'])} />
                        <FieldRow label="组链思路" value={get(obj, ['idea', 'Idea'])} />
                    </Section>

                    <Section title="链路包含的 ExploitIdea">
                        {Array.isArray(eideas) && eideas.length > 0 ? (
                            <div style={{ display: 'grid', gap: 10 }}>
                                {eideas.map((one: any, idx: number) => (
                                    <div key={one?.exploitIdeaId ?? one?.ExploitIdeaId ?? idx} className="aix-item">
                                        <div style={{ fontWeight: 800 }}>{String(one?.exploitIdeaId ?? one?.ExploitIdeaId ?? 'exploitIdea')}</div>
                                        <div style={{ opacity: 0.8, marginTop: 4 }}>状态：{String(one?.state ?? one?.State ?? '-')}</div>
                                    </div>
                                ))}
                            </div>
                        ) : (
                            <div className="aix-empty">无</div>
                        )}
                    </Section>

                    <Section title="证据与 PoC">
                        <FieldRow label="Evidence" value={get(obj, ['evidence', 'Evidence'])} />
                        <FieldRow label="PoC" value={get(obj, ['poc', 'Poc'])} />
                    </Section>
                </div>
            );
        }

        return (
            <pre className="aix-pre" style={{ maxHeight: '70vh', overflow: 'auto' }}>
                {JSON.stringify(obj, null, 2)}
            </pre>
        );
    }

    async function startProject(projectID: string, displayName?: string) {
		const key = `start_${projectID}`;
        setBtnLoading(prev => ({ ...prev, [key]: true }));
        try {
			const msg = await apiGet<string>(`/projects/${encodeURIComponent(projectID)}/start`);
			showToast(msg || `${displayName || projectID} 已加入启动队列`);
            await refreshProjects();
			if (detailProject === projectID) {
                await refreshProjectDetail();
            }
        } catch (e: any) {
            showToast(e?.message || '启动失败', 'err');
        } finally {
            setBtnLoading(prev => ({ ...prev, [key]: false }));
        }
    }
    async function stopProject(projectID: string, displayName?: string) {
		const key = `stop_${projectID}`;
        setBtnLoading(prev => ({ ...prev, [key]: true }));
        try {
			const msg = await apiGet<string>(`/projects/${encodeURIComponent(projectID)}/cancel`);
			showToast(msg || `${displayName || projectID} 已停止`);
            await refreshProjects();
        } catch (e: any) {
            showToast(e?.message || '停止失败', 'err');
        } finally {
            setBtnLoading(prev => ({ ...prev, [key]: false }));
        }
    }
    async function deleteProject(projectID: string, displayName?: string) {
		const key = `del_${projectID}`;
        setBtnLoading(prev => ({ ...prev, [key]: true }));
        try {
			await apiGet(`/projects/${encodeURIComponent(projectID)}/del`);
            await refreshProjects();
			showToast(`${displayName || projectID} 已删除`);
        } catch (e: any) {
            showToast(e?.message || '删除失败', 'err');
        } finally {
            setBtnLoading(prev => ({ ...prev, [key]: false }));
        }
    }

    function connectWS(projectName: string) {
        if (!authHeader) return;
        wsRef.current?.close();
        const name = encodeURIComponent(projectName);
        let wsBase = '';
        // If baseURL is absolute (for example local dev API), connect websocket to that host.
        if (baseURL && /^https?:\/\//.test(baseURL)) {
            wsBase = baseURL;
        } else if (typeof window !== 'undefined' && window.location.origin) {
            // Same-origin (web mode)
            wsBase = window.location.origin;
        }
        if (!wsBase) return;
        const wsToken = token || '';
        const wsURL = wsBase.replace(/^http/, 'ws') + `/ws?projectName=${name}&token=${encodeURIComponent(wsToken)}`;
        const ws = new WebSocket(wsURL);
        ws.onopen = () => {
            // Connected
        };
        ws.onmessage = (ev) => {
            try {
                const msg = JSON.parse(ev.data);

                if (msg?.type === 'string' && typeof msg?.data === 'string') {
                    // legacy event stream: no longer displayed
                    return;
                }
                if (msg?.type === 'ReportAdd' && msg?.data && typeof msg.data === 'object') {
                    // Legacy event. Reports are now shared at lab level through reportCatalog.
                    return;
                }
                if (msg?.type === 'ReportCatalogUpdate' && msg?.data && typeof msg.data === 'object') {
                    setSharedResources((prev: any) => ({
                        ...(prev && typeof prev === 'object' ? prev : {}),
                        ...(msg.data as any),
                    }));
                    return;
                }
                if (msg?.type === 'EnvInfo' && msg?.data) {
                    const d: any = msg.data;
                    upsertGroupDetail('web_security', (current) => ({
                        ...current,
                        envInfo: d?.env ?? d,
                    }));
                    return;
                }

                if (msg?.type === 'ContainerAdd' && msg?.data) {
                    const c = msg.data as any;
                    upsertGroupDetail('web_security', (current) => {
                        const prev = Array.isArray(current?.containers) ? current.containers : [];
                        const id = c?.containerId ?? c?.ContainerId;
                        if (!id) return { ...current, containers: prev };
                        const exists = prev.some((x: any) => (x?.containerId ?? x?.ContainerId) === id);
                        return { ...current, containers: exists ? prev : [c, ...prev] };
                    });
                    return;
                }
                if (msg?.type === 'ContainerRemove' && msg?.data) {
                    const id = (msg.data as any)?.containerId ?? (msg.data as any)?.ContainerId;
                    if (id) {
                        upsertGroupDetail('web_security', (current) => ({
                            ...current,
                            containers: (Array.isArray(current?.containers) ? current.containers : []).filter((x: any) => (x?.containerId ?? x?.ContainerId) !== id),
                        }));
                    }
                    return;
                }

                if (msg?.type === 'AgentRuntimeUpdate' && Array.isArray(msg?.data)) {
                    const groupId = getRuntimeGroupIdFromProjectName(msg?.projectName);
                    upsertGroupDetail(groupId, (current) => ({
                        ...current,
                        agentRuns: msg.data as any[],
                    }));
                    return;
                }

                if (msg?.type === 'DigitalHumanRosterUpdate' && msg?.data && typeof msg.data === 'object') {
                    const groupId = getRuntimeGroupIdFromProjectName(msg?.projectName);
                    upsertGroupDetail(groupId, (current) => ({
                        ...current,
                        digitalHumans: msg.data as any,
                    }));
                    return;
                }

                if (msg?.type === 'LabGroupsUpdate' && Array.isArray(msg?.data)) {
                    const groups = msg.data as any[];
                    setLabGroups(groups);
                    mergeGroupSummariesIntoDetails(groups);
                    return;
                }
                if (msg?.type === 'LabSharedResourcesUpdate' && msg?.data && typeof msg.data === 'object') {
                    setSharedResources(msg.data as any);
                    return;
                }

                if (msg?.type === 'HostCommandApprovalsUpdate' && Array.isArray(msg?.data)) {
                    setHostCommandApprovals((msg.data as any[]).map((item: any) => normalizeHostCommandApproval(item)));
                    return;
                }
                if (msg?.type === 'BackgroundProcessesUpdate' && Array.isArray(msg?.data)) {
                    setBackgroundProcesses((msg.data as any[]).map((item: any) => normalizeBackgroundProcess(item)));
                    return;
                }

                if (msg?.type === 'BrainFinished' && msg?.data) {
                    const finished = Boolean((msg.data as any)?.brain_finished);
                    setBrainFinished(finished);
                    if (finished) {
                        setProjectStatus('决策结束');
                    } else {
                        setProjectStatus('正在运行');
                    }
                    return;
                }

                if (msg?.type === 'LabChatMessage' && msg?.data) {
                    const normalized = normalizeChatMessage(msg.data);
                    setChatMessages(prev => appendUniqueChatMessage(prev, normalized));
                    return;
                }

                if (msg?.type === 'UserMessage' || msg?.type === 'TeamMessage') {
                    return;
                }

                if (msg?.type === 'TokenUsage' && msg?.data) {
                    const d = msg.data as any;
                    setTokenUsage({
                        prompt_tokens: Number(d?.prompt_tokens ?? 0),
                        completion_tokens: Number(d?.completion_tokens ?? 0),
                        total_tokens: Number(d?.total_tokens ?? 0),
                        agents: Array.isArray(d?.agents) ? d.agents : [],
                    });
                    return;
                }

                if (msg?.type === 'BrainContextBreakdown' && msg?.data) {
                    const d = msg.data as any;
                    setContextBreakdown({
                        sections: Array.isArray(d?.sections) ? d.sections : [],
                        total: Number(d?.total ?? 0),
                        max_context: Number(d?.max_context ?? 0),
                    });
                    return;
                }

                if (msg?.type === 'ExploitIdeaAdd' && msg?.data) {
                    const e = msg.data as any;
                    const id = e?.exploitIdeaId ?? e?.ExploitIdeaId;
                    if (id) {
                        upsertGroupDetail('web_security', (current) => {
                            const prev = Array.isArray(current?.exploitIdeas) ? current.exploitIdeas : [];
                            const exists = prev.some((x: any) => (x?.exploitIdeaId ?? x?.ExploitIdeaId) === id);
                            return { ...current, exploitIdeas: exists ? prev : [e, ...prev] };
                        });
                    }
                    return;
                }
                if (msg?.type === 'ExploitIdeaUpdate' && msg?.data) {
                    const e = msg.data as any;
                    const id = e?.exploitIdeaId ?? e?.ExploitIdeaId;
                    if (id) {
                        upsertGroupDetail('web_security', (current) => {
                            const prev = Array.isArray(current?.exploitIdeas) ? current.exploitIdeas : [];
                            const out = prev.map((x: any) => ((x?.exploitIdeaId ?? x?.ExploitIdeaId) === id ? e : x));
                            const exists = out.some((x: any) => (x?.exploitIdeaId ?? x?.ExploitIdeaId) === id);
                            return { ...current, exploitIdeas: exists ? out : [e, ...prev] };
                        });
                    }
                    return;
                }
                if (msg?.type === 'ExploitIdeaList' && Array.isArray(msg?.data)) {
                    upsertGroupDetail('web_security', (current) => ({
                        ...current,
                        exploitIdeas: msg.data as any[],
                    }));
                    return;
                }

                if (msg?.type === 'ExploitChainAdd' && msg?.data) {
                    const e = msg.data as any;
                    const id = e?.exploit_chain_id ?? e?.ExploitChainId;
                    if (id) {
                        upsertGroupDetail('web_security', (current) => {
                            const prev = Array.isArray(current?.exploitChains) ? current.exploitChains : [];
                            const exists = prev.some((x: any) => (x?.exploit_chain_id ?? x?.ExploitChainId) === id);
                            return { ...current, exploitChains: exists ? prev : [e, ...prev] };
                        });
                    }
                    return;
                }
                if (msg?.type === 'ExploitChainUpdate' && msg?.data) {
                    const e = msg.data as any;
                    const id = e?.exploit_chain_id ?? e?.ExploitChainId;
                    if (id) {
                        upsertGroupDetail('web_security', (current) => {
                            const prev = Array.isArray(current?.exploitChains) ? current.exploitChains : [];
                            const out = prev.map((x: any) => ((x?.exploit_chain_id ?? x?.ExploitChainId) === id ? e : x));
                            const exists = out.some((x: any) => (x?.exploit_chain_id ?? x?.ExploitChainId) === id);
                            return { ...current, exploitChains: exists ? out : [e, ...prev] };
                        });
                    }
                    return;
                }
                if (msg?.type === 'ExploitChainList' && Array.isArray(msg?.data)) {
                    upsertGroupDetail('web_security', (current) => ({
                        ...current,
                        exploitChains: msg.data as any[],
                    }));
                    return;
                }

                // Unknown messages: keep for debugging
                setEvents((prev) => [msg, ...prev].slice(0, 200));
            } catch {
                setEvents((prev) => [{ raw: ev.data }, ...prev].slice(0, 200));
            }
        };
        ws.onerror = () => {
            // ignore
        };
        wsRef.current = ws;
    }

    useEffect(() => {
        if (view !== 'detail' || !detailProject) return;
        connectWS(detailProject);
        (async () => {
            try {
                await refreshProjectDetail();
            } catch (e: any) {
                openDetail('json', 'Load Project Error', {
                    project: detailProject,
                    error: String(e?.message ?? e ?? 'error'),
                });
            }
        })();
        return () => {
            wsRef.current?.close();
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [view, detailProject, baseURL]);

    useEffect(() => {
        if (view !== 'detail' || !detailProject || !activeGroupId) return;
        refreshActiveGroupDetail(detailProject, activeGroupId).catch(() => { /* ignore */ });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [view, detailProject, activeGroupId]);

    useEffect(() => {
        if (view !== 'detail' || !detailProject) return;
        const channelId = String(chatChannelId || '').trim();
        if (!channelId.startsWith('group:')) return;
        const targetGroupId = channelId.replace(/^group:/, '');
        if (!targetGroupId) return;
        const current = getGroupDetailRecord(groupDetails, targetGroupId);
        const roster = current?.digitalHumans;
        if (countRosterMembers(roster) > 0) return;
        refreshActiveGroupDetail(detailProject, targetGroupId).catch(() => { /* ignore */ });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [view, detailProject, chatChannelId, groupDetails]);

    useEffect(() => {
        if (view !== 'detail' || !detailProject) return;
        refreshMemoryDebugTargets(detailProject).catch(() => { /* ignore */ });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [view, detailProject]);

    useEffect(() => {
        if (view !== 'detail' || !detailProject) return;
        const status = String(projectStatus || '').trim();
        const shouldPoll = status === '启动中' || status.startsWith('排队中');
        if (!shouldPoll) return;
        const timer = window.setInterval(() => {
            refreshProjectDetail().catch(() => { /* ignore */ });
        }, 5000);
        return () => window.clearInterval(timer);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [view, detailProject, projectStatus]);

    useEffect(() => {
        const validIds = new Set((labGroups || []).map((g: any) => getGroupId(g)).filter(Boolean));
        setActiveGroupId((current) => {
            const trimmedCurrent = String(current || '').trim();
            if (validIds.size === 0) return '';
            if (trimmedCurrent && validIds.has(trimmedCurrent)) return trimmedCurrent;
            return validIds.has('web_security') ? 'web_security' : Array.from(validIds)[0];
        });
    }, [labGroups]);

    useEffect(() => {
        const validIds = new Set(getChatChannels(labGroups, controlBrain).map(c => c.id));
        if (!validIds.has(chatChannelId)) {
            setChatChannelId('decision_makers');
        }
    }, [labGroups, controlBrain, chatChannelId]);

    const chatChannels = getChatChannels(labGroups, controlBrain);
    const activeChatChannel = chatChannels.find(c => c.id === chatChannelId) || chatChannels[0] || { id: 'decision_makers', name: '决策者群', subtitle: '' };
    const visibleChatMessages = chatMessages.filter(m => String(m?.channel_id || 'decision_makers') === activeChatChannel.id);
    const channelParticipants = getChannelParticipants(activeChatChannel.id, labGroups, groupDetails, controlBrain);
    const chatShortcutTargets = buildChatShortcutTargets(activeChatChannel.id, labGroups, groupDetails, controlBrain);
    const activeGroup = labGroups.find((group: any) => getGroupId(group) === activeGroupId) || labGroups[0] || null;
    const activeGroupDetail = activeGroup ? (getGroupDetailRecord(groupDetails, getGroupId(activeGroup)) || {}) : null;
    const activeGroupRoster: Record<string, any[]> = (activeGroupDetail?.digitalHumans && typeof activeGroupDetail.digitalHumans === 'object') ? activeGroupDetail.digitalHumans : {};
    const activeGroupExploitIdeas: any[] = Array.isArray(activeGroupDetail?.exploitIdeas) ? activeGroupDetail.exploitIdeas : [];
    const activeGroupExploitChains: any[] = Array.isArray(activeGroupDetail?.exploitChains) ? activeGroupDetail.exploitChains : [];
    const activeGroupContainers: any[] = Array.isArray(activeGroupDetail?.containers) ? activeGroupDetail.containers : [];
    const activeGroupEnvInfo = activeGroupDetail?.envInfo || null;
    const sharedReportCatalog: any[] = Array.isArray(sharedResources?.reportCatalog) ? sharedResources.reportCatalog : [];
    const activeGroupKind = String(activeGroup?.groupKind ?? activeGroup?.group_kind ?? '');
    const isWebActiveGroup = activeGroupKind === 'web_security';
    const detailProjectIsQueued = String(projectStatus || '').startsWith('排队中');
    const detailProjectIsStarting = String(projectStatus || '').trim() === '启动中';
    const pendingHostCommandApprovals = hostCommandApprovals.filter(item => String(item?.status ?? '') === 'pending_approval');
    const activeHostCommandApprovals = hostCommandApprovals.filter(item => {
        const status = String(item?.status ?? '');
        return status === 'pending_approval' || status === 'approved' || status === 'starting' || status === 'running' || status === 'terminating';
    });
    const recentResolvedHostCommandApprovals = hostCommandApprovals.filter(item => {
        const status = String(item?.status ?? '');
        return status !== 'pending_approval' && status !== 'approved' && status !== 'starting' && status !== 'running' && status !== 'terminating';
    }).slice(0, 8);
    const activeBackgroundProcesses = backgroundProcesses.filter(item => {
        const status = String(item?.status ?? '').trim().toLowerCase();
        return !!item?.running || status === 'starting' || status === 'running' || status === 'terminating';
    });
    const exportableMemoryTargets = memoryDebugTargets.filter(item => item.available);
    const dashboardGroupDetails = labGroups.map((group: any) => {
        const gid = getGroupId(group);
        const detail = getGroupDetailRecord(groupDetails, gid) || {};
        const roster = detail?.digitalHumans || group?.digitalHumans || {};
        const runs = Array.isArray(detail?.agentRuns) ? detail.agentRuns : (Array.isArray(group?.agentRuns) ? group.agentRuns : []);
        return { group, gid, detail, roster, runs, memberCount: Math.max(Number(group?.memberCount ?? 0), countRosterMembers(roster)) };
    });
    const dashboardAgentRuns = dashboardGroupDetails.flatMap(({ group, gid, roster, runs }) => (runs || [])
        .filter((run: any) => !isTemporaryAgentRun(run))
        .map((run: any) => ({ run, member: findRosterMemberForRun(roster, run), groupId: gid, groupName: String(group?.groupName ?? gid) })));
    const dashboardActiveRuns = dedupeActiveRuns(dashboardAgentRuns.filter(({ run }) => isRunActive(run)));
    const dashboardContextTotal = Number(contextBreakdown.total || 0);
    const dashboardContextMax = Number(contextBreakdown.max_context || 0);
    const dashboardContextPct = dashboardContextMax > 0 ? Math.min(100, (dashboardContextTotal / dashboardContextMax) * 100) : 0;
    const topTokenAgents = [...(tokenUsage.agents || [])].sort((a, b) => Number(b.total_tokens || 0) - Number(a.total_tokens || 0)).slice(0, 8);

    // Wait for startup checks (init status + token validation) before rendering anything.
    if (!initChecked) {
        return (
            <div className="aix-shell cyber-bg" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh' }}>
                <div style={{ textAlign: 'center', opacity: 0.5 }}>
                    <div style={{ fontSize: 18, fontWeight: 600 }}>AIxVuln</div>
                    <div style={{ fontSize: 13, marginTop: 8 }}>正在初始化…</div>
                </div>
            </div>
        );
    }

    // Init wizard: first-run setup
    if (needsInit && initChecked && (initStep === 'docker' || !authHeader)) {
        const handleInitSubmit = async () => {
            if (initStep === 'user') {
                if (!initUsername.trim()) { setSetupError('用户名不能为空'); return; }
                if (initPassword.length < 6) { setSetupError('密码至少6位'); return; }
                if (initPassword !== initPassword2) { setSetupError('两次密码不一致'); return; }
                setInitLoading(true);
                setSetupError('');
                try {
                    const initBase = baseURL || '';
                    const resp = await fetch(`${initBase}/init`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ username: initUsername.trim(), password: initPassword }),
                    });
                    const js = await resp.json();
                    if (!resp.ok || !js?.success) { setSetupError(js?.error || '初始化失败'); return; }
                    setToken(js.token);
                    localStorage.setItem('aix_token', js.token);
                    // Check if docker images need building.
                    const needsDocker = !dockerImages['aisandbox'] || !dockerImages['java_env'];
                    if (needsDocker) {
                        setInitStep('docker');
                    } else {
                        setNeedsInit(false);
                    }
                } catch (e: any) {
                    setSetupError(e?.message || '网络错误');
                } finally {
                    setInitLoading(false);
                }
            }
        };

        const streamDockerCmd = async (imageName: string, endpoint: string, label: string) => {
            setDockerBuilding(prev => ({ ...prev, [imageName]: true }));
            setDockerBuildOutput(prev => ({ ...prev, [imageName]: `${label}中，请稍候…` }));
            try {
                const initBase = baseURL || '';
                const headers: Record<string, string> = {};
                if (authHeader) headers['Authorization'] = authHeader;
                else if (token) headers['Authorization'] = 'Bearer ' + token;
                let pullUrl = `${initBase}/${endpoint}/${imageName}`;
                if (endpoint === 'docker_pull' && customRegistry.trim()) {
                    pullUrl += `?registry=${encodeURIComponent(customRegistry.trim())}`;
                }
                const resp = await fetch(pullUrl, { method: 'POST', headers });
                if (!resp.ok) {
                    let errMsg = `${label}失败 (HTTP ${resp.status})`;
                    try { const js = await resp.json(); errMsg = js?.error || errMsg; } catch {}
                    setDockerBuildOutput(prev => ({ ...prev, [imageName]: errMsg + (endpoint.includes('pull') ? '\n建议尝试「本地构建」方式。' : '') }));
                    return;
                }
                const reader = resp.body?.getReader();
                if (!reader) { setDockerBuildOutput(prev => ({ ...prev, [imageName]: `${label}失败: 无法读取响应流` })); return; }
                const decoder = new TextDecoder();
                let buf = '';
                let lines: string[] = [];
                let hasError = false;
                const MAX_LINES = 50;
                while (true) {
                    const { done, value } = await reader.read();
                    if (done) break;
                    buf += decoder.decode(value, { stream: true });
                    const parts = buf.split('\n');
                    buf = parts.pop() || '';
                    for (const part of parts) {
                        const trimmed = part.trim();
                        if (!trimmed || !trimmed.startsWith('data: ')) continue;
                        try {
                            const evt = JSON.parse(trimmed.slice(6));
                            if (evt.line) {
                                lines.push(evt.line);
                                if (lines.length > MAX_LINES) lines = lines.slice(-MAX_LINES);
                                setDockerBuildOutput(prev => ({ ...prev, [imageName]: lines.join('\n') }));
                            }
                            if (evt.done) {
                                if (evt.error) {
                                    hasError = true;
                                    lines.push(`\n❌ ${label}失败: ${evt.error}`);
                                    if (endpoint.includes('pull')) lines.push('建议尝试「本地构建」方式。');
                                } else {
                                    lines.push(`\n✅ ${label}成功！`);
                                    setDockerImages(prev => ({ ...prev, [imageName]: true }));
                                }
                                setDockerBuildOutput(prev => ({ ...prev, [imageName]: lines.join('\n') }));
                            }
                        } catch {}
                    }
                }
                if (!hasError && !lines.some(l => l.includes('成功'))) {
                    lines.push(`\n✅ ${label}成功！`);
                    setDockerImages(prev => ({ ...prev, [imageName]: true }));
                    setDockerBuildOutput(prev => ({ ...prev, [imageName]: lines.join('\n') }));
                }
            } catch (e: any) {
                setDockerBuildOutput(prev => ({ ...prev, [imageName]: `${label}失败: ${e?.message || ''}` + (endpoint.includes('pull') ? '\n建议尝试「本地构建」方式。' : '') }));
            } finally {
                setDockerBuilding(prev => ({ ...prev, [imageName]: false }));
            }
        };

        const handleDockerPull = (imageName: string) => streamDockerCmd(imageName, 'docker_pull', '拉取镜像');
        const handleDockerBuild = (imageName: string) => streamDockerCmd(imageName, 'docker_build', '本地构建');

        const registryPrefix = customRegistry.trim() ? customRegistry.trim().replace(/\/+$/, '') + '/' : 'aixvuln/';
        const remoteImageNames: Record<string, string> = { aisandbox: `${registryPrefix}aisandbox`, java_env: `${registryPrefix}java_env` };

        return (
            <div className="aix-shell cyber-bg" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh' }}>
                <Card className="w-full max-w-[560px] mx-4">
                    <CardHeader className="text-center">
                        <CardTitle className="text-2xl">AIxVuln 初始化设置</CardTitle>
                        <div className="text-sm text-muted-foreground mt-1">
                            {initStep === 'user' ? '首次启动，请设置管理员账户' : 'Docker 镜像引导'}
                        </div>
                    </CardHeader>
                    <CardContent>
                        {initStep === 'user' ? (
                            <div className="space-y-4">
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">用户名</label>
                                    <Input placeholder="设置管理员用户名" value={initUsername} onChange={e => setInitUsername(e.target.value)} autoFocus />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">密码</label>
                                    <Input type="password" placeholder="设置密码（至少6位）" value={initPassword} onChange={e => setInitPassword(e.target.value)} />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">确认密码</label>
                                    <Input type="password" placeholder="再次输入密码" value={initPassword2} onChange={e => setInitPassword2(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') handleInitSubmit(); }} />
                                </div>
                                {setupError && <div className="text-sm text-destructive text-center">{setupError}</div>}
                                <Button className="w-full" disabled={initLoading} onClick={handleInitSubmit}>
                                    {initLoading ? '创建中…' : '创建管理员账户'}
                                </Button>
                            </div>
                        ) : (
                            <div className="space-y-4">
                                <div className="text-xs text-muted-foreground mb-2">
                                    系统需要以下 Docker 镜像才能正常运行。推荐使用<strong>「拉取镜像」</strong>方式（更快），拉取失败时再使用「本地构建」。
                                </div>
                                <div className="rounded-lg border border-border p-3 space-y-1.5 mb-1">
                                    <label className="text-sm font-medium">自定义镜像仓库（可选）</label>
                                    <Input
                                        placeholder="例如：registry.cn-hangzhou.aliyuncs.com/yourns"
                                        value={customRegistry}
                                        onChange={e => setCustomRegistry(e.target.value)}
                                    />
                                    <div className="text-[11px] text-muted-foreground">
                                        留空则从 Docker Hub（aixvuln/）拉取。填写后将从指定仓库拉取，例如填写 <code>registry.cn-hangzhou.aliyuncs.com/myns</code> 则拉取 <code>registry.cn-hangzhou.aliyuncs.com/myns/aisandbox</code>
                                    </div>
                                </div>
                                {(['aisandbox', 'java_env'] as const).map(img => (
                                    <div key={img} className="rounded-lg border border-border p-4 space-y-2">
                                        <div className="flex items-center justify-between">
                                            <div>
                                                <span className="font-semibold text-sm">{img}</span>
                                                <span className="ml-2 text-xs text-muted-foreground">
                                                    {img === 'aisandbox' ? '攻击/验证沙箱' : 'Java 多版本运行环境'}
                                                </span>
                                            </div>
                                            {dockerImages[img] && <Badge variant="success">已就绪</Badge>}
                                        </div>
                                        {!dockerImages[img] && (
                                            <div className="space-y-1.5">
                                                <div className="text-[11px] text-muted-foreground">
                                                    远程镜像：<code className="text-xs">{remoteImageNames[img]}</code>
                                                </div>
                                                <div className="flex gap-2">
                                                    <Button size="sm" disabled={!!dockerBuilding[img]} onClick={() => handleDockerPull(img)}>
                                                        {dockerBuilding[img] ? '处理中…' : '拉取镜像（推荐）'}
                                                    </Button>
                                                    <Button size="sm" variant="outline" disabled={!!dockerBuilding[img]} onClick={() => handleDockerBuild(img)}>
                                                        {dockerBuilding[img] ? '处理中…' : '本地构建'}
                                                    </Button>
                                                </div>
                                            </div>
                                        )}
                                        {dockerBuildOutput[img] && (
                                            <pre className="text-xs bg-background/50 rounded p-2 max-h-[120px] overflow-auto whitespace-pre-wrap break-words">{dockerBuildOutput[img]}</pre>
                                        )}
                                    </div>
                                ))}
                                <Button className="w-full" onClick={() => setNeedsInit(false)}>
                                    {dockerImages['aisandbox'] && dockerImages['java_env'] ? '完成设置，进入系统' : '跳过，稍后构建'}
                                </Button>
                            </div>
                        )}
                    </CardContent>
                </Card>
            </div>
        );
    }

    // Show login page if no valid auth
    if (!authHeader) {
        return (
            <div className="aix-shell cyber-bg" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh' }}>
                <Card className="w-full max-w-[380px] mx-4">
                    <CardHeader className="text-center">
                        <CardTitle className="text-2xl">AIxVuln</CardTitle>
                        <div className="text-sm text-muted-foreground mt-1">登录以继续</div>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-4">
                            <div className="space-y-2">
                                <label className="text-sm font-medium">用户名</label>
                                <Input
                                    placeholder="请输入用户名"
                                    value={loginUser}
                                    onChange={e => setLoginUser(e.target.value)}
                                    onKeyDown={e => { if (e.key === 'Enter') handleLogin(); }}
                                    autoFocus
                                />
                            </div>
                            <div className="space-y-2">
                                <label className="text-sm font-medium">密码</label>
                                <Input
                                    type="password"
                                    placeholder="请输入密码"
                                    value={loginPass}
                                    onChange={e => setLoginPass(e.target.value)}
                                    onKeyDown={e => { if (e.key === 'Enter') handleLogin(); }}
                                />
                            </div>
                            {loginError && (
                                <div className="text-sm text-destructive text-center">{loginError}</div>
                            )}
                            <Button className="w-full" disabled={loginLoading} onClick={handleLogin}>
                                {loginLoading ? '登录中…' : '登录'}
                            </Button>
                        </div>
                    </CardContent>
                </Card>
            </div>
        );
    }

    return (<>
        <div className="aix-shell cyber-bg">
            <Dialog open={detailOpen} onOpenChange={(v) => (v ? null : closeDetail())}>
                <DialogContent className="max-h-[85vh] overflow-auto">
                    <DialogHeader>
                        <DialogTitle>{detailTitle || '详情'}</DialogTitle>
                        <button className="aix-btn" onClick={closeDetail}>关闭</button>
                    </DialogHeader>
                    <div style={{ height: 12 }} />
                    {renderDetailContent()}
                </DialogContent>
            </Dialog>

            {toast && (
                <div
                    style={{
                        position: 'fixed', top: 24, left: '50%', transform: 'translateX(-50%)',
                        zIndex: 9999, padding: '10px 24px', borderRadius: 8,
                        fontSize: 14, fontWeight: 500, pointerEvents: 'none',
                        color: '#fff',
                        background: toast.type === 'ok' ? 'rgba(34,197,94,0.9)' : 'rgba(239,68,68,0.9)',
                        boxShadow: '0 4px 16px rgba(0,0,0,0.25)',
                        animation: 'fadeIn 0.15s ease-out',
                    }}
                >
                    {toast.msg}
                </div>
            )}

            <div className="aix-header">
                <div>
                    <div className="aix-title">AIxVuln AI驱动全流程漏洞挖掘</div>
                </div>
                <div className="aix-actions">
                    {view === 'detail' ? (
                        <>
                            <button className="aix-nav-pill aix-nav-pill--accent" onClick={() => {
                                connectWS(detailProject);
                                refreshProjectDetail().catch(() => null);
                            }}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21.5 2v6h-6"/><path d="M21.34 15.57a10 10 0 1 1-.57-8.38"/></svg>
                                刷新详情
                            </button>
                            <button className="aix-nav-pill" onClick={leaveProject}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m15 18-6-6 6-6"/></svg>
                                返回列表
                            </button>
                        </>
                    ) : view === 'settings' || view === 'mcp' || view === 'skills' || view === 'digital_humans' || view === 'report_templates' || view === 'graph' ? (
                        <>
                            <button className="aix-nav-pill" onClick={() => { window.location.hash = '#/'; }}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m15 18-6-6 6-6"/></svg>
                                返回列表
                            </button>
                        </>
                    ) : (
                        <>
                            <button className="aix-nav-pill aix-nav-pill--accent" onClick={refreshProjects}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21.5 2v6h-6"/><path d="M21.34 15.57a10 10 0 1 1-.57-8.38"/></svg>
                                刷新
                            </button>
                            <div className="aix-nav-divider" />
                            <button className="aix-nav-pill" onClick={() => { window.location.hash = '#/digital_humans'; }}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
                                数字人
                            </button>
                            <button className="aix-nav-pill" onClick={() => { window.location.hash = '#/report_templates'; }}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>
                                报告模板
                            </button>
                            <button className="aix-nav-pill" onClick={() => { window.location.hash = '#/graph'; }}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="6" cy="6" r="2.5"/><circle cx="18" cy="6" r="2.5"/><circle cx="12" cy="18" r="2.5"/><path d="M8.2 7.2 10.3 16"/><path d="M15.8 7.2 13.7 16"/><path d="M8.5 6h7"/></svg>
                                溯源图
                            </button>
                            <button className="aix-nav-pill" onClick={() => { window.location.hash = '#/mcp'; }}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="4" width="18" height="8" rx="2"/><rect x="3" y="12" width="18" height="8" rx="2"/><path d="M7 8h.01"/><path d="M7 16h.01"/><path d="M11 8h6"/><path d="M11 16h6"/></svg>
                                MCP
                            </button>
                            <button className="aix-nav-pill" onClick={() => { window.location.hash = '#/skills'; }}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></svg>
                                Skills
                            </button>
                            <button className="aix-nav-pill" onClick={() => { window.location.hash = '#/settings'; }}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
                                设置
                            </button>
                        </>
                    )}
                    {token && (
                        <>
                            <div className="aix-nav-divider" />
                            <button className="aix-nav-pill aix-nav-pill--muted" onClick={handleLogout}>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
                                退出
                            </button>
                        </>
                    )}
                </div>
            </div>

            <div className={view === 'detail' ? 'aix-grid aix-grid--detail' : 'aix-grid'}>
                {view === 'settings' ? (() => {
                    const sectionDescriptions: Record<string, string> = {
                        misc: '全局通用配置（消息长度限制、重试次数、数据目录等）',
                        decision: '决策大脑专用配置（可覆盖 main_setting 中的 LLM 和并发配置）',
                        main_setting: '全局默认 LLM 配置（各职业配置未单独设置时使用）',
                        general: '通用杂项专家专用配置（可覆盖 main_setting）',
                        pentest: '黑盒渗透专家专用配置（可覆盖 main_setting）',
                        src: 'SRC 深挖专家专用配置（可覆盖 main_setting）',
                        remote_control: '后渗透专家专用配置（历史 section 名仍为 remote_control，可覆盖 main_setting）',
                        ops: '运维专家专用配置（可覆盖 main_setting）',
                        analyze: '代码审计专家专用配置（可覆盖 main_setting）',
                        android: 'Android 逆向研究员专用配置（可覆盖 main_setting）',
                        verifier: '漏洞验证专家专用配置（可覆盖 main_setting）',
                        report: '报告编写专员专用配置（可覆盖 main_setting）',
                        overview: '项目概览专用配置（可覆盖 main_setting）',
                    };
                    const keyDescriptions: Record<string, string> = {
                        MaxContext: '最大Context大小（KB），超过将触发压缩机制',
                        MessageMaximum: '单条消息最大长度（字符），超过将截断',
                        MaxTryCount: 'API请求错误的最大重试次数',
                        PROJECT_TASK_MAX_CONCURRENCY: '项目任务最大并发数（点击“启动”后先进入队列，按该并发数执行）',
                        DATA_DIR: '数据存储目录',
                        AUTO_MODE: '全局自动化模式。off = 关闭；full = 全自动。',
                        EMPTY_PROJECT_SKIP_OVERVIEW: '新建空项目时是否跳过项目概览。true = 跳过；false = 仍执行概览与基础侦查。',
                        MaxRequest: '单个API-KEY的最大并发请求数',
                        FeiShuAPI: '飞书机器人 Webhook API 地址',
                        AGENT_TOOLS_BASE_URL: '给数字人使用的工具下载基址兜底配置。系统优先使用启动时传入的 -ip 与运行端口生成 download_url；仅在运行时地址不可用时才回退到这里。',
                        USER_AGENT: 'HTTP请求的 User-Agent 头',
                        STREAM: '是否启用流式响应（true/false）',
                        API_MODE: 'API模式：chat = OpenAI Chat，responses = OpenAI Responses，anthropic/claude = Claude Messages API',
                        BASE_URL: 'LLM API 基础地址。Claude 官方可填 https://api.anthropic.com',
                        OPENAI_API_KEY: 'API密钥（多个用 |-| 分隔；Claude Key 也填这里）',
                        MODEL: '模型名称',
                    };
                    const sectionKeys: Record<string, string[]> = {
                        misc: ['MessageMaximum', 'MaxTryCount', 'PROJECT_TASK_MAX_CONCURRENCY', 'DATA_DIR', 'AUTO_MODE', 'EMPTY_PROJECT_SKIP_OVERVIEW', 'FeiShuAPI', 'AGENT_TOOLS_BASE_URL'],
                        main_setting: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        decision: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        general: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        pentest: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        src: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        remote_control: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        ops: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        analyze: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        android: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        verifier: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        report: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                        overview: ['BASE_URL', 'OPENAI_API_KEY', 'MODEL', 'MaxContext', 'MaxRequest', 'USER_AGENT', 'STREAM', 'API_MODE'],
                    };
                    const requiredKeys = new Set(['main_setting:BASE_URL', 'main_setting:OPENAI_API_KEY', 'main_setting:MODEL']);
                    const isRequired = (section: string, key: string) => requiredKeys.has(`${section}:${key}`);
                    const predefinedSections = Object.keys(sectionKeys);
                    const availableSectionsToAdd = predefinedSections.filter(s => !configDraft[s]);
                    return (
                <>
                <Card className="col-span-full">
                    <CardHeader>
                        <div className="flex flex-col sm:flex-row sm:items-center justify-between w-full gap-3">
                            <div>
                                <CardTitle>系统设置</CardTitle>
                                <div className="mt-1 text-xs text-muted-foreground">配置存储于 SQLite · 首次启动自动生成默认值 · 保存后部分配置需要重启生效</div>
                            </div>
                            <div className="flex items-center gap-2 shrink-0">
                                <Button size="sm" variant="outline" onClick={fetchConfig} disabled={configLoading}>{configLoading ? '加载中…' : '重新加载'}</Button>
                                <Button size="sm" onClick={saveConfig} disabled={configSaving}>{configSaving ? '保存中…' : '保存配置'}</Button>
                            </div>
                        </div>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-6">
                            {Object.keys(configDraft).length === 0 && !configLoading ? (
                                <div className="text-sm text-muted-foreground p-4 text-center">暂无配置数据，请点击"重新加载"</div>
                            ) : null}
                            {Object.entries(configDraft).map(([section, kv]) => {
                                const allKeys = sectionKeys[section] || [];
                                const existingKeys = Object.keys(kv);
                                const availableKeys = allKeys.filter(k => !existingKeys.includes(k));
                                const sectionLooksLikeLLMConfig = allKeys.includes('MODEL') || existingKeys.includes('MODEL');
                                const effectiveBaseURL = getEffectiveConfigValue(section, 'BASE_URL');
                                const effectiveAPIKey = getEffectiveConfigValue(section, 'OPENAI_API_KEY');
                                const effectiveModel = getEffectiveConfigValue(section, 'MODEL');
                                const sectionLLMTestResult = llmTestResults[section];
                                return (
                                    <div key={section} className="rounded-lg border border-border bg-background/30 overflow-hidden">
                                        <div className="flex items-center justify-between px-4 py-3 bg-muted/30 border-b border-border">
                                            <div>
                                                <span className="font-semibold text-sm">[{section}]</span>
                                                {sectionDescriptions[section] && (
                                                    <span className="ml-2 text-xs text-muted-foreground">{sectionDescriptions[section]}</span>
                                                )}
                                            </div>
                                            <Button size="sm" variant="destructive" onClick={() => { if (confirm(`确定删除整个 [${section}] 配置段？`)) deleteConfigSection(section); }}>删除段</Button>
                                        </div>
                                        <div className="p-4 space-y-3">
                                            {Object.entries(kv).map(([key, value]) => {
                                                const models = modelOptions[section] || [];
                                                const isModelKey = key === 'MODEL';
                                                const hasModels = isModelKey && models.length > 0;
                                                const isCustomModel = isModelKey && value.trim() !== '' && !models.includes(value);
                                                return (
                                                <div key={key} className="aix-config-row flex items-start gap-2">
                                                    <div className="aix-config-label w-[200px] shrink-0">
                                                        <div className="text-sm font-medium">{key}{isRequired(section, key) && <span className="text-destructive ml-1">*</span>}</div>
                                                        {keyDescriptions[key] && (
                                                            <div className="text-[11px] text-muted-foreground leading-tight mt-0.5">{keyDescriptions[key]}</div>
                                                        )}
                                                    </div>
                                                    {isModelKey ? (
                                                        <div className="flex-1 flex items-center gap-2 min-w-0">
                                                            {hasModels && (
                                                                <select
                                                                    className="aix-select flex-1 font-mono"
                                                                    value={isCustomModel ? '__custom__' : value}
                                                                    onChange={e => {
                                                                        if (e.target.value === '__custom__') return;
                                                                        updateConfigDraft(section, key, e.target.value);
                                                                    }}
                                                                >
                                                                    <option value="">选择模型…</option>
                                                                    {models.map(m => (
                                                                        <option key={m} value={m}>{m}</option>
                                                                    ))}
                                                                    <option value="__custom__">✏ 自定义输入</option>
                                                                </select>
                                                            )}
                                                            {(!hasModels || isCustomModel) && (
                                                                <Input
                                                                    className={`flex-1 font-mono text-sm ${isRequired(section, key) && !value.trim() ? 'border-destructive' : ''}`}
                                                                    value={value}
                                                                    placeholder={isRequired(section, key) ? '必填 — 输入模型名称' : '输入模型名称'}
                                                                    onChange={e => updateConfigDraft(section, key, e.target.value)}
                                                                />
                                                            )}
                                                            <Button
                                                                size="sm"
                                                                variant="outline"
                                                                className="shrink-0 text-xs"
                                                                disabled={modelLoading[section] || !effectiveBaseURL || !effectiveAPIKey}
                                                                onClick={() => fetchModels(section)}
                                                            >
                                                                {modelLoading[section] ? '获取中…' : '获取模型'}
                                                            </Button>
                                                            <Button
                                                                size="sm"
                                                                variant="outline"
                                                                className="shrink-0 text-xs"
                                                                disabled={llmTestLoading[section] || !effectiveBaseURL || !effectiveAPIKey || !effectiveModel}
                                                                onClick={() => testLLM(section)}
                                                            >
                                                                {llmTestLoading[section] ? '测试中…' : 'Say Hello'}
                                                            </Button>
                                                        </div>
                                                    ) : (
                                                        <Input
                                                            className={`flex-1 font-mono text-sm ${isRequired(section, key) && !value.trim() ? 'border-destructive' : ''}`}
                                                            value={value}
                                                            type={key === 'OPENAI_API_KEY' ? 'password' : 'text'}
                                                            placeholder={isRequired(section, key) ? '必填' : ''}
                                                            onChange={e => updateConfigDraft(section, key, e.target.value)}
                                                            onBlur={() => {
                                                                if ((key === 'BASE_URL' || key === 'OPENAI_API_KEY') && effectiveBaseURL && effectiveAPIKey) {
                                                                    fetchModels(section);
                                                                }
                                                            }}
                                                        />
                                                    )}
                                                    <Button size="sm" variant="ghost" className="text-destructive shrink-0" onClick={() => deleteConfigKey(section, key)}>✕</Button>
                                                </div>
                                                );
                                            })}
                                            {availableKeys.length > 0 && (
                                            <div className="aix-config-add-row flex items-center gap-2 pt-2 border-t border-border/50">
                                                <select
                                                    className="aix-select w-[220px]"
                                                    value={newKeyInputs[section]?.key || ''}
                                                    onChange={e => setNewKeyInputs(prev => ({ ...prev, [section]: { key: e.target.value, value: prev[section]?.value || '' } }))}
                                                >
                                                    <option value="">选择配置项…</option>
                                                    {availableKeys.map(k => (
                                                        <option key={k} value={k}>{k}{keyDescriptions[k] ? ` — ${keyDescriptions[k]}` : ''}</option>
                                                    ))}
                                                </select>
                                                <Input
                                                    className="flex-1 text-sm"
                                                    placeholder="值"
                                                    value={newKeyInputs[section]?.value || ''}
                                                    onChange={e => setNewKeyInputs(prev => ({ ...prev, [section]: { key: prev[section]?.key || '', value: e.target.value } }))}
                                                    onKeyDown={e => { if (e.key === 'Enter') addConfigKey(section); }}
                                                />
                                                <Button size="sm" variant="outline" onClick={() => addConfigKey(section)}>添加</Button>
                                            </div>
                                            )}
                                            {sectionLooksLikeLLMConfig && sectionLLMTestResult && (
                                                <div className={`rounded-md border px-3 py-2 text-xs leading-5 ${sectionLLMTestResult.ok ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-200' : 'border-destructive/40 bg-destructive/10 text-destructive'}`}>
                                                    <div className="font-medium">
                                                        {sectionLLMTestResult.ok ? 'API 测试成功' : 'API 测试失败'}
                                                        {sectionLLMTestResult.backend ? ` · ${sectionLLMTestResult.backend}` : ''}
                                                    </div>
                                                    {sectionLLMTestResult.reply ? (
                                                        <div className="mt-1 break-all whitespace-pre-wrap">返回内容: {sectionLLMTestResult.reply}</div>
                                                    ) : null}
                                                    {sectionLLMTestResult.message ? (
                                                        <div className="mt-1 break-all whitespace-pre-wrap">{sectionLLMTestResult.message}</div>
                                                    ) : null}
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                );
                            })}
                            {availableSectionsToAdd.length > 0 && (
                            <div className="flex items-center gap-2 pt-2 border-t border-border">
                                <select
                                    className="aix-select w-[280px]"
                                    value={newSectionName}
                                    onChange={e => setNewSectionName(e.target.value)}
                                >
                                    <option value="">选择配置段…</option>
                                    {availableSectionsToAdd.map(s => (
                                        <option key={s} value={s}>[{s}]{sectionDescriptions[s] ? ` — ${sectionDescriptions[s]}` : ''}</option>
                                    ))}
                                </select>
                                <Button size="sm" variant="outline" disabled={!newSectionName} onClick={() => addConfigSection(newSectionName)}>添加配置段</Button>
                            </div>
                            )}
                        </div>
                    </CardContent>
                </Card>
                </>
                    );
                })()
                : view === 'mcp' ? (() => {
                    return (
                <Card className="col-span-full">
                    <CardHeader>
                        <div className="flex flex-col sm:flex-row sm:items-center justify-between w-full gap-3">
                            <div>
                                <CardTitle>MCP 管理</CardTitle>
                                <div className="mt-1 text-xs text-muted-foreground">统一管理所有 MCP 连接、测试可用性，并按数字人职业注册。同一个 MCP 可以对不同职业暴露不同工具子集。</div>
                                <div className="mt-1 text-xs text-amber-300/90">`ida-pro-mcp`、`jadx-ai-mcp`、`c2-mcp` 会作为内置 MCP 默认出现在列表中，可按需启用、配置和绑定数字人职业。</div>
                            </div>
                            <div className="flex items-center gap-2 shrink-0">
                                <Button size="sm" variant="outline" onClick={fetchMCPSettings} disabled={mcpLoading}>{mcpLoading ? '加载中…' : '重新加载'}</Button>
                                <Button size="sm" variant="outline" onClick={addMCPConfig}>新增 MCP</Button>
                                <Button size="sm" onClick={saveMCPSettings} disabled={mcpSaving}>{mcpSaving ? '保存中…' : '保存 MCP'}</Button>
                            </div>
                        </div>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-5">
                            {mcpConfigs.length === 0 && !mcpLoading ? (
                                <div className="text-sm text-muted-foreground p-4 text-center border border-dashed border-border rounded-lg">暂无 MCP 配置，点击“新增 MCP”开始添加。</div>
                            ) : null}
                            {mcpConfigs.map((row, index) => {
                                const test = mcpTestResults[row.id];
                                const bindings = normalizeMCPAgentBindings(row.agent_bindings, row.agent_types);
                                const boundCount = bindings.length;
                                const expanded = !!mcpExpanded[row.id];
                                const argList = normalizeOrderedStringArray(row.args);
                                const argDraft = mcpArgDrafts[row.id] || '';
                                const endpointSummary = row.transport === 'stdio'
                                    ? (row.command?.trim() || '未配置 Command')
                                    : (row.url?.trim() || '未配置 URL');
                                return (
                                    <div key={row.id} className="rounded-lg border border-border bg-background/30 overflow-hidden">
                                        <div className="flex flex-col lg:flex-row lg:items-start justify-between gap-3 px-4 py-3 bg-muted/30 border-b border-border">
                                            <button type="button" className="flex-1 min-w-0 text-left" onClick={() => toggleMCPExpanded(row.id)}>
                                                <div className="text-xs text-muted-foreground mb-1">MCP #{index + 1}</div>
                                                <div className="flex items-center gap-2 flex-wrap">
                                                    <span className="text-sm font-semibold break-all">{row.name || row.id}</span>
                                                    <Badge variant={row.enabled ? 'success' : 'secondary'}>{row.enabled ? 'enabled' : 'disabled'}</Badge>
                                                    <Badge variant="secondary">{row.transport}</Badge>
                                                    <Badge variant="default">绑定职业 {boundCount}</Badge>
                                                </div>
                                                <div className="mt-2 text-xs text-muted-foreground break-all">{endpointSummary}</div>
                                                <div className="mt-1 text-[11px] text-muted-foreground break-all">ID: {row.id}</div>
                                            </button>
                                            <div className="flex items-center gap-2 flex-wrap shrink-0">
                                                <Button size="sm" variant="outline" onClick={() => toggleMCPExpanded(row.id)}>
                                                    {expanded ? '折叠' : '展开'}
                                                </Button>
                                                <Button size="sm" variant="outline" disabled={!!mcpTesting[row.id]} onClick={() => testMCPConfigRow(row)}>
                                                    {mcpTesting[row.id] ? '测试中…' : '测试连接'}
                                                </Button>
                                                <Button size="sm" variant="destructive" onClick={() => { if (confirm(`确定删除 MCP「${row.name || row.id}」？`)) removeMCPConfig(row.id); }}>删除</Button>
                                            </div>
                                        </div>
                                        {expanded ? (
                                            <div className="p-4 space-y-4">
                                                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                                                    <div className="space-y-2">
                                                        <div className="text-xs text-muted-foreground">Name</div>
                                                        <Input value={row.name || ''} placeholder="例如：Ghidra Reverse MCP" onChange={e => updateMCPConfig(row.id, { name: e.target.value })} />
                                                    </div>
                                                    <div className="space-y-2">
                                                        <div className="text-xs text-muted-foreground">Transport</div>
                                                        <select className="aix-select w-full" value={row.transport} onChange={e => updateMCPConfig(row.id, { transport: e.target.value as MCPConfigRow['transport'] })}>
                                                            <option value="stdio">Command / stdio</option>
                                                            <option value="streamable">Streamable HTTP</option>
                                                            <option value="sse">Legacy SSE</option>
                                                        </select>
                                                    </div>
                                                    <div className="space-y-2">
                                                        <div className="text-xs text-muted-foreground">Tool Prefix</div>
                                                        <Input value={row.tool_name_prefix || ''} placeholder="例如：MCP_BinaryGhidra" onChange={e => updateMCPConfig(row.id, { tool_name_prefix: e.target.value })} />
                                                    </div>
                                                    <div className="space-y-2">
                                                        <div className="text-xs text-muted-foreground">Connect Timeout (sec)</div>
                                                        <Input type="number" value={String(row.connect_timeout_sec || 20)} onChange={e => updateMCPConfig(row.id, { connect_timeout_sec: Number(e.target.value) || 20 })} />
                                                    </div>
                                                    <div className="space-y-2">
                                                        <div className="text-xs text-muted-foreground">Call Timeout (sec)</div>
                                                        <Input type="number" value={String(row.call_timeout_sec || 120)} onChange={e => updateMCPConfig(row.id, { call_timeout_sec: Number(e.target.value) || 120 })} />
                                                    </div>
                                                </div>
                                                <div className="flex flex-wrap gap-5">
                                                    <label className="inline-flex items-center gap-2 text-sm">
                                                        <input type="checkbox" checked={!!row.enabled} onChange={e => updateMCPConfig(row.id, { enabled: e.target.checked })} />
                                                        启用
                                                    </label>
                                                    <label className="inline-flex items-center gap-2 text-sm">
                                                        <input type="checkbox" checked={!!row.preserve_schema} onChange={e => updateMCPConfig(row.id, { preserve_schema: e.target.checked })} />
                                                        保留原始 Tool Schema
                                                    </label>
                                                </div>
                                                {row.transport === 'stdio' ? (
                                                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                                                        <div className="space-y-2 lg:col-span-2">
                                                            <div className="text-xs text-muted-foreground">Command</div>
                                                            <Input value={row.command || ''} placeholder="/path/to/mcp-server" onChange={e => updateMCPConfig(row.id, { command: e.target.value })} />
                                                        </div>
                                                        <div className="space-y-2">
                                                            <div className="flex items-center justify-between gap-2">
                                                                <div className="text-xs text-muted-foreground">Args（每个参数一个输入框，按顺序保存）</div>
                                                                <div className="text-[11px] text-muted-foreground">{argList.length} 项</div>
                                                            </div>
                                                            <div className="space-y-2">
                                                                {argList.map((arg, argIndex) => (
                                                                    <div key={`${row.id}-arg-${argIndex}`} className="flex items-center gap-2">
                                                                        <div className="w-7 shrink-0 text-center text-[11px] text-muted-foreground">{argIndex + 1}</div>
                                                                        <Input
                                                                            className="font-mono text-sm"
                                                                            value={arg}
                                                                            placeholder="例如：--stdio"
                                                                            onChange={e => updateMCPArg(row.id, argIndex, e.target.value)}
                                                                        />
                                                                        <Button
                                                                            type="button"
                                                                            size="sm"
                                                                            variant="ghost"
                                                                            className="shrink-0 text-destructive"
                                                                            onClick={() => removeMCPArg(row.id, argIndex)}
                                                                        >
                                                                            删除
                                                                        </Button>
                                                                    </div>
                                                                ))}
                                                                <div className="flex items-center gap-2">
                                                                    <div className="w-7 shrink-0 text-center text-[11px] text-muted-foreground">{argList.length + 1}</div>
                                                                    <Input
                                                                        className="font-mono text-sm"
                                                                        value={argDraft}
                                                                        placeholder={argList.length === 0 ? '--stdio' : '新增参数，回车或点添加'}
                                                                        onChange={e => updateMCPArgDraft(row.id, e.target.value)}
                                                                        onKeyDown={e => {
                                                                            if (e.key === 'Enter') {
                                                                                e.preventDefault();
                                                                                commitMCPArgDraft(row.id);
                                                                            }
                                                                        }}
                                                                    />
                                                                    <Button
                                                                        type="button"
                                                                        size="sm"
                                                                        variant="outline"
                                                                        className="shrink-0"
                                                                        disabled={!argDraft.trim()}
                                                                        onClick={() => commitMCPArgDraft(row.id)}
                                                                    >
                                                                        添加
                                                                    </Button>
                                                                </div>
                                                            </div>
                                                            <div className="text-[11px] text-muted-foreground">参数顺序会按从上到下原样保存，不再自动排序。</div>
                                                        </div>
                                                        <div className="space-y-2">
                                                            <div className="text-xs text-muted-foreground">Env（每行 key=value）</div>
                                                            <Textarea className="min-h-[96px] font-mono text-sm" value={mapToKVText(row.env)} onChange={e => updateMCPConfig(row.id, { env: kvTextToMap(e.target.value) })} placeholder="GHIDRA_HOME=/opt/ghidra" />
                                                        </div>
                                                        <div className="space-y-2 lg:col-span-2">
                                                            <div className="text-xs text-muted-foreground">WorkDir</div>
                                                            <Input value={row.workdir || ''} placeholder="/path/to/workdir" onChange={e => updateMCPConfig(row.id, { workdir: e.target.value })} />
                                                        </div>
                                                    </div>
                                                ) : (
                                                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                                                        <div className="space-y-2 lg:col-span-2">
                                                            <div className="text-xs text-muted-foreground">URL</div>
                                                            <Input value={row.url || ''} placeholder={row.transport === 'sse' ? 'http://127.0.0.1:8080/sse' : 'http://127.0.0.1:8080/mcp'} onChange={e => updateMCPConfig(row.id, { url: e.target.value })} />
                                                        </div>
                                                        <div className="space-y-2 lg:col-span-2">
                                                            <div className="text-xs text-muted-foreground">Headers（每行 key=value）</div>
                                                            <Textarea className="min-h-[96px] font-mono text-sm" value={mapToKVText(row.headers)} onChange={e => updateMCPConfig(row.id, { headers: kvTextToMap(e.target.value) })} placeholder="Authorization=Bearer xxx" />
                                                        </div>
                                                    </div>
                                                )}
                                                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                                                    <div className="space-y-2">
                                                        <div className="text-xs text-muted-foreground">Tool Allowlist（每行一个，可空）</div>
                                                        <Textarea className="min-h-[96px] font-mono text-sm" value={listToLines(row.tool_allowlist)} onChange={e => updateMCPConfig(row.id, { tool_allowlist: linesToList(e.target.value) })} placeholder={"list_functions\ndecompile_function"} />
                                                    </div>
                                                    <div className="space-y-2">
                                                        <div className="text-xs text-muted-foreground">Tool Denylist（每行一个，可空）</div>
                                                        <Textarea className="min-h-[96px] font-mono text-sm" value={listToLines(row.tool_denylist)} onChange={e => updateMCPConfig(row.id, { tool_denylist: linesToList(e.target.value) })} placeholder={"dangerous_tool"} />
                                                    </div>
                                                </div>
                                                <div className="space-y-2">
                                                    <div className="text-xs text-muted-foreground">注册到哪些数字人职业</div>
                                                    <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
                                                        {mcpAgentTargets.map(target => {
                                                            const checked = bindings.some(binding => binding.agent_type === target.agent_type);
                                                            const personas = (target.digital_humans || []).map(item => item.persona_name).join('、');
                                                            return (
                                                                <label key={target.agent_type} className={`rounded-lg border px-3 py-3 cursor-pointer transition ${checked ? 'border-emerald-500 bg-emerald-500/5' : 'border-border bg-background/40'}`}>
                                                                    <div className="flex items-start gap-3">
                                                                        <input className="mt-1" type="checkbox" checked={checked} onChange={() => toggleMCPAgentType(row.id, target.agent_type)} />
                                                                        <div className="min-w-0">
                                                                            <div className="text-sm font-medium">{getRoleLabel(target.agent_type)}</div>
                                                                            <div className="mt-1 text-xs text-muted-foreground">{target.description}</div>
                                                                            <div className="mt-2 text-xs">
                                                                                已注册数字人：{personas || '暂无'}
                                                                            </div>
                                                                        </div>
                                                                    </div>
                                                                </label>
                                                            );
                                                        })}
                                                    </div>
                                                </div>
                                                {bindings.length > 0 ? (
                                                    <div className="space-y-3">
                                                        <div className="text-xs text-muted-foreground">按职业覆盖工具过滤</div>
                                                        <div className="text-[11px] text-muted-foreground">留空表示继承上方默认 Tool Allowlist / Denylist。职业级 Allowlist 会覆盖默认 Allowlist；职业级 Denylist 会在默认 Denylist 基础上追加。</div>
                                                        {bindings.map(binding => {
                                                            const target = mcpAgentTargets.find(item => item.agent_type === binding.agent_type);
                                                            return (
                                                                <div key={`${row.id}-${binding.agent_type}`} className="rounded-lg border border-border bg-background/40 p-3 space-y-3">
                                                                    <div>
                                                                        <div className="text-sm font-medium">{getRoleLabel(binding.agent_type)}</div>
                                                                    </div>
                                                                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                                                                        <div className="space-y-2">
                                                                            <div className="text-xs text-muted-foreground">职业 Allowlist Override（每行一个，可空）</div>
                                                                            <Textarea
                                                                                className="min-h-[96px] font-mono text-sm"
                                                                                value={listToLines(binding.tool_allowlist)}
                                                                                onChange={e => updateMCPAgentBinding(row.id, binding.agent_type, { tool_allowlist: linesToList(e.target.value) })}
                                                                                placeholder={"tool_for_this_agent_only\nanother_tool"}
                                                                            />
                                                                        </div>
                                                                        <div className="space-y-2">
                                                                            <div className="text-xs text-muted-foreground">职业 Denylist Add-on（每行一个，可空）</div>
                                                                            <Textarea
                                                                                className="min-h-[96px] font-mono text-sm"
                                                                                value={listToLines(binding.tool_denylist)}
                                                                                onChange={e => updateMCPAgentBinding(row.id, binding.agent_type, { tool_denylist: linesToList(e.target.value) })}
                                                                                placeholder={"tool_to_hide_from_this_agent"}
                                                                            />
                                                                        </div>
                                                                    </div>
                                                                </div>
                                                            );
                                                        })}
                                                    </div>
                                                ) : null}
                                                {test ? (
                                                    <div className={`rounded-lg border px-3 py-3 text-sm ${test.ok ? 'border-emerald-500/40 bg-emerald-500/5' : 'border-destructive/40 bg-destructive/5'}`}>
                                                        <div className="font-medium">{test.ok ? '测试成功' : '测试失败'}</div>
                                                        <div className="mt-1 text-xs text-muted-foreground">{test.message || '无详细信息'}</div>
                                                        {Array.isArray(test.tool_names) && test.tool_names.length > 0 ? (
                                                            <div className="mt-2 text-xs break-all">Tools: {test.tool_names.join(', ')}</div>
                                                        ) : null}
                                                    </div>
                                                ) : null}
                                            </div>
                                        ) : null}
                                    </div>
                                );
                            })}
                        </div>
                    </CardContent>
                </Card>
                    );
                })()
                : view === 'skills' ? (() => {
                    const selectedEditable = !!selectedSkill?.editable;
                    return (
                <>
                <Card className="col-span-full">
                    <CardHeader>
                        <div className="flex flex-col sm:flex-row sm:items-center justify-between w-full gap-3">
                            <div>
                                <CardTitle>Skills 管理</CardTitle>
                                <div className="mt-1 text-xs text-muted-foreground">管理本地 skills 搜索路径、托管目录与技能文件。托管根目录内的 skill 可直接编辑，外部搜索路径中的 skill 仅查看。</div>
                            </div>
                            <div className="flex items-center gap-2 shrink-0">
                                <Button size="sm" variant="outline" onClick={() => fetchSkillsSettings(selectedSkillName, selectedSkillPath)} disabled={skillsLoading}>{skillsLoading ? '加载中…' : '重新加载'}</Button>
                                <Button size="sm" onClick={saveSkillSettings} disabled={skillsSaving}>{skillsSaving ? '保存中…' : '保存搜索路径'}</Button>
                            </div>
                        </div>
                    </CardHeader>
                    <CardContent>
                        <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <div className="text-xs text-muted-foreground">搜索路径（换行 / 分号 / 逗号分隔；留空则使用默认路径）</div>
                                <Textarea
                                    className="min-h-[96px] font-mono text-sm"
                                    value={skillSearchPathsDraft}
                                    onChange={e => setSkillSearchPathsDraft(e.target.value)}
                                    placeholder={"./skills\n./data/skills"}
                                />
                                <div className="text-[11px] text-muted-foreground">当前托管根目录：<span className="font-mono break-all">{skillManagedRoot || '未解析'}</span></div>
                            </div>
                            <div className="space-y-2">
                                <div className="text-xs text-muted-foreground">当前生效的解析根目录</div>
                                <div className="rounded-lg border border-border bg-background/30 p-3 space-y-2">
                                    {skillResolvedRoots.length === 0 ? (
                                        <div className="text-xs text-muted-foreground">暂无</div>
                                    ) : skillResolvedRoots.map(root => (
                                        <div key={root} className="text-xs font-mono break-all">{root}</div>
                                    ))}
                                </div>
                            </div>
                        </div>
                        <div className="mt-4 rounded-lg border border-border bg-background/30 p-4">
                            <div className="text-sm font-semibold mb-3">新建 Skill</div>
                            <div className="grid grid-cols-1 xl:grid-cols-[240px_minmax(0,1fr)_auto] gap-3">
                                <Input
                                    value={newSkillName}
                                    onChange={e => setNewSkillName(e.target.value)}
                                    placeholder="skill 名称，例如 android_apk"
                                />
                                <Input
                                    value={newSkillDescription}
                                    onChange={e => setNewSkillDescription(e.target.value)}
                                    placeholder="简短描述，会写入默认 SKILL.md"
                                />
                                <Button onClick={createSkillAPI} disabled={skillsCreating || !newSkillName.trim()}>{skillsCreating ? '创建中…' : '创建 Skill'}</Button>
                            </div>
                            <div className="mt-2 text-[11px] text-muted-foreground">名称限制为字母、数字、点、下划线、短横线，且必须以字母或数字开头。</div>
                        </div>
                    </CardContent>
                </Card>

                <Card className="col-span-full">
                    <CardHeader>
                        <CardTitle>Skill 列表</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-3">
                            {skillItems.length === 0 && !skillsLoading ? (
                                <div className="text-sm text-muted-foreground p-4 text-center border border-dashed border-border rounded-lg">暂无 Skill，先在上方创建，或把技能目录加入搜索路径。</div>
                            ) : null}
                            {skillItems.map(item => {
                                const isActive = selectedSkillName === item.name;
                                return (
                                    <div key={item.name} className={`rounded-lg border p-3 transition ${isActive ? 'border-emerald-500 bg-emerald-500/5' : 'border-border bg-background/30'}`}>
                                        <button
                                            type="button"
                                            className="w-full text-left"
                                            onClick={async () => {
                                                if (skillFileDirty && selectedSkillName && selectedSkillName !== item.name) {
                                                    if (!confirm('当前文件有未保存修改，确认切换 Skill 并丢弃这些修改？')) return;
                                                }
                                                setSelectedSkillName(item.name);
                                                await loadSkillFiles(item.name);
                                            }}
                                        >
                                            <div className="flex items-center gap-2 flex-wrap">
                                                <span className="font-semibold text-sm break-all">{item.name}</span>
                                                <Badge variant={item.editable ? 'success' : 'secondary'}>{item.editable ? '可编辑' : '只读'}</Badge>
                                                {item.has_scripts ? <Badge variant="secondary">scripts</Badge> : null}
                                                {item.has_references ? <Badge variant="secondary">references</Badge> : null}
                                                {item.has_assets ? <Badge variant="secondary">assets</Badge> : null}
                                            </div>
                                            {item.description ? <div className="mt-2 text-xs text-muted-foreground break-words">{item.description}</div> : null}
                                            <div className="mt-2 text-[11px] font-mono text-muted-foreground break-all">{item.root_path}</div>
                                        </button>
                                        {item.editable ? (
                                            <div className="mt-3 flex justify-end">
                                                <Button size="sm" variant="destructive" disabled={skillsDeleting} onClick={() => deleteSkillAPI(item.name)}>
                                                    {skillsDeleting && selectedSkillName === item.name ? '删除中…' : '删除 Skill'}
                                                </Button>
                                            </div>
                                        ) : null}
                                    </div>
                                );
                            })}
                        </div>
                    </CardContent>
                </Card>

                <Card className="col-span-full">
                    <CardHeader>
                        <div className="flex items-center justify-between gap-3">
                            <div>
                                <CardTitle>{selectedSkillName || 'Skill 编辑器'}</CardTitle>
                                <div className="mt-1 text-xs text-muted-foreground">
                                    {selectedSkill ? (
                                        selectedEditable
                                            ? '当前 Skill 位于托管根目录，可直接编辑。'
                                            : '当前 Skill 来自外部搜索路径，仅支持查看。'
                                    ) : '先从左侧选择一个 Skill。'}
                                </div>
                            </div>
                            {selectedSkill ? (
                                <Badge variant={selectedEditable ? 'success' : 'secondary'}>{selectedEditable ? 'Editable' : 'Read Only'}</Badge>
                            ) : null}
                        </div>
                    </CardHeader>
                    <CardContent>
                        {!selectedSkill ? (
                            <div className="text-sm text-muted-foreground p-4 text-center border border-dashed border-border rounded-lg">请选择一个 Skill。</div>
                        ) : (
                            <div className="space-y-4">
                                <div className="text-[11px] font-mono text-muted-foreground break-all">Root: {selectedSkill.root_path}</div>
                                {selectedEditable ? (
                                    <div className="flex flex-col xl:flex-row gap-2">
                                        <Input
                                            className="font-mono text-sm"
                                            value={newSkillFilePath}
                                            onChange={e => setNewSkillFilePath(e.target.value)}
                                            placeholder="新增文件路径，例如 references/guide.md"
                                        />
                                        <Button variant="outline" onClick={createSkillFile} disabled={skillFileSaving || !newSkillFilePath.trim()}>{skillFileSaving ? '处理中…' : '新增文件'}</Button>
                                    </div>
                                ) : null}
                                <div className="grid grid-cols-1 2xl:grid-cols-[320px_minmax(0,1fr)] gap-4">
                                    <div className="rounded-lg border border-border bg-background/30 p-3 space-y-2">
                                        <div className="flex items-center justify-between gap-2">
                                            <div className="text-sm font-semibold">文件列表</div>
                                            <div className="text-[11px] text-muted-foreground">{skillFiles.length} 项</div>
                                        </div>
                                        <div className="space-y-1 max-h-[560px] overflow-auto pr-1">
                                            {skillFiles.length === 0 ? (
                                                <div className="text-xs text-muted-foreground">暂无文件</div>
                                            ) : skillFiles.map(entry => {
                                                const active = selectedSkillPath === entry.path;
                                                const indent = Math.max(0, Number(entry.depth || 0)) * 12;
                                                return (
                                                    <button
                                                        key={`${entry.type}:${entry.path}`}
                                                        type="button"
                                                        className={`w-full rounded-md px-2 py-2 text-left text-sm transition ${active ? 'bg-emerald-500/10 border border-emerald-500/40' : 'border border-transparent hover:bg-muted/40'} ${entry.type === 'dir' ? 'text-muted-foreground' : ''}`}
                                                        style={{ paddingLeft: `${indent + 8}px` }}
                                                        disabled={entry.type !== 'file'}
                                                        onClick={() => openSkillFile(selectedSkillName, entry.path)}
                                                    >
                                                        <div className="flex items-center justify-between gap-2">
                                                            <span className="truncate font-mono text-[12px]">{entry.path}</span>
                                                            {entry.type === 'file' && typeof entry.size === 'number' ? (
                                                                <span className="shrink-0 text-[10px] text-muted-foreground">{entry.size} B</span>
                                                            ) : null}
                                                        </div>
                                                    </button>
                                                );
                                            })}
                                        </div>
                                    </div>

                                    <div className="rounded-lg border border-border bg-background/30 p-3 space-y-3">
                                        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                                            <div>
                                                <div className="text-sm font-semibold">{selectedSkillPath || '未选择文件'}</div>
                                                <div className="text-[11px] text-muted-foreground">建议优先维护 `SKILL.md`，其它文件按需新增。</div>
                                            </div>
                                            <div className="flex items-center gap-2">
                                                {selectedEditable ? (
                                                    <>
                                                        <Button size="sm" variant="outline" disabled={!selectedSkillPath || selectedSkillPath === 'SKILL.md' || skillFileSaving} onClick={deleteSelectedSkillFile}>删除文件</Button>
                                                        <Button size="sm" disabled={!selectedSkillPath || !skillFileDirty || skillFileSaving} onClick={saveSkillFile}>{skillFileSaving ? '保存中…' : '保存文件'}</Button>
                                                    </>
                                                ) : null}
                                            </div>
                                        </div>
                                        <Textarea
                                            className="min-h-[560px] font-mono text-sm"
                                            value={skillFileContent}
                                            readOnly={!selectedEditable || !selectedSkillPath}
                                            onChange={e => {
                                                setSkillFileContent(e.target.value);
                                                setSkillFileDirty(true);
                                            }}
                                            placeholder={skillFileLoading ? '加载中…' : (selectedSkillPath ? '文件内容' : '从左侧选择文件')}
                                        />
                                        <div className="flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
                                            <span>{skillFileLoading ? '正在读取文件…' : (skillFileDirty ? '有未保存修改' : '内容已同步')}</span>
                                            <span>{skillFileContent.length} chars</span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        )}
                    </CardContent>
                </Card>
                </>
                    );
                })()
                : view === 'digital_humans' ? (() => {
                    const agentTypeLabels: Record<string, string> = {
                        'Agent-General-GeneralCommonAgent': '通用杂项专家',
                        'Agent-Pentest-PentestCommonAgent': '渗透突破专家',
                        'Agent-Pentest-SrcCommonAgent': '黑盒漏洞研究员',
                        'Agent-Pentest-PostExploitationCommonAgent': '后渗透研究员',
                        'Agent-Binary-ReverseCommonAgent': '二进制逆向研究员',
                        'Agent-Binary-AndroidCommonAgent': 'Android逆向研究员',
                        'Agent-Ops-OpsCommonAgent': '环境运维专家',
                        'Agent-Ops-OpsEnvScoutAgent': '远程环境运维工程师',
                        'Agent-Analyze-AnalyzeCommonAgent': '代码审计专家',
                        'Agent-Verifier-VerifierCommonAgent': '漏洞验证专家',
                        'Agent-Report-ReportCommonAgent': '报告编写专员',
                    };
                    const agentTypes = Object.keys(agentTypeLabels);
                    const emptyDh = () => ({
                        id: generateUUID(),
                        agent_type: agentTypes[0],
                        persona_name: '',
                        gender: '男',
                        avatar_file: '',
                        personality: '',
                        age: 25,
                        extra_sys_prompt: '',
                    });
                    return (
                <Card className="col-span-full">
                    <CardHeader>
                        <div className="flex flex-col sm:flex-row sm:items-center justify-between w-full gap-3">
                            <div>
                                <CardTitle>数字人管理</CardTitle>
                                <div className="mt-1 text-xs text-muted-foreground">管理数字人职业与角色 · 修改后重启项目生效</div>
                            </div>
                            <div className="flex items-center gap-2 shrink-0">
                                <Button size="sm" variant="outline" onClick={fetchDigitalHumans} disabled={dhLoading}>{dhLoading ? '加载中…' : '刷新'}</Button>
                                <Button size="sm" onClick={() => setDhEditing(emptyDh())}>新增数字人</Button>
                            </div>
                        </div>
                    </CardHeader>
                    <CardContent>
                        {dhEditing && (
                            <div className="mb-6 rounded-lg border border-primary/30 bg-background/50 p-4 space-y-3">
                                <div className="text-sm font-semibold mb-2">{digitalHumans.some((d: any) => d.id === dhEditing.id) ? '编辑数字人' : '新增数字人'}</div>
                                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                    <div>
                                        <label className="text-xs text-muted-foreground">姓名 <span className="text-destructive">*</span></label>
                                        <Input className="mt-1" value={dhEditing.persona_name} onChange={e => setDhEditing({...dhEditing, persona_name: e.target.value})} placeholder="角色姓名" />
                                    </div>
                                    <div>
                                        <label className="text-xs text-muted-foreground">职业方向 <span className="text-destructive">*</span></label>
                                        <select className="aix-select w-full mt-1" value={dhEditing.agent_type} onChange={e => setDhEditing({...dhEditing, agent_type: e.target.value})}>
                                            {agentTypes.map(t => <option key={t} value={t}>{agentTypeLabels[t]}</option>)}
                                        </select>
                                    </div>
                                    <div>
                                        <label className="text-xs text-muted-foreground">性别</label>
                                        <select className="aix-select w-full mt-1" value={dhEditing.gender} onChange={e => setDhEditing({...dhEditing, gender: e.target.value})}>
                                            <option value="男">男</option>
                                            <option value="女">女</option>
                                        </select>
                                    </div>
                                    <div>
                                        <label className="text-xs text-muted-foreground">年龄</label>
                                        <Input className="mt-1" type="number" value={dhEditing.age} onChange={e => setDhEditing({...dhEditing, age: parseInt(e.target.value) || 0})} />
                                    </div>
                                    <div>
                                        <label className="text-xs text-muted-foreground">头像</label>
                                        <div className="mt-1 flex items-center gap-3">
                                            {dhEditing.avatar_file && (
                                                <img
                                                    src={(() => {
                                                        const f = dhEditing.avatar_file;
                                                        if (f.includes('-') && !f.includes('/')) {
                                                            try { return new URL(`./assets/avatar/${f}`, import.meta.url).toString(); } catch {}
                                                        }
                                                        return `${baseURL}/avatar/${f}`;
                                                    })()}
                                                    alt="avatar"
                                                    className="h-10 w-10 rounded-full border border-border object-cover shrink-0"
                                                    onError={e => { (e.target as HTMLImageElement).style.display = 'none'; }}
                                                />
                                            )}
                                            <div className="flex-1 space-y-1">
                                                <Input
                                                    type="file"
                                                    accept=".png,.jpg,.jpeg,.gif,.webp"
                                                    onChange={async (e) => {
                                                        const file = e.target.files?.[0];
                                                        if (!file) return;
                                                        const fd = new FormData();
                                                        fd.append('file', file);
                                                        try {
                                                            const resp = await fetch(`${baseURL}/avatar/upload`, {
                                                                method: 'POST',
                                                                headers: { Authorization: authHeader },
                                                                body: fd,
                                                            });
                                                            const js = await resp.json();
                                                            const fname = getRespData<string>(js);
                                                            if (fname) {
                                                                setDhEditing((prev: any) => ({ ...prev, avatar_file: fname }));
                                                                showToast('头像上传成功');
                                                            } else {
                                                                showToast(js?.error || '上传失败', 'err');
                                                            }
                                                        } catch (err: any) {
                                                            showToast('上传失败: ' + (err?.message || ''), 'err');
                                                        }
                                                    }}
                                                />
                                                {dhEditing.avatar_file && (
                                                    <div className="text-[10px] text-muted-foreground truncate">{dhEditing.avatar_file}</div>
                                                )}
                                            </div>
                                        </div>
                                    </div>
                                    <div>
                                        <label className="text-xs text-muted-foreground">性格特征</label>
                                        <Input className="mt-1" value={dhEditing.personality} onChange={e => setDhEditing({...dhEditing, personality: e.target.value})} placeholder="例如：沉稳踏实、逻辑缜密" />
                                    </div>
                                </div>
                                <div>
                                    <label className="text-xs text-muted-foreground">额外系统提示词</label>
                                    <Textarea className="mt-1 min-h-[80px] text-sm" value={dhEditing.extra_sys_prompt} onChange={e => setDhEditing({...dhEditing, extra_sys_prompt: e.target.value})} placeholder="自定义该数字人的语气、风格等系统提示词" />
                                </div>
                                <div className="flex gap-2 pt-2">
                                    <Button size="sm" disabled={!dhEditing.persona_name.trim()} onClick={() => saveDigitalHumanAPI(dhEditing)}>保存</Button>
                                    <Button size="sm" variant="outline" onClick={() => setDhEditing(null)}>取消</Button>
                                </div>
                            </div>
                        )}
                        <div className="space-y-3">
                            {digitalHumans.length === 0 && !dhLoading ? (
                                <div className="text-sm text-muted-foreground p-4 text-center">暂无数字人数据</div>
                            ) : null}
                            {digitalHumans.map((dh: any) => {
                                const dhAvatarSrc = (() => {
                                    const f = dh.avatar_file;
                                    if (!f) return '';
                                    if (f.includes('-') && !f.includes('/')) {
                                        try { return new URL(`./assets/avatar/${f}`, import.meta.url).toString(); } catch {}
                                    }
                                    return `${baseURL}/avatar/${f}`;
                                })();
                                return (
                                <div key={dh.id} className="rounded-lg border border-border bg-background/30 p-4 flex items-start gap-4">
                                    {dhAvatarSrc && (
                                        <img src={dhAvatarSrc} alt={dh.persona_name} className="h-10 w-10 rounded-full border border-border object-cover shrink-0" onError={e => { (e.target as HTMLImageElement).style.display = 'none'; }} />
                                    )}
                                    <div className="flex-1 min-w-0">
                                        <div className="flex items-center gap-2 mb-1">
                                            <span className="font-semibold text-sm">{dh.persona_name}</span>
                                            <Badge variant="secondary" className="text-[10px]">{agentTypeLabels[dh.agent_type] || getRoleLabel(dh.agent_type)}</Badge>
                                            <span className="text-xs text-muted-foreground">{dh.gender} · {dh.age}岁</span>
                                        </div>
                                        {dh.personality && <div className="text-xs text-muted-foreground mb-1">性格：{dh.personality}</div>}
                                        {dh.extra_sys_prompt && <div className="text-xs text-muted-foreground/70 line-clamp-2">提示词：{dh.extra_sys_prompt}</div>}
                                    </div>
                                    <div className="flex gap-1 shrink-0">
                                        <Button size="sm" variant="outline" onClick={() => setDhEditing({...dh})}>编辑</Button>
                                        <Button size="sm" variant="destructive" onClick={() => { if (confirm(`确定删除数字人「${dh.persona_name}」？`)) deleteDigitalHumanAPI(dh.id); }}>删除</Button>
                                    </div>
                                </div>
                                );
                            })}
                        </div>
                    </CardContent>
                </Card>
                    );
                })()
                : view === 'report_templates' ? (() => {
                    const templateLabels: Record<string, string> = {
                        verifier: '验证型报告模板（含运行证据、PoC、HTTP包等）',
                        analyze: '分析型报告模板（代码审计、利用链分析）',
                    };
                    return (
                <Card className="col-span-full">
                    <CardHeader>
                        <div className="flex flex-col sm:flex-row sm:items-center justify-between w-full gap-3">
                            <div>
                                <CardTitle>报告模板管理</CardTitle>
                                <div className="mt-1 text-xs text-muted-foreground">自定义漏洞报告模板 · 存储于 data/.reportTemplate/ · 修改后立即生效</div>
                            </div>
                            <div className="flex items-center gap-2 shrink-0">
                                <Button size="sm" variant="outline" onClick={fetchReportTemplates} disabled={rtLoading}>{rtLoading ? '加载中…' : '重新加载'}</Button>
                                <Button size="sm" onClick={saveReportTemplates} disabled={rtSaving}>{rtSaving ? '保存中…' : '保存模板'}</Button>
                            </div>
                        </div>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-6">
                            {Object.entries(reportTemplatesDraft).map(([name, content]) => (
                                <div key={name} className="rounded-lg border border-border bg-background/30 overflow-hidden">
                                    <div className="px-4 py-3 bg-muted/30 border-b border-border">
                                        <span className="font-semibold text-sm">{name}</span>
                                        {templateLabels[name] && (
                                            <span className="ml-2 text-xs text-muted-foreground">{templateLabels[name]}</span>
                                        )}
                                    </div>
                                    <div className="p-4">
                                        <Textarea
                                            className="min-h-[300px] font-mono text-sm"
                                            value={content}
                                            onChange={e => setReportTemplatesDraft(prev => ({ ...prev, [name]: e.target.value }))}
                                        />
                                    </div>
                                </div>
                            ))}
                            {Object.keys(reportTemplatesDraft).length === 0 && !rtLoading && (
                                <div className="text-sm text-muted-foreground p-4 text-center">暂无模板数据</div>
                            )}
                        </div>
                    </CardContent>
                </Card>
                    );
                })()
                : view === 'graph' ? (
                    <GraphPanel baseURL={baseURL} authHeader={authHeader} />
                )
                : view === 'home' ? (
                <>
                <Card>
                    <CardHeader>
                        <CardTitle>实验室</CardTitle>
                        <Badge variant="primary">实验室列表</Badge>
                    </CardHeader>
                    <CardContent>
                        <ScrollArea className="h-[50vh] rounded-lg border border-border bg-background/20">
                            <div className="p-2 space-y-2">
	                                {projects.map((p) => (
	                                    <div
	                                        key={p.projectID}
	                                        className="aix-project-item flex items-center justify-between gap-2 rounded-lg border border-border bg-background/10 px-3 py-2 hover:bg-muted/30"
	                                    >
	                                        <button className="flex-1 text-left min-w-0" onClick={() => enterProject(p.projectID)}>
                                            <div className="font-semibold truncate">{p.projectName}</div>
                                            <div className="mt-1 flex items-center gap-2">
                                                <Badge variant={p.isRunning || p.isStarting ? 'warning' : p.isQueued ? 'secondary' : p.brainFinished ? 'default' : 'secondary'}>
                                                    {p.status || p.rawStatus || (p.isRunning ? '正在运行' : '未运行')}
                                                </Badge>
                                            </div>
	                                        </button>
	                                        <div className="aix-project-btns flex shrink-0 flex-wrap gap-2">
	                                            <Button size="sm" variant="secondary" disabled={!!btnLoading[`start_${p.projectID}`] || !!p.isRunning || !!p.isStarting || !!p.isQueued} onClick={() => startProject(p.projectID, p.projectName)}>{btnLoading[`start_${p.projectID}`] ? '启动中…' : '启动'}</Button>
	                                            <Button size="sm" variant="outline" disabled={!!btnLoading[`stop_${p.projectID}`]} onClick={() => stopProject(p.projectID, p.projectName)}>{btnLoading[`stop_${p.projectID}`] ? '停止中…' : '停止'}</Button>
	                                            <Button size="sm" variant="default" onClick={() => enterProject(p.projectID)}>进入实验室</Button>
	                                            <Button size="sm" variant="destructive" disabled={!!btnLoading[`del_${p.projectID}`]} onClick={() => deleteProject(p.projectID, p.projectName)}>{btnLoading[`del_${p.projectID}`] ? '删除中…' : '删除'}</Button>
	                                        </div>
                                    </div>
                                ))}
                                {projects.length === 0 ? <div className="text-sm text-muted-foreground p-2">暂无实验室</div> : null}
                            </div>
                        </ScrollArea>

                        <div className="mt-3 text-xs text-muted-foreground">
                            说明：现有实验室同时支持 Web 安全研究组与二进制安全研究组并行协作。
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle>创建实验室</CardTitle>
                        <Badge>源码/空工作目录 + 总任务</Badge>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-3">
                            <Input
                                placeholder="实验室名称（可选，例如：demo-lab）"
                                value={projectName}
                                onChange={(e) => setProjectName(e.target.value)}
                            />
                            <Textarea
                                placeholder="实验室总任务（会先由总脑接收，再向研究组分发）"
                                value={taskContent}
                                onChange={(e) => setTaskContent(e.target.value)}
                            />
                            <div>
                                <label className="text-xs text-muted-foreground mb-1 block">源码来源</label>
                                <div className="flex gap-1">
                                    {([['file', '压缩包文件'], ['git', 'Git 仓库'], ['url', '压缩包链接'], ['empty', '空工作目录']] as const).map(([val, label]) => (
                                        <button
                                            key={val}
                                            className={`px-3 py-1.5 text-xs rounded-md border transition-colors ${sourceType === val ? 'bg-primary text-primary-foreground border-primary' : 'bg-background border-border text-muted-foreground hover:text-foreground'}`}
                                            onClick={() => setSourceType(val)}
                                        >
                                            {label}
                                        </button>
                                    ))}
                                </div>
                            </div>
                            {sourceType === 'file' && (
                                <Input ref={fileRef as any} type="file" accept=".zip,.tar.gz,.tgz" />
                            )}
                            {sourceType === 'git' && (
                                <Input
                                    placeholder="Git 仓库地址（例如：https://github.com/user/repo.git）"
                                    value={gitUrl}
                                    onChange={(e) => setGitUrl(e.target.value)}
                                />
                            )}
                            {sourceType === 'url' && (
                                <Input
                                    placeholder="压缩包下载链接（例如：https://example.com/source.zip）"
                                    value={fileUrl}
                                    onChange={(e) => setFileUrl(e.target.value)}
                                />
                            )}
                            {sourceType === 'empty' && (
                                <div className="text-xs text-muted-foreground rounded-md border border-dashed border-border px-3 py-2">
                                    不上传源码，直接创建空的 workdir/sourceCodeDir 目录。适用于黑盒渗透测试、远程目标攻防或后续再下载工件的场景。
                                </div>
                            )}
                            <div className="flex gap-2">
                                <Button disabled={!!btnLoading.create} onClick={createProject}>
                                    {btnLoading.create ? '创建中…' : sourceType === 'file' ? '上传并创建实验室' : sourceType === 'git' ? '克隆并创建实验室' : sourceType === 'url' ? '下载并创建实验室' : '创建空工作目录实验室'}
                                </Button>
                            </div>
                        </div>

                        <div style={{ display: 'none' }}>{initError}{baseURL}{events.length}</div>
                    </CardContent>
                </Card>

                </>
                ) : (
                <>
    <Card className="overflow-hidden min-w-0">
        <CardHeader>
            <div>
                <CardTitle>实验室详情</CardTitle>
                <div className="mt-1 text-xs text-muted-foreground">总脑调度与研究组协同面板</div>
            </div>
            <div className="flex items-center gap-2">
	                <Badge variant="primary">{detailProjectName || detailProject || '-'}</Badge>
                <Badge variant="secondary">{labGroups.length || 0} 个研究组</Badge>
                {controlBrain?.status ? <Badge variant="secondary">总脑 {String(controlBrain.status)}</Badge> : null}
                {projectStatus ? (
                    <Badge variant={brainFinished ? 'default' : projectIsRunning ? 'warning' : 'secondary'}>{projectStatus}</Badge>
                ) : null}
            </div>
        </CardHeader>
        <CardContent className="overflow-hidden min-w-0">
            <div className="flex flex-wrap gap-2">
	                <Button variant="secondary" disabled={!detailProject || projectIsRunning || detailProjectIsQueued || detailProjectIsStarting || !!btnLoading[`start_${detailProject}`]} onClick={() => detailProject && startProject(detailProject, detailProjectName)}>{btnLoading[`start_${detailProject}`] ? '启动中…' : '启动实验室'}</Button>
	                <Button variant="outline" disabled={!detailProject} onClick={() => detailProject && stopProject(detailProject, detailProjectName)}>停止实验室</Button>
                <Button variant={debugMode ? 'secondary' : 'outline'} disabled={!detailProject} onClick={() => setDebugMode(prev => !prev)}>
                    {debugMode ? '关闭调试模式' : '调试模式'}
                </Button>
                {brainFinished && projectStatus === '决策结束' && (
                    <Button
                        variant="destructive"
                        disabled={!detailProject || !!btnLoading[`finish_${detailProject}`]}
                        onClick={async () => {
                            if (!detailProject) return;
                            setBtnLoading(prev => ({ ...prev, [`finish_${detailProject}`]: true }));
                            try {
                                await apiGet(`/projects/${encodeURIComponent(detailProject)}/cancel`);
                                showToast('实验室已结束');
                            } catch (e: any) {
                                showToast(e?.message || '结束失败', 'err');
                            } finally {
                                setBtnLoading(prev => ({ ...prev, [`finish_${detailProject}`]: false }));
                            }
                        }}
                    >
                        {btnLoading[`finish_${detailProject}`] ? '结束中…' : '结束实验室'}
                    </Button>
                )}
                <Button
                    variant="destructive"
                    disabled={!detailProject}
	                    onClick={() => detailProject && deleteProject(detailProject, detailProjectName).then(() => leaveProject())}
                >
                    删除实验室
                </Button>
            </div>

            {debugMode ? (
                <div className="mt-4 rounded-xl border border-fuchsia-500/30 bg-fuchsia-500/5 p-4">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                        <div>
                            <div className="text-sm font-semibold">记忆调试导出</div>
                            <div className="mt-1 text-xs text-muted-foreground">导出当前数字人或研究组决策大脑的记忆快照，便于观察系统提示词、关键记忆、原始 turns，以及当前实际发送给 LLM 的渲染上下文。</div>
                        </div>
                        <Badge variant={exportableMemoryTargets.length > 0 ? 'warning' : 'secondary'}>可导出 {exportableMemoryTargets.length}</Badge>
                    </div>
                    <div className="mt-3 space-y-3">
                        {memoryDebugTargets.map((target) => {
                            const targetId = String(target.target_id || '');
                            const exporting = !!memoryDebugExporting[targetId];
                            return (
                                <div key={`memory-debug-${targetId}`} className="rounded-lg border border-border bg-background/40 p-3">
                                    <div className="flex flex-wrap items-start justify-between gap-3">
                                        <div className="min-w-0 flex-1">
                                            <div className="flex flex-wrap items-center gap-2">
                                                <div className="font-medium text-sm break-all">{target.label || target.persona_name || target.target_type || '未命名目标'}</div>
                                                {target.group_name ? <Badge variant="secondary">{target.group_name}</Badge> : null}
                                                {target.target_type ? <Badge variant="secondary">{target.target_type}</Badge> : null}
                                                {target.state ? <Badge variant="secondary">{target.state}</Badge> : null}
                                                <Badge variant={target.available ? 'success' : 'secondary'}>{target.available ? '可导出' : '不可导出'}</Badge>
                                            </div>
                                            <div className="mt-1 text-xs text-muted-foreground break-words">
                                                {target.agent_tool_name ? `职业：${getRoleLabel(target.agent_tool_name)}` : ''}
                                                {target.digital_human_id ? ` · 数字人ID: ${target.digital_human_id}` : ''}
                                            </div>
                                            {!target.available && target.reason ? (
                                                <div className="mt-2 text-xs text-muted-foreground break-words">{target.reason}</div>
                                            ) : null}
                                        </div>
                                        <div className="flex flex-wrap gap-2">
                                            <Button size="sm" variant="secondary" disabled={!target.available || exporting} onClick={() => exportMemoryDebug(target)}>
                                                {exporting ? '导出中…' : '导出 JSON'}
                                            </Button>
                                        </div>
                                    </div>
                                </div>
                            );
                        })}
                        {memoryDebugTargets.length === 0 ? (
                            <div className="rounded-lg border border-dashed border-border bg-background/30 px-3 py-6 text-sm text-muted-foreground text-center">当前没有可用的记忆导出目标。</div>
                        ) : null}
                    </div>
                </div>
            ) : null}

            <div className="mt-4 rounded-xl border border-amber-500/30 bg-amber-500/5 p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                        <div className="text-sm font-semibold">决策栏</div>
			<div className="mt-1 text-xs text-muted-foreground">数字人提交需要确认的事项时，会进入这里等待审核。高影响操作可批准/拒绝；思路取舍可选择继续或放弃，并可补充引导。</div>
                    </div>
                    <div className="flex items-center gap-2">
                        <Badge variant={pendingHostCommandApprovals.length > 0 ? 'warning' : 'secondary'}>待审 {pendingHostCommandApprovals.length}</Badge>
                        <Badge variant="secondary">活跃 {activeHostCommandApprovals.length}</Badge>
                        <Button size="sm" variant="ghost" className="h-7 px-2 text-xs" onClick={() => setDecisionPanelExpanded(v => !v)}>
                            {decisionPanelExpanded ? '收起' : '展开'}
                        </Button>
                    </div>
                </div>

                {decisionPanelExpanded ? (
                <div className="mt-3 space-y-3">
                    {activeHostCommandApprovals.map((item) => {
                        const approvalId = String(item.id || '');
                        const status = String(item.status || '');
			const isPending = status === 'pending_approval';
			const isActing = !!hostCommandDecisionLoading[approvalId];
			const isHostCommand = String(item.kind || 'host_command') === 'host_command';
			const isAbandonIdea = String(item.kind || '') === 'abandon_idea';
			const kindLabel = isHostCommand ? '命令执行' : isAbandonIdea ? '思路取舍' : '高影响操作';
			const actionText = item.action || item.command || '-';
                        return (
                            <div key={`host-approval-${approvalId}`} className="rounded-lg border border-border bg-background/40 p-3">
                                <div className="flex flex-wrap items-start justify-between gap-3">
                                    <div className="min-w-0 flex-1">
                                        <div className="flex flex-wrap items-center gap-2">
								<div className="font-medium text-sm break-all">{item.title || (isHostCommand ? '宿主机命令执行' : isAbandonIdea ? '是否放弃当前思路' : '待确认操作')}</div>
                                            <Badge variant="secondary">{item.persona_name || getRoleLabel(item.agent_tool_name || '') || '未知请求者'}</Badge>
                                            {item.group_name ? <Badge variant="secondary">{item.group_name}</Badge> : null}
								<Badge variant={isHostCommand ? 'secondary' : isAbandonIdea ? 'primary' : 'warning'}>{kindLabel}</Badge>
                                            <Badge variant={badgeVariantForHostApprovalStatus(status)}>{status || 'unknown'}</Badge>
                                        </div>
                                        <div className="mt-1 text-xs text-muted-foreground break-words">
                                            {item.created_at ? `创建 ${item.created_at}` : '刚刚创建'}
                                            {item.target ? ` · 目标：${item.target}` : ''}
                                            {isHostCommand && item.workdir ? ` · workdir: ${item.workdir}` : ''}
                                            {isHostCommand && item.timeout_sec ? ` · timeout: ${item.timeout_sec}s` : ''}
                                        </div>
                                    </div>
                                </div>

                                <div className="mt-3 grid gap-3 md:grid-cols-2">
                                    <div className="min-w-0">
							<div className="text-[11px] font-semibold text-muted-foreground mb-1">{isAbandonIdea ? '已尝试与失败证据' : '申请理由'}</div>
                                        <div className="rounded-md border border-border bg-background/50 px-3 py-2 text-sm break-words">{item.reason || '-'}</div>
                                    </div>
                                    <div className="min-w-0">
							<div className="text-[11px] font-semibold text-muted-foreground mb-1">{isHostCommand ? '命令 / 操作' : isAbandonIdea ? '拟放弃思路' : '申请操作'}</div>
	                                        <pre className="aix-scroll-code rounded-md border border-border bg-background/60 px-3 py-2 text-xs">{actionText}</pre>
                                    </div>
						{!isHostCommand && (item.impact || item.safer_alternative) ? (
                                        <div className="min-w-0 md:col-span-2">
								<div className="text-[11px] font-semibold text-muted-foreground mb-1">{isAbandonIdea ? '放弃影响与后续方向' : '风险与替代方案'}</div>
                                            <div className="rounded-md border border-border bg-background/50 px-3 py-2 text-sm break-words space-y-1">
									{item.impact ? <div>{isAbandonIdea ? '可能遗漏：' : '潜在影响：'}{item.impact}</div> : null}
									{item.safer_alternative ? <div>{isAbandonIdea ? '后续方向：' : '替代方案：'}{item.safer_alternative}</div> : null}
                                            </div>
                                        </div>
                                    ) : null}
                                </div>

                                {isPending ? (
                                    <div className="mt-3">
						<div className="text-[11px] font-semibold text-muted-foreground mb-1">{isAbandonIdea ? '引导备注（可选）' : '审核备注（可选）'}</div>
						<Textarea
							className="min-h-[84px]"
							placeholder={isAbandonIdea ? '例如：继续看 /api/export 的 SSRF 参数；换 header 绕过；可以放弃并转向鉴权链路等' : '例如：允许本次操作；仅限当前目标；拒绝原因等'}
                                            value={hostCommandDecisionNotes[approvalId] || ''}
                                            onChange={(e) => setHostCommandDecisionNotes(prev => ({ ...prev, [approvalId]: e.target.value }))}
                                            disabled={isActing}
                                        />
                                        <div className="mt-2 flex flex-wrap gap-2">
						<Button size="sm" variant="secondary" disabled={isActing} onClick={() => decideHostCommandApproval(approvalId, true)}>
							{isActing ? '处理中…' : isAbandonIdea ? '同意放弃' : '批准执行'}
						</Button>
						<Button size="sm" variant={isAbandonIdea ? 'default' : 'destructive'} disabled={isActing} onClick={() => decideHostCommandApproval(approvalId, false)}>
							{isActing ? '处理中…' : isAbandonIdea ? '继续该思路' : '拒绝执行'}
						</Button>
                                        </div>
                                    </div>
                                ) : null}

                                {!isPending && (item.decision_note || item.reviewer || item.error || item.output_preview) ? (
                                    <div className="mt-3 grid gap-3 md:grid-cols-2">
                                        <div className="min-w-0">
                                            <div className="text-[11px] font-semibold text-muted-foreground mb-1">审核结果</div>
                                            <div className="rounded-md border border-border bg-background/50 px-3 py-2 text-xs break-words">
                                                <div>{item.decision_at ? `时间：${item.decision_at}` : '时间：-'}</div>
                                                <div>{item.reviewer ? `审核人：${item.reviewer}` : '审核人：用户'}</div>
                                                <div>{item.decision_note ? `备注：${item.decision_note}` : '备注：-'}</div>
                                            </div>
                                        </div>
                                        {isHostCommand ? (
                                            <div className="min-w-0">
                                                <div className="text-[11px] font-semibold text-muted-foreground mb-1">执行反馈</div>
                                                <div className="rounded-md border border-border bg-background/50 px-3 py-2 text-xs break-words space-y-1">
                                                    {item.error ? <div className="text-destructive break-all">{item.error}</div> : null}
                                                    {item.output_preview ? <pre className="aix-scroll-code aix-scroll-code--lg">{item.output_preview}{item.output_truncated ? '\n...[truncated]' : ''}</pre> : null}
                                                    {!item.error && !item.output_preview ? <div className="text-muted-foreground">暂无输出</div> : null}
                                                </div>
                                            </div>
                                        ) : null}
                                    </div>
                                ) : null}
                            </div>
                        );
                    })}

                    {activeHostCommandApprovals.length === 0 ? (
                        <div className="rounded-lg border border-dashed border-border bg-background/30 px-3 py-6 text-sm text-muted-foreground text-center">当前没有待审核或执行中的决策事项。</div>
                    ) : null}

                    {recentResolvedHostCommandApprovals.length > 0 ? (
                        <div className="rounded-lg border border-border bg-background/20 p-3">
                            <div className="flex items-center justify-between gap-2">
                                <div className="flex items-center gap-2">
                                    <div className="text-xs font-semibold text-muted-foreground">历史决策</div>
                                    <Badge variant="secondary">{recentResolvedHostCommandApprovals.length}</Badge>
                                </div>
                                <Button size="sm" variant="ghost" className="h-7 px-2 text-xs" onClick={() => setRecentDecisionsExpanded(v => !v)}>
                                    {recentDecisionsExpanded ? '收起' : '展开'}
                                </Button>
                            </div>
                            {recentDecisionsExpanded ? (
                                <div className="mt-2 space-y-2">
                                    {recentResolvedHostCommandApprovals.map((item) => (
                                        <div key={`host-approval-recent-${item.id}`} className="rounded-md border border-border bg-background/40 px-3 py-2">
                                            <div className="flex flex-wrap items-center gap-2 text-xs">
                                                <span className="font-medium">{item.persona_name || getRoleLabel(item.agent_tool_name || '') || '未知请求者'}</span>
                                                <Badge variant={badgeVariantForHostApprovalStatus(item.status)}>{item.status || 'unknown'}</Badge>
                                                {item.group_name ? <span className="text-muted-foreground">{item.group_name}</span> : null}
                                                {item.updated_at ? <span className="text-muted-foreground">{item.updated_at}</span> : null}
                                            </div>
                                            <div className="mt-1 text-xs text-muted-foreground break-all">{item.action || item.command || '-'}</div>
                                        </div>
                                    ))}
                                </div>
                            ) : (
                                <div className="mt-2 text-xs text-muted-foreground">历史决策默认已隐藏，点击“展开”查看。</div>
                            )}
                        </div>
                    ) : null}
                </div>
                ) : (
                    <div className="mt-3 text-xs text-muted-foreground">决策列表默认已隐藏，点击“展开”查看待审、活跃和历史决策。</div>
                )}
            </div>

            <div className="mt-4 rounded-xl border border-sky-500/30 bg-sky-500/5 p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                        <div className="text-sm font-semibold">后台进程</div>
                        <div className="mt-1 text-xs text-muted-foreground">这里显示当前实验室里还没结束的 sandbox / 宿主机后台进程。你可以直接终止它们，数字人后续读取时会看到是用户手动 kill 的。</div>
                    </div>
                    <Badge variant={activeBackgroundProcesses.length > 0 ? 'warning' : 'secondary'}>运行中 {activeBackgroundProcesses.length}</Badge>
                </div>

                <div className="mt-3 space-y-3">
                    {activeBackgroundProcesses.map((item) => {
                        const sessionId = String(item.session_id || '');
                        const isActing = !!backgroundKillLoading[sessionId];
                        return (
                            <div key={`bg-process-${sessionId}`} className="rounded-lg border border-border bg-background/40 p-3">
                                <div className="flex flex-wrap items-start justify-between gap-3">
                                    <div className="min-w-0 flex-1">
                                        <div className="flex flex-wrap items-center gap-2">
                                            <div className="font-medium text-sm break-all">{item.persona_name || getRoleLabel(item.agent_tool_name || item.agent_name || '') || '未知数字人'}</div>
                                            {item.group_name ? <Badge variant="secondary">{item.group_name}</Badge> : null}
                                            {item.source ? <Badge variant="secondary">{item.source}</Badge> : null}
                                            <Badge variant={badgeVariantForBackgroundStatus(item.status)}>{item.status || 'unknown'}</Badge>
                                        </div>
                                        <div className="mt-1 text-xs text-muted-foreground break-words">
                                            {item.started_at ? `启动 ${item.started_at}` : '刚刚启动'}
                                            {item.pid ? ` · PID ${item.pid}` : ''}
                                            {typeof item.running_for_sec === 'number' ? ` · 已运行 ${formatRuntimeSeconds(item.running_for_sec)}` : ''}
                                            {item.workdir ? ` · workdir: ${item.workdir}` : ''}
                                        </div>
                                    </div>
                                    <div className="flex flex-wrap gap-2">
                                        <Button size="sm" variant="destructive" disabled={isActing || !sessionId} onClick={() => killBackgroundProcess(sessionId)}>
                                            {isActing ? '终止中…' : 'Kill'}
                                        </Button>
                                    </div>
                                </div>

                                <div className="mt-3">
                                    <div className="text-[11px] font-semibold text-muted-foreground mb-1">命令</div>
                                    <pre className="aix-scroll-code rounded-md border border-border bg-background/60 px-3 py-2 text-xs">{item.command || '-'}</pre>
                                </div>

	                                {item.error || item.termination_reason ? (
	                                    <div className="mt-3 rounded-md border border-border bg-background/50 px-3 py-2 text-xs break-words space-y-1">
	                                        {item.error ? <div className="text-destructive break-all">{item.error}</div> : null}
	                                        {item.termination_reason ? <div>终止原因：{item.termination_reason}</div> : null}
	                                    </div>
	                                ) : null}
                            </div>
                        );
                    })}

                    {activeBackgroundProcesses.length === 0 ? (
                        <div className="rounded-lg border border-dashed border-border bg-background/30 px-3 py-6 text-sm text-muted-foreground text-center">当前没有未结束的后台进程。</div>
                    ) : null}
                </div>
            </div>

            <div className="status-wall mt-4">
                <div className="status-wall__hero">
                    <div className="min-w-0">
                        <div className="status-wall__eyebrow">STATUS COMMAND CENTER</div>
                        <div className="status-wall__title">状态大屏</div>
                        <div className="status-wall__subtitle">实时汇总上下文、研究组、Token 与运行中数字人活动；工具调用仅展示泛化状态。</div>
                    </div>
                    <div className="status-wall__live">
                        <span className="status-wall__pulse" />
                        {projectIsRunning ? '实时运行中' : (projectStatus || '待命')}
                    </div>
                </div>

                <div className="status-wall__metrics">
                    <div className="status-metric"><span>决策大脑上下文占用</span><strong>{dashboardContextPct ? `${dashboardContextPct.toFixed(1)}%` : '等待数据'}</strong><em>决策大脑上下文使用率</em></div>
                </div>

                <div className="status-wall__grid">
                    <div className="status-panel status-panel--wide">
                        <div className="status-panel__head"><span>运行中数字人可视化</span><Badge variant={dashboardActiveRuns.length ? 'warning' : 'secondary'}>{dashboardActiveRuns.length ? 'active' : 'idle'}</Badge></div>
                        <div className="status-agent-grid">
                            {dashboardActiveRuns.slice(0, 12).map(({ run, member, groupId, groupName }, idx) => {
                                const contextSize = Number(member?.context_size ?? run?.context_size ?? 0);
                                const maxContext = Number(member?.max_context ?? run?.max_context ?? 0);
                                const pct = maxContext > 0 ? Math.min(100, (contextSize / maxContext) * 100) : 0;
                                const barColor = pct > 85 ? '#ef4444' : pct > 60 ? '#f59e0b' : '#10b981';
                                return (
                                    <button key={`${groupId}-${String(run?.AgentID ?? idx)}`} type="button" className="status-agent-card" onClick={() => setActiveGroupId(groupId)}>
                                        <img src={getAvatarUrl(getRunAvatarFile(run))} alt={getRunPersonaName(run)} className="status-agent-card__avatar" />
                                        <div className="status-agent-card__body">
                                            <div className="status-agent-card__top"><strong>{getRunPersonaName(run)}</strong><span>{groupName}</span></div>
                                            <div className="status-agent-card__activity"><span className="status-wall__pulse" />{getRunActivity(run)}</div>
                                            <div className="status-agent-card__task">{shortText(run?.RUNTask || run?.RUNSummary || '等待下一步动作', 110)}</div>
                                            {maxContext > 0 ? <div className="status-agent-card__context-meta"><span>上下文</span><em>{contextSize.toLocaleString()} / {maxContext.toLocaleString()} tokens</em></div> : null}
                                            <div className="status-agent-card__bar"><i style={{ width: `${pct}%`, background: barColor }} /></div>
                                        </div>
                                    </button>
                                );
                            })}
                            {dashboardActiveRuns.length === 0 ? <div className="status-empty">当前没有需要展示的运行中数字人。</div> : null}
                        </div>
                    </div>

                    <div className="status-panel">
                        <div className="status-panel__head"><span>Token 用量</span><span>{formatTokens(tokenUsage.total_tokens || 0)}</span></div>
                        <div className="status-token-split">
                            <div><span>Prompt</span><strong>{formatTokens(tokenUsage.prompt_tokens || 0)}</strong></div>
                            <div><span>Completion</span><strong>{formatTokens(tokenUsage.completion_tokens || 0)}</strong></div>
                        </div>
                        <div className="status-context-list">
                            {topTokenAgents.map((agent, idx) => <div key={`${agent.label}-${idx}`}><span>{String(agent.label || '数字人')}</span><em>{formatTokens(Number(agent.total_tokens || 0))}</em></div>)}
                            {topTokenAgents.length === 0 ? <div className="status-empty status-empty--small">等待 token 统计</div> : null}
                        </div>
                    </div>

                    <div className="status-panel status-panel--wide">
                        <div className="status-panel__head"><span>研究组总览</span><Badge variant="secondary">{dashboardGroupDetails.length}</Badge></div>
                        <div className="status-group-grid">
                            {dashboardGroupDetails.map(({ group, gid, runs, memberCount }) => {
                                const activeCount = (runs || []).filter((run: any) => isRunActive(run)).length;
                                return (
                                    <button key={gid} type="button" className={`status-group-card ${gid === activeGroupId ? 'status-group-card--active' : ''}`} onClick={() => setActiveGroupId(gid)}>
                                        <div><strong>{String(group?.groupName ?? gid)}</strong><Badge variant={group?.isRunning ? 'warning' : group?.brainFinished ? 'default' : 'secondary'}>{String(group?.status ?? '待命')}</Badge></div>
                                        <p>{String(group?.description ?? getDecisionBrainName(group))}</p>
                                        <footer><span>成员 {memberCount}</span><span>运行 {activeCount}/{runs.length}</span></footer>
                                    </button>
                                );
                            })}
                        </div>
                    </div>
                </div>
            </div>

            <div className="mt-4 grid gap-4 min-w-0 aix-detail-grid">
                <div className="grid gap-4 min-w-0 overflow-hidden">
                    <div className="aix-list-section">
                        <div className="flex items-center justify-between">
                            <div className="text-sm font-semibold">实验室总览</div>
                            <Badge variant="secondary">协同信息</Badge>
                        </div>
                        <div className="mt-2 rounded-lg border border-border bg-background/20 p-3 space-y-3">
                            <div>
                                <div className="text-xs font-semibold text-muted-foreground mb-2">总脑状态</div>
                                {controlBrain ? (
                                    <div className="rounded-lg border border-border bg-background/10 px-3 py-3">
                                        <div className="flex items-center justify-between gap-2">
                                            <div>
                                                <div className="font-semibold text-sm">总脑</div>
                                                <div className="mt-1 text-xs text-muted-foreground">{String(controlBrain?.status ?? '待命')}</div>
                                            </div>
                                            <Badge variant="secondary">{String(controlBrain?.status ?? '待命')}</Badge>
                                        </div>
                                        {controlBrain?.last_dispatch ? (
                                            <div className="mt-2 text-xs text-muted-foreground break-words">最近调度：{String(controlBrain.last_dispatch)}</div>
                                        ) : null}
                                        {Array.isArray(controlBrain?.last_dispatch_groups) && controlBrain.last_dispatch_groups.length > 0 ? (
                                            <div className="mt-1 text-xs text-muted-foreground">下发到：{controlBrain.last_dispatch_groups.join('、')}</div>
                                        ) : null}
                                    </div>
                                ) : (
                                    <div className="text-sm text-muted-foreground">暂无总脑状态</div>
                                )}
                            </div>

                            <div>
                                <div className="text-xs font-semibold text-muted-foreground mb-2">项目速览</div>
                                <div className="rounded-lg border border-border bg-background/10 p-3">
                                    {String(controlBrain?.project_overview ?? get(sharedResources, ['projectOverview']) ?? '').trim() ? (
                                        <div className="prose prose-invert prose-sm max-w-none break-words [&>*:first-child]:mt-0 [&>*:last-child]:mb-0">
                                            <ReactMarkdown>{String(controlBrain?.project_overview ?? get(sharedResources, ['projectOverview']) ?? '')}</ReactMarkdown>
                                        </div>
                                    ) : (
                                        <div className="text-sm text-muted-foreground">暂无项目速览</div>
                                    )}
                                </div>
                            </div>

                            <div>
                                <div className="flex items-center justify-between gap-2">
                                    <div className="text-xs font-semibold text-muted-foreground">共享报告</div>
                                    <Badge variant="secondary">{sharedReportCatalog.length}</Badge>
                                </div>
                                <div className="mt-2 rounded-lg border border-border bg-background/10 p-3">
                                    <div className="flex flex-wrap gap-2">
                                        <Button
                                            size="sm"
                                            variant="secondary"
                                            disabled={!detailProject || sharedReportCatalog.length === 0 || reportSummaryGenerating}
                                            onClick={generateReportSummary}
                                        >
                                            {reportSummaryGenerating ? '生成中…' : '生成总结'}
                                        </Button>
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            disabled={!detailProject || sharedReportCatalog.length === 0}
                                            onClick={() => {
                                                if (!detailProject) return;
                                                const name = encodeURIComponent(detailProject);
				downloadFileWithAuth(`${baseURL}/projects/${name}/reports/downloadAll`, `${detailProjectName || detailProject}-report.zip`);
                                            }}
                                        >
                                            下载全部
                                        </Button>
                                    </div>
                                    <ScrollArea className="mt-2 h-[220px] rounded-lg border border-border bg-background/10">
                                        <div className="p-2 space-y-2">
                                            {sharedReportCatalog.map((report: any, idx: number) => {
                                                const reportId = String(report?.reportId ?? `report-${idx}`);
                                                const downloadId = String(report?.downloadId ?? reportId);
                                                const filename = String(report?.fileName ?? reportId);
                                                const summary = String(report?.summary ?? '').trim();
                                                const groupName = String(report?.groupName ?? '');
                                                const targetId = String(report?.targetId ?? '').trim();
                                                const createdAt = String(report?.createdAt ?? '').trim();
                                                return (
                                                    <div key={downloadId} className="rounded-lg border border-border bg-background/10 px-3 py-2">
                                                        <div className="flex items-center justify-between gap-2">
                                                            <div className="min-w-0">
                                                                <div className="font-semibold text-sm break-all">{filename}</div>
                                                                {summary ? <div className="mt-1 text-xs text-muted-foreground break-words">{summary}</div> : null}
                                                            </div>
                                                            <div className="flex shrink-0 items-center gap-2">
                                                                <Button size="sm" variant="secondary" onClick={async () => {
                                                                    if (!detailProject) return;
                                                                    try {
                                                                        const content = await fetchReportText(detailProject, downloadId);
                                                                        openReportPreview(reportId, filename, content, downloadId);
                                                                    } catch (e: any) {
                                                                        openDetail('json', 'Report Preview Error', { id: reportId, filename, error: String(e?.message ?? e ?? 'error') });
                                                                    }
                                                                }}>预览</Button>
                                                                <Button size="sm" variant="outline" onClick={() => {
                                                                    if (!detailProject) return;
                                                                    const name = encodeURIComponent(detailProject);
                                                                    const rid = encodeURIComponent(downloadId);
                                                                    downloadFileWithAuth(`${baseURL}/projects/${name}/reports/download/${rid}`, filename);
                                                                }}>下载</Button>
                                                            </div>
                                                        </div>
                                                        <div className="mt-1 text-xs text-muted-foreground break-words">
                                                            id：{reportId}{groupName ? ` · 来源：${groupName}` : ''}{targetId ? ` · 目标：${targetId}` : ''}{createdAt ? ` · 时间：${createdAt}` : ''}
                                                        </div>
                                                    </div>
                                                );
                                            })}
                                            {sharedReportCatalog.length === 0 ? <div className="text-sm text-muted-foreground p-2">暂无共享报告</div> : null}
                                        </div>
                                    </ScrollArea>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <div className="grid gap-4 min-w-0 overflow-hidden">
                    {!activeGroup ? (
                        <div className="rounded-lg border border-border bg-background/20 p-4 text-sm text-muted-foreground">请先从左侧选择一个研究组查看组内私有信息。</div>
                    ) : (
                        <>
                            <div className="rounded-lg border border-border bg-background/20 p-4">
                                <div className="flex items-center justify-between gap-3">
                                    <div className="min-w-0">
                                        <div className="text-lg font-semibold truncate">{String(activeGroup?.groupName ?? '-')} · 组内详情</div>
                                    </div>
                                    <div className="flex items-center gap-2 shrink-0">
                                        <Badge variant={activeGroup?.isRunning ? 'warning' : activeGroup?.brainFinished ? 'default' : 'secondary'}>
                                            {String(activeGroup?.status ?? '待命')}
                                        </Badge>
                                    </div>
                                </div>
                            </div>

                            {isWebActiveGroup ? (
                                <>
                                    <div className="aix-list-section">
                                        <div className="flex items-center justify-between">
                                            <div className="text-sm font-semibold">容器</div>
                                            <Badge>{activeGroupContainers.length}</Badge>
                                        </div>
                                        <ScrollArea className="mt-2 h-[220px] rounded-lg border border-border bg-background/20 min-w-0">
                                            <div className="p-2 space-y-2 min-w-0">
                                                {activeGroupContainers.map((c, idx) => (
                                                    <div
                                                        key={c?.containerId ?? c?.ContainerId ?? idx}
                                                        className="rounded-lg border border-border bg-background/10 px-3 py-2 hover:bg-muted/30 cursor-pointer min-w-0 overflow-hidden"
                                                        onClick={() => openDetail('container', String(c?.containerId ?? c?.ContainerId ?? 'container'), c)}
                                                    >
                                                        <div className="font-semibold text-sm truncate">{String(c?.image ?? c?.Image ?? '-')}</div>
                                                        <div className="mt-1 text-xs text-muted-foreground truncate">containerId：{String(c?.containerId ?? c?.ContainerId ?? '-')}</div>
                                                        <div className="mt-1 text-xs text-muted-foreground truncate">containerIP：{String(c?.containerIP ?? c?.ContainerIP ?? '-')}</div>
                                                        {(() => { const wp = c?.webPort ?? c?.WebPort; return Array.isArray(wp) && wp.length > 0 ? <div className="mt-1 text-xs text-muted-foreground truncate">webPort：{wp.join(', ')}</div> : null; })()}
                                                    </div>
                                                ))}
                                                {activeGroupContainers.length === 0 ? <div className="text-sm text-muted-foreground p-2">暂无容器</div> : null}
                                            </div>
                                        </ScrollArea>
                                    </div>

                                    <div className="aix-list-section">
                                        <div className="flex items-center justify-between">
                                            <div className="text-sm font-semibold">ExploitIdea 状态</div>
                                            <Badge>{activeGroupExploitIdeas.length}</Badge>
                                        </div>
                                        <ScrollArea className="mt-2 h-[220px] rounded-lg border border-border bg-background/20 min-w-0">
                                            <div className="p-2 space-y-2 min-w-0">
                                                {activeGroupExploitIdeas.map((e, idx) => (
                                                    <div
                                                        key={e?.exploitIdeaId ?? e?.ExploitIdeaId ?? idx}
                                                        className="rounded-lg border border-border bg-background/10 px-3 py-2 hover:bg-muted/30 cursor-pointer min-w-0 overflow-hidden"
                                                        onClick={() => openDetail('exploitIdea', String(e?.exploitIdeaId ?? e?.ExploitIdeaId ?? 'exploitIdea'), e)}
                                                    >
                                                        <div className="flex items-center justify-between gap-2 min-w-0">
                                                            <div className="font-semibold text-sm truncate min-w-0">{String(e?.exploitIdeaId ?? e?.ExploitIdeaId ?? 'exploitIdea')}</div>
                                                            <Badge className="shrink-0" variant={badgeVariantForExploitState(e?.state ?? e?.State)}>{String(e?.state ?? e?.State ?? '-')}</Badge>
                                                        </div>
                                                        {(e?.review_reason || e?.ReviewReason) && (String(e?.state ?? e?.State ?? '')).includes('审核失败') && (
                                                            <div className="mt-1 text-xs text-destructive break-words" style={{ overflowWrap: 'anywhere' }}>审核失败原因：{String(e?.review_reason ?? e?.ReviewReason)}</div>
                                                        )}
                                                    </div>
                                                ))}
                                                {activeGroupExploitIdeas.length === 0 ? <div className="text-sm text-muted-foreground p-2">暂无 exploitIdea</div> : null}
                                            </div>
                                        </ScrollArea>
                                    </div>

                                    <div className="aix-list-section">
                                        <div className="flex items-center justify-between">
                                            <div className="text-sm font-semibold">ExploitChain 状态</div>
                                            <Badge>{activeGroupExploitChains.length}</Badge>
                                        </div>
                                        <ScrollArea className="mt-2 h-[220px] rounded-lg border border-border bg-background/20 min-w-0">
                                            <div className="p-2 space-y-2 min-w-0">
                                                {activeGroupExploitChains.map((e, idx) => (
                                                    <div
                                                        key={e?.exploit_chain_id ?? e?.ExploitChainId ?? idx}
                                                        className="rounded-lg border border-border bg-background/10 px-3 py-2 hover:bg-muted/30 cursor-pointer min-w-0 overflow-hidden"
                                                        onClick={() => openDetail('exploitChain', String(e?.exploit_chain_id ?? e?.ExploitChainId ?? 'exploitChain'), e)}
                                                    >
                                                        <div className="flex items-center justify-between gap-2 min-w-0">
                                                            <div className="font-semibold text-sm truncate min-w-0">{String(e?.exploit_chain_id ?? e?.ExploitChainId ?? 'exploitChain')}</div>
                                                            <Badge className="shrink-0" variant={badgeVariantForExploitState(e?.state ?? e?.State)}>{String(e?.state ?? e?.State ?? '-')}</Badge>
                                                        </div>
                                                        {(e?.review_reason || e?.ReviewReason) && (String(e?.state ?? e?.State ?? '')).includes('审核失败') && (
                                                            <div className="mt-1 text-xs text-destructive break-words" style={{ overflowWrap: 'anywhere' }}>审核失败原因：{String(e?.review_reason ?? e?.ReviewReason)}</div>
                                                        )}
                                                    </div>
                                                ))}
                                                {activeGroupExploitChains.length === 0 ? <div className="text-sm text-muted-foreground p-2">暂无 exploitChain</div> : null}
                                            </div>
                                        </ScrollArea>
                                    </div>

                                    <div>
                                        <div className="flex items-center justify-between">
                                            <div className="text-sm font-semibold">环境信息</div>
                                            <Badge variant={activeGroupEnvInfo ? 'success' : 'warning'}>{activeGroupEnvInfo ? 'ready' : 'pending'}</Badge>
                                        </div>
                                        <div className="mt-2 rounded-lg border border-border bg-background/20 p-3">
                                            {activeGroupEnvInfo ? (
                                                <div style={{ paddingTop: 6 }}>
                                                    <div style={{ marginTop: 10 }}>
                                                        <div style={{ fontWeight: 800, letterSpacing: 0.2, marginBottom: 8 }}>基础信息</div>
                                                        <FieldRow label="containerId" value={get(activeGroupEnvInfo, ['containerId', 'ContainerId'])} />
                                                    </div>

                                                    <div style={{ marginTop: 14 }}>
                                                        <div style={{ fontWeight: 800, letterSpacing: 0.2, marginBottom: 8 }}>登录信息</div>
                                                        <FieldRow label="username" value={get(get(activeGroupEnvInfo, ['loginInfo']), ['username'])} />
                                                        <FieldRow label="password" value={get(get(activeGroupEnvInfo, ['loginInfo']), ['password'])} />
                                                        <FieldRow label="loginURL" value={get(get(activeGroupEnvInfo, ['loginInfo']), ['loginURL'])} />
                                                        <FieldRow label="credentials" value={get(get(activeGroupEnvInfo, ['loginInfo']), ['credentials'])} />
                                                    </div>

                                                    <div style={{ marginTop: 14 }}>
                                                        <div style={{ fontWeight: 800, letterSpacing: 0.2, marginBottom: 8 }}>数据库信息</div>
                                                        <FieldRow label="username" value={get(get(activeGroupEnvInfo, ['dbInfo']), ['username'])} />
                                                        <FieldRow label="password" value={get(get(activeGroupEnvInfo, ['dbInfo']), ['password'])} />
                                                        <FieldRow label="Host" value={get(get(activeGroupEnvInfo, ['dbInfo']), ['Host', 'host'])} />
                                                        <FieldRow label="Base" value={get(get(activeGroupEnvInfo, ['dbInfo']), ['Base', 'base'])} />
                                                    </div>

                                                    <div style={{ marginTop: 14 }}>
                                                        <div style={{ fontWeight: 800, letterSpacing: 0.2, marginBottom: 8 }}>路由示例</div>
                                                        {Array.isArray(get(activeGroupEnvInfo, ['routeInfo'])) && (get(activeGroupEnvInfo, ['routeInfo']) as any[]).length > 0 ? (
                                                            <ScrollArea className="h-[160px] rounded-lg border border-border bg-background/10">
                                                                <div className="p-2 space-y-2">
                                                                    {(get(activeGroupEnvInfo, ['routeInfo']) as any[]).slice(0, 50).map((r, idx) => (
                                                                        <div key={idx} className="text-xs text-foreground/90">{String(r)}</div>
                                                                    ))}
                                                                </div>
                                                            </ScrollArea>
                                                        ) : (
                                                            <div className="text-sm text-muted-foreground">暂无路由示例</div>
                                                        )}
                                                    </div>
                                                </div>
                                            ) : (
                                                <div className="text-sm text-muted-foreground">暂无环境信息（需该组完成环境搭建后才会生成）</div>
                                            )}
                                        </div>
                                    </div>
                                </>
                            ) : null}

                        </>
                    )}
                </div>
            </div>

            {/* Team Chat */}
            <div className="mt-4">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                        <div className="text-sm font-semibold">实验室聊天</div>
                        <button
                            className="text-xs px-2 py-0.5 rounded border border-border bg-muted/40 hover:bg-primary/20 text-muted-foreground hover:text-foreground transition-colors"
                            onClick={() => { setChatFullscreen(true); setTimeout(() => chatEndRefFull.current?.scrollIntoView({ behavior: 'auto' }), 50); }}
                            title="展开聊天"
                        >⤢ 展开</button>
                    </div>
                    <div className="flex items-center gap-1 flex-wrap">
                        {chatShortcutTargets.map((target) => (
                            <button
                                key={`${activeChatChannel.id}-${target.label}`}
                                className="text-xs px-2 py-0.5 rounded-full border border-border bg-muted/40 hover:bg-primary/20 text-muted-foreground hover:text-foreground transition-colors"
                                onClick={() => setChatInput(target.value)}
                                title={target.description}
                            >
                                {target.label}
                            </button>
                        ))}
                    </div>
                </div>
                <div className="mt-2 flex flex-wrap gap-2">
                    {chatChannels.map((channel) => (
                        <button
                            key={channel.id}
                            className={`px-3 py-1.5 text-xs rounded-md border transition-colors ${activeChatChannel.id === channel.id ? 'bg-primary text-primary-foreground border-primary' : 'bg-background border-border text-muted-foreground hover:text-foreground'}`}
                            onClick={() => setChatChannelId(channel.id)}
                            title={channel.subtitle}
                        >
                            {channel.name}
                        </button>
                    ))}
                </div>
                <div className="mt-2 wx-inline-chat flex flex-col overflow-hidden" style={{ height: 300 }}>
                    <ScrollArea className="flex-1 px-3 py-2.5">
                        <div className="space-y-3">
                            {visibleChatMessages.length === 0 ? (
                                <div className="text-[11px] text-white/20 py-6 text-center">当前频道暂无消息：{activeChatChannel.name}</div>
                            ) : null}
                            {visibleChatMessages.map((m, idx) => {
                                const avatarSrc = (() => {
                                    if (m.role === 'user') return null;
                                    const file = m.avatar_file && m.avatar_file !== '' ? m.avatar_file : 'system.png';
                                    try { return new URL(`./assets/avatar/${file}`, import.meta.url).toString(); }
                                    catch { return new URL(`./assets/avatar/system.png`, import.meta.url).toString(); }
                                })();
                                const isSelf = m.role === 'user';
                                return (
                                <div key={idx} className={`flex ${isSelf ? 'flex-row-reverse' : 'flex-row'} items-start gap-2`}>
                                    {!isSelf && avatarSrc ? (
                                        <img src={avatarSrc} alt={m.persona_name || ''} className="wx-avatar shrink-0" style={{ width: 30, height: 30 }} />
                                    ) : isSelf ? (
                                        <div className="wx-avatar shrink-0 bg-[#57a85c] flex items-center justify-center text-white text-[10px] font-bold" style={{ width: 30, height: 30 }}>我</div>
                                    ) : null}
                                    <div className={`max-w-[85%] sm:max-w-[75%] min-w-0`} style={{ wordBreak: 'break-word', overflowWrap: 'break-word' }}>
                                        {!isSelf && m.persona_name && (
                                            <div className="text-[10px] font-medium text-white/25 mb-0.5 ml-1 truncate">
                                                {m.persona_name}
                                                {m.sender_type === 'group_brain' && m.group_name ? ` · ${m.group_name}` : ''}
                                            </div>
                                        )}
                                        <div className={`wx-inline-bubble ${isSelf ? 'wx-inline-bubble--self' : 'wx-inline-bubble--other'}`}>
                                            <div className="prose prose-invert prose-sm max-w-none break-words [&>*:first-child]:mt-0 [&>*:last-child]:mb-0 [&_pre]:whitespace-pre-wrap [&_pre]:break-all [&_code]:break-all" style={{ wordBreak: 'break-all', overflowWrap: 'anywhere' }}><ReactMarkdown>{m.text}</ReactMarkdown></div>
                                        </div>
                                        <div className={`text-[9px] mt-0.5 ${isSelf ? 'text-right mr-1' : 'ml-1'} text-white/15`}>{m.ts}</div>
                                    </div>
                                </div>
                                );
                            })}
                            <div ref={chatEndRef} />
                        </div>
                    </ScrollArea>
                    <div className="wx-input-bar px-3 py-2 flex items-center gap-2">
                        <Input
                            className="wx-input flex-1 text-[13px] h-9"
                            placeholder={activeChatChannel.id === 'decision_makers' ? '输入消息… 默认发给总脑，可 @某组决策大脑' : `输入消息到 ${activeChatChannel.name}…`}
                            value={chatInput}
                            onChange={e => setChatInput(e.target.value)}
                            onKeyDown={e => {
                                if (e.key === 'Enter' && !e.shiftKey) {
                                    e.preventDefault();
                                    sendChat();
                                }
                            }}
                            disabled={chatSending}
                        />
                        <Button size="sm" disabled={chatSending || !chatInput.trim()} onClick={sendChat} className="wx-send-btn h-9 px-4 text-[13px]">
                            {chatSending ? '…' : '发送'}
                        </Button>
                    </div>
                </div>
            </div>

        </CardContent>
    </Card>
                </>
                )}
            </div>
        </div>

        {/* Fullscreen Chat — rendered at top level, completely independent of the project page */}
        {chatFullscreen && (() => {
            const getAvatarUrl = (file: string) => {
                const f = file && file !== '-' ? file : 'system.png';
                try { return new URL(`./assets/avatar/${f}`, import.meta.url).toString(); }
                catch { return new URL(`./assets/avatar/system.png`, import.meta.url).toString(); }
            };
            const profileCardContent = (p: { name: string; role: string; state: string }, avatarUrl: string) => (
                <div className="rounded-xl border border-border bg-card shadow-lg p-3 w-[220px] max-w-[90vw] text-card-foreground">
                    <div className="flex items-center gap-2.5 mb-2">
                        <img src={avatarUrl} alt={p.name} className="h-10 w-10 rounded-full border border-border object-cover shrink-0" />
                        <div className="min-w-0">
                            <div className="text-sm font-bold truncate">{p.name}</div>
                        </div>
                    </div>
                    <div className="space-y-1 text-[11px]">
                        <div className="flex justify-between"><span className="text-muted-foreground">角色</span><span className="text-right max-w-[140px] truncate">{p.role}</span></div>
                        <div className="flex justify-between"><span className="text-muted-foreground">状态</span><span>{p.state}</span></div>
                    </div>
                </div>
            );
            const fixedHoverHandler = (e: { currentTarget: HTMLDivElement }, show: boolean) => {
                const target = e.currentTarget;
                const card = target.querySelector('[data-profile-card]') as HTMLElement | null;
                if (!card) return;
                if (show) {
                    const rect = target.getBoundingClientRect();
                    card.style.position = 'fixed';
                    card.style.left = `${rect.right + 8}px`;
                    card.style.top = `${rect.top}px`;
                    card.style.display = 'block';
                } else {
                    card.style.display = 'none';
                }
            };
            return (
            <div className="fixed inset-0 z-[100] flex h-screen w-screen overflow-hidden wx-chat-bg" style={{ animation: 'fadeIn 0.15s ease-out' }}>
                {/* Left Sidebar — Contacts (WeChat style) */}
                <div className="aix-chat-sidebar w-[260px] shrink-0 border-r border-white/[0.04] flex flex-col wx-sidebar min-h-0">
                    <div className="px-4 py-3.5 border-b border-white/[0.04] shrink-0">
                        <div className="flex items-center justify-between">
                            <div className="text-[13px] font-bold tracking-wide text-white/80">聊天结构</div>
                            <span className="text-[11px] text-white/30 font-medium">{chatChannels.length} 个频道</span>
                        </div>
                    </div>
                    <ScrollArea className="flex-1 min-h-0">
                        <div className="p-2 space-y-2">
                            <div className="space-y-0.5">
                                {chatChannels.map((channel) => (
                                    <button
                                        key={channel.id}
                                        className={`wx-sidebar-item w-full flex items-center gap-3 px-3 py-2.5 text-left ${activeChatChannel.id === channel.id ? 'bg-white/[0.08]' : ''}`}
                                        onClick={() => setChatChannelId(channel.id)}
                                    >
                                        <div className="wx-avatar wx-avatar-sm shrink-0 bg-[#2a2d35] flex items-center justify-center text-[11px] font-bold text-white/50">
                                            {channel.id === 'decision_makers' ? '策' : '组'}
                                        </div>
                                        <div className="min-w-0 flex-1">
                                            <div className="text-[13px] font-medium truncate text-white/90">{channel.name}</div>
                                            <div className="text-[11px] text-white/30 mt-0.5 truncate">{channel.subtitle}</div>
                                        </div>
                                    </button>
                                ))}
                            </div>
                            <div className="border-t border-white/[0.04] my-1.5 mx-3" />
                            <div className="px-3 text-[11px] uppercase tracking-[0.2em] text-white/25">参与者</div>
                            {channelParticipants.map((p) => (
                                <button key={`${activeChatChannel.id}-${p.name}`} className="wx-sidebar-item w-full flex items-center gap-3 px-3 py-2.5 text-left" onClick={() => {
                                    setChatInput(p.name === '总脑' ? '' : `@${p.name} `);
                                }}>
                                    <div
                                        className="relative shrink-0"
                                        onMouseEnter={(e) => fixedHoverHandler(e, true)}
                                        onMouseLeave={(e) => fixedHoverHandler(e, false)}
                                    >
                                        <img src={getAvatarUrl(p.avatar)} alt={p.name} className="wx-avatar wx-avatar-sm" />
                                        {p && (
                                            <div data-profile-card="" className="z-[300] pointer-events-none" style={{ display: 'none' }}>
                                                {profileCardContent({ name: p.name, role: p.role, state: p.state }, getAvatarUrl(p.avatar))}
                                            </div>
                                        )}
                                    </div>
                                    <div className="min-w-0 flex-1">
                                        <div className="text-[13px] font-medium truncate text-white/90">{p.name}</div>
                                        <div className="text-[11px] text-white/30 mt-0.5 truncate">{p.role}</div>
                                    </div>
                                    <span className="h-2 w-2 rounded-full shrink-0 bg-[#57a85c]" />
                                </button>
                            ))}
                        </div>
                    </ScrollArea>
                    <div className="px-3 py-2.5 border-t border-white/[0.04] shrink-0">
                        <button className="w-full text-[12px] text-white/40 hover:text-white/70 transition-colors py-1.5" onClick={() => setChatFullscreen(false)}>← 返回实验室</button>
                    </div>
                </div>

                {/* Right Main — Chat Area (WeChat style) */}
                <div className="aix-chat-main flex-1 flex flex-col min-w-0 min-h-0">
                    <div className="px-4 sm:px-5 py-3 border-b border-white/[0.04] shrink-0 flex items-center justify-between gap-2" style={{ background: '#1e2028' }}>
                        <div className="flex items-center gap-2 sm:gap-3 min-w-0">
                            <div className="text-[14px] sm:text-[15px] font-bold text-white/90 whitespace-nowrap">{activeChatChannel.name}</div>
                            <span className="text-[11px] text-white/25 font-medium">{visibleChatMessages.length} 条消息</span>
                        </div>
                        <div className="text-[11px] text-white/25 hidden sm:block">{activeChatChannel.subtitle} · Esc 返回</div>
                        <button className="sm:hidden text-[12px] text-white/40 hover:text-white/70 shrink-0" onClick={() => setChatFullscreen(false)}>← 返回</button>
                    </div>
                    <ScrollArea className="flex-1 min-h-0">
                        <div className="px-4 sm:px-8 py-5 space-y-4 max-w-4xl mx-auto">
                            {visibleChatMessages.length === 0 ? (
                                <div className="flex flex-col items-center justify-center py-24 text-white/20">
                                    <div className="text-5xl mb-4 opacity-40">💬</div>
                                    <div className="text-sm font-medium">暂无消息</div>
                                    <div className="text-xs mt-1.5 text-white/15">从左侧选择频道开始对话</div>
                                </div>
                            ) : null}
                            {visibleChatMessages.map((m, idx) => {
                                const avatarSrc = (() => {
                                    if (m.role === 'user') return null;
                                    const file = m.avatar_file && m.avatar_file !== '' ? m.avatar_file : 'system.png';
                                    try { return new URL(`./assets/avatar/${file}`, import.meta.url).toString(); }
                                    catch { return new URL(`./assets/avatar/system.png`, import.meta.url).toString(); }
                                })();
                                const isSelf = m.role === 'user';
                                return (
                                    <div key={idx} className={`flex ${isSelf ? 'flex-row-reverse' : 'flex-row'} items-start gap-2.5`}>
                                        {!isSelf && avatarSrc ? (
                                            <div
                                                className="relative shrink-0"
                                                onMouseEnter={(e) => fixedHoverHandler(e, true)}
                                                onMouseLeave={(e) => fixedHoverHandler(e, false)}
                                            >
                                                <img src={avatarSrc} alt={m.persona_name || ''} className="wx-avatar wx-avatar-md cursor-pointer" />
                                                {m.persona_name && (
                                                    <div data-profile-card="" className="z-[300] pointer-events-none" style={{ display: 'none' }}>
                                                        {profileCardContent({ name: m.persona_name, role: m.sender_type === 'group_brain' ? '组决策大脑' : (m.group_name || '研究成员'), state: m.group_name || '' }, avatarSrc)}
                                                    </div>
                                                )}
                                            </div>
                                        ) : isSelf ? (
                                            <div className="wx-avatar wx-avatar-md shrink-0 bg-[#57a85c] flex items-center justify-center text-white text-xs font-bold">我</div>
                                        ) : null}
                                        <div className={`max-w-[80%] sm:max-w-[60%] min-w-0`} style={{ wordBreak: 'break-word', overflowWrap: 'break-word' }}>
                                            {!isSelf && m.persona_name && (
                                                <div className={`text-[11px] font-medium text-white/30 mb-1 ${isSelf ? 'text-right mr-2' : 'ml-2'}`}>
                                                    {m.persona_name}
                                                    {m.sender_type === 'group_brain' && m.group_name ? ` · ${m.group_name}` : ''}
                                                </div>
                                            )}
                                            <div className={`wx-bubble ${isSelf ? 'wx-bubble--self' : 'wx-bubble--other'}`}>
                                                <div className="prose prose-invert prose-sm max-w-none break-words [&>*:first-child]:mt-0 [&>*:last-child]:mb-0 [&_pre]:whitespace-pre-wrap [&_pre]:break-all [&_code]:break-all" style={{ wordBreak: 'break-all', overflowWrap: 'anywhere' }}><ReactMarkdown>{m.text}</ReactMarkdown></div>
                                            </div>
                                            <div className={`text-[10px] mt-1 ${isSelf ? 'text-right mr-2' : 'ml-2'} text-white/20`}>{m.ts}</div>
                                        </div>
                                    </div>
                                );
                            })}
                            <div ref={chatEndRefFull} />
                        </div>
                    </ScrollArea>
                    <div className="wx-input-bar px-4 sm:px-5 py-3 shrink-0">
                        <div className="max-w-4xl mx-auto flex items-center gap-2.5">
                            <Input
                                className="wx-input flex-1 h-10"
                                placeholder={activeChatChannel.id === 'decision_makers' ? '输入消息… 默认发给总脑，可 @某组决策大脑' : `输入消息到 ${activeChatChannel.name}…`}
                                value={chatInput}
                                onChange={e => setChatInput(e.target.value)}
                                onKeyDown={e => {
                                    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendChat(); }
                                    if (e.key === 'Escape') { setChatFullscreen(false); }
                                }}
                                disabled={chatSending}
                                autoFocus
                            />
                            <Button disabled={chatSending || !chatInput.trim()} onClick={sendChat} className="wx-send-btn h-10 px-6">
                                {chatSending ? '发送中…' : '发送'}
                            </Button>
                        </div>
                    </div>
                </div>
            </div>
            );
        })()}
    </>
    );

    async function sendChat() {
        const msg = chatInput.trim();
        if (!msg || !detailProject || chatSending) return;
        const channelId = activeChatChannel.id;
        const channelName = activeChatChannel.name;
        setChatInput('');
        setChatSending(true);
        setTimeout(() => {
            chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
            chatEndRefFull.current?.scrollIntoView({ behavior: 'smooth' });
        }, 30);
        try {
            const name = encodeURIComponent(detailProject);
            await apiPost<string>(`/projects/${name}/chat`, { message: msg, channel_id: channelId });
        } catch (e: any) {
            setChatInput((current) => current || msg);
            setChatMessages(prev => appendUniqueChatMessage(prev, {
                role: 'system',
                text: `Error: ${String(e?.message ?? e)}`,
                ts: new Date().toLocaleTimeString(),
                channel_id: channelId,
                channel_name: channelName,
                sender_type: 'system',
            }));
        } finally {
            setChatSending(false);
            setTimeout(() => {
                chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
                chatEndRefFull.current?.scrollIntoView({ behavior: 'smooth' });
            }, 100);
        }
    }
}

export default App;
