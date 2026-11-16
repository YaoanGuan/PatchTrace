package misc

import (
	"AIxVuln/prompts"
	"database/sql"
	"encoding/json"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	binaryReversePrimaryID              = "binary-reverse-primary"
	binaryReverseSkeletonID             = "binary-reverse-skeleton"
	binaryAndroidPrimaryID              = "binary-android-primary"
	binaryAndroidSkeletonID             = "binary-android-skeleton"
	binaryReverseAgentType              = "Agent-Binary-ReverseCommonAgent"
	binaryAndroidAgentType              = "Agent-Binary-AndroidCommonAgent"
	binaryReportAgentType               = "Agent-Binary-ReportCommonAgent"
	pentestPrimaryID                    = "pentest-blackbox-primary"
	pentestSrcPrimaryID                 = "pentest-src-primary"
	pentestPostExploitationPrimaryID    = "pentest-post-exploitation-primary"
	legacyPentestRemoteControlPrimaryID = "pentest-remote-control-primary"
	pentestAgentType                    = "Agent-Pentest-PentestCommonAgent"
	pentestSrcAgentType                 = "Agent-Pentest-SrcCommonAgent"
	pentestPostExploitationAgentType    = "Agent-Pentest-PostExploitationCommonAgent"
	legacyPentestRemoteControlAgentType = "Agent-Pentest-RemoteControlCommonAgent"
)

// DigitalHumanRow represents a digital human profile stored in SQLite.
type DigitalHumanRow struct {
	ID             string `json:"id"`
	AgentType      string `json:"agent_type"`
	PersonaName    string `json:"persona_name"`
	Gender         string `json:"gender"`
	AvatarFile     string `json:"avatar_file"`
	Personality    string `json:"personality"`
	Age            int    `json:"age"`
	ExtraSysPrompt string `json:"extra_sys_prompt"`
}

func initDigitalHumanTable() {
	initConfigDB()
	configDB.Exec(`
		CREATE TABLE IF NOT EXISTS digital_human (
			id              TEXT PRIMARY KEY,
			agent_type      TEXT NOT NULL,
			persona_name    TEXT NOT NULL,
			gender          TEXT NOT NULL DEFAULT '',
			avatar_file     TEXT NOT NULL DEFAULT '',
			personality     TEXT NOT NULL DEFAULT '',
			age             INTEGER NOT NULL DEFAULT 0,
			extra_sys_prompt TEXT NOT NULL DEFAULT ''
		);
	`)

	defaultSeeds := prompts.DefaultDigitalHumanSeeds()
	defaults := make([]DigitalHumanRow, 0, len(defaultSeeds))
	for _, seed := range defaultSeeds {
		defaults = append(defaults, DigitalHumanRow{
			ID:             seed.ID,
			AgentType:      seed.AgentType,
			PersonaName:    seed.PersonaName,
			Gender:         seed.Gender,
			AvatarFile:     seed.AvatarFile,
			Personality:    seed.Personality,
			Age:            seed.Age,
			ExtraSysPrompt: seed.ExtraSysPrompt,
		})
	}

	stmt, err := configDB.Prepare(`INSERT OR IGNORE INTO digital_human (id, agent_type, persona_name, gender, avatar_file, personality, age, extra_sys_prompt) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return
	}
	defer stmt.Close()
	for _, d := range defaults {
		stmt.Exec(d.ID, d.AgentType, d.PersonaName, d.Gender, d.AvatarFile, d.Personality, d.Age, d.ExtraSysPrompt)
	}
	normalizeLegacyDigitalHumans()
}

func normalizeLegacyDigitalHumans() {
	initConfigDB()
	_, _ = configDB.Exec(`UPDATE digital_human SET id = ? WHERE id = ? AND agent_type = ? AND NOT EXISTS (SELECT 1 FROM digital_human WHERE id = ? AND agent_type = ?)`,
		binaryReversePrimaryID, binaryReverseSkeletonID, binaryReverseAgentType, binaryReversePrimaryID, binaryReverseAgentType)
	_, _ = configDB.Exec(`UPDATE digital_human SET id = ? WHERE id = ? AND agent_type = ? AND NOT EXISTS (SELECT 1 FROM digital_human WHERE id = ? AND agent_type = ?)`,
		binaryAndroidPrimaryID, binaryAndroidSkeletonID, binaryAndroidAgentType, binaryAndroidPrimaryID, binaryAndroidAgentType)
	_, _ = configDB.Exec(`DELETE FROM digital_human WHERE id = ? AND agent_type = ? AND EXISTS (SELECT 1 FROM digital_human WHERE id = ? AND agent_type = ?)`,
		binaryReverseSkeletonID, binaryReverseAgentType, binaryReversePrimaryID, binaryReverseAgentType)
	_, _ = configDB.Exec(`DELETE FROM digital_human WHERE id = ? AND agent_type = ? AND EXISTS (SELECT 1 FROM digital_human WHERE id = ? AND agent_type = ?)`,
		binaryAndroidSkeletonID, binaryAndroidAgentType, binaryAndroidPrimaryID, binaryAndroidAgentType)
	_, _ = configDB.Exec(`UPDATE digital_human SET id = ?, agent_type = ? WHERE id = ? AND agent_type = ? AND NOT EXISTS (SELECT 1 FROM digital_human WHERE id = ? AND agent_type = ?)`,
		pentestPostExploitationPrimaryID, pentestPostExploitationAgentType,
		legacyPentestRemoteControlPrimaryID, legacyPentestRemoteControlAgentType,
		pentestPostExploitationPrimaryID, pentestPostExploitationAgentType)
	_, _ = configDB.Exec(`DELETE FROM digital_human WHERE id = ? AND agent_type = ? AND EXISTS (SELECT 1 FROM digital_human WHERE id = ? AND agent_type = ?)`,
		legacyPentestRemoteControlPrimaryID, legacyPentestRemoteControlAgentType,
		pentestPostExploitationPrimaryID, pentestPostExploitationAgentType)
	_, _ = configDB.Exec(`UPDATE digital_human SET agent_type = ? WHERE agent_type = ?`,
		pentestPostExploitationAgentType, legacyPentestRemoteControlAgentType)
	_, _ = configDB.Exec(`DELETE FROM digital_human WHERE agent_type = ?`, binaryReportAgentType)
}

func canonicalDigitalHumanIdentity(agentType string, id string) (string, string) {
	agentType = strings.TrimSpace(agentType)
	id = strings.TrimSpace(id)
	switch {
	case agentType == binaryReverseAgentType && (id == binaryReversePrimaryID || id == binaryReverseSkeletonID):
		return binaryReverseAgentType, binaryReversePrimaryID
	case agentType == binaryAndroidAgentType && (id == binaryAndroidPrimaryID || id == binaryAndroidSkeletonID):
		return binaryAndroidAgentType, binaryAndroidPrimaryID
	case agentType == legacyPentestRemoteControlAgentType:
		if id == legacyPentestRemoteControlPrimaryID || id == pentestPostExploitationPrimaryID {
			id = pentestPostExploitationPrimaryID
		}
		return pentestPostExploitationAgentType, id
	case agentType == pentestPostExploitationAgentType && (id == legacyPentestRemoteControlPrimaryID || id == pentestPostExploitationPrimaryID):
		return pentestPostExploitationAgentType, pentestPostExploitationPrimaryID
	default:
		return agentType, id
	}
}

func canonicalDigitalHumanLookupID(id string) string {
	id = strings.TrimSpace(id)
	switch id {
	case binaryReverseSkeletonID:
		return binaryReversePrimaryID
	case binaryAndroidSkeletonID:
		return binaryAndroidPrimaryID
	case legacyPentestRemoteControlPrimaryID:
		return pentestPostExploitationPrimaryID
	default:
		return id
	}
}

func normalizeDigitalHumanRow(row DigitalHumanRow) DigitalHumanRow {
	row.AgentType, row.ID = canonicalDigitalHumanIdentity(row.AgentType, row.ID)
	return row
}

func dedupeLegacyDigitalHumans(rows []DigitalHumanRow) []DigitalHumanRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]DigitalHumanRow, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		row = normalizeDigitalHumanRow(row)
		agentType := strings.TrimSpace(row.AgentType)
		id := strings.TrimSpace(row.ID)
		key := id
		if key == "" {
			key = strings.TrimSpace(row.PersonaName) + "\x00" + strings.TrimSpace(row.AvatarFile)
		}
		if agentType == binaryReverseAgentType || agentType == binaryAndroidAgentType {
			key = agentType + "\x00" + key
		} else {
			key = agentType + "\x00id\x00" + key
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, row)
	}
	return out
}

// GetAllDigitalHumans returns all digital humans grouped by agent_type.
func GetAllDigitalHumans() map[string][]DigitalHumanRow {
	initDigitalHumanTable()
	normalizeLegacyDigitalHumans()
	rows, err := configDB.Query(`SELECT id, agent_type, persona_name, gender, avatar_file, personality, age, extra_sys_prompt FROM digital_human ORDER BY agent_type, persona_name, id`)
	if err != nil {
		return make(map[string][]DigitalHumanRow)
	}
	defer rows.Close()
	flat := make([]DigitalHumanRow, 0)
	for rows.Next() {
		var d DigitalHumanRow
		if err := rows.Scan(&d.ID, &d.AgentType, &d.PersonaName, &d.Gender, &d.AvatarFile, &d.Personality, &d.Age, &d.ExtraSysPrompt); err != nil {
			continue
		}
		flat = append(flat, normalizeDigitalHumanRow(d))
	}
	result := make(map[string][]DigitalHumanRow)
	for _, d := range dedupeLegacyDigitalHumans(flat) {
		result[d.AgentType] = append(result[d.AgentType], d)
	}
	return result
}

// GetDigitalHumansList returns all digital humans as a flat list.
func GetDigitalHumansList() []DigitalHumanRow {
	initDigitalHumanTable()
	normalizeLegacyDigitalHumans()
	rows, err := configDB.Query(`SELECT id, agent_type, persona_name, gender, avatar_file, personality, age, extra_sys_prompt FROM digital_human ORDER BY agent_type, persona_name, id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []DigitalHumanRow
	for rows.Next() {
		var d DigitalHumanRow
		if err := rows.Scan(&d.ID, &d.AgentType, &d.PersonaName, &d.Gender, &d.AvatarFile, &d.Personality, &d.Age, &d.ExtraSysPrompt); err != nil {
			continue
		}
		result = append(result, normalizeDigitalHumanRow(d))
	}
	return dedupeLegacyDigitalHumans(result)
}

// SaveDigitalHuman inserts or updates a digital human.
func SaveDigitalHuman(d DigitalHumanRow) error {
	initDigitalHumanTable()
	normalizeLegacyDigitalHumans()
	d = normalizeDigitalHumanRow(d)
	_, err := configDB.Exec(`INSERT INTO digital_human (id, agent_type, persona_name, gender, avatar_file, personality, age, extra_sys_prompt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET agent_type=excluded.agent_type, persona_name=excluded.persona_name, gender=excluded.gender,
		avatar_file=excluded.avatar_file, personality=excluded.personality, age=excluded.age, extra_sys_prompt=excluded.extra_sys_prompt`,
		d.ID, d.AgentType, d.PersonaName, d.Gender, d.AvatarFile, d.Personality, d.Age, d.ExtraSysPrompt)
	if err == nil {
		normalizeLegacyDigitalHumans()
	}
	return err
}

// DeleteDigitalHuman removes a digital human by ID.
func DeleteDigitalHuman(id string) error {
	initDigitalHumanTable()
	normalizeLegacyDigitalHumans()
	_, err := configDB.Exec(`DELETE FROM digital_human WHERE id = ?`, canonicalDigitalHumanLookupID(id))
	return err
}

// SaveAllDigitalHumans replaces all digital humans with the provided list.
func SaveAllDigitalHumans(list []DigitalHumanRow) error {
	initDigitalHumanTable()
	normalizeLegacyDigitalHumans()
	tx, err := configDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec(`DELETE FROM digital_human`)
	stmt, err := tx.Prepare(`INSERT INTO digital_human (id, agent_type, persona_name, gender, avatar_file, personality, age, extra_sys_prompt) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	normalized := make([]DigitalHumanRow, 0, len(list))
	for _, d := range list {
		normalized = append(normalized, normalizeDigitalHumanRow(d))
	}
	for _, d := range dedupeLegacyDigitalHumans(normalized) {
		if _, err := stmt.Exec(d.ID, d.AgentType, d.PersonaName, d.Gender, d.AvatarFile, d.Personality, d.Age, d.ExtraSysPrompt); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	normalizeLegacyDigitalHumans()
	return nil
}

// DigitalHumansToJSON serializes the list to JSON.
func DigitalHumansToJSON(list []DigitalHumanRow) string {
	b, _ := json.Marshal(list)
	return string(b)
}

// DigitalHumansFromJSON deserializes JSON to a list.
func DigitalHumansFromJSON(data string) ([]DigitalHumanRow, error) {
	var list []DigitalHumanRow
	err := json.Unmarshal([]byte(data), &list)
	return list, err
}

// GetDigitalHumanByID returns a single digital human by ID.
func GetDigitalHumanByID(id string) (*DigitalHumanRow, error) {
	initDigitalHumanTable()
	normalizeLegacyDigitalHumans()
	var d DigitalHumanRow
	err := configDB.QueryRow(`SELECT id, agent_type, persona_name, gender, avatar_file, personality, age, extra_sys_prompt FROM digital_human WHERE id = ?`, canonicalDigitalHumanLookupID(id)).
		Scan(&d.ID, &d.AgentType, &d.PersonaName, &d.Gender, &d.AvatarFile, &d.Personality, &d.Age, &d.ExtraSysPrompt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d = normalizeDigitalHumanRow(d)
	return &d, nil
}
