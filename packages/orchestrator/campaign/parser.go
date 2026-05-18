package campaign

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"qa-orchestrator/packages/shared/types"
	"gopkg.in/yaml.v3"
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
		return fmt.Errorf("campaign name is required")
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
	}
	return nil
}

func (p *CampaignParser) ParseNaturalLanguage(text string) (*types.Campaign, error) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty description")
	}

	flow := types.Flow{
		ID:    "auto-flow-1",
		Name:  strings.TrimSpace(lines[0]),
		Goal:  text,
		Steps: []types.Step{},
	}

	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		flow.Description = strings.TrimSpace(lines[1])
	}

	campaign := &types.Campaign{
		Name:  "Natural Campaign",
		Flows: []types.Flow{flow},
	}

	return campaign, nil
}
