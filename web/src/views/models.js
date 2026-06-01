(function () {
const { cssVar, escapeHTML, formatNumber, formatTokens, formatUSD, setAlert, setEmptyOverlay } = window.OmniTokenUtils;

function createModelsView(api) {
  let models = [];
  let loaded = false;
  let chart = null;

  const nodes = {
    alert: document.getElementById("models-alert"),
    empty: document.getElementById("models-empty"),
    insight: document.getElementById("model-insight"),
    aggregate: document.getElementById("model-aggregate"),
    reload: document.querySelector('[data-action="reload-models"]'),
  };

  nodes.reload?.addEventListener("click", () => load(true));
  window.addEventListener?.("omnitoken:themechange", () => {
    if (chart) render();
  });

  function initChart() {
    if (chart || !window.Chart) return;
    const colors = chartColors();
    const context = document.getElementById("models-bar-chart").getContext("2d");
    chart = new Chart(context, {
      type: "bar",
      data: {
        labels: ["暂无数据"],
        datasets: [
          {
            label: "Prompt Tokens",
            data: [0],
            backgroundColor: colors.info,
            borderRadius: 4,
          },
          {
            label: "Completion Tokens",
            data: [0],
            backgroundColor: colors.primary,
            borderRadius: 4,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { position: "top" },
          tooltip: {
            callbacks: {
              label: (context) => `${context.dataset.label}: ${formatTokens(context.parsed.y || 0)}`,
            },
          },
        },
        scales: {
          y: {
            stacked: true,
            beginAtZero: true,
            ticks: { callback: (value) => formatTokens(value) },
            grid: { color: colors.borderSoft },
            border: { display: false },
          },
          x: {
            stacked: true,
            grid: { display: false },
            border: { display: false },
          },
        },
      },
    });
  }

  function render() {
    initChart();
    const rows = visibleRows();
    const colors = chartColors();
    if (chart) {
      chart.data.datasets[0].backgroundColor = colors.info;
      chart.data.datasets[1].backgroundColor = colors.primary;
      chart.options.scales.y.grid.color = colors.borderSoft;
      chart.data.labels = rows.length ? rows.map((model) => model.model) : ["暂无数据"];
      chart.data.datasets[0].data = rows.length ? rows.map((model) => model.prompt_tokens) : [0];
      chart.data.datasets[1].data = rows.length ? rows.map((model) => model.completion_tokens) : [0];
      chart.update();
    }

    setEmptyOverlay(nodes.empty, rows.length === 0);
    renderPanels(rows);
  }

  function renderPanels(rows) {
    if (!rows.length) {
      nodes.insight.textContent = "暂无模型用量数据。完成一次网关调用后会自动显示真实成本线索。";
      nodes.aggregate.innerHTML = '<div class="aggregate-row"><span>模型数</span><strong>0</strong></div>';
      return;
    }

    const totalTokens = rows.reduce((sum, row) => sum + row.total_tokens, 0);
    const promptTokens = rows.reduce((sum, row) => sum + row.prompt_tokens, 0);
    const completionTokens = rows.reduce((sum, row) => sum + row.completion_tokens, 0);
    const totalCost = rows.reduce((sum, row) => sum + row.cost_usd, 0);
    const totalCalls = rows.reduce((sum, row) => sum + row.call_count, 0);
    const providers = new Set(rows.map((row) => row.provider)).size;
    const topModel = rows[0];
    const topShare = totalTokens > 0 ? (topModel.total_tokens / totalTokens) * 100 : 0;
    const completionShare = totalTokens > 0 ? (completionTokens / totalTokens) * 100 : 0;

    nodes.insight.innerHTML = `
      <p><strong>${escapeHTML(topModel.model)}</strong> 是当前最高消耗模型，占本月模型 Token 的 ${topShare.toFixed(1)}%。</p>
      <p>输出 Token 占比 ${completionShare.toFixed(1)}%，可结合业务场景检查 max_tokens 与流式响应长度。</p>
    `;
    nodes.aggregate.innerHTML = `
      <div class="aggregate-row"><span>总调用次数</span><strong>${formatNumber(totalCalls)}</strong></div>
      <div class="aggregate-row"><span>Prompt / Completion</span><strong>${formatTokens(promptTokens)} / ${formatTokens(completionTokens)}</strong></div>
      <div class="aggregate-row"><span>预估成本</span><strong>${formatUSD(totalCost)}</strong></div>
      <div class="aggregate-row"><span>Provider 数</span><strong>${formatNumber(providers)}</strong></div>
    `;
  }

  async function load(force = false) {
    if (loaded && !force) return;
    initChart();
    setAlert(nodes.alert, "loading", "正在加载模型用量数据...");

    try {
      const payload = await api.getModels();
      models = normalizeModels(payload);
      loaded = true;
      const hasRows = visibleRows().length > 0;
      setAlert(nodes.alert, hasRows ? "" : "empty", hasRows ? "" : "暂无模型用量数据。完成一次网关调用后会自动显示 Prompt / Completion 拆分。");
      render();
    } catch (error) {
      setAlert(nodes.alert, "error", `无法加载模型用量 (${error.code || error.message})。请确认 admin 服务已启动，且 CORS 允许当前页面 origin。`);
      render();
    }
  }

  function visibleRows() {
    return [...models]
      .filter((model) => model.total_tokens > 0 || model.prompt_tokens > 0 || model.completion_tokens > 0 || model.cost_usd > 0 || model.call_count > 0)
      .sort((a, b) => b.total_tokens - a.total_tokens || a.model.localeCompare(b.model));
  }

  return { load };
}

function normalizeModels(raw) {
  const rows = raw && Array.isArray(raw.models) ? raw.models : [];
  return rows.map((model) => ({
    model: String(model.model || "unknown").trim() || "unknown",
    provider: String(model.provider || "unknown").trim() || "unknown",
    prompt_tokens: Math.max(0, Number(model.prompt_tokens) || 0),
    completion_tokens: Math.max(0, Number(model.completion_tokens) || 0),
    total_tokens: Math.max(0, Number(model.total_tokens) || 0),
    cost_usd: Math.max(0, Number(model.cost_usd) || 0),
    call_count: Math.max(0, Number(model.call_count) || 0),
  }));
}

function chartColors() {
  return {
    info: cssVar("--color-info"),
    primary: cssVar("--color-primary"),
    borderSoft: cssVar("--color-border-soft"),
  };
}

window.OmniTokenViews = { ...(window.OmniTokenViews || {}), createModelsView };
})();
