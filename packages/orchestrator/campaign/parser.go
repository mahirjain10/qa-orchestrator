package campaign

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"qa-orchestrator/packages/orchestrator/validator"
	"qa-orchestrator/packages/shared/types"
)

type CampaignParser struct{}

func NewCampaignParser() *CampaignParser {
	return &CampaignParser{}
}

func (p *CampaignParser) ParseFile(path string) (*types.Campaign, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading campaign file %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return p.parseYAML(data)
	case ".json":
		return p.parseJSON(data)
	default:
		return nil, fmt.Errorf("unsupported file format %q (supported: .yaml, .yml, .json)", ext)
	}
}

func (p *CampaignParser) parseYAML(data []byte) (*types.Campaign, error) {
	var campaign types.Campaign
	if err := yaml.Unmarshal(data, &campaign); err != nil {
		return nil, fmt.Errorf("parsing YAML campaign: %w", err)
	}
	return &campaign, p.validate(&campaign)
}

func (p *CampaignParser) parseJSON(data []byte) (*types.Campaign, error) {
	var campaign types.Campaign
	if err := json.Unmarshal(data, &campaign); err != nil {
		return nil, fmt.Errorf("parsing JSON campaign: %w", err)
	}
	return &campaign, p.validate(&campaign)
}

func (p *CampaignParser) validate(campaign *types.Campaign) error {
	if campaign.Name == "" {
		return fmt.Errorf("campaign 'name' is required")
	}
	if campaign.Version == "" {
		return fmt.Errorf("campaign 'version' is required")
	}
	if len(campaign.Flows) == 0 {
		return fmt.Errorf("campaign must have at least one flow")
	}

	seenIDs := make(map[string]bool)
	for _, flow := range campaign.Flows {
		if flow.ID == "" {
			return fmt.Errorf("flow (name=%q) missing required 'id' field", flow.Name)
		}
		if seenIDs[flow.ID] {
			return fmt.Errorf("duplicate flow ID %q", flow.ID)
		}
		seenIDs[flow.ID] = true

		if flow.Goal == "" {
			return fmt.Errorf("flow %q: 'goal' is required", flow.ID)
		}

		switch flow.Mode {
		case types.FlowModeGuided:
			if len(flow.Steps) == 0 {
				return fmt.Errorf("flow %q: 'guided' mode requires non-empty 'steps'", flow.ID)
			}
		case types.FlowModeAutonomous:
			// Steps are optional in autonomous mode
		case "":
			return fmt.Errorf("flow %q: 'mode' is required", flow.ID)
		default:
			return fmt.Errorf("flow %q: invalid mode %q (must be 'guided' or 'autonomous')", flow.ID, flow.Mode)
		}

		switch flow.Priority {
		case types.FlowPriorityHigh, types.FlowPriorityMedium, types.FlowPriorityLow:
			// valid
		case "":
			return fmt.Errorf("flow %q: 'priority' is required", flow.ID)
		default:
			return fmt.Errorf("flow %q: invalid priority %q (must be 'high', 'medium', or 'low')", flow.ID, flow.Priority)
		}
	}

	dependencyValidator := validator.NewDependencyValidator()
	dependencyResult := dependencyValidator.Validate(campaign.Flows)
	if !dependencyResult.Valid {
		return fmt.Errorf("%s", dependencyValidator.FormatError(dependencyResult.Error))
	}

	return nil
}

func (p *CampaignParser) ParseNaturalLanguage(text string) (*types.Campaign, error) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty description")
	}

	flow := types.Flow{
		ID:       fmt.Sprintf("auto-flow-%d", time.Now().UnixNano()),
		Name:     strings.TrimSpace(lines[0]),
		Goal:     text,
		Mode:     types.FlowModeAutonomous,
		Priority: types.FlowPriorityMedium,
		Steps:    []types.Step{},
	}

	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		flow.Description = strings.TrimSpace(lines[1])
	}

	campaign := &types.Campaign{
		Name:    "Natural Campaign",
		Version: "1.0",
		Flows:   []types.Flow{flow},
	}

	return campaign, p.validate(campaign)
}
