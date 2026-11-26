import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  listProtocols,
  getProtocol,
  redactConfig,
  mergeConfigPatch,
} from "./index.js";

describe("protocol-meta", () => {
  it("lists south and north built-ins", () => {
    const south = listProtocols("south");
    const north = listProtocols("north");
    assert.ok(south.some((p) => p.id === "modbus-tcp"));
    assert.ok(south.some((p) => p.id === "opcua"));
    assert.ok(north.some((p) => p.id === "http"));
    assert.ok(north.some((p) => p.id === "mqtt"));
    assert.equal(getProtocol("kafka")?.enabled, false);
  });

  it("redacts secrets and preserves hasSecret semantics via secrets map", () => {
    const { config, secrets } = redactConfig({
      host: "10.0.0.1",
      password: "s3cret",
      authToken: "tok",
    });
    assert.equal(config.host, "10.0.0.1");
    assert.equal(config.password, undefined);
    assert.equal(secrets.password, true);
    assert.equal(secrets.authToken, true);
  });

  it("mergeConfigPatch keeps secret when omitted", () => {
    const merged = mergeConfigPatch(
      { host: "a", password: "old" },
      { host: "b" },
    );
    assert.equal(merged.host, "b");
    assert.equal(merged.password, "old");
    const cleared = mergeConfigPatch({ password: "old" }, { password: "" });
    assert.equal(cleared.password, undefined);
  });
});
