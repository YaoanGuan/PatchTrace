import type {
  AcceptTaskResponse,
  GetMeResponse,
  GetReputationResponse,
  GetTasksQuery,
  GetTasksResponse,
  GetTraceResponse,
  PublishTaskInput,
  RegisterChannelInput,
  RegisterChannelResponse,
  SignContractInput,
  SignContractResponse,
  SubmitRatingInput,
  SubmitRatingResponse,
} from '../../types/api';
import type {
  AiToolIntegration,
  ApiGatewayOverview,
  ApiKeyRecord,
  ComplianceFinding,
  CreateApiKeyInput,
  CreateApiKeyResponse,
  CrossValidation,
  ExpertOverview,
  ModelCatalogItem,
  PipelineStage,
  PipelineStageKey,
  Rating,
  RechargeInput,
  RiskAssessment,
  SubscribeVipInput,
  TaskDetail,
  TaskStatus,
  TaskSummary,
  TaskTrust,
  TraceNode,
  User,
  UserSummary,
  WalletOverview,
  WalletTransaction,
} from '../../types/domain';

/** mock 延迟 80–150ms，模拟真实网络往返 */
const latency = () =>
  new Promise<void>((resolve) => {
    setTimeout(resolve, 80 + Math.floor(Math.random() * 71));
  });

const clone = <T,>(value: T): T => structuredClone(value);

let hashCounter = 0;

const pseudoHash = (seed: string): string => {
  let hash = 0;
  for (let index = 0; index < seed.length; index += 1) {
    hash = (hash * 31 + seed.charCodeAt(index)) >>> 0;
  }
  hashCounter += 1;
  const head = hash.toString(16).padStart(8, '0');
  const tail = hashCounter.toString(16).padStart(8, '0');
  return `sha256:${head}${tail}`;
};

const me: User = {
  id: 'usr_001',
  displayName: '林知远',
  avatarUrl: '/avatars/u1.png',
  activeRole: 'COMMANDER',
  roles: ['EMPLOYER', 'COMMANDER'],
  verificationChannel: 'UNIVERSITY',
  reputation: {
    score: 4.8,
    completedTasks: 32,
    onTimeRate: 0.97,
    disputes: 1,
  },
  skills: [
    { key: 'INSTRUCTION_PRECISION', label: 'AI指令精准度', score: 88 },
    { key: 'TASK_CLOSURE', label: '任务闭环能力', score: 76 },
    { key: 'COST_EFFICIENCY', label: '成本控制', score: 81 },
    { key: 'RISK_CONTROL', label: '风险控制', score: 72 },
    { key: 'EVIDENCE_QUALITY', label: '证据质量', score: 84 },
  ],
  expertise: [
    {
      id: 'exp_001',
      kind: 'AGENT',
      title: '代码审计 Agent 编排',
      level: 'L3',
      usageCount: 42,
      updatedAt: '2026-11-01T09:20:00Z',
    },
    {
      id: 'exp_002',
      kind: 'DATA_ASSET',
      title: 'IoT 固件协议语料库',
      level: 'L2',
      usageCount: 18,
      updatedAt: '2026-10-22T14:05:00Z',
    },
    {
      id: 'exp_003',
      kind: 'PROMPT_TEMPLATE',
      title: '多智能体交叉复核提示词模板',
      level: 'L3',
      usageCount: 57,
      updatedAt: '2026-11-08T03:40:00Z',
    },
  ],
  badges: [
    {
      id: 'bdg_001',
      type: 'UNIVERSITY',
      label: '高校学生认证',
      issuer: 'PatchTrace × 某高校',
      verifiedAt: '2026-09-12T00:00:00Z',
      status: 'VERIFIED',
    },
    {
      id: 'bdg_002',
      type: 'TASK_CLEARANCE',
      label: '任务通关认证 · 安全审计',
      issuer: 'PatchTrace 平台',
      verifiedAt: '2026-10-30T00:00:00Z',
      status: 'VERIFIED',
    },
    {
      id: 'bdg_003',
      type: 'ENTERPRISE',
      label: '企业白名单认证',
      issuer: '某安全实验室',
      verifiedAt: '2026-11-02T00:00:00Z',
      status: 'PENDING',
    },
  ],
};

const userSummaries: Record<string, UserSummary> = {
  usr_001: { id: 'usr_001', displayName: '林知远', reputationScore: 4.8 },
  usr_002: { id: 'usr_002', displayName: '某安全实验室', reputationScore: 4.6 },
  usr_003: { id: 'usr_003', displayName: '星环数据', reputationScore: 4.9 },
  usr_004: { id: 'usr_004', displayName: '云栖智造', reputationScore: 4.4 },
  usr_005: { id: 'usr_005', displayName: '独立研究员陈默', reputationScore: 4.7 },
};

type MockTask = TaskDetail & { acceptCount: number; validationScore: number };

const stage = (
  key: PipelineStageKey,
  status: PipelineStage['status'],
  at?: string,
  by?: string,
  summary?: string,
): PipelineStage => ({ key, status, at, by, summary });

const tasks: MockTask[] = [
  {
    id: 'task_1001',
    title: '为一款 IoT 固件编写协议模糊测试 Agent',
    description:
      '针对某型工业网关固件，构建可复现的协议模糊测试 Agent，覆盖 MQTT / Modbus 入口，输出测试计划、语料种子与崩溃归因报告。',
    category: 'SECURITY',
    status: 'PUBLISHED',
    publisher: userSummaries.usr_002,
    assignee: null,
    reward: { amount: 1200, currency: 'CNY' },
    escrow: { state: 'FROZEN', amount: 1200, frozenAt: '2026-11-15T10:00:00Z' },
    pipeline: {
      currentStage: 'AI_GENERATION',
      stages: [
        stage('AI_GENERATION', 'PENDING'),
        stage('AI_REVIEW', 'PENDING'),
        stage('COMPLIANCE_SCAN', 'PENDING'),
        stage('SETTLEMENT', 'PENDING'),
      ],
    },
    requiredSkills: ['AGENT_ORCHESTRATION', 'FUZZING'],
    acceptanceCriteria: ['可复现脚本', '执行日志', '结论报告'],
    createdAt: '2026-11-15T10:00:00Z',
    deadline: '2026-11-30T12:00:00Z',
    acceptCount: 3,
    validationScore: 87,
  },
  {
    id: 'task_1002',
    title: '电商风控日志异常检测 Agent',
    description:
      '基于脱敏后的登录与支付日志，搭建异常检测 Agent，要求给出特征说明、阈值策略与误报分析，并保留可审计的模型调用链。',
    category: 'DATA',
    status: 'AI_VALIDATING',
    publisher: userSummaries.usr_003,
    assignee: userSummaries.usr_001,
    reward: { amount: 860, currency: 'CNY' },
    escrow: { state: 'FROZEN', amount: 860, frozenAt: '2026-11-16T02:00:00Z' },
    pipeline: {
      currentStage: 'AI_REVIEW',
      stages: [
        stage('AI_GENERATION', 'DONE', '2026-11-16T02:10:00Z', 'agent://commander-01', '生成 3 条候选路径'),
        stage('AI_REVIEW', 'RUNNING', '2026-11-16T02:20:00Z', undefined, '平台审核 AI 交叉复核'),
        stage('COMPLIANCE_SCAN', 'PENDING'),
        stage('SETTLEMENT', 'PENDING'),
      ],
    },
    requiredSkills: ['DATA_PIPELINE', 'ANOMALY_DETECTION'],
    acceptanceCriteria: ['特征说明文档', '阈值策略', '误报分析报告'],
    createdAt: '2026-11-14T07:30:00Z',
    deadline: '2026-11-22T12:00:00Z',
    acceptCount: 5,
    validationScore: 91,
  },
  {
    id: 'task_1003',
    title: '多租户 SaaS 权限边界自动化回归',
    description:
      '为多租户 SaaS 设计跨租户越权回归套件，覆盖角色继承、资源归属与 API 网关策略，要求产出可重复执行的测试脚本与失败用例归因。',
    category: 'DEVELOPMENT',
    status: 'IN_PROGRESS',
    publisher: userSummaries.usr_004,
    assignee: userSummaries.usr_005,
    reward: { amount: 2400, currency: 'CNY' },
    escrow: { state: 'FROZEN', amount: 2400, frozenAt: '2026-11-14T08:30:00Z' },
    pipeline: {
      currentStage: 'AI_GENERATION',
      stages: [
        stage('AI_GENERATION', 'RUNNING', '2026-11-15T01:10:00Z', 'agent://commander-03', '生成越权测试矩阵'),
        stage('AI_REVIEW', 'PENDING'),
        stage('COMPLIANCE_SCAN', 'PENDING'),
        stage('SETTLEMENT', 'PENDING'),
      ],
    },
    requiredSkills: ['TEST_AUTOMATION', 'ACCESS_CONTROL'],
    acceptanceCriteria: ['测试矩阵', '可重复脚本', '失败用例归因'],
    createdAt: '2026-11-13T03:00:00Z',
    deadline: '2026-11-27T12:00:00Z',
    acceptCount: 2,
    validationScore: 78,
  },
  {
    id: 'task_1004',
    title: 'Linux 内核提权 PoC 复现与缓解建议',
    description:
      '在隔离环境中复现指定内核提权漏洞，提交最小 PoC、触发条件、受影响版本区间与缓解配置建议，禁止触碰生产数据。',
    category: 'SECURITY',
    status: 'COMPLIANCE_SCAN',
    publisher: userSummaries.usr_002,
    assignee: userSummaries.usr_001,
    reward: { amount: 1500, currency: 'CNY' },
    escrow: { state: 'FROZEN', amount: 1500, frozenAt: '2026-11-13T06:00:00Z' },
    pipeline: {
      currentStage: 'COMPLIANCE_SCAN',
      stages: [
        stage('AI_GENERATION', 'DONE', '2026-11-13T07:20:00Z', 'agent://commander-01', '生成 PoC 与复现步骤'),
        stage('AI_REVIEW', 'DONE', '2026-11-14T09:40:00Z', 'agent://reviewer-02', '交叉复核通过'),
        stage('COMPLIANCE_SCAN', 'RUNNING', '2026-11-15T02:00:00Z', undefined, '合规扫描进行中'),
        stage('SETTLEMENT', 'PENDING'),
      ],
    },
    requiredSkills: ['KERNEL', 'EXPLOIT_REPRODUCTION'],
    acceptanceCriteria: ['最小 PoC', '受影响版本区间', '缓解建议'],
    createdAt: '2026-11-12T05:00:00Z',
    deadline: '2026-11-25T12:00:00Z',
    acceptCount: 6,
    validationScore: 94,
  },
  {
    id: 'task_1005',
    title: '公开漏洞公告语义抽取与归因',
    description:
      '从公开漏洞公告中抽取受影响产品、攻击向量与利用条件，构建可追溯的结构化数据集，并给出抽取准确率评估。',
    category: 'RESEARCH',
    status: 'SETTLING',
    publisher: userSummaries.usr_005,
    assignee: userSummaries.usr_001,
    reward: { amount: 600, currency: 'CNY' },
    escrow: { state: 'FROZEN', amount: 600, frozenAt: '2026-11-11T09:00:00Z' },
    pipeline: {
      currentStage: 'SETTLEMENT',
      stages: [
        stage('AI_GENERATION', 'DONE', '2026-11-11T10:10:00Z', 'agent://commander-01', '生成抽取模板'),
        stage('AI_REVIEW', 'DONE', '2026-11-12T04:30:00Z', 'agent://reviewer-01', '抽样复核通过'),
        stage('COMPLIANCE_SCAN', 'DONE', '2026-11-12T08:00:00Z', 'agent://compliance-01', '无敏感数据'),
        stage('SETTLEMENT', 'RUNNING', '2026-11-16T01:00:00Z', undefined, '托管资金结算中'),
      ],
    },
    requiredSkills: ['NLP_EXTRACTION', 'DATA_QUALITY'],
    acceptanceCriteria: ['结构化数据集', '准确率评估', '来源溯源清单'],
    createdAt: '2026-11-10T02:00:00Z',
    deadline: '2026-11-20T12:00:00Z',
    acceptCount: 4,
    validationScore: 89,
  },
  {
    id: 'task_1006',
    title: '供应链依赖 SBOM 差分审计',
    description:
      '对两个版本的 SBOM 做差分审计，识别新增高危依赖、许可证冲突与传递依赖风险，输出可执行的升级与隔离建议。',
    category: 'DATA',
    status: 'COMPLETED',
    publisher: userSummaries.usr_003,
    assignee: userSummaries.usr_005,
    reward: { amount: 980, currency: 'CNY' },
    escrow: { state: 'RELEASED', amount: 980, frozenAt: '2026-11-08T03:00:00Z' },
    pipeline: {
      currentStage: 'SETTLEMENT',
      stages: [
        stage('AI_GENERATION', 'DONE', '2026-11-08T04:20:00Z', 'agent://commander-05', '生成差分清单'),
        stage('AI_REVIEW', 'DONE', '2026-11-09T06:10:00Z', 'agent://reviewer-03', '交叉复核通过'),
        stage('COMPLIANCE_SCAN', 'DONE', '2026-11-09T09:00:00Z', 'agent://compliance-02', '许可证冲突已标注'),
        stage('SETTLEMENT', 'DONE', '2026-11-10T02:00:00Z', 'platform://escrow', '资金已释放'),
      ],
    },
    requiredSkills: ['SBOM', 'SUPPLY_CHAIN'],
    acceptanceCriteria: ['差分清单', '风险分级', '升级与隔离建议'],
    createdAt: '2026-11-07T02:00:00Z',
    deadline: '2026-11-18T12:00:00Z',
    acceptCount: 3,
    validationScore: 92,
  },
  {
    id: 'task_1007',
    title: '红队演练证据链复核',
    description:
      '对一次红队演练的完整证据链做独立复核，验证时间戳、哈希链与操作授权记录的一致性，输出争议点与仲裁建议。',
    category: 'SECURITY',
    status: 'DISPUTED',
    publisher: userSummaries.usr_004,
    assignee: userSummaries.usr_005,
    reward: { amount: 3200, currency: 'CNY' },
    escrow: { state: 'FROZEN', amount: 3200, frozenAt: '2026-11-05T02:00:00Z' },
    pipeline: {
      currentStage: 'AI_REVIEW',
      stages: [
        stage('AI_GENERATION', 'DONE', '2026-11-05T03:20:00Z', 'agent://commander-04', '生成复核路径'),
        stage('AI_REVIEW', 'FAILED', '2026-11-06T05:40:00Z', 'agent://reviewer-02', '交叉复核存在争议'),
        stage('COMPLIANCE_SCAN', 'PENDING'),
        stage('SETTLEMENT', 'PENDING'),
      ],
    },
    requiredSkills: ['FORENSICS', 'HASH_CHAIN'],
    acceptanceCriteria: ['复核报告', '争议点清单', '仲裁建议'],
    createdAt: '2026-11-04T01:00:00Z',
    deadline: '2026-11-19T12:00:00Z',
    acceptCount: 1,
    validationScore: 64,
  },
];

const toSummary = (task: MockTask): TaskSummary => ({
  id: task.id,
  title: task.title,
  category: task.category,
  status: task.status,
  reward: task.reward,
  deadline: task.deadline,
  publisher: task.publisher,
  requiredSkills: task.requiredSkills,
  acceptCount: task.acceptCount,
  validationScore: task.validationScore,
});

const traceNode = (node: Omit<TraceNode, 'evidence'> & { evidence: TraceNode['evidence'] }): TraceNode => node;

const traceTask1001: GetTraceResponse = {
  taskId: 'task_1001',
  headHash: 'sha256:6f7a8b9c0d1e2f3a',
  nodes: [
    traceNode({
      id: 'node_001',
      type: 'REQUIREMENT',
      actor: { kind: 'USER', id: 'usr_002', label: '某安全实验室' },
      at: '2026-11-15T10:00:00Z',
      summary: '发布任务并冻结赏金 ¥1200',
      evidence: {
        hash: 'sha256:1a2b3c4d5e6f7a8b',
        payloadRef: 'blob://task/1001/req',
        model: null,
        cost: 0,
      },
      prevHash: null,
      hash: 'sha256:1a2b3c4d5e6f7a8b',
    }),
    traceNode({
      id: 'node_002',
      type: 'ACCEPTANCE',
      actor: { kind: 'SYSTEM', id: 'platform://validator', label: '平台验收标准生成器' },
      at: '2026-11-15T10:02:00Z',
      summary: '固化 3 项验收标准并写入哈希链',
      evidence: {
        hash: 'sha256:2b3c4d5e6f7a8b9c',
        payloadRef: 'blob://task/1001/acceptance',
        model: null,
        cost: 0,
      },
      prevHash: 'sha256:1a2b3c4d5e6f7a8b',
      hash: 'sha256:2b3c4d5e6f7a8b9c',
    }),
    traceNode({
      id: 'node_003',
      type: 'MODEL_CALL',
      actor: { kind: 'AGENT', id: 'agent://commander-01', label: 'Claude Sonnet' },
      at: '2026-11-16T02:11:00Z',
      summary: '生成候选测试计划',
      evidence: {
        hash: 'sha256:3c4d5e6f7a8b9c0d',
        payloadRef: 'blob://trace/1001/step3',
        model: 'claude-sonnet-5-5',
        cost: 0.12,
      },
      prevHash: 'sha256:2b3c4d5e6f7a8b9c',
      hash: 'sha256:3c4d5e6f7a8b9c0d',
    }),
    traceNode({
      id: 'node_004',
      type: 'ACTION',
      actor: { kind: 'AGENT', id: 'agent://executor-mqtt', label: 'MQTT 模糊执行器' },
      at: '2026-11-16T02:24:00Z',
      summary: '在隔离沙箱执行 12 轮变异语料',
      evidence: {
        hash: 'sha256:4d5e6f7a8b9c0d1e',
        payloadRef: 'blob://trace/1001/step4',
        model: null,
        cost: 0.03,
      },
      prevHash: 'sha256:3c4d5e6f7a8b9c0d',
      hash: 'sha256:4d5e6f7a8b9c0d1e',
    }),
    traceNode({
      id: 'node_005',
      type: 'OUTPUT',
      actor: { kind: 'AGENT', id: 'agent://evidence-packager', label: '证据打包器' },
      at: '2026-11-16T02:31:00Z',
      summary: '产出可复现脚本、崩溃样本与执行日志',
      evidence: {
        hash: 'sha256:5e6f7a8b9c0d1e2f',
        payloadRef: 'blob://trace/1001/output',
        model: null,
        cost: 0,
      },
      prevHash: 'sha256:4d5e6f7a8b9c0d1e',
      hash: 'sha256:5e6f7a8b9c0d1e2f',
    }),
    traceNode({
      id: 'node_006',
      type: 'VALIDATION',
      actor: { kind: 'SYSTEM', id: 'platform://cross-check', label: '平台审核 AI' },
      at: '2026-11-16T02:40:00Z',
      summary: '交叉复核通过：证据链完整、哈希连续',
      evidence: {
        hash: 'sha256:6f7a8b9c0d1e2f3a',
        payloadRef: 'blob://trace/1001/validation',
        model: 'claude-sonnet-5-5',
        cost: 0.08,
      },
      prevHash: 'sha256:5e6f7a8b9c0d1e2f',
      hash: 'sha256:6f7a8b9c0d1e2f3a',
    }),
  ],
};

const traceTask1004: GetTraceResponse = {
  taskId: 'task_1004',
  headHash: 'sha256:9c0d1e2f3a4b5c6d',
  nodes: [
    traceNode({
      id: 'node_101',
      type: 'REQUIREMENT',
      actor: { kind: 'USER', id: 'usr_002', label: '某安全实验室' },
      at: '2026-11-12T05:00:00Z',
      summary: '发布内核提权 PoC 复现任务并冻结赏金',
      evidence: {
        hash: 'sha256:6c0d1e2f3a4b5c6d',
        payloadRef: 'blob://task/1004/req',
        model: null,
        cost: 0,
      },
      prevHash: null,
      hash: 'sha256:6c0d1e2f3a4b5c6d',
    }),
    traceNode({
      id: 'node_102',
      type: 'MODEL_CALL',
      actor: { kind: 'AGENT', id: 'agent://commander-01', label: 'Claude Sonnet' },
      at: '2026-11-13T07:20:00Z',
      summary: '生成 PoC 与复现步骤',
      evidence: {
        hash: 'sha256:7d1e2f3a4b5c6d7e',
        payloadRef: 'blob://trace/1004/step2',
        model: 'claude-sonnet-5-5',
        cost: 0.21,
      },
      prevHash: 'sha256:6c0d1e2f3a4b5c6d',
      hash: 'sha256:7d1e2f3a4b5c6d7e',
    }),
    traceNode({
      id: 'node_103',
      type: 'ACTION',
      actor: { kind: 'AGENT', id: 'agent://executor-kernel', label: '隔离内核沙箱' },
      at: '2026-11-13T08:05:00Z',
      summary: '在隔离环境复现提权并采集内核日志',
      evidence: {
        hash: 'sha256:8e2f3a4b5c6d7e8f',
        payloadRef: 'blob://trace/1004/step3',
        model: null,
        cost: 0.05,
      },
      prevHash: 'sha256:7d1e2f3a4b5c6d7e',
      hash: 'sha256:8e2f3a4b5c6d7e8f',
    }),
    traceNode({
      id: 'node_104',
      type: 'VALIDATION',
      actor: { kind: 'SYSTEM', id: 'platform://cross-check', label: '平台审核 AI' },
      at: '2026-11-14T09:40:00Z',
      summary: '交叉复核通过，进入合规扫描',
      evidence: {
        hash: 'sha256:9c0d1e2f3a4b5c6d',
        payloadRef: 'blob://trace/1004/validation',
        model: 'claude-sonnet-5-5',
        cost: 0.09,
      },
      prevHash: 'sha256:8e2f3a4b5c6d7e8f',
      hash: 'sha256:9c0d1e2f3a4b5c6d',
    }),
  ],
};

const traces: Record<string, GetTraceResponse> = {
  task_1001: traceTask1001,
  task_1004: traceTask1004,
};

const wallet: WalletOverview = {
  balance: { available: 3200, frozen: 1200, currency: 'CNY' },
  transactions: [
    {
      id: 'tx_001',
      type: 'ESCROW_FREEZE',
      amount: 1200,
      taskId: 'task_1001',
      at: '2026-11-15T10:00:00Z',
      status: 'SETTLED',
    },
    {
      id: 'tx_002',
      type: 'ESCROW_FREEZE',
      amount: 860,
      taskId: 'task_1002',
      at: '2026-11-16T02:00:00Z',
      status: 'SETTLED',
    },
    {
      id: 'tx_003',
      type: 'DEPOSIT',
      amount: 5000,
      at: '2026-11-10T08:00:00Z',
      status: 'SETTLED',
    },
    {
      id: 'tx_004',
      type: 'ESCROW_RELEASE',
      amount: 980,
      taskId: 'task_1006',
      at: '2026-11-10T02:00:00Z',
      status: 'SETTLED',
    },
    {
      id: 'tx_005',
      type: 'FEE',
      amount: 12,
      taskId: 'task_1006',
      at: '2026-11-10T02:00:00Z',
      status: 'SETTLED',
    },
    {
      id: 'tx_006',
      type: 'DEPOSIT',
      amount: 1500,
      at: '2026-11-08T06:30:00Z',
      status: 'SETTLED',
    },
  ] as WalletTransaction[],
  vip: {
    level: 'FREE',
    expiresAt: null,
    feeRate: 0.15,
  },
};

let nextWalletTransactionId = 7;

const makeWalletTransaction = (
  type: WalletTransaction['type'],
  amount: number,
): WalletTransaction => {
  const transaction: WalletTransaction = {
    id: `tx_${String(nextWalletTransactionId).padStart(3, '0')}`,
    type,
    amount,
    at: new Date().toISOString(),
    status: 'SETTLED',
  };
  nextWalletTransactionId += 1;
  return transaction;
};

/* ------------------------------------------------------------------ */
/* M2 AI 专家种子                                                      */
/* ------------------------------------------------------------------ */

const expertTools: AiToolIntegration[] = [
  {
    id: 'tool_grok_bot',
    name: 'Grok Bot',
    vendor: 'xAI',
    category: 'CLOUD_COMPUTER',
    protocol: 'MCP',
    grade: 'S',
    status: 'CONNECTED',
    roles: ['EMPLOYER', 'COMMANDER'],
    permissions: ['task:publish', 'task:execute', 'evidence:read'],
    compliance: ['PCPD', 'SOC2'],
  },
  {
    id: 'tool_chatgpt_dot',
    name: 'ChatGPT Dot',
    vendor: 'OpenAI',
    category: 'CLOUD_COMPUTER',
    protocol: 'Screen',
    grade: 'A',
    status: 'CONNECTED',
    roles: ['EMPLOYER'],
    permissions: ['task:publish', 'catalog:read'],
    compliance: ['PCPD'],
  },
  {
    id: 'tool_claude_code',
    name: 'Claude Code',
    vendor: 'Anthropic',
    category: 'AI_CLI',
    protocol: 'CLI',
    grade: 'S',
    status: 'CONNECTED',
    roles: ['COMMANDER'],
    permissions: ['task:execute', 'task:submit', 'evidence:write'],
    compliance: ['SOC2'],
  },
  {
    id: 'tool_codex_cli',
    name: 'Codex CLI',
    vendor: 'OpenAI',
    category: 'AI_CLI',
    protocol: 'CLI',
    grade: 'A',
    status: 'SANDBOX',
    roles: ['COMMANDER'],
    permissions: ['task:execute', 'task:submit'],
    compliance: ['PCPD'],
  },
  {
    id: 'tool_cursor',
    name: 'Cursor',
    vendor: 'Anysphere',
    category: 'AI_IDE',
    protocol: 'IDE Plugin',
    grade: 'B',
    status: 'CONNECTED',
    roles: ['COMMANDER'],
    permissions: ['task:execute', 'evidence:write'],
    compliance: ['SOC2'],
  },
  {
    id: 'tool_trae',
    name: 'Trae',
    vendor: 'ByteDance',
    category: 'AI_IDE',
    protocol: 'IDE Plugin',
    grade: 'B',
    status: 'PENDING',
    roles: ['COMMANDER'],
    permissions: ['task:execute'],
    compliance: ['PCPD'],
  },
  {
    id: 'tool_chatbox',
    name: 'Chatbox',
    vendor: 'Chatbox',
    category: 'CHAT_UI',
    protocol: 'Screen',
    grade: 'C',
    status: 'SANDBOX',
    roles: ['EMPLOYER', 'COMMANDER'],
    permissions: ['task:publish', 'task:execute'],
    compliance: ['PCPD'],
  },
];

const expertValidation: CrossValidation = {
  taskId: 'task_1001',
  generator: 'Claude',
  generatorModel: 'claude-opus-5-5',
  generatorVerdict: 'PASS',
  reviewer: 'Codex',
  reviewerModel: 'gpt-5-codex',
  reviewerVerdict: 'PASS',
  consensus: 'PASS',
  confidence: 0.94,
  cost: 12.6,
  elapsedMs: 8421,
  at: '2026-11-16T02:40:00Z',
};

const expertCompliance: ComplianceFinding[] = [
  {
    id: 'cmp_001',
    rule: 'PCPD-ART-13 个人数据跨境传输',
    severity: 'HIGH',
    status: 'BLOCK',
    detail: '任务输入包含境外传输字段，需先完成跨境评估并显式阻断。',
  },
  {
    id: 'cmp_002',
    rule: 'PCPD-ART-17 最小必要原则',
    severity: 'MEDIUM',
    status: 'WARN',
    detail: '采集字段超出验收所需范围，建议删减非必要样本。',
  },
  {
    id: 'cmp_003',
    rule: 'PCPD-ART-24 数据留存期限',
    severity: 'LOW',
    status: 'WARN',
    detail: '默认留存 90 天，请确认与委托方约定一致。',
  },
  {
    id: 'cmp_004',
    rule: 'PCPD-ART-09 安全审计日志',
    severity: 'LOW',
    status: 'PASS',
    detail: '审计日志、哈希链与操作授权记录完整。',
  },
];

const expertRisk: RiskAssessment = {
  score: 82,
  level: 'HIGH',
  threshold: 70,
  blocked: true,
  reasons: ['PCPD BLOCK 规则命中，已阻断结算', '任务涉及跨境数据传输路径，需补充合法依据'],
};

const expertOverview: ExpertOverview = {
  tools: expertTools,
  validation: expertValidation,
  compliance: expertCompliance,
  risk: expertRisk,
  pricing: [
    { service: '多 Agent 交叉验证', unit: 'CNY/次', price: 8 },
    { service: '合规扫描', unit: 'CNY/次', price: 5 },
    { service: '风险阻断', unit: 'CNY/次', price: 3 },
  ],
};

/* ------------------------------------------------------------------ */
/* 任务信任流水线（必须与任务状态一致）                                */
/* ------------------------------------------------------------------ */

const HIGH_RISK_TASK_IDS = new Set(['task_1003', 'task_1004', 'task_1007']);

const validationForStatus = (taskId: string, status: TaskStatus): CrossValidation => {
  const base: CrossValidation = {
    taskId,
    generator: 'Claude',
    generatorModel: 'claude-opus-5-5',
    generatorVerdict: 'PENDING',
    reviewer: 'Codex',
    reviewerModel: 'gpt-5-codex',
    reviewerVerdict: 'PENDING',
    consensus: 'PENDING',
    confidence: 0,
    cost: 0,
    elapsedMs: 0,
    at: '2026-11-16T02:40:00Z',
  };

  switch (status) {
    case 'SUBMITTED':
    case 'AI_VALIDATING':
      return { ...base, generatorVerdict: 'PASS', confidence: 0.6, cost: 6.2, elapsedMs: 3200 };
    case 'VALIDATION_FAILED':
      return {
        ...base,
        generatorVerdict: 'PASS',
        reviewerVerdict: 'FAIL',
        consensus: 'FAIL',
        confidence: 0.35,
        cost: 9.8,
        elapsedMs: 6100,
      };
    case 'COMPLIANCE_SCAN':
    case 'SETTLING':
    case 'COMPLETED':
      return {
        ...base,
        generatorVerdict: 'PASS',
        reviewerVerdict: 'PASS',
        consensus: 'PASS',
        confidence: 0.94,
        cost: 12.6,
        elapsedMs: 8421,
      };
    default:
      return base;
  }
};

const complianceForTask = (highRisk: boolean): ComplianceFinding[] => {
  const crossBorderStatus: ComplianceFinding['status'] = highRisk ? 'BLOCK' : 'WARN';
  return [
    {
      id: 'cmp_001',
      rule: 'PCPD-ART-13 个人数据跨境传输',
      severity: 'HIGH',
      status: crossBorderStatus,
      detail: '任务输入包含境外传输字段，需先完成跨境评估并显式阻断。',
    },
    {
      id: 'cmp_002',
      rule: 'PCPD-ART-17 最小必要原则',
      severity: 'MEDIUM',
      status: 'WARN',
      detail: '采集字段超出验收所需范围，建议删减非必要样本。',
    },
    {
      id: 'cmp_003',
      rule: 'PCPD-ART-24 数据留存期限',
      severity: 'LOW',
      status: 'WARN',
      detail: '默认留存 90 天，请确认与委托方约定一致。',
    },
    {
      id: 'cmp_004',
      rule: 'PCPD-ART-09 安全审计日志',
      severity: 'LOW',
      status: 'PASS',
      detail: '审计日志、哈希链与操作授权记录完整。',
    },
  ];
};

const riskForTask = (highRisk: boolean): RiskAssessment =>
  highRisk
    ? {
        score: 82,
        level: 'HIGH',
        threshold: 70,
        blocked: true,
        reasons: ['PCPD BLOCK 规则命中，已阻断结算', '任务涉及跨境数据传输路径，需补充合法依据'],
      }
    : {
        score: 58,
        level: 'MEDIUM',
        threshold: 70,
        blocked: false,
        reasons: ['任务涉及跨境数据传输路径，需补充合法依据', '沙箱内执行授权尚未完成二次复核'],
      };

const hasBlockFinding = (findings: ComplianceFinding[]): boolean =>
  findings.some((finding) => finding.status === 'BLOCK');

/* ------------------------------------------------------------------ */
/* API 中转站种子                                                      */
/* ------------------------------------------------------------------ */

const gatewayModels: ModelCatalogItem[] = [
  {
    id: 'deepseek-chat',
    provider: 'DeepSeek',
    name: 'DeepSeek V3.2',
    contextWindow: 131072,
    inputPricePerMTok: 1,
    outputPricePerMTok: 2,
    latencyMs: 680,
    status: 'ONLINE',
    capabilities: ['chat', 'code', 'tools'],
  },
  {
    id: 'qwen3-max',
    provider: 'Qwen',
    name: 'Qwen3 Max',
    contextWindow: 131072,
    inputPricePerMTok: 1.6,
    outputPricePerMTok: 6.4,
    latencyMs: 720,
    status: 'ONLINE',
    capabilities: ['chat', 'code', 'reasoning'],
  },
  {
    id: 'glm-4.6',
    provider: 'GLM',
    name: 'GLM-4.6',
    contextWindow: 128000,
    inputPricePerMTok: 2,
    outputPricePerMTok: 8,
    latencyMs: 760,
    status: 'ONLINE',
    capabilities: ['chat', 'tools'],
  },
  {
    id: 'kimi-k2',
    provider: 'Kimi',
    name: 'Kimi K2',
    contextWindow: 262144,
    inputPricePerMTok: 2.4,
    outputPricePerMTok: 10,
    latencyMs: 910,
    status: 'ONLINE',
    capabilities: ['chat', 'long-context'],
  },
  {
    id: 'minimax-m2',
    provider: 'MiniMax',
    name: 'MiniMax M2',
    contextWindow: 200000,
    inputPricePerMTok: 1.8,
    outputPricePerMTok: 7.2,
    latencyMs: 840,
    status: 'DEGRADED',
    capabilities: ['chat', 'code'],
  },
  {
    id: 'claude-opus-5-5',
    provider: 'Claude',
    name: 'Claude Opus 5.5',
    contextWindow: 200000,
    inputPricePerMTok: 30,
    outputPricePerMTok: 150,
    latencyMs: 1200,
    status: 'ONLINE',
    capabilities: ['chat', 'code', 'vision', 'tools'],
  },
  {
    id: 'gpt-5-codex',
    provider: 'GPT',
    name: 'GPT-5 Codex',
    contextWindow: 400000,
    inputPricePerMTok: 25,
    outputPricePerMTok: 100,
    latencyMs: 1150,
    status: 'ONLINE',
    capabilities: ['chat', 'code', 'tools'],
  },
  {
    id: 'gemini-3-pro',
    provider: 'Gemini',
    name: 'Gemini 3 Pro',
    contextWindow: 1000000,
    inputPricePerMTok: 12,
    outputPricePerMTok: 48,
    latencyMs: 980,
    status: 'ONLINE',
    capabilities: ['chat', 'vision', 'long-context'],
  },
];

const gateway: ApiGatewayOverview = {
  models: gatewayModels,
  keys: [
    {
      id: 'key_001',
      label: '默认开发密钥',
      maskedKey: 'sk-pt-****-7f3a',
      createdAt: '2026-11-01T08:00:00Z',
      lastUsedAt: '2026-11-17T01:20:00Z',
      scopes: ['models:read', 'chat:write'],
      status: 'ACTIVE',
    },
  ],
  usage: {
    period: '2026-11',
    tokensIn: 1_284_000,
    tokensOut: 496_000,
    requests: 3_912,
    spend: 86.4,
  },
};

let nextApiKeyId = 2;

const randomHex = (length: number): string => {
  let output = '';
  while (output.length < length) {
    output += Math.floor(Math.random() * 16).toString(16);
  }
  return output;
};

const ratingsForUsr001: Rating[] = [
  {
    id: 'rat_001',
    taskId: 'task_1006',
    direction: 'COMMANDER_TO_EMPLOYER',
    score: 5,
    tags: ['需求清晰', '结算及时'],
    comment: '验收标准明确，托管释放很快。',
    createdAt: '2026-11-10T03:10:00Z',
  },
  {
    id: 'rat_002',
    taskId: 'task_1004',
    direction: 'EMPLOYER_TO_COMMANDER',
    score: 4.8,
    tags: ['证据完整', '沟通顺畅'],
    comment: 'PoC 与复现步骤可执行，证据链完整。',
    createdAt: '2026-11-15T06:20:00Z',
  },
  {
    id: 'rat_003',
    taskId: 'task_1002',
    direction: 'EMPLOYER_TO_COMMANDER',
    score: 4.6,
    tags: ['交付准时', '文档清晰'],
    comment: '阈值策略解释充分，误报分析到位。',
    createdAt: '2026-11-16T02:50:00Z',
  },
];

const ratingsForUsr002: Rating[] = [
  {
    id: 'rat_101',
    taskId: 'task_1001',
    direction: 'COMMANDER_TO_EMPLOYER',
    score: 4.5,
    tags: ['授权完整', '沟通顺畅'],
    comment: '测试范围与授权边界清晰。',
    createdAt: '2026-11-15T12:00:00Z',
  },
];

const extraRatings: Rating[] = [];

let nextTaskId = 1008;
let nextRatingId = 4;

const findTask = (taskId: string): MockTask | undefined =>
  tasks.find((task) => task.id === taskId);

const defaultTrace = (taskId: string): GetTraceResponse => ({
  taskId,
  headHash: 'sha256:0000000000000000',
  nodes: [],
});

export const mockApi = {
  async getMe(): Promise<GetMeResponse> {
    await latency();
    return clone(me);
  },

  async getTasks(query: GetTasksQuery = {}): Promise<GetTasksResponse> {
    await latency();
    const page = query.page ?? 1;
    const pageSize = 20;
    let items = tasks.map(toSummary);

    if (query.status) {
      items = items.filter((task) => task.status === query.status);
    }

    if (query.keyword && query.keyword.trim().length > 0) {
      const keyword = query.keyword.trim().toLowerCase();
      items = items.filter((task) =>
        `${task.title} ${task.category} ${task.publisher.displayName}`
          .toLowerCase()
          .includes(keyword),
      );
    }

    if (typeof query.rewardMin === 'number') {
      items = items.filter((task) => task.reward.amount >= query.rewardMin!);
    }

    if (typeof query.rewardMax === 'number') {
      items = items.filter((task) => task.reward.amount <= query.rewardMax!);
    }

    const total = items.length;
    const start = (page - 1) * pageSize;

    return {
      items: items.slice(start, start + pageSize),
      total,
      page,
      pageSize,
    };
  },

  async getTask(taskId: string): Promise<TaskDetail> {
    await latency();
    const task = findTask(taskId);
    if (!task) {
      throw new Error(`Task ${taskId} not found`);
    }
    return clone(task);
  },

  async publishTask(input: PublishTaskInput): Promise<TaskDetail> {
    await latency();
    if (!input.title.trim()) {
      throw new Error('Task title is required');
    }
    if (input.reward.amount < 0) {
      throw new Error('Reward amount cannot be negative');
    }

    const now = new Date().toISOString();
    const id = `task_${nextTaskId}`;
    nextTaskId += 1;
    const status: TaskStatus = input.complianceAck ? 'PUBLISHED' : 'DRAFT';

    const created: MockTask = {
      id,
      title: input.title,
      description: input.description,
      category: input.category,
      status,
      publisher: {
        id: me.id,
        displayName: me.displayName,
        reputationScore: me.reputation.score,
      },
      assignee: null,
      reward: input.reward,
      escrow: {
        state: input.complianceAck && input.reward.amount > 0 ? 'FROZEN' : 'NONE',
        amount: input.reward.amount,
        frozenAt: input.complianceAck && input.reward.amount > 0 ? now : undefined,
      },
      pipeline: {
        currentStage: 'AI_GENERATION',
        stages: [
          stage('AI_GENERATION', 'PENDING'),
          stage('AI_REVIEW', 'PENDING'),
          stage('COMPLIANCE_SCAN', 'PENDING'),
          stage('SETTLEMENT', 'PENDING'),
        ],
      },
      requiredSkills: input.requiredSkills,
      acceptanceCriteria: input.acceptanceCriteria,
      createdAt: now,
      deadline: input.deadline,
      acceptCount: 0,
      validationScore: 0,
    };

    tasks.unshift(created);
    return clone(created);
  },

  async acceptTask(taskId: string): Promise<AcceptTaskResponse> {
    await latency();
    const task = findTask(taskId);
    if (!task) {
      throw new Error(`Task ${taskId} not found`);
    }
    if (task.status !== 'PUBLISHED') {
      throw new Error(`Task ${taskId} is not open for acceptance`);
    }

    const now = new Date();
    task.status = 'ACCEPTED';
    task.assignee = { ...userSummaries.usr_001 };
    task.acceptCount += 1;
    const leaseExpiresAt = new Date(now.getTime() + 2 * 60 * 60 * 1000).toISOString();

    return { taskId, status: 'ACCEPTED', leaseExpiresAt };
  },

  async getWallet(): Promise<WalletOverview> {
    await latency();
    return clone(wallet);
  },

  async rechargeWallet(input: RechargeInput): Promise<WalletOverview> {
    await latency();
    if (!Number.isFinite(input.amount) || input.amount <= 0) {
      throw new Error('充值金额必须大于 0');
    }
    wallet.balance.available += input.amount;
    wallet.transactions.unshift(makeWalletTransaction('DEPOSIT', input.amount));
    return clone(wallet);
  },

  async subscribeVip(input: SubscribeVipInput): Promise<WalletOverview> {
    await latency();
    const priceByLevel: Record<SubscribeVipInput['level'], number> = {
      PRO: 99,
      ENTERPRISE: 999,
    };
    const price = priceByLevel[input.level];
    if (wallet.balance.available < price) {
      throw new Error('余额不足，无法订阅 VIP');
    }

    wallet.balance.available -= price;
    wallet.transactions.unshift(makeWalletTransaction('FEE', price));

    const expiresAt = new Date();
    expiresAt.setFullYear(expiresAt.getFullYear() + 1);
    wallet.vip = {
      level: input.level,
      expiresAt: expiresAt.toISOString(),
      feeRate: input.level === 'PRO' ? 0.1 : 0.05,
    };

    return clone(wallet);
  },

  async getExpertOverview(): Promise<ExpertOverview> {
    await latency();
    return clone(expertOverview);
  },

  async getTaskTrust(taskId: string): Promise<TaskTrust> {
    await latency();
    const task = findTask(taskId);
    if (!task) {
      throw new Error('任务不存在');
    }
    const highRisk = HIGH_RISK_TASK_IDS.has(taskId);
    const compliance = complianceForTask(highRisk);
    const risk = riskForTask(highRisk);
    // 硬约束：blocked 必须与是否存在 BLOCK 命中完全一致。
    risk.blocked = hasBlockFinding(compliance);
    return clone({
      taskId,
      validation: validationForStatus(taskId, task.status),
      compliance,
      risk,
    });
  },

  async getApiGateway(): Promise<ApiGatewayOverview> {
    await latency();
    return clone(gateway);
  },

  async createApiKey(input: CreateApiKeyInput): Promise<CreateApiKeyResponse> {
    await latency();
    const hex = randomHex(8);
    const record: ApiKeyRecord = {
      id: `key_${String(nextApiKeyId).padStart(3, '0')}`,
      label: input.label,
      maskedKey: `sk-pt-****-${hex.slice(-4)}`,
      createdAt: new Date().toISOString(),
      lastUsedAt: null,
      scopes: [...input.scopes],
      status: 'ACTIVE',
    };
    nextApiKeyId += 1;
    gateway.keys.unshift(record);
    return { key: clone(record), plaintextKey: `sk-pt-${hex}` };
  },

  async revokeApiKey(keyId: string): Promise<ApiKeyRecord> {
    await latency();
    const record = gateway.keys.find((key) => key.id === keyId);
    if (!record) {
      throw new Error('API Key 不存在');
    }
    record.status = 'REVOKED';
    return clone(record);
  },

  async getTrace(taskId: string): Promise<GetTraceResponse> {
    await latency();
    return clone(traces[taskId] ?? defaultTrace(taskId));
  },

  async getReputation(userId: string): Promise<GetReputationResponse> {
    await latency();
    const summary = userSummaries[userId];
    const ratings =
      userId === 'usr_001'
        ? [...ratingsForUsr001, ...extraRatings]
        : userId === 'usr_002'
          ? ratingsForUsr002
          : [];

    return clone({
      userId,
      score: summary?.reputationScore ?? 4.5,
      dimensions: me.skills.map((skill) => ({ ...skill })),
      ratings,
    });
  },

  async submitRating(input: SubmitRatingInput): Promise<SubmitRatingResponse> {
    await latency();
    const rating: Rating = {
      id: `rat_${String(nextRatingId).padStart(3, '0')}`,
      taskId: input.taskId,
      direction: input.direction,
      score: input.score,
      tags: input.tags,
      comment: input.comment,
      createdAt: new Date().toISOString(),
    };
    nextRatingId += 1;
    extraRatings.unshift(rating);
    return clone(rating);
  },

  async registerChannel(input: RegisterChannelInput): Promise<RegisterChannelResponse> {
    await latency();
    const hasEvidence = Object.values(input.evidence).some(
      (value) => typeof value === 'string' && value.trim().length > 0,
    );
    return {
      channel: input.channel,
      status: hasEvidence ? 'VERIFIED' : 'PENDING',
    };
  },

  async signContract(input: SignContractInput): Promise<SignContractResponse> {
    await latency();
    const signedAt = new Date().toISOString();
    return {
      taskId: input.taskId,
      signerRole: input.signerRole,
      clausesAccepted: [...input.clausesAccepted],
      signatureHash: pseudoHash(`${input.taskId}:${input.signerRole}:${signedAt}`),
      signedAt,
    };
  },
};
