import { FormEvent, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { SchemaForm } from "../components/SchemaForm";
import { StatusBadge } from "../components/StatusBadge";
import { useToast } from "../components/Toast";

type Channel = {
  id: string;
  name: string;
  protocolId: string;
  config: Record<string, unknown>;
  secrets: Record<string, boolean>;
  status: string;
  enabled: boolean;
  lastError: string | null;
  lastSampleAt: string | null;
};

type Protocol = {
  id: string;
  configSchema: Record<string, unknown>;
  uiSchema?: Record<string, { "ui:widget"?: string }>;
  pointSchema?: Record<string, unknown>;
  capabilities?: string[];
};

type Point = {
  id: string;
  name: string;
  thingModelId: string;
  propertyId: string;
  addressConfig: Record<string, unknown>;
};

type ThingModel = {
  id: string;
  name: string;
  key: string;
  status: string;
  properties: Array<{ id: string; key: string; name: string }>;
};

export function SouthboundDetailPage() {
  const { id = "" } = useParams();
  const toast = useToast();
  const navigate = useNavigate();
  const [tab, setTab] = useState<"connection" | "points" | "status">("connection");
  const [channel, setChannel] = useState<Channel | null>(null);
  const [protocol, setProtocol] = useState<Protocol | null>(null);
  const [config, setConfig] = useState<Record<string, unknown>>({});
  const [points, setPoints] = useState<Point[]>([]);
  const [models, setModels] = useState<ThingModel[]>([]);
  const [error, setError] = useState("");
  const [testMsg, setTestMsg] = useState("");
  const [pending, setPending] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [pointName, setPointName] = useState("");
  const [modelId, setModelId] = useState("");
  const [propertyId, setPropertyId] = useState("");
  const [addressConfig, setAddressConfig] = useState<Record<string, unknown>>({});

  async function load() {
    setError("");
    try {
      const ch = await api<Channel>(`/api/southbound/channels/${id}`);
      const proto = await api<Protocol>(`/api/protocols/${ch.protocolId}`);
      const pts = await api<{ items: Point[] }>(`/api/southbound/channels/${id}/points`);
      const ms = await api<{ items: ThingModel[] }>("/api/thing-models?status=enabled");
      setChannel(ch);
      setProtocol(proto);
      setConfig(ch.config || {});
      setPoints(pts.items);
      setModels(ms.items);
      if (ms.items[0] && !modelId) {
        setModelId(ms.items[0].id);
        setPropertyId(ms.items[0].properties[0]?.id || "");
      }
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  useEffect(() => {
    load();
  }, [id]);

  const selectedModel = models.find((m) => m.id === modelId);

  async function saveConnection(e: FormEvent) {
    e.preventDefault();
    setPending(true);
    setError("");
    try {
      const updated = await api<Channel>(`/api/southbound/channels/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ config }),
      });
      setChannel(updated);
      setConfig(updated.config);
      toast("连接配置已保存");
    } catch (err) {
      setError((err as ApiError).message);
    } finally {
      setPending(false);
    }
  }

  async function testConnection() {
    setPending(true);
    setTestMsg("");
    try {
      const res = await api<{ ok: boolean; message: string }>(
        `/api/southbound/channels/${id}/test-connection`,
        { method: "POST" },
      );
      setTestMsg(res.message);
      toast(res.ok ? "测试成功" : "测试失败");
    } catch (err) {
      setError((err as ApiError).message);
    } finally {
      setPending(false);
    }
  }

  async function toggleEnable() {
    if (!channel) return;
    const path = channel.enabled ? "disable" : "enable";
    try {
      const updated = await api<Channel>(`/api/southbound/channels/${id}/${path}`, {
        method: "POST",
      });
      setChannel(updated);
      toast(updated.enabled ? "已启用" : "已停用");
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  async function addPoint(e: FormEvent) {
    e.preventDefault();
    try {
      await api(`/api/southbound/channels/${id}/points`, {
        method: "POST",
        body: JSON.stringify({
          name: pointName,
          thingModelId: modelId,
          propertyId,
          addressConfig,
        }),
      });
      setPointName("");
      toast("点位已添加");
      load();
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  async function removePoint(pointId: string) {
    await api(`/api/southbound/channels/${id}/points/${pointId}`, { method: "DELETE" });
    toast("点位已删除");
    load();
  }

  async function removeChannel() {
    await api(`/api/southbound/channels/${id}`, { method: "DELETE" });
    toast("通道已删除");
    navigate("/southbound");
  }

  if (!channel) {
    return (
      <div>
        {error ? <div className="banner error">{error}</div> : <div className="panel">加载中…</div>}
      </div>
    );
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{channel.name}</h1>
          <p>
            协议 <span className="mono">{channel.protocolId}</span> ·{" "}
            <StatusBadge status={channel.status} />
          </p>
        </div>
        <div className="row-actions config-only">
          <button type="button" className="btn" onClick={toggleEnable}>
            {channel.enabled ? "停用" : "启用"}
          </button>
          <button type="button" className="btn danger" onClick={() => setConfirmDelete(true)}>
            删除
          </button>
          <Link className="btn secondary" to="/southbound">
            返回
          </Link>
        </div>
      </div>

      {error ? (
        <div className="banner error" role="alert">
          {error}
        </div>
      ) : null}

      <div className="tabs" role="tablist">
        {(
          [
            ["connection", "连接"],
            ["points", "点位映射"],
            ["status", "状态与日志"],
          ] as const
        ).map(([key, label]) => (
          <button
            key={key}
            type="button"
            role="tab"
            aria-selected={tab === key}
            className={`tab${tab === key ? " active" : ""}`}
            onClick={() => setTab(key)}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === "connection" ? (
        <form className="panel config-only" onSubmit={saveConnection}>
          {protocol ? (
            <SchemaForm
              schema={protocol.configSchema as never}
              uiSchema={protocol.uiSchema}
              value={config}
              secrets={channel.secrets}
              onChange={setConfig}
            />
          ) : null}
          <div className="row-actions">
            <button className="btn" type="submit" disabled={pending}>
              保存配置
            </button>
            <button
              className="btn secondary"
              type="button"
              disabled={pending}
              onClick={testConnection}
            >
              测试连接
            </button>
          </div>
          {testMsg ? <div className="banner ok">{testMsg}</div> : null}
        </form>
      ) : null}

      {tab === "points" ? (
        <div className="config-only">
          <form className="panel" onSubmit={addPoint}>
            <h2 style={{ marginTop: 0, fontSize: "1.05rem" }}>添加点位</h2>
            <div className="form-grid">
              <div className="field">
                <label htmlFor="point-name">点位名称 *</label>
                <input
                  id="point-name"
                  value={pointName}
                  onChange={(e) => setPointName(e.target.value)}
                  required
                />
              </div>
              <div className="field">
                <label htmlFor="model">物模型 *</label>
                <select
                  id="model"
                  value={modelId}
                  onChange={(e) => {
                    setModelId(e.target.value);
                    const m = models.find((x) => x.id === e.target.value);
                    setPropertyId(m?.properties[0]?.id || "");
                  }}
                  required
                >
                  {models.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.name} ({m.key})
                    </option>
                  ))}
                </select>
              </div>
              <div className="field">
                <label htmlFor="prop">属性 *</label>
                <select
                  id="prop"
                  value={propertyId}
                  onChange={(e) => setPropertyId(e.target.value)}
                  required
                >
                  {(selectedModel?.properties || []).map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name} ({p.key})
                    </option>
                  ))}
                </select>
              </div>
            </div>
            {protocol?.pointSchema ? (
              <SchemaForm
                schema={protocol.pointSchema as never}
                value={addressConfig}
                onChange={setAddressConfig}
                idPrefix="point"
              />
            ) : null}
            <button className="btn" type="submit">
              添加点位
            </button>
          </form>
          <div className="panel">
            {!points.length ? (
              <div className="empty">暂无点位</div>
            ) : (
              <table className="data">
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>物模型</th>
                    <th>属性</th>
                    <th>地址</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {points.map((p) => (
                    <tr key={p.id}>
                      <td>{p.name}</td>
                      <td className="mono">{p.thingModelId.slice(0, 8)}</td>
                      <td className="mono">{p.propertyId.slice(0, 8)}</td>
                      <td className="mono">{JSON.stringify(p.addressConfig)}</td>
                      <td>
                        <button
                          type="button"
                          className="btn secondary"
                          onClick={() => removePoint(p.id)}
                        >
                          删除
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      ) : null}

      {tab === "status" ? (
        <div className="panel">
          <p>
            运行状态：<StatusBadge status={channel.status} />
          </p>
          <p>最近采样：{channel.lastSampleAt ? new Date(channel.lastSampleAt).toLocaleString() : "—"}</p>
          <p>最近错误：{channel.lastError || "无"}</p>
        </div>
      ) : null}

      <ConfirmDialog
        open={confirmDelete}
        title="删除通道"
        message="删除后点位一并清除，且不可恢复。"
        confirmLabel="删除"
        danger
        onClose={() => setConfirmDelete(false)}
        onConfirm={removeChannel}
      />
    </div>
  );
}
