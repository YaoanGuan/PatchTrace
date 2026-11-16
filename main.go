package main

import (
	"AIxVuln/Web"
	"AIxVuln/dispatch"
	"AIxVuln/graphserver"
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
	laneCtx, cancelLane := context.WithCancel(context.Background())
	lane := startGraphLane(laneCtx, server)
	// StartWebServerWithUIFS blocks until SIGINT/SIGTERM. shutdown cancels the
	// scheduler, waits for Run and the in-flight executor tasks to return, and
	// only then releases the lane-owned stores (see graphLane.shutdown).
	defer lane.shutdown(cancelLane)
	server.StartWebServerWithUIFS(strings.TrimSpace(*port), uiFS)
}

// graphLane is the optional dispatcher lane owned by main. done is closed when
// the Run goroutine exits.
type graphLane struct {
	dispatcher *dispatch.Dispatcher
	done       <-chan struct{}
}

// shutdown stops the lane in the order the dispatcher contract requires:
// cancel the lane context, wait for Run to return (it may be inside a tick that
// started work), drain the executor's in-flight tasks, and only then close the
// stores the lane opened. Closing earlier would let an in-flight explore or
// tick touch a closed SQLite handle (data race, nil deref, "database is
// closed"). A nil lane is a no-op: the flag is off or the lane failed to start.
func (l *graphLane) shutdown(cancel context.CancelFunc) {
	if cancel != nil {
		cancel()
	}
	if l == nil || l.dispatcher == nil {
		return
	}
	if l.done != nil {
		<-l.done
	}
	l.dispatcher.Wait()
	if err := l.dispatcher.Close(); err != nil {
		misc.Warn("graph-lane", fmt.Sprintf("关闭图调度车道失败: %v", err), nil)
	}
}

// startGraphLane starts the optional graph dispatcher lane when
// misc.graph_lane_enabled is on (default false) and mounts its HTTP protocol
// under /api/graph on the existing web server. The routes are mounted only when
// the lane actually started, because they need the lane's store.
//
// The lane is strictly additive: every failure is logged and the existing
// closed loop still starts. The graph lane must never block web-server startup,
// so Run executes in its own goroutine. The returned graphLane is owned by main;
// its shutdown method waits for Run and the executor before closing stores.
func startGraphLane(ctx context.Context, server *Web.Server) *graphLane {
	if !misc.GraphLaneEnabled() {
		return nil
	}
	configPath := strings.TrimSpace(misc.GetConfigValueDefault("misc", "graph_lane_config", "dispatch.yaml"))
	cfg, err := dispatch.LoadConfig(configPath)
	if err != nil {
		misc.Warn("graph-lane", fmt.Sprintf("未启动图调度车道（配置 %s）: %v", configPath, err), nil)
		return nil
	}
	complete, err := dispatch.NativeComplete("main_setting")
	if err != nil {
		misc.Warn("graph-lane", fmt.Sprintf("未启动图调度车道: %v", err), nil)
		return nil
	}
	dispatcher, err := dispatch.StartLane(ctx, dispatch.LaneOptions{Config: cfg, Complete: complete})
	if err != nil {
		misc.Warn("graph-lane", fmt.Sprintf("未启动图调度车道: %v", err), nil)
		return nil
	}
	if server != nil && dispatcher.Store() != nil {
		server.RegisterAPIRoutes(graphserver.New(dispatcher.Store()).Register)
	}
	misc.Info("graph-lane", fmt.Sprintf("图调度车道已启动，配置: %s", configPath), nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := dispatcher.Run(ctx); err != nil {
			misc.Warn("graph-lane", fmt.Sprintf("图调度车道退出: %v", err), nil)
		}
	}()
	return &graphLane{dispatcher: dispatcher, done: done}
}
