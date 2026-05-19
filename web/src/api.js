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
    getAuditLogs: (filters = {}) => fetchJSON(`${base}/api/admin/audit-logs${toQueryString(filters)}`),
    updateUserQuota: (userID, budgetCents) => fetchJSON(`${base}/api/admin/users/${encodeURIComponent(userID)}/quota`, {
      method: "PATCH",
      body: JSON.stringify({ budget_cents: budgetCents }),
    }),
  };
}

function toQueryString(filters) {
  const params = new URLSearchParams();
  for (const key of ["actor_id", "resource_type", "resource_id", "since", "until", "limit"]) {
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
  const headers = {
    Accept: "application/json",
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
