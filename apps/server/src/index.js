import http from "node:http";
import { URL } from "node:url";
import { createStore } from "./store/memory.js";
import { createDomain } from "./services/domain.js";
import {
  sendJson,
  sendError,
  readJson,
  matchPath,
  parseCookies,
} from "./http.js";

const PORT = Number(process.env.PORT || 8787);
const store = createStore();
const domain = createDomain(store);

const PUBLIC = new Set(["POST /api/auth/login", "OPTIONS "]);

async function handler(req, res) {
  const url = new URL(req.url || "/", `http://${req.headers.host || "localhost"}`);
  const method = req.method || "GET";
  const pathname = url.pathname;
  req.cookies = parseCookies(req.headers.cookie || "");

  if (method === "OPTIONS") {
    return sendJson(res, 204, {});
  }

  try {
    const routeKey = `${method} ${pathname}`;
    if (!routeKey.startsWith("POST /api/auth/login") && method !== "OPTIONS") {
      // allow unauthenticated health
      if (pathname !== "/api/health") domain.requireAuth(req);
    }

    // Auth
    if (method === "POST" && pathname === "/api/auth/login") {
      const body = await readJson(req);
      const result = domain.login(body.username, body.password);
      res.setHeader(
        "Set-Cookie",
        `dag_session=${encodeURIComponent(result.token)}; Path=/; HttpOnly; SameSite=Lax`,
      );
      return sendJson(res, 200, result);
    }
    if (method === "POST" && pathname === "/api/auth/logout") {
      domain.logout();
      res.setHeader("Set-Cookie", "dag_session=; Path=/; Max-Age=0");
      return sendJson(res, 200, { ok: true });
    }
    if (method === "GET" && pathname === "/api/health") {
      return sendJson(res, 200, { ok: true });
    }

    // Protocols
    if (method === "GET" && pathname === "/api/protocols") {
      return sendJson(res, 200, {
        items: domain.listProtocols(url.searchParams.get("direction") || undefined),
      });
    }
    {
      const p = matchPath("/api/protocols/:id", pathname);
      if (method === "GET" && p) return sendJson(res, 200, domain.getProtocol(p.id));
    }

    // Overview
    if (method === "GET" && pathname === "/api/overview/summary") {
      return sendJson(res, 200, domain.overview());
    }
    if (method === "GET" && pathname === "/api/overview/recent-errors") {
      return sendJson(res, 200, { items: domain.recentErrors() });
    }
    if (method === "GET" && pathname === "/api/northbound/mapping-options") {
      return sendJson(res, 200, { items: domain.reportableMappingOptions() });
    }

    // Southbound
    if (method === "GET" && pathname === "/api/southbound/channels") {
      return sendJson(res, 200, {
        items: domain.listSouth({
          q: url.searchParams.get("q") || undefined,
          status: url.searchParams.get("status") || undefined,
        }),
      });
    }
    if (method === "POST" && pathname === "/api/southbound/channels") {
      const body = await readJson(req);
      return sendJson(res, 201, domain.createSouth(body));
    }
    {
      const p = matchPath("/api/southbound/channels/:id", pathname);
      if (p) {
        if (method === "GET") return sendJson(res, 200, domain.getSouth(p.id));
        if (method === "PATCH") {
          const body = await readJson(req);
          return sendJson(res, 200, domain.updateSouth(p.id, body));
        }
        if (method === "DELETE") {
          domain.deleteSouth(p.id);
          return sendJson(res, 200, { ok: true });
        }
      }
    }
    {
      const p = matchPath("/api/southbound/channels/:id/test-connection", pathname);
      if (method === "POST" && p) {
        return sendJson(res, 200, domain.testConnection(p.id));
      }
    }
    {
      const p = matchPath("/api/southbound/channels/:id/enable", pathname);
      if (method === "POST" && p) return sendJson(res, 200, domain.enableSouth(p.id));
    }
    {
      const p = matchPath("/api/southbound/channels/:id/disable", pathname);
      if (method === "POST" && p) return sendJson(res, 200, domain.disableSouth(p.id));
    }
    {
      const p = matchPath("/api/southbound/channels/:id/points", pathname);
      if (p) {
        if (method === "GET") return sendJson(res, 200, { items: domain.listPoints(p.id) });
        if (method === "POST") {
          const body = await readJson(req);
          return sendJson(res, 201, domain.createPoint(p.id, body));
        }
      }
    }
    {
      const p = matchPath("/api/southbound/channels/:channelId/points/:pointId", pathname);
      if (method === "DELETE" && p) {
        domain.deletePoint(p.channelId, p.pointId);
        return sendJson(res, 200, { ok: true });
      }
    }

    // Thing models
    if (method === "GET" && pathname === "/api/thing-models") {
      return sendJson(res, 200, {
        items: domain.listThingModels({
          q: url.searchParams.get("q") || undefined,
          status: url.searchParams.get("status") || undefined,
        }),
      });
    }
    if (method === "POST" && pathname === "/api/thing-models") {
      const body = await readJson(req);
      return sendJson(res, 201, domain.createThingModel(body));
    }
    {
      const p = matchPath("/api/thing-models/:id", pathname);
      if (p) {
        if (method === "GET") return sendJson(res, 200, domain.getThingModel(p.id));
        if (method === "PATCH") {
          const body = await readJson(req);
          return sendJson(res, 200, domain.updateThingModel(p.id, body));
        }
        if (method === "DELETE") {
          domain.deleteThingModel(p.id);
          return sendJson(res, 200, { ok: true });
        }
      }
    }
    {
      const p = matchPath("/api/thing-models/:id/enable", pathname);
      if (method === "POST" && p) return sendJson(res, 200, domain.enableThingModel(p.id));
    }
    {
      const p = matchPath("/api/thing-models/:id/properties", pathname);
      if (method === "POST" && p) {
        const body = await readJson(req);
        return sendJson(res, 201, domain.addProperty(p.id, body));
      }
    }
    {
      const p = matchPath("/api/thing-models/:id/properties/:propId", pathname);
      if (method === "DELETE" && p) {
        domain.deleteProperty(p.id, p.propId);
        return sendJson(res, 200, { ok: true });
      }
    }

    // Northbound
    if (method === "GET" && pathname === "/api/northbound/targets") {
      return sendJson(res, 200, {
        items: domain.listNorth({ q: url.searchParams.get("q") || undefined }),
      });
    }
    if (method === "POST" && pathname === "/api/northbound/targets") {
      const body = await readJson(req);
      return sendJson(res, 201, domain.createNorth(body));
    }
    {
      const p = matchPath("/api/northbound/targets/:id", pathname);
      if (p) {
        if (method === "GET") return sendJson(res, 200, domain.getNorth(p.id));
        if (method === "PATCH") {
          const body = await readJson(req);
          return sendJson(res, 200, domain.updateNorth(p.id, body));
        }
        if (method === "DELETE") {
          domain.deleteNorth(p.id);
          return sendJson(res, 200, { ok: true });
        }
      }
    }
    {
      const p = matchPath("/api/northbound/targets/:id/enable", pathname);
      if (method === "POST" && p) return sendJson(res, 200, domain.enableNorth(p.id));
    }
    {
      const p = matchPath("/api/northbound/targets/:id/disable", pathname);
      if (method === "POST" && p) return sendJson(res, 200, domain.disableNorth(p.id));
    }
    {
      const p = matchPath("/api/northbound/targets/:id/recent-results", pathname);
      if (method === "GET" && p) {
        return sendJson(res, 200, { items: domain.recentNorthResults(p.id) });
      }
    }

    return sendError(res, 404, "NOT_FOUND", `No route ${method} ${pathname}`);
  } catch (err) {
    const status = err.status || 500;
    const code = err.code || "UPSTREAM";
    return sendError(res, status, code, err.message || "Internal error", err.fields);
  }
}

export function createServer() {
  return http.createServer(handler);
}

const server = createServer();
server.listen(PORT, () => {
  console.log(`dag server listening on http://127.0.0.1:${PORT}`);
});

