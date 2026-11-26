import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { StatusBadge } from "../components/StatusBadge";
import { useToast } from "../components/Toast";

type Target = {
  id: string;
  name: string;
  protocolId: string;
  status: string;
  enabled: boolean;
  successCount: number;
  failCount: number;
  lastResult: {
    ok: boolean;
    message: string;
    time: string;
    payload?: unknown;
  } | null;
  mapping: { properties: Array<{ thingModelId: string; propertyId: string }> };
  secrets: Record<string, boolean>;
  config: Record<string, unknown>;
};

type Result = {
  time: string;
  ok: boolean;
  message: string;
  payload?: unknown;
};

export function NorthboundDetailPage() {
  const { id = "" } = useParams();
  const toast = useToast();
  const [target, setTarget] = useState<Target | null>(null);
  const [results, setResults] = useState<Result[]>([]);
  const [error, setError] = useState("");

  async function load() {
    try {
      const [t, r] = await Promise.all([
        api<Target>(`/api/northbound/targets/${id}`),
        api<{ items: Result[] }>(`/api/northbound/targets/${id}/recent-results`),
      ]);
      setTarget(t);
      setResults(r.items);
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  useEffect(() => {
    load();
  }, [id]);

  async function toggle() {
    if (!target) return;
    const path = target.enabled ? "disable" : "enable";
    try {
      const updated = await api<Target>(`/api/northbound/targets/${id}/${path}`, {
        method: "POST",
      });
      setTarget(updated);
      toast(updated.enabled ? "已启用" : "已停用");
      load();
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  if (!target) {
    return error ? <div className="banner error">{error}</div> : <div className="panel">加载中…</div>;
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{target.name}</h1>
          <p>
            <span className="mono">{target.protocolId}</span> ·{" "}
            <StatusBadge status={target.status} />
          </p>
        </div>
        <div className="row-actions config-only">
          <button type="button" className="btn" onClick={toggle}>
            {target.enabled ? "停用" : "启用"}
          </button>
          <Link className="btn secondary" to="/northbound">
            返回
          </Link>
        </div>
      </div>
      {error ? <div className="banner error">{error}</div> : null}
      <div className="panel">
        <p>
          成功 / 失败：{target.successCount} / {target.failCount}
        </p>
        <p>
          密钥字段：
          {Object.keys(target.secrets || {}).length
            ? Object.entries(target.secrets)
                .filter(([, v]) => v)
                .map(([k]) => `${k}=已保存`)
                .join(", ") || "无"
            : "无"}
        </p>
        <p>
          配置摘要：
          <span className="mono">{JSON.stringify(target.config)}</span>
        </p>
        <p>映射属性数：{target.mapping?.properties?.length || 0}</p>
        {target.lastResult ? (
          <div className="banner ok">
            最近上报：{new Date(target.lastResult.time).toLocaleString()} ·{" "}
            {target.lastResult.message}
            <pre className="mono" style={{ whiteSpace: "pre-wrap" }}>
              {JSON.stringify(target.lastResult.payload, null, 2)}
            </pre>
          </div>
        ) : (
          <div className="banner info">尚无上报结果，启用后将触发一次模拟发送</div>
        )}
      </div>
      <div className="panel">
        <h2 style={{ marginTop: 0, fontSize: "1.05rem" }}>最近发送记录</h2>
        {!results.length ? (
          <div className="empty">暂无记录</div>
        ) : (
          <table className="data">
            <thead>
              <tr>
                <th>时间</th>
                <th>结果</th>
                <th>消息</th>
              </tr>
            </thead>
            <tbody>
              {results.map((r, idx) => (
                <tr key={`${r.time}-${idx}`}>
                  <td className="mono">{new Date(r.time).toLocaleString()}</td>
                  <td>{r.ok ? "成功" : "失败"}</td>
                  <td>{r.message}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
