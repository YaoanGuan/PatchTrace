package llm

import (
	"AIxVuln/misc"
	"sync"
	"testing"
)

func resetClientFactoryStateForTest() {
	keySemaphores.mu.Lock()
	keySemaphores.m = make(map[string]chan struct{})
	keySemaphores.mu.Unlock()

	sharedKeyRings.mu.Lock()
	sharedKeyRings.m = make(map[string]*llmKeyRing)
	sharedKeyRings.mu.Unlock()

	cachedLLMClients.mu.Lock()
	cachedLLMClients.m = make(map[string]Client)
	cachedLLMClients.mu.Unlock()
}

func setTestConfigValue(t *testing.T, section, key, value string) {
	t.Helper()
	oldValue := misc.GetConfigValueDefault(section, key, "")
	if err := misc.SetConfigValue(section, key, value); err != nil {
		t.Fatalf("SetConfigValue(%s,%s): %v", section, key, err)
	}
	t.Cleanup(func() {
		if oldValue == "" {
			_ = misc.DeleteConfigValue(section, key)
			return
		}
		_ = misc.SetConfigValue(section, key, oldValue)
	})
}

func TestSplitConfiguredAPIKeysFiltersEmptyEntries(t *testing.T) {
	got := splitConfiguredAPIKeys(" key-1 |-| |-| key-2 |-|   |-| key-3 ")
	if len(got) != 3 {
		t.Fatalf("expected 3 keys, got %d: %#v", len(got), got)
	}
	if got[0] != "key-1" || got[1] != "key-2" || got[2] != "key-3" {
		t.Fatalf("unexpected keys: %#v", got)
	}
}

func TestNextRotatedAPIKeySharedAndIndependentRings(t *testing.T) {
	resetClientFactoryStateForTest()

	sharedA := nextRotatedAPIKey("https://api.example.com/v1/", []string{"a1", "a2"})
	sharedB := nextRotatedAPIKey("https://api.example.com/v1", []string{"a1", "a2"})
	sharedC := nextRotatedAPIKey("https://api.example.com/v1", []string{"a1", "a2"})
	if sharedA != "a1" || sharedB != "a2" || sharedC != "a1" {
		t.Fatalf("unexpected shared ring sequence: %q %q %q", sharedA, sharedB, sharedC)
	}

	independentA := nextRotatedAPIKey("https://api-a.example.com/v1", []string{"k1", "k2"})
	independentB := nextRotatedAPIKey("https://api-b.example.com/v1", []string{"k1", "k2"})
	if independentA != "k1" || independentB != "k1" {
		t.Fatalf("expected different rings to start independently, got %q and %q", independentA, independentB)
	}
}

func TestGetResponsesClientSharesRotationAcrossSectionsUsingSameRing(t *testing.T) {
	resetClientFactoryStateForTest()

	const (
		sectionA = "test_rotation_a"
		sectionB = "test_rotation_b"
	)

	setTestConfigValue(t, sectionA, "OPENAI_API_KEY", "key-A|-|key-B")
	setTestConfigValue(t, sectionA, "BASE_URL", "https://api.example.com/v1/")
	setTestConfigValue(t, sectionA, "API_MODE", "chat")
	setTestConfigValue(t, sectionB, "OPENAI_API_KEY", "key-A|-|key-B")
	setTestConfigValue(t, sectionB, "BASE_URL", "https://api.example.com/v1")
	setTestConfigValue(t, sectionB, "API_MODE", "chat")

	firstA, ok := GetResponsesClient(sectionA).(*rateLimitedClient)
	if !ok || firstA == nil {
		t.Fatalf("expected rateLimitedClient for %s", sectionA)
	}
	firstB, ok := GetResponsesClient(sectionB).(*rateLimitedClient)
	if !ok || firstB == nil {
		t.Fatalf("expected rateLimitedClient for %s", sectionB)
	}
	secondA, ok := GetResponsesClient(sectionA).(*rateLimitedClient)
	if !ok || secondA == nil {
		t.Fatalf("expected second rateLimitedClient for %s", sectionA)
	}

	if firstA.semaphore == firstB.semaphore {
		t.Fatal("expected sectionB to advance to the next key instead of reusing sectionA's first key")
	}
	if firstA.semaphore != secondA.semaphore {
		t.Fatal("expected sectionA to wrap back to its first key on the third shared rotation step")
	}
	if firstA != secondA {
		t.Fatal("expected cached client reuse for the same section+key pair")
	}
}

func TestGetResponsesClientConcurrentInitCreatesOneClientPerKey(t *testing.T) {
	resetClientFactoryStateForTest()

	const section = "test_rotation_concurrent"
	setTestConfigValue(t, section, "OPENAI_API_KEY", "concurrent-1|-|concurrent-2")
	setTestConfigValue(t, section, "BASE_URL", "https://api.concurrent.example.com/v1")
	setTestConfigValue(t, section, "API_MODE", "responses")

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cli := GetResponsesClient(section)
			if cli == nil {
				t.Error("expected non-nil client")
			}
		}()
	}
	wg.Wait()

	cachedLLMClients.mu.Lock()
	clientCount := len(cachedLLMClients.m)
	cachedLLMClients.mu.Unlock()
	if clientCount != 2 {
		t.Fatalf("expected exactly one cached client per key, got %d", clientCount)
	}
}

func TestGetResponsesClientUsesAnthropicBackend(t *testing.T) {
	resetClientFactoryStateForTest()

	const section = "test_anthropic_backend"
	setTestConfigValue(t, section, "OPENAI_API_KEY", "anthropic-key")
	setTestConfigValue(t, section, "BASE_URL", "https://api.anthropic.com")
	setTestConfigValue(t, section, "API_MODE", "anthropic")

	cli, ok := GetResponsesClient(section).(*rateLimitedClient)
	if !ok || cli == nil {
		t.Fatalf("expected rateLimitedClient for %s", section)
	}
	if _, ok := cli.inner.(*AnthropicMessagesClient); !ok {
		t.Fatalf("expected AnthropicMessagesClient, got %T", cli.inner)
	}
}
