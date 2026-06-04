(function () {
const {
  cssVar,
  escapeHTML,
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
  let trendMode = "tokens";
  let overview = normalizeOverview(null);

  const nodes = {
    alert: document.getElementById("overview-alert"),
    period: document.getElementById("current-period"),
    ledgerStatus: document.getElementById("overview-ledger-status"),
    costCaption: document.getElementById("overview-cost-caption"),
    totalTokens: document.getElementById("overview-total-tokens"),
    estimatedCost: document.getElementById("overview-estimated-cost"),
    averageDailyTokens: document.getElementById("overview-average-daily-tokens"),
    peakDay: document.getElementById("overview-peak-day"),
    activeUsers: document.getElementById("overview-active-users"),
    quotaWarningUsers: document.getElementById("overview-quota-warning-users"),
    topModel: document.getElementById("overview-top-model"),
    topModelCaption: document.getElementById("overview-top-model-caption"),
    trendSummary: document.getElementById("overview-trend-summary"),
    trendEmpty: document.getElementById("trend-empty"),
    shareEmpty: document.getElementById("model-share-empty"),
    actionList: document.getElementById("overview-action-list"),
    modelRanking: document.getElementById("overview-model-ranking"),
    trendModeButtons: document.querySelectorAll("[data-overview-trend-mode]"),
  };

  nodes.trendModeButtons.forEach((button) => {
    button.addEventListener("click", () => {
      trendMode = button.dataset.overviewTrendMode === "cost" ? "cost" : "tokens";
      syncTrendModeControls();
      renderCharts();
      renderTrendSummary(deriveStats(overview));
    });
  });

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
          label: "每日 Token",
          data: [],
          borderColor: colors.primary,
          backgroundColor: colors.primarySoft,
          borderWidth: 2,
          pointBackgroundColor: colors.surface,
          pointBorderColor: colors.primary,
          pointBorderWidth: 2,
          pointRadius: 3,
          fill: true,
          tension: 0.32,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: { intersect: false, mode: "index" },
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              label: trendTooltipLabel,
              afterLabel: trendTooltipAfterLabel,
            },
          },
        },
        scales: {
          y: {
            beginAtZero: true,
            ticks: { callback: trendTickLabel, color: colors.muted },
            grid: { color: colors.borderSoft },
            border: { display: false },
          },
          x: {
            ticks: { color: colors.muted, maxRotation: 0 },
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
        cutout: "72%",
        plugins: {
          legend: {
            position: "bottom",
            labels: {
              color: colors.muted,
              padding: 14,
              usePointStyle: true,
              boxWidth: 8,
            },
          },
          tooltip: {
            callbacks: {
              label: modelShareTooltipLabel,
            },
          },
        },
      },
    });
  }

  function render(data) {
    overview = normalizeOverview(data);
    const stats = deriveStats(overview);

    nodes.period.textContent = formatPeriod(overview.period);
    nodes.totalTokens.textContent = formatTokens(overview.total_tokens);
    nodes.estimatedCost.textContent = formatUSD(overview.estimated_cost_usd);
    nodes.averageDailyTokens.textContent = formatTokens(stats.averageDailyTokens);
    nodes.peakDay.textContent = stats.peak
      ? `${formatTrendLabel(stats.peak.date)} · ${formatTokens(stats.peak.tokens)}`
      : "--";
    nodes.activeUsers.textContent = formatNumber(overview.active_users);
    nodes.quotaWarningUsers.textContent = formatNumber(overview.quota_warning_users);
    nodes.costCaption.textContent = `${formatPeriod(overview.period)}，按成本账本聚合。`;
    nodes.ledgerStatus.textContent = stats.hasAnyUsage ? "账本已同步" : "等待账本";
    nodes.ledgerStatus.className = stats.hasAnyUsage
      ? "health-chip health-chip-ok"
      : "health-chip health-chip-neutral";

    if (stats.topModel) {
      nodes.topModel.textContent = stats.topModel.model;
      nodes.topModelCaption.textContent = `${formatPercent(stats.topModelShare)} · ${formatUSD(stats.topModel.cost_usd)}`;
    } else {
      nodes.topModel.textContent = "--";
      nodes.topModelCaption.textContent = "暂无模型数据";
    }

    renderTrendSummary(stats);
    renderActionList(stats);
    renderModelRanking(stats);

    initCharts();
    renderCharts();

    if (!stats.hasAnyUsage) {
      setAlert(nodes.alert, "empty", "暂无用量数据。完成一次网关调用并写入账本后，这里会自动显示组织消耗。");
    } else {
      setAlert(nodes.alert, "", "");
    }
  }

  function renderTrendSummary(stats) {
    if (!nodes.trendSummary) return;
    if (!stats.trendRows.length) {
      nodes.trendSummary.textContent = "等待账本数据";
      return;
    }
    const latest = stats.latest;
    const trailing = trendMode === "cost"
      ? `近 7 日 ${formatUSD(stats.lastSevenCost)}`
      : `近 7 日 ${formatTokens(stats.lastSevenTokens)}`;
    const latestValue = trendMode === "cost"
      ? formatUSD(latest.cost_usd)
      : formatTokens(latest.tokens);
    nodes.trendSummary.textContent = `${trailing}，最新 ${formatTrendLabel(latest.date)} 为 ${latestValue}`;
  }

  function renderActionList(stats) {
    if (!nodes.actionList) return;
    const items = [];

    if (!stats.hasAnyUsage) {
      items.push({
        tone: "neutral",
        title: "等待首批调用",
        body: "账本写入后会显示成本、模型和额度风险。",
      });
    } else {
      if (overview.quota_warning_users > 0) {
        items.push({
          tone: "warning",
          title: `${formatNumber(overview.quota_warning_users)} 个用户触发额度预警`,
          body: "优先查看用户额度分配，确认是否需要调高预算或限制 Key。",
        });
      }

      if (overview.total_tokens > 0 && overview.estimated_cost_usd === 0) {
        items.push({
          tone: "warning",
          title: "Token 已产生，但成本仍为 0",
          body: "检查模型定价或 cost_ledger 写入，避免账单被低估。",
        });
      }

      if (stats.topModel && stats.topModelShare >= 60) {
        items.push({
          tone: "info",
          title: `${stats.topModel.model} 占 ${formatPercent(stats.topModelShare)}`,
          body: "模型集中度偏高，适合检查路由策略和默认模型配置。",
        });
      }

      if (stats.spikeRatio >= 2 && stats.peak) {
        items.push({
          tone: "warning",
          title: `${formatTrendLabel(stats.peak.date)} 出现峰值`,
          body: `当日 Token 约为日均 ${stats.spikeRatio.toFixed(1)} 倍。`,
        });
      }

      if (!items.length) {
        items.push({
          tone: "ok",
          title: "当前消耗结构稳定",
          body: "未发现明显额度风险、成本缺口或模型集中异常。",
        });
      }
    }

    nodes.actionList.innerHTML = items.slice(0, 4).map((item) => `
      <li class="attention-item attention-${item.tone}">
        <strong>${escapeHTML(item.title)}</strong>
        <span>${escapeHTML(item.body)}</span>
      </li>
    `).join("");
  }

  function renderModelRanking(stats) {
    if (!nodes.modelRanking) return;
    const rows = stats.rankingRows.slice(0, 6);
    if (!rows.length) {
      nodes.modelRanking.innerHTML = '<div class="table-state">暂无模型用量数据</div>';
      return;
    }

    const maxValue = Math.max(...rows.map((row) => stats.rankByCost ? row.cost_usd : row.tokens), 1);
    nodes.modelRanking.innerHTML = rows.map((row, index) => {
      const value = stats.rankByCost ? row.cost_usd : row.tokens;
      const width = Math.max(4, Math.min(100, (value / maxValue) * 100));
      const share = overview.total_tokens > 0 ? (row.tokens / overview.total_tokens) * 100 : row.share * 100;
      const primaryValue = stats.rankByCost ? formatUSD(row.cost_usd) : formatTokens(row.tokens);
      const secondaryValue = stats.rankByCost
        ? `${formatTokens(row.tokens)} · ${formatPercent(share)}`
        : `${formatUSD(row.cost_usd)} · ${formatPercent(share)}`;
      return `
        <div class="model-rank-row">
          <div class="model-rank-index">${index + 1}</div>
          <div class="model-rank-main">
            <div class="model-rank-label">
              <strong title="${escapeHTML(row.model)}">${escapeHTML(row.model)}</strong>
              <span>${escapeHTML(secondaryValue)}</span>
            </div>
            <div class="model-rank-track" aria-hidden="true">
              <div class="model-rank-fill" style="width: ${width}%"></div>
            </div>
          </div>
          <div class="model-rank-value">${escapeHTML(primaryValue)}</div>
        </div>
      `;
    }).join("");
  }

  function renderCharts() {
    const trendRows = visibleTrendRows(overview.trend);
    const colors = chartColors();
    if (trendChart) {
      const isCost = trendMode === "cost";
      trendChart.data.datasets[0].label = isCost ? "每日成本" : "每日 Token";
      trendChart.data.datasets[0].borderColor = isCost ? colors.warning : colors.primary;
      trendChart.data.datasets[0].backgroundColor = isCost ? colors.warningSoft : colors.primarySoft;
      trendChart.data.datasets[0].pointBackgroundColor = colors.surface;
      trendChart.data.datasets[0].pointBorderColor = isCost ? colors.warning : colors.primary;
      trendChart.options.scales.y.grid.color = colors.borderSoft;
      trendChart.options.scales.y.ticks.color = colors.muted;
      trendChart.options.scales.x.ticks.color = colors.muted;
      trendChart.data.labels = trendRows.map((item) => formatTrendLabel(item.date));
      trendChart.data.datasets[0].data = trendRows.map((item) => isCost ? item.cost_usd : item.tokens);
      trendChart.update();
    }
    setEmptyOverlay(nodes.trendEmpty, trendRows.length === 0);

    const rows = modelShareRows();
    if (shareChart) {
      shareChart.options.plugins.legend.labels.color = colors.muted;
      shareChart.data.labels = rows.length ? rows.map((item) => item.model) : ["暂无数据"];
      shareChart.data.datasets[0].data = rows.length ? rows.map((item) => item.tokens) : [1];
      shareChart.data.datasets[0].backgroundColor = rows.length
        ? [colors.primary, colors.info, colors.success, colors.warning, colors.primaryStrong, colors.danger]
        : [colors.border];
      shareChart.update();
    }
    setEmptyOverlay(nodes.shareEmpty, rows.length === 0);
  }

  function trendTooltipLabel(context) {
    const row = visibleTrendRows(overview.trend)[context.dataIndex];
    if (!row) return "暂无数据";
    return trendMode === "cost"
      ? `成本: ${formatUSD(row.cost_usd)}`
      : `Token: ${formatTokens(row.tokens)}`;
  }

  function trendTooltipAfterLabel(context) {
    const row = visibleTrendRows(overview.trend)[context.dataIndex];
    if (!row) return "";
    return trendMode === "cost"
      ? `Token: ${formatTokens(row.tokens)}`
      : `成本: ${formatUSD(row.cost_usd)}`;
  }

  function modelShareTooltipLabel(context) {
    const row = modelShareRows()[context.dataIndex];
    if (!row) return "暂无数据";
    const percent = overview.total_tokens > 0 ? (Number(row.tokens || 0) / overview.total_tokens) * 100 : 0;
    return `${row.model}: ${formatPercent(percent)} (${formatTokens(row.tokens)})`;
  }

  function trendTickLabel(value) {
    return trendMode === "cost" ? formatUSD(value) : formatTokens(value);
  }

  function syncTrendModeControls() {
    nodes.trendModeButtons.forEach((button) => {
      const isActive = button.dataset.overviewTrendMode === trendMode;
      button.classList.toggle("is-active", isActive);
      button.setAttribute("aria-pressed", String(isActive));
    });
  }

  function modelShareRows() {
    const rows = visibleModelRows(overview.model_usage);
    const head = rows.slice(0, 5);
    const rest = rows.slice(5);
    if (!rest.length) return head;
    const other = rest.reduce((acc, row) => ({
      model: "其他",
      tokens: acc.tokens + row.tokens,
      cost_usd: acc.cost_usd + row.cost_usd,
      share: acc.share + row.share,
    }), { model: "其他", tokens: 0, cost_usd: 0, share: 0 });
    return [...head, other].filter((item) => item.tokens > 0 || item.cost_usd > 0);
  }

  async function load() {
    setAlert(nodes.alert, "loading", "正在加载组织消耗数据...");
    try {
      render(await api.getOverview());
    } catch (error) {
      render(overview);
      setAlert(nodes.alert, "error", `无法加载 admin overview (${error.code || error.message})。请确认 admin 服务已启动，且 CORS 允许当前页面 origin。`);
    }
  }

  return { load, render };
}

function deriveStats(source) {
  const trendRows = visibleTrendRows(source.trend);
  const modelRows = visibleModelRows(source.model_usage);
  const totalTrendTokens = trendRows.reduce((sum, row) => sum + row.tokens, 0);
  const averageDailyTokens = trendRows.length ? totalTrendTokens / trendRows.length : 0;
  const peak = trendRows.reduce((best, row) => !best || row.tokens > best.tokens ? row : best, null);
  const latest = trendRows[trendRows.length - 1] || { date: "", tokens: 0, cost_usd: 0 };
  const lastSevenRows = trendRows.slice(-7);
  const lastSevenTokens = lastSevenRows.reduce((sum, row) => sum + row.tokens, 0);
  const lastSevenCost = lastSevenRows.reduce((sum, row) => sum + row.cost_usd, 0);
  const topModel = modelRows[0] || null;
  const topModelShare = topModel
    ? source.total_tokens > 0 ? (topModel.tokens / source.total_tokens) * 100 : topModel.share * 100
    : 0;
  const rankByCost = modelRows.some((row) => row.cost_usd > 0);
  const rankingRows = [...modelRows].sort((a, b) => {
    if (rankByCost) return b.cost_usd - a.cost_usd || b.tokens - a.tokens || a.model.localeCompare(b.model);
    return b.tokens - a.tokens || a.model.localeCompare(b.model);
  });
  const spikeRatio = peak && averageDailyTokens > 0 ? peak.tokens / averageDailyTokens : 0;
  const hasAnyUsage = source.total_tokens > 0 || source.estimated_cost_usd > 0 || trendRows.length > 0 || modelRows.length > 0;

  return {
    trendRows,
    modelRows,
    rankingRows,
    rankByCost,
    averageDailyTokens,
    peak,
    latest,
    lastSevenTokens,
    lastSevenCost,
    topModel,
    topModelShare,
    spikeRatio,
    hasAnyUsage,
  };
}

function visibleTrendRows(rows) {
  return [...(Array.isArray(rows) ? rows : [])]
    .filter((item) => Number(item.tokens) > 0 || Number(item.cost_usd) > 0)
    .sort((a, b) => String(a.date).localeCompare(String(b.date)));
}

function visibleModelRows(rows) {
  return [...(Array.isArray(rows) ? rows : [])]
    .filter((item) => Number(item.tokens) > 0 || Number(item.cost_usd) > 0)
    .sort((a, b) => b.tokens - a.tokens || b.cost_usd - a.cost_usd || a.model.localeCompare(b.model));
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

function formatPercent(value) {
  const percent = Number(value) || 0;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function chartColors() {
  return {
    primary: cssVar("--color-primary"),
    primaryStrong: cssVar("--color-primary-strong"),
    primarySoft: cssVar("--color-primary-soft"),
    info: cssVar("--color-info"),
    success: cssVar("--color-success"),
    warning: cssVar("--color-warning"),
    warningSoft: cssVar("--color-warning-soft"),
    danger: cssVar("--color-danger"),
    muted: cssVar("--color-muted"),
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
