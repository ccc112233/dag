# 插件开发说明

数采服务（`dap`）通过进程内插件扩展南向驱动与北向通道。核心流水线（`internal/pipeline`）与存储（`internal/store`）不感知具体协议。

## 南向 `SouthDriver`

实现 `internal/plugin` 中的接口：

- `Type() string`：配置里的 `driverType`
- `ValidateConfig(cfg json.RawMessage) error`
- `Open(ctx, cfg, creds) (Session, error)`
- `Session.Read` / `Session.Subscribe` / `Session.Close`

示例：`plugins/south/stub`、`plugins/south/modbus`、`plugins/south/opcua`。

注册：在 `cmd/dap/plugins.go` 增加一行 `reg.RegisterSouth(...)`。

### 配置要点

| type | endpoint 关键字段 |
|------|-------------------|
| `stub_south` | `startValue`, `step` |
| `modbus_tcp` | `address`(`host:port`), `slaveId`, `timeoutMs`；点位 address：`registerType`,`address`,`quantity`,`dataType` |
| `opcua` | `endpoint`, `insecure`；点位 address：`nodeId` |
| `stub_south_fail` | 任意（测试隔离） |

凭证：`credentialsRef` 支持 `env:VAR` 或 `file:/path`（见 `plugin.ResolveCredentials`）。

## 北向 `NorthChannel`

实现：

- `Type() string`：配置里的 `channelType`
- `ValidateConfig` / `Open` → `Publisher.Publish` / `Close`

示例：`plugins/north/stub`、`http`、`mqtt`、`amqp`。

注册：`cmd/dap/plugins.go` 中 `reg.RegisterNorth(...)`。

| type | config 关键字段 |
|------|----------------|
| `stub_north` | `path`（可选追加 JSONL），`failOnce`（测试缓冲） |
| `http` | `url`, `method`, `headers`, `timeoutMs` |
| `mqtt` | `broker`, `topic`（可用 `{deviceId}`）, `qos`, `clientId` |
| `amqp` | `url`, `exchange`, `routingKey`, `queue` |

## 扩展验收（AC-E1/E2）

1. 新增包实现接口。
2. 仅修改 `cmd/dap/plugins.go` 注册。
3. 通过管理 API 创建连接/通道并启用；不修改 `internal/pipeline` / `internal/store`。
