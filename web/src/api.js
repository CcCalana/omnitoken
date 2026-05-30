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
  const base = trimTrailingSlash(baseURL);
  return {
    baseURL: base,
    getOverview: () => fetchJSON(`${base}/api/admin/overview`),
    getUsers: () => fetchJSON(`${base}/api/admin/users`),
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
