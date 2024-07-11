# Go Gin Web Service with Observability
This is a demo project to show how to use LGTM (Loki, Grafana, Tempo, Mimir) to monitor Go Gin web service.
## Background
When we are at iKala, the biggest challenge we face is deciding what to eat for lunch with your colleagues ~~or can we survive in falling elevator~~. </p>
To simulate this issue, we have created the demo simulator. This demo service includes the following APIs:

- `[POST] /lunch-decision/:food`: Simulates the lunch decision-making process. You will receive the outcome of the decision.
- `[GET]  /person`               : Represents your colleague who will give you their decision about the lunch option you provided.


---

## Setup
1. Install Loki Docker Driver
```bash
docker plugin install grafana/loki-docker-driver:latest --alias loki --grant-all-permissions 
```
2. build binary
```bash
cd demo
GOOS=linux GOARCH=amd64 go build main.go 
```
3. Start LGTM and demo service with docker-compose
```bash
docker compose up -d
```
4. If got the error message Error response from daemon: error looking up logging plugin loki: plugin loki found but disabled, please run the following command to enable the plugin:
```bash
docker plugin enable loki
```

## Development
```bash
cd demo
SERVICE_NAME=ExampleService PERSON_LIST=localhost CONSIDERATION_TIME_MILLISECONDS=100 go run main.go
```

## Run Testing
```bash
cd demo
K6_PROMETHEUS_RW_SERVER_URL=http://localhost:9090/api/v1/write \
K6_PROMETHEUS_RW_TREND_STATS=p(90),p(95),p(99),min,max \
K6_WEB_DASHBOARD=true \
K6_WEB_DASHBOARD_EXPORT=html-report.html \
k6 run -o experimental-prometheus-rw script.js  --tag testid="demo-$(date +%s)"
```


# Reference
- [blueswen / fastapi-observability](https://github.com/blueswen/fastapi-observability)
