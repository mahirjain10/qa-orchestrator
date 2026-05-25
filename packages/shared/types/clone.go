package types

func (s *Session) Clone() *Session {
	if s == nil {
		return nil
	}
	
	cloned := &Session{
		RunID:         s.RunID,
		SessionID:     s.SessionID,
		CampaignName:  s.CampaignName,
		Status:        s.Status,
		CurrentFlowID: s.CurrentFlowID,
		CurrentAgent:  s.CurrentAgent,
		StartedAt:     s.StartedAt,
		UpdatedAt:     s.UpdatedAt,
		RetryCount:    s.RetryCount,
	}

	if s.CompletedAt != nil {
		t := *s.CompletedAt
		cloned.CompletedAt = &t
	}

	if s.Flows != nil {
		cloned.Flows = make([]FlowRunState, len(s.Flows))
		for i, f := range s.Flows {
			cf := f
			if f.StartedAt != nil {
				t := *f.StartedAt
				cf.StartedAt = &t
			}
			if f.FinishedAt != nil {
				t := *f.FinishedAt
				cf.FinishedAt = &t
			}
			cloned.Flows[i] = cf
		}
	}

	if s.Checkpoint != nil {
		cp := &Checkpoint{
			FlowID:    s.Checkpoint.FlowID,
			StepIndex: s.Checkpoint.StepIndex,
			StepID:    s.Checkpoint.StepID,
			Timestamp: s.Checkpoint.Timestamp,
		}
		if s.Checkpoint.Payload != nil {
			cp.Payload = deepCloneMap(s.Checkpoint.Payload)
		}
		cloned.Checkpoint = cp
	}

	return cloned
}

func deepCloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	res := make(map[string]any, len(m))
	for k, v := range m {
		res[k] = deepCloneValue(v)
	}
	return res
}

func deepCloneValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCloneMap(val)
	case []any:
		res := make([]any, len(val))
		for i, item := range val {
			res[i] = deepCloneValue(item)
		}
		return res
	case map[string]bool:
		res := make(map[string]bool, len(val))
		for k, v := range val {
			res[k] = v
		}
		return res
	case map[string]string:
		res := make(map[string]string, len(val))
		for k, v := range val {
			res[k] = v
		}
		return res
	default:
		return val
	}
}
