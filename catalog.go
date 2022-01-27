package main

import (
	"github.com/google/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/brokerapi/domain"
)

func CatalogLoad(config brokerConfig, plans Plans) ([]domain.Service, error) {
	var service brokerapi.Service
	boolTrue := true

	serviceGUID := config.ServiceGUID
	if serviceGUID == "" {
		serviceGUID = uuid.NewString()
	}
	service.ID = serviceGUID
	service.Name = config.ServiceName
	service.Description = config.ServiceDescription
	service.Bindable = true
	service.BindingsRetrievable = false
	service.InstancesRetrievable = false
	service.Tags = config.ServiceTags
	service.PlanUpdatable = true
	service.Metadata = &domain.ServiceMetadata{
		DisplayName:         config.ServiceName,
		ImageUrl:            "", //todo: put image here
		LongDescription:     config.ServiceDescription,
		ProviderDisplayName: "Postgres",
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
