(function () {
let active = null;

function openModal(options = {}) {
  closeActive();
  const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const backdrop = document.createElement("div");
  backdrop.className = "modal-backdrop";
  const titleID = `modal-title-${Date.now()}`;
  const panel = document.createElement("section");
  panel.className = "modal-panel";
  panel.setAttribute("role", "dialog");
  panel.setAttribute("aria-modal", "true");
  panel.setAttribute("aria-labelledby", titleID);

  const title = document.createElement("h2");
  title.id = titleID;
  title.textContent = String(options.title || "");
  const header = document.createElement("div");
  header.className = "panel-header";
  header.appendChild(title);
  panel.appendChild(header);

  const body = document.createElement("div");
  if (options.body instanceof Node) {
    body.appendChild(options.body);
  } else {
    body.innerHTML = String(options.body || "");
  }
  panel.appendChild(body);

  if (Array.isArray(options.actions) && options.actions.length) {
    const actions = document.createElement("div");
    actions.className = "modal-actions";
    options.actions.forEach((action) => {
      const button = document.createElement("button");
      button.type = "button";
      button.className = action.kind === "primary" ? "primary-button" : "secondary-button";
      button.textContent = action.label || "OK";
      button.addEventListener("click", () => {
        if (typeof action.onClick === "function") action.onClick();
        if (action.close !== false) close();
      });
      actions.appendChild(button);
    });
    panel.appendChild(actions);
  }

  backdrop.appendChild(panel);
  document.body.appendChild(backdrop);

  function close() {
    backdrop.remove();
    document.removeEventListener("keydown", onKeydown);
    active = null;
    if (typeof options.onClose === "function") options.onClose();
    previousFocus?.focus?.();
  }

  function onKeydown(event) {
    if (event.key === "Escape") {
      event.preventDefault();
      close();
      return;
    }
    if (event.key !== "Tab") return;
    trapFocus(panel, event);
  }

  backdrop.addEventListener("mousedown", (event) => {
    if (event.target === backdrop) close();
  });
  document.addEventListener("keydown", onKeydown);
  active = { close };
  focusFirst(panel);
  return { close };
}

function confirmModal({ title, message, confirmLabel = "确认", cancelLabel = "取消", onConfirm }) {
  return openModal({
    title,
    body: `<p class="modal-copy">${escapeHTML(message)}</p>`,
    actions: [
      { label: cancelLabel },
      { label: confirmLabel, kind: "primary", onClick: onConfirm },
    ],
  });
}

function closeActive() {
  active?.close?.();
}

function focusable(container) {
  return Array.from(container.querySelectorAll("a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex='-1'])"));
}

function focusFirst(container) {
  const first = focusable(container)[0];
  if (first) first.focus();
  else container.focus();
}

function trapFocus(container, event) {
  const nodes = focusable(container);
  if (!nodes.length) return;
  const first = nodes[0];
  const last = nodes[nodes.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[char]));
}

window.OmniTokenModal = { openModal, confirmModal };
})();
