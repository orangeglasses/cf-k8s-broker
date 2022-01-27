package main

type PlanConfigElement struct {
	Value        interface{} `json:"value"`
	UserOverride bool        `json:"userOverride"`
}

type PlanConfig map[string]PlanConfigElement

type Plan struct {
	ID              string `json:"id,omitempty"`
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
	value := p.Config[configName].Value

	return value
}
