import type {
  AcceptTaskResponse,
  GetMeResponse,
  GetReputationResponse,
  GetTasksQuery,
  GetTasksResponse,
  GetTraceResponse,
  GetWalletResponse,
  PublishTaskInput,
  RegisterChannelInput,
  RegisterChannelResponse,
  SignContractInput,
  SignContractResponse,
  SubmitRatingInput,
  SubmitRatingResponse,
} from '../../types/api';
import type { TaskDetail } from '../../types/domain';
import { mockApi } from './mock';

/**
 * API 层唯一出口。
 *
 * - 默认（`VITE_API_MODE !== 'http'`）走内存 mock，行为与之前完全一致。
 * - `VITE_API_MODE=http` 时走真实 Go 服务：基址 `VITE_API_BASE_URL`（默认同源，
 *   开发/预览由 Vite proxy 把 `/api` 转发到 `127.0.0.1:8081`）。
 *
 * 两种模式下函数签名与返回 DTO 保持一致，页面与 store 无需感知切换。
 */

const useHttp = import.meta.env.VITE_API_MODE === 'http';
// 归一化末尾斜杠，避免基址以 `/` 结尾时拼出 `//api/...`。
const baseUrl = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/+$/, '');

/** 非 2xx 响应统一抛出，带 HTTP 状态码与后端错误文本。 */
export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  if (!headers.has('Accept')) {
    headers.set('Accept', 'application/json');
  }
  if (init?.body !== undefined && init.body !== null && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  const response = await fetch(`${baseUrl}${path}`, { ...init, headers });

  if (!response.ok) {
    let message = `Request failed with status ${response.status}`;
    try {
      const payload: unknown = await response.json();
      if (
        payload !== null &&
        typeof payload === 'object' &&
        'error' in payload &&
        typeof (payload as { error: unknown }).error === 'string' &&
        (payload as { error: string }).error.length > 0
      ) {
        message = (payload as { error: string }).error;
      }
    } catch {
      // 错误体不是 JSON（或为空）时保留状态码兜底文案。
    }
    throw new ApiError(message, response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  if (text.length === 0) {
    return undefined as T;
  }

  return JSON.parse(text) as T;
}

/** 查询参数统一拼装：`undefined`/`null`/空字符串不写入 URL。 */
function appendQueryParam(
  params: URLSearchParams,
  key: string,
  value: string | number | undefined | null,
): void {
  if (value === undefined || value === null) {
    return;
  }
  const text = String(value);
  if (text.trim().length === 0) {
    return;
  }
  params.set(key, text);
}

const jsonInit = (method: string, body: unknown): RequestInit => ({
  method,
  body: JSON.stringify(body),
});

export async function getMe(): Promise<GetMeResponse> {
  if (!useHttp) {
    return mockApi.getMe();
  }
  return request<GetMeResponse>('/api/me');
}

export async function getTasks(query?: GetTasksQuery): Promise<GetTasksResponse> {
  if (!useHttp) {
    return mockApi.getTasks(query);
  }

  const params = new URLSearchParams();
  appendQueryParam(params, 'status', query?.status);
  appendQueryParam(params, 'keyword', query?.keyword);
  appendQueryParam(params, 'rewardMin', query?.rewardMin);
  appendQueryParam(params, 'rewardMax', query?.rewardMax);
  appendQueryParam(params, 'page', query?.page);

  const search = params.toString();
  return request<GetTasksResponse>(search ? `/api/tasks?${search}` : '/api/tasks');
}

export async function getTask(taskId: string): Promise<TaskDetail> {
  if (!useHttp) {
    return mockApi.getTask(taskId);
  }
  return request<TaskDetail>(`/api/tasks/${encodeURIComponent(taskId)}`);
}

export async function publishTask(input: PublishTaskInput): Promise<TaskDetail> {
  if (!useHttp) {
    return mockApi.publishTask(input);
  }
  return request<TaskDetail>('/api/tasks', jsonInit('POST', input));
}

export async function acceptTask(taskId: string): Promise<AcceptTaskResponse> {
  if (!useHttp) {
    return mockApi.acceptTask(taskId);
  }
  return request<AcceptTaskResponse>(
    `/api/tasks/${encodeURIComponent(taskId)}/accept`,
    { method: 'POST' },
  );
}

export async function getWallet(): Promise<GetWalletResponse> {
  if (!useHttp) {
    return mockApi.getWallet();
  }
  return request<GetWalletResponse>('/api/wallet');
}

export async function getTrace(taskId: string): Promise<GetTraceResponse> {
  if (!useHttp) {
    return mockApi.getTrace(taskId);
  }
  return request<GetTraceResponse>(`/api/tasks/${encodeURIComponent(taskId)}/trace`);
}

export async function getReputation(userId: string): Promise<GetReputationResponse> {
  if (!useHttp) {
    return mockApi.getReputation(userId);
  }
  return request<GetReputationResponse>(`/api/reputation/${encodeURIComponent(userId)}`);
}

export async function submitRating(input: SubmitRatingInput): Promise<SubmitRatingResponse> {
  if (!useHttp) {
    return mockApi.submitRating(input);
  }
  return request<SubmitRatingResponse>('/api/ratings', jsonInit('POST', input));
}

export async function registerChannel(
  input: RegisterChannelInput,
): Promise<RegisterChannelResponse> {
  if (!useHttp) {
    return mockApi.registerChannel(input);
  }
  return request<RegisterChannelResponse>(
    `/api/auth/register/${encodeURIComponent(input.channel)}`,
    jsonInit('POST', input),
  );
}

export async function signContract(input: SignContractInput): Promise<SignContractResponse> {
  if (!useHttp) {
    return mockApi.signContract(input);
  }
  return request<SignContractResponse>(
    `/api/tasks/${encodeURIComponent(input.taskId)}/contracts/sign`,
    jsonInit('POST', input),
  );
}
