(function () {
const { createAdminAPI } = window.OmniTokenAPI;
const { createAuditView, createCredentialsView, createModelsView, createOverviewView, createUsersView, createVirtualModelsView, createLoginView } = window.OmniTokenViews;

const api = createAdminAPI();
const state = {
  role: "viewer",
  themeMode: localStorage.getItem("omnitoken.theme") || "system",
};
const views = {
  overview: createOverviewView(api),
  users: createUsersView(api, { getRole: () => state.role }),
  models: createModelsView(api),
  virtualModels: createVirtualModelsView ? createVirtualModelsView(api) : null,
  credentials: createCredentialsView ? createCredentialsView(api) : null,
  audit: createAuditView(api),
  login: createLoginView ? createLoginView(api) : null,
};

const titles = {
  overview: {
    title: "组织消耗概览",
    subtitle: "查看本月成本、Token 趋势、模型消耗和额度风险。",
  },
  users: {
    title: "用户额度分配",
    subtitle: "查看用户消耗，并在权限允许时管理月度预算。",
  },
  models: {
    title: "模型调用分析",
    subtitle: "按真实上游模型查看 Token、成本和调用次数。",
  },
  virtualModels: {
    title: "虚拟模型映射",
    subtitle: "管理网关路由别名与真实模型标识的映射关系。",
  },
  credentials: {
    title: "上游凭据管理",
    subtitle: "新增或停用 provider key，网关会按轮询周期重载凭据池。",
  },
  audit: {
    title: "审计日志",
    subtitle: "按操作者、资源、状态和时间窗口追踪管理动作。",
  },
  login: {
    title: "管理员登录",
    subtitle: "使用本地初始化的 admin 或 viewer 账号。",
  },
};

let activeTab = "overview";
const themeQuery = window.matchMedia?.("(prefers-color-scheme: dark)");

document.getElementById("admin-origin").textContent = api.baseURL;
document.getElementById("refresh-button").addEventListener("click", () => {
  views[activeTab]?.load(true);
});
document.getElementById("theme-toggle-button")?.addEventListener("click", cycleTheme);
themeQuery?.addEventListener?.("change", () => {
  if (state.themeMode === "system") applyTheme();
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
  applyTheme();
  const urlParams = new URLSearchParams(window.location.search);
  if (!api.isDemo && !localStorage.getItem("omnitokenAdminToken") && !urlParams.get("token")) {
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

function cycleTheme() {
  const order = ["system", "light", "dark"];
  const current = order.includes(state.themeMode) ? state.themeMode : "system";
  state.themeMode = order[(order.indexOf(current) + 1) % order.length];
  localStorage.setItem("omnitoken.theme", state.themeMode);
  applyTheme();
}

function applyTheme() {
  const systemDark = Boolean(themeQuery?.matches);
  const resolved = state.themeMode === "dark" || (state.themeMode === "system" && systemDark) ? "dark" : "light";
  const previous = document.documentElement.dataset.theme;
  document.documentElement.dataset.theme = resolved;
  document.documentElement.dataset.themeMode = state.themeMode;
  const label = document.getElementById("theme-toggle-label");
  const icon = document.getElementById("theme-toggle-icon");
  if (label) label.textContent = state.themeMode === "system" ? "System" : state.themeMode === "dark" ? "Dark" : "Light";
  if (icon) icon.textContent = state.themeMode === "system" ? "◐" : state.themeMode === "dark" ? "●" : "○";
  if (previous && previous !== resolved) window.dispatchEvent(new Event("omnitoken:themechange"));
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
