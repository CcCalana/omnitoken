(function () {
const { createAdminAPI } = window.OmniTokenAPI;
const { createAuditView, createModelsView, createOverviewView, createUsersView, createVirtualModelsView, createLoginView } = window.OmniTokenViews;

const api = createAdminAPI();
const state = {
  role: "viewer",
};
const views = {
  overview: createOverviewView(api),
  users: createUsersView(api, { getRole: () => state.role }),
  models: createModelsView(api),
  virtualModels: createVirtualModelsView ? createVirtualModelsView(api) : null,
  audit: createAuditView(api),
  login: createLoginView ? createLoginView(api) : null,
};

const titles = {
  overview: {
    title: "Organization Overview",
    subtitle: "Live gateway requests, token consumption, and cost ledger signals.",
  },
  users: {
    title: "Users And Budgets",
    subtitle: "Review user usage and manage monthly budgets when your role allows it.",
  },
  models: {
    title: "Model Usage",
    subtitle: "Tokens, cost, and call counts grouped by actual upstream model.",
  },
  virtualModels: {
    title: "Virtual Models",
    subtitle: "Gateway routing aliases mapped to real Ark model identifiers.",
  },
  audit: {
    title: "Audit Logs",
    subtitle: "Trace admin actions by actor, resource, status, and time window.",
  },
  login: {
    title: "Sign In",
    subtitle: "Use a seeded local admin or viewer account.",
  },
};

let activeTab = "overview";

document.getElementById("admin-origin").textContent = api.baseURL;
document.getElementById("refresh-button").addEventListener("click", () => {
  views[activeTab]?.load(true);
});

document.querySelectorAll("[data-tab]").forEach((button) => {
  button.addEventListener("click", () => switchTab(button.dataset.tab));
});

const logoutBtn = document.getElementById("logout-button");
if (logoutBtn) {
  logoutBtn.addEventListener("click", () => api.logout());
}

window.addEventListener("omnitoken:unauthorized", () => {
  showAuthenticatedShell(false);
  switchTab("login");
});

init();

async function init() {
  const urlParams = new URLSearchParams(window.location.search);
  if (!localStorage.getItem("omnitokenAdminToken") && !urlParams.get("token")) {
    window.dispatchEvent(new Event("omnitoken:unauthorized"));
    return;
  }

  try {
    const me = await api.getMe();
    state.role = me.role || "viewer";
    showAuthenticatedShell(true);
    switchTab("overview");
  } catch (_) {
    window.dispatchEvent(new Event("omnitoken:unauthorized"));
  }
}

function showAuthenticatedShell(isAuthenticated) {
  const sidebar = document.querySelector(".sidebar");
  const actions = document.querySelector(".topbar-actions");
  if (sidebar) sidebar.style.display = isAuthenticated ? "" : "none";
  if (actions) actions.style.display = isAuthenticated ? "" : "none";
}

function switchTab(tab) {
  if (!views[tab]) return;
  activeTab = tab;

  document.querySelectorAll("[data-tab]").forEach((button) => {
    button.classList.toggle("is-active", button.dataset.tab === tab);
  });

  document.querySelectorAll(".view").forEach((view) => {
    view.classList.toggle("is-active", view.id === `view-${tab}`);
  });

  const copy = titles[tab];
  document.getElementById("page-title").textContent = copy.title;
  document.getElementById("page-subtitle").textContent = copy.subtitle;
  views[tab].load();
}
})();
