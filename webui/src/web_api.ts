export async function GetAPIBaseURL(): Promise<string> {
    const origin = (typeof window !== 'undefined' && window.location?.origin) || '';
    if (origin && origin !== 'null') {
        try {
            const url = new URL(origin);
            const host = String(url.hostname || '').toLowerCase();
            const port = String(url.port || '');
            if ((host === 'localhost' || host === '127.0.0.1') && port && port !== '9999') {
                return 'http://127.0.0.1:9999';
            }
        } catch {
            // ignore malformed browser origin and fall through
        }
    }
    return '';
}

export async function GetBasicAuthUser(): Promise<string> {
    return '';
}

export async function GetBasicAuthPassword(): Promise<string> {
    return '';
}
