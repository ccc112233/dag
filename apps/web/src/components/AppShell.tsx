import { useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { api, setToken } from "../api/client";
import { t } from "../lib/i18n";

const links = [
  { to: "/", label: t.nav.overview, end: true },
  { to: "/southbound", label: t.nav.south },
  { to: "/thing-models", label: t.nav.models },
  { to: "/northbound", label: t.nav.north },
  { to: "/protocols", label: t.nav.protocols },
  { to: "/system", label: t.nav.system },
];

export function AppShell() {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();

  async function logout() {
    try {
      await api("/api/auth/logout", { method: "POST" });
    } catch {
      /* ignore */
    }
    setToken(null);
    navigate("/login");
  }

  return (
    <div className="app-shell">
      <aside className={`nav ${open ? "open" : ""}`} aria-label="主导航">
        <div className="brand">
          <div className="brand-name">{t.appName}</div>
          <div className="brand-sub">{t.appSubtitle}</div>
        </div>
        <nav className="nav-links">
          {links.map((l) => (
            <NavLink
              key={l.to}
              to={l.to}
              end={l.end}
              className={({ isActive }) => `nav-link${isActive ? " active" : ""}`}
              onClick={() => setOpen(false)}
            >
              {l.label}
            </NavLink>
          ))}
        </nav>
        <div style={{ marginTop: "auto" }}>
          <button type="button" className="btn ghost" onClick={logout}>
            {t.actions.logout}
          </button>
        </div>
      </aside>
      <div className="main">
        <div className="topbar">
          <button
            type="button"
            className="btn secondary nav-toggle"
            aria-expanded={open}
            aria-controls="mobile-nav"
            onClick={() => setOpen((v) => !v)}
          >
            菜单
          </button>
          <strong>{t.appName}</strong>
        </div>
        <div className="banner info mobile-readonly-note" role="status">
          {t.mobileReadonly}
        </div>
        <Outlet />
      </div>
      {open ? (
        <div
          className="dialog-backdrop"
          style={{ background: "rgba(0,0,0,0.25)", zIndex: 20 }}
          onClick={() => setOpen(false)}
          aria-hidden="true"
        />
      ) : null}
    </div>
  );
}
