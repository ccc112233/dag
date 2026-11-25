import { FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { SchemaForm, validateRequired } from "../components/SchemaForm";
import { useToast } from "../components/Toast";

type Protocol = {
  id: string;
  displayName: string;
  enabled: boolean;
  configSchema: Record<string, unknown>;
  uiSchema?: Record<string, { "ui:widget"?: string }>;
  helpMarkdown?: string;
};

type MappingOption = {
  id: string;
  key: string;
  name: string;
  properties: Array<{ id: string; key: string; name: string; dataType: string }>;
};

export function NorthboundFormPage() {
  const navigate = useNavigate();
  const toast = useToast();
  const [params] = useSearchParams();
  const [protocols, setProtocols] = useState<Protocol[]>([]);
  const [options, setOptions] = useState<MappingOption[]>([]);
  const [protocolId, setProtocolId] = useState(params.get("protocolId") || "http");
  const [name, setName] = useState("");
  const [config, setConfig] = useState<Record<string, unknown>>({});
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  useEffect(() => {
    Promise.all([
      api<{ items: Protocol[] }>("/api/protocols?direction=north"),
      api<{ items: MappingOption[] }>("/api/northbound/mapping-options"),
    ]).then(([p, m]) => {
      const enabled = p.items.filter((x) => x.enabled);
      setProtocols(enabled);
      setOptions(m.items);
      if (!enabled.find((x) => x.id === protocolId) && enabled[0]) {
        setProtocolId(enabled[0].id);
      }
    });
  }, []);

  const protocol = useMemo(
    () => protocols.find((p) => p.id === protocolId),
    [protocols, protocolId],
  );

  useEffect(() => {
    if (!protocol) return;
    const defaults: Record<string, unknown> = {};
    const props =
      (protocol.configSchema as { properties?: Record<string, { default?: unknown }> }).properties ||
      {};
    for (const [k, v] of Object.entries(props)) {
      if (v.default !== undefined) defaults[k] = v.default;
    }
    setConfig(defaults);
  }, [protocol?.id]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!protocol) return;
    const missing = validateRequired(protocol.configSchema as never, config);
    if (missing.length) {
      setError(`请填写必填项：${missing.join(", ")}`);
      return;
    }
    const properties = Object.entries(selected)
      .filter(([, v]) => v)
      .map(([composite]) => {
        const [thingModelId, propertyId] = composite.split("::");
        return { thingModelId, propertyId };
      });
    setPending(true);
    setError("");
    try {
      const created = await api<{ id: string }>("/api/northbound/targets", {
        method: "POST",
        body: JSON.stringify({
          name,
          protocolId,
          config,
          mapping: { properties },
        }),
      });
      toast("北向目标已创建");
      navigate(`/northbound/${created.id}`);
    } catch (err) {
      setError((err as ApiError).message);
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="config-only">
      <div className="page-header">
        <div>
          <h1>新建北向目标</h1>
          <p>配置 HTTP / MQTT 并勾选可上报属性</p>
        </div>
        <Link className="btn secondary" to="/northbound">
          返回
        </Link>
      </div>
      {error ? (
        <div className="banner error" role="alert">
          {error}
        </div>
      ) : null}
      <form className="panel" onSubmit={onSubmit}>
        <div className="field">
          <label htmlFor="north-name">名称 *</label>
          <input
            id="north-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </div>
        <div className="field">
          <label htmlFor="north-protocol">协议 *</label>
          <select
            id="north-protocol"
            value={protocolId}
            onChange={(e) => setProtocolId(e.target.value)}
          >
            {protocols.map((p) => (
              <option key={p.id} value={p.id}>
                {p.displayName}
              </option>
            ))}
          </select>
        </div>
        {protocol?.helpMarkdown ? <div className="banner info">{protocol.helpMarkdown}</div> : null}
        {protocol ? (
          <SchemaForm
            schema={protocol.configSchema as never}
            uiSchema={protocol.uiSchema}
            value={config}
            onChange={setConfig}
          />
        ) : null}

        <h2 style={{ fontSize: "1.05rem" }}>上报范围</h2>
        {!options.length ? (
          <div className="banner info">暂无已启用且可上报的物模型属性，请先创建并启用物模型。</div>
        ) : (
          options.map((m) => (
            <fieldset key={m.id} style={{ border: "1px solid var(--line)", borderRadius: 8, marginBottom: 12 }}>
              <legend>
                {m.name} ({m.key})
              </legend>
              {m.properties.map((p) => {
                const composite = `${m.id}::${p.id}`;
                return (
                  <label key={p.id} style={{ display: "block", margin: "0.35rem 0" }}>
                    <input
                      type="checkbox"
                      checked={Boolean(selected[composite])}
                      onChange={(e) =>
                        setSelected({ ...selected, [composite]: e.target.checked })
                      }
                    />{" "}
                    {p.name} <span className="mono">({p.key}/{p.dataType})</span>
                  </label>
                );
              })}
            </fieldset>
          ))
        )}

        <button className="btn" type="submit" disabled={pending}>
          {pending ? "保存中…" : "保存"}
        </button>
      </form>
    </div>
  );
}
