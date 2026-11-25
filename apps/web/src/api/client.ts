export type ApiError = {
  code: string;
  message: string;
  fields?: Record<string, string>;
};

const TOKEN_KEY = "dag_token";

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string | null) {
  if (!token) localStorage.removeItem(TOKEN_KEY);
  else localStorage.setItem(TOKEN_KEY, token);
}

export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers || {});
  if (!headers.has("Content-Type") && options.body) {
    headers.set("Content-Type", "application/json");
  }
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);

  let res: Response;
  try {
    res = await fetch(path, {
      ...options,
      headers,
      credentials: "include",
    });
  } catch {
    const err: ApiError = { code: "UPSTREAM", message: "网络错误，请检查服务是否可用后重试" };
    throw err;
  }

  const text = await res.text();
  const data = text ? JSON.parse(text) : {};
  if (!res.ok) {
    const err: ApiError = {
      code: data.code || "UPSTREAM",
      message: data.message || `请求失败 (${res.status})`,
      fields: data.fields,
    };
    throw err;
  }
  return data as T;
}
