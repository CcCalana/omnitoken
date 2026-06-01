(function () {
const {
  cssVar,
  formatNumber,
  formatPeriod,
  formatTokens,
  formatTrendLabel,
  formatUSD,
  setAlert,
  setEmptyOverlay,
} = window.OmniTokenUtils;

function createOverviewView(api) {
  let trendChart = null;
  let shareChart = null;
  let overview = normalizeOverview(null);

  const nodes = {
    alert: document.getElementById("overview-alert"),
    period: document.getElementById("current-period"),
    totalTokens: document.getElementById("overview-total-tokens"),
    estimatedCost: document.getElementById("overview-estimated-cost"),
    activeUsers: document.getElementById("overview-active-users"),
    quotaWarningUsers: document.getElementById("overview-quota-warning-users"),
    trendEmpty: document.getElementById("trend-empty"),
    shareEmpty: document.getElementById("model-share-empty"),
  };
  window.addEventListener?.("omnitoken:themechange", () => {
    if (trendChart || shareChart) renderCharts();
  });

  function initCharts() {
    if (trendChart || !window.Chart) return;

    const colors = chartColors();
    const trendContext = document.getElementById("trend-chart").getContext("2d");
    trendChart = new Chart(trendContext, {
      type: "line",
      data: {
        labels: [],
        datasets: [{
          label: "每日总消耗",
          data: [],
          borderColor: colors.primary,
          backgroundColor: colors.primarySoft,
          borderWidth: 2,
          pointBackgroundColor: colors.surface,
          pointBorderColor: colors.primary,
          pointBorderWidth: 2,
          pointRadius: 4,
          fill: true,
          tension: 0.35,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              label: (context) => `Token: ${formatNumber(context.parsed.y || 0)}`,
            },
          },
        },
        scales: {
          y: {
            beginAtZero: true,
            ticks: { callback: (value) => formatTokens(value) },
            grid: { color: colors.borderSoft },
            border: { display: false },
          },
          x: {
            grid: { display: false },
            border: { display: false },
          },
        },
      },
    });

    const shareContext = document.getElementById("model-share-chart").getContext("2d");
    shareChart = new Chart(shareContext, {
      type: "doughnut",
      data: {
        labels: ["暂无数据"],
        datasets: [{
          data: [1],
          backgroundColor: [colors.border],
          borderWidth: 0,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        cutout: "70%",
        plugins: {
          legend: {
            position: "bottom",
            labels: { padding: 18, usePointStyle: true, boxWidth: 8 },
          },
          tooltip: {
            callbacks: {
              label: (context) => {
                const row = visibleModelRows()[context.dataIndex];
                if (!row) return "暂无数据";
                const percent = overview.total_tokens > 0 ? (Number(row.tokens || 0) / overview.total_tokens) * 100 : 0;
                return `${row.model || "unknown"}: ${percent.toFixed(1)}% (${formatTokens(row.tokens)} Tokens)`;
              },
            },
          },
        },
      },
    });
  }

  function render(data) {
    overview = normalizeOverview(data);
    nodes.period.textContent = formatPeriod(overview.period);
    nodes.totalTokens.textContent = formatTokens(overview.total_tokens);
    nodes.estimatedCost.textContent = formatUSD(overview.estimated_cost_usd);
    nodes.activeUsers.textContent = formatNumber(overview.active_users);
    nodes.quotaWarningUsers.textContent = formatNumber(overview.quota_warning_users);

    initCharts();
    renderCharts();

    if (overview.total_tokens === 0 && overview.trend.length === 0 && overview.model_usage.length === 0) {
      setAlert(nodes.alert, "empty", "暂无用量数据。完成一次网关调用并写入账本后，这里会自动显示实时统计。");
    } else {
      setAlert(nodes.alert, "", "");
    }
  }

  function renderCharts() {
    const trendRows = overview.trend.filter((item) => Number(item.tokens) > 0 || Number(item.cost_usd) > 0);
    const colors = chartColors();
    if (trendChart) {
      trendChart.data.datasets[0].borderColor = colors.primary;
      trendChart.data.datasets[0].backgroundColor = colors.primarySoft;
      trendChart.data.datasets[0].pointBackgroundColor = colors.surface;
      trendChart.data.datasets[0].pointBorderColor = colors.primary;
      trendChart.options.scales.y.grid.color = colors.borderSoft;
      trendChart.data.labels = trendRows.map((item) => formatTrendLabel(item.date));
      trendChart.data.datasets[0].data = trendRows.map((item) => Number(item.tokens) || 0);
      trendChart.update();
    }
    setEmptyOverlay(nodes.trendEmpty, trendRows.length === 0);

    const rows = visibleModelRows();
    if (shareChart) {
      shareChart.data.labels = rows.length ? rows.map((item) => item.model || "unknown") : ["暂无数据"];
      shareChart.data.datasets[0].data = rows.length ? rows.map((item) => Number(item.tokens) || 0) : [1];
      shareChart.data.datasets[0].backgroundColor = rows.length
        ? [colors.primary, colors.info, colors.success, colors.warning, colors.primaryStrong, colors.danger]
        : [colors.border];
      shareChart.update();
    }
    setEmptyOverlay(nodes.shareEmpty, rows.length === 0);
  }

  function visibleModelRows() {
    return overview.model_usage.filter((item) => Number(item.tokens) > 0 || Number(item.cost_usd) > 0);
  }

  async function load() {
    setAlert(nodes.alert, "loading", "正在加载实时用量数据...");
    try {
      render(await api.getOverview());
    } catch (error) {
      render(overview);
      setAlert(nodes.alert, "error", `无法加载 admin overview (${error.code || error.message})。请确认 admin 服务已启动，且 CORS 允许当前页面 origin。`);
    }
  }

  return { load, render };
}

function normalizeOverview(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  return {
    period: typeof source.period === "string" ? source.period : "--",
    total_tokens: Math.max(0, Number(source.total_tokens) || 0),
    estimated_cost_usd: Math.max(0, Number(source.estimated_cost_usd) || 0),
    active_users: Math.max(0, Number(source.active_users) || 0),
    quota_warning_users: Math.max(0, Number(source.quota_warning_users) || 0),
    trend: normalizeTrend(source.trend),
    model_usage: normalizeModelUsage(source.model_usage),
  };
}

function normalizeTrend(raw) {
  const rows = Array.isArray(raw) ? raw : [];
  return rows.map((item) => ({
    date: String(item.date || "").trim(),
    tokens: Math.max(0, Number(item.tokens ?? item.total_tokens) || 0),
    cost_usd: Math.max(0, Number(item.cost_usd) || 0),
  }));
}

function normalizeModelUsage(raw) {
  const rows = Array.isArray(raw) ? raw : [];
  return rows.map((item) => ({
    model: String(item.model || "unknown").trim() || "unknown",
    tokens: Math.max(0, Number(item.tokens ?? item.total_tokens) || 0),
    cost_usd: Math.max(0, Number(item.cost_usd) || 0),
    share: Math.max(0, Number(item.share) || 0),
  }));
}

function chartColors() {
  return {
    primary: cssVar("--color-primary"),
    primaryStrong: cssVar("--color-primary-strong"),
    primarySoft: cssVar("--color-primary-soft"),
    info: cssVar("--color-info"),
    success: cssVar("--color-success"),
    warning: cssVar("--color-warning"),
    danger: cssVar("--color-danger"),
    surface: cssVar("--color-surface"),
    border: cssVar("--color-border"),
    borderSoft: cssVar("--color-border-soft"),
  };
}

window.OmniTokenViews = {
  ...(window.OmniTokenViews || {}),
  createOverviewView,
};
})();
