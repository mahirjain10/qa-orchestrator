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

type ParsedCampaign struct {
	Campaign         *types.Campaign
	TopologicalOrder []string
}

func (p *CampaignParser) ParseFile(path string) (*ParsedCampaign, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading campaign file %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var campaign *types.Campaign
	switch ext {
	case ".yaml", ".yml":
		campaign, err = p.parseYAML(data)
	case ".json":
		campaign, err = p.parseJSON(data)
	default:
		return nil, fmt.Errorf("unsupported file format %q (supported: .yaml, .yml, .json)", ext)
	}
	if err != nil {
		return nil, err
	}

	dependencyValidator := validator.NewDependencyValidator()
	dependencyResult := dependencyValidator.Validate(campaign.Flows)
	if !dependencyResult.Valid {
		return nil, fmt.Errorf("%s", dependencyValidator.FormatError(dependencyResult.Error))
	}

	return &ParsedCampaign{
		Campaign:         campaign,
		TopologicalOrder: dependencyResult.TopologicalOrder,
	}, nil
}

func (p *CampaignParser) parseYAML(data []byte) (*types.Campaign, error) {
	var campaign types.Campaign
	if err := yaml.Unmarshal(data, &campaign); err != nil {
		return nil, fmt.Errorf("parsing YAML campaign: %w", err)
	}
	return &campaign, p.validateSchema(&campaign)
}

func (p *CampaignParser) parseJSON(data []byte) (*types.Campaign, error) {
	var campaign types.Campaign
	if err := json.Unmarshal(data, &campaign); err != nil {
		return nil, fmt.Errorf("parsing JSON campaign: %w", err)
	}
	return &campaign, p.validateSchema(&campaign)
}

func (p *CampaignParser) validateSchema(campaign *types.Campaign) error {
	if campaign.Name == "" {
		return fmt.Errorf("campaign 'name' is required")
	}
	if campaign.Version == "" {
		return fmt.Errorf("campaign 'version' is required")
	}
	if len(campaign.Flows) == 0 {
		return fmt.Errorf("campaign must have at least one flow")
	}

	if campaign.Config.Timeout <= 0 {
		return fmt.Errorf("campaign config: 'timeout' must be greater than 0")
	}
	if campaign.Config.RetryLimit < 0 {
		return fmt.Errorf("campaign config: 'retry_limit' must be >= 0")
	}
	if campaign.Config.ParallelLimit < 1 {
		return fmt.Errorf("campaign config: 'parallel_limit' must be >= 1")
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

		if flow.Config.Timeout != 0 && flow.Config.Timeout < 0 {
			return fmt.Errorf("flow %q config: 'timeout' must be greater than 0", flow.ID)
		}
		if flow.Config.RetryLimit < 0 {
			return fmt.Errorf("flow %q config: 'retry_limit' must be >= 0", flow.ID)
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

		seenStepIDs := make(map[string]bool)
		for _, step := range flow.Steps {
			if step.ID == "" {
				return fmt.Errorf("flow %q: step missing required 'id' field", flow.ID)
			}
			if seenStepIDs[step.ID] {
				return fmt.Errorf("flow %q: duplicate step ID %q", flow.ID, step.ID)
			}
			seenStepIDs[step.ID] = true

			if flow.Mode == types.FlowModeGuided && step.Tool == "" {
				return fmt.Errorf("flow %q: step %q: 'tool' is required", flow.ID, step.ID)
			}
		}
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
		Config: types.CampaignConfig{
			Timeout:       300 * time.Second,
			RetryLimit:    2,
			ParallelLimit: 1,
		},
		Flows: []types.Flow{flow},
	}

	return campaign, p.validateSchema(campaign)
}
