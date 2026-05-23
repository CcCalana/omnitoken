(function () {
const { escapeHTML, setAlert } = window.OmniTokenUtils;

function createCredentialsView(api) {
  let loaded = false;
  let bannerTimer = null;

  const nodes = {
    alert: document.getElementById("credentials-alert"),
    banner: document.getElementById("credentials-banner"),
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
    if (!window.confirm("Disable this upstream credential?")) return;
    button.disabled = true;
    try {
      await api.disableCredential(id);
      showReloadBanner();
      await load(true);
    } catch (error) {
      setAlert(nodes.alert, "error", `Disable failed (${error.code || error.message}).`);
    } finally {
      button.disabled = false;
    }
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
      setAlert(nodes.alert, "error", `Create failed (${error.code || error.message}).`);
    }
  }

  function renderRows(credentials) {
    if (!credentials.length) {
      nodes.body.innerHTML = '<tr><td colspan="6" class="table-state">No upstream credentials</td></tr>';
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
            <button class="ghost-mini-button mini-button" type="button" data-disable-credential="${escapeHTML(item.id)}" ${isActive ? "" : "disabled"}>Disable</button>
          </td>
        </tr>
      `;
    }).join("");
  }

  async function load(force = false) {
    if (loaded && !force) return;
    setAlert(nodes.alert, "loading", "Loading upstream credentials...");
    nodes.body.innerHTML = '<tr><td colspan="6" class="table-state">Loading upstream credentials</td></tr>';
    try {
      const response = await api.getCredentials();
      const credentials = Array.isArray(response?.credentials) ? response.credentials : [];
      loaded = true;
      setAlert(nodes.alert, credentials.length ? "" : "empty", credentials.length ? "" : "No upstream credentials.");
      renderRows(credentials);
    } catch (error) {
      setAlert(nodes.alert, "error", `Unable to load credentials (${error.code || error.message}).`);
      nodes.body.innerHTML = '<tr><td colspan="6" class="table-state">Credential load failed</td></tr>';
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
    if (!nodes.banner) return;
    setAlert(nodes.banner, "loading", "Written to DB. Gateway will reload the credential pool within 30s; restart gateway for immediate effect.");
    nodes.banner.classList.remove("is-hidden");
    clearTimeout(bannerTimer);
    bannerTimer = setTimeout(() => nodes.banner.classList.add("is-hidden"), 30000);
  }

  return { load };
}

window.OmniTokenViews = { ...(window.OmniTokenViews || {}), createCredentialsView };
})();
