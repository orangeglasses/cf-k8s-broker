package main

import (
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"
)

func main() {
	var logLevels = map[string]lager.LogLevel{
		"DEBUG": lager.DEBUG,
		"INFO":  lager.INFO,
		"ERROR": lager.ERROR,
		"FATAL": lager.FATAL,
	}

	config, plans, err := brokerConfigLoad()
	if err != nil {
		panic(err)
	}

	logger := lager.NewLogger("cf-k8s-broker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, logLevels[config.LogLevel]))

	services, err := CatalogLoad(config, plans)
	if err != nil {
		panic(err)
	}

	for i := range services {
		services[i].Metadata.DocumentationUrl = config.DocsURL
	}

	logger.Info("Catalog Loaded")

	klient := NewK8sClient(config.KubeconfigPath, config.TemplatesPath)

	logger.Info("Kubernetes Client Created")

	serviceBroker := &broker{
		logger:   logger,
		services: services,
		env:      config,
		plans:    plans,
		klient:   *klient,
	}

	brokerHandler := brokerapi.New(serviceBroker, logger, brokerapi.BrokerCredentials{
		Username: config.BrokerUsername,
		Password: config.BrokerPassword,
	})
	logger.Info("Starting Broker Now")
	http.Handle("/", brokerHandler)
	http.ListenAndServe(":"+config.Port, nil)
}
