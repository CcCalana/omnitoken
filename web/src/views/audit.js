(function () {
const { escapeHTML, setAlert } = window.OmniTokenUtils;

function createAuditView(api) {
  let logs = [];
  let loaded = false;
  let expandedRow = "";

  const nodes = {
    alert: document.getElementById("audit-alert"),
    body: document.getElementById("audit-table-body"),
    actor: document.getElementById("audit-filter-actor"),
    resourceType: document.getElementById("audit-filter-resource-type"),
    since: document.getElementById("audit-filter-since"),
    until: document.getElementById("audit-filter-until"),
    reload: document.querySelector('[data-action="reload-audit"]'),
    apply: document.querySelector('[data-action="apply-audit-filters"]'),
    clear: document.querySelector('[data-action="clear-audit-filters"]'),
  };

  nodes.reload?.addEventListener("click", () => load(true));
  nodes.apply?.addEventListener("click", () => load(true));
  nodes.clear?.addEventListener("click", () => {
    nodes.actor.value = "";
    nodes.resourceType.value = "";
    nodes.since.value = "";
    nodes.until.value = "";
    load(true);
  });
  nodes.body?.addEventListener("click", (event) => {
    const row = event.target.closest?.("[data-audit-row]");
    if (!row) return;
    expandedRow = expandedRow === row.dataset.auditRow ? "" : row.dataset.auditRow;
    renderRows();
  });

  function renderRows() {
    if (!logs.length) {
      nodes.body.innerHTML = '<tr><td colspan="5" class="table-state">暂无审计日志</td></tr>';
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
    if (loaded && !force) return;
    setAlert(nodes.alert, "loading", "正在加载审计日志...");
    nodes.body.innerHTML = '<tr><td colspan="5" class="table-state">正在加载审计日志...</td></tr>';

    try {
      logs = normalizeAuditLogs(await api.getAuditLogs(currentFilters()));
      loaded = true;
      expandedRow = "";
      setAlert(nodes.alert, logs.length ? "" : "empty", logs.length ? "" : "暂无审计日志。");
      renderRows();
    } catch (error) {
      setAlert(nodes.alert, "error", `无法加载审计日志 (${error.code || error.message})。请确认 admin 服务已启动，且 CORS 允许当前页面 origin。`);
      nodes.body.innerHTML = '<tr><td colspan="5" class="table-state">审计日志加载失败</td></tr>';
    }
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

  return { load };
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

function toRFC3339(value) {
  value = String(value || "").trim();
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
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
