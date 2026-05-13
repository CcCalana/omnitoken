(function () {
function formatNumber(value) {
  const number = Number(value) || 0;
  return Math.round(number).toLocaleString("en-US");
}

function formatTokens(value) {
  const tokens = Number(value) || 0;
  if (tokens >= 1_000_000_000) return `${(tokens / 1_000_000_000).toFixed(2)} B`;
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(2)} M`;
  if (tokens >= 1_000) return `${(tokens / 1_000).toFixed(1)} K`;
  return formatNumber(tokens);
}

function formatUSD(value) {
  const amount = Number(value) || 0;
  const digits = amount > 0 && amount < 0.01 ? 4 : 2;
  return `$${amount.toLocaleString("en-US", {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })}`;
}

function formatPeriod(period) {
  const match = /^(\d{4})-(\d{2})$/.exec(period || "");
  if (!match) return period || "--";
  return `${match[1]}年${Number(match[2])}月`;
}

function formatTrendLabel(date) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date || "");
  if (!match) return date || "--";
  return `${Number(match[2])}月${Number(match[3])}日`;
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

function setAlert(element, type, message) {
  if (!element) return;
  element.className = "alert";
  if (!message) {
    element.classList.add("is-hidden");
    element.textContent = "";
    return;
  }
  element.classList.add(`alert-${type || "empty"}`);
  element.textContent = message;
}

function setEmptyOverlay(element, visible) {
  if (!element) return;
  element.classList.toggle("is-hidden", !visible);
}

window.OmniTokenUtils = {
  escapeHTML,
  formatNumber,
  formatPeriod,
  formatTokens,
  formatTrendLabel,
  formatUSD,
  setAlert,
  setEmptyOverlay,
};
})();
