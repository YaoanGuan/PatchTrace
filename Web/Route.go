package Web

import (
	"AIxVuln/DecisionBrain"
	"AIxVuln/LabManager"
	"AIxVuln/agents"
	binaryagents "AIxVuln/agents/binary"
	pentestagents "AIxVuln/agents/pentest"
	webagents "AIxVuln/agents/web"
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/skill"
	"AIxVuln/taskManager"
	"AIxVuln/toolCalling"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type llmTestRequest struct {
	Section   string `json:"section"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	APIMode   string `json:"api_mode"`
	UserAgent string `json:"user_agent"`
	Model     string `json:"model"`
}

type llmTestResult struct {
	OK      bool   `json:"ok"`
	Backend string `json:"backend,omitempty"`
	Reply   string `json:"reply,omitempty"`
	Message string `json:"message,omitempty"`
	Model   string `json:"model,omitempty"`
	Section string `json:"section,omitempty"`
}

func (s *Server) getProjectByParam(c *gin.Context) (*LabManager.LabManager, string, bool) {
	return s.getProjectByIDOrName(c.Param("name"))
}

func (s *Server) getProjectByIDOrName(raw string) (*LabManager.LabManager, string, bool) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return nil, "", false
	}
	s.mu.RLock()
	if pm, exists := s.pms[key]; exists {
		s.mu.RUnlock()
		return pm, key, true
	}
	var matched *LabManager.LabManager
	var matchedID string
	for id, pm := range s.pms {
		if pm == nil || pm.GetProjectName() != key {
			continue
		}
		if matched != nil {
			s.mu.RUnlock()
			return nil, "", false
		}
		matched = pm
		matchedID = id
	}
	s.mu.RUnlock()
	if matched == nil {
		return nil, "", false
	}
	return matched, matchedID, true
}

func (s *Server) getPms(c *gin.Context) {
	s.waitForProjectRestore(c.Request.Context())
	res := make([]map[string]interface{}, 0)
	s.mu.RLock()
	ids := make([]string, 0, len(s.pms))
	projectByID := make(map[string]*LabManager.LabManager, len(s.pms))
	for id, pm := range s.pms {
		ids = append(ids, id)
		projectByID[id] = pm
	}
	s.mu.RUnlock()
	sort.Slice(ids, func(i, j int) bool {
		left := projectByID[ids[i]]
		right := projectByID[ids[j]]
		if left == nil || right == nil {
			return ids[i] < ids[j]
		}
		if left.GetProjectName() == right.GetProjectName() {
			return ids[i] < ids[j]
		}
		return left.GetProjectName() < right.GetProjectName()
	})
	for _, id := range ids {
		pm := projectByID[id]
		if pm == nil {
			continue
		}
		name := pm.GetProjectName()
		rawStatus := pm.GetStatus()
		isRunning := pm.IsRunning()
		queued, queuePos, starting := s.getProjectStartQueueState(id)
		status := rawStatus
		if queued && !isRunning {
			if queuePos > 0 {
				status = fmt.Sprintf("排队中（第%d位）", queuePos)
			} else {
				status = "排队中"
			}
		} else if starting && !isRunning {
			status = "启动中"
		}
		res = append(res, map[string]interface{}{
			"projectID":     id,
			"project_id":    id,
			"projectUUID":   id,
			"project_uuid":  id,
			"projectName":   name,
			"labName":       name,
			"status":        status,
			"rawStatus":     rawStatus,
			"isRunning":     isRunning,
			"isStarting":    starting,
			"isQueued":      queued,
			"queuePosition": queuePos,
			"brainFinished": pm.GetBrainFinished(),
			"groupCount":    pm.GetGroupCount(),
			"startTime":     pm.GetStartTime(),
			"endTime":       pm.GetEndTime(),
		})
	}
	c.JSON(200, Success(res))
}

func (s *Server) containerList(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	var cl []Container
	for _, co := range pm.GetContainerList() {
		if len(co.WebPort) > 0 {
			cl = append(cl, Container{ContainerId: co.ContainerId, ContainerIP: co.ContainerIP, Image: co.Image, WebPort: co.WebPort})
		} else {
			cl = append(cl, Container{ContainerId: co.ContainerId, ContainerIP: co.ContainerIP, Image: co.Image, WebPort: nil})
		}

	}
	c.JSON(200, Success(cl))
}

func (s *Server) agentList(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	c.JSON(200, Success(pm.GetDigitalHumanRoster()))
}

func (s *Server) exploitIdeaList(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	c.JSON(200, Success(pm.GetExploitIdeaList()))
}

func (s *Server) exploitChainList(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	c.JSON(200, Success(pm.GetExploitChainList()))
}

func (s *Server) vulnList(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	res := make(map[string]any)
	res["exploitIdeaList"] = pm.GetExploitIdeaList()
	res["exploitChainList"] = pm.GetExploitChainList()
	c.JSON(200, Success(res))
}

func (s *Server) eventList(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	count := c.DefaultQuery("count", "50")
	countInt, err := strconv.Atoi(count)
	if err != nil {
		c.JSON(400, Fail("count not int"))
		return
	}
	c.JSON(200, Success(pm.GetEvent(countInt)))
}

func (s *Server) reportList(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	report := make(map[string]string)
	for k, v := range pm.GetReportList() {
		report[k] = filepath.Base(v)
	}
	c.JSON(200, Success(report))
}

func (s *Server) downloadReport(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	id := c.Param("id")
	p, fileName, ok := pm.ResolveSharedReportPath(id)
	if !ok {
		c.JSON(404, Fail("report not found"))
		return
	}
	_, err := os.Stat(p)
	if err != nil {
		c.JSON(404, Fail("file not found"))
		return
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = filepath.Base(p)
	}
	c.Header("Content-Disposition", attachmentDisposition(fileName))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Transfer-Encoding", "binary")
	c.File(p)
}

func (s *Server) downloadReportAll(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	tempDir, err := filepath.Abs(filepath.Join(misc.GetDataDir(), "temp"))
	if err != nil {
		c.JSON(400, Fail("get temp zipfile error"))
		return
	}
	stagingDir := filepath.Join(tempDir, uuid.New().String())
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		c.JSON(400, Fail("prepare temp directory error"))
		return
	}
	defer os.RemoveAll(stagingDir)
	reportEntries := pm.SharedReportEntries()
	if len(reportEntries) == 0 {
		c.JSON(404, Fail("report not found"))
		return
	}
	countByName := make(map[string]int)
	for _, entry := range reportEntries {
		src := strings.TrimSpace(entry.Meta.Path)
		if src == "" {
			continue
		}
		if _, err := os.Stat(src); err != nil {
			continue
		}
		groupDir := filepath.Join(stagingDir, strings.TrimSpace(entry.GroupID))
		if err := os.MkdirAll(groupDir, 0755); err != nil {
			continue
		}
		fileName := strings.TrimSpace(entry.Meta.FileName)
		if fileName == "" {
			fileName = filepath.Base(src)
		}
		baseName := strings.TrimSpace(entry.Meta.ReportID)
		if baseName == "" {
			baseName = "report"
		}
		targetName := baseName + "-" + fileName
		if n := countByName[targetName]; n > 0 {
			targetName = fmt.Sprintf("%s-%d", baseName, n) + "-" + fileName
		}
		countByName[baseName+"-"+fileName]++
		if err := misc.CopyFile(src, filepath.Join(groupDir, targetName)); err != nil {
			continue
		}
	}
	tempZipfile := filepath.Join(tempDir, uuid.New().String()+".zip")
	err = misc.ZipDirectory(stagingDir, tempZipfile)
	if err != nil {
		c.JSON(400, Fail("zip file error"))
		return
	}
	defer os.Remove(tempZipfile)
	c.Header("Content-Disposition", attachmentDisposition(pm.GetProjectName()+"-report.zip"))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Transfer-Encoding", "binary")
	c.File(tempZipfile)
}

func (s *Server) generateReportSummary(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	result, err := pm.GenerateSharedReportSummary()
	if err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	c.JSON(200, Success(result))
}

func (s *Server) teamChat(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	var body struct {
		Message   string `json:"message"`
		ChannelID string `json:"channel_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Message == "" {
		c.JSON(400, Fail("message is required"))
		return
	}
	wasFinished := pm.GetBrainFinished()
	result := pm.HandleUserChat(body.Message, body.ChannelID)
	if wasFinished && pm.NeedsQueuedRestartAfterChat(body.ChannelID) {
		state, pos := s.enqueueProjectStart(pm.GetProjectID())
		queueMsg := formatQueueStartMessage(state, pos)
		if strings.TrimSpace(result) == "" {
			result = queueMsg
		} else {
			result = result + "；" + queueMsg
		}
	}
	c.JSON(200, Success(result))
}

func (s *Server) getChatMessages(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	msgs := pm.GetChatMessages()
	if msgs == nil {
		msgs = make([]DecisionBrain.ChatMessage, 0)
	}
	c.JSON(200, Success(msgs))
}

func (s *Server) decideHostCommandApproval(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	var body struct {
		Approved bool   `json:"approved"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, Fail("invalid decision payload"))
		return
	}
	if err := pm.DecideHostCommandApproval(c.Param("id"), body.Approved, body.Note, "用户"); err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	if body.Approved {
		c.JSON(200, Success("已批准该决策事项"))
		return
	}
	c.JSON(200, Success("已拒绝该决策事项"))
}

func (s *Server) listBackgroundProcesses(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	c.JSON(200, Success(pm.GetBackgroundProcessSnapshot()))
}

func (s *Server) killBackgroundProcess(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	snapshot, err := pm.KillBackgroundProcess(c.Param("id"))
	if err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	c.JSON(200, Success(snapshot))
}

func (s *Server) exportDebugMemory(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	targetID := strings.TrimSpace(c.Query("target_id"))
	if targetID == "" {
		c.JSON(400, Fail("target_id is required"))
		return
	}
	exported, err := pm.ExportMemoryDebug(targetID)
	if err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	fileName := buildMemoryDebugFileName(pm.GetProjectName(), exported.Target)
	c.Header("Content-Disposition", attachmentDisposition(fileName))
	c.Data(200, "text/plain; charset=utf-8", []byte(exported.Content))
}

func (s *Server) getTokenUsage(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	c.JSON(200, Success(pm.GetTokenUsage()))
}

func (s *Server) getContextBreakdown(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	c.JSON(200, Success(pm.GetContextBreakdown()))
}

// sandboxContainerID reports the container id registered for a lab, or "" when
// no sandbox exists. The graph lane's explore task resolves sandbox evidence
// through taskManager.GetSandbox(graphProjectID), so operators use this field
// to confirm the graph project id matches a started lab before expecting a
// sandbox-backed verdict.
func sandboxContainerID(projectID string) string {
	sandbox, err := taskManager.GetSandbox(projectID)
	if err != nil || sandbox == nil {
		return ""
	}
	return strings.TrimSpace(sandbox.ContainerId)
}

func (s *Server) getProject(c *gin.Context) {
	s.waitForProjectRestore(c.Request.Context())
	res := make(map[string]any)
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	res["projectName"] = pm.GetProjectName()
	res["labName"] = pm.GetLabName()
	res["eventLog"] = pm.GetEvent(50)
	res["groups"] = pm.GetGroupsSnapshot()
	res["controlBrain"] = pm.GetLabBrainSnapshot()
	rawStatus := pm.GetStatus()
	res["status"] = rawStatus
	res["rawStatus"] = rawStatus
	// "决策结束" is treated as paused and should not be considered actively running.
	res["isRunning"] = pm.IsRunning()
	projectID := pm.GetProjectID()
	res["projectID"] = projectID
	res["project_id"] = projectID
	res["projectUUID"] = projectID
	res["project_uuid"] = projectID
	queued, queuePos, starting := s.getProjectStartQueueState(projectID)
	res["isQueued"] = queued
	res["isStarting"] = starting
	res["queuePosition"] = queuePos
	if queued && !res["isRunning"].(bool) {
		if queuePos > 0 {
			res["status"] = fmt.Sprintf("排队中（第%d位）", queuePos)
		} else {
			res["status"] = "排队中"
		}
	} else if starting && !res["isRunning"].(bool) {
		res["status"] = "启动中"
	}
	res["brainFinished"] = pm.GetBrainFinished()
	res["startTime"] = pm.GetStartTime()
	res["endTime"] = pm.GetEndTime()
	// sandboxContainerID lets graph-lane operators verify the preflight mapping
	// explore depends on: taskManager.GetSandbox(labID) must resolve to a live
	// container before graph projects can claim sandbox evidence. Empty means
	// no sandbox is registered for this lab (not started, or already stopped).
	res["sandboxContainerID"] = sandboxContainerID(projectID)
	res["sharedResources"] = pm.GetSharedResourcesSnapshot()
	res["hostCommandApprovals"] = pm.GetHostCommandApprovalSnapshot()
	res["backgroundProcesses"] = pm.GetBackgroundProcessSnapshot()
	c.JSON(200, Success(res))
}

func (s *Server) getProjectGroupDetail(c *gin.Context) {
	s.waitForProjectRestore(c.Request.Context())
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	groupID := strings.TrimSpace(c.Param("group_id"))
	if groupID == "" {
		c.JSON(400, Fail("group_id is required"))
		return
	}
	detail, ok := pm.GetGroupPrivateSnapshot(groupID)
	if !ok {
		c.JSON(404, Fail("group not found"))
		return
	}
	c.JSON(200, Success(detail))
}

func (s *Server) getProjectMemoryDebugTargets(c *gin.Context) {
	s.waitForProjectRestore(c.Request.Context())
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	c.JSON(200, Success(pm.GetMemoryDebugTargets()))
}

func buildMemoryDebugFileName(projectName string, target taskManager.MemoryDebugTarget) string {
	parts := make([]string, 0, 4)
	if trimmed := strings.TrimSpace(projectName); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(target.GroupID); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(target.TargetType); trimmed != "" {
		parts = append(parts, trimmed)
	}
	name := strings.TrimSpace(target.Label)
	if name == "" {
		name = strings.TrimSpace(target.TargetID)
	}
	if name != "" {
		parts = append(parts, sanitizeMemoryDebugFileNamePart(name))
	}
	if len(parts) == 0 {
		return "memory-debug.txt"
	}
	return strings.Join(parts, "-") + ".txt"
}

func sanitizeMemoryDebugFileNamePart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(raw)
}
func (s *Server) createProject(c *gin.Context) {
	// Check required config before creating project.
	missing := misc.CheckRequiredConfig()
	misc.Debug("createProject: CheckRequiredConfig => missing=%v", missing)
	if len(missing) > 0 {
		c.JSON(400, Fail("请先在「设置」中配置必填项: "+strings.Join(missing, ", ")))
		return
	}

	projectName := c.DefaultQuery("projectName", "project-"+uuid.New().String())
	if err := validateProjectName(projectName); err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}

	taskContent := c.PostForm("taskContent")
	sourceType := c.DefaultPostForm("source_type", "file") // file | git | url | empty

	absPath, err := filepath.Abs(misc.GetDataDir())
	if err != nil {
		c.JSON(500, Fail("获取DataDir"))
		return
	}
	tempDir := filepath.Join(absPath, "temp", uuid.New().String())
	defer os.RemoveAll(tempDir)

	switch sourceType {
	case "file":
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(400, Fail("请上传源码压缩包: "+err.Error()))
			return
		}
		ext := filepath.Ext(file.Filename)
		tempFile, err := os.CreateTemp("", "upload-*"+ext)
		if err != nil {
			c.JSON(500, Fail("创建临时文件失败"))
			return
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()
		src, err := file.Open()
		if err != nil {
			c.JSON(500, Fail("打开上传文件失败"))
			return
		}
		defer src.Close()
		if _, err := io.Copy(tempFile, src); err != nil {
			c.JSON(500, Fail("保存上传文件失败"))
			return
		}
		if err := UncompressFile(tempFile.Name(), tempDir); err != nil {
			c.JSON(500, Fail("解压失败: "+err.Error()))
			return
		}

	case "git":
		gitURL := c.PostForm("git_url")
		if gitURL == "" {
			c.JSON(400, Fail("请提供 Git 仓库地址"))
			return
		}
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			c.JSON(500, Fail("创建临时目录失败"))
			return
		}
		cmd := exec.CommandContext(c.Request.Context(), "git", "clone", "--depth", "1", gitURL, tempDir)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		output, err := cmd.CombinedOutput()
		if err != nil {
			c.JSON(500, Fail("Git clone 失败: "+err.Error()+"\n"+string(output)))
			return
		}

	case "url":
		fileURL := c.PostForm("file_url")
		if fileURL == "" {
			c.JSON(400, Fail("请提供压缩包下载链接"))
			return
		}
		resp, err := http.Get(fileURL)
		if err != nil {
			c.JSON(500, Fail("下载失败: "+err.Error()))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			c.JSON(500, Fail("下载失败: HTTP "+strconv.Itoa(resp.StatusCode)))
			return
		}
		// Determine extension from URL or Content-Type
		ext := filepath.Ext(fileURL)
		if ext == "" || len(ext) > 6 {
			ext = ".zip"
		}
		tempFile, err := os.CreateTemp("", "download-*"+ext)
		if err != nil {
			c.JSON(500, Fail("创建临时文件失败"))
			return
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()
		if _, err := io.Copy(tempFile, resp.Body); err != nil {
			c.JSON(500, Fail("保存下载文件失败"))
			return
		}
		if err := UncompressFile(tempFile.Name(), tempDir); err != nil {
			c.JSON(500, Fail("解压失败: "+err.Error()))
			return
		}

	case "empty":
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			c.JSON(500, Fail("创建空工作目录失败"))
			return
		}

	default:
		c.JSON(400, Fail("不支持的 source_type: "+sourceType))
		return
	}

	flattenSingleRootDirectory(tempDir)
	skipProjectOverview := false
	if sourceType == "empty" {
		skipProjectOverview = misc.GetEmptyProjectSkipProjectOverview()
	}
	if err := s.createProjectFromSourceDir(projectName, taskContent, tempDir, skipProjectOverview); err != nil {
		c.JSON(500, Fail(err.Error()))
		return
	}
	c.JSON(200, Success("成功新建项目"))
}

func (s *Server) startProject(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	if pm.IsRunning() {
		c.JSON(400, Fail("实验室正在运行中，无需重复启动"))
		return
	}
	if err := validateLabStartPrerequisites(); err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	state, pos := s.enqueueProjectStart(pm.GetProjectID())
	c.JSON(200, Success(formatQueueStartMessage(state, pos)))
}
func (s *Server) cancelProject(c *gin.Context) {
	pm, projectID, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	if removed, pos := s.removeProjectFromStartQueue(projectID); removed {
		if pos > 0 {
			c.JSON(200, Success(fmt.Sprintf("项目已从启动队列移除（原排队第%d位）", pos)))
		} else {
			c.JSON(200, Success("项目已从启动队列移除"))
		}
		return
	}
	go pm.StopTask()
	c.JSON(200, Success("项目已经取消"))
}
func (s *Server) getEnvInfo(c *gin.Context) {
	pm, _, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	c.JSON(200, Success(pm.GetEnvInfo()))
}

func removeManagedProjectDir(projectID string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return fmt.Errorf("project id is empty")
	}
	projectsRoot, err := filepath.Abs(filepath.Join(misc.GetDataDir(), "projects"))
	if err != nil {
		return err
	}
	targetDir, err := filepath.Abs(filepath.Join(projectsRoot, projectID))
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(projectsRoot, targetDir)
	if err != nil {
		return err
	}
	if rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("refusing to delete unsafe project dir: %s", targetDir)
	}
	info, err := os.Stat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("project path is not a directory: %s", targetDir)
	}
	return os.RemoveAll(targetDir)
}

func (s *Server) delProject(c *gin.Context) {
	// 如果项目还在启动队列中，先移除，避免后续被调度。
	pm, projectID, exists := s.getProjectByParam(c)
	if !exists {
		c.JSON(404, Fail("project not found"))
		return
	}
	_, _ = s.removeProjectFromStartQueue(projectID)
	pm.StopTask()
	pm.RemoveDockerAll()
	if err := removeManagedProjectDir(projectID); err != nil {
		c.JSON(500, Fail("delete project directory failed: "+err.Error()))
		return
	}
	s.mu.Lock()
	delete(s.pms, projectID)
	s.mu.Unlock()
	s.SaveProjectManagerToFile()
	c.JSON(200, Success("删除成功"))
}

func (s *Server) listModels(c *gin.Context) {
	baseURL := c.Query("base_url")
	apiKey := c.Query("api_key")
	apiMode := c.Query("api_mode")
	if baseURL == "" || apiKey == "" {
		c.JSON(400, Fail("base_url and api_key are required"))
		return
	}
	// Trim trailing slash
	baseURL = regexp.MustCompile(`/+$`).ReplaceAllString(baseURL, "")
	backend := llm.ResolveLLMBackend(apiMode, baseURL)
	url := baseURL + "/models"
	if backend == llm.BackendAnthropic {
		url = llm.AnthropicModelsURL(baseURL)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(500, Fail("request error: "+err.Error()))
		return
	}
	if backend == llm.BackendAnthropic {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("User-Agent", misc.GetConfigValueDefault("main_setting", "USER_AGENT", "AIxVuln"))

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(502, Fail("upstream error: "+err.Error()))
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Parse model-list responses from OpenAI-compatible and Anthropic-style APIs.
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	var models []string
	if err := json.Unmarshal(body, &parsed); err == nil && len(parsed.Data) > 0 {
		for _, m := range parsed.Data {
			if m.ID != "" {
				models = append(models, m.ID)
			}
		}
	}
	c.JSON(200, Success(models))
}

func (s *Server) testLLM(c *gin.Context) {
	var body llmTestRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, Fail("invalid JSON: "+err.Error()))
		return
	}
	body.Section = strings.TrimSpace(body.Section)
	body.BaseURL = strings.TrimSpace(body.BaseURL)
	body.APIKey = strings.TrimSpace(body.APIKey)
	body.APIMode = strings.TrimSpace(body.APIMode)
	body.UserAgent = strings.TrimSpace(body.UserAgent)
	body.Model = strings.TrimSpace(body.Model)

	backend := llm.ResolveLLMBackend(body.APIMode, body.BaseURL)
	result := llmTestResult{
		OK:      false,
		Backend: backend,
		Model:   body.Model,
		Section: body.Section,
	}
	if body.BaseURL == "" {
		result.Message = "missing BASE_URL"
		c.JSON(200, Success(result))
		return
	}
	if body.APIKey == "" {
		result.Message = "missing OPENAI_API_KEY"
		c.JSON(200, Success(result))
		return
	}
	if body.Model == "" {
		result.Message = "missing MODEL"
		c.JSON(200, Success(result))
		return
	}

	cli, backend, err := llm.NewAdHocClient(body.BaseURL, body.APIKey, body.APIMode, body.UserAgent, 1)
	result.Backend = backend
	if err != nil {
		result.Message = err.Error()
		c.JSON(200, Success(result))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	resp, err := llm.RequestLLM(cli, ctx, body.Model, []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a connectivity test assistant. Reply with exactly: hello"},
		{Role: llm.RoleUser, Content: "Say hello."},
	}, nil)
	if err != nil {
		result.Message = err.Error()
		c.JSON(200, Success(result))
		return
	}
	result.OK = true
	result.Message = "连接成功"
	result.Reply = strings.TrimSpace(resp.Content)
	if result.Reply == "" {
		result.Reply = "(empty response)"
	}
	c.JSON(200, Success(result))
}

func (s *Server) getConfig(c *gin.Context) {
	c.JSON(200, Success(misc.GetAllConfig()))
}

func (s *Server) setConfig(c *gin.Context) {
	var body map[string]map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, Fail("invalid JSON: "+err.Error()))
		return
	}
	if err := misc.SetAllConfig(body); err != nil {
		c.JSON(500, Fail("save config failed: "+err.Error()))
		return
	}
	misc.ReloadDebugFlag()
	// Re-evaluate queued project scheduling in case concurrency config changed.
	s.notifyStartQueue()
	c.JSON(200, Success("配置已保存，部分配置需要重启后生效"))
}

func validateLabStartPrerequisites() error {
	missing := misc.CheckRequiredConfig()
	if len(missing) > 0 {
		return fmt.Errorf("请先在「设置」中配置必填项: %s", strings.Join(missing, ", "))
	}
	if err := toolCalling.ValidateRequiredMCPRequirements(); err != nil {
		return err
	}
	return nil
}

func allMCPAgentDefinitions() []agents.AgentDefinition {
	seen := make(map[string]struct{})
	out := make([]agents.AgentDefinition, 0)
	appendDefs := func(defs []agents.AgentDefinition) {
		for _, def := range defs {
			if strings.TrimSpace(def.Name) == "" {
				continue
			}
			if _, ok := seen[def.Name]; ok {
				continue
			}
			seen[def.Name] = struct{}{}
			out = append(out, def)
		}
	}
	appendDefs([]agents.AgentDefinition{agents.GeneralAgentDefinition()})
	if defs, _ := webagents.GetAgentDescription(); len(defs) > 0 {
		appendDefs(defs)
	}
	if defs, _ := binaryagents.GetAgentDescription(); len(defs) > 0 {
		appendDefs(defs)
	}
	if defs, _ := pentestagents.GetAgentDescription(); len(defs) > 0 {
		appendDefs(defs)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func knownMCPAgentTypeSet() map[string]struct{} {
	defs := allMCPAgentDefinitions()
	out := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		out[def.Name] = struct{}{}
	}
	return out
}

func buildMCPAgentTargets() []map[string]interface{} {
	defs := allMCPAgentDefinitions()
	peopleByType := misc.GetAllDigitalHumans()
	out := make([]map[string]interface{}, 0, len(defs))
	for _, def := range defs {
		humans := peopleByType[def.Name]
		items := make([]map[string]interface{}, 0, len(humans))
		for _, human := range humans {
			items = append(items, map[string]interface{}{
				"id":           human.ID,
				"persona_name": human.PersonaName,
				"avatar_file":  human.AvatarFile,
			})
		}
		out = append(out, map[string]interface{}{
			"agent_type":     def.Name,
			"agent_name":     shortAgentName(def.Name),
			"description":    def.Description,
			"digital_humans": items,
			"digital_count":  len(items),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return fmt.Sprintf("%v", out[i]["agent_type"]) < fmt.Sprintf("%v", out[j]["agent_type"])
	})
	return out
}

func shortAgentName(agentType string) string {
	parts := strings.Split(strings.TrimSpace(agentType), "-")
	if len(parts) == 0 {
		return agentType
	}
	return parts[len(parts)-1]
}

func (s *Server) getMCPSettings(c *gin.Context) {
	c.JSON(200, Success(map[string]interface{}{
		"configs":       misc.GetMCPConfigs(),
		"store_path":    misc.ResolveMCPConfigStorePath(),
		"agent_targets": buildMCPAgentTargets(),
	}))
}

func (s *Server) setMCPSettings(c *gin.Context) {
	var body struct {
		Configs []misc.MCPConfigRow `json:"configs"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, Fail("invalid JSON: "+err.Error()))
		return
	}
	knownAgents := knownMCPAgentTypeSet()
	requiredByID := make(map[string]misc.RequiredMCPBindingSpec)
	requiredByName := make(map[string]misc.RequiredMCPBindingSpec)
	for _, spec := range misc.BuiltinMCPBindings() {
		requiredByID[spec.ID] = spec
		requiredByName[strings.ToLower(strings.TrimSpace(spec.Name))] = spec
	}
	seenIDs := make(map[string]struct{}, len(body.Configs))
	for i := range body.Configs {
		row := misc.NormalizeMCPConfigForRuntime(body.Configs[i])
		requiredSpec, isRequiredBuiltin := requiredByID[row.ID]
		if !isRequiredBuiltin {
			requiredSpec, isRequiredBuiltin = requiredByName[strings.ToLower(strings.TrimSpace(row.Name))]
		}
		if isRequiredBuiltin {
			row.ID = requiredSpec.ID
			row.Name = requiredSpec.Name
			row.ToolNamePrefix = requiredSpec.ToolNamePrefix
			bound := false
			for _, binding := range row.AgentBindings {
				if binding.AgentType == requiredSpec.AgentType {
					bound = true
					break
				}
			}
			if !bound {
				row.AgentBindings = append(row.AgentBindings, misc.MCPAgentBinding{AgentType: requiredSpec.AgentType})
				row = misc.NormalizeMCPConfigForRuntime(row)
			}
		}
		if strings.TrimSpace(row.Name) == "" {
			c.JSON(400, Fail(fmt.Sprintf("第 %d 个 MCP 缺少名称", i+1)))
			return
		}
		if _, ok := seenIDs[row.ID]; ok {
			c.JSON(400, Fail(fmt.Sprintf("MCP ID 重复: %s", row.ID)))
			return
		}
		seenIDs[row.ID] = struct{}{}
		switch row.Transport {
		case "stdio":
			if strings.TrimSpace(row.Command) == "" && (!isRequiredBuiltin || row.Enabled) {
				c.JSON(400, Fail(fmt.Sprintf("%s 缺少 Command", row.Name)))
				return
			}
		case "streamable", "sse":
			if strings.TrimSpace(row.URL) == "" && (!isRequiredBuiltin || row.Enabled) {
				c.JSON(400, Fail(fmt.Sprintf("%s 缺少 URL", row.Name)))
				return
			}
		default:
			c.JSON(400, Fail(fmt.Sprintf("%s 使用了不支持的 transport: %s", row.Name, row.Transport)))
			return
		}
		filteredBindings := make([]misc.MCPAgentBinding, 0, len(row.AgentBindings))
		for _, binding := range row.AgentBindings {
			agentType := strings.TrimSpace(binding.AgentType)
			if _, ok := knownAgents[agentType]; ok {
				filteredBindings = append(filteredBindings, binding)
			}
		}
		row.AgentBindings = filteredBindings
		row = misc.NormalizeMCPConfigForRuntime(row)
		body.Configs[i] = row
	}
	if err := misc.SaveAllMCPConfigs(body.Configs); err != nil {
		c.JSON(500, Fail("save MCP settings failed: "+err.Error()))
		return
	}
	toolCalling.ReloadMCPRegistryFromStore()
	c.JSON(200, Success("MCP 配置已保存到 "+misc.ResolveMCPConfigStorePath()+"，新启动的数字人会使用最新绑定"))
}

func (s *Server) testMCPSettings(c *gin.Context) {
	var body misc.MCPConfigRow
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, Fail("invalid JSON: "+err.Error()))
		return
	}
	result := toolCalling.TestMCPConfig(body)
	c.JSON(200, Success(result))
}

func (s *Server) getSkillSettings(c *gin.Context) {
	result, err := skill.GetSettingsSnapshot()
	if err != nil {
		c.JSON(500, Fail("load skills failed: "+err.Error()))
		return
	}
	c.JSON(200, Success(result))
}

func (s *Server) setSkillSettings(c *gin.Context) {
	var body struct {
		SearchPaths string `json:"search_paths"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, Fail("invalid JSON: "+err.Error()))
		return
	}
	if err := skill.SaveSearchPaths(body.SearchPaths); err != nil {
		c.JSON(500, Fail("save skills settings failed: "+err.Error()))
		return
	}
	c.JSON(200, Success("Skills 搜索路径已保存"))
}

func (s *Server) createSkill(c *gin.Context) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, Fail("invalid JSON: "+err.Error()))
		return
	}
	item, err := skill.CreateSkill(body.Name, body.Description)
	if err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	c.JSON(200, Success(item))
}

func (s *Server) deleteSkill(c *gin.Context) {
	name := c.Param("name")
	if err := skill.DeleteSkill(name); err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	c.JSON(200, Success("Skill 已删除"))
}

func (s *Server) listSkillFiles(c *gin.Context) {
	name := c.Param("name")
	relativePath := c.Query("path")
	result, err := skill.ListFiles(name, relativePath, 5)
	if err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	c.JSON(200, Success(result))
}

func (s *Server) readSkillFile(c *gin.Context) {
	name := c.Param("name")
	relativePath := c.Query("path")
	result, err := skill.ReadDocument(name, relativePath)
	if err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	c.JSON(200, Success(result))
}

func (s *Server) saveSkillFile(c *gin.Context) {
	name := c.Param("name")
	var body struct {
		RelativePath string `json:"relative_path"`
		Content      string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, Fail("invalid JSON: "+err.Error()))
		return
	}
	result, err := skill.SaveDocument(name, body.RelativePath, body.Content)
	if err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	c.JSON(200, Success(result))
}

func (s *Server) deleteSkillFile(c *gin.Context) {
	name := c.Param("name")
	relativePath := c.Query("path")
	if err := skill.DeleteDocument(name, relativePath); err != nil {
		c.JSON(400, Fail(err.Error()))
		return
	}
	c.JSON(200, Success("Skill 文件已删除"))
}

func normalizeStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func (s *Server) getDigitalHumans(c *gin.Context) {
	c.JSON(200, Success(misc.GetDigitalHumansList()))
}

func (s *Server) saveDigitalHuman(c *gin.Context) {
	var body misc.DigitalHumanRow
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, Fail("invalid JSON: "+err.Error()))
		return
	}
	if body.ID == "" || body.AgentType == "" || body.PersonaName == "" {
		c.JSON(400, Fail("id, agent_type, persona_name are required"))
		return
	}
	if err := misc.SaveDigitalHuman(body); err != nil {
		c.JSON(500, Fail("save failed: "+err.Error()))
		return
	}
	c.JSON(200, Success("保存成功，重启项目后生效"))
}

func (s *Server) deleteDigitalHuman(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, Fail("id is required"))
		return
	}
	if err := misc.DeleteDigitalHuman(id); err != nil {
		c.JSON(500, Fail("delete failed: "+err.Error()))
		return
	}
	c.JSON(200, Success("删除成功，重启项目后生效"))
}

// initStatus returns the current initialization state (no auth required).
func initStatusHandler(c *gin.Context) {
	needsUser := !misc.HasAnyUser()

	// Check Docker images.
	images := map[string]bool{"aisandbox": false, "java_env": false}
	cmd := exec.Command("docker", "images", "--format", "{{.Repository}}")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			name := strings.TrimSpace(line)
			switch name {
			case "aisandbox", "aixvuln/aisandbox":
				images["aisandbox"] = true
			case "java_env", "aixvuln/java_env":
				images["java_env"] = true
			}
		}
	}

	c.JSON(200, gin.H{
		"needs_user":    needsUser,
		"docker_images": images,
		"initialized":   !needsUser,
	})
}

// initSetup handles the first-run setup (no auth required, only works if no user exists).
func initSetupHandler(c *gin.Context) {
	if misc.HasAnyUser() {
		c.JSON(400, gin.H{"success": false, "error": "系统已初始化，无法重复设置"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Username) == "" || len(req.Password) < 6 {
		c.JSON(400, gin.H{"success": false, "error": "用户名不能为空，密码至少6位"})
		return
	}
	if err := misc.CreateUser(strings.TrimSpace(req.Username), req.Password); err != nil {
		c.JSON(500, gin.H{"success": false, "error": "创建用户失败: " + err.Error()})
		return
	}
	token := generateToken(req.Username)
	c.JSON(200, gin.H{"success": true, "token": token})
}

// dockerAuthCheck validates token for docker endpoints.
// During init (no users exist) it is unauthenticated; after init it requires a valid token.
func dockerAuthCheck(c *gin.Context) bool {
	if misc.HasAnyUser() {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(401, gin.H{"success": false, "error": "unauthorized"})
			return false
		}
		if _, err := validateToken(strings.TrimPrefix(auth, "Bearer ")); err != nil {
			c.AbortWithStatusJSON(401, gin.H{"success": false, "error": "unauthorized: " + err.Error()})
			return false
		}
	}
	return true
}

// streamCmdOutput runs a command and streams its combined stdout/stderr via SSE.
// Each line is sent as a "data: {json}\n\n" event.
// Final event is either {"done":true} or {"done":true,"error":"..."}.
func streamCmdOutput(c *gin.Context, cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	// Set SSE headers.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		evt, _ := json.Marshal(map[string]interface{}{"line": line})
		fmt.Fprintf(c.Writer, "data: %s\n\n", evt)
		c.Writer.Flush()
	}

	cmdErr := cmd.Wait()
	if cmdErr != nil {
		evt, _ := json.Marshal(map[string]interface{}{"done": true, "error": cmdErr.Error()})
		fmt.Fprintf(c.Writer, "data: %s\n\n", evt)
	} else {
		evt, _ := json.Marshal(map[string]interface{}{"done": true})
		fmt.Fprintf(c.Writer, "data: %s\n\n", evt)
	}
	c.Writer.Flush()
	return nil
}

// dockerBuild triggers a docker build for the given image name (SSE streaming output).
func dockerBuildHandler(c *gin.Context) {
	if !dockerAuthCheck(c) {
		return
	}
	imageName := c.Param("name")
	if imageName != "aisandbox" && imageName != "java_env" {
		c.JSON(400, gin.H{"success": false, "error": "不支持的镜像: " + imageName})
		return
	}
	contextPath, err := misc.GetDockerfilePath(imageName)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": "释放 Dockerfile 失败: " + err.Error()})
		return
	}
	dockerfilePath := filepath.Join(contextPath, "Dockerfile")
	cmd := exec.Command("docker", "build", "-t", imageName, "-f", dockerfilePath, contextPath)
	if err := streamCmdOutput(c, cmd); err != nil {
		c.JSON(500, gin.H{"success": false, "error": "构建启动失败: " + err.Error()})
	}
}

// dockerPull pulls a pre-built image from a registry and tags it with the local name (SSE streaming output).
// Accepts optional ?registry= query parameter to specify a custom registry prefix.
func dockerPullHandler(c *gin.Context) {
	if !dockerAuthCheck(c) {
		return
	}
	imageName := c.Param("name")
	allowedImages := map[string]bool{"aisandbox": true, "java_env": true}
	if !allowedImages[imageName] {
		c.JSON(400, gin.H{"success": false, "error": "不支持的镜像: " + imageName})
		return
	}
	// Determine remote image path: custom registry or default Docker Hub namespace.
	registry := strings.TrimSpace(c.Query("registry"))
	var remoteImage string
	if registry != "" {
		registry = strings.TrimRight(registry, "/")
		remoteImage = registry + "/" + imageName
	} else {
		remoteImage = "aixvuln/" + imageName
	}
	// Pull from registry (streamed).
	cmd := exec.Command("docker", "pull", remoteImage)
	if err := streamCmdOutput(c, cmd); err != nil {
		c.JSON(500, gin.H{"success": false, "error": "拉取启动失败: " + err.Error()})
		return
	}
	// Tag as local name so the system can find it.
	tagCmd := exec.Command("docker", "tag", remoteImage, imageName)
	tagOutput, err := tagCmd.CombinedOutput()
	if err != nil {
		evt, _ := json.Marshal(map[string]interface{}{"done": true, "error": "标记镜像失败: " + err.Error(), "output": string(tagOutput)})
		fmt.Fprintf(c.Writer, "data: %s\n\n", evt)
		c.Writer.Flush()
	}
}

func (s *Server) uploadAvatar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, Fail("请上传头像文件: "+err.Error()))
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".webp" {
		c.JSON(400, Fail("仅支持 png/jpg/jpeg/gif/webp 格式"))
		return
	}
	avatarDir := filepath.Join(misc.GetDataDir(), ".avatars")
	_ = os.MkdirAll(avatarDir, 0755)
	filename := uuid.New().String() + ext
	dst := filepath.Join(avatarDir, filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(500, Fail("保存头像失败: "+err.Error()))
		return
	}
	c.JSON(200, Success(filename))
}

func serveAvatar(c *gin.Context) {
	name := c.Param("name")
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		c.JSON(400, gin.H{"error": "invalid filename"})
		return
	}
	avatarDir := filepath.Join(misc.GetDataDir(), ".avatars")
	filePath := filepath.Join(avatarDir, name)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.Status(404)
		return
	}
	c.File(filePath)
}

func (s *Server) getReportTemplates(c *gin.Context) {
	c.JSON(200, Success(misc.GetAllReportTemplates()))
}

func (s *Server) setReportTemplate(c *gin.Context) {
	var body map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, Fail("invalid JSON: "+err.Error()))
		return
	}
	for name, content := range body {
		if err := misc.SetReportTemplate(name, content); err != nil {
			c.JSON(500, Fail("保存模板失败: "+err.Error()))
			return
		}
	}
	c.JSON(200, Success("报告模板已保存"))
}
