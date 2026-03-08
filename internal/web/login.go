package web

const loginHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Omni CD · Login</title>
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
</style>
</head>
<body>
<div class="login-wrap">
  <div class="login-logo">
    <img src="https://mintlify.s3.us-west-1.amazonaws.com/siderolabs-fe86397c/images/omni.svg" alt="Omni">
    <span class="login-logo-text">Omni <span>CD</span></span>
  </div>
  <div class="login-card">
    <div class="login-title">Sign in</div>
    <div class="login-sub">Enter your credentials to continue</div>
    <!--ERROR-->
    <form method="POST" action="/login">
      <div class="login-field">
        <label for="username">Username</label>
        <input id="username" name="username" type="text" placeholder="admin" autocomplete="username" required autofocus>
      </div>
      <div class="login-field">
        <label for="password">Password</label>
        <input id="password" name="password" type="password" placeholder="••••••••" autocomplete="current-password" required>
      </div>
      <button class="login-btn" type="submit">Sign in</button>
    </form>
  </div>
  <div class="login-footer">Omni CD · Real-time updates</div>
</div>
</body>
</html>`
