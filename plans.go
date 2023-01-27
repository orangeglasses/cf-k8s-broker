package main

type PlanConfigElement struct {
	Value        interface{} `json:"value"`
	UserOverride bool        `json:"userOverride"`
	Required     bool        `json:"required,omitempty"`
}

type PlanConfig map[string]PlanConfigElement

type Plan struct {
	ID              string `json:"id"`
	Name            string
	DisplayName     string `json:"displayName"`
	Description     string `json:"description"`
	Free            *bool  `json:"free"`
	MaintenanceInfo struct {
		Version     string `json:"version"`
		Description string `json:"description"`
	} `json:"maintenance_info"`
	Config PlanConfig `json:"config,omitempty"`
}

type Plans map[string]Plan

func (p Plan) GetConfigValue(configName string) interface{} {
	if ce, ok := p.Config[configName]; ok {
		return ce.Value
	}

	return nil
}
