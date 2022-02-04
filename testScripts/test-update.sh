curl http://broker:broker@localhost:3000/v2/service_instances/90349a10-7309-4ddc-999b-ef7d851c55c0?accepts_incomplete=true -d '{
  "service_id": "3c318f49-9fa1-4a40-a1d6-10a10509ef54",
  "plan_id": "3c318f49-9fa1-4a40-a1d6-10a10509ef54-db-small",
  "context": {
    "platform": "cloudfoundry"
  },
  "parameters": { "cpu": "200m"},
  "organization_guid": "c0eda3a0-a224-4985-9e50-6c6b9a4a9115",
  "space_guid": "21284559-5dfb-4e72-98fc-16cc92b2012e",
  "previous_values": {
      "plan_id": "3c318f49-9fa1-4a40-a1d6-10a10509ef54-db-small"
  }
}' -X PATCH -H "X-Broker-API-Version: 2.16" -H "Content-Type: application/json"