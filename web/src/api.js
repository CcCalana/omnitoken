(function () {
const DEFAULT_TIMEOUT_MS = 8000;

function resolveAdminBaseURL() {
  const params = new URLSearchParams(window.location.search);
  const explicitBaseURL = params.get("admin");
  if (explicitBaseURL) {
    return trimTrailingSlash(explicitBaseURL);
  }

  let savedBaseURL = "";
  try {
    savedBaseURL = localStorage.getItem("omnitokenAdminBaseURL") || "";
  } catch (_) {
    savedBaseURL = "";
  }
  if (savedBaseURL) {
    return trimTrailingSlash(savedBaseURL);
  }

  if (window.location.protocol === "http:" || window.location.protocol === "https:") {
    if (window.location.port === "8081") {
      return window.location.origin;
    }
    return `${window.location.protocol}//${window.location.hostname}:8081`;
  }

  return "http://localhost:8081";
}

class APIError extends Error {
  constructor(message, status, code) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
  }
}

function createAdminAPI(baseURL = resolveAdminBaseURL()) {
  if (isDemoMode()) {
    return createDemoAPI();
  }

  const base = trimTrailingSlash(baseURL);
  return {
    baseURL: base,
    getOverview: () => fetchJSON(`${base}/api/admin/overview`),
    getUsers: () => fetchJSON(`${base}/api/admin/users`),
    createUser: (payload) => fetchJSON(`${base}/api/admin/users`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
    getModels: () => fetchJSON(`${base}/api/admin/models`),
    getMe: () => fetchJSON(`${base}/api/admin/me`),
    getVirtualModels: () => fetchJSON(`${base}/api/admin/virtual-models`),
    createVirtualModel: (payload) => fetchJSON(`${base}/api/admin/virtual-models`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
    updateVirtualModel: (name, payload) => fetchJSON(`${base}/api/admin/virtual-models/${encodeURIComponent(name)}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),
    getCredentials: () => fetchJSON(`${base}/api/admin/credentials`),
    getAuditLogs: (filters = {}) => fetchJSON(`${base}/api/admin/audit-logs${toQueryString(filters)}`),
    getUserUsage: (userID, filters = {}) => fetchJSON(`${base}/api/admin/users/${encodeURIComponent(userID)}/usage${toQueryString(filters)}`),
    createCredential: (payload) => fetchJSON(`${base}/api/admin/credentials`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
    disableCredential: (id) => fetchJSON(`${base}/api/admin/credentials/${encodeURIComponent(id)}/disable`, {
      method: "PATCH",
    }),
    createVirtualKey: (payload) => fetchJSON(`${base}/api/admin/dev/virtual-keys`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
    updateUserQuota: (userID, budgetCents) => fetchJSON(`${base}/api/admin/users/${encodeURIComponent(userID)}/quota`, {
      method: "PATCH",
      body: JSON.stringify({ budget_cents: budgetCents }),
    }),
    logout: async () => {
      try {
        await fetchJSON(`${base}/api/admin/logout`, { method: "POST" });
      } catch (_) {}
      localStorage.removeItem("omnitokenAdminToken");
      window.location.reload();
    },
  };
}

function isDemoMode() {
  try {
    return Boolean(window.__OMNITOKEN_DEMO__) || new URLSearchParams(window.location.search).get("demo") === "1";
  } catch (_) {
    return false;
  }
}

function createDemoAPI() {
  return {
    baseURL: "demo://omnitoken",
    isDemo: true,
    getOverview: async () => demoOverview(),
    getUsers: async () => ({ users: [] }),
    createUser: async () => Promise.reject(new APIError("demo_mode_readonly", 400, "demo_mode_readonly")),
    getModels: async () => ({ models: [] }),
    getMe: async () => ({ role: "admin" }),
    getVirtualModels: async () => ({ virtual_models: [] }),
    createVirtualModel: async () => Promise.reject(new APIError("demo_mode_readonly", 400, "demo_mode_readonly")),
    updateVirtualModel: async () => Promise.reject(new APIError("demo_mode_readonly", 400, "demo_mode_readonly")),
    getCredentials: async () => ({ credentials: [] }),
    getAuditLogs: async () => ({ logs: [] }),
    getUserUsage: async () => ({ usage: [] }),
    createCredential: async () => Promise.reject(new APIError("demo_mode_readonly", 400, "demo_mode_readonly")),
    disableCredential: async () => Promise.reject(new APIError("demo_mode_readonly", 400, "demo_mode_readonly")),
    createVirtualKey: async () => Promise.reject(new APIError("demo_mode_readonly", 400, "demo_mode_readonly")),
    updateUserQuota: async () => Promise.reject(new APIError("demo_mode_readonly", 400, "demo_mode_readonly")),
    logout: async () => {
      localStorage.removeItem("omnitokenAdminToken");
      window.location.reload();
    },
  };
}

function demoOverview() {
  const trend = Array.from({ length: 30 }, (_, index) => {
    const day = index + 1;
    const baseline = 2_600_000 + Math.sin(index / 3) * 900_000 + index * 140_000;
    const spike = index === 23 ? 6_100_000 : 0;
    const tokens = Math.max(0, Math.round(baseline + spike));
    return {
      date: `2026-06-${String(day).padStart(2, "0")}`,
      tokens,
      cost_usd: Number(((tokens / 1_000_000) * (3.8 + (index % 5) * 0.12)).toFixed(2)),
    };
  });

  return {
    period: "2026-06",
    total_tokens: 184250000,
    estimated_cost_usd: 923.48,
    active_users: 128,
    quota_warning_users: 7,
    trend,
    model_usage: [
      { model: "gpt-4.1", tokens: 72000000, cost_usd: 446.2, share: 0.391 },
      { model: "gpt-4.1-mini", tokens: 43800000, cost_usd: 88.7, share: 0.238 },
      { model: "claude-3.7-sonnet", tokens: 32900000, cost_usd: 266.4, share: 0.178 },
      { model: "deepseek-v3", tokens: 21400000, cost_usd: 57.9, share: 0.116 },
      { model: "doubao-seed-1.6", tokens: 10150000, cost_usd: 37.3, share: 0.055 },
      { model: "embedding-3-large", tokens: 4000000, cost_usd: 26.98, share: 0.022 },
    ],
  };
}

function toQueryString(filters) {
  const params = new URLSearchParams();
  for (const key of ["actor_id", "resource_type", "resource_id", "since", "until", "limit", "top_n"]) {
    const value = filters[key];
    if (value !== undefined && value !== null && String(value).trim() !== "") {
      params.set(key, String(value).trim());
    }
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

async function fetchJSON(url, options = {}) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), DEFAULT_TIMEOUT_MS);
  const sessionToken =
    (typeof localStorage !== "undefined" && localStorage.getItem("omnitokenAdminToken")) ||
    new URLSearchParams(window.location.search).get("token") ||
    "";
  const headers = {
    Accept: "application/json",
    ...(sessionToken ? { Authorization: `Bearer ${sessionToken}` } : {}),
    ...(options.body ? { "Content-Type": "application/json" } : {}),
    ...(options.headers || {}),
  };
  try {
    const response = await fetch(url, {
      ...options,
      headers,
      cache: "no-store",
      signal: controller.signal,
    });

      if (!response.ok) {
      let code = `HTTP ${response.status}`;
      try {
        const body = await response.json();
        code = body?.error?.code || code;
      } catch (_) {}
      
      if (response.status === 401 && !url.endsWith("/login")) {
        // Clear token and reload to trigger login view
        localStorage.removeItem("omnitokenAdminToken");
        window.dispatchEvent(new Event('omnitoken:unauthorized'));
      }
      
      throw new APIError(code, response.status, code);
    }

    return await response.json();
  } catch (error) {
    if (error.name === "AbortError") {
      throw new APIError("请求超时", 0, "request_timeout");
    }
    throw error;
  } finally {
    clearTimeout(timeout);
  }
}

function trimTrailingSlash(value) {
  return String(value || "").replace(/\/+$/, "");
}

window.OmniTokenAPI = {
  APIError,
  createAdminAPI,
  resolveAdminBaseURL,
};
})();
