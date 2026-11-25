# 运行手册

## 构建与启动

```bash
go test ./...
go run ./cmd/dap --config ./configs/example.yaml
```

默认监听 `:8080`，Token `dev-token`（可用环境变量 `DAP_API_TOKEN` 覆盖）。

健康检查：

```bash
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
```

鉴权头：`Authorization: Bearer <token>`。

## 配置备份（P-03）

复制 SQLite 文件（默认 `dap.db`，含 `-wal`/`-shm`）即可备份配置与北向缓冲。
本地时序采样缓存目录默认为 `data/ts`（按日分区 `.pts` 文件，默认保留 7 天）；一并备份即可。

## 本地时序缓存

单服务目标：**≥ 40000 点/秒**写入；保留期由 `tsCache.retentionDays` 控制（默认 7）。

```bash
curl -s localhost:8080/api/v1/ts/stats -H "$AUTH"
curl -s 'localhost:8080/api/v1/ts/query?pointId=p1&limit=100' -H "$AUTH"
```

压测入口：`go test ./internal/tscache/ -count=1 -run TestWriteThroughput40kPerSec -v`

## 最小联调（stub）

```bash
TOKEN=dev-token
AUTH="Authorization: Bearer $TOKEN"

# 南向连接 + 点位 + 任务
curl -s -X POST localhost:8080/api/v1/south/connections -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"id":"c1","name":"stub","driverType":"stub_south","endpoint":{"startValue":1,"step":1},"timeoutMs":3000}'

curl -s -X POST localhost:8080/api/v1/south/points -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"id":"p1","connectionId":"c1","name":"temp","address":{"n":1},"dataType":"float"}'

curl -s -X POST localhost:8080/api/v1/south/tasks -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"id":"t1","connectionId":"c1","mode":"poll","intervalMs":500,"pointSet":["p1"]}'

# 物模型 / 设备 / 映射
curl -s -X POST localhost:8080/api/v1/models -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"modelId":"motor","version":"1.0.0","name":"Motor","properties":[{"id":"temp","name":"T","dataType":"float"},{"id":"rpm","name":"RPM","dataType":"float"},{"id":"on","name":"On","dataType":"bool"}]}'

curl -s -X POST localhost:8080/api/v1/devices -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"id":"d1","deviceId":"dev-1","modelId":"motor","modelVersion":"1.0.0","name":"M1"}'

curl -s -X POST localhost:8080/api/v1/mappings -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"deviceId":"d1","pointId":"p1","propertyId":"temp","scale":1,"offset":0}'

# 北向 stub
curl -s -X POST localhost:8080/api/v1/north/channels -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"id":"n1","name":"stub","channelType":"stub_north","config":{"path":"/tmp/dap-north.jsonl"},"enabled":true,"trigger":"change"}'

curl -s -X POST localhost:8080/api/v1/north/channels/n1/bindings -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"deviceId":"d1"}'

curl -s -X POST localhost:8080/api/v1/south/tasks/t1/start -H "$AUTH"
```

查看 `/tmp/dap-north.jsonl` 或任务指标：

```bash
curl -s localhost:8080/api/v1/south/tasks/t1/metrics -H "$AUTH"
```

## 可选依赖（compose）

```bash
docker compose -f deployments/compose.yaml up -d
```

提供 Mosquitto、RabbitMQ，以及可选的 Modbus/OPC UA 模拟说明见 compose 注释。

## 容器构建

```bash
docker build -t dap:dev -f deployments/Dockerfile .
```
