import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { StatusBadge } from "../components/StatusBadge";
import { useToast } from "../components/Toast";
import { t } from "../lib/i18n";

type Target = {
  id: string;
  name: string;
  protocolId: string;
  status: string;
  enabled: boolean;
  lastResult: { ok: boolean; message: string; time: string } | null;
};

export function NorthboundListPage() {
  const toast = useToast();
  const [items, setItems] = useState<Target[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const res = await api<{ items: Target[] }>("/api/northbound/targets");
      setItems(res.items);
    } catch (err) {
      setError((err as ApiError).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function toggle(tItem: Target) {
    const path = tItem.enabled ? "disable" : "enable";
    try {
      await api(`/api/northbound/targets/${tItem.id}/${path}`, { method: "POST" });
      toast(tItem.enabled ? "已停用" : "已启用");
      load();
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>北向目标</h1>
          <p>HTTP / MQTT 上报配置与最近发送结果</p>
        </div>
        <Link className="btn config-only" to="/northbound/new">
          {t.actions.create}
        </Link>
      </div>
      {error ? (
        <div className="banner error" role="alert">
          {error}
        </div>
      ) : null}
      <div className="panel">
        {loading ? <div>加载中…</div> : null}
        {!loading && !items.length ? (
          <div className="empty">
            <p>{t.empty.north}</p>
            <Link className="btn config-only" to="/northbound/new">
              新建目标
            </Link>
          </div>
        ) : null}
        {items.length ? (
          <table className="data">
            <thead>
              <tr>
                <th>名称</th>
                <th>协议</th>
                <th>状态</th>
                <th>最近结果</th>
                <th className="config-only">操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td>
                    <Link to={`/northbound/${item.id}`}>{item.name}</Link>
                  </td>
                  <td className="mono">{item.protocolId}</td>
                  <td>
                    <StatusBadge status={item.status} />
                  </td>
                  <td>
                    {item.lastResult
                      ? `${item.lastResult.ok ? "成功" : "失败"} · ${item.lastResult.message}`
                      : "—"}
                  </td>
                  <td className="config-only">
                    <button type="button" className="btn secondary" onClick={() => toggle(item)}>
                      {item.enabled ? "停用" : "启用"}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : null}
      </div>
    </div>
  );
}
