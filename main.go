package main

import (
	"AIxVuln/Web"
	"AIxVuln/dispatch"
	"AIxVuln/misc"
	"AIxVuln/toolCalling"
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

//go:embed all:webui/dist
var webUIFS embed.FS

//go:embed all:dockerfile
var dockerfileFS embed.FS

func init() {
	sub, err := fs.Sub(dockerfileFS, "dockerfile")
	if err != nil {
		misc.PanicWithStack("embed dockerfile 失败: %v", err)
	}
	misc.SetDockerfileFS(sub)
	err = misc.CreateDirIfNotExists("data/temp/")
	if err != nil {
		misc.PanicWithStack("创建 data/temp 失败: %v", err)
	}
}

func main() {
	defer misc.CleanupDockerfiles()
	ip := flag.String("ip", "", "public IP used in download_url for agent tools")
	port := flag.String("port", "9999", "http port")
	autoMode := flag.String("auto-mode", "", "global automation mode: off or full")
	fullAuto := flag.Bool("full-auto", false, "enable full automation mode (alias of -auto-mode full)")
	mcpConfig := flag.String("mcp-config", "", "path to the MCP config JSON store file")
	webMode := flag.String("web-mode", "", "web security research mode: audit or comprehensive")
	flag.Parse()
	runtimeAutoMode := strings.TrimSpace(*autoMode)
	if *fullAuto {
		runtimeAutoMode = misc.AutomationModeFull
	}
	if runtimeAutoMode != "" {
		if err := misc.SetRuntimeAutomationMode(runtimeAutoMode); err != nil {
			misc.PanicWithStack("无效的自动化模式 %q: %v", runtimeAutoMode, err)
		}
	}
	if rawWebMode := strings.TrimSpace(*webMode); rawWebMode != "" {
		if err := misc.SetRuntimeWebResearchMode(rawWebMode); err != nil {
			misc.PanicWithStack("无效的 Web 研究模式 %q: %v", rawWebMode, err)
		}
	}
	if path := strings.TrimSpace(*mcpConfig); path != "" {
		misc.SetRuntimeMCPConfigStorePath(path)
		resolvedPath := misc.ResolveMCPConfigStorePath()
		if _, err := os.Stat(resolvedPath); err == nil {
			if err := misc.LoadMCPConfigsFromFile(resolvedPath); err != nil {
				misc.PanicWithStack("加载 MCP 配置文件失败 %q: %v", resolvedPath, err)
			}
			fmt.Println("已加载 MCP 配置:", resolvedPath)
		} else if !os.IsNotExist(err) {
			misc.PanicWithStack("检查 MCP 配置文件失败 %q: %v", resolvedPath, err)
		} else {
			fmt.Println("将使用 MCP 配置文件:", resolvedPath)
		}
		toolCalling.ReloadMCPRegistryFromStore()
	}
	accessIP := misc.ResolveStartupAccessIP(*ip)
	misc.SetRuntimeServerBaseURL(misc.ComposeRuntimeBaseURL(accessIP, *port))
	uiFS, err := fs.Sub(webUIFS, "webui/dist")
	if err != nil {
		misc.PanicWithStack("加载前端静态资源失败: %v", err)
	}
	server := Web.NewServer()
	startGraphLane(context.Background())
	server.StartWebServerWithUIFS(strings.TrimSpace(*port), uiFS)
}

// startGraphLane starts the optional graph dispatcher lane when
// misc.graph_lane_enabled is on (default false).
//
// The lane is strictly additive: every failure is logged and the existing
// closed loop still starts. The graph lane must never block web-server startup,
// so Run executes in its own goroutine.
func startGraphLane(ctx context.Context) {
	if !misc.GraphLaneEnabled() {
		return
	}
	configPath := strings.TrimSpace(misc.GetConfigValueDefault("misc", "graph_lane_config", "dispatch.yaml"))
	cfg, err := dispatch.LoadConfig(configPath)
	if err != nil {
		misc.Warn("graph-lane", fmt.Sprintf("未启动图调度车道（配置 %s）: %v", configPath, err), nil)
		return
	}
	complete, err := dispatch.NativeComplete("main_setting")
	if err != nil {
		misc.Warn("graph-lane", fmt.Sprintf("未启动图调度车道: %v", err), nil)
		return
	}
	dispatcher, err := dispatch.StartLane(ctx, dispatch.LaneOptions{Config: cfg, Complete: complete})
	if err != nil {
		misc.Warn("graph-lane", fmt.Sprintf("未启动图调度车道: %v", err), nil)
		return
	}
	misc.Info("graph-lane", fmt.Sprintf("图调度车道已启动，配置: %s", configPath), nil)
	go func() {
		if err := dispatcher.Run(ctx); err != nil {
			misc.Warn("graph-lane", fmt.Sprintf("图调度车道退出: %v", err), nil)
		}
	}()
}
