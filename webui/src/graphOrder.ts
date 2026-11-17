export type OrderedProject = {
    id: string;
    created_at?: string;
};

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
