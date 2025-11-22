package firewall

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// PIIPatternDefinition represents a pattern definition from the config file
type PIIPatternDefinition struct {
	Name        string  `yaml:"name"`
	Type        string  `yaml:"type"`
	Pattern     string  `yaml:"pattern"`
	Mask        string  `yaml:"mask"`
	Confidence  float64 `yaml:"confidence"`
	Description string  `yaml:"description"`
}

// PIIPatternRegistry holds all loaded PII patterns organized by region
type PIIPatternRegistry struct {
	Global         []PIIPatternDefinition   `yaml:"global"`
	US             []PIIPatternDefinition   `yaml:"us"`
	CN             []PIIPatternDefinition   `yaml:"cn"`
	EU             []PIIPatternDefinition   `yaml:"eu"`
	UK             []PIIPatternDefinition   `yaml:"uk"`
	CA             []PIIPatternDefinition   `yaml:"ca"`
	AU             []PIIPatternDefinition   `yaml:"au"`
	IN             []PIIPatternDefinition   `yaml:"in"`
	JP             []PIIPatternDefinition   `yaml:"jp"`
	DE             []PIIPatternDefinition   `yaml:"de"`
	FR             []PIIPatternDefinition   `yaml:"fr"`
	SG             []PIIPatternDefinition   `yaml:"sg"`
	DefaultEnabled []string                 `yaml:"default_enabled"`
}

// LoadPIIPatterns loads PII patterns from a YAML file
func LoadPIIPatterns(path string) (*PIIPatternRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PII patterns file %s: %w", path, err)
	}

	var registry PIIPatternRegistry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse PII patterns file %s: %w", path, err)
	}

	return &registry, nil
}

// CompilePatterns compiles PIIPatternDefinitions into PIIPattern with compiled regex
func CompilePatterns(defs []PIIPatternDefinition) ([]PIIPattern, error) {
	patterns := make([]PIIPattern, 0, len(defs))

	for _, def := range defs {
		re, err := regexp.Compile(def.Pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern %s: %w", def.Name, err)
		}

		patterns = append(patterns, PIIPattern{
			Name:       def.Name,
			Type:       def.Type,
			Pattern:    re,
			Mask:       def.Mask,
			Confidence: def.Confidence,
		})
	}

	return patterns, nil
}

// GetPatternsByRegions returns compiled patterns for specified regions
func (r *PIIPatternRegistry) GetPatternsByRegions(regions []string) ([]PIIPattern, error) {
	var allDefs []PIIPatternDefinition

	// Always include global patterns
	allDefs = append(allDefs, r.Global...)

	// Add region-specific patterns
	for _, region := range regions {
		switch strings.ToLower(region) {
		case "us", "usa", "united_states":
			allDefs = append(allDefs, r.US...)
		case "cn", "china":
			allDefs = append(allDefs, r.CN...)
		case "eu", "europe":
			allDefs = append(allDefs, r.EU...)
		case "uk", "gb", "united_kingdom":
			allDefs = append(allDefs, r.UK...)
		case "ca", "canada":
			allDefs = append(allDefs, r.CA...)
		case "au", "australia":
			allDefs = append(allDefs, r.AU...)
		case "in", "india":
			allDefs = append(allDefs, r.IN...)
		case "jp", "japan":
			allDefs = append(allDefs, r.JP...)
		case "de", "germany":
			allDefs = append(allDefs, r.DE...)
		case "fr", "france":
			allDefs = append(allDefs, r.FR...)
		case "sg", "singapore":
			allDefs = append(allDefs, r.SG...)
		default:
			return nil, fmt.Errorf("unknown region: %s", region)
		}
	}

	return CompilePatterns(allDefs)
}

// GetDefaultPatterns returns the default enabled patterns
func (r *PIIPatternRegistry) GetDefaultPatterns() ([]PIIPattern, error) {
	if len(r.DefaultEnabled) == 0 {
		// If no default specified, use global + US + CN patterns
		allDefs := append([]PIIPatternDefinition{}, r.Global...)
		allDefs = append(allDefs, r.US...)
		allDefs = append(allDefs, r.CN...)
		return CompilePatterns(allDefs)
	}

	// Parse default_enabled list (format: "region.pattern_name")
	var selectedDefs []PIIPatternDefinition

	for _, fullName := range r.DefaultEnabled {
		parts := strings.SplitN(fullName, ".", 2)
		if len(parts) != 2 {
			continue
		}

		region, patternName := parts[0], parts[1]
		var regionDefs []PIIPatternDefinition

		switch strings.ToLower(region) {
		case "global":
			regionDefs = r.Global
		case "us":
			regionDefs = r.US
		case "cn":
			regionDefs = r.CN
		case "eu":
			regionDefs = r.EU
		case "uk":
			regionDefs = r.UK
		case "ca":
			regionDefs = r.CA
		case "au":
			regionDefs = r.AU
		case "in":
			regionDefs = r.IN
		case "jp":
			regionDefs = r.JP
		case "de":
			regionDefs = r.DE
		case "fr":
			regionDefs = r.FR
		case "sg":
			regionDefs = r.SG
		}

		// Find the pattern by name
		for _, def := range regionDefs {
			if def.Name == patternName {
				selectedDefs = append(selectedDefs, def)
				break
			}
		}
	}

	return CompilePatterns(selectedDefs)
}

// GetPatternsByTypes returns compiled patterns filtered by PII types
func (r *PIIPatternRegistry) GetPatternsByTypes(regions []string, types []string) ([]PIIPattern, error) {
	// Get all patterns for regions
	allPatterns, err := r.GetPatternsByRegions(regions)
	if err != nil {
		return nil, err
	}

	if len(types) == 0 {
		return allPatterns, nil
	}

	// Filter by types
	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[strings.ToUpper(t)] = true
	}

	filtered := make([]PIIPattern, 0)
	for _, p := range allPatterns {
		if typeMap[p.Type] {
			filtered = append(filtered, p)
		}
	}

	return filtered, nil
}
