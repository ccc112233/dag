import { FormEvent, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { StatusBadge } from "../components/StatusBadge";
import { useToast } from "../components/Toast";

type Property = {
  id: string;
  key: string;
  name: string;
  dataType: string;
  unit: string;
  rw: string;
  reportable: boolean;
};

type Model = {
  id: string;
  key: string;
  name: string;
  status: string;
  properties: Property[];
};

const DATA_TYPES = ["bool", "int", "float", "string", "enum"];

export function ThingModelEditPage() {
  const { id } = useParams();
  const isNew = !id || id === "new";
  const navigate = useNavigate();
  const toast = useToast();
  const [model, setModel] = useState<Model | null>(null);
  const [key, setKey] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [prop, setProp] = useState({
    key: "",
    name: "",
    dataType: "float",
    unit: "",
    rw: "r",
    reportable: true,
  });

  async function load() {
    if (isNew) return;
    try {
      const m = await api<Model>(`/api/thing-models/${id}`);
      setModel(m);
      setKey(m.key);
      setName(m.name);
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  useEffect(() => {
    load();
  }, [id]);

  async function createModel(e: FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const m = await api<Model>("/api/thing-models", {
        method: "POST",
        body: JSON.stringify({ key, name }),
      });
      toast("物模型已创建");
      navigate(`/thing-models/${m.id}`);
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  async function saveName(e: FormEvent) {
    e.preventDefault();
    try {
      const m = await api<Model>(`/api/thing-models/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ name }),
      });
      setModel(m);
      toast("已保存");
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  async function addProperty(e: FormEvent) {
    e.preventDefault();
    try {
      await api(`/api/thing-models/${id}/properties`, {
        method: "POST",
        body: JSON.stringify(prop),
      });
      setProp({ key: "", name: "", dataType: "float", unit: "", rw: "r", reportable: true });
      toast("属性已添加");
      load();
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  async function enable() {
    try {
      const m = await api<Model>(`/api/thing-models/${id}/enable`, { method: "POST" });
      setModel(m);
      toast("物模型已启用");
    } catch (err) {
      setError((err as ApiError).message);
    }
  }

  async function remove() {
    try {
      await api(`/api/thing-models/${id}`, { method: "DELETE" });
      toast("已删除");
      navigate("/thing-models");
    } catch (err) {
      setError((err as ApiError).message);
      setConfirmDelete(false);
    }
  }

  if (isNew) {
    return (
      <div>
        <div className="page-header">
          <div>
            <h1>新建物模型</h1>
            <p>先创建模型，再添加至少 1 个属性后启用</p>
          </div>
          <Link className="btn secondary" to="/thing-models">
            返回
          </Link>
        </div>
        {error ? (
          <div className="banner error" role="alert">
            {error}
          </div>
        ) : null}
        <form className="panel config-only" onSubmit={createModel}>
          <div className="field">
            <label htmlFor="tm-key">标识 *</label>
            <input id="tm-key" value={key} onChange={(e) => setKey(e.target.value)} required />
          </div>
          <div className="field">
            <label htmlFor="tm-name">名称 *</label>
            <input id="tm-name" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <button className="btn" type="submit">
            创建
          </button>
        </form>
      </div>
    );
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{model?.name || "物模型"}</h1>
          <p>
            <span className="mono">{model?.key}</span> ·{" "}
            {model ? <StatusBadge status={model.status === "enabled" ? "enabled" : "draft"} /> : null}
          </p>
        </div>
        <div className="row-actions">
          <div className="row-actions config-only">
            {model?.status !== "enabled" ? (
              <button type="button" className="btn" onClick={enable}>
                启用
              </button>
            ) : null}
            <button type="button" className="btn danger" onClick={() => setConfirmDelete(true)}>
              删除
            </button>
          </div>
          <Link className="btn secondary" to="/thing-models">
            返回
          </Link>
        </div>
      </div>
      {error ? (
        <div className="banner error" role="alert">
          {error}
        </div>
      ) : null}
      <form className="panel config-only" onSubmit={saveName}>
        <div className="field">
          <label htmlFor="tm-name-edit">名称</label>
          <input id="tm-name-edit" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <button className="btn" type="submit">
          保存
        </button>
      </form>

      <form className="panel config-only" onSubmit={addProperty}>
        <h2 style={{ marginTop: 0, fontSize: "1.05rem" }}>添加属性</h2>
        <div className="form-grid">
          <div className="field">
            <label htmlFor="prop-key">标识 *</label>
            <input
              id="prop-key"
              value={prop.key}
              onChange={(e) => setProp({ ...prop, key: e.target.value })}
              required
            />
          </div>
          <div className="field">
            <label htmlFor="prop-name">名称 *</label>
            <input
              id="prop-name"
              value={prop.name}
              onChange={(e) => setProp({ ...prop, name: e.target.value })}
              required
            />
          </div>
          <div className="field">
            <label htmlFor="prop-type">数据类型 *</label>
            <select
              id="prop-type"
              value={prop.dataType}
              onChange={(e) => setProp({ ...prop, dataType: e.target.value })}
            >
              {DATA_TYPES.map((d) => (
                <option key={d} value={d}>
                  {d}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label htmlFor="prop-unit">单位</label>
            <input
              id="prop-unit"
              value={prop.unit}
              onChange={(e) => setProp({ ...prop, unit: e.target.value })}
            />
          </div>
          <div className="field">
            <label htmlFor="prop-rw">读写</label>
            <select
              id="prop-rw"
              value={prop.rw}
              onChange={(e) => setProp({ ...prop, rw: e.target.value })}
            >
              <option value="r">只读</option>
              <option value="rw">读写</option>
            </select>
          </div>
          <div className="field">
            <label htmlFor="prop-report">
              <input
                id="prop-report"
                type="checkbox"
                checked={prop.reportable}
                onChange={(e) => setProp({ ...prop, reportable: e.target.checked })}
              />{" "}
              可上报
            </label>
          </div>
        </div>
        <button className="btn" type="submit">
          添加属性
        </button>
      </form>

      <div className="panel">
        {!model?.properties.length ? (
          <div className="empty">暂无属性</div>
        ) : (
          <table className="data">
            <thead>
              <tr>
                <th>标识</th>
                <th>名称</th>
                <th>类型</th>
                <th>单位</th>
                <th>读写</th>
                <th>上报</th>
              </tr>
            </thead>
            <tbody>
              {model.properties.map((p) => (
                <tr key={p.id}>
                  <td className="mono">{p.key}</td>
                  <td>{p.name}</td>
                  <td>{p.dataType}</td>
                  <td>{p.unit || "—"}</td>
                  <td>{p.rw}</td>
                  <td>{p.reportable ? "是" : "否"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <ConfirmDialog
        open={confirmDelete}
        title="删除物模型"
        message="若已绑定点位将被拒绝。确认删除？"
        confirmLabel="删除"
        danger
        onClose={() => setConfirmDelete(false)}
        onConfirm={remove}
      />
    </div>
  );
}
