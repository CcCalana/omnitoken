(function () {
const { createAdminAPI } = window.OmniTokenAPI;
const { createAuditView, createModelsView, createOverviewView, createUsersView, createVirtualModelsView, createLoginView } = window.OmniTokenViews;

const api = createAdminAPI();
const views = {
  overview: createOverviewView(api),
  users: createUsersView(api),
  models: createModelsView(api),
  virtualModels: createVirtualModelsView ? createVirtualModelsView(api) : null,
  audit: createAuditView(api),
  login: createLoginView ? createLoginView(api) : null,
};

const titles = {
  overview: {
    title: "组织消耗概览",
    subtitle: "实时查看网关请求、Token 消耗和成本账本。",
  },
  users: {
    title: "用户额度分配与管控",
    subtitle: "展示组织成员本月模型使用情况；配额系统未接入时显示无限额。",
  },
  models: {
    title: "模型调用分析",
    subtitle: "按模型聚合 Prompt、Completion、成本和调用次数。",
  },
  virtualModels: {
    title: "虚拟模型映射",
    subtitle: "网关层的虚拟模型与真实 Ark 模型映射表。",
  },
  audit: {
    title: "审计日志",
    subtitle: "按 actor、资源与时间窗口查看 admin 写操作记录。",
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

window.addEventListener('omnitoken:unauthorized', () => {
  switchTab("login");
  document.querySelector('.sidebar').style.display = 'none';
  document.querySelector('.topbar-actions').style.display = 'none';
  document.getElementById("page-title").textContent = "访问受限";
  document.getElementById("page-subtitle").textContent = "请登录以继续。";
});

// Check if we already have a token or ?token= in URL, otherwise show login
const urlParams = new URLSearchParams(window.location.search);
if (!localStorage.getItem("omnitokenAdminToken") && !urlParams.get("token")) {
  window.dispatchEvent(new Event('omnitoken:unauthorized'));
} else {
  switchTab("overview");
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
