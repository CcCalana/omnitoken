(function () {
const { createAdminAPI } = window.OmniTokenAPI;
const { createModelsView, createOverviewView, createUsersView } = window.OmniTokenViews;

const api = createAdminAPI();
const views = {
  overview: createOverviewView(api),
  users: createUsersView(api),
  models: createModelsView(api),
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
};

let activeTab = "overview";

document.getElementById("admin-origin").textContent = api.baseURL;
document.getElementById("refresh-button").addEventListener("click", () => {
  views[activeTab]?.load(true);
});

document.querySelectorAll("[data-tab]").forEach((button) => {
  button.addEventListener("click", () => switchTab(button.dataset.tab));
});

switchTab("overview");

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
