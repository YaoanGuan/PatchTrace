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
 * 当前全部委托给内存 mock；接真实 Go 后端时只替换本文件实现，页面与 store 无需改动。
 */

export async function getMe(): Promise<GetMeResponse> {
  return mockApi.getMe();
}

export async function getTasks(query?: GetTasksQuery): Promise<GetTasksResponse> {
  return mockApi.getTasks(query);
}

export async function getTask(taskId: string): Promise<TaskDetail> {
  return mockApi.getTask(taskId);
}

export async function publishTask(input: PublishTaskInput): Promise<TaskDetail> {
  return mockApi.publishTask(input);
}

export async function acceptTask(taskId: string): Promise<AcceptTaskResponse> {
  return mockApi.acceptTask(taskId);
}

export async function getWallet(): Promise<GetWalletResponse> {
  return mockApi.getWallet();
}

export async function getTrace(taskId: string): Promise<GetTraceResponse> {
  return mockApi.getTrace(taskId);
}

export async function getReputation(userId: string): Promise<GetReputationResponse> {
  return mockApi.getReputation(userId);
}

export async function submitRating(input: SubmitRatingInput): Promise<SubmitRatingResponse> {
  return mockApi.submitRating(input);
}

export async function registerChannel(
  input: RegisterChannelInput,
): Promise<RegisterChannelResponse> {
  return mockApi.registerChannel(input);
}

export async function signContract(input: SignContractInput): Promise<SignContractResponse> {
  return mockApi.signContract(input);
}
