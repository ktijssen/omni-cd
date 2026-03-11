package web

import (
	"fmt"
	"net/http"
	"strings"

	"omni-cd/internal/auth"
)

// handleSetup serves GET /setup (first-time setup page) and POST /setup (create admin account).
// Once any user exists the endpoint returns 404 to prevent re-initialisation.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if s.authStore == nil || !s.authStore.IsEmpty() {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, setupHTML)

	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
		password := r.FormValue("password")
		confirm := r.FormValue("confirm")
		if err := auth.ValidatePasswordStrength(password); err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, strings.ReplaceAll(setupHTML, "<!--ERROR-->",
				`<div class="login-error">`+err.Error()+`</div>`))
			return
		}
		if password != confirm {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, strings.ReplaceAll(setupHTML, "<!--ERROR-->",
				`<div class="login-error">Passwords do not match</div>`))
			return
		}
		if err := s.authStore.SetUser("admin", "Admin", password); err != nil {
			http.Error(w, "Failed to save credentials", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

const setupHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Omni CD · Setup</title>
<link rel="icon" type="image/svg+xml" href="https://mintlify.s3.us-west-1.amazonaws.com/siderolabs-fe86397c/images/omni.svg">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #1b1b1d;
    color: #e4e4e7;
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .login-wrap {
    width: 100%;
    max-width: 380px;
    padding: 24px;
  }
  .login-logo {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    margin-bottom: 32px;
  }
  .login-logo img { width: 32px; height: 32px; }
  .login-logo-text {
    font-size: 20px;
    font-weight: 700;
    color: #fff;
    letter-spacing: -0.4px;
  }
  .login-logo-text span { color: #FB326E; }
  .login-card {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 14px;
    padding: 28px 28px 24px;
  }
  .login-title {
    font-size: 15px;
    font-weight: 600;
    color: #fff;
    margin-bottom: 4px;
  }
  .login-sub {
    font-size: 12px;
    color: #71717a;
    margin-bottom: 24px;
  }
  .login-error {
    background: rgba(248, 113, 113, 0.1);
    border: 1px solid rgba(248, 113, 113, 0.3);
    border-radius: 8px;
    color: #f87171;
    font-size: 13px;
    padding: 10px 14px;
    margin-bottom: 16px;
  }
  .login-field { margin-bottom: 16px; }
  .login-field label {
    display: block;
    font-size: 12px;
    font-weight: 500;
    color: #a1a1aa;
    margin-bottom: 6px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .login-field input {
    width: 100%;
    background: #18181b;
    border: 1px solid #3f3f46;
    border-radius: 8px;
    color: #e4e4e7;
    font-size: 14px;
    padding: 10px 14px;
    outline: none;
    transition: border-color 0.15s;
  }
  .login-field input:focus { border-color: #FB326E; }
  .login-field input::placeholder { color: #52525b; }
  .login-btn {
    width: 100%;
    background: #FB326E;
    color: #fff;
    border: none;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 600;
    padding: 11px;
    cursor: pointer;
    margin-top: 8px;
    transition: background 0.2s;
  }
  .login-btn:hover { background: #e0285f; }
  .login-btn:active { background: #c92255; }
  .login-footer {
    text-align: center;
    font-size: 11px;
    color: #52525b;
    margin-top: 24px;
  }
  .pw-checks { display: flex; flex-direction: column; gap: 5px; margin: 0 0 16px; }
  .pw-check { display: flex; align-items: center; gap: 8px; font-size: 12px; color: #52525b; transition: color 0.15s; }
  .pw-check.met { color: #4ade80; }
  .pw-check-icon { font-size: 11px; width: 12px; flex-shrink: 0; }
</style>
</head>
<body>
<div class="login-wrap">
  <div class="login-logo">
    <img src="https://mintlify.s3.us-west-1.amazonaws.com/siderolabs-fe86397c/images/omni.svg" alt="Omni">
    <span class="login-logo-text">Omni <span>CD</span></span>
  </div>
  <div class="login-card">
    <div class="login-title">Create your Admin account</div>
    <div class="login-sub">Set up access to Omni CD</div>
    <!--ERROR-->
    <form method="POST" action="/setup">
      <div class="login-field">
        <label for="username">Username</label>
        <input id="username" name="username" type="text" value="admin" readonly style="opacity:0.5;cursor:not-allowed;">
      </div>
      <div class="login-field">
        <label for="password">Password</label>
        <input id="password" name="password" type="password" placeholder="••••••••" autocomplete="new-password" required autofocus>
      </div>
      <div class="login-field">
        <label for="confirm">Confirm password</label>
        <input id="confirm" name="confirm" type="password" placeholder="••••••••" autocomplete="new-password" required>
        <div id="confirm-msg" style="font-size:12px;margin-top:6px;min-height:16px;"></div>
      </div>
      <div class="pw-checks" id="pw-checks">
        <div class="pw-check" id="chk-len"><span class="pw-check-icon">✗</span>12 characters or more</div>
        <div class="pw-check" id="chk-upper"><span class="pw-check-icon">✗</span>Uppercase letter</div>
        <div class="pw-check" id="chk-num"><span class="pw-check-icon">✗</span>Number</div>
        <div class="pw-check" id="chk-special"><span class="pw-check-icon">✗</span>Special character</div>
      </div>
      <button class="login-btn" type="submit">Set Password</button>
    </form>
  </div>
  <div class="login-footer">Omni CD · First-time setup</div>
</div>
<script>
  function checkConfirm() {
    var msg = document.getElementById('confirm-msg');
    var confirm = document.getElementById('confirm').value;
    if (!confirm) { msg.textContent = ''; return; }
    if (confirm === document.getElementById('password').value) {
      msg.style.color = '#4ade80';
      msg.textContent = '✓ Passwords match';
    } else {
      msg.style.color = '#f87171';
      msg.textContent = '✗ Passwords do not match';
    }
  }
  document.getElementById('password').addEventListener('input', function() {
    var v = this.value;
    var checks = {
      'chk-len':     v.length >= 12,
      'chk-upper':   /[A-Z]/.test(v),
      'chk-num':     /[0-9]/.test(v),
      'chk-special': /[^a-zA-Z0-9]/.test(v)
    };
    for (var id in checks) {
      var el = document.getElementById(id);
      var icon = el.querySelector('.pw-check-icon');
      if (checks[id]) {
        el.classList.add('met');
        icon.textContent = '✓';
      } else {
        el.classList.remove('met');
        icon.textContent = '✗';
      }
    }
    checkConfirm();
  });
  document.getElementById('confirm').addEventListener('input', checkConfirm);
</script>
</body>
</html>`
