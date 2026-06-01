(function () {
const MAX_TOASTS = 3;
const DEFAULT_TIMEOUT_MS = 4000;
let region = null;
const toasts = [];

function ensureRegion() {
  if (region) return region;
  region = document.createElement("div");
  region.className = "toast-region";
  region.setAttribute("aria-live", "polite");
  region.setAttribute("aria-relevant", "additions");
  document.body.appendChild(region);
  return region;
}

function showToast(message, kind = "info") {
  const text = String(message || "").trim();
  if (!text) return null;

  const item = document.createElement("div");
  const normalized = ["info", "success", "warning", "danger"].includes(kind) ? kind : "info";
  item.className = `toast toast-${normalized}`;
  item.setAttribute("role", normalized === "danger" ? "alert" : "status");
  item.textContent = text;
  ensureRegion().appendChild(item);

  const state = { item, timer: null, remaining: DEFAULT_TIMEOUT_MS, started: Date.now() };
  state.start = () => {
    state.started = Date.now();
    state.timer = window.setTimeout(() => dismiss(state), state.remaining);
  };
  state.pause = () => {
    window.clearTimeout(state.timer);
    state.remaining = Math.max(600, state.remaining - (Date.now() - state.started));
  };
  item.addEventListener("mouseenter", state.pause);
  item.addEventListener("mouseleave", state.start);
  toasts.push(state);
  while (toasts.length > MAX_TOASTS) dismiss(toasts[0]);
  state.start();
  return () => dismiss(state);
}

function dismiss(state) {
  const index = toasts.indexOf(state);
  if (index >= 0) toasts.splice(index, 1);
  window.clearTimeout(state.timer);
  state.item.remove();
}

window.OmniTokenToast = { showToast };
})();
