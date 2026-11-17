package marketplace

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Error is a domain error carrying the HTTP status the handler should emit.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(status int, format string, args ...any) *Error {
	return &Error{Status: status, Message: fmt.Sprintf(format, args...)}
}

// TaskQuery is the normalized filter set for GET /api/tasks.
type TaskQuery struct {
	Status    TaskStatus
	Keyword   string
	RewardMin *float64
	RewardMax *float64
	Page      int
	PageSize  int
}

type taskRecord struct {
	detail          TaskDetail
	acceptCount     int
	validationScore int
}

// Store is the thread-safe marketplace state. When db is nil the store is
// memory-only (NewStore); OpenStore attaches a SQLite handle and persists every
// mutation.
type Store struct {
	mu sync.RWMutex

	db *sql.DB

	me          User
	users       map[string]UserSummary
	tasks       []*taskRecord
	traces      map[string]GetTraceResponse
	wallet      GetWalletResponse
	reputations map[string]ReputationSummary

	nextTaskID   int
	nextRatingID int
	nextTxID     int
}

const pageSize = 20

// NewStore builds a Store seeded with the deterministic demo data.
func NewStore() *Store {
	users := map[string]UserSummary{
		"usr_001": {ID: "usr_001", DisplayName: "林知远", ReputationScore: 4.8},
		"usr_002": {ID: "usr_002", DisplayName: "某安全实验室", ReputationScore: 4.6},
		"usr_003": {ID: "usr_003", DisplayName: "星环数据", ReputationScore: 4.9},
		"usr_004": {ID: "usr_004", DisplayName: "云栖智造", ReputationScore: 4.4},
		"usr_005": {ID: "usr_005", DisplayName: "独立研究员陈默", ReputationScore: 4.7},
	}

	me := User{
		ID:                  "usr_001",
		DisplayName:         "林知远",
		AvatarURL:           "/avatars/u1.png",
		ActiveRole:          RoleCommander,
		Roles:               []UserRole{RoleEmployer, RoleCommander},
		VerificationChannel: ChannelUniversity,
		Reputation: UserReputation{
			Score:          4.8,
			CompletedTasks: 32,
			OnTimeRate:     0.97,
			Disputes:       1,
		},
		Skills: []SkillScore{
			{Key: SkillInstructionPrecision, Label: "AI指令精准度", Score: 88},
			{Key: SkillTaskClosure, Label: "任务闭环能力", Score: 76},
			{Key: SkillCostEfficiency, Label: "成本控制", Score: 81},
			{Key: SkillRiskControl, Label: "风险控制", Score: 72},
			{Key: SkillEvidenceQuality, Label: "证据质量", Score: 84},
		},
		Expertise: []ExpertiseItem{
			{ID: "exp_001", Kind: ExpertiseAgent, Title: "代码审计 Agent 编排", Level: "L3", UsageCount: 42, UpdatedAt: "2026-11-01T09:20:00Z"},
			{ID: "exp_002", Kind: ExpertiseDataAsset, Title: "IoT 固件协议语料库", Level: "L2", UsageCount: 18, UpdatedAt: "2026-10-22T14:05:00Z"},
			{ID: "exp_003", Kind: ExpertisePromptTemplate, Title: "多智能体交叉复核提示词模板", Level: "L3", UsageCount: 57, UpdatedAt: "2026-11-08T03:40:00Z"},
		},
		Badges: []TrustBadge{
			{ID: "bdg_001", Type: BadgeUniversity, Label: "高校学生认证", Issuer: "PatchTrace × 某高校", VerifiedAt: "2026-09-12T00:00:00Z", Status: BadgeVerified},
			{ID: "bdg_002", Type: BadgeTaskClearance, Label: "任务通关认证 · 安全审计", Issuer: "PatchTrace 平台", VerifiedAt: "2026-10-30T00:00:00Z", Status: BadgeVerified},
			{ID: "bdg_003", Type: BadgeEnterprise, Label: "企业白名单认证", Issuer: "某安全实验室", VerifiedAt: "2026-11-02T00:00:00Z", Status: BadgePending},
		},
	}

	return &Store{
		me:           me,
		users:        users,
		tasks:        seedTasks(users),
		traces:       seedTraces(),
		wallet:       seedWallet(),
		reputations:  seedReputations(me.Skills),
		nextTaskID:   1008,
		nextRatingID: 4,
		nextTxID:     7,
	}
}

func stringPtr(value string) *string { return &value }

func stage(key PipelineStageKey, status PipelineStageStatus, at, by, summary string) PipelineStage {
	result := PipelineStage{Key: key, Status: status}
	if at != "" {
		result.At = stringPtr(at)
	}
	if by != "" {
		result.By = stringPtr(by)
	}
	if summary != "" {
		result.Summary = stringPtr(summary)
	}
	return result
}

func pendingStage(key PipelineStageKey) PipelineStage {
	return PipelineStage{Key: key, Status: PipelineStatusPending}
}

func allPendingPipeline() Pipeline {
	return Pipeline{
		CurrentStage: PipelineStageAIGeneration,
		Stages: []PipelineStage{
			pendingStage(PipelineStageAIGeneration),
			pendingStage(PipelineStageAIReview),
			pendingStage(PipelineStageComplianceScan),
			pendingStage(PipelineStageSettlement),
		},
	}
}

func seedTasks(users map[string]UserSummary) []*taskRecord {
	pub := func(id string) UserSummary { return users[id] }
	assignee := func(id string) *UserSummary {
		user := users[id]
		return &user
	}

	return []*taskRecord{
		{
			detail: TaskDetail{
				ID:          "task_1001",
				Title:       "为一款 IoT 固件编写协议模糊测试 Agent",
				Description: "针对某型工业网关固件，构建可复现的协议模糊测试 Agent，覆盖 MQTT / Modbus 入口，输出测试计划、语料种子与崩溃归因报告。",
				Category:    "SECURITY",
				Status:      StatusPublished,
				Publisher:   pub("usr_002"),
				Assignee:    nil,
				Reward:      Money{Amount: 1200, Currency: "CNY"},
				Escrow:      EscrowState{State: EscrowFrozen, Amount: 1200, FrozenAt: stringPtr("2026-11-15T10:00:00Z")},
				Pipeline:    allPendingPipeline(),
				RequiredSkills: []string{
					"AGENT_ORCHESTRATION", "FUZZING",
				},
				AcceptanceCriteria: []string{"可复现脚本", "执行日志", "结论报告"},
				CreatedAt:          "2026-11-15T10:00:00Z",
				Deadline:           "2026-11-30T12:00:00Z",
			},
			acceptCount:     3,
			validationScore: 87,
		},
		{
			detail: TaskDetail{
				ID:          "task_1002",
				Title:       "电商风控日志异常检测 Agent",
				Description: "基于脱敏后的登录与支付日志，搭建异常检测 Agent，要求给出特征说明、阈值策略与误报分析，并保留可审计的模型调用链。",
				Category:    "DATA",
				Status:      StatusAIValidating,
				Publisher:   pub("usr_003"),
				Assignee:    assignee("usr_001"),
				Reward:      Money{Amount: 860, Currency: "CNY"},
				Escrow:      EscrowState{State: EscrowFrozen, Amount: 860, FrozenAt: stringPtr("2026-11-16T02:00:00Z")},
				Pipeline: Pipeline{
					CurrentStage: PipelineStageAIReview,
					Stages: []PipelineStage{
						stage(PipelineStageAIGeneration, PipelineStatusDone, "2026-11-16T02:10:00Z", "agent://commander-01", "生成 3 条候选路径"),
						stage(PipelineStageAIReview, PipelineStatusRunning, "2026-11-16T02:20:00Z", "", "平台审核 AI 交叉复核"),
						pendingStage(PipelineStageComplianceScan),
						pendingStage(PipelineStageSettlement),
					},
				},
				RequiredSkills:     []string{"DATA_PIPELINE", "ANOMALY_DETECTION"},
				AcceptanceCriteria: []string{"特征说明文档", "阈值策略", "误报分析报告"},
				CreatedAt:          "2026-11-14T07:30:00Z",
				Deadline:           "2026-11-22T12:00:00Z",
			},
			acceptCount:     5,
			validationScore: 91,
		},
		{
			detail: TaskDetail{
				ID:          "task_1003",
				Title:       "多租户 SaaS 权限边界自动化回归",
				Description: "为多租户 SaaS 设计跨租户越权回归套件，覆盖角色继承、资源归属与 API 网关策略，要求产出可重复执行的测试脚本与失败用例归因。",
				Category:    "DEVELOPMENT",
				Status:      StatusInProgress,
				Publisher:   pub("usr_004"),
				Assignee:    assignee("usr_005"),
				Reward:      Money{Amount: 2400, Currency: "CNY"},
				Escrow:      EscrowState{State: EscrowFrozen, Amount: 2400, FrozenAt: stringPtr("2026-11-14T08:30:00Z")},
				Pipeline: Pipeline{
					CurrentStage: PipelineStageAIGeneration,
					Stages: []PipelineStage{
						stage(PipelineStageAIGeneration, PipelineStatusRunning, "2026-11-15T01:10:00Z", "agent://commander-03", "生成越权测试矩阵"),
						pendingStage(PipelineStageAIReview),
						pendingStage(PipelineStageComplianceScan),
						pendingStage(PipelineStageSettlement),
					},
				},
				RequiredSkills:     []string{"TEST_AUTOMATION", "ACCESS_CONTROL"},
				AcceptanceCriteria: []string{"测试矩阵", "可重复脚本", "失败用例归因"},
				CreatedAt:          "2026-11-13T03:00:00Z",
				Deadline:           "2026-11-27T12:00:00Z",
			},
			acceptCount:     2,
			validationScore: 78,
		},
		{
			detail: TaskDetail{
				ID:          "task_1004",
				Title:       "Linux 内核提权 PoC 复现与缓解建议",
				Description: "在隔离环境中复现指定内核提权漏洞，提交最小 PoC、触发条件、受影响版本区间与缓解配置建议，禁止触碰生产数据。",
				Category:    "SECURITY",
				Status:      StatusComplianceScan,
				Publisher:   pub("usr_002"),
				Assignee:    assignee("usr_001"),
				Reward:      Money{Amount: 1500, Currency: "CNY"},
				Escrow:      EscrowState{State: EscrowFrozen, Amount: 1500, FrozenAt: stringPtr("2026-11-13T06:00:00Z")},
				Pipeline: Pipeline{
					CurrentStage: PipelineStageComplianceScan,
					Stages: []PipelineStage{
						stage(PipelineStageAIGeneration, PipelineStatusDone, "2026-11-13T07:20:00Z", "agent://commander-01", "生成 PoC 与复现步骤"),
						stage(PipelineStageAIReview, PipelineStatusDone, "2026-11-14T09:40:00Z", "agent://reviewer-02", "交叉复核通过"),
						stage(PipelineStageComplianceScan, PipelineStatusRunning, "2026-11-15T02:00:00Z", "", "合规扫描进行中"),
						pendingStage(PipelineStageSettlement),
					},
				},
				RequiredSkills:     []string{"KERNEL", "EXPLOIT_REPRODUCTION"},
				AcceptanceCriteria: []string{"最小 PoC", "受影响版本区间", "缓解建议"},
				CreatedAt:          "2026-11-12T05:00:00Z",
				Deadline:           "2026-11-25T12:00:00Z",
			},
			acceptCount:     6,
			validationScore: 94,
		},
		{
			detail: TaskDetail{
				ID:          "task_1005",
				Title:       "公开漏洞公告语义抽取与归因",
				Description: "从公开漏洞公告中抽取受影响产品、攻击向量与利用条件，构建可追溯的结构化数据集，并给出抽取准确率评估。",
				Category:    "RESEARCH",
				Status:      StatusSettling,
				Publisher:   pub("usr_005"),
				Assignee:    assignee("usr_001"),
				Reward:      Money{Amount: 600, Currency: "CNY"},
				Escrow:      EscrowState{State: EscrowFrozen, Amount: 600, FrozenAt: stringPtr("2026-11-11T09:00:00Z")},
				Pipeline: Pipeline{
					CurrentStage: PipelineStageSettlement,
					Stages: []PipelineStage{
						stage(PipelineStageAIGeneration, PipelineStatusDone, "2026-11-11T10:10:00Z", "agent://commander-01", "生成抽取模板"),
						stage(PipelineStageAIReview, PipelineStatusDone, "2026-11-12T04:30:00Z", "agent://reviewer-01", "抽样复核通过"),
						stage(PipelineStageComplianceScan, PipelineStatusDone, "2026-11-12T08:00:00Z", "agent://compliance-01", "无敏感数据"),
						stage(PipelineStageSettlement, PipelineStatusRunning, "2026-11-16T01:00:00Z", "", "托管资金结算中"),
					},
				},
				RequiredSkills:     []string{"NLP_EXTRACTION", "DATA_QUALITY"},
				AcceptanceCriteria: []string{"结构化数据集", "准确率评估", "来源溯源清单"},
				CreatedAt:          "2026-11-10T02:00:00Z",
				Deadline:           "2026-11-20T12:00:00Z",
			},
			acceptCount:     4,
			validationScore: 89,
		},
		{
			detail: TaskDetail{
				ID:          "task_1006",
				Title:       "供应链依赖 SBOM 差分审计",
				Description: "对两个版本的 SBOM 做差分审计，识别新增高危依赖、许可证冲突与传递依赖风险，输出可执行的升级与隔离建议。",
				Category:    "DATA",
				Status:      StatusCompleted,
				Publisher:   pub("usr_003"),
				Assignee:    assignee("usr_005"),
				Reward:      Money{Amount: 980, Currency: "CNY"},
				Escrow:      EscrowState{State: EscrowReleased, Amount: 980, FrozenAt: stringPtr("2026-11-08T03:00:00Z")},
				Pipeline: Pipeline{
					CurrentStage: PipelineStageSettlement,
					Stages: []PipelineStage{
						stage(PipelineStageAIGeneration, PipelineStatusDone, "2026-11-08T04:20:00Z", "agent://commander-05", "生成差分清单"),
						stage(PipelineStageAIReview, PipelineStatusDone, "2026-11-09T06:10:00Z", "agent://reviewer-03", "交叉复核通过"),
						stage(PipelineStageComplianceScan, PipelineStatusDone, "2026-11-09T09:00:00Z", "agent://compliance-02", "许可证冲突已标注"),
						stage(PipelineStageSettlement, PipelineStatusDone, "2026-11-10T02:00:00Z", "platform://escrow", "资金已释放"),
					},
				},
				RequiredSkills:     []string{"SBOM", "SUPPLY_CHAIN"},
				AcceptanceCriteria: []string{"差分清单", "风险分级", "升级与隔离建议"},
				CreatedAt:          "2026-11-07T02:00:00Z",
				Deadline:           "2026-11-18T12:00:00Z",
			},
			acceptCount:     3,
			validationScore: 92,
		},
		{
			detail: TaskDetail{
				ID:          "task_1007",
				Title:       "红队演练证据链复核",
				Description: "对一次红队演练的完整证据链做独立复核，验证时间戳、哈希链与操作授权记录的一致性，输出争议点与仲裁建议。",
				Category:    "SECURITY",
				Status:      StatusDisputed,
				Publisher:   pub("usr_004"),
				Assignee:    assignee("usr_005"),
				Reward:      Money{Amount: 3200, Currency: "CNY"},
				Escrow:      EscrowState{State: EscrowFrozen, Amount: 3200, FrozenAt: stringPtr("2026-11-05T02:00:00Z")},
				Pipeline: Pipeline{
					CurrentStage: PipelineStageAIReview,
					Stages: []PipelineStage{
						stage(PipelineStageAIGeneration, PipelineStatusDone, "2026-11-05T03:20:00Z", "agent://commander-04", "生成复核路径"),
						stage(PipelineStageAIReview, PipelineStatusFailed, "2026-11-06T05:40:00Z", "agent://reviewer-02", "交叉复核存在争议"),
						pendingStage(PipelineStageComplianceScan),
						pendingStage(PipelineStageSettlement),
					},
				},
				RequiredSkills:     []string{"FORENSICS", "HASH_CHAIN"},
				AcceptanceCriteria: []string{"复核报告", "争议点清单", "仲裁建议"},
				CreatedAt:          "2026-11-04T01:00:00Z",
				Deadline:           "2026-11-19T12:00:00Z",
			},
			acceptCount:     1,
			validationScore: 64,
		},
	}
}

func seedTraces() map[string]GetTraceResponse {
	return map[string]GetTraceResponse{
		"task_1001": {
			TaskID:   "task_1001",
			HeadHash: "sha256:6f7a8b9c0d1e2f3a",
			Nodes: []TraceNode{
				{
					ID:      "node_001",
					Type:    TraceRequirement,
					Actor:   TraceActor{Kind: ActorUser, ID: "usr_002", Label: "某安全实验室"},
					At:      "2026-11-15T10:00:00Z",
					Summary: "发布任务并冻结赏金 ¥1200",
					Evidence: TraceEvidence{
						Hash:       "sha256:1a2b3c4d5e6f7a8b",
						PayloadRef: stringPtr("blob://task/1001/req"),
						Model:      nil,
						Cost:       0,
					},
					PrevHash: nil,
					Hash:     "sha256:1a2b3c4d5e6f7a8b",
				},
				{
					ID:      "node_002",
					Type:    TraceAcceptance,
					Actor:   TraceActor{Kind: ActorSystem, ID: "platform://validator", Label: "平台验收标准生成器"},
					At:      "2026-11-15T10:02:00Z",
					Summary: "固化 3 项验收标准并写入哈希链",
					Evidence: TraceEvidence{
						Hash:       "sha256:2b3c4d5e6f7a8b9c",
						PayloadRef: stringPtr("blob://task/1001/acceptance"),
						Model:      nil,
						Cost:       0,
					},
					PrevHash: stringPtr("sha256:1a2b3c4d5e6f7a8b"),
					Hash:     "sha256:2b3c4d5e6f7a8b9c",
				},
				{
					ID:      "node_003",
					Type:    TraceModelCall,
					Actor:   TraceActor{Kind: ActorAgent, ID: "agent://commander-01", Label: "Claude Sonnet"},
					At:      "2026-11-16T02:11:00Z",
					Summary: "生成候选测试计划",
					Evidence: TraceEvidence{
						Hash:       "sha256:3c4d5e6f7a8b9c0d",
						PayloadRef: stringPtr("blob://trace/1001/step3"),
						Model:      stringPtr("claude-sonnet-5-5"),
						Cost:       0.12,
					},
					PrevHash: stringPtr("sha256:2b3c4d5e6f7a8b9c"),
					Hash:     "sha256:3c4d5e6f7a8b9c0d",
				},
				{
					ID:      "node_004",
					Type:    TraceAction,
					Actor:   TraceActor{Kind: ActorAgent, ID: "agent://executor-mqtt", Label: "MQTT 模糊执行器"},
					At:      "2026-11-16T02:24:00Z",
					Summary: "在隔离沙箱执行 12 轮变异语料",
					Evidence: TraceEvidence{
						Hash:       "sha256:4d5e6f7a8b9c0d1e",
						PayloadRef: stringPtr("blob://trace/1001/step4"),
						Model:      nil,
						Cost:       0.03,
					},
					PrevHash: stringPtr("sha256:3c4d5e6f7a8b9c0d"),
					Hash:     "sha256:4d5e6f7a8b9c0d1e",
				},
				{
					ID:      "node_005",
					Type:    TraceOutput,
					Actor:   TraceActor{Kind: ActorAgent, ID: "agent://evidence-packager", Label: "证据打包器"},
					At:      "2026-11-16T02:31:00Z",
					Summary: "产出可复现脚本、崩溃样本与执行日志",
					Evidence: TraceEvidence{
						Hash:       "sha256:5e6f7a8b9c0d1e2f",
						PayloadRef: stringPtr("blob://trace/1001/output"),
						Model:      nil,
						Cost:       0,
					},
					PrevHash: stringPtr("sha256:4d5e6f7a8b9c0d1e"),
					Hash:     "sha256:5e6f7a8b9c0d1e2f",
				},
				{
					ID:      "node_006",
					Type:    TraceValidation,
					Actor:   TraceActor{Kind: ActorSystem, ID: "platform://cross-check", Label: "平台审核 AI"},
					At:      "2026-11-16T02:40:00Z",
					Summary: "交叉复核通过：证据链完整、哈希连续",
					Evidence: TraceEvidence{
						Hash:       "sha256:6f7a8b9c0d1e2f3a",
						PayloadRef: stringPtr("blob://trace/1001/validation"),
						Model:      stringPtr("claude-sonnet-5-5"),
						Cost:       0.08,
					},
					PrevHash: stringPtr("sha256:5e6f7a8b9c0d1e2f"),
					Hash:     "sha256:6f7a8b9c0d1e2f3a",
				},
			},
		},
		"task_1004": {
			TaskID:   "task_1004",
			HeadHash: "sha256:9c0d1e2f3a4b5c6d",
			Nodes: []TraceNode{
				{
					ID:      "node_101",
					Type:    TraceRequirement,
					Actor:   TraceActor{Kind: ActorUser, ID: "usr_002", Label: "某安全实验室"},
					At:      "2026-11-12T05:00:00Z",
					Summary: "发布内核提权 PoC 复现任务并冻结赏金",
					Evidence: TraceEvidence{
						Hash:       "sha256:6c0d1e2f3a4b5c6d",
						PayloadRef: stringPtr("blob://task/1004/req"),
						Model:      nil,
						Cost:       0,
					},
					PrevHash: nil,
					Hash:     "sha256:6c0d1e2f3a4b5c6d",
				},
				{
					ID:      "node_102",
					Type:    TraceModelCall,
					Actor:   TraceActor{Kind: ActorAgent, ID: "agent://commander-01", Label: "Claude Sonnet"},
					At:      "2026-11-13T07:20:00Z",
					Summary: "生成 PoC 与复现步骤",
					Evidence: TraceEvidence{
						Hash:       "sha256:7d1e2f3a4b5c6d7e",
						PayloadRef: stringPtr("blob://trace/1004/step2"),
						Model:      stringPtr("claude-sonnet-5-5"),
						Cost:       0.21,
					},
					PrevHash: stringPtr("sha256:6c0d1e2f3a4b5c6d"),
					Hash:     "sha256:7d1e2f3a4b5c6d7e",
				},
				{
					ID:      "node_103",
					Type:    TraceAction,
					Actor:   TraceActor{Kind: ActorAgent, ID: "agent://executor-kernel", Label: "隔离内核沙箱"},
					At:      "2026-11-13T08:05:00Z",
					Summary: "在隔离环境复现提权并采集内核日志",
					Evidence: TraceEvidence{
						Hash:       "sha256:8e2f3a4b5c6d7e8f",
						PayloadRef: stringPtr("blob://trace/1004/step3"),
						Model:      nil,
						Cost:       0.05,
					},
					PrevHash: stringPtr("sha256:7d1e2f3a4b5c6d7e"),
					Hash:     "sha256:8e2f3a4b5c6d7e8f",
				},
				{
					ID:      "node_104",
					Type:    TraceValidation,
					Actor:   TraceActor{Kind: ActorSystem, ID: "platform://cross-check", Label: "平台审核 AI"},
					At:      "2026-11-14T09:40:00Z",
					Summary: "交叉复核通过，进入合规扫描",
					Evidence: TraceEvidence{
						Hash:       "sha256:9c0d1e2f3a4b5c6d",
						PayloadRef: stringPtr("blob://trace/1004/validation"),
						Model:      stringPtr("claude-sonnet-5-5"),
						Cost:       0.09,
					},
					PrevHash: stringPtr("sha256:8e2f3a4b5c6d7e8f"),
					Hash:     "sha256:9c0d1e2f3a4b5c6d",
				},
			},
		},
	}
}

func seedWallet() GetWalletResponse {
	return GetWalletResponse{
		Balance: WalletBalance{Available: 3200, Frozen: 1200, Currency: "CNY"},
		Transactions: []WalletTransaction{
			{ID: "tx_001", Type: TxEscrowFreeze, Amount: 1200, TaskID: "task_1001", At: "2026-11-15T10:00:00Z", Status: TxSettled},
			{ID: "tx_002", Type: TxEscrowFreeze, Amount: 860, TaskID: "task_1002", At: "2026-11-16T02:00:00Z", Status: TxSettled},
			{ID: "tx_003", Type: TxDeposit, Amount: 5000, At: "2026-11-10T08:00:00Z", Status: TxSettled},
			{ID: "tx_004", Type: TxEscrowRelease, Amount: 980, TaskID: "task_1006", At: "2026-11-10T02:00:00Z", Status: TxSettled},
			{ID: "tx_005", Type: TxFee, Amount: 12, TaskID: "task_1006", At: "2026-11-10T02:00:00Z", Status: TxSettled},
			{ID: "tx_006", Type: TxDeposit, Amount: 1500, At: "2026-11-08T06:30:00Z", Status: TxSettled},
		},
	}
}

func dimensionsFromSkills(skills []SkillScore) []ReputationDimension {
	dimensions := make([]ReputationDimension, len(skills))
	for index, skill := range skills {
		dimensions[index] = ReputationDimension{Key: string(skill.Key), Label: skill.Label, Score: skill.Score}
	}
	return dimensions
}

func seedReputations(skills []SkillScore) map[string]ReputationSummary {
	dimensions := dimensionsFromSkills(skills)

	return map[string]ReputationSummary{
		"usr_001": {
			UserID:     "usr_001",
			Score:      4.8,
			Dimensions: append([]ReputationDimension(nil), dimensions...),
			Ratings: []Rating{
				{ID: "rat_001", TaskID: "task_1006", Direction: RatingCommanderToEmployer, Score: 5, Tags: []string{"需求清晰", "结算及时"}, Comment: "验收标准明确，托管释放很快。", CreatedAt: "2026-11-10T03:10:00Z"},
				{ID: "rat_002", TaskID: "task_1004", Direction: RatingEmployerToCommander, Score: 4.8, Tags: []string{"证据完整", "沟通顺畅"}, Comment: "PoC 与复现步骤可执行，证据链完整。", CreatedAt: "2026-11-15T06:20:00Z"},
				{ID: "rat_003", TaskID: "task_1002", Direction: RatingEmployerToCommander, Score: 4.6, Tags: []string{"交付准时", "文档清晰"}, Comment: "阈值策略解释充分，误报分析到位。", CreatedAt: "2026-11-16T02:50:00Z"},
			},
		},
		"usr_002": {
			UserID:     "usr_002",
			Score:      4.6,
			Dimensions: append([]ReputationDimension(nil), dimensions...),
			Ratings: []Rating{
				{ID: "rat_101", TaskID: "task_1001", Direction: RatingCommanderToEmployer, Score: 4.5, Tags: []string{"授权完整", "沟通顺畅"}, Comment: "测试范围与授权边界清晰。", CreatedAt: "2026-11-15T12:00:00Z"},
			},
		},
	}
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// Me returns a copy of the current user profile.
func (s *Store) Me() User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneUser(s.me)
}

// ListTasks returns the filtered, paginated task summaries.
func (s *Store) ListTasks(query TaskQuery) GetTasksResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	page := query.Page
	if page < 1 {
		page = 1
	}
	size := query.PageSize
	if size < 1 {
		size = pageSize
	}

	filtered := make([]TaskSummary, 0, len(s.tasks))
	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	for _, record := range s.tasks {
		if query.Status != "" && record.detail.Status != query.Status {
			continue
		}
		if keyword != "" {
			haystack := strings.ToLower(record.detail.Title + " " + record.detail.Category + " " + record.detail.Publisher.DisplayName)
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		if query.RewardMin != nil && record.detail.Reward.Amount < *query.RewardMin {
			continue
		}
		if query.RewardMax != nil && record.detail.Reward.Amount > *query.RewardMax {
			continue
		}
		filtered = append(filtered, summaryFrom(record))
	}

	total := len(filtered)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}

	items := make([]TaskSummary, end-start)
	copy(items, filtered[start:end])

	return GetTasksResponse{Items: items, Total: total, Page: page, PageSize: size}
}

// GetTask returns a task detail by ID.
func (s *Store) GetTask(id string) (TaskDetail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record := s.findTaskLocked(id)
	if record == nil {
		return TaskDetail{}, false
	}
	return cloneTaskDetail(record.detail), true
}

// PublishTask validates and creates a task, freezing the reward into escrow.
func (s *Store) PublishTask(input PublishTaskInput) (TaskDetail, error) {
	if strings.TrimSpace(input.Title) == "" {
		return TaskDetail{}, newError(400, "title is required")
	}
	if strings.TrimSpace(input.Description) == "" {
		return TaskDetail{}, newError(400, "description is required")
	}
	if !input.ComplianceAck {
		return TaskDetail{}, newError(400, "complianceAck must be true")
	}
	if input.Reward.Amount <= 0 {
		return TaskDetail{}, newError(400, "reward.amount must be greater than 0")
	}
	if input.Reward.Currency == "" {
		input.Reward.Currency = "CNY"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := nowString()
	id := fmt.Sprintf("task_%d", s.nextTaskID)
	s.nextTaskID++

	detail := TaskDetail{
		ID:                 id,
		Title:              input.Title,
		Description:        input.Description,
		Category:           input.Category,
		Status:             StatusPublished,
		Publisher:          s.users[s.me.ID],
		Assignee:           nil,
		Reward:             input.Reward,
		Escrow:             EscrowState{State: EscrowFrozen, Amount: input.Reward.Amount, FrozenAt: stringPtr(now)},
		Pipeline:           allPendingPipeline(),
		RequiredSkills:     cloneStrings(input.RequiredSkills),
		AcceptanceCriteria: cloneStrings(input.AcceptanceCriteria),
		CreatedAt:          now,
		Deadline:           input.Deadline,
	}

	s.tasks = append([]*taskRecord{{detail: detail}}, s.tasks...)

	s.wallet.Balance.Available -= input.Reward.Amount
	s.wallet.Balance.Frozen += input.Reward.Amount
	transaction := WalletTransaction{
		ID:     fmt.Sprintf("tx_%03d", s.nextTxID),
		Type:   TxEscrowFreeze,
		Amount: input.Reward.Amount,
		TaskID: id,
		At:     now,
		Status: TxSettled,
	}
	s.nextTxID++
	s.wallet.Transactions = append([]WalletTransaction{transaction}, s.wallet.Transactions...)

	if err := s.persistLocked(); err != nil {
		return TaskDetail{}, newError(http.StatusInternalServerError, "persist task: %v", err)
	}

	return cloneTaskDetail(detail), nil
}

// AcceptTask transitions a PUBLISHED task to ACCEPTED and grants a two-hour lease.
func (s *Store) AcceptTask(id string) (AcceptTaskResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := s.findTaskLocked(id)
	if record == nil {
		return AcceptTaskResponse{}, newError(404, "task not found")
	}
	if !CanTransition(record.detail.Status, StatusAccepted) {
		return AcceptTaskResponse{}, newError(409, "illegal transition %s -> %s", record.detail.Status, StatusAccepted)
	}

	record.detail.Status = StatusAccepted
	assignee := s.users[s.me.ID]
	record.detail.Assignee = &assignee
	record.acceptCount++

	leaseExpiresAt := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)

	if err := s.persistLocked(); err != nil {
		return AcceptTaskResponse{}, newError(http.StatusInternalServerError, "persist task acceptance: %v", err)
	}

	return AcceptTaskResponse{TaskID: id, Status: StatusAccepted, LeaseExpiresAt: leaseExpiresAt}, nil
}

// GetTrace returns the evidence chain for a known task, or an empty chain.
func (s *Store) GetTrace(id string) (GetTraceResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.findTaskLocked(id) == nil {
		return GetTraceResponse{}, false
	}
	if trace, ok := s.traces[id]; ok {
		return cloneTrace(trace), true
	}
	return GetTraceResponse{
		TaskID:   id,
		HeadHash: "sha256:0000000000000000",
		Nodes:    []TraceNode{},
	}, true
}

// Wallet returns a copy of the wallet snapshot.
func (s *Store) Wallet() GetWalletResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneWallet(s.wallet)
}

// Reputation returns a user's reputation summary. Unknown users get a default.
func (s *Store) Reputation(userID string) ReputationSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if summary, ok := s.reputations[userID]; ok {
		return cloneReputation(summary)
	}
	return ReputationSummary{
		UserID:     userID,
		Score:      4.5,
		Dimensions: dimensionsFromSkills(s.me.Skills),
		Ratings:    []Rating{},
	}
}

// SubmitRating appends a rating to the current user's history.
func (s *Store) SubmitRating(input SubmitRatingInput) (Rating, error) {
	if strings.TrimSpace(input.TaskID) == "" {
		return Rating{}, newError(400, "taskId is required")
	}
	if !ValidRatingDirection(input.Direction) {
		return Rating{}, newError(400, "invalid direction")
	}
	if input.Score < 0 || input.Score > 5 {
		return Rating{}, newError(400, "score must be between 0 and 5")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rating := Rating{
		ID:        fmt.Sprintf("rat_%03d", s.nextRatingID),
		TaskID:    input.TaskID,
		Direction: input.Direction,
		Score:     input.Score,
		Tags:      cloneStrings(input.Tags),
		Comment:   input.Comment,
		CreatedAt: nowString(),
	}
	s.nextRatingID++

	summary := s.reputations[s.me.ID]
	summary.Ratings = append([]Rating{rating}, summary.Ratings...)
	s.reputations[s.me.ID] = summary

	if err := s.persistLocked(); err != nil {
		return Rating{}, newError(http.StatusInternalServerError, "persist rating: %v", err)
	}

	return cloneRating(rating), nil
}

// RegisterChannel accepts a channel registration and returns its verification state.
func (s *Store) RegisterChannel(channel Channel, evidence RegisterEvidence) (RegisterChannelResponse, error) {
	if !ValidChannel(channel) {
		return RegisterChannelResponse{}, newError(400, "invalid channel")
	}
	status := "PENDING"
	if evidence.hasAny() {
		status = "VERIFIED"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.persistLocked(); err != nil {
		return RegisterChannelResponse{}, newError(http.StatusInternalServerError, "persist channel registration: %v", err)
	}

	return RegisterChannelResponse{Channel: channel, Status: status}, nil
}

// SignContract produces a deterministic contract signature for a known task.
func (s *Store) SignContract(input SignContractInput) (SignContractResponse, error) {
	if strings.TrimSpace(input.TaskID) == "" {
		return SignContractResponse{}, newError(400, "taskId is required")
	}
	if !ValidUserRole(input.SignerRole) {
		return SignContractResponse{}, newError(400, "invalid signerRole")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.findTaskLocked(input.TaskID) == nil {
		return SignContractResponse{}, newError(404, "task not found")
	}
	if err := s.persistLocked(); err != nil {
		return SignContractResponse{}, newError(http.StatusInternalServerError, "persist contract signing: %v", err)
	}

	clauses := input.ClausesAccepted
	if clauses == nil {
		clauses = []bool{}
	}

	return SignContractResponse{
		TaskID:          input.TaskID,
		SignerRole:      input.SignerRole,
		ClausesAccepted: cloneBools(clauses),
		SignatureHash:   signatureHash(input.TaskID, input.SignerRole, clauses),
		SignedAt:        nowString(),
	}, nil
}

func (s *Store) findTaskLocked(id string) *taskRecord {
	for _, record := range s.tasks {
		if record.detail.ID == id {
			return record
		}
	}
	return nil
}

func summaryFrom(record *taskRecord) TaskSummary {
	return TaskSummary{
		ID:              record.detail.ID,
		Title:           record.detail.Title,
		Category:        record.detail.Category,
		Status:          record.detail.Status,
		Reward:          record.detail.Reward,
		Deadline:        record.detail.Deadline,
		Publisher:       record.detail.Publisher,
		RequiredSkills:  cloneStrings(record.detail.RequiredSkills),
		AcceptCount:     record.acceptCount,
		ValidationScore: record.validationScore,
	}
}

// signatureHash is deterministic for the signing inputs; signedAt is excluded so
// repeated signatures of the same contract yield the same digest.
func signatureHash(taskID string, role UserRole, clauses []bool) string {
	var builder strings.Builder
	builder.WriteString(taskID)
	builder.WriteByte('|')
	builder.WriteString(string(role))
	builder.WriteByte('|')
	for index, accepted := range clauses {
		if index > 0 {
			builder.WriteByte(',')
		}
		if accepted {
			builder.WriteByte('1')
		} else {
			builder.WriteByte('0')
		}
	}
	digest := sha256.Sum256([]byte(builder.String()))
	return "sha256:" + hex.EncodeToString(digest[:])
}

// cloneStrings always returns a non-nil slice so JSON encodes an empty array
// rather than null; every string-array field in the DTO contract is required.
func cloneStrings(input []string) []string {
	output := make([]string, len(input))
	copy(output, input)
	return output
}

func cloneBools(input []bool) []bool {
	output := make([]bool, len(input))
	copy(output, input)
	return output
}

func cloneStages(input []PipelineStage) []PipelineStage {
	if input == nil {
		return nil
	}
	output := make([]PipelineStage, len(input))
	copy(output, input)
	for index, item := range input {
		output[index] = cloneStage(item)
	}
	return output
}

func cloneStage(input PipelineStage) PipelineStage {
	output := input
	if input.At != nil {
		output.At = stringPtr(*input.At)
	}
	if input.By != nil {
		output.By = stringPtr(*input.By)
	}
	if input.Summary != nil {
		output.Summary = stringPtr(*input.Summary)
	}
	return output
}

func cloneTaskDetail(input TaskDetail) TaskDetail {
	output := input
	output.RequiredSkills = cloneStrings(input.RequiredSkills)
	output.AcceptanceCriteria = cloneStrings(input.AcceptanceCriteria)
	output.Pipeline.Stages = cloneStages(input.Pipeline.Stages)
	if input.Assignee != nil {
		assignee := *input.Assignee
		output.Assignee = &assignee
	}
	if input.Escrow.FrozenAt != nil {
		output.Escrow.FrozenAt = stringPtr(*input.Escrow.FrozenAt)
	}
	return output
}

func cloneUser(input User) User {
	output := input
	output.Roles = append([]UserRole(nil), input.Roles...)
	output.Skills = append([]SkillScore(nil), input.Skills...)
	output.Expertise = append([]ExpertiseItem(nil), input.Expertise...)
	output.Badges = append([]TrustBadge(nil), input.Badges...)
	return output
}

func cloneTrace(input GetTraceResponse) GetTraceResponse {
	output := input
	output.Nodes = make([]TraceNode, len(input.Nodes))
	for index, node := range input.Nodes {
		output.Nodes[index] = node
		if node.PrevHash != nil {
			output.Nodes[index].PrevHash = stringPtr(*node.PrevHash)
		}
		if node.Evidence.PayloadRef != nil {
			output.Nodes[index].Evidence.PayloadRef = stringPtr(*node.Evidence.PayloadRef)
		}
		if node.Evidence.Model != nil {
			output.Nodes[index].Evidence.Model = stringPtr(*node.Evidence.Model)
		}
	}
	return output
}

func cloneWallet(input GetWalletResponse) GetWalletResponse {
	output := input
	output.Transactions = append([]WalletTransaction(nil), input.Transactions...)
	return output
}

func cloneRating(input Rating) Rating {
	output := input
	output.Tags = cloneStrings(input.Tags)
	return output
}

func cloneReputation(input ReputationSummary) ReputationSummary {
	output := input
	output.Dimensions = append([]ReputationDimension(nil), input.Dimensions...)
	output.Ratings = make([]Rating, len(input.Ratings))
	for index, rating := range input.Ratings {
		output.Ratings[index] = cloneRating(rating)
	}
	return output
}
