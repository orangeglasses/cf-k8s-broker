# cf-k8s-broker
This is a OSBAPI compliant broker. It was developed specifically to offer posstgres on kubernetes as a service in the cloudfoundry marketplace. Since this broker just renders and applies yaml it should work for other products as well with no or minor changes. It probably won't win any beauty contests but it works :)

## Configuring the broker

### Environment variables
There are a few different places where you'll need to provide configuration. The following config needs to be provided through environment variables:

| Variable    | required    | Description |
| ----------- | ----------- | ----------- |
| broker_username |yes| Username for the broker, used by CF to access this broker|
| broker_password |yes| Password for the broker, used in combination with the broker_username|
| service_guid |yes| The GUID for the service. You can generate a UUID and put that here|
| service_name |yes| The name of the Service. e.g: Postgres |
| service_desc |No| A description for the service |
| service_tags |No| Tags attached to the service in the VCAP_SERVICES json |
| service_plans_path|No| Path to plans.json, default: "plans.json" in working dir |
| plan_change_supported|No| If changing between plans is supported. Default to false |
| templates_path|No| Path to kubernetes yaml templates. Defaults to "templates" folder in working dir |
| kubeconfig_path|No| Path to kubeconfig file needed to access the k8s cluster. Defaults to .kube/config |
| log_level|No| Logging level, Defaults to INFO |
| docsurl|No| A url to the docs for this service. Set it to an internal wiki, confluence or whater you have. Defaults to the string "default" |

### plans.json
Then there is the plans.json which contains the service plans offered in the marketplace. The plans.json contains an object for each plan. The key for the object is also the plan name. Here is the structure of the json:

```
{ 
  "plan-name": {    
    "id": "plan-id",
    "description": "plan-description",
    "free": true or false,      
    "displayName": "plan dispaly name",
    "maintenance_info": {
      "version": "semver version number",
      "description": "version description"
    },
    "config": {
      "param name": {
        "value": "param default value",
        "userOverride": true/false (allow user override through "cf create-service ... -c '{}'" 
      }
   }
}

```

Let's take a look at an example:
```
{ 
  "db-small": {    
    "id": "3c318f49-9fa1-4a40-a1d6-10a10509ef54-db-small",
    "description": "Small pg db",
    "free": true,      
    "displayName": "DB-Small",
    "maintenance_info": {
      "version": "0.0.1",
      "description": "postgres 1.3"
    },
    "config": {
      "diskSize": {
        "value": "5Gi",
        "userOverride": true
      },
      "cpu": {
        "value": "500m",
        "userOverride": true
      },
      "ram": {
        "value": "512Mi",
        "userOverride": true        
      },
      "replicas": {
        "value": 1,
        "userOverride": true
      }
  }      
  },
  "db-medium": {    
    "id": "3c318f49-9fa1-4a40-a1d6-10a10509ef54-db-medium",
    "description": "Medium pg db",
    "free": true,      
    "displayName": "DB-Medium",
    "maintenance_info": {
      "version": "0.0.1",
      "description": "postgres 1.3"
    },
    "config": {
        "diskSize": {
          "value": "10Gi",
          "userOverride": true
        },
        "cpu": {
          "value": "800m",
          "userOverride": true
        },
        "ram": {
          "value": "1Gi",
          "userOverride": false        
        },
        "replicas": {
          "value": 2,
          "userOverride": true
        }
    }      
  }
}
```
For the plan ID you can generate a UUID or come up with something else as long as it's unique within this broker. Plan name, display name and description are self explanetory. The "free" attribute determinse if this plan is considered a pai plan or not. 
The maintenance_info object sets a version number (semver) and a service descrption. This service version is used by CF to determine if there are updates available for the plan. You can then run these upgrades by invokin "cf upgrade-service". So whenever you update any of your yamls templates you'll want to bump this version so users can upgrade their service instances and the new versions of the yamls will be rendered and applied for them.
The config object holds all the variables and their values. These variables can then be used in the yaml templates. The uverOverride attribute determines if the parameter is overrideable by the platform user. If this is set to true then the user can provide their own value using the "-c" argument with "cf create-service" or "cf update-service"

### yaml templates
You'll have to provide a folder container yml files. You'll find a few examples in the templates folder in the repo. The yaml templates are parsed using golang templating. The following functions are available in the templates:

- GetObjectByLabel (mostly used in bindTemplate.json, see that section for more info) 
- GetObjectByName (mostly used in bindTemplate.json, see that section for more info) 
- base64decode
- .Plan.GetConfigValue (example: {{ .Plan.GetConfigValue "cpu" }} <- this will retrieve the value of the "cpu" parameter from the plans.json or as provided by the user if userOverride was enabled)

The templates are applied in alphetecial order. So if you need anything (like a namespace defintion) to be applied first you can prefix the yaml filenames with number as in the example

### bindTemplate.json
This tempalte will be rendered whenever a service instance is bound to an app or when a service-key is created. The bindTemplate.json in this repo was made fot VMwares tanzu postgres on kubernetes. please take a look at the bindTemplate.json to see how it ca be used.

### logo.png
The broker will try to load  logo.png from the working dir. This has to be a 256x256 pixel PNG. This will be loaded by CF and displayed in CF GUIs like apps manager or strator.

## Installing the broker
Clone this repo. Edit the manifest.yml file to include all required environment variables, then edit bindTemplate.json, plans.json and the yaml templates in the templates folder to your liking. The run "cf push".
when the apps is running you can connect CF to the broker by running "cf create-service-broker". Use the username/password as configured in the env vars.

## Using the broker
Once the broker is registered with CF you can enable service access using the "cf enable-service-access" command. After that you'll find the service in the marketplace and you can now request the service using "cf create-service". 
