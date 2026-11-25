import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, ApiError } from "../api/client";

type Summary = {
  south: { total: number; running: number; error: number; stopped: number };
  thingModels: { total: number; enabled: number };
  north: { total: number; running: number };
};

type EventItem = {
  id: string;
  time: string;
  level: string;
  sourceType: string;
  sourceId: string;
  message: string;
};

export function OverviewPage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [events, setEvents] = useState<EventItem[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const [s, e] = await Promise.all([
        api<Summary>("/api/overview/summary"),
        api<{ items: EventItem[] }>("/api/overview/recent-errors"),
      ]);
      setSummary(s);
      setEvents(e.items);
    } catch (err) {
      setError((err as ApiError).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const timer = window.setInterval(load, 8000);
    return () => window.clearInterval(timer);
  }, []);

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>总览</h1>
          <p>通道健康、异常摘要与快捷入口</p>
        </div>
        <div className="row-actions config-only">
          <Link className="btn" to="/southbound/new">
            新建南向
          </Link>
          <Link className="btn secondary" to="/thing-models/new">
            新建物模型
          </Link>
          <Link className="btn secondary" to="/northbound/new">
            新建北向
          </Link>
        </div>
      </div>

      {error ? (
        <div className="banner error" role="alert">
          {error}{" "}
          <button type="button" className="btn secondary" onClick={load}>
            重试
          </button>
        </div>
      ) : null}

      {loading && !summary ? <div className="panel">加载中…</div> : null}

      {summary ? (
        <div className="stats">
          <div className="stat">
            南向总数
            <strong>{summary.south.total}</strong>
          </div>
          <div className="stat">
            运行中
            <strong>{summary.south.running}</strong>
          </div>
          <div className="stat">
            异常
            <strong>{summary.south.error}</strong>
          </div>
          <div className="stat">
            已启用物模型
            <strong>
              {summary.thingModels.enabled}/{summary.thingModels.total}
            </strong>
          </div>
          <div className="stat">
            北向运行中
            <strong>{summary.north.running}</strong>
          </div>
        </div>
      ) : null}

      <div className="panel" style={{ marginTop: "1rem" }}>
        <h2 style={{ marginTop: 0, fontSize: "1.05rem" }}>最近告警 / 操作</h2>
        {!events.length ? (
          <div className="empty">暂无告警或警告事件</div>
        ) : (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>级别</th>
                  <th>来源</th>
                  <th>消息</th>
                </tr>
              </thead>
              <tbody>
                {events.map((ev) => (
                  <tr key={ev.id}>
                    <td className="mono">{new Date(ev.time).toLocaleString()}</td>
                    <td>{ev.level}</td>
                    <td>
                      {ev.sourceType}/{ev.sourceId.slice(0, 8)}
                    </td>
                    <td>{ev.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
