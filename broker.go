package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
)

type broker struct {
	logger   lager.Logger
	services []brokerapi.Service
	env      brokerConfig
	plans    Plans
	klient   k8sclient
}

func (b *broker) Services(ctx context.Context) ([]domain.Service, error) {
	return b.services, nil
}

func processUserParams(params map[string]interface{}, plan *Plan, ignoreUnknown bool) error {
	for paramName, param := range params {
		planConfig, ok := plan.Config[paramName]
		if !ok {
			if ignoreUnknown {
				continue
			}

			return apiresponses.ErrRawParamsInvalid
		}

		if !planConfig.UserOverride {
			return fmt.Errorf("param: %s is not user overridable", paramName)
		}

		planConfig.Value = param
		plan.Config[paramName] = planConfig
	}

	for paramName, planConfig := range plan.Config {
		if planConfig.Required {
			if planConfig.Value == nil {
				return fmt.Errorf("Required parameter '%v' not provided", paramName)
			}

			switch planConfig.Value.(type) {
			case string:
				if planConfig.Value.(string) == "" {
					return fmt.Errorf("Required parameter '%v' not provided", paramName)
				}
			}
		}
	}

	return nil
}

func (b *broker) Provision(ctx context.Context, instanceID string, details domain.ProvisionDetails, asyncAllowed bool) (domain.ProvisionedServiceSpec, error) {
	if !asyncAllowed {
		return domain.ProvisionedServiceSpec{}, apiresponses.ErrAsyncRequired
	}

	//get plan name from plan ID
	planName := b.env.PlansRevGUIDMap[details.PlanID]

	//get plan
	plan := b.plans[planName]

	//Process user params
	params := make(map[string]interface{})
	if details.RawParameters != nil && len(details.RawParameters) > 0 {
		err := json.Unmarshal(details.RawParameters, &params)
		if err != nil {
			return domain.ProvisionedServiceSpec{}, apiresponses.ErrRawParamsInvalid
		}
		err = processUserParams(params, &plan, false)
		if err != nil {
			return domain.ProvisionedServiceSpec{}, err
		}
	}

	renderedYaml, err := b.klient.RenderTemplatesForPlan(ctx, plan, details.OrganizationGUID, details.SpaceGUID, instanceID)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("Error rendering YAML templates: %s", err)
	}

	//Track applied files and rollback on first failure
	var applied []string
	for _, ry := range renderedYaml {
		if err := b.klient.CreateFromYaml(ctx, ry, instanceID); err != nil {
			b.logger.Error("Error applying YAML, rolling back", err)

			for _, a := range applied {
				b.klient.DeleteFromYaml(ctx, a, instanceID, true)
			}

			return domain.ProvisionedServiceSpec{}, err
		}

		applied = append(applied, ry)
	}

	if len(params) > 0 {
		fmt.Println(params)
		err = b.klient.StoreUserParams(ctx, instanceID, params)
		if err != nil {
			return domain.ProvisionedServiceSpec{}, fmt.Errorf("Unable to store user params: %s", err)
		}
	}

	return domain.ProvisionedServiceSpec{
		IsAsync:       true,
		AlreadyExists: false,
		DashboardURL:  "",
		OperationData: time.Now().UTC().String(),
	}, nil
}

func (b *broker) GetInstance(ctx context.Context, instanceID string) (domain.GetInstanceDetailsSpec, error) {
	if b.klient.getInstanceTemplate == nil {
		return domain.GetInstanceDetailsSpec{}, fmt.Errorf("Instance retrieval not configured")
	}

	templ, err := b.klient.RenderGetInstanceTemplate(ctx, instanceID)
	if err != nil {
		b.logger.Error("unable to render getInstance template: ", err)
		return domain.GetInstanceDetailsSpec{}, fmt.Errorf("Unable to render getInstance template: %s", err)
	}

	return domain.GetInstanceDetailsSpec{
		ServiceID:    b.env.ServiceGUID,
		PlanID:       templ["planid"].(string),
		DashboardURL: templ["dashboardurl"].(string),
		Parameters:   templ["parameters"],
	}, nil
}

func (b *broker) Deprovision(ctx context.Context, instanceID string, details domain.DeprovisionDetails, asyncAllowed bool) (domain.DeprovisionServiceSpec, error) {
	planName := b.env.PlansRevGUIDMap[details.PlanID]
	renderedYaml, err := b.klient.RenderTemplatesForPlan(ctx, b.plans[planName], "", "", instanceID)
	if err != nil {
		b.logger.Error("Error rendering YAML templates for deprovision: ", err)

		return domain.DeprovisionServiceSpec{}, nil
	}

	for _, ry := range renderedYaml {
		if err := b.klient.DeleteFromYaml(ctx, ry, instanceID, details.Force); err != nil {
			b.logger.Error("Error deleting from YAML:", err)
		}
	}

	b.klient.DeleteStoredUserParams(ctx, instanceID)

	return domain.DeprovisionServiceSpec{}, nil
}

func (b *broker) Bind(ctx context.Context, instanceID, bindingID string, details domain.BindDetails, asyncAllowed bool) (domain.Binding, error) {
	templ, err := b.klient.RenderBindTemplate(ctx, instanceID)
	if err != nil {
		b.logger.Error("unable to render bind template: ", err)
		return domain.Binding{}, fmt.Errorf("Unable to render bind template: %s", err)
	}

	binding := domain.Binding{
		Credentials: templ,
	}

	return binding, nil
}

func (b *broker) GetBinding(ctx context.Context, instanceID, bindingID string) (domain.GetBindingSpec, error) {
	return domain.GetBindingSpec{}, fmt.Errorf("Bindings are not retrievable")
}

func (b *broker) Unbind(ctx context.Context, instanceID, bindingID string, details domain.UnbindDetails, asyncAllowed bool) (domain.UnbindSpec, error) {
	return domain.UnbindSpec{}, nil
}

func (b *broker) Update(ctx context.Context, instanceID string, details domain.UpdateDetails, asyncAllowed bool) (domain.UpdateServiceSpec, error) {
	if !asyncAllowed {
		return domain.UpdateServiceSpec{}, apiresponses.ErrAsyncRequired
	}

	if details.PlanID != details.PreviousValues.PlanID && !b.env.PlanChangeSSupported {
		return domain.UpdateServiceSpec{}, apiresponses.ErrPlanChangeNotSupported
	}

	//get plan name from plan ID
	planName := b.env.PlansRevGUIDMap[details.PlanID]

	//get plan
	plan := b.plans[planName]

	//get previous params
	previousParams := b.klient.GetPreviousUserParams(ctx, instanceID)

	//marshal the incoming params
	var newParams map[string]interface{}
	if details.RawParameters != nil && len(details.RawParameters) > 0 {
		err := json.Unmarshal(details.RawParameters, &newParams)
		if err != nil {
			return domain.UpdateServiceSpec{}, apiresponses.ErrRawParamsInvalid
		}

		//And the merge them with previous params
		for newKey, newParam := range newParams {
			if newParam != nil {
				if newParamString, ok := newParam.(string); ok && newParamString != "" {
					previousParams[newKey] = newParam
				}
			}

		}
	}

	//Process user params for update
	ignoreUnknown := (details.PlanID != details.PreviousValues.PlanID) //if changing plans then it's fine to have parameters stored that are not in the plans.json
	err := processUserParams(previousParams, &plan, ignoreUnknown)
	if err != nil {
		return domain.UpdateServiceSpec{}, err
	}

	//store the used params before applying so we can choose not to do the update when storing the params fails
	if len(previousParams) > 0 {
		err = b.klient.StoreUserParams(ctx, instanceID, previousParams)
		if err != nil {
			return domain.UpdateServiceSpec{}, err
		}
	}

	renderedYaml, err := b.klient.RenderTemplatesForPlan(ctx, plan, details.PreviousValues.OrgID, details.PreviousValues.SpaceID, instanceID)
	if err != nil {
		return domain.UpdateServiceSpec{}, fmt.Errorf("Error rendering YAML templates: %s", err)
	}

	for _, ry := range renderedYaml {
		if err := b.klient.UpdateFromYaml(ctx, ry, instanceID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				err = b.klient.CreateFromYaml(ctx, ry, instanceID)
				if err != nil {
					b.logger.Error("Error Creating new object", err)
					return domain.UpdateServiceSpec{}, err
				}
			} else {

				b.logger.Error("Error applying YAML", err)
				return domain.UpdateServiceSpec{}, err
			}
		}
	}

	return domain.UpdateServiceSpec{
		IsAsync:       true,
		DashboardURL:  "",
		OperationData: time.Now().UTC().String(),
	}, nil
}

func (b *broker) LastOperation(ctx context.Context, instanceID string, details domain.PollDetails) (brokerapi.LastOperation, error) {
	//Check timeout
	startedTime, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", details.OperationData)
	if startedTime.Add(time.Minute * 60).Before(time.Now().UTC()) {
		//timeout expired
		return brokerapi.LastOperation{
			State:       domain.Failed,
			Description: "Timeout (60 minutes) expired",
		}, fmt.Errorf("Deployment timed out")
	}

	//render files
	planName := b.env.PlansRevGUIDMap[details.PlanID]
	renderedYaml, err := b.klient.RenderTemplatesForPlan(ctx, b.plans[planName], "", "", instanceID)
	if err != nil {
		return brokerapi.LastOperation{}, fmt.Errorf("Error rendering YAML templates: %s", err)
	}

	//getobject status
	for _, ry := range renderedYaml {
		obj, err := b.klient.GetObject(ctx, instanceID, ry)
		if err != nil {
			b.logger.Debug("LastOperation: could not get object: " + err.Error())
			return brokerapi.LastOperation{}, err
		}

		wait, ok, err := b.klient.GetObjectStatus(obj)
		if wait {
			if err != nil {
				b.logger.Debug("LastOperation: deploy error: " + err.Error())
				return brokerapi.LastOperation{
					State:       domain.Failed,
					Description: err.Error(),
				}, nil
			}

			if !ok {
				b.logger.Debug("LastOperation: deploy still running")
				return brokerapi.LastOperation{
					State:       domain.InProgress,
					Description: "",
				}, nil
			}
		}
	}

	return brokerapi.LastOperation{
		State:       domain.Succeeded,
		Description: "",
	}, nil
}

func (b *broker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details domain.PollDetails) (domain.LastOperation, error) {
	return domain.LastOperation{}, nil
}
