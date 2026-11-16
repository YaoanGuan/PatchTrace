package toolCalling

import "sync"

var projectMCPRegisterOnce sync.Once

func EnsureProjectMCPProfilesRegistered() {
	projectMCPRegisterOnce.Do(func() {
		ReloadMCPRegistryFromStore()
	})
}

func registerProjectMCPProfiles() {
}
