export type GraphReason = {
    worker?: string;
    trigger?: string;
    started_at?: string;
    last_heartbeat_at?: string;
};

export type GraphProject = {
    id: string;
    title?: string;
    origin?: string;
    goal?: string;
    status?: string;
    bootstrap_enabled?: boolean;
    created_at?: string;
    reason?: GraphReason | null;
};

export type OrderedProject = {
    id: string;
    created_at?: string;
};

function stringField(raw: Record<string, unknown>, key: string): string | undefined {
    const value = raw[key];
    return typeof value === 'string' ? value : undefined;
}

function normalizeReason(raw: unknown): GraphReason | null {
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return null;
    const record = raw as Record<string, unknown>;
    return {
        worker: stringField(record, 'worker'),
        trigger: stringField(record, 'trigger'),
        started_at: stringField(record, 'started_at'),
        last_heartbeat_at: stringField(record, 'last_heartbeat_at'),
    };
}

// normalizeProjects keeps only usable project objects. The API normally
// returns well-formed records, but one malformed element must not be able to
// blank the whole homepage during render or produce an undefined fetch key.
export function normalizeProjects(raw: unknown): GraphProject[] {
    if (!Array.isArray(raw)) return [];
    const projects: GraphProject[] = [];
    for (const item of raw) {
        if (!item || typeof item !== 'object' || Array.isArray(item)) continue;
        const record = item as Record<string, unknown>;
        const id = stringField(record, 'id')?.trim();
        if (!id) continue;
        projects.push({
            id,
            title: stringField(record, 'title'),
            origin: stringField(record, 'origin'),
            goal: stringField(record, 'goal'),
            status: stringField(record, 'status'),
            bootstrap_enabled: record.bootstrap_enabled === true,
            created_at: stringField(record, 'created_at'),
            reason: normalizeReason(record.reason),
        });
    }
    return projects;
}

function timestamp(project: OrderedProject): number | null {
    if (!project.created_at) return null;
    const parsed = Date.parse(project.created_at);
    return Number.isFinite(parsed) ? parsed : null;
}

// recentProjects returns a copy ordered newest-first. The graph API lists
// projects by ascending created_at, so callers that show "recent" projects must
// re-sort; selecting from the server order silently shows the oldest ones.
export function recentProjects<T extends OrderedProject>(projects: T[], limit: number): T[] {
    if (limit <= 0) return [];
    return [...projects]
        .sort((a, b) => {
            const ta = timestamp(a);
            const tb = timestamp(b);
            if (ta !== null && tb !== null) {
                if (ta !== tb) return tb - ta;
                return String(b.id).localeCompare(String(a.id));
            }
            if (ta !== null) return -1;
            if (tb !== null) return 1;
            return 0;
        })
        .slice(0, limit);
}
