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
  assert.match(harness.element("audit-table-body").innerHTML, /正在加载审计日志/);
  resolveLogs([]);
  await pending;

  assert.match(harness.element("audit-table-body").innerHTML, /暂无审计日志/);
  assert.equal(harness.element("audit-alert").textContent, "暂无审计日志。");
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

  assert.match(harness.element("audit-table-body").innerHTML, /审计日志加载失败/);
  assert.match(harness.element("audit-alert").textContent, /network_down/);
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
    for (const id of [
      "audit-alert",
      "audit-table-body",
      "audit-filter-actor",
      "audit-filter-resource-type",
      "audit-filter-since",
      "audit-filter-until",
    ]) {
      this.elements.set(id, new FakeElement(id));
    }
    for (const action of ["reload-audit", "apply-audit-filters", "clear-audit-filters"]) {
      this.actions.set(action, new FakeElement(action));
    }
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
}

class FakeElement {
  constructor(id) {
    this.id = id;
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
