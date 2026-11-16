package DecisionBrain

import "testing"

func TestShouldPublishVisibleEvent(t *testing.T) {
	if shouldPublishVisibleEvent("记忆体", 0) {
		t.Fatal("memory/internal events should not be published")
	}
	if shouldPublishVisibleEvent("记忆体", 1) {
		t.Fatal("memory events should stay hidden from visible event feeds")
	}
	if !shouldPublishVisibleEvent("decision", 1) {
		t.Fatal("normal visible events should still be published")
	}
}
