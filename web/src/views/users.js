(function () {
const { escapeHTML, formatTokens, setAlert } = window.OmniTokenUtils;

function createUsersView(api) {
  let users = [];
  let loaded = false;

  const nodes = {
    alert: document.getElementById("users-alert"),
    body: document.getElementById("users-table-body"),
    reload: document.querySelector('[data-action="reload-users"]'),
  };

  nodes.reload?.addEventListener("click", () => load(true));

  function renderRows() {
    if (!users.length) {
      nodes.body.innerHTML = '<tr><td colspan="5" class="table-state">暂无用户用量数据</td></tr>';
      return;
    }

    const rows = [...users].sort((a, b) => {
      const aRatio = a.quota > 0 ? a.used_tokens / a.quota : -1;
      const bRatio = b.quota > 0 ? b.used_tokens / b.quota : -1;
      if (aRatio !== bRatio) return bRatio - aRatio;
      return b.used_tokens - a.used_tokens;
    });

    nodes.body.innerHTML = rows.map((user) => {
      const initial = Array.from(user.display_name || user.email || "?")[0] || "?";
      const hasQuota = user.quota > 0;
      const percent = hasQuota ? Math.min(100, (user.used_tokens / user.quota) * 100) : 0;
      const progressClass = percent >= 100
        ? "progress-fill progress-fill-danger"
        : percent >= 85
          ? "progress-fill progress-fill-warning"
          : "progress-fill";
      const progress = hasQuota
        ? `<div class="progress-track"><div class="${progressClass}" style="width: ${percent}%"></div></div>`
        : '<span class="quota-open">无限额</span>';
      const statusClass = user.status === "active" ? "status-pill status-pill-active" : "status-pill";

      return `
        <tr>
          <td>
            <div class="user-cell">
              <div class="avatar">${escapeHTML(initial)}</div>
              <div>
                <div class="primary-text">${escapeHTML(user.display_name)}</div>
                <div class="secondary-text">${escapeHTML(user.email || "未提供邮箱")}</div>
              </div>
            </div>
          </td>
          <td>${formatTokens(user.used_tokens)}</td>
          <td>${hasQuota ? formatTokens(user.quota) : '<span class="quota-open">无限额</span>'}</td>
          <td>${progress}</td>
          <td class="align-right"><span class="${statusClass}">${escapeHTML(user.status)}</span></td>
        </tr>
      `;
    }).join("");
  }

  async function load(force = false) {
    if (loaded && !force) return;
    setAlert(nodes.alert, "loading", "正在加载用户用量数据...");
    nodes.body.innerHTML = '<tr><td colspan="5" class="table-state">正在加载用户用量数据...</td></tr>';

    try {
      const payload = await api.getUsers();
      users = normalizeUsers(payload);
      loaded = true;
      setAlert(nodes.alert, users.length ? "" : "empty", users.length ? "" : "暂无用户用量数据。");
      renderRows();
    } catch (error) {
      setAlert(nodes.alert, "error", `无法加载用户用量 (${error.code || error.message})。请确认 admin 服务已启动，且 CORS 允许当前页面 origin。`);
      nodes.body.innerHTML = '<tr><td colspan="5" class="table-state">用户用量加载失败</td></tr>';
    }
  }

  return { load };
}

function normalizeUsers(raw) {
  const rows = raw && Array.isArray(raw.users) ? raw.users : [];
  return rows.map((user) => {
    const displayName = String(user.display_name || user.email || "Unknown User").trim();
    const email = String(user.email || "").trim();
    return {
      user_id: String(user.user_id || email || displayName),
      display_name: displayName,
      email,
      used_tokens: Math.max(0, Number(user.used_tokens) || 0),
      quota: Math.max(0, Number(user.quota) || 0),
      status: String(user.status || "unknown").trim() || "unknown",
    };
  });
}

window.OmniTokenViews = { ...(window.OmniTokenViews || {}), createUsersView };
})();
