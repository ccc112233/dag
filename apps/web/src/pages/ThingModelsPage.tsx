import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { StatusBadge } from "../components/StatusBadge";
import { t } from "../lib/i18n";

type Model = {
  id: string;
  key: string;
  name: string;
  status: string;
  properties: unknown[];
};

export function ThingModelsPage() {
  const [items, setItems] = useState<Model[]>([]);
  const [q, setQ] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      const res = await api<{ items: Model[] }>(`/api/thing-models?${params}`);
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

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>物模型</h1>
          <p>属性级语义模型，用于点位绑定与北向上报映射</p>
        </div>
        <Link className="btn config-only" to="/thing-models/new">
          {t.actions.create}
        </Link>
      </div>
      <div className="toolbar">
        <div className="field" style={{ margin: 0 }}>
          <label htmlFor="tm-q">搜索</label>
          <input id="tm-q" value={q} onChange={(e) => setQ(e.target.value)} />
        </div>
        <button type="button" className="btn secondary" onClick={load} style={{ alignSelf: "end" }}>
          筛选
        </button>
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
            <p>{t.empty.models}</p>
            <Link className="btn config-only" to="/thing-models/new">
              新建物模型
            </Link>
          </div>
        ) : null}
        {items.length ? (
          <table className="data">
            <thead>
              <tr>
                <th>名称</th>
                <th>标识</th>
                <th>状态</th>
                <th>属性数</th>
              </tr>
            </thead>
            <tbody>
              {items.map((m) => (
                <tr key={m.id}>
                  <td>
                    <Link to={`/thing-models/${m.id}`}>{m.name}</Link>
                  </td>
                  <td className="mono">{m.key}</td>
                  <td>
                    <StatusBadge status={m.status === "enabled" ? "enabled" : "draft"} />
                  </td>
                  <td>{m.properties.length}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : null}
      </div>
    </div>
  );
}
