import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { StatusBadge } from "../components/StatusBadge";
import { useToast } from "../components/Toast";
import { t } from "../lib/i18n";

type Channel = {
  id: string;
  name: string;
  protocolId: string;
  status: string;
  enabled: boolean;
  lastError: string | null;
};

export function SouthboundListPage() {
  const toast = useToast();
  const [items, setItems] = useState<Channel[]>([]);
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [confirmId, setConfirmId] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      if (status) params.set("status", status);
      const res = await api<{ items: Channel[] }>(
        `/api/southbound/channels?${params.toString()}`,
      );
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

  async function enable(id: string) {
    try {
      await api(`/api/southbound/channels/${id}/enable`, { method: "POST" });
      toast("通道已启用");
      load();
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  async function disable(id: string) {
    try {
      await api(`/api/southbound/channels/${id}/disable`, { method: "POST" });
      toast("通道已停用");
      setConfirmId(null);
      load();
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>南向通道</h1>
          <p>工业协议接入、启停与状态</p>
        </div>
        <Link className="btn config-only" to="/southbound/new">
          {t.actions.create}
        </Link>
      </div>

      <div className="toolbar">
        <div className="field" style={{ margin: 0 }}>
          <label htmlFor="south-q">搜索</label>
          <input
            id="south-q"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="名称或协议"
          />
        </div>
        <div className="field" style={{ margin: 0 }}>
          <label htmlFor="south-status">状态</label>
          <select
            id="south-status"
            value={status}
            onChange={(e) => setStatus(e.target.value)}
          >
            <option value="">全部</option>
            <option value="running">运行中</option>
            <option value="stopped">已停用</option>
            <option value="error">异常</option>
          </select>
        </div>
        <button type="button" className="btn secondary" onClick={load} style={{ alignSelf: "end" }}>
          筛选
        </button>
      </div>

      {error ? (
        <div className="banner error" role="alert">
          {error}{" "}
          <button type="button" className="btn secondary" onClick={load}>
            重试
          </button>
        </div>
      ) : null}

      <div className="panel">
        {loading ? <div>加载中…</div> : null}
        {!loading && !items.length ? (
          <div className="empty">
            <p>{t.empty.south}</p>
            <Link className="btn config-only" to="/southbound/new">
              新建通道
            </Link>
          </div>
        ) : null}
        {items.length ? (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>协议</th>
                  <th>状态</th>
                  <th>最近错误</th>
                  <th className="config-only">操作</th>
                </tr>
              </thead>
              <tbody>
                {items.map((c) => (
                  <tr key={c.id}>
                    <td>
                      <Link to={`/southbound/${c.id}`}>{c.name}</Link>
                    </td>
                    <td className="mono">{c.protocolId}</td>
                    <td>
                      <StatusBadge status={c.status} />
                    </td>
                    <td>{c.lastError || "—"}</td>
                    <td className="config-only">
                      <div className="row-actions">
                        {c.enabled ? (
                          <button
                            type="button"
                            className="btn secondary"
                            onClick={() => setConfirmId(c.id)}
                          >
                            停用
                          </button>
                        ) : (
                          <button type="button" className="btn" onClick={() => enable(c.id)}>
                            启用
                          </button>
                        )}
                        <Link className="btn secondary" to={`/southbound/${c.id}`}>
                          详情
                        </Link>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </div>

      <ConfirmDialog
        open={Boolean(confirmId)}
        title="停用通道"
        message="停用后将停止采集。确认继续？"
        confirmLabel="停用"
        danger
        onClose={() => setConfirmId(null)}
        onConfirm={() => confirmId && disable(confirmId)}
      />
    </div>
  );
}
