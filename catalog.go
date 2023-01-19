package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"

	"github.com/pivotal-cf/brokerapi/domain"
)

func LoadCatalogImage() string {
	inBuf, err := ioutil.ReadFile("logo.png")
	if err != nil {
		return ""
	}

	return fmt.Sprintf("data:image/png;base64,%v", base64.StdEncoding.EncodeToString(inBuf))
}

func CatalogLoad(config brokerConfig, plans Plans) ([]domain.Service, error) {
	var service domain.Service
	boolTrue := true

	service.ID = config.ServiceGUID
	service.Name = config.ServiceName
	service.Description = config.ServiceDescription
	service.Bindable = true
	service.BindingsRetrievable = false
	service.InstancesRetrievable = true
	service.Tags = config.ServiceTags
	service.PlanUpdatable = true
	service.Metadata = &domain.ServiceMetadata{
		DisplayName:         config.ServiceName,
		ImageUrl:            LoadCatalogImage(),
		LongDescription:     config.ServiceDescription,
		ProviderDisplayName: "",
		DocumentationUrl:    config.DocsURL,
		SupportUrl:          "",
		Shareable:           &boolTrue,
		AdditionalMetadata:  map[string]interface{}{},
	}

	for planName, plan := range plans {
		service.Plans = append(service.Plans, domain.ServicePlan{
			ID:          plan.ID,
			Name:        planName,
			Description: plan.Description,
			Free:        plan.Free,
			Bindable:    &boolTrue,
			Metadata: &domain.ServicePlanMetadata{
				DisplayName: plan.DisplayName,
			},
			Schemas: &domain.ServiceSchemas{},
			MaintenanceInfo: &domain.MaintenanceInfo{
				Version:     plan.MaintenanceInfo.Version,
				Description: plan.MaintenanceInfo.Description,
			},
		})
	}

	services := []domain.Service{service}

	return services, nil
}
