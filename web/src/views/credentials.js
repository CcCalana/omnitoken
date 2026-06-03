(function () {
const { escapeHTML, setAlert } = window.OmniTokenUtils;
const { showToast } = window.OmniTokenToast || {};
const { confirmModal } = window.OmniTokenModal || {};

function createCredentialsView(api) {
  let loaded = false;
  const nodes = {
    alert: document.getElementById("credentials-alert"),
    body: document.getElementById("credentials-table-body"),
    open: document.querySelector('[data-action="open-credential-modal"]'),
    modal: document.getElementById("credential-modal"),
    form: document.getElementById("credential-form"),
    provider: document.getElementById("credential-provider"),
    alias: document.getElementById("credential-alias"),
    priority: document.getElementById("credential-priority"),
    baseURL: document.getElementById("credential-base-url"),
    key: document.getElementById("credential-key"),
  };

  nodes.open?.addEventListener("click", openModal);
  document.querySelectorAll('[data-action="close-credential-modal"]').forEach((button) => {
    button.addEventListener("click", closeModal);
  });
  nodes.provider?.addEventListener("change", () => {
    if (nodes.provider.value === "ark") {
      nodes.baseURL.value = "https://ark.cn-beijing.volces.com/api/coding/v3";
    } else {
      nodes.baseURL.value = "https://api.deepseek.com/v1";
    }
  });
  nodes.form?.addEventListener("submit", submitCredential);
  nodes.body?.addEventListener("click", async (event) => {
    const button = event.target.closest?.("[data-disable-credential]");
    if (!button) return;
    const id = button.dataset.disableCredential;
    confirmDisable(id, button);
  });

  async function submitCredential(event) {
    event.preventDefault();
    const payload = {
      provider: nodes.provider.value,
      alias: nodes.alias.value.trim(),
      priority: Number(nodes.priority.value),
      base_url: nodes.baseURL.value.trim(),
      key: nodes.key.value,
    };
    try {
      await api.createCredential(payload);
      nodes.key.value = "";
      closeModal();
      showReloadBanner();
      await load(true);
    } catch (error) {
      setAlert(nodes.alert, "error", `创建失败 (${error.code || error.message}).`);
    }
  }

  function renderRows(credentials) {
    if (!credentials.length) {
      nodes.body.innerHTML = '<tr><td colspan="6" class="table-state">暂无上游凭据</td></tr>';
      return;
    }
    nodes.body.innerHTML = credentials.map((item) => {
      const alias = item.metadata?.alias || "--";
      const isActive = item.status === "active";
      const statusClass = isActive ? "status-pill status-pill-active" : "status-pill";
      return `
        <tr>
          <td>
            <div class="primary-text">${escapeHTML(item.provider || "--")}</div>
            <div class="secondary-text">${escapeHTML(alias)}</div>
          </td>
          <td>${escapeHTML(item.priority ?? "--")}</td>
          <td><span class="mono-cell">${escapeHTML(item.base_url || "--")}</span></td>
          <td><span class="${statusClass}">${escapeHTML(item.status || "--")}</span></td>
          <td>${escapeHTML(item.health_state || "--")}</td>
          <td class="align-right">
            <button class="ghost-mini-button mini-button" type="button" data-disable-credential="${escapeHTML(item.id)}" ${isActive ? "" : "disabled"}>禁用</button>
          </td>
        </tr>
      `;
    }).join("");
  }

  async function load(force = false) {
    if (loaded && !force) return;
    setAlert(nodes.alert, "loading", "正在加载上游凭据...");
    nodes.body.innerHTML = '<tr><td colspan="6" class="table-state">正在加载上游凭据</td></tr>';
    try {
      const response = await api.getCredentials();
      const credentials = Array.isArray(response?.credentials) ? response.credentials : [];
      loaded = true;
      setAlert(nodes.alert, credentials.length ? "" : "empty", credentials.length ? "" : "暂无上游凭据.");
      renderRows(credentials);
    } catch (error) {
      setAlert(nodes.alert, "error", `无法加载凭据 (${error.code || error.message}).`);
      nodes.body.innerHTML = '<tr><td colspan="6" class="table-state">凭据加载失败</td></tr>';
    }
  }

  function openModal() {
    nodes.modal?.classList.remove("is-hidden");
    nodes.alias?.focus();
  }

  function closeModal() {
    nodes.modal?.classList.add("is-hidden");
    nodes.form?.reset();
    if (nodes.baseURL) nodes.baseURL.value = "https://api.deepseek.com/v1";
  }

  function showReloadBanner() {
    if (typeof showToast === "function") {
      showToast("已写入数据库。Gateway 会在 30 秒内重新加载 credential pool。", "success");
    } else {
      setAlert(nodes.alert, "loading", "凭据已写入数据库，Gateway 将在 30 秒内热加载生效");
    }
  }

  function confirmDisable(id, button) {
    const run = async () => {
      button.disabled = true;
      try {
        await api.disableCredential(id);
        showReloadBanner();
        await load(true);
      } catch (error) {
        setAlert(nodes.alert, "error", `禁用失败 (${error.code || error.message}).`);
      } finally {
        button.disabled = false;
      }
    };
    if (typeof confirmModal === "function") {
      confirmModal({
        title: "禁用凭据",
        message: "确定禁用该上游凭据？",
        confirmLabel: "禁用",
        onConfirm: run,
      });
      return;
    }
    run();
  }

  return { load };
}

window.OmniTokenViews = { ...(window.OmniTokenViews || {}), createCredentialsView };
})();
