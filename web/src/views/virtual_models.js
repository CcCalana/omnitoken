(function () {
const { showAlert, escapeHTML } = window.OmniTokenUtils;

function createVirtualModelsView(api) {
  let isLoaded = false;
  const tbody = document.getElementById("virtual-models-table-body");
  const alertContainer = document.getElementById("virtual-models-alert");
  const reloadButton = document.querySelector('[data-action="reload-virtual-models"]');

  if (reloadButton) {
    reloadButton.addEventListener("click", () => load(true));
  }

  async function load(force = false) {
    if (isLoaded && !force) return;

    showAlert(alertContainer, "");
    tbody.innerHTML = `<tr><td colspan="4" class="table-state">加载中...</td></tr>`;

    try {
      const response = await api.getVirtualModels();
      render(response.virtual_models || []);
      isLoaded = true;
    } catch (error) {
      tbody.innerHTML = `<tr><td colspan="4" class="table-state table-error">加载失败</td></tr>`;
      showAlert(alertContainer, error.message || "无法加载虚拟模型映射", "error");
    }
  }

  function render(models) {
    if (models.length === 0) {
      tbody.innerHTML = `<tr><td colspan="4" class="table-state">暂无映射记录</td></tr>`;
      return;
    }

    tbody.innerHTML = models
      .map((m) => {
        const badgeClass = m.status === "active" ? "badge-success" : "badge-secondary";
        return `
          <tr>
            <td class="font-mono"><strong>${escapeHTML(m.name)}</strong></td>
            <td class="font-mono">${escapeHTML(m.real_model)}</td>
            <td><span class="badge ${badgeClass}">${escapeHTML(m.status)}</span></td>
            <td class="text-secondary">${escapeHTML(m.description || "")}</td>
          </tr>
        `;
      })
      .join("");
  }

  return { load };
}

window.OmniTokenViews = window.OmniTokenViews || {};
window.OmniTokenViews.createVirtualModelsView = createVirtualModelsView;
})();
