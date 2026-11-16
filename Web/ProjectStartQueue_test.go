package Web

import (
	"AIxVuln/DecisionBrain"
	"AIxVuln/LabManager"
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"reflect"
	"testing"
	"time"
	"unsafe"
)

type fakeQueueGroup struct {
	id      string
	name    string
	kind    string
	running bool
}

func (g *fakeQueueGroup) ID() string                { return g.id }
func (g *fakeQueueGroup) Name() string              { return g.name }
func (g *fakeQueueGroup) Kind() string              { return g.kind }
func (g *fakeQueueGroup) DecisionBrainName() string { return g.name + "-brain" }
func (g *fakeQueueGroup) Status() string {
	if g.running {
		return "正在运行"
	}
	return "待命"
}
func (g *fakeQueueGroup) IsRunning() bool                          { return g.running }
func (g *fakeQueueGroup) BrainFinished() bool                      { return false }
func (g *fakeQueueGroup) IsDemo() bool                             { return false }
func (g *fakeQueueGroup) HasMember(string) bool                    { return false }
func (g *fakeQueueGroup) HandleDecisionChat(string, string) string { return "" }
func (g *fakeQueueGroup) HandleGroupChat(string, string) string    { return "" }
func (g *fakeQueueGroup) NotifyDecisionBrainEvent(string)          {}
func (g *fakeQueueGroup) SyncGlobalStatusPanel(map[string]interface{}) {}
func (g *fakeQueueGroup) NormalizeGroupChatMessage(msg DecisionBrain.ChatMessage) DecisionBrain.ChatMessage {
	return msg
}
func (g *fakeQueueGroup) StartTask()                                                      {}
func (g *fakeQueueGroup) StopTask()                                                       {}
func (g *fakeQueueGroup) SharedReportCatalog() []taskManager.ReportMeta                   { return nil }
func (g *fakeQueueGroup) NotifySharedReportUpdate(taskManager.ReportMeta, string, string) {}
func (g *fakeQueueGroup) PrivateSnapshot() map[string]interface{}                         { return nil }
func (g *fakeQueueGroup) GetDigitalHumanRoster() map[string]interface{}                   { return nil }
func (g *fakeQueueGroup) GetAgentRuntimeList() []map[string]interface{}                   { return nil }
func (g *fakeQueueGroup) Snapshot() map[string]interface{}                                { return nil }

func setUnexportedField(target interface{}, fieldName string, value interface{}) {
	v := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func newRunningLabManagerForQueueTest(t *testing.T) (*LabManager.LabManager, *fakeQueueGroup) {
	t.Helper()
	lm := &LabManager.LabManager{}
	group := &fakeQueueGroup{
		id:      "fake_group",
		name:    "fake_group",
		kind:    "fake",
		running: true,
	}

	groupMapField := reflect.ValueOf(lm).Elem().FieldByName("groups")
	groupMap := reflect.MakeMap(groupMapField.Type())
	groupMap.SetMapIndex(reflect.ValueOf(group.id), reflect.ValueOf(group).Convert(groupMapField.Type().Elem()))
	reflect.NewAt(groupMapField.Type(), unsafe.Pointer(groupMapField.UnsafeAddr())).Elem().Set(groupMap)

	setUnexportedField(lm, "groupOrder", []string{group.id})
	lm.SetStartTime(time.Now().Format("2006-01-02 15:04:05"))
	lm.SetStatus("正在运行")
	return lm, group
}

func TestStartQueueDispatchRespectsActiveRunningProjects(t *testing.T) {
	oldConcurrency := misc.GetConfigValueDefault("misc", "PROJECT_TASK_MAX_CONCURRENCY", "")
	if err := misc.SetConfigValue("misc", "PROJECT_TASK_MAX_CONCURRENCY", "1"); err != nil {
		t.Fatalf("SetConfigValue: %v", err)
	}
	t.Cleanup(func() {
		_ = misc.SetConfigValue("misc", "PROJECT_TASK_MAX_CONCURRENCY", oldConcurrency)
	})

	running, fakeGroup := newRunningLabManagerForQueueTest(t)

	s := &Server{
		pms: map[string]*LabManager.LabManager{
			"already-running": running,
		},
		startQueue:       []string{"queued-project"},
		startQueueSet:    map[string]struct{}{"queued-project": {}},
		startRunningSet:  make(map[string]struct{}),
		startQueueNotify: make(chan struct{}, 1),
	}
	s.initStartQueueDispatcher()

	s.notifyStartQueue()
	time.Sleep(200 * time.Millisecond)

	s.startQueueMu.Lock()
	queuedCount := len(s.startQueue)
	s.startQueueMu.Unlock()
	if queuedCount != 1 {
		t.Fatalf("expected queued project to stay queued while another project is running, got queue len=%d", queuedCount)
	}

	fakeGroup.running = false
	running.SetStatus("已停止")
	running.SetEndTime(time.Now().Format("2006-01-02 15:04:05"))

	s.notifyStartQueue()
	time.Sleep(200 * time.Millisecond)

	s.startQueueMu.Lock()
	queuedCount = len(s.startQueue)
	s.startQueueMu.Unlock()
	if queuedCount != 0 {
		t.Fatalf("expected queued project to dispatch after running slot is released, got queue len=%d", queuedCount)
	}
}
