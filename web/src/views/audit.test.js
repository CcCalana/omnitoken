const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

test("audit view renders loading and empty states", async () => {
  const harness = newAuditHarness();
  let resolveLogs;
  const view = harness.createView({
    getAuditLogs: () => new Promise((resolve) => {
      resolveLogs = resolve;
    }),
  });

  const pending = view.load(true);
  assert.match(harness.element("audit-table-body").innerHTML, /Loading admin audit logs/);
  resolveLogs([]);
  await pending;

  assert.match(harness.element("audit-table-body").innerHTML, /No admin audit logs/);
  assert.equal(harness.element("audit-alert").textContent, "No admin audit logs.");
});

test("audit view renders error state", async () => {
  const harness = newAuditHarness();
  const view = harness.createView({
    getAuditLogs: async () => {
      const error = new Error("failed");
      error.code = "network_down";
      throw error;
    },
  });

  await view.load(true);

  assert.match(harness.element("audit-table-body").innerHTML, /Admin audit logs failed to load/);
  assert.match(harness.element("audit-alert").textContent, /network_down/);
});

test("audit usage view renders user model top and recent calls", async () => {
  const harness = newAuditHarness();
  const view = harness.createView({
    getAuditLogs: async () => [],
    getUsers: async () => ({
      users: [{
        user_id: "00000000-0000-0000-0000-000000000201",
        email: "viewer@democorp.local",
        display_name: "Viewer",
        used_tokens: 300,
      }],
    }),
    getUserUsage: async () => ({
      user_id: "00000000-0000-0000-0000-000000000201",
      period: { name: "current_month", since: "2026-05-01T00:00:00Z", until: "2026-06-01T00:00:00Z" },
      model_top: [{ model: "kimi-k2.6", tokens: 300, call_count: 3 }],
      hourly_distribution: Array.from({ length: 24 }, (_, hour) => hour === 10 ? 3 : 0),
      recent_calls: [{ created_at: "2026-05-11T10:00:00Z", model: "kimi-k2.6", status_code: 200, total_tokens: 120, streaming: true }],
    }),
  });

  await view.switchAuditView("usage");

  assert.match(harness.element("audit-usage-user").innerHTML, /Viewer/);
  assert.match(harness.element("audit-usage-model-body").innerHTML, /kimi-k2.6/);
  assert.match(harness.element("audit-usage-model-body").innerHTML, /100.0%/);
  assert.match(harness.element("audit-usage-recent-body").innerHTML, /Yes/);
});

function newAuditHarness() {
  const document = new FakeDocument();
  const context = {
    console,
    document,
    window: {},
  };
  context.window.document = document;
  context.window.window = context.window;
  vm.createContext(context);

  runBrowserScript(context, "web/src/utils.js");
  runBrowserScript(context, "web/src/views/audit.js");

  return {
    createView(api) {
      return context.window.OmniTokenViews.createAuditView(api);
    },
    element(id) {
      return document.getElementById(id);
    },
  };
}

function runBrowserScript(context, filename) {
  const source = fs.readFileSync(path.join(process.cwd(), filename), "utf8");
  vm.runInContext(source, context, { filename });
}

class FakeDocument {
  constructor() {
    this.elements = new Map();
    this.actions = new Map();
    this.tabs = [];
    for (const id of [
      "audit-alert",
      "audit-logs-panel",
      "audit-usage-panel",
      "audit-table-body",
      "audit-filter-actor",
      "audit-filter-resource-type",
      "audit-filter-since",
      "audit-filter-until",
      "audit-usage-alert",
      "audit-usage-user",
      "audit-usage-since",
      "audit-usage-until",
      "audit-usage-top-n",
      "audit-usage-model-body",
      "audit-usage-recent-body",
      "audit-usage-hourly-chart",
      "audit-usage-hourly-empty",
    ]) {
      this.elements.set(id, new FakeElement(id));
    }
    this.elements.get("audit-usage-top-n").value = "10";
    for (const action of ["reload-audit", "apply-audit-filters", "clear-audit-filters", "apply-audit-usage-filters", "clear-audit-usage-filters"]) {
      this.actions.set(action, new FakeElement(action));
    }
    this.tabs.push(new FakeElement("audit-tab-logs", { auditView: "logs" }));
    this.tabs.push(new FakeElement("audit-tab-usage", { auditView: "usage" }));
  }

  getElementById(id) {
    if (!this.elements.has(id)) {
      this.elements.set(id, new FakeElement(id));
    }
    return this.elements.get(id);
  }

  querySelector(selector) {
    const match = /^\[data-action="([^"]+)"\]$/.exec(selector);
    if (!match) return null;
    return this.actions.get(match[1]) || null;
  }

  querySelectorAll(selector) {
    if (selector === "[data-audit-view]") return this.tabs;
    return [];
  }
}

class FakeElement {
  constructor(id, dataset = {}) {
    this.id = id;
    this.dataset = dataset;
    this.value = "";
    this.innerHTML = "";
    this.textContent = "";
    this.className = "";
    this.listeners = new Map();
    this.classList = {
      add: (...tokens) => {
        const classes = new Set(this.className.split(/\s+/).filter(Boolean));
        for (const token of tokens) classes.add(token);
        this.className = [...classes].join(" ");
      },
      toggle: (token, force) => {
        const classes = new Set(this.className.split(/\s+/).filter(Boolean));
        if (force) {
          classes.add(token);
        } else {
          classes.delete(token);
        }
        this.className = [...classes].join(" ");
      },
    };
  }

  addEventListener(type, handler) {
    this.listeners.set(type, handler);
  }
}
