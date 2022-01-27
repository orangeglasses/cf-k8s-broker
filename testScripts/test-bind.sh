curl http://broker:broker@localhost:3000/v2/service_instances/90349a10-7309-4ddc-999b-ef7d851c55c0/service_bindings/c63606aa-54d4-4037-93e8-56da7000ba5e?accepts_incomplete=true -d '{
  "context": {
    "platform": "cloudfoundry"  
  },
  "service_id": "3c318f49-9fa1-4a40-a1d6-10a10509ef54",
  "plan_id": "9ffb5654-b0ff-4215-8519-70daf5e79b0f",
  "bind_resource": {
    "app_guid": "a31fec23-a86b-4d3a-87d2-f44b620b9c04"
  },
  "parameters": {
    "parameter1-name-here": 1,
    "parameter2-name-here": "parameter2-value-here"
  }
}' -X PUT -H "X-Broker-API-Version: 2.16" -H "Content-Type: application/json"