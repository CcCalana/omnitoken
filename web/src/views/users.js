(function () {
const { escapeHTML, formatTokens, formatUSD, setAlert } = window.OmniTokenUtils;
const { openModal } = window.OmniTokenModal || {};
const { showToast } = window.OmniTokenToast || {};

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
    open: document.querySelector('[data-action="open-user-modal"]'),
  };

  syncRoleControls();
  nodes.reload?.addEventListener("click", () => load(true));
  nodes.open?.addEventListener("click", openCreateUserModal);
  nodes.body?.addEventListener("click", (event) => {
    const button = event.target && event.target.closest ? event.target.closest("[data-action]") : null;
    if (!button) return;
    const userID = button.dataset.userId || "";
    switch (button.dataset.action) {
      case "generate-key":
        openGenerateKeyModal(userID);
        break;
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
    syncRoleControls();
    if (!users.length) {
      nodes.body.innerHTML = '<tr><td colspan="6" class="table-state table-state-illustrated empty-state-access">暂无用户用量数据</td></tr>';
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
    return `
      <div class="quota-actions">
        <button class="mini-button" type="button" data-action="generate-key" data-user-id="${escapeHTML(user.user_id)}">生成 Key</button>
        <button class="mini-button ghost-mini-button" type="button" data-action="edit-quota" data-user-id="${escapeHTML(user.user_id)}">编辑</button>
      </div>
    `;
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

  function syncRoleControls() {
    if (nodes.open) nodes.open.hidden = getRole() !== "admin";
  }

  function openCreateUserModal() {
    if (getRole() !== "admin") return;
    if (typeof openModal !== "function") {
      setAlert(nodes.alert, "error", "无法打开新建用户窗口。");
      return;
    }

    const body = document.createElement("div");
    body.innerHTML = `
      <form id="create-user-form" class="modal-form">
        <div class="form-grid">
          <label>
            <span>邮箱</span>
            <input name="email" type="email" autocomplete="off" required>
          </label>
          <label>
            <span>显示名</span>
            <input name="display_name" type="text" required>
          </label>
          <label>
            <span>角色</span>
            <select name="role" required>
              <option value="member">member</option>
              <option value="viewer">viewer</option>
              <option value="admin">admin</option>
            </select>
          </label>
          <label>
            <span>初始密码</span>
            <input name="password" type="password" autocomplete="new-password" required>
          </label>
        </div>
        <div class="modal-actions">
          <button class="secondary-button" type="button" data-action="close-create-user">取消</button>
          <button class="primary-button" type="submit">创建用户</button>
        </div>
      </form>
      <div class="key-result is-hidden" data-create-user-result></div>
    `;

    const modal = openModal({ title: "新建用户", body });
    body.querySelector('[data-action="close-create-user"]')?.addEventListener("click", () => modal.close());
    body.querySelector("#create-user-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      await submitCreateUser(event.currentTarget, body.querySelector("[data-create-user-result]"));
    });
  }

  async function submitCreateUser(form, resultNode) {
    const submit = form.querySelector('button[type="submit"]');
    submit.disabled = true;
    const payload = {
      email: form.elements.email.value.trim(),
      display_name: form.elements.display_name.value.trim(),
      role: form.elements.role.value,
      password: form.elements.password.value,
    };

    try {
      const created = await api.createUser(payload);
      form.reset();
      renderGenerateKeyPrompt(resultNode, normalizeUsers({ users: [created] })[0]);
      await load(true);
    } catch (error) {
      setAlert(nodes.alert, "error", `无法创建用户 (${error.code || error.message})。`);
    } finally {
      submit.disabled = false;
    }
  }

  function openGenerateKeyModal(userID) {
    if (typeof openModal !== "function") {
      setAlert(nodes.alert, "error", "无法打开 Key 生成窗口。");
      return;
    }
    const user = users.find((item) => item.user_id === userID);
    if (!user) return;
    const body = document.createElement("div");
    const result = document.createElement("div");
    body.appendChild(result);
    openModal({
      title: `生成 Key · ${user.display_name || user.email}`,
      body,
      actions: [{ label: "关闭" }],
    });
    renderGenerateKeyPrompt(result, user);
  }

  function renderGenerateKeyPrompt(container, user) {
    if (!container) return;
    container.classList.remove("is-hidden");
    container.innerHTML = `
      <div class="key-result-panel">
        <p class="modal-copy">为 <strong>${escapeHTML(user.display_name || user.email)}</strong> 生成一个新的 virtual key。</p>
        <button class="primary-button" type="button" data-action="confirm-generate-key">生成 Key</button>
      </div>
    `;
    container.querySelector('[data-action="confirm-generate-key"]')?.addEventListener("click", () => generateKeyForUser(user, container));
  }

  async function generateKeyForUser(user, container) {
    const orgID = String(user.organization_id || "").trim();
    if (!orgID) {
      container.innerHTML = '<p class="modal-copy">该用户缺少 organization_id，无法生成 Key。</p>';
      return;
    }
    const button = container.querySelector('[data-action="confirm-generate-key"]');
    if (button) button.disabled = true;
    try {
      const response = await api.createVirtualKey({
        organization_id: orgID,
        user_id: user.user_id,
      });
      renderCreatedKey(container, response);
    } catch (error) {
      container.innerHTML = `<p class="modal-copy">无法生成 Key (${escapeHTML(error.code || error.message)})。</p>`;
    }
  }

  function renderCreatedKey(container, response) {
    const key = String(response?.virtual_key || "");
    const prefix = String(response?.key_prefix || "");
    container.innerHTML = `
      <div class="key-result-panel">
        <p class="security-note">请立即复制此 Key，关闭后不可再次查看</p>
        <code class="key-code">${escapeHTML(key)}</code>
        <div class="key-actions">
          <span class="secondary-text">prefix: ${escapeHTML(prefix)}</span>
          <button class="primary-button" type="button" data-action="copy-generated-key">复制</button>
        </div>
      </div>
    `;
    container.querySelector('[data-action="copy-generated-key"]')?.addEventListener("click", async (event) => {
      await copyText(key);
      event.currentTarget.textContent = "已复制";
    });
  }

  async function copyText(value) {
    if (navigator.clipboard && typeof navigator.clipboard.writeText === "function") {
      await navigator.clipboard.writeText(value);
    } else {
      const textarea = document.createElement("textarea");
      textarea.value = value;
      textarea.setAttribute("readonly", "");
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      textarea.remove();
    }
    if (typeof showToast === "function") showToast("Key 已复制", "success");
  }

  async function load(force = false) {
    if (loaded && !force) return;
    syncRoleControls();
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
      organization_id: String(user.organization_id || "").trim(),
      display_name: displayName,
      email,
      role: String(user.role || "").trim(),
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
