(function () {
const { escapeHTML, setAlert } = window.OmniTokenUtils;

function createVirtualModelsView(api) {
  let isLoaded = false;
  let currentMode = "create";
  let models = [];

  const nodes = {
    body: document.getElementById("virtual-models-table-body"),
    alert: document.getElementById("virtual-models-alert"),
    reload: document.querySelector('[data-action="reload-virtual-models"]'),
    open: document.querySelector('[data-action="open-virtual-model-modal"]'),
    modal: document.getElementById("virtual-model-modal"),
    form: document.getElementById("virtual-model-form"),
    title: document.getElementById("virtual-model-modal-title"),
    name: document.getElementById("virtual-model-name"),
    realModel: document.getElementById("virtual-model-real-model"),
    provider: document.getElementById("virtual-model-provider"),
    status: document.getElementById("virtual-model-status"),
    description: document.getElementById("virtual-model-description"),
  };

  nodes.reload?.addEventListener("click", () => load(true));
  nodes.open?.addEventListener("click", () => openModal());
  document.querySelectorAll('[data-action="close-virtual-model-modal"]').forEach((button) => {
    button.addEventListener("click", closeModal);
  });
  nodes.form?.addEventListener("submit", submit);
  nodes.body?.addEventListener("click", onTableClick);

  async function load(force = false) {
    if (isLoaded && !force) return;

    setAlert(nodes.alert, "", "");
    nodes.body.innerHTML = `<tr><td colspan="6" class="table-state">加载中...</td></tr>`;

    try {
      const response = await api.getVirtualModels();
      models = Array.isArray(response?.virtual_models) ? response.virtual_models : [];
      render(models);
      isLoaded = true;
    } catch (error) {
      nodes.body.innerHTML = `<tr><td colspan="6" class="table-state table-error">加载失败</td></tr>`;
      setAlert(nodes.alert, "error", error.message || "无法加载虚拟模型映射");
    }
  }

  function render(items) {
    if (!items.length) {
      nodes.body.innerHTML = `<tr><td colspan="6" class="table-state">暂无映射记录</td></tr>`;
      return;
    }

    nodes.body.innerHTML = items.map((model) => {
      const active = model.status === "active";
      const statusClass = active ? "status-pill status-pill-active" : "status-pill";
      const nextStatus = active ? "disabled" : "active";
      const toggleLabel = active ? "禁用" : "启用";
      return `
        <tr>
          <td class="font-mono"><strong>${escapeHTML(model.name)}</strong></td>
          <td class="font-mono">${escapeHTML(model.real_model)}</td>
          <td>${escapeHTML(model.provider || "ark")}</td>
          <td><span class="${statusClass}">${escapeHTML(model.status)}</span></td>
          <td class="text-secondary">${escapeHTML(model.description || "")}</td>
          <td class="align-right">
            <button class="ghost-mini-button mini-button" type="button" data-edit-virtual-model="${escapeHTML(model.name)}">编辑</button>
            <button class="ghost-mini-button mini-button" type="button" data-toggle-virtual-model="${escapeHTML(model.name)}" data-next-status="${nextStatus}">${toggleLabel}</button>
          </td>
        </tr>
      `;
    }).join("");
  }

  async function onTableClick(event) {
    const edit = event.target.closest?.("[data-edit-virtual-model]");
    if (edit) {
      const model = models.find((item) => item.name === edit.dataset.editVirtualModel);
      if (model) openModal(model);
      return;
    }
    const toggle = event.target.closest?.("[data-toggle-virtual-model]");
    if (!toggle) return;
    toggle.disabled = true;
    try {
      await api.updateVirtualModel(toggle.dataset.toggleVirtualModel, { status: toggle.dataset.nextStatus });
      setAlert(nodes.alert, "loading", "状态已更新。");
      await load(true);
    } catch (error) {
      setAlert(nodes.alert, "error", `更新失败 (${error.code || error.message}).`);
    } finally {
      toggle.disabled = false;
    }
  }

  async function submit(event) {
    event.preventDefault();
    const payload = {
      real_model: nodes.realModel.value.trim(),
      provider: nodes.provider.value,
      description: nodes.description.value.trim(),
    };
    if (currentMode === "create") {
      payload.name = nodes.name.value.trim();
    } else {
      payload.status = nodes.status.value;
    }

    try {
      if (currentMode === "create") {
        await api.createVirtualModel(payload);
      } else {
        await api.updateVirtualModel(nodes.name.value.trim(), payload);
      }
      closeModal();
      setAlert(nodes.alert, "loading", "虚拟模型已保存。");
      await load(true);
    } catch (error) {
      setAlert(nodes.alert, "error", `保存失败 (${error.code || error.message}).`);
    }
  }

  function openModal(model = null) {
    currentMode = model ? "edit" : "create";
    nodes.title.textContent = model ? "编辑虚拟模型" : "新建虚拟模型";
    nodes.name.value = model?.name || "";
    nodes.name.disabled = Boolean(model);
    nodes.realModel.value = model?.real_model || "";
    nodes.provider.value = model?.provider || "ark";
    nodes.status.value = model?.status || "active";
    nodes.status.closest("label").classList.toggle("is-hidden", !model);
    nodes.description.value = model?.description || "";
    nodes.modal?.classList.remove("is-hidden");
    (model ? nodes.realModel : nodes.name)?.focus();
  }

  function closeModal() {
    nodes.modal?.classList.add("is-hidden");
    nodes.form?.reset();
    nodes.name.disabled = false;
    nodes.status.closest("label").classList.add("is-hidden");
    currentMode = "create";
  }

  return { load };
}

window.OmniTokenViews = window.OmniTokenViews || {};
window.OmniTokenViews.createVirtualModelsView = createVirtualModelsView;
})();
