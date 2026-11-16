package LabManager

import (
	"AIxVuln/DecisionBrain"
	"encoding/json"
	"os"
	"strings"
	"time"
)

func (lm *LabManager) AppendChatMessage(msg DecisionBrain.ChatMessage) {
	lm.chatMu.Lock()
	lm.chatMessages = append(lm.chatMessages, msg)
	if len(lm.chatMessages) > maxLabChatRecord {
		lm.chatMessages = append([]DecisionBrain.ChatMessage(nil), lm.chatMessages[len(lm.chatMessages)-maxLabChatRecord:]...)
	}
	now := time.Now()
	if lm.lastChatPersistAt.IsZero() || now.Sub(lm.lastChatPersistAt) >= labChatDebounce {
		lm.saveChatMessagesLocked()
		lm.lastChatPersistAt = now
		if lm.chatPersistTimer != nil {
			lm.chatPersistTimer.Stop()
			lm.chatPersistTimer = nil
		}
	} else if lm.chatPersistTimer == nil {
		wait := labChatDebounce - now.Sub(lm.lastChatPersistAt)
		if wait < 100*time.Millisecond {
			wait = 100 * time.Millisecond
		}
		lm.chatPersistTimer = time.AfterFunc(wait, func() {
			lm.chatMu.Lock()
			defer lm.chatMu.Unlock()
			lm.saveChatMessagesLocked()
			lm.lastChatPersistAt = time.Now()
			lm.chatPersistTimer = nil
		})
	}
	lm.chatMu.Unlock()
}

func (lm *LabManager) GetChatMessages() []DecisionBrain.ChatMessage {
	lm.chatMu.Lock()
	defer lm.chatMu.Unlock()
	out := make([]DecisionBrain.ChatMessage, len(lm.chatMessages))
	copy(out, lm.chatMessages)
	return out
}

func (lm *LabManager) saveChatMessagesLocked() {
	if strings.TrimSpace(lm.chatFilePath) == "" {
		return
	}
	data, err := json.Marshal(lm.chatMessages)
	if err != nil {
		return
	}
	_ = os.WriteFile(lm.chatFilePath, data, 0644)
}

func (lm *LabManager) persistChatMessagesNow() {
	lm.chatMu.Lock()
	defer lm.chatMu.Unlock()
	if lm.chatPersistTimer != nil {
		lm.chatPersistTimer.Stop()
		lm.chatPersistTimer = nil
	}
	lm.saveChatMessagesLocked()
	lm.lastChatPersistAt = time.Now()
}

func (lm *LabManager) emitChatMessage(msg DecisionBrain.ChatMessage) {
	lm.AppendChatMessage(msg)
	lm.sendWS("LabChatMessage", msg)
}

func (lm *LabManager) emitBrainMessage(text string) {
	lm.emitChatMessage(DecisionBrain.ChatMessage{
		Role:        "system",
		Text:        text,
		Ts:          time.Now().Format("15:04:05"),
		PersonaName: totalBrainName,
		AvatarFile:  totalBrainAvatar,
		ChannelID:   decisionMakersChannelID,
		ChannelName: decisionMakersChanName,
		SenderType:  "total_brain",
	})
}

func (lm *LabManager) emitTotalBrainDirective(group researchGroup, text string) {
	if group == nil || strings.TrimSpace(text) == "" {
		return
	}
	lm.appendTotalBrainMemoryMessages(lm.totalBrainDirectiveMemoryMessage(group, text))
	lm.emitChatMessage(DecisionBrain.ChatMessage{
		Role:        "system",
		Text:        "@" + group.DecisionBrainName() + " " + strings.TrimSpace(text),
		Ts:          time.Now().Format("15:04:05"),
		PersonaName: totalBrainName,
		AvatarFile:  totalBrainAvatar,
		ChannelID:   decisionMakersChannelID,
		ChannelName: decisionMakersChanName,
		GroupID:     group.ID(),
		GroupName:   group.Name(),
		SenderType:  "total_brain",
	})
}

func (lm *LabManager) emitDecisionBrainMessage(group researchGroup, text string) {
	if group == nil {
		return
	}
	lm.appendTotalBrainMemoryMessages(lm.totalBrainGroupMessageMemoryMessage(group, text))
	lm.emitChatMessage(DecisionBrain.ChatMessage{
		Role:        "system",
		Text:        text,
		Ts:          time.Now().Format("15:04:05"),
		PersonaName: group.DecisionBrainName(),
		AvatarFile:  totalBrainAvatar,
		ChannelID:   decisionMakersChannelID,
		ChannelName: decisionMakersChanName,
		GroupID:     group.ID(),
		GroupName:   group.Name(),
		SenderType:  "group_brain",
	})
}

func (lm *LabManager) emitGroupBrainCoordinationMessage(groupID string, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	lm.emitGroupChatMessage(groupID, DecisionBrain.ChatMessage{
		Role:        "system",
		Text:        strings.TrimSpace(text),
		Ts:          time.Now().Format("15:04:05"),
		PersonaName: "决策大脑",
		AvatarFile:  groupBrainAvatar,
	})
}

func (lm *LabManager) emitGroupChatMessage(groupID string, msg DecisionBrain.ChatMessage) {
	g, ok := lm.groups[groupID]
	if !ok {
		lm.emitChatMessage(msg)
		return
	}
	lm.emitChatMessage(g.NormalizeGroupChatMessage(msg))
}
