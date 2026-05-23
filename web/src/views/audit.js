(function () {
const {
  escapeHTML,
  formatNumber,
  formatTokens,
  setAlert,
  setEmptyOverlay,
} = window.OmniTokenUtils;

function createAuditView(api) {
  let logs = [];
  let loadedLogs = false;
  let loadedUsers = false;
  let loadedUsageUserID = "";
  let activeView = "logs";
  let expandedRow = "";
  let hourlyChart = null;

  const nodes = {
    alert: document.getElementById("audit-alert"),
    logPanel: document.getElementById("audit-logs-panel"),
    usagePanel: document.getElementById("audit-usage-panel"),
    body: document.getElementById("audit-table-body"),
    actor: document.getElementById("audit-filter-actor"),
    resourceType: document.getElementById("audit-filter-resource-type"),
    since: document.getElementById("audit-filter-since"),
    until: document.getElementById("audit-filter-until"),
    reload: document.querySelector('[data-action="reload-audit"]'),
    apply: document.querySelector('[data-action="apply-audit-filters"]'),
    clear: document.querySelector('[data-action="clear-audit-filters"]'),
    usageAlert: document.getElementById("audit-usage-alert"),
    usageUser: document.getElementById("audit-usage-user"),
    usageSince: document.getElementById("audit-usage-since"),
    usageUntil: document.getElementById("audit-usage-until"),
    usageTopN: document.getElementById("audit-usage-top-n"),
    usageApply: document.querySelector('[data-action="apply-audit-usage-filters"]'),
    usageClear: document.querySelector('[data-action="clear-audit-usage-filters"]'),
    usageModelBody: document.getElementById("audit-usage-model-body"),
    usageRecentBody: document.getElementById("audit-usage-recent-body"),
    usageHourlyCanvas: document.getElementById("audit-usage-hourly-chart"),
    usageHourlyEmpty: document.getElementById("audit-usage-hourly-empty"),
    tabs: Array.from(document.querySelectorAll?.("[data-audit-view]") || []),
  };

  nodes.reload?.addEventListener("click", () => load(true));
  nodes.apply?.addEventListener("click", () => loadLogs(true));
  nodes.clear?.addEventListener("click", () => {
    nodes.actor.value = "";
    nodes.resourceType.value = "";
    nodes.since.value = "";
    nodes.until.value = "";
    loadLogs(true);
  });
  nodes.body?.addEventListener("click", (event) => {
    const row = event.target.closest?.("[data-audit-row]");
    if (!row) return;
    expandedRow = expandedRow === row.dataset.auditRow ? "" : row.dataset.auditRow;
    renderRows();
  });
  nodes.usageApply?.addEventListener("click", () => loadUsage(true));
  nodes.usageUser?.addEventListener("change", () => loadUsage(true));
  nodes.usageClear?.addEventListener("click", () => {
    nodes.usageSince.value = "";
    nodes.usageUntil.value = "";
    nodes.usageTopN.value = "10";
    loadUsage(true);
  });
  nodes.tabs.forEach((tab) => {
    tab.addEventListener("click", () => switchAuditView(tab.dataset.auditView || "logs"));
  });

  function switchAuditView(view) {
    activeView = view === "usage" ? "usage" : "logs";
    nodes.tabs.forEach((tab) => tab.classList.toggle("is-active", tab.dataset.auditView === activeView));
    nodes.logPanel?.classList.toggle("is-hidden", activeView !== "logs");
    nodes.usagePanel?.classList.toggle("is-hidden", activeView !== "usage");
    return load(false);
  }

  function renderRows() {
    if (!logs.length) {
      nodes.body.innerHTML = '<tr><td colspan="5" class="table-state">No admin audit logs</td></tr>';
      return;
    }

    nodes.body.innerHTML = logs.map((entry, index) => {
      const rowKey = `${entry.request_id || "audit"}-${index}`;
      const resourceID = entry.resource_id || "--";
      const statusClass = Number(entry.status_code) >= 400 ? "status-pill" : "status-pill status-pill-active";
      const detail = expandedRow === rowKey ? `
        <tr class="audit-json-row">
          <td colspan="5">
            <div class="audit-json-grid">
              <section>
                <h3>Before</h3>
                <pre>${escapeHTML(formatJSON(entry.before))}</pre>
              </section>
              <section>
                <h3>After</h3>
                <pre>${escapeHTML(formatJSON(entry.after))}</pre>
              </section>
            </div>
          </td>
        </tr>
      ` : "";

      return `
        <tr class="audit-row" data-audit-row="${escapeHTML(rowKey)}">
          <td>
            <div class="primary-text">${escapeHTML(entry.actor_id || "--")}</div>
            <div class="secondary-text">${escapeHTML(entry.actor_type || "--")}</div>
          </td>
          <td>${escapeHTML(entry.action || "--")}</td>
          <td>
            <div class="audit-resource">
              <span class="primary-text">${escapeHTML(entry.resource_type || "--")}</span>
              <span class="secondary-text">${escapeHTML(resourceID)}</span>
            </div>
          </td>
          <td><span class="${statusClass}">${escapeHTML(entry.status_code || "--")}</span></td>
          <td>${escapeHTML(formatDateTime(entry.created_at))}</td>
        </tr>
        ${detail}
      `;
    }).join("");
  }

  async function load(force = false) {
    if (activeView === "usage") {
      await ensureUsers();
      return loadUsage(force);
    }
    return loadLogs(force);
  }

  async function loadLogs(force = false) {
    if (loadedLogs && !force) return;
    setAlert(nodes.alert, "loading", "Loading admin audit logs...");
    nodes.body.innerHTML = '<tr><td colspan="5" class="table-state">Loading admin audit logs...</td></tr>';

    try {
      logs = normalizeAuditLogs(await api.getAuditLogs(currentFilters()));
      loadedLogs = true;
      expandedRow = "";
      setAlert(nodes.alert, logs.length ? "" : "empty", logs.length ? "" : "No admin audit logs.");
      renderRows();
    } catch (error) {
      setAlert(nodes.alert, "error", `Unable to load admin audit logs (${error.code || error.message}).`);
      nodes.body.innerHTML = '<tr><td colspan="5" class="table-state">Admin audit logs failed to load</td></tr>';
    }
  }

  async function ensureUsers() {
    if (loadedUsers) return;
    setAlert(nodes.usageAlert, "loading", "Loading users...");
    try {
      const response = await api.getUsers();
      const users = normalizeUsers(response?.users);
      nodes.usageUser.innerHTML = users.length
        ? users.map((user) => `<option value="${escapeHTML(user.user_id)}">${escapeHTML(user.label)}</option>`).join("")
        : '<option value="">No users</option>';
      if (users.length && !users.some((user) => user.user_id === nodes.usageUser.value)) {
        nodes.usageUser.value = users[0].user_id;
      }
      loadedUsers = true;
      setAlert(nodes.usageAlert, users.length ? "" : "empty", users.length ? "" : "No users found.");
    } catch (error) {
      setAlert(nodes.usageAlert, "error", `Unable to load users (${error.code || error.message}).`);
    }
  }

  async function loadUsage(force = false) {
    const userID = String(nodes.usageUser?.value || "").trim();
    if (!userID) {
      renderUsage(normalizeUserUsage(null));
      setAlert(nodes.usageAlert, "empty", "Select a user to inspect usage.");
      return;
    }
    if (!force && loadedUsageUserID === userID) return;

    setAlert(nodes.usageAlert, "loading", "Loading user usage...");
    setUsageTablesLoading();
    try {
      const usage = normalizeUserUsage(await api.getUserUsage(userID, currentUsageFilters()));
      loadedUsageUserID = userID;
      renderUsage(usage);
      setAlert(nodes.usageAlert, usage.recent_calls.length ? "" : "empty", usage.recent_calls.length ? "" : "No usage in this period.");
    } catch (error) {
      loadedUsageUserID = "";
      setAlert(nodes.usageAlert, "error", `Unable to load user usage (${error.code || error.message}).`);
      renderUsage(normalizeUserUsage(null));
    }
  }

  function setUsageTablesLoading() {
    nodes.usageModelBody.innerHTML = '<tr><td colspan="4" class="table-state">Loading user usage...</td></tr>';
    nodes.usageRecentBody.innerHTML = '<tr><td colspan="5" class="table-state">Loading user usage...</td></tr>';
  }

  function renderUsage(usage) {
    renderModelTop(usage.model_top);
    renderRecentCalls(usage.recent_calls);
    renderHourlyChart(usage.hourly_distribution);
  }

  function renderModelTop(rows) {
    if (!rows.length) {
      nodes.usageModelBody.innerHTML = '<tr><td colspan="4" class="table-state">No model usage</td></tr>';
      return;
    }
    const total = rows.reduce((sum, row) => sum + row.tokens, 0);
    nodes.usageModelBody.innerHTML = rows.map((row) => {
      const share = total > 0 ? `${((row.tokens / total) * 100).toFixed(1)}%` : "0.0%";
      return `
        <tr>
          <td class="primary-text">${escapeHTML(row.model)}</td>
          <td>${escapeHTML(formatTokens(row.tokens))}</td>
          <td>${escapeHTML(formatNumber(row.call_count))}</td>
          <td>${share}</td>
        </tr>
      `;
    }).join("");
  }

  function renderRecentCalls(rows) {
    if (!rows.length) {
      nodes.usageRecentBody.innerHTML = '<tr><td colspan="5" class="table-state">No recent calls</td></tr>';
      return;
    }
    nodes.usageRecentBody.innerHTML = rows.map((row) => {
      const statusClass = row.status_code >= 400 ? "status-pill" : "status-pill status-pill-active";
      return `
        <tr>
          <td>${escapeHTML(formatDateTime(row.created_at))}</td>
          <td class="primary-text">${escapeHTML(row.model)}</td>
          <td><span class="${statusClass}">${escapeHTML(row.status_code || "--")}</span></td>
          <td>${escapeHTML(formatTokens(row.total_tokens))}</td>
          <td>${row.streaming ? "Yes" : "No"}</td>
        </tr>
      `;
    }).join("");
  }

  function renderHourlyChart(rawHourly) {
    const localHourly = toLocalHourlyDistribution(rawHourly);
    const hasData = localHourly.some((count) => count > 0);
    setEmptyOverlay(nodes.usageHourlyEmpty, !hasData);
    if (!window.Chart || !nodes.usageHourlyCanvas?.getContext) return;
    if (!hourlyChart) {
      hourlyChart = new Chart(nodes.usageHourlyCanvas.getContext("2d"), {
        type: "bar",
        data: {
          labels: hourLabels(),
          datasets: [{
            label: "Calls",
            data: localHourly,
            backgroundColor: "#4f46e5",
            borderRadius: 6,
          }],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: { legend: { display: false } },
          scales: {
            y: { beginAtZero: true, ticks: { precision: 0 }, grid: { color: "#f1f5f9" }, border: { display: false } },
            x: { grid: { display: false }, border: { display: false } },
          },
        },
      });
      return;
    }
    hourlyChart.data.datasets[0].data = localHourly;
    hourlyChart.update();
  }

  function currentFilters() {
    return {
      actor_id: nodes.actor.value,
      resource_type: nodes.resourceType.value,
      since: toRFC3339(nodes.since.value),
      until: toRFC3339(nodes.until.value),
      limit: 100,
    };
  }

  function currentUsageFilters() {
    return {
      since: toRFC3339(nodes.usageSince.value),
      until: toRFC3339(nodes.usageUntil.value),
      top_n: Math.min(50, Math.max(1, Number(nodes.usageTopN.value) || 10)),
    };
  }

  return { load, switchAuditView };
}

function normalizeAuditLogs(raw) {
  const rows = Array.isArray(raw) ? raw : [];
  return rows.map((entry) => ({
    actor_id: String(entry.actor_id || "").trim(),
    actor_type: String(entry.actor_type || "").trim(),
    action: String(entry.action || "").trim(),
    resource_type: String(entry.resource_type || "").trim(),
    resource_id: entry.resource_id === null || entry.resource_id === undefined ? "" : String(entry.resource_id).trim(),
    before: entry.before === undefined ? null : entry.before,
    after: entry.after === undefined ? null : entry.after,
    ip: entry.ip === null || entry.ip === undefined ? "" : String(entry.ip).trim(),
    user_agent: String(entry.user_agent || "").trim(),
    request_id: String(entry.request_id || "").trim(),
    status_code: Math.max(0, Number(entry.status_code) || 0),
    created_at: String(entry.created_at || "").trim(),
  }));
}

function normalizeUsers(raw) {
  const rows = Array.isArray(raw) ? raw : [];
  return rows.map((user) => {
    const name = String(user.display_name || user.email || user.user_id || "").trim();
    const tokens = Math.max(0, Number(user.used_tokens) || 0);
    return {
      user_id: String(user.user_id || "").trim(),
      label: `${name || "unknown"} - ${formatTokens(tokens)} tokens`,
    };
  }).filter((user) => user.user_id);
}

function normalizeUserUsage(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const hourly = Array.isArray(source.hourly_distribution) ? source.hourly_distribution : [];
  return {
    user_id: String(source.user_id || "").trim(),
    period: source.period || {},
    model_top: normalizeModelTop(source.model_top),
    hourly_distribution: Array.from({ length: 24 }, (_, index) => Math.max(0, Number(hourly[index]) || 0)),
    recent_calls: normalizeRecentCalls(source.recent_calls),
  };
}

function normalizeModelTop(raw) {
  const rows = Array.isArray(raw) ? raw : [];
  return rows.map((row) => ({
    model: String(row.model || "unknown").trim() || "unknown",
    tokens: Math.max(0, Number(row.tokens ?? row.total_tokens) || 0),
    call_count: Math.max(0, Number(row.call_count) || 0),
  }));
}

function normalizeRecentCalls(raw) {
  const rows = Array.isArray(raw) ? raw : [];
  return rows.map((row) => ({
    created_at: String(row.created_at || "").trim(),
    model: String(row.model || "unknown").trim() || "unknown",
    status_code: Math.max(0, Number(row.status_code) || 0),
    total_tokens: Math.max(0, Number(row.total_tokens) || 0),
    streaming: Boolean(row.streaming),
  }));
}

function toRFC3339(value) {
  value = String(value || "").trim();
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

function toLocalHourlyDistribution(rawHourly) {
  const output = Array.from({ length: 24 }, () => 0);
  const offsetHours = Math.round(-new Date().getTimezoneOffset() / 60);
  rawHourly.forEach((count, utcHour) => {
    const localHour = (utcHour + offsetHours + 24) % 24;
    output[localHour] += Math.max(0, Number(count) || 0);
  });
  return output;
}

function hourLabels() {
  return Array.from({ length: 24 }, (_, hour) => `${String(hour).padStart(2, "0")}:00`);
}

function formatDateTime(value) {
  if (!value) return "--";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatJSON(value) {
  if (value === null || value === undefined) return "null";
  return JSON.stringify(value, null, 2);
}

window.OmniTokenViews = { ...(window.OmniTokenViews || {}), createAuditView };
})();
