package taskManager

type ReportMeta struct {
	ReportID  string `json:"reportId"`
	TargetID  string `json:"targetId,omitempty"`
	Path      string `json:"path"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"createdAt"`
	FileName  string `json:"fileName,omitempty"`
}

type ReportChunk struct {
	ReportID   string `json:"reportId"`
	TargetID   string `json:"targetId,omitempty"`
	Summary    string `json:"summary"`
	CreatedAt  string `json:"createdAt"`
	FileName   string `json:"fileName,omitempty"`
	Offset     int    `json:"offset"`
	Limit      int    `json:"limit"`
	Returned   int    `json:"returned"`
	Total      int    `json:"total"`
	NextOffset int    `json:"nextOffset"`
	Done       bool   `json:"done"`
	Content    string `json:"content"`
}
