// Package graphserver exposes the Cairn-style provenance graph over HTTP and
// provides Client implementations used by the dispatcher.
package graphserver

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"AIxVuln/graph"

	"github.com/gin-gonic/gin"
)

// routePrefix is the mount point owned by Register.
const routePrefix = "/api/graph"

// defaultLease is applied when a request omits or zeroes its lease duration.
const defaultLease = 5 * time.Minute

// Server adapts a graph.Store to the graph protocol HTTP API.
type Server struct {
	store *graph.Store
}

// New returns a Server backed by store.
func New(store *graph.Store) *Server {
	return &Server{store: store}
}

// Register mounts the whole protocol family under /api/graph on r.
func (s *Server) Register(r gin.IRouter) {
	g := r.Group(routePrefix)
	g.POST("/projects", s.createProject)
	g.GET("/projects", s.listProjects)
	g.GET("/projects/:id", s.getProject)
	g.PUT("/projects/:id/status", s.setProjectStatus)
	g.POST("/projects/:id/reopen", s.reopen)
	g.POST("/projects/:id/hints", s.addHint)
	g.POST("/projects/:id/intents", s.declareIntent)
	g.POST("/projects/:id/intents/:iid/heartbeat", s.heartbeatIntent)
	g.POST("/projects/:id/intents/:iid/release", s.releaseIntent)
	g.POST("/projects/:id/intents/:iid/conclude", s.concludeIntent)
	g.POST("/projects/:id/reason/claim", s.claimReason)
	g.POST("/projects/:id/reason/heartbeat", s.heartbeatReason)
	g.POST("/projects/:id/reason/release", s.releaseReason)
	g.POST("/projects/:id/complete", s.complete)
	g.GET("/projects/:id/export", s.export)
	g.GET("/projects/:id/snapshot", s.snapshot)
	g.GET("/projects/:id/provenance", s.provenance)
}

// --- wire shapes -----------------------------------------------------------

type projectView struct {
	ID               string      `json:"id"`
	Title            string      `json:"title"`
	Origin           string      `json:"origin"`
	Goal             string      `json:"goal"`
	Status           string      `json:"status"`
	BootstrapEnabled bool        `json:"bootstrap_enabled"`
	CreatedAt        time.Time   `json:"created_at"`
	Reason           *reasonView `json:"reason"`
}

type reasonView struct {
	Worker          string    `json:"worker"`
	Trigger         string    `json:"trigger"`
	StartedAt       time.Time `json:"started_at"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at"`
}

type nodeView struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Kind        string     `json:"kind"`
	Type        string     `json:"type"`
	Content     string     `json:"content"`
	State       string     `json:"state"`
	From        []string   `json:"from"`
	To          *string    `json:"to"`
	Creator     string     `json:"creator"`
	Worker      string     `json:"worker"`
	LeaseUntil  *time.Time `json:"lease_until"`
	CreatedAt   *time.Time `json:"created_at"`
	ConcludedAt *time.Time `json:"concluded_at"`
}

type edgeView struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

func toEdgeView(e graph.Edge) edgeView {
	return edgeView{From: e.From, To: e.To, Kind: e.Kind}
}

func toEdgeViews(edges []graph.Edge) []edgeView {
	out := make([]edgeView, 0, len(edges))
	for _, e := range edges {
		out = append(out, toEdgeView(e))
	}
	return out
}

// snapshotView is the read-only payload the web UI renders. Everything is
// derived from store reads; it is never accepted as input.
type snapshotView struct {
	Project  projectView `json:"project"`
	Nodes    []nodeView  `json:"nodes"`
	Edges    []edgeView  `json:"edges"`
	Timeline string      `json:"timeline"`
}

type createProjectRequest struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Origin           string `json:"origin"`
	Goal             string `json:"goal"`
	BootstrapEnabled *bool  `json:"bootstrap_enabled"`
}

type projectStatusRequest struct {
	Status string `json:"status"`
}

type reopenRequest struct {
	Creator     string `json:"creator"`
	Description string `json:"description"`
}

type hintRequest struct {
	Content string `json:"content"`
	Creator string `json:"creator"`
}

type intentRequest struct {
	Type        string   `json:"type"`
	Content     string   `json:"content"`
	Description string   `json:"description"`
	From        []string `json:"from"`
	Creator     string   `json:"creator"`
	Worker      string   `json:"worker"`
}

type workerLeaseRequest struct {
	Worker       string `json:"worker"`
	Trigger      string `json:"trigger"`
	LeaseSeconds int    `json:"lease_seconds"`
}

type releaseRequest struct {
	Worker string `json:"worker"`
}

type concludeRequest struct {
	Worker      string `json:"worker"`
	Description string `json:"description"`
	FactType    string `json:"fact_type"`
	Type        string `json:"type"`
}

type completeRequest struct {
	From        []string `json:"from"`
	Description string   `json:"description"`
	Worker      string   `json:"worker"`
}

func toProjectView(p graph.Project) projectView {
	v := projectView{
		ID:               p.ID,
		Title:            p.Title,
		Origin:           p.Origin,
		Goal:             p.Goal,
		Status:           p.Status,
		BootstrapEnabled: p.BootstrapEnabled,
		CreatedAt:        p.CreatedAt,
	}
	if p.Reason != nil {
		v.Reason = &reasonView{
			Worker:          p.Reason.Worker,
			Trigger:         p.Reason.Trigger,
			StartedAt:       p.Reason.StartedAt,
			LastHeartbeatAt: p.Reason.LastHeartbeatAt,
		}
	}
	return v
}

func toProjectViews(projects []graph.Project) []projectView {
	out := make([]projectView, 0, len(projects))
	for _, p := range projects {
		out = append(out, toProjectView(p))
	}
	return out
}

func toNodeView(n graph.Node) nodeView {
	return nodeView{
		ID:          n.ID,
		ProjectID:   n.ProjectID,
		Kind:        n.Kind,
		Type:        n.Type,
		Content:     n.Content,
		State:       n.State,
		From:        n.From,
		To:          n.To,
		Creator:     n.Creator,
		Worker:      n.Worker,
		LeaseUntil:  n.LeaseUntil,
		CreatedAt:   n.CreatedAt,
		ConcludedAt: n.ConcludedAt,
	}
}

func toNodeViews(nodes []graph.Node) []nodeView {
	out := make([]nodeView, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, toNodeView(n))
	}
	return out
}

// --- error mapping ---------------------------------------------------------

// respondError maps graph sentinel errors onto protocol status codes.
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, graph.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, graph.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, graph.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, graph.ErrInvalid), errors.Is(err, graph.ErrCycle):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func invalidBody(c *gin.Context, err error) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "graph: invalid request body: " + err.Error()})
}

func leaseFromSeconds(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultLease
	}
	return time.Duration(seconds) * time.Second
}

func intentContent(req intentRequest) string {
	if strings.TrimSpace(req.Description) != "" {
		return req.Description
	}
	return req.Content
}

// --- project handlers ------------------------------------------------------

func (s *Server) createProject(c *gin.Context) {
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	// The protocol defaults bootstrap_enabled to true when the field is absent.
	bootstrap := true
	if req.BootstrapEnabled != nil {
		bootstrap = *req.BootstrapEnabled
	}
	p, err := s.store.CreateProject(c.Request.Context(), graph.Project{
		ID:               req.ID,
		Title:            req.Title,
		Origin:           req.Origin,
		Goal:             req.Goal,
		BootstrapEnabled: bootstrap,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toProjectView(p))
}

func (s *Server) listProjects(c *gin.Context) {
	projects, err := s.store.ListProjects(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectViews(projects))
}

func (s *Server) getProject(c *gin.Context) {
	p, err := s.store.GetProject(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectView(p))
}

func (s *Server) setProjectStatus(c *gin.Context) {
	var req projectStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	p, err := s.store.SetProjectStatus(c.Request.Context(), c.Param("id"), req.Status)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectView(p))
}

func (s *Server) reopen(c *gin.Context) {
	var req reopenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	p, err := s.store.Reopen(c.Request.Context(), c.Param("id"), req.Creator, req.Description)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectView(p))
}

func (s *Server) addHint(c *gin.Context) {
	var req hintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	hint, err := s.store.AddHint(c.Request.Context(), c.Param("id"), req.Content, req.Creator)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toNodeView(hint))
}

// --- intent handlers -------------------------------------------------------

func (s *Server) declareIntent(c *gin.Context) {
	var req intentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	intent, err := s.store.DeclareIntent(c.Request.Context(), c.Param("id"), graph.Node{
		Type:    req.Type,
		Content: intentContent(req),
		From:    req.From,
		Creator: req.Creator,
		Worker:  req.Worker,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toNodeView(intent))
}

func (s *Server) heartbeatIntent(c *gin.Context) {
	var req workerLeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	intent, err := s.store.HeartbeatIntent(c.Request.Context(), c.Param("id"), c.Param("iid"),
		req.Worker, leaseFromSeconds(req.LeaseSeconds))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toNodeView(intent))
}

func (s *Server) releaseIntent(c *gin.Context) {
	var req releaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	intent, err := s.store.ReleaseIntent(c.Request.Context(), c.Param("id"), c.Param("iid"), req.Worker)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toNodeView(intent))
}

func (s *Server) concludeIntent(c *gin.Context) {
	var req concludeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	factType := req.FactType
	if strings.TrimSpace(factType) == "" {
		factType = req.Type
	}
	fact, intent, err := s.store.ConcludeIntent(c.Request.Context(), c.Param("id"), c.Param("iid"),
		req.Worker, req.Description, factType)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"fact": toNodeView(fact), "intent": toNodeView(intent)})
}

// --- reason handlers -------------------------------------------------------

func (s *Server) claimReason(c *gin.Context) {
	var req workerLeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	p, err := s.store.ClaimReason(c.Request.Context(), c.Param("id"), req.Worker, req.Trigger,
		leaseFromSeconds(req.LeaseSeconds))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectView(p))
}

func (s *Server) heartbeatReason(c *gin.Context) {
	var req workerLeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	p, err := s.store.HeartbeatReason(c.Request.Context(), c.Param("id"), req.Worker,
		leaseFromSeconds(req.LeaseSeconds))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectView(p))
}

func (s *Server) releaseReason(c *gin.Context) {
	var req releaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	p, err := s.store.ReleaseReason(c.Request.Context(), c.Param("id"), req.Worker)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectView(p))
}

// --- completion, export and query handlers ---------------------------------

func (s *Server) complete(c *gin.Context) {
	var req completeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidBody(c, err)
		return
	}
	p, err := s.store.Complete(c.Request.Context(), c.Param("id"), req.From, req.Description, req.Worker)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectView(p))
}

func (s *Server) export(c *gin.Context) {
	format := c.DefaultQuery("format", "yaml")
	var (
		out         string
		contentType string
		err         error
	)
	switch format {
	case "yaml":
		out, err = s.store.ExportYAML(c.Request.Context(), c.Param("id"))
		contentType = "application/x-yaml; charset=utf-8"
	case "timeline":
		out, err = s.store.ExportTimeline(c.Request.Context(), c.Param("id"))
		contentType = "text/plain; charset=utf-8"
	default:
		respondError(c, graph.ErrInvalid)
		return
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.Data(http.StatusOK, contentType, []byte(out))
}

// snapshot returns the whole project graph in one read for the read-only UI
// panel: project metadata, nodes, edges and the plain-text timeline.
func (s *Server) snapshot(c *gin.Context) {
	ctx := c.Request.Context()
	projectID := c.Param("id")
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		respondError(c, err)
		return
	}
	nodes, err := s.store.Nodes(ctx, projectID)
	if err != nil {
		respondError(c, err)
		return
	}
	edges, err := s.store.Edges(ctx, projectID)
	if err != nil {
		respondError(c, err)
		return
	}
	timeline, err := s.store.ExportTimeline(ctx, projectID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result": snapshotView{
			Project:  toProjectView(p),
			Nodes:    toNodeViews(nodes),
			Edges:    toEdgeViews(edges),
			Timeline: timeline,
		},
	})
}

func (s *Server) provenance(c *gin.Context) {
	nodeID := strings.TrimSpace(c.Query("node"))
	if nodeID == "" {
		respondError(c, graph.ErrInvalid)
		return
	}
	chain, err := s.store.Provenance(c.Request.Context(), c.Param("id"), nodeID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toNodeViews(chain))
}
