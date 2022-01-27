package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/kelseyhightower/envconfig"
)

type brokerConfig struct {
	BrokerUsername       string            `envconfig:"broker_username" required:"true"`
	BrokerPassword       string            `envconfig:"broker_password" required:"true"`
	ServiceGUID          string            `envconfig:"service_guid" required:"false"`
	ServiceName          string            `envconfig:"service_name" default:"postgres-db"`
	ServiceDescription   string            `envconfig:"service_desc" default:"postgres-db on k8s"`
	ServiceTags          []string          `envconfig:"service_tags" rquired:"false"`
	PlansPath            string            `envconfig:"service_plans_path" default:"plans.json"`
	PlansGUIDMap         map[string]string `envconfig:"service_plans_mapping" required:"false"`
	PlansRevGUIDMap      map[string]string
	PlanChangeSSupported bool   `envconfig:"plan_change_supported" default:"false"`
	TemplatesPath        string `envconfig:"templates_path" default:"templates"`
	KubeconfigPath       string `envconfig:"kubeconfig_path" default:""`
	LogLevel             string `envconfig:"log_level" default:"INFO"`
	Port                 string `envconfig:"port" default:"3000"`
	DocsURL              string `envconfig:"docsurl" default:"default"`
}

func brokerConfigLoad() (brokerConfig, Plans, error) {
	var config brokerConfig
	var plans Plans
	config.PlansRevGUIDMap = make(map[string]string)

	err := envconfig.Process("", &config)
	if err != nil {
		return brokerConfig{}, nil, err
	}

	inBuf, err := ioutil.ReadFile(config.PlansPath)
	if err != nil {
		return brokerConfig{}, nil, err
	}

	err = json.Unmarshal(inBuf, &plans)
	if err != nil {
		fmt.Println(err)
		return brokerConfig{}, nil, err
	}

	//if no UUID is given in the plan check if there is an entry in the map or else throw an error.
	for planName, plan := range plans {

		if plan.ID == "" {
			if id, ok := config.PlansGUIDMap[planName]; ok {
				plan.ID = id
			} else {
				log.Fatalf("No ID and no mapping given for plan %s", planName)
			}
		}

		plan.Name = planName
		plans[planName] = plan
		config.PlansRevGUIDMap[plan.ID] = planName //populate reverse guid map so we can find plans by ID easily
	}

	return config, plans, nil
}
