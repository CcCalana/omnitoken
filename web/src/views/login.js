(function () {
const { showAlert } = window.OmniTokenUtils;

function createLoginView(api) {
  let isLoaded = false;
  const loginForm = document.getElementById("login-form");
  const alertContainer = document.getElementById("login-alert");

  if (loginForm) {
    loginForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      const email = document.getElementById("login-email").value.trim();
      const password = document.getElementById("login-password").value;

      if (!email || !password) {
        showAlert(alertContainer, "请输入邮箱和密码", "error");
        return;
      }

      const btn = loginForm.querySelector('button[type="submit"]');
      const originalText = btn.textContent;
      btn.textContent = "登录中...";
      btn.disabled = true;

      try {
        const response = await fetch(`${api.baseURL}/api/admin/login`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email, password }),
        });

        if (!response.ok) {
          const data = await response.json().catch(() => ({}));
          throw new Error(data.error?.message || `HTTP ${response.status}`);
        }

        const data = await response.json();
        localStorage.setItem("omnitokenAdminToken", data.token);
        
        // Reload app state to switch out of login view
        window.location.reload();
      } catch (err) {
        showAlert(alertContainer, `登录失败: ${err.message}`, "error");
      } finally {
        btn.textContent = originalText;
        btn.disabled = false;
      }
    });
  }

  async function load(force = false) {
    if (isLoaded && !force) return;
    showAlert(alertContainer, "");
    isLoaded = true;
  }

  return { load };
}

window.OmniTokenViews = window.OmniTokenViews || {};
window.OmniTokenViews.createLoginView = createLoginView;
})();
