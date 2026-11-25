/** Shared south/north protocol registry metadata + JSON Schema. */

export const protocols = [
  {
    id: "modbus-tcp",
    direction: "south",
    displayName: "Modbus TCP",
    version: "1.0.0",
    enabled: true,
    capabilities: ["pointMapping", "pollInterval", "testConnection", "simulator"],
    helpMarkdown: "通过 TCP 连接 Modbus 从站。验收可用模拟器模式。",
    configSchema: {
      $schema: "https://json-schema.org/draft/2020-12/schema",
      type: "object",
      required: ["host", "port", "unitId"],
      properties: {
        host: { type: "string", title: "主机地址", minLength: 1 },
        port: { type: "integer", title: "端口", minimum: 1, maximum: 65535, default: 502 },
        unitId: { type: "integer", title: "从站号", minimum: 1, maximum: 247, default: 1 },
        timeoutMs: { type: "integer", title: "超时(ms)", minimum: 100, default: 3000 },
        useSimulator: {
          type: "boolean",
          title: "使用模拟器",
          default: true,
          description: "无真实设备时启用模拟连接",
        },
      },
      additionalProperties: false,
    },
    uiSchema: {},
    pointSchema: {
      type: "object",
      required: ["address", "registerType"],
      properties: {
        address: { type: "integer", title: "寄存器地址", minimum: 0 },
        registerType: {
          type: "string",
          title: "寄存器类型",
          enum: ["holding", "input", "coil", "discrete"],
          default: "holding",
        },
        pollIntervalMs: { type: "integer", title: "采集周期(ms)", minimum: 200, default: 1000 },
      },
    },
  },
  {
    id: "opcua",
    direction: "south",
    displayName: "OPC UA",
    version: "1.0.0",
    enabled: true,
    capabilities: ["pointMapping", "subscription", "testConnection", "simulator"],
    helpMarkdown: "连接 OPC UA 服务器。可用模拟器完成测试连接。",
    configSchema: {
      $schema: "https://json-schema.org/draft/2020-12/schema",
      type: "object",
      required: ["endpointUrl"],
      properties: {
        endpointUrl: {
          type: "string",
          title: "Endpoint URL",
          minLength: 1,
          default: "opc.tcp://127.0.0.1:4840",
        },
        securityMode: {
          type: "string",
          title: "安全模式",
          enum: ["None", "Sign", "SignAndEncrypt"],
          default: "None",
        },
        username: { type: "string", title: "用户名" },
        password: { type: "string", title: "密码", writeOnly: true },
        useSimulator: { type: "boolean", title: "使用模拟器", default: true },
      },
      additionalProperties: false,
    },
    uiSchema: {
      password: { "ui:widget": "password" },
    },
    pointSchema: {
      type: "object",
      required: ["nodeId"],
      properties: {
        nodeId: { type: "string", title: "NodeId", minLength: 1 },
        samplingIntervalMs: {
          type: "integer",
          title: "采样间隔(ms)",
          minimum: 100,
          default: 1000,
        },
      },
    },
  },
  {
    id: "http",
    direction: "north",
    displayName: "HTTP Webhook",
    version: "1.0.0",
    enabled: true,
    capabilities: ["mapping", "testSend", "simulator"],
    helpMarkdown: "按物模型可上报属性组装 JSON，POST/PUT 到上级平台。",
    configSchema: {
      $schema: "https://json-schema.org/draft/2020-12/schema",
      type: "object",
      required: ["url", "method"],
      properties: {
        url: { type: "string", title: "URL", minLength: 1 },
        method: {
          type: "string",
          title: "方法",
          enum: ["POST", "PUT"],
          default: "POST",
        },
        headersJson: {
          type: "string",
          title: "额外 Headers (JSON)",
          default: "{}",
        },
        authToken: { type: "string", title: "Bearer Token", writeOnly: true },
        useSimulator: { type: "boolean", title: "模拟发送", default: true },
      },
      additionalProperties: false,
    },
    uiSchema: {
      authToken: { "ui:widget": "password" },
    },
  },
  {
    id: "mqtt",
    direction: "north",
    displayName: "MQTT",
    version: "1.0.0",
    enabled: true,
    capabilities: ["mapping", "simulator"],
    helpMarkdown: "将载荷发布到指定 Topic。",
    configSchema: {
      $schema: "https://json-schema.org/draft/2020-12/schema",
      type: "object",
      required: ["brokerUrl", "topic"],
      properties: {
        brokerUrl: {
          type: "string",
          title: "Broker URL",
          default: "mqtt://127.0.0.1:1883",
        },
        topic: { type: "string", title: "Topic", minLength: 1 },
        qos: { type: "integer", title: "QoS", enum: [0, 1, 2], default: 1 },
        clientId: { type: "string", title: "Client ID" },
        username: { type: "string", title: "用户名" },
        password: { type: "string", title: "密码", writeOnly: true },
        useSimulator: { type: "boolean", title: "模拟发送", default: true },
      },
      additionalProperties: false,
    },
    uiSchema: {
      password: { "ui:widget": "password" },
    },
  },
  {
    id: "kafka",
    direction: "north",
    displayName: "Kafka（即将支持）",
    version: "0.0.0",
    enabled: false,
    capabilities: [],
    helpMarkdown: "消息队列北向本期仅占位，不进入验收。",
    configSchema: {
      type: "object",
      properties: {
        brokers: { type: "string", title: "Brokers" },
      },
    },
    uiSchema: {},
  },
];

export function listProtocols(direction) {
  return protocols.filter((p) => !direction || p.direction === direction);
}

export function getProtocol(id) {
  return protocols.find((p) => p.id === id) || null;
}

export const SECRET_KEYS = new Set(["password", "authToken"]);

export function redactConfig(config = {}) {
  const out = { ...config };
  const secrets = {};
  for (const key of Object.keys(out)) {
    if (SECRET_KEYS.has(key) || /password|token|secret|privateKey/i.test(key)) {
      const has = out[key] != null && String(out[key]).length > 0;
      secrets[key] = has;
      delete out[key];
    }
  }
  return { config: out, secrets };
}

export function mergeConfigPatch(existing = {}, patch = {}) {
  const next = { ...existing };
  for (const [k, v] of Object.entries(patch)) {
    if (SECRET_KEYS.has(k) || /password|token|secret|privateKey/i.test(k)) {
      if (v === undefined) continue;
      if (v === "") {
        delete next[k];
        continue;
      }
      next[k] = v;
      continue;
    }
    next[k] = v;
  }
  return next;
}
