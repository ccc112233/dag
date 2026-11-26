import { describe, it, beforeEach } from "node:test";
import assert from "node:assert/strict";
import { createStore } from "./store/memory.js";
import { createDomain } from "./services/domain.js";

describe("domain API behaviors", () => {
  let store;
  let domain;

  beforeEach(() => {
    process.env.DAG_DATA_DIR = `/tmp/dag-test-${process.pid}-${Date.now()}`;
    store = createStore();
    store.reset();
    domain = createDomain(store);
  });

  it("creates thing model with unique property keys and rejects duplicates", () => {
    const m = domain.createThingModel({ key: "pump_v1", name: "水泵" });
    domain.addProperty(m.id, { key: "pressure", name: "压力", dataType: "float", unit: "MPa" });
    domain.addProperty(m.id, { key: "running", name: "运行", dataType: "bool" });
    domain.addProperty(m.id, { key: "mode", name: "模式", dataType: "enum" });
    assert.throws(
      () => domain.addProperty(m.id, { key: "pressure", name: "重复", dataType: "int" }),
      (e) => e.code === "CONFLICT",
    );
    domain.enableThingModel(m.id);
    assert.equal(domain.getThingModel(m.id).status, "enabled");
  });

  it("south channel test-connection + enable/disable", () => {
    const ch = domain.createSouth({
      name: "线体-1",
      protocolId: "modbus-tcp",
      config: { host: "10.0.0.8", port: 502, unitId: 1, useSimulator: true },
    });
    const test = domain.testConnection(ch.id);
    assert.equal(test.ok, true);
    const enabled = domain.enableSouth(ch.id);
    assert.equal(enabled.status, "running");
    const disabled = domain.disableSouth(ch.id);
    assert.equal(disabled.status, "stopped");
    assert.equal(domain.overview().south.stopped, 1);
  });

  it("blocks deleting thing model bound to points", () => {
    const m = domain.createThingModel({ key: "m1", name: "M1" });
    const prop = domain.addProperty(m.id, { key: "a", name: "A", dataType: "int" });
    domain.enableThingModel(m.id);
    const ch = domain.createSouth({
      name: "c1",
      protocolId: "opcua",
      config: { endpointUrl: "opc.tcp://x", useSimulator: true },
    });
    domain.createPoint(ch.id, {
      name: "p1",
      thingModelId: m.id,
      propertyId: prop.id,
      addressConfig: { nodeId: "ns=2;s=A" },
    });
    assert.throws(() => domain.deleteThingModel(m.id), (e) => e.code === "CONFLICT");
  });

  it("redacts secrets on read and keeps them on omit patch", () => {
    const ch = domain.createSouth({
      name: "sec",
      protocolId: "opcua",
      config: { endpointUrl: "opc.tcp://x", password: "secret", useSimulator: true },
    });
    assert.equal(ch.config.password, undefined);
    assert.equal(ch.secrets.password, true);
    domain.updateSouth(ch.id, { config: { endpointUrl: "opc.tcp://y" } });
    const raw = store.raw.southChannels.find((c) => c.id === ch.id);
    assert.equal(raw.config.password, "secret");
    assert.equal(raw.config.endpointUrl, "opc.tcp://y");
  });

  it("north http enable produces recent result", () => {
    const m = domain.createThingModel({ key: "tank", name: "Tank" });
    const p = domain.addProperty(m.id, {
      key: "level",
      name: "液位",
      dataType: "float",
      reportable: true,
    });
    domain.enableThingModel(m.id);
    const t = domain.createNorth({
      name: "上行-HTTP",
      protocolId: "http",
      config: { url: "https://example.com/hook", method: "POST", useSimulator: true },
      mapping: { properties: [{ thingModelId: m.id, propertyId: p.id }] },
    });
    const enabled = domain.enableNorth(t.id);
    assert.equal(enabled.status, "running");
    assert.ok(enabled.lastResult?.ok);
    assert.equal(domain.recentNorthResults(t.id).length, 1);
  });
});
