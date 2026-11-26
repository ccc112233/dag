import { FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { SchemaForm, validateRequired } from "../components/SchemaForm";
import { useToast } from "../components/Toast";

type Protocol = {
  id: string;
  displayName: string;
  enabled: boolean;
  helpMarkdown?: string;
  configSchema: Record<string, unknown>;
  uiSchema?: Record<string, { "ui:widget"?: string }>;
};

export function SouthboundFormPage() {
  const navigate = useNavigate();
  const toast = useToast();
  const [params] = useSearchParams();
  const [protocols, setProtocols] = useState<Protocol[]>([]);
  const [protocolId, setProtocolId] = useState(params.get("protocolId") || "");
  const [name, setName] = useState("");
  const [config, setConfig] = useState<Record<string, unknown>>({});
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    api<{ items: Protocol[] }>("/api/protocols?direction=south").then((res) => {
      const enabled = res.items.filter((p) => p.enabled);
      setProtocols(enabled);
      if (!protocolId && enabled[0]) setProtocolId(enabled[0].id);
    });
  }, []);

  useEffect(() => {
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (!dirty) return;
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [dirty]);

  const selected = useMemo(
    () => protocols.find((p) => p.id === protocolId),
    [protocols, protocolId],
  );

  useEffect(() => {
    if (!selected) return;
    const defaults: Record<string, unknown> = {};
    const props = (selected.configSchema as { properties?: Record<string, { default?: unknown }> })
      .properties || {};
    for (const [k, v] of Object.entries(props)) {
      if (v.default !== undefined) defaults[k] = v.default;
    }
    setConfig(defaults);
  }, [selected?.id]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!selected) return;
    const missing = validateRequired(selected.configSchema as never, config);
    if (missing.length) {
      setError(`请填写必填项：${missing.join(", ")}`);
      return;
    }
    setPending(true);
    setError("");
    try {
      const created = await api<{ id: string }>("/api/southbound/channels", {
        method: "POST",
        body: JSON.stringify({ name, protocolId, config }),
      });
      setDirty(false);
      toast("南向通道已创建");
      navigate(`/southbound/${created.id}`);
    } catch (err) {
      setError((err as ApiError).message);
    } finally {
      setPending(false);
    }
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>新建南向通道</h1>
          <p>选择协议后按元数据填写连接参数</p>
        </div>
        <Link className="btn secondary" to="/southbound">
          返回列表
        </Link>
      </div>
      {error ? (
        <div className="banner error" role="alert">
          {error}
        </div>
      ) : null}
      <form
        className="panel config-only"
        onSubmit={onSubmit}
        onChange={() => setDirty(true)}
      >
        <div className="field">
          <label htmlFor="channel-name">通道名称 *</label>
          <input
            id="channel-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </div>
        <div className="field">
          <label htmlFor="protocol">协议 *</label>
          <select
            id="protocol"
            value={protocolId}
            onChange={(e) => setProtocolId(e.target.value)}
            required
          >
            {protocols.map((p) => (
              <option key={p.id} value={p.id}>
                {p.displayName}
              </option>
            ))}
          </select>
        </div>
        {selected?.helpMarkdown ? (
          <div className="banner info">{selected.helpMarkdown}</div>
        ) : null}
        {selected ? (
          <SchemaForm
            schema={selected.configSchema as never}
            uiSchema={selected.uiSchema}
            value={config}
            onChange={(next) => {
              setConfig(next);
              setDirty(true);
            }}
          />
        ) : null}
        <button className="btn" type="submit" disabled={pending}>
          {pending ? "保存中…" : "保存"}
        </button>
      </form>
    </div>
  );
}
