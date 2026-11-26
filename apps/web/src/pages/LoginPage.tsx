import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, setToken } from "../api/client";
import { t } from "../lib/i18n";

export function LoginPage() {
  const navigate = useNavigate();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("admin");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setPending(true);
    setError("");
    try {
      const res = await api<{ token: string }>("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
      setToken(res.token);
      navigate("/");
    } catch (err) {
      setError((err as { message: string }).message);
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="login-page">
      <form className="login-card" onSubmit={onSubmit}>
        <div className="brand-name">{t.appName}</div>
        <p style={{ color: "var(--ink-muted)", marginTop: 0 }}>{t.appSubtitle}</p>
        {error ? (
          <div className="banner error" role="alert">
            {error}
          </div>
        ) : null}
        <div className="field">
          <label htmlFor="username">用户名</label>
          <input
            id="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
            autoComplete="username"
          />
        </div>
        <div className="field">
          <label htmlFor="password">密码</label>
          <input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="current-password"
          />
        </div>
        <button className="btn" type="submit" disabled={pending}>
          {pending ? "登录中…" : "登录"}
        </button>
        <p className="hint" style={{ marginTop: "1rem", color: "var(--ink-muted)" }}>
          MVP 本地账号：admin / admin
        </p>
      </form>
    </div>
  );
}
