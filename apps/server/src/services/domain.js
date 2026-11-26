import {
  getProtocol,
  listProtocols,
  redactConfig,
  mergeConfigPatch,
} from "@dag/protocol-meta";

const DATA_TYPES = new Set(["bool", "int", "float", "string", "enum"]);

export function createDomain(store) {
  function publicChannel(ch) {
    const { config, secrets } = redactConfig(ch.config);
    return {
      id: ch.id,
      name: ch.name,
      protocolId: ch.protocolId,
      config,
      secrets,
      status: ch.status,
      lastError: ch.lastError,
      lastSampleAt: ch.lastSampleAt,
      enabled: ch.enabled,
      createdAt: ch.createdAt,
      updatedAt: ch.updatedAt,
    };
  }

  function publicNorth(t) {
    const { config, secrets } = redactConfig(t.config);
    return {
      id: t.id,
      name: t.name,
      protocolId: t.protocolId,
      config,
      secrets,
      mapping: t.mapping,
      status: t.status,
      enabled: t.enabled,
      successCount: t.successCount,
      failCount: t.failCount,
      lastResult: t.lastResult,
      createdAt: t.createdAt,
      updatedAt: t.updatedAt,
    };
  }

  function requireAuth(req) {
    const cookies = req.cookies || {};
    const auth = req.headers.authorization || "";
    const bearer = auth.startsWith("Bearer ") ? auth.slice(7) : "";
    const token = cookies.dag_session || bearer;
    if (!store.raw.sessionToken || token !== store.raw.sessionToken) {
      const err = new Error("未登录或会话已失效");
      err.status = 401;
      err.code = "UNAUTHORIZED";
      throw err;
    }
  }

  return {
    requireAuth,
    login(username, password) {
      if (!username || !password) {
        const err = new Error("用户名和密码必填");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      // MVP local admin
      if (username !== "admin" || password !== "admin") {
        const err = new Error("用户名或密码错误");
        err.status = 401;
        err.code = "UNAUTHORIZED";
        throw err;
      }
      const token = store.id();
      store.raw.sessionToken = token;
      store.persist();
      return { token, user: { username: "admin", displayName: "管理员" } };
    },
    logout() {
      store.raw.sessionToken = null;
      store.persist();
    },
    listProtocols(direction) {
      return listProtocols(direction).map((p) => ({
        id: p.id,
        direction: p.direction,
        displayName: p.displayName,
        version: p.version,
        enabled: p.enabled,
        capabilities: p.capabilities,
        helpMarkdown: p.helpMarkdown,
        configSchema: p.configSchema,
        uiSchema: p.uiSchema,
        pointSchema: p.pointSchema || null,
      }));
    },
    getProtocol(id) {
      const p = getProtocol(id);
      if (!p) {
        const err = new Error("协议不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      return {
        ...p,
        pointSchema: p.pointSchema || null,
      };
    },
    listSouth({ q, status } = {}) {
      let items = store.raw.southChannels.map(publicChannel);
      if (q) {
        const s = q.toLowerCase();
        items = items.filter(
          (c) =>
            c.name.toLowerCase().includes(s) ||
            c.protocolId.toLowerCase().includes(s),
        );
      }
      if (status) items = items.filter((c) => c.status === status);
      return items;
    },
    getSouth(id) {
      const ch = store.raw.southChannels.find((c) => c.id === id);
      if (!ch) {
        const err = new Error("通道不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      return publicChannel(ch);
    },
    createSouth(body) {
      const protocol = getProtocol(body.protocolId);
      if (!protocol || protocol.direction !== "south" || !protocol.enabled) {
        const err = new Error("请选择已启用的南向协议");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      if (!body.name?.trim()) {
        const err = new Error("通道名称必填");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      const now = new Date().toISOString();
      const ch = {
        id: store.id(),
        name: body.name.trim(),
        protocolId: body.protocolId,
        config: body.config || {},
        status: "stopped",
        lastError: null,
        lastSampleAt: null,
        enabled: false,
        createdAt: now,
        updatedAt: now,
      };
      store.raw.southChannels.push(ch);
      store.persist();
      store.pushEvent({
        level: "info",
        sourceType: "south",
        sourceId: ch.id,
        message: `创建南向通道「${ch.name}」`,
      });
      return publicChannel(ch);
    },
    updateSouth(id, body) {
      const ch = store.raw.southChannels.find((c) => c.id === id);
      if (!ch) {
        const err = new Error("通道不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      if (body.name != null) ch.name = String(body.name).trim();
      if (body.config != null) ch.config = mergeConfigPatch(ch.config, body.config);
      ch.updatedAt = new Date().toISOString();
      store.persist();
      return publicChannel(ch);
    },
    deleteSouth(id) {
      const idx = store.raw.southChannels.findIndex((c) => c.id === id);
      if (idx < 0) {
        const err = new Error("通道不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      const [ch] = store.raw.southChannels.splice(idx, 1);
      store.raw.points = store.raw.points.filter((p) => p.channelId !== id);
      store.persist();
      store.pushEvent({
        level: "warn",
        sourceType: "south",
        sourceId: id,
        message: `删除南向通道「${ch.name}」`,
      });
    },
    testConnection(id) {
      const ch = store.raw.southChannels.find((c) => c.id === id);
      if (!ch) {
        const err = new Error("通道不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      const useSim = ch.config.useSimulator !== false;
      if (useSim) {
        return {
          ok: true,
          message: `模拟连接成功（${ch.protocolId} → ${ch.config.host || ch.config.endpointUrl || "simulator"}）`,
          latencyMs: 12,
        };
      }
      // Without simulator, still soft-succeed for MVP host reachability stub
      if (!ch.config.host && !ch.config.endpointUrl) {
        return { ok: false, message: "缺少主机或 Endpoint 配置", latencyMs: 0 };
      }
      return {
        ok: true,
        message: "连接探测完成（桩实现，未真实握手工业协议）",
        latencyMs: 35,
      };
    },
    enableSouth(id) {
      const ch = store.raw.southChannels.find((c) => c.id === id);
      if (!ch) {
        const err = new Error("通道不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      ch.enabled = true;
      ch.status = "running";
      ch.lastError = null;
      ch.lastSampleAt = new Date().toISOString();
      ch.updatedAt = new Date().toISOString();
      store.persist();
      store.pushEvent({
        level: "info",
        sourceType: "south",
        sourceId: id,
        message: `启用南向通道「${ch.name}」`,
      });
      return publicChannel(ch);
    },
    disableSouth(id) {
      const ch = store.raw.southChannels.find((c) => c.id === id);
      if (!ch) {
        const err = new Error("通道不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      ch.enabled = false;
      ch.status = "stopped";
      ch.updatedAt = new Date().toISOString();
      store.persist();
      store.pushEvent({
        level: "warn",
        sourceType: "south",
        sourceId: id,
        message: `停用南向通道「${ch.name}」`,
      });
      return publicChannel(ch);
    },
    listPoints(channelId) {
      return store.raw.points.filter((p) => p.channelId === channelId);
    },
    createPoint(channelId, body) {
      const ch = store.raw.southChannels.find((c) => c.id === channelId);
      if (!ch) {
        const err = new Error("通道不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      if (!body.name?.trim()) {
        const err = new Error("点位名称必填");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      const model = store.raw.thingModels.find((m) => m.id === body.thingModelId);
      if (!model || model.status !== "enabled") {
        const err = new Error("须绑定已启用的物模型");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      const prop = model.properties.find((p) => p.id === body.propertyId);
      if (!prop) {
        const err = new Error("物模型属性不存在");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      const point = {
        id: store.id(),
        channelId,
        name: body.name.trim(),
        addressConfig: body.addressConfig || {},
        thingModelId: model.id,
        propertyId: prop.id,
        collect: body.collect || {},
        createdAt: new Date().toISOString(),
      };
      store.raw.points.push(point);
      store.persist();
      return point;
    },
    deletePoint(channelId, pointId) {
      const idx = store.raw.points.findIndex(
        (p) => p.id === pointId && p.channelId === channelId,
      );
      if (idx < 0) {
        const err = new Error("点位不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      store.raw.points.splice(idx, 1);
      store.persist();
    },
    listThingModels({ q, status } = {}) {
      let items = [...store.raw.thingModels];
      if (q) {
        const s = q.toLowerCase();
        items = items.filter(
          (m) =>
            m.name.toLowerCase().includes(s) || m.key.toLowerCase().includes(s),
        );
      }
      if (status) items = items.filter((m) => m.status === status);
      return items;
    },
    getThingModel(id) {
      const m = store.raw.thingModels.find((x) => x.id === id);
      if (!m) {
        const err = new Error("物模型不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      return m;
    },
    createThingModel(body) {
      if (!body.key?.trim() || !body.name?.trim()) {
        const err = new Error("标识与名称必填");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      if (store.raw.thingModels.some((m) => m.key === body.key.trim())) {
        const err = new Error("物模型标识已存在");
        err.status = 409;
        err.code = "CONFLICT";
        throw err;
      }
      const now = new Date().toISOString();
      const model = {
        id: store.id(),
        key: body.key.trim(),
        name: body.name.trim(),
        status: "draft",
        properties: [],
        createdAt: now,
        updatedAt: now,
      };
      store.raw.thingModels.push(model);
      store.persist();
      return model;
    },
    updateThingModel(id, body) {
      const m = this.getThingModel(id);
      if (body.name != null) m.name = String(body.name).trim();
      m.updatedAt = new Date().toISOString();
      store.persist();
      return m;
    },
    deleteThingModel(id) {
      const bound = store.raw.points.some((p) => p.thingModelId === id);
      if (bound) {
        const err = new Error("该物模型已绑定点位，无法删除");
        err.status = 409;
        err.code = "CONFLICT";
        throw err;
      }
      const idx = store.raw.thingModels.findIndex((m) => m.id === id);
      if (idx < 0) {
        const err = new Error("物模型不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      store.raw.thingModels.splice(idx, 1);
      store.persist();
    },
    addProperty(modelId, body) {
      const m = this.getThingModel(modelId);
      if (!body.key?.trim() || !body.name?.trim() || !body.dataType) {
        const err = new Error("属性标识、名称、类型必填");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      if (!DATA_TYPES.has(body.dataType)) {
        const err = new Error("非法数据类型");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      if (m.properties.some((p) => p.key === body.key.trim())) {
        const err = new Error("属性标识在模型内必须唯一");
        err.status = 409;
        err.code = "CONFLICT";
        throw err;
      }
      const prop = {
        id: store.id(),
        key: body.key.trim(),
        name: body.name.trim(),
        dataType: body.dataType,
        unit: body.unit || "",
        rw: body.rw || "r",
        reportable: body.reportable !== false,
      };
      m.properties.push(prop);
      m.updatedAt = new Date().toISOString();
      store.persist();
      return prop;
    },
    deleteProperty(modelId, propId) {
      const m = this.getThingModel(modelId);
      const bound = store.raw.points.some(
        (p) => p.thingModelId === modelId && p.propertyId === propId,
      );
      if (bound) {
        const err = new Error("属性已绑定点位，无法删除");
        err.status = 409;
        err.code = "CONFLICT";
        throw err;
      }
      m.properties = m.properties.filter((p) => p.id !== propId);
      m.updatedAt = new Date().toISOString();
      store.persist();
    },
    enableThingModel(id) {
      const m = this.getThingModel(id);
      if (m.properties.length < 1) {
        const err = new Error("至少需要一个属性才能启用");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      m.status = "enabled";
      m.updatedAt = new Date().toISOString();
      store.persist();
      return m;
    },
    listNorth({ q } = {}) {
      let items = store.raw.northTargets.map(publicNorth);
      if (q) {
        const s = q.toLowerCase();
        items = items.filter(
          (t) =>
            t.name.toLowerCase().includes(s) ||
            t.protocolId.toLowerCase().includes(s),
        );
      }
      return items;
    },
    getNorth(id) {
      const t = store.raw.northTargets.find((x) => x.id === id);
      if (!t) {
        const err = new Error("北向目标不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      return publicNorth(t);
    },
    createNorth(body) {
      const protocol = getProtocol(body.protocolId);
      if (!protocol || protocol.direction !== "north" || !protocol.enabled) {
        const err = new Error("请选择已启用的北向协议");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      if (!body.name?.trim()) {
        const err = new Error("名称必填");
        err.status = 400;
        err.code = "VALIDATION";
        throw err;
      }
      const now = new Date().toISOString();
      const t = {
        id: store.id(),
        name: body.name.trim(),
        protocolId: body.protocolId,
        config: body.config || {},
        mapping: body.mapping || { properties: [] },
        status: "stopped",
        enabled: false,
        successCount: 0,
        failCount: 0,
        lastResult: null,
        createdAt: now,
        updatedAt: now,
      };
      store.raw.northTargets.push(t);
      store.persist();
      return publicNorth(t);
    },
    updateNorth(id, body) {
      const t = store.raw.northTargets.find((x) => x.id === id);
      if (!t) {
        const err = new Error("北向目标不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      if (body.name != null) t.name = String(body.name).trim();
      if (body.config != null) t.config = mergeConfigPatch(t.config, body.config);
      if (body.mapping != null) t.mapping = body.mapping;
      t.updatedAt = new Date().toISOString();
      store.persist();
      return publicNorth(t);
    },
    deleteNorth(id) {
      const idx = store.raw.northTargets.findIndex((t) => t.id === id);
      if (idx < 0) {
        const err = new Error("北向目标不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      store.raw.northTargets.splice(idx, 1);
      delete store.raw.northResults[id];
      store.persist();
    },
    enableNorth(id) {
      const t = store.raw.northTargets.find((x) => x.id === id);
      if (!t) {
        const err = new Error("北向目标不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      t.enabled = true;
      t.status = "running";
      // Simulate a send result
      const payload = this.buildPayload(t);
      const result = {
        time: new Date().toISOString(),
        ok: true,
        message: t.config.useSimulator === false
          ? "已启用（桩发送）"
          : "模拟上报成功",
        payload,
      };
      t.lastResult = result;
      t.successCount += 1;
      t.updatedAt = new Date().toISOString();
      if (!store.raw.northResults[id]) store.raw.northResults[id] = [];
      store.raw.northResults[id].unshift(result);
      store.raw.northResults[id] = store.raw.northResults[id].slice(0, 20);
      store.persist();
      store.pushEvent({
        level: "info",
        sourceType: "north",
        sourceId: id,
        message: `启用北向目标「${t.name}」并完成一次模拟上报`,
      });
      return publicNorth(t);
    },
    disableNorth(id) {
      const t = store.raw.northTargets.find((x) => x.id === id);
      if (!t) {
        const err = new Error("北向目标不存在");
        err.status = 404;
        err.code = "NOT_FOUND";
        throw err;
      }
      t.enabled = false;
      t.status = "stopped";
      t.updatedAt = new Date().toISOString();
      store.persist();
      return publicNorth(t);
    },
    recentNorthResults(id) {
      this.getNorth(id);
      return store.raw.northResults[id] || [];
    },
    buildPayload(target) {
      const props = {};
      let modelKey = "unknown";
      for (const item of target.mapping?.properties || []) {
        const model = store.raw.thingModels.find((m) => m.id === item.thingModelId);
        if (!model) continue;
        modelKey = model.key;
        const prop = model.properties.find((p) => p.id === item.propertyId);
        if (!prop || !prop.reportable) continue;
        props[prop.key] =
          prop.dataType === "bool"
            ? true
            : prop.dataType === "int"
              ? 1
              : prop.dataType === "float"
                ? 1.23
                : prop.dataType === "enum"
                  ? "A"
                  : "sample";
      }
      return {
        modelKey,
        timestamp: new Date().toISOString(),
        properties: props,
      };
    },
    overview() {
      const channels = store.raw.southChannels;
      const running = channels.filter((c) => c.status === "running").length;
      const error = channels.filter((c) => c.status === "error").length;
      const stopped = channels.filter((c) => c.status === "stopped").length;
      return {
        south: {
          total: channels.length,
          running,
          error,
          stopped,
        },
        thingModels: {
          total: store.raw.thingModels.length,
          enabled: store.raw.thingModels.filter((m) => m.status === "enabled")
            .length,
        },
        north: {
          total: store.raw.northTargets.length,
          running: store.raw.northTargets.filter((t) => t.status === "running")
            .length,
        },
      };
    },
    recentErrors(limit = 20) {
      return store.raw.events
        .filter((e) => e.level === "error" || e.level === "warn")
        .slice(0, limit);
    },
    reportableMappingOptions() {
      return store.raw.thingModels
        .filter((m) => m.status === "enabled")
        .map((m) => ({
          id: m.id,
          key: m.key,
          name: m.name,
          properties: m.properties
            .filter((p) => p.reportable)
            .map((p) => ({ id: p.id, key: p.key, name: p.name, dataType: p.dataType })),
        }));
    },
  };
}
