package web

const loginHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Omni CD · Login</title>
<link rel="icon" type="image/svg+xml" href="{{OMNI_LOGO_URI}}">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Roboto:wght@300;400;500;700&family=Roboto+Mono:wght@300;400;500&display=swap" rel="stylesheet">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: Roboto, sans-serif;
    background: #101118;
    color: #e8e8e9;
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
  .login-logo-text span { color: #ff8b59; }
  .login-card {
    background: #1f222e;
    border: 1px solid #2a2d3a;
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
    background: #101118;
    border: 1px solid #2a2d3a;
    border-radius: 8px;
    color: #e8e8e9;
    font-size: 14px;
    padding: 10px 14px;
    outline: none;
    transition: border-color 0.15s;
  }
  .login-field input:focus { border-color: #ff8b59; }
  .login-field input::placeholder { color: #52525b; }
  .login-btn {
    width: 100%;
    background: #ff8b59;
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
  .login-btn:hover { background: #e67a4a; }
  .login-btn:active { background: #cc5e35; }
  .login-footer {
    text-align: center;
    font-size: 11px;
    color: #52525b;
    margin-top: 24px;
  }
  .sso-btn {
    display: block;
    width: 100%;
    background: #101118;
    border: 1px solid #2a2d3a;
    border-radius: 8px;
    color: #e8e8e9;
    font-size: 14px;
    font-weight: 600;
    padding: 11px;
    text-align: center;
    text-decoration: none;
    transition: border-color 0.15s, background 0.15s;
  }
  .sso-btn:hover { border-color: #ff8b59; background: #1f222e; }
  .login-divider {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 16px 0;
    color: #52525b;
    font-size: 12px;
  }
  .login-divider::before,
  .login-divider::after {
    content: '';
    flex: 1;
    height: 1px;
    background: #2a2d3a;
  }
</style>
</head>
<body>
<div class="login-wrap">
  <div class="login-logo">
    <img src="{{OMNI_LOGO_URI}}" alt="Omni">
    <span class="login-logo-text">Omni <span>CD</span></span>
  </div>
  <div class="login-card">
    <div class="login-title">Sign in</div>
    <div class="login-sub">Enter your credentials to continue</div>
    <!--ERROR-->
    <!--LOCAL_FORM-->
    <!--OIDC_BUTTON-->
  </div>
  <div class="login-footer">Omni CD · Real-time updates</div>
</div>
</body>
</html>`

const localFormHTML = `<form method="POST" action="/login">
      <div class="login-field">
        <label for="username">Username</label>
        <input id="username" name="username" type="text" placeholder="username" autocomplete="username" required autofocus>
      </div>
      <div class="login-field">
        <label for="password">Password</label>
        <input id="password" name="password" type="password" placeholder="••••••••" autocomplete="current-password" required>
      </div>
      <button class="login-btn" type="submit">Sign in</button>
    </form>`
