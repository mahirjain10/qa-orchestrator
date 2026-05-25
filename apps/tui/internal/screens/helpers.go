package screens

import (
	"fmt"
	"sort"
	"time"

	"qa-orchestrator/packages/shared/types"
)

func (m *MainScreen) visualSessions() []*types.Session {
	var sessions []*types.Session
	if m.currentRun != nil {
		for _, s := range m.sessions {
			if s.RunID == m.currentRun.RunID {
				sessions = append(sessions, s)
				break
			}
		}
	}
	sorted := make([]*types.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if m.currentRun != nil && s.RunID == m.currentRun.RunID {
			continue
		}
		sorted = append(sorted, s)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartedAt.After(sorted[j].StartedAt)
	})
	sessions = append(sessions, sorted...)
	return sessions
}

func (m *MainScreen) campaignNames() []string {
	names := []string{}
	for _, s := range m.visualSessions() {
		age := m.formatSessionAge(s.StartedAt)
		marker := ""
		if m.currentRun != nil && s.RunID == m.currentRun.RunID {
			marker = " [CURRENT]"
		}
		names = append(names, fmt.Sprintf("%s [%s] (%s)%s", s.CampaignName, s.RunID, age, marker))
	}
	return names
}

func (m *MainScreen) formatSessionAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	elapsed := time.Since(t)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	}
}
