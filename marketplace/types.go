// Package marketplace implements the PatchTrace AI task marketplace HTTP API.
//
// The package is intentionally self-contained: it uses only the Go standard
// library and keeps all state in memory behind a sync.RWMutex. JSON tags mirror
// webui-marketplace/src/types/domain.ts and webui-marketplace/src/types/api.ts
// exactly; those files are the authoritative contract.
package marketplace

import "strings"

// UserRole is the platform identity perspective.
type UserRole string

const (
	RoleEmployer  UserRole = "EMPLOYER"
	RoleCommander UserRole = "COMMANDER"
)

// Channel is a registration / verification channel.
type Channel string

const (
	ChannelUniversity Channel = "UNIVERSITY"
	ChannelEnterprise Channel = "ENTERPRISE"
	ChannelFreelancer Channel = "FREELANCER"
)

// TaskStatus models the marketplace task state machine.
type TaskStatus string

const (
	StatusDraft            TaskStatus = "DRAFT"
	StatusPublished        TaskStatus = "PUBLISHED"
	StatusAccepted         TaskStatus = "ACCEPTED"
	StatusInProgress       TaskStatus = "IN_PROGRESS"
	StatusSubmitted        TaskStatus = "SUBMITTED"
	StatusAIValidating     TaskStatus = "AI_VALIDATING"
	StatusValidationFailed TaskStatus = "VALIDATION_FAILED"
	StatusComplianceScan   TaskStatus = "COMPLIANCE_SCAN"
	StatusSettling         TaskStatus = "SETTLING"
	StatusCompleted        TaskStatus = "COMPLETED"
	StatusDisputed         TaskStatus = "DISPUTED"
	StatusRefunded         TaskStatus = "REFUNDED"
	StatusCancelled        TaskStatus = "CANCELLED"
)

// allowedTransitions mirrors ALLOWED_TRANSITIONS in domain.ts.
var allowedTransitions = map[TaskStatus][]TaskStatus{
	StatusDraft:            {StatusPublished},
	StatusPublished:        {StatusAccepted, StatusDisputed, StatusCancelled},
	StatusAccepted:         {StatusInProgress, StatusDisputed},
	StatusInProgress:       {StatusSubmitted, StatusDisputed},
	StatusSubmitted:        {StatusAIValidating},
	StatusAIValidating:     {StatusComplianceScan, StatusValidationFailed},
	StatusValidationFailed: {StatusInProgress},
	StatusComplianceScan:   {StatusSettling},
	StatusSettling:         {StatusCompleted},
	StatusCompleted:        {},
	StatusDisputed:         {StatusCompleted, StatusRefunded},
	StatusRefunded:         {},
	StatusCancelled:        {},
}

// CanTransition reports whether from -> to is a legal task state transition.
func CanTransition(from, to TaskStatus) bool {
	for _, candidate := range allowedTransitions[from] {
		if candidate == to {
			return true
		}
	}
	return false
}

// ValidTaskStatus reports whether status is a known task status.
func ValidTaskStatus(status TaskStatus) bool {
	_, ok := allowedTransitions[status]
	return ok
}

// ValidUserRole reports whether role is a known identity perspective.
func ValidUserRole(role UserRole) bool {
	return role == RoleEmployer || role == RoleCommander
}

// ValidChannel reports whether channel is a known registration channel.
func ValidChannel(channel Channel) bool {
	switch channel {
	case ChannelUniversity, ChannelEnterprise, ChannelFreelancer:
		return true
	default:
		return false
	}
}

// PipelineStageKey identifies a stage of the AI validation pipeline.
type PipelineStageKey string

const (
	PipelineStageAIGeneration   PipelineStageKey = "AI_GENERATION"
	PipelineStageAIReview       PipelineStageKey = "AI_REVIEW"
	PipelineStageComplianceScan PipelineStageKey = "COMPLIANCE_SCAN"
	PipelineStageSettlement     PipelineStageKey = "SETTLEMENT"
)

// PipelineStageStatus is the execution state of a pipeline stage.
type PipelineStageStatus string

const (
	PipelineStatusPending PipelineStageStatus = "PENDING"
	PipelineStatusRunning PipelineStageStatus = "RUNNING"
	PipelineStatusDone    PipelineStageStatus = "DONE"
	PipelineStatusFailed  PipelineStageStatus = "FAILED"
)

// EscrowStatus is the escrow lifecycle state.
type EscrowStatus string

const (
	EscrowNone     EscrowStatus = "NONE"
	EscrowFrozen   EscrowStatus = "FROZEN"
	EscrowReleased EscrowStatus = "RELEASED"
	EscrowRefunded EscrowStatus = "REFUNDED"
)

// WalletTransactionType classifies a wallet ledger entry.
type WalletTransactionType string

const (
	TxEscrowFreeze  WalletTransactionType = "ESCROW_FREEZE"
	TxEscrowRelease WalletTransactionType = "ESCROW_RELEASE"
	TxRefund        WalletTransactionType = "REFUND"
	TxDeposit       WalletTransactionType = "DEPOSIT"
	TxWithdraw      WalletTransactionType = "WITHDRAW"
	TxFee           WalletTransactionType = "FEE"
)

// WalletTransactionStatus is the settlement state of a ledger entry.
type WalletTransactionStatus string

const (
	TxPending WalletTransactionStatus = "PENDING"
	TxSettled WalletTransactionStatus = "SETTLED"
	TxFailed  WalletTransactionStatus = "FAILED"
)

// TraceNodeType classifies an evidence-chain node.
type TraceNodeType string

const (
	TraceRequirement TraceNodeType = "REQUIREMENT"
	TraceAcceptance  TraceNodeType = "ACCEPTANCE"
	TraceModelCall   TraceNodeType = "MODEL_CALL"
	TraceAction      TraceNodeType = "ACTION"
	TraceOutput      TraceNodeType = "OUTPUT"
	TraceValidation  TraceNodeType = "VALIDATION"
)

// TraceActorKind identifies who produced a trace node.
type TraceActorKind string

const (
	ActorUser   TraceActorKind = "USER"
	ActorAgent  TraceActorKind = "AGENT"
	ActorSystem TraceActorKind = "SYSTEM"
)

// SkillDimension is a radar-chart skill dimension key.
type SkillDimension string

const (
	SkillInstructionPrecision SkillDimension = "INSTRUCTION_PRECISION"
	SkillTaskClosure          SkillDimension = "TASK_CLOSURE"
	SkillCostEfficiency       SkillDimension = "COST_EFFICIENCY"
	SkillRiskControl          SkillDimension = "RISK_CONTROL"
	SkillEvidenceQuality      SkillDimension = "EVIDENCE_QUALITY"
)

// ExpertiseKind classifies an expertise repository item.
type ExpertiseKind string

const (
	ExpertiseAgent          ExpertiseKind = "AGENT"
	ExpertiseDataAsset      ExpertiseKind = "DATA_ASSET"
	ExpertisePromptTemplate ExpertiseKind = "PROMPT_TEMPLATE"
)

// TrustBadgeType classifies a verification badge.
type TrustBadgeType string

const (
	BadgeUniversity    TrustBadgeType = "UNIVERSITY"
	BadgeEnterprise    TrustBadgeType = "ENTERPRISE"
	BadgeTaskClearance TrustBadgeType = "TASK_CLEARANCE"
)

// TrustBadgeStatus is a badge verification state.
type TrustBadgeStatus string

const (
	BadgeVerified TrustBadgeStatus = "VERIFIED"
	BadgePending  TrustBadgeStatus = "PENDING"
	BadgeRevoked  TrustBadgeStatus = "REVOKED"
)

// RatingDirection identifies who rates whom.
type RatingDirection string

const (
	RatingEmployerToCommander RatingDirection = "EMPLOYER_TO_COMMANDER"
	RatingCommanderToEmployer RatingDirection = "COMMANDER_TO_EMPLOYER"
)

// ValidRatingDirection reports whether direction is known.
func ValidRatingDirection(direction RatingDirection) bool {
	return direction == RatingEmployerToCommander || direction == RatingCommanderToEmployer
}

// Money is an amount in CNY.
type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// UserSummary is the lightweight user shape embedded in tasks.
type UserSummary struct {
	ID              string  `json:"id"`
	DisplayName     string  `json:"displayName"`
	ReputationScore float64 `json:"reputationScore"`
}

// PublisherSummary aliases UserSummary for readability.
type PublisherSummary = UserSummary

// TaskSummary is an item of GET /api/tasks.
type TaskSummary struct {
	ID              string      `json:"id"`
	Title           string      `json:"title"`
	Category        string      `json:"category"`
	Status          TaskStatus  `json:"status"`
	Reward          Money       `json:"reward"`
	Deadline        string      `json:"deadline"`
	Publisher       UserSummary `json:"publisher"`
	RequiredSkills  []string    `json:"requiredSkills"`
	AcceptCount     int         `json:"acceptCount"`
	ValidationScore int         `json:"validationScore"`
}

// PipelineStage is one stage of the AI validation pipeline.
type PipelineStage struct {
	Key     PipelineStageKey    `json:"key"`
	Status  PipelineStageStatus `json:"status"`
	At      *string             `json:"at,omitempty"`
	By      *string             `json:"by,omitempty"`
	Summary *string             `json:"summary,omitempty"`
}

// Pipeline is the full AI validation pipeline.
type Pipeline struct {
	CurrentStage PipelineStageKey `json:"currentStage"`
	Stages       []PipelineStage  `json:"stages"`
}

// EscrowState is the escrow snapshot on a task.
type EscrowState struct {
	State    EscrowStatus `json:"state"`
	Amount   float64      `json:"amount"`
	FrozenAt *string      `json:"frozenAt,omitempty"`
}

// TaskDetail is the full task shape (GET /api/tasks/{id}).
type TaskDetail struct {
	ID                 string       `json:"id"`
	Title              string       `json:"title"`
	Description        string       `json:"description"`
	Category           string       `json:"category"`
	Status             TaskStatus   `json:"status"`
	Publisher          UserSummary  `json:"publisher"`
	Assignee           *UserSummary `json:"assignee"`
	Reward             Money        `json:"reward"`
	Escrow             EscrowState  `json:"escrow"`
	Pipeline           Pipeline     `json:"pipeline"`
	RequiredSkills     []string     `json:"requiredSkills"`
	AcceptanceCriteria []string     `json:"acceptanceCriteria"`
	CreatedAt          string       `json:"createdAt"`
	Deadline           string       `json:"deadline"`
}

// WalletTransaction is one ledger entry.
type WalletTransaction struct {
	ID     string                  `json:"id"`
	Type   WalletTransactionType   `json:"type"`
	Amount float64                 `json:"amount"`
	TaskID string                  `json:"taskId,omitempty"`
	At     string                  `json:"at"`
	Status WalletTransactionStatus `json:"status"`
}

// WalletBalance is the available/frozen balance snapshot.
type WalletBalance struct {
	Available float64 `json:"available"`
	Frozen    float64 `json:"frozen"`
	Currency  string  `json:"currency"`
}

// TraceActor identifies the producer of a trace node.
type TraceActor struct {
	Kind  TraceActorKind `json:"kind"`
	ID    string         `json:"id"`
	Label string         `json:"label"`
}

// TraceEvidence is the tamper-evident payload reference for a trace node.
type TraceEvidence struct {
	Hash       string  `json:"hash"`
	PayloadRef *string `json:"payloadRef"`
	Model      *string `json:"model"`
	Cost       float64 `json:"cost"`
}

// TraceNode is one node of the evidence chain.
type TraceNode struct {
	ID       string        `json:"id"`
	Type     TraceNodeType `json:"type"`
	Actor    TraceActor    `json:"actor"`
	At       string        `json:"at"`
	Summary  string        `json:"summary"`
	Evidence TraceEvidence `json:"evidence"`
	PrevHash *string       `json:"prevHash"`
	Hash     string        `json:"hash"`
}

// ReputationDimension is one reputation radar dimension.
type ReputationDimension struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Score int    `json:"score"`
}

// SkillScore is a typed reputation dimension on the user profile.
type SkillScore struct {
	Key   SkillDimension `json:"key"`
	Label string         `json:"label"`
	Score int            `json:"score"`
}

// ExpertiseItem is one expertise repository entry.
type ExpertiseItem struct {
	ID         string        `json:"id"`
	Kind       ExpertiseKind `json:"kind"`
	Title      string        `json:"title"`
	Level      string        `json:"level"`
	UsageCount int           `json:"usageCount"`
	UpdatedAt  string        `json:"updatedAt"`
}

// TrustBadge is a verification badge.
type TrustBadge struct {
	ID         string           `json:"id"`
	Type       TrustBadgeType   `json:"type"`
	Label      string           `json:"label"`
	Issuer     string           `json:"issuer"`
	VerifiedAt string           `json:"verifiedAt"`
	Status     TrustBadgeStatus `json:"status"`
}

// UserReputation is the aggregate reputation counters.
type UserReputation struct {
	Score          float64 `json:"score"`
	CompletedTasks int     `json:"completedTasks"`
	OnTimeRate     float64 `json:"onTimeRate"`
	Disputes       int     `json:"disputes"`
}

// Rating is one bidirectional rating record.
type Rating struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"taskId"`
	Direction RatingDirection `json:"direction"`
	Score     float64         `json:"score"`
	Tags      []string        `json:"tags"`
	Comment   string          `json:"comment"`
	CreatedAt string          `json:"createdAt"`
}

// ReputationSummary is GET /api/reputation/{userId}.
type ReputationSummary struct {
	UserID     string                `json:"userId"`
	Score      float64               `json:"score"`
	Dimensions []ReputationDimension `json:"dimensions"`
	Ratings    []Rating              `json:"ratings"`
}

// User is GET /api/me.
type User struct {
	ID                  string          `json:"id"`
	DisplayName         string          `json:"displayName"`
	AvatarURL           string          `json:"avatarUrl"`
	ActiveRole          UserRole        `json:"activeRole"`
	Roles               []UserRole      `json:"roles"`
	VerificationChannel Channel         `json:"verificationChannel"`
	Reputation          UserReputation  `json:"reputation"`
	Skills              []SkillScore    `json:"skills"`
	Expertise           []ExpertiseItem `json:"expertise"`
	Badges              []TrustBadge    `json:"badges"`
}

// GetTasksResponse is GET /api/tasks.
type GetTasksResponse struct {
	Items    []TaskSummary `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

// PublishTaskInput is POST /api/tasks.
type PublishTaskInput struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Category           string   `json:"category"`
	Reward             Money    `json:"reward"`
	Deadline           string   `json:"deadline"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
	RequiredSkills     []string `json:"requiredSkills"`
	ComplianceAck      bool     `json:"complianceAck"`
}

// AcceptTaskResponse is POST /api/tasks/{id}/accept.
type AcceptTaskResponse struct {
	TaskID         string     `json:"taskId"`
	Status         TaskStatus `json:"status"`
	LeaseExpiresAt string     `json:"leaseExpiresAt"`
}

// GetTraceResponse is GET /api/tasks/{id}/trace.
type GetTraceResponse struct {
	TaskID   string      `json:"taskId"`
	HeadHash string      `json:"headHash"`
	Nodes    []TraceNode `json:"nodes"`
}

// GetWalletResponse is the legacy wallet shape (balance + transactions).
type GetWalletResponse struct {
	Balance      WalletBalance       `json:"balance"`
	Transactions []WalletTransaction `json:"transactions"`
}

// WalletVipLevel is the subscription tier of the current user.
type WalletVipLevel string

const (
	VipFree       WalletVipLevel = "FREE"
	VipPro        WalletVipLevel = "PRO"
	VipEnterprise WalletVipLevel = "ENTERPRISE"
)

// WalletVip is the VIP subscription snapshot.
type WalletVip struct {
	Level     WalletVipLevel `json:"level"`
	ExpiresAt *string        `json:"expiresAt"`
	FeeRate   float64        `json:"feeRate"`
}

// WalletOverview is GET /api/wallet: the legacy balance/transactions plus vip.
type WalletOverview struct {
	Balance      WalletBalance       `json:"balance"`
	Transactions []WalletTransaction `json:"transactions"`
	Vip          WalletVip           `json:"vip"`
}

// RechargeInput is POST /api/wallet/recharge.
type RechargeInput struct {
	Amount float64 `json:"amount"`
}

// SubscribeVipInput is POST /api/wallet/vip.
type SubscribeVipInput struct {
	Level string `json:"level"`
}

// SubmitRatingInput is POST /api/ratings.
type SubmitRatingInput struct {
	TaskID    string          `json:"taskId"`
	Direction RatingDirection `json:"direction"`
	Score     float64         `json:"score"`
	Tags      []string        `json:"tags"`
	Comment   string          `json:"comment"`
}

// RegisterEvidence is the optional evidence payload for channel registration.
type RegisterEvidence struct {
	StudentID   *string `json:"studentId,omitempty"`
	SchoolEmail *string `json:"schoolEmail,omitempty"`
	WorkEmail   *string `json:"workEmail,omitempty"`
	RealName    *string `json:"realName,omitempty"`
}

func (e RegisterEvidence) hasAny() bool {
	for _, value := range []*string{e.StudentID, e.SchoolEmail, e.WorkEmail, e.RealName} {
		if value != nil && strings.TrimSpace(*value) != "" {
			return true
		}
	}
	return false
}

// RegisterChannelInput is the body of POST /api/auth/register/{channel}.
type RegisterChannelInput struct {
	Channel  Channel          `json:"channel"`
	Evidence RegisterEvidence `json:"evidence"`
}

// RegisterChannelResponse is the registration result.
type RegisterChannelResponse struct {
	Channel Channel `json:"channel"`
	Status  string  `json:"status"`
}

// SignContractInput is POST /api/tasks/{id}/contracts/sign.
type SignContractInput struct {
	TaskID          string   `json:"taskId"`
	SignerRole      UserRole `json:"signerRole"`
	ClausesAccepted []bool   `json:"clausesAccepted"`
}

// SignContractResponse is the deterministic contract signature.
type SignContractResponse struct {
	TaskID          string   `json:"taskId"`
	SignerRole      UserRole `json:"signerRole"`
	ClausesAccepted []bool   `json:"clausesAccepted"`
	SignatureHash   string   `json:"signatureHash"`
	SignedAt        string   `json:"signedAt"`
}
