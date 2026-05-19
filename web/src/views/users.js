(function () {
const { escapeHTML, formatTokens, formatUSD, setAlert } = window.OmniTokenUtils;

function createUsersView(api, options = {}) {
  let users = [];
  let loaded = false;
  let editingUserID = "";

  const getRole = typeof options.getRole === "function"
    ? options.getRole
    : () => options.role || "admin";

  const nodes = {
    alert: document.getElementById("users-alert"),
    body: document.getElementById("users-table-body"),
    reload: document.querySelector('[data-action="reload-users"]'),
  };

  nodes.reload?.addEventListener("click", () => load(true));
  nodes.body?.addEventListener("click", (event) => {
    const button = event.target && event.target.closest ? event.target.closest("[data-action]") : null;
    if (!button) return;
    const userID = button.dataset.userId || "";
    switch (button.dataset.action) {
      case "edit-quota":
        editingUserID = userID;
        renderRows();
        break;
      case "cancel-quota":
        editingUserID = "";
        renderRows();
        break;
      case "clear-quota":
        saveQuota(userID, null);
        break;
      case "save-quota":
        try {
          saveQuota(userID, quotaInputCents(userID));
        } catch (error) {
          setAlert(nodes.alert, "error", `无法保存用户预算 (${error.message})。`);
        }
        break;
    }
  });

  function renderRows() {
    if (!users.length) {
      nodes.body.innerHTML = '<tr><td colspan="6" class="table-state">暂无用户用量数据</td></tr>';
      return;
    }

    const rows = [...users].sort((a, b) => {
      const aRatio = a.budget_cents !== null && a.budget_cents > 0 ? a.used_budget_cents / a.budget_cents : -1;
      const bRatio = b.budget_cents !== null && b.budget_cents > 0 ? b.used_budget_cents / b.budget_cents : -1;
      if (aRatio !== bRatio) return bRatio - aRatio;
      return b.used_tokens - a.used_tokens;
    });
    const canEdit = getRole() === "admin";

    nodes.body.innerHTML = rows.map((user) => {
      const initial = Array.from(user.display_name || user.email || "?")[0] || "?";
      const hasBudget = user.budget_cents !== null;
      const percent = hasBudget && user.budget_cents > 0 ? Math.min(100, (user.used_budget_cents / user.budget_cents) * 100) : 0;
      const progressClass = percent >= 100
        ? "progress-fill progress-fill-danger"
        : percent >= 85
          ? "progress-fill progress-fill-warning"
          : "progress-fill";
      const progressTitle = "进度按分向上取整展示；拦截按账本 decimal 成本精确比较。";
      const progress = hasBudget
        ? `<div class="progress-track" title="${progressTitle}"><div class="${progressClass}" style="width: ${percent}%"></div></div><span class="quota-caption">${formatCents(user.used_budget_cents)} / ${formatCents(user.budget_cents)}</span>`
        : '<span class="quota-open">无限额</span>';
      const budgetCell = editingUserID === user.user_id
        ? renderBudgetEditor(user)
        : hasBudget
          ? `<span title="${progressTitle}">${formatCents(user.budget_cents)}</span>`
          : '<span class="quota-open">无限额</span>';
      const actions = canEdit
        ? renderActions(user)
        : "";
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
          <td>${budgetCell}</td>
          <td>${progress}</td>
          <td class="align-right"><span class="${statusClass}">${escapeHTML(user.status)}</span></td>
          <td class="align-right">${actions}</td>
        </tr>
      `;
    }).join("");
  }

  function renderBudgetEditor(user) {
    const value = user.budget_cents === null ? "" : (user.budget_cents / 100).toFixed(2);
    return `
      <div class="quota-edit">
        <input id="quota-input-${escapeHTML(user.user_id)}" type="number" min="0" step="0.01" value="${escapeHTML(value)}" aria-label="月度预算 USD">
        <span>USD</span>
      </div>
    `;
  }

  function renderActions(user) {
    if (editingUserID === user.user_id) {
      return `
        <div class="quota-actions">
          <button class="mini-button" type="button" data-action="save-quota" data-user-id="${escapeHTML(user.user_id)}">保存</button>
          <button class="mini-button" type="button" data-action="clear-quota" data-user-id="${escapeHTML(user.user_id)}">无限制</button>
          <button class="mini-button ghost-mini-button" type="button" data-action="cancel-quota" data-user-id="${escapeHTML(user.user_id)}">取消</button>
        </div>
      `;
    }
    return `<button class="mini-button" type="button" data-action="edit-quota" data-user-id="${escapeHTML(user.user_id)}">编辑</button>`;
  }

  function quotaInputCents(userID) {
    const input = document.getElementById(`quota-input-${userID}`);
    const raw = String(input?.value || "").trim();
    if (raw === "") return null;
    const value = Number(raw);
    if (!Number.isFinite(value) || value < 0) {
      throw new Error("budget_cents must be non-negative");
    }
    return Math.round(value * 100);
  }

  async function saveQuota(userID, budgetCents) {
    try {
      setAlert(nodes.alert, "loading", "正在保存用户预算...");
      await api.updateUserQuota(userID, budgetCents);
      editingUserID = "";
      await load(true);
    } catch (error) {
      setAlert(nodes.alert, "error", `无法保存用户预算 (${error.code || error.message})。`);
    }
  }

  async function load(force = false) {
    if (loaded && !force) return;
    setAlert(nodes.alert, "loading", "正在加载用户用量数据...");
    nodes.body.innerHTML = '<tr><td colspan="6" class="table-state">正在加载用户用量数据...</td></tr>';

    try {
      const payload = await api.getUsers();
      users = normalizeUsers(payload);
      loaded = true;
      setAlert(nodes.alert, users.length ? "" : "empty", users.length ? "" : "暂无用户用量数据。");
      renderRows();
    } catch (error) {
      setAlert(nodes.alert, "error", `无法加载用户用量 (${error.code || error.message})。请确认 admin 服务已启动，且 CORS 允许当前页面 origin。`);
      nodes.body.innerHTML = '<tr><td colspan="6" class="table-state">用户用量加载失败</td></tr>';
    }
  }

  return { load };
}

function normalizeUsers(raw) {
  const rows = raw && Array.isArray(raw.users) ? raw.users : [];
  return rows.map((user) => {
    const displayName = String(user.display_name || user.email || "Unknown User").trim();
    const email = String(user.email || "").trim();
    const hasBudgetField = Object.prototype.hasOwnProperty.call(user, "budget_cents");
    const legacyQuota = Math.max(0, Number(user.quota) || 0);
    let budgetCents = null;
    if (hasBudgetField) {
      budgetCents = user.budget_cents === null ? null : Math.max(0, Math.round(Number(user.budget_cents) || 0));
    } else if (legacyQuota > 0) {
      budgetCents = legacyQuota;
    }
    return {
      user_id: String(user.user_id || email || displayName),
      display_name: displayName,
      email,
      used_tokens: Math.max(0, Number(user.used_tokens) || 0),
      used_budget_cents: Math.max(0, Math.round(Number(user.used_budget_cents) || 0)),
      budget_cents: budgetCents,
      quota: budgetCents || 0,
      status: String(user.status || "unknown").trim() || "unknown",
    };
  });
}

function formatCents(cents) {
  return formatUSD((Number(cents) || 0) / 100);
}

window.OmniTokenViews = { ...(window.OmniTokenViews || {}), createUsersView };
})();
