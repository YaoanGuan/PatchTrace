package Web

import (
	resourcepack "AIxVuln/resource"
	"errors"
	"io/fs"
	"mime"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) listAgentTools(c *gin.Context) {
	items, err := resourcepack.ListAgentTools()
	if err != nil {
		c.JSON(500, Fail("load agent tools failed: "+err.Error()))
		return
	}
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]interface{}{
			"id":           item.ID,
			"name":         item.Name,
			"filename":     item.Filename,
			"file":         item.File,
			"category":     item.Category,
			"description":  item.Description,
			"size":         item.Size,
			"sha256":       item.SHA256,
			"downloadPath": "/agent_tools/" + item.ID + "/download",
		})
	}
	c.JSON(200, Success(out))
}

func (s *Server) downloadAgentTool(c *gin.Context) {
	item, data, err := resourcepack.ReadAgentTool(c.Param("id"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			c.JSON(404, Fail("agent tool not found"))
			return
		}
		c.JSON(500, Fail("load agent tool failed: "+err.Error()))
		return
	}
	filename := item.Filename
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Disposition", attachmentDisposition(filename))
	c.Header("Content-Length", strconv.Itoa(len(data)))
	c.Header("X-Agent-Tool-Id", item.ID)
	c.Data(200, contentType, data)
}
