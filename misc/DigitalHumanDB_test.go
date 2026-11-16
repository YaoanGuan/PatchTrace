package misc

import "testing"

func TestNormalizeDigitalHumanRowUpgradesLegacyPentestRemoteControlProfile(t *testing.T) {
	row := normalizeDigitalHumanRow(DigitalHumanRow{
		ID:        legacyPentestRemoteControlPrimaryID,
		AgentType: legacyPentestRemoteControlAgentType,
	})
	if row.ID != pentestPostExploitationPrimaryID {
		t.Fatalf("unexpected normalized id: %q", row.ID)
	}
	if row.AgentType != pentestPostExploitationAgentType {
		t.Fatalf("unexpected normalized agent type: %q", row.AgentType)
	}
}

func TestDedupeLegacyDigitalHumansCollapsesLegacyAndCurrentPentestPostExploitationRows(t *testing.T) {
	rows := dedupeLegacyDigitalHumans([]DigitalHumanRow{
		{ID: legacyPentestRemoteControlPrimaryID, AgentType: legacyPentestRemoteControlAgentType, PersonaName: "祁越"},
		{ID: pentestPostExploitationPrimaryID, AgentType: pentestPostExploitationAgentType, PersonaName: "祁越"},
	})
	if len(rows) != 1 {
		t.Fatalf("unexpected deduped row count: %d", len(rows))
	}
	if rows[0].ID != pentestPostExploitationPrimaryID || rows[0].AgentType != pentestPostExploitationAgentType {
		t.Fatalf("unexpected deduped row: %+v", rows[0])
	}
}
