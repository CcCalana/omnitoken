const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

test("users view shows quota editing for admin role", async () => {
  const harness = newUsersHarness();
  const view = harness.createView({
    getUsers: async () => ({
      users: [{
        user_id: "user-1",
        organization_id: "org-1",
        email: "admin@democorp.local",
        display_name: "Demo Admin",
        used_tokens: 42,
        used_budget_cents: 38,
        budget_cents: 100,
        status: "active",
      }],
    }),
  }, { role: "admin" });

  await view.load(true);

  const html = harness.element("users-table-body").innerHTML;
  assert.match(html, /data-action="generate-key"/);
  assert.match(html, /data-action="edit-quota"/);
  assert.match(html, /\$0\.38 \/ \$1\.00/);
  assert.match(html, /进度按分向上取整展示/);
});

test("users view hides quota editing for viewer role", async () => {
  const harness = newUsersHarness();
  const view = harness.createView({
    getUsers: async () => ({
      users: [{
        user_id: "user-1",
        organization_id: "org-1",
        email: "viewer@democorp.local",
        display_name: "Demo Viewer",
        used_tokens: 7,
        used_budget_cents: 1,
        budget_cents: 50,
        status: "active",
      }],
    }),
  }, { role: "viewer" });

  await view.load(true);

  const html = harness.element("users-table-body").innerHTML;
  assert.doesNotMatch(html, /data-action="generate-key"/);
  assert.doesNotMatch(html, /data-action="edit-quota"/);
  assert.match(html, /\$0\.01 \/ \$0\.50/);
});

function newUsersHarness() {
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
  runBrowserScript(context, "web/src/views/users.js");

  return {
    createView(api, options) {
      return context.window.OmniTokenViews.createUsersView(api, options);
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
    for (const id of ["users-alert", "users-table-body"]) {
      this.elements.set(id, new FakeElement(id));
    }
    this.actions.set("reload-users", new FakeElement("reload-users"));
    this.actions.set("open-user-modal", new FakeElement("open-user-modal"));
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
