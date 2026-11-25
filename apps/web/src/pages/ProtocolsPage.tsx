import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, ApiError } from "../api/client";

type Protocol = {
  id: string;
  direction: string;
  displayName: string;
  version: string;
  enabled: boolean;
  capabilities: string[];
};

export function ProtocolsPage() {
  const [items, setItems] = useState<Protocol[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    (async () => {
      try {
        const res = await api<{ items: Protocol[] }>("/api/protocols");
        setItems(res.items);
      } catch (err) {
        setError((err as ApiError).message);
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>协议目录</h1>
          <p>已注册南/北向协议；扩展协议安装后自动出现在此</p>
        </div>
      </div>
      {error ? (
        <div className="banner error" role="alert">
          {error}
        </div>
      ) : null}
      <div className="panel">
        {loading ? <div>加载中…</div> : null}
        {!loading && !items.length ? <div className="empty">暂无协议</div> : null}
        {items.length ? (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>方向</th>
                  <th>版本</th>
                  <th>状态</th>
                  <th>能力</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {items.map((p) => (
                  <tr key={p.id}>
                    <td>
                      <strong>{p.displayName}</strong>
                      <div className="mono">{p.id}</div>
                    </td>
                    <td>{p.direction === "south" ? "南向" : "北向"}</td>
                    <td className="mono">{p.version}</td>
                    <td>{p.enabled ? "已启用" : "即将支持"}</td>
                    <td>{p.capabilities.join(", ") || "—"}</td>
                    <td className="config-only">
                      {p.enabled ? (
                        <Link
                          className="btn secondary"
                          to={
                            p.direction === "south"
                              ? `/southbound/new?protocolId=${p.id}`
                              : `/northbound/new?protocolId=${p.id}`
                          }
                        >
                          用此协议新建
                        </Link>
                      ) : null}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </div>
    </div>
  );
}
