package web

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

var omniLogoURI = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(omniLogoSVG))
var profileIconURI = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(profileIconSVG))

// handleUI serves the embedded UI.
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	html := strings.ReplaceAll(uiHTML, "{{APP_VERSION}}", s.version)
	html = strings.ReplaceAll(html, "{{OMNI_LOGO_URI}}", omniLogoURI)
	html = strings.ReplaceAll(html, "{{PROFILE_ICON_URI}}", profileIconURI)
	username := s.sessionUsername(r)
	html = strings.ReplaceAll(html, "{{LOGGED_IN_AS}}", username)
	authDisabledVal := "false"
	if s.authDisabled {
		authDisabledVal = "true"
	}
	html = strings.ReplaceAll(html, "{{AUTH_DISABLED}}", authDisabledVal)
	// Inject the user's role so the JS can show/hide admin-only controls.
	// AUTH_DISABLED and local-auth users always get "admin".
	userRole := "admin"
	if !s.authDisabled {
		userRole = s.sessionRole(r)
		if userRole == "" {
			userRole = "admin"
		}
	}
	html = strings.ReplaceAll(html, "{{USER_ROLE}}", userRole)
	oidcEnabledVal := "false"
	if s.oidcEnabled() {
		oidcEnabledVal = "true"
	}
	html = strings.ReplaceAll(html, "{{OIDC_ENABLED}}", oidcEnabledVal)
	fmt.Fprint(w, html)
}

const uiHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Omni CD</title>
<link rel="icon" type="image/svg+xml" href="{{OMNI_LOGO_URI}}">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #1b1b1d;
    color: #e4e4e7;
    min-height: 100vh;
  }
  .container { padding: 24px; }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 32px;
    padding-bottom: 16px;
    border-bottom: 1px solid #27272a;
  }
  .header h1 {
    font-size: 24px;
    font-weight: 700;
    color: #fff;
    letter-spacing: -0.5px;
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .header h1 span { color: #FB326E; margin: 0; padding: 0; }
  .logo { width: 28px; height: 28px; }
  .header-buttons { display: flex; align-items: center; gap: 10px; }
  .btn-check {
    background: #FB326E;
    color: #fff;
    border: none;
    padding: 10px 20px;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }
  .btn-check:hover { background: #e0285f; }
  .btn-check:active { background: #c92255; }
  .btn-check:disabled { background: #27272a; color: #52525b; cursor: not-allowed; }
  .btn-reconcile {
    background: #FB326E;
    color: #fff;
    border: none;
    padding: 10px 20px;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }
  .btn-reconcile:hover { background: #e0285f; }
  .btn-reconcile:active { background: #c92255; }
  .btn-reconcile:disabled { background: #27272a; color: #52525b; cursor: not-allowed; }

  .status-bar {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 16px;
    margin-bottom: 24px;
  }
  .status-card {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 12px;
    padding: 20px;
  }
  .status-card .label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: #71717a;
    margin-bottom: 8px;
  }
  .status-card .value {
    font-size: 18px;
    font-weight: 600;
    color: #fff;
    word-break: break-all;
  }
  .status-card .sub {
    font-size: 12px;
    color: #a1a1aa;
    margin-top: 4px;
  }

  .badge {
    display: inline-block;
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 600;
    margin-left: 3px;
  }
  .badge-success { background: #14532d; color: #4ade80; }
  .badge-running { background: #1e3a5f; color: #60a5fa; }
  .badge-failed { background: #451a1e; color: #f87171; }
  .badge-outofsync { background: #431407; color: #fb923c; }
  .badge-orphaned  { background: #2e1065; color: #a78bfa; }
  .badge-unmanaged { background: #27272a; color: #71717a; border: 1px solid #3f3f46; }
  .badge-deleting { background: #451a1e; color: #f87171; }
  .badge-syncing { background: #0d2d2a; color: #2dd4bf; }
  .badge-connecting { background: #2d0d0d; color: #f87171; }
  .badge-idle { background: #3f3f46; color: #a1a1aa; }
  .badge-ready { background: #14532d; color: #4ade80; }
  .badge-notready { background: #451a1e; color: #f87171; }

  .provision-type {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    margin-right: 8px;
    border: 1px solid;
  }
  .provision-type.auto {
    background: #1e3a5f;
    color: #60a5fa;
    border-color: #60a5fa;
  }
  .provision-type.manual {
    background: #431407;
    color: #fb923c;
    border-color: #fb923c;
  }

  .version-warning {
    background: #431407;
    border: 1px solid #fb923c;
    border-radius: 8px;
    padding: 8px 14px;
    color: #fb923c;
    font-size: 12px;
    font-weight: 500;
    display: flex;
    align-items: center;
    gap: 6px;
    white-space: nowrap;
  }
  .version-warning .warn-icon { font-size: 14px; }

  /* Toggle switch */
  .toggle-switch {
    position: relative;
    width: 36px;
    height: 20px;
    background: #3f3f46;
    border-radius: 10px;
    cursor: pointer;
    transition: background 0.2s;
    border: none;
    padding: 0;
    flex-shrink: 0;
  }
  .toggle-switch.on { background: #FB326E; }
  .toggle-switch .toggle-knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 16px;
    height: 16px;
    background: #fff;
    border-radius: 50%;
    transition: transform 0.2s;
  }
  .toggle-switch.on .toggle-knob { transform: translateX(16px); }
  .panel-header-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .toggle-status { font-size: 11px; font-weight: 600; }
  .toggle-status.on { color: #4ade80; }
  .toggle-status.off { color: #f87171; }

  .panels {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
    margin-bottom: 24px;
  }
  @media (max-width: 768px) { .panels { grid-template-columns: 1fr; } }
  .panel {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 12px;
    overflow: hidden;
  }
  .panel-header {
    padding: 16px 20px;
    border-bottom: 1px solid #3f3f46;
    font-size: 14px;
    font-weight: 600;
    color: #fff;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .panel-header .count {
    background: #3f3f46;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 12px;
    color: #a1a1aa;
  }
  .resource-list { padding: 8px 0; }
  .resource-item {
    padding: 10px 20px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 13px;
    border-bottom: 1px solid #1b1b1d;
  }
  .resource-item:last-child { border-bottom: none; }
  .resource-id { font-family: 'SF Mono', 'Fira Code', monospace; color: #e4e4e7; }
  .resource-id.clickable { cursor: pointer; }
  .resource-id.clickable:hover { color: #FB326E; }
  .resource-right { display: flex; align-items: center; gap: 8px; }
  .btn-diff {
    background: none;
    border: 1px solid #3f3f46;
    color: #a1a1aa;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    cursor: pointer;
    font-family: 'SF Mono', 'Fira Code', monospace;
  }
  .btn-diff:hover { border-color: #fb923c; color: #fb923c; }
  .btn-sync {
    background: none;
    border: 1px solid #ca8a04;
    color: #fbbf24;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    cursor: pointer;
    font-family: 'SF Mono', 'Fira Code', monospace;
  }
  .btn-sync:hover { border-color: #fbbf24; background: rgba(251, 191, 36, 0.1); }
  .btn-export {
    background: none;
    border: 1px solid #0891b2;
    color: #22d3ee;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    cursor: pointer;
    font-family: 'SF Mono', 'Fira Code', monospace;
    margin-right: 8px;
  }
  .btn-export:hover { border-color: #22d3ee; background: rgba(34, 211, 238, 0.1); }
  .btn-force-sync {
    background: none;
    border: 1px solid #c2410c;
    color: #fb923c;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    cursor: pointer;
    font-family: 'SF Mono', 'Fira Code', monospace;
    margin-right: 8px;
  }
  .btn-force-sync:hover { border-color: #fb923c; background: rgba(251, 146, 60, 0.1); }
  .btn-sort { background: none; border: 1px solid #3f3f46; color: #71717a; padding: 2px 8px; border-radius: 4px; font-size: 11px; cursor: pointer; font-family: 'SF Mono', 'Fira Code', monospace; white-space: nowrap; }
  .btn-sort:hover { border-color: #a1a1aa; color: #a1a1aa; }
  .btn-sort:disabled { opacity: 0.4; cursor: not-allowed; }
  .btn-sort:disabled:hover { border-color: #3f3f46; color: #71717a; }
  .btn-sort.active { border-color: #FB326E; color: #FB326E; background: rgba(251, 50, 110, 0.1); }
  .btn-sort.btn-primary:hover { border-color: #FB326E; color: #FB326E; }
  .btn-sort.btn-primary:disabled:hover { border-color: #3f3f46; color: #71717a; }
  .btn-sort.btn-primary.active:hover { border-color: #FB326E; color: #FB326E; background: rgba(251, 50, 110, 0.1); }
  .cluster-card-actions .btn-sort.btn-primary:hover { border-color: #a1a1aa; color: #a1a1aa; }
  .cluster-card-actions .btn-sort.btn-primary.active:hover { border-color: #71717a; color: #71717a; background: rgba(63,63,70,0.4); }
  .btn-sort.btn-primary.auto-sync:not(.active):hover { border-color: #3f3f46; color: #a1a1aa; background: transparent; }
  .btn-sort.btn-primary.auto-sync.active:hover { border-color: #FB326E; color: #FB326E; background: rgba(251, 50, 110, 0.1); }
  .cluster-card-actions .btn-sort.btn-primary.auto-sync:not(.active):hover { border-color: #3f3f46; color: #a1a1aa; background: transparent; }
  .cluster-card-actions .btn-sort.btn-primary.auto-sync.active:hover { border-color: #FB326E; color: #FB326E; background: rgba(251, 50, 110, 0.1); }
  .breadcrumb { display:flex; align-items:center; gap:6px; }
  .breadcrumb-link { color:#a1a1aa; text-decoration:none; font-size:18px; font-weight:600; letter-spacing:-0.3px; }
  .breadcrumb-link:hover { color:#fff; }
  .breadcrumb-sep { color:#3f3f46; font-size:15px; font-weight:400; }
  .breadcrumb-current { color:#fff; font-family:'SF Mono','Fira Code',monospace; font-size:16px; font-weight:600; }
  .panel-nav-link { cursor: pointer; transition: color 0.15s; }
  .panel-nav-link:hover { color: #FB326E; }
  .cluster-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 16px; padding: 16px 0; }
  .cluster-card { background: #27272a; border: 1px solid #3f3f46; border-radius: 12px; overflow: hidden; display: flex; }
  .cluster-card.clickable { cursor: pointer; }
  .cluster-card.clickable:hover { border-color: #71717a; }
  .cluster-card-actions { display: flex; gap: 6px; padding: 2px 0 0; margin-top: auto; flex-wrap: nowrap; }
  .btn-sort.sync:hover    { border-color: #fb923c; color: #fb923c; background: rgba(251,146,60,0.08); }
  .btn-sort.refresh:hover { border-color: #22d3ee; color: #22d3ee; background: rgba(34,211,238,0.08); }
  .btn-sort.danger:hover  { border-color: #f87171; color: #f87171; background: rgba(248,113,113,0.08); }
  .cluster-card-accent { width: 4px; flex-shrink: 0; background: #3f3f46; }
  .cluster-card[data-status="success"] .cluster-card-accent,
  .cluster-card[data-status="applied"] .cluster-card-accent { background: #4ade80; }
  .cluster-card[data-status="failed"] .cluster-card-accent { background: #f87171; }
  .cluster-card[data-status="outofsync"] .cluster-card-accent { background: #fb923c; }
  .cluster-card[data-status="orphaned"]  .cluster-card-accent { background: #a78bfa; }
  .cluster-card[data-status="syncing"] .cluster-card-accent { background: #2dd4bf; }
  .cluster-card[data-status="unmanaged"] .cluster-card-accent { background: #52525b; }
  .cluster-card[data-status="deleting"] .cluster-card-accent { background: #f87171; }
  /* Override accent to red when synced but cluster not ready */
  .cluster-card[data-status="success"][data-health="not-ready"] .cluster-card-accent,
  .cluster-card[data-status="applied"][data-health="not-ready"] .cluster-card-accent { background: #f87171; }
  .cluster-card[data-phase="scaling-up"] .cluster-card-accent    { background: #60a5fa; }
  .cluster-card[data-phase="scaling-down"] .cluster-card-accent  { background: #f59e0b; }
  .cluster-card[data-phase="destroying"] .cluster-card-accent    { background: #f43f5e; }
  .cluster-card[data-phase="reconfiguring"] .cluster-card-accent { background: #a78bfa; }
  .cluster-health-bar-wrap { padding: 0 0 16px; display: flex; align-items: center; gap: 12px; }
  .cluster-health-bar { flex: 1; height: 8px; border-radius: 4px; background: #3f3f46; overflow: hidden; display: flex; }
  .cluster-health-bar-seg { height: 100%; cursor: pointer; transition: width 0.3s, opacity 0.15s; opacity: 0.85; }
  .cluster-health-bar-seg:hover { opacity: 1; }
  .cluster-health-bar.has-filter .cluster-health-bar-seg { opacity: 0.3; }
  .cluster-health-bar.has-filter .cluster-health-bar-seg.active { opacity: 1; }
  .cluster-health-bar-seg--ready { background: #4ade80; }
  .cluster-health-bar-seg--notready { background: #f87171; }
  .cluster-health-bar-seg--failed { background: #ef4444; }
  .cluster-health-bar-seg--outofsync { background: #fb923c; }
  .cluster-health-bar-seg--orphaned   { background: #a78bfa; }
  .cluster-health-bar-seg--unmanaged { background: #52525b; }
  .cluster-health-bar-seg--scalingup   { background: #60a5fa; }
  .cluster-health-bar-seg--scalingdown { background: #f59e0b; }
  .cluster-health-bar-seg--destroying  { background: #f43f5e; }
  .cluster-health-bar-seg--reconfiguring { background: #a78bfa; }
  .cluster-health-summary { font-size: 12px; color: #71717a; white-space: nowrap; }
  .cluster-card-body { flex: 1; padding: 12px 14px; min-width: 0; display: flex; flex-direction: column; }
  .cluster-card-header { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; margin-bottom: 2px; }
  .cluster-card-title { font-size: 15px; font-weight: 600; color: #fff; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .cluster-card-title.clickable { color: inherit; }
  .cluster-card-status { font-size: 11px; white-space: nowrap; flex-shrink: 0; }
  .cluster-card-versions { font-size: 11px; color: #71717a; margin-bottom: 10px; }
  .cluster-card-divider { height: 1px; background: #3f3f46; margin-bottom: 8px; }
  .cluster-pool-row { display: flex; align-items: baseline; gap: 8px; font-size: 12px; padding: 2px 0; }
  .cluster-pool-row-label { color: #a1a1aa; width: 88px; flex-shrink: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .cluster-pool-row-count { color: #e4e4e7; font-weight: 600; width: 20px; text-align: right; flex-shrink: 0; }
  .cluster-pool-row-mc { color: #71717a; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; font-size: 11px; }
  .diff-viewer {
    background: #18181b;
    border-top: 1px solid #3f3f46;
    padding: 12px 20px;
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    word-break: break-all;
    max-height: 300px;
    overflow-y: auto;
    color: #a1a1aa;
  }
  .diff-viewer::-webkit-scrollbar { width: 6px; }
  .diff-viewer::-webkit-scrollbar-track { background: #18181b; }
  .diff-viewer::-webkit-scrollbar-thumb { background: #3f3f46; border-radius: 3px; }
  .diff-add { color: #4ade80; }
  .diff-del { color: #f87171; }
  .diff-hdr { color: #60a5fa; }
  .sbs-block { margin-bottom: 16px; }
  .sbs-table { display: grid; grid-template-columns: 1fr 1fr; column-gap: 1px; background: #3f3f46; border-radius: 4px; overflow: hidden; border: 1px solid #3f3f46; }
  .sbs-col-hdr { padding: 4px 10px; background: #1c1c1e; color: #71717a; font-size: 11px; border-bottom: 1px solid #3f3f46; }
  .sbs-live-hdr { border-right: none; }
  .sbs-cell { padding: 2px 10px; font-size: 12px; white-space: pre-wrap; word-break: break-all; min-height: 20px; background: #27272a; }
  .sbs-del { background: rgba(248,113,113,0.12); color: #f87171; }
  .sbs-add { background: rgba(74,222,128,0.10); color: #4ade80; }
  .sbs-ln { color: #52525b; margin-right: 8px; user-select: none; font-size: 11px; }
  .drawer-body.mc-live-mode { padding: 0; white-space: normal; word-break: normal; }
  .mc-live-toolbar { display:flex; align-items:center; justify-content:flex-end; padding:8px 16px; border-bottom:1px solid #3f3f46; background:#1c1c1e; position:sticky; top:0; z-index:1; }
  .mc-live-toggle { font-size:12px; color:#a1a1aa; cursor:pointer; display:flex; align-items:center; gap:6px; }
  .mc-live-toggle input[type=checkbox] { accent-color:#fb923c; cursor:pointer; }
  .mc-live-table { width:100%; border-collapse:collapse; font-family:'SF Mono','Fira Code',monospace; font-size:12px; line-height:1.6; }
  .mc-live-ln { color:#52525b; text-align:right; padding:2px 10px 2px 16px; user-select:none; min-width:36px; border-right:1px solid #27272a; white-space:nowrap; vertical-align:top; }
  .mc-live-code { padding:2px 16px; white-space:pre-wrap; word-break:break-all; color:#e4e4e7; }
  .mc-live-meta-row td { color:#52525b; opacity:0.55; }
  .mc-live-ignored-row td { color:#3f3f46; font-style:italic; }
  .sbs-table-single { background: #27272a; border-radius: 4px; overflow: hidden; border: 1px solid #3f3f46; }
  .sbs-meta-dim { opacity: 0.45; }
  .sbs-ignored { color: #52525b; font-style: italic; }

  .logs-page { display:flex; flex-direction:column; height:100%; }
  .logs-page-header { display:flex; align-items:center; justify-content:space-between; padding:0; margin-bottom:0; border-bottom:none; }
  .logs-page-body { flex:1; overflow-y:auto; background:#1b1b1d; border-radius:8px; }
  .logs-filters { display:flex; align-items:center; gap:8px; flex-wrap:wrap; padding:8px 0 0; }
  .logs-page-body::-webkit-scrollbar { width:6px; }
  .logs-page-body::-webkit-scrollbar-track { background:#1b1b1d; }
  .logs-page-body::-webkit-scrollbar-thumb { background:#3f3f46; border-radius:3px; }
  .logs-modal-header {
    padding: 20px 24px 16px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid #3f3f46;
  }
  .logs-modal-title {
    font-size: 16px;
    font-weight: 600;
    color: #fff;
  }
  .logs-modal-actions {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .logs-container {
    height: 400px;
    overflow-y: auto;
    padding: 12px 0;
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 12px;
    line-height: 1.6;
  }
  .logs-container::-webkit-scrollbar { width: 6px; }
  .logs-container::-webkit-scrollbar-track { background: #1b1b1d; }
  .logs-container::-webkit-scrollbar-thumb { background: #3f3f46; border-radius: 3px; }
  .log-entry { padding: 2px 20px; font-size: 11px; font-weight: 400; }
  .log-entry:hover { background: #323235; }
  .log-ts { color: #52525b; }
  .log-debug { color: #818cf8; }
  .log-info { color: #e4e4e7; }
  .log-warn { color: #facc15; }
  .log-error { color: #f87171; }
  .log-label { color: #a1a1aa; }
  .log-msg { color: #e4e4e7; }

  .refresh-indicator {
    position: fixed;
    bottom: 0;
    left: var(--sidebar-w, 200px);
    right: 0;
    transition: left 0.2s ease;
    font-size: 11px;
    color: #52525b;
    text-align: center;
    padding: 8px 16px;
    background: #1b1b1d;
    border-top: 1px solid #27272a;
    z-index: 10;
  }

  @keyframes spin { to { transform: rotate(360deg); } }
  .spinner {
    display: inline-block;
    width: 16px;
    height: 16px;
    border: 2px solid #3f3f46;
    border-top-color: #FB326E;
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
    vertical-align: middle;
    margin-right: 8px;
    flex-shrink: 0;
  }

  .loading-overlay {
    position: fixed;
    inset: 0;
    background: #1b1b1d;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 20px;
    z-index: 9999;
    transition: opacity 0.3s ease;
  }
  .loading-overlay.hidden {
    opacity: 0;
    pointer-events: none;
  }
  .loading-spinner-large {
    width: 48px;
    height: 48px;
    border: 3px solid #3f3f46;
    border-top-color: #FB326E;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  .loading-overlay-title {
    font-size: 18px;
    font-weight: 600;
    color: #e4e4e7;
  }
  .loading-overlay-sub {
    font-size: 13px;
    color: #71717a;
  }

  /* Right slide-over drawer */
  .drawer-backdrop {
    display: none;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 200;
  }
  .drawer-backdrop.show { display: block; }
  .drawer {
    position: fixed;
    bottom: 0;
    left: 200px;
    right: 0;
    width: calc(100% - 200px);
    height: 78vh;
    background: #27272a;
    border-top: 1px solid #3f3f46;
    border-radius: 12px 12px 0 0;
    z-index: 201;
    display: none;
    flex-direction: column;
  }
  .drawer.show { display: flex; animation: slideInUp 0.25s cubic-bezier(0.4, 0, 0.2, 1); }
  @keyframes slideInUp {
    from { transform: translateY(100%); }
    to   { transform: translateY(0); }
  }
  .drawer-header {
    padding: 20px 24px 0;
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-shrink: 0;
  }
  .drawer-title {
    font-size: 15px;
    font-weight: 600;
    color: #fff;
    font-family: 'SF Mono', 'Fira Code', monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .drawer-close {
    background: none;
    border: none;
    color: #a1a1aa;
    font-size: 24px;
    cursor: pointer;
    padding: 0;
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    transition: all 0.2s;
    flex-shrink: 0;
  }
  .drawer-close:hover { background: #3f3f46; color: #fff; }
  .drawer-tabs {
    display: flex;
    gap: 4px;
    padding: 0 24px;
    margin-top: 16px;
    border-bottom: 1px solid #3f3f46;
    flex-shrink: 0;
    align-items: center;
  }
  .mc-ignored-toggle { margin-left: auto; font-size: 12px; color: #a1a1aa; cursor: pointer; display: flex; align-items: center; gap: 6px; padding: 0 0 0 16px; border-left: 1px solid #3f3f46; user-select: none; }
  .mc-ignored-toggle input[type=checkbox] { accent-color: #fb923c; cursor: pointer; }
  .drawer-tab {
    background: none;
    border: none;
    color: #a1a1aa;
    padding: 10px 16px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    border-bottom: 2px solid transparent;
    transition: all 0.2s;
  }
  .drawer-tab:hover { color: #e4e4e7; }
  .drawer-tab.active { color: #FB326E; border-bottom-color: #FB326E; }
  .drawer-body {
    padding: 24px;
    overflow-y: auto;
    flex: 1;
    min-height: 0;
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 13px;
    line-height: 1.6;
    color: #e4e4e7;
    white-space: pre-wrap;
    word-break: break-all;
  }
  .drawer-body.graph-mode { padding: 0; overflow: hidden; display: flex; flex-direction: column; white-space: normal; word-break: normal; }
  .drawer-body::-webkit-scrollbar { width: 8px; }
  .drawer-body::-webkit-scrollbar-track { background: #1b1b1d; }
  .drawer-body::-webkit-scrollbar-thumb { background: #3f3f46; border-radius: 4px; }

  /* Machine classes grid */
  .mc-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 12px; }
  .mc-card { background: #27272a; border: 1px solid #3f3f46; border-radius: 12px; overflow: hidden; display: flex; }
  .mc-card.clickable { cursor: pointer; }
  .mc-card.clickable:hover { border-color: #71717a; }
  .mc-card-accent { width: 4px; flex-shrink: 0; background: #3f3f46; }
  .mc-card[data-status="success"] .mc-card-accent,
  .mc-card[data-status="applied"] .mc-card-accent { background: #4ade80; }
  .mc-card[data-status="failed"] .mc-card-accent { background: #f87171; }
  .mc-card[data-status="outofsync"] .mc-card-accent { background: #fb923c; }
  .mc-card[data-status="syncing"] .mc-card-accent { background: #2dd4bf; }
  .mc-card-header { flex: 1; padding: 12px 14px; min-width: 0; display: flex; flex-direction: column; }
  .mc-card-title-row { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; margin-bottom: 8px; }
  .mc-card-name {
    font-size: 13px;
    font-weight: 600;
    color: #e4e4e7;
    font-family: 'SF Mono', 'Fira Code', monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .mc-card-name.clickable { color: inherit; }
  .mc-card-status { font-size: 11px; white-space: nowrap; flex-shrink: 0; }
  .mc-card-divider { height: 1px; background: #3f3f46; margin-bottom: 8px; }
  .mc-used-by { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 2px; }
  .mc-used-by-chip { font-size: 10px; font-family: 'SF Mono','Fira Code',monospace; background: #3f3f46; color: #a1a1aa; border-radius: 4px; padding: 2px 6px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 160px; cursor: pointer; border: none; transition: background 0.15s, color 0.15s; }
  .mc-used-by-chip:hover { background: rgba(251, 50, 110, 0.1); color: #FB326E; }
  .mc-used-by-none { font-size: 11px; color: #52525b; font-style: italic; }
  .page-size-bar { display: flex; align-items: center; gap: 6px; }
  .page-size-btn { background: none; border: 1px solid #3f3f46; color: #a1a1aa; padding: 3px 10px; border-radius: 4px; font-size: 12px; cursor: pointer; transition: all 0.2s; }
  .page-size-btn:hover:not(.active) { border-color: #71717a; color: #e4e4e7; }
  .page-size-btn.active { background: #3f3f46; border-color: #52525b; color: #fff; }
  .mc-info-row { display: flex; gap: 8px; font-size: 12px; padding: 1px 0; }
  .mc-info-label { color: #71717a; min-width: 74px; flex-shrink: 0; }
  .mc-info-value { color: #e4e4e7; font-weight: 500; }

  /* Modal */
  .modal {
    display: none;
    position: fixed;
    z-index: 1000;
    left: 0;
    top: 0;
    width: 100%;
    height: 100%;
    background-color: rgba(0, 0, 0, 0.7);
    animation: fadeIn 0.2s;
  }
  .modal.show { display: flex; align-items: center; justify-content: center; }
  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  .modal-content {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 12px;
    width: 900px;
    max-width: 90%;
    height: 75vh;
    display: flex;
    flex-direction: column;
    animation: slideIn 0.2s;
  }
  @keyframes slideIn {
    from { transform: translateY(-20px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }
  .modal-header {
    padding: 20px 24px 0;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .modal-title {
    font-size: 16px;
    font-weight: 600;
    color: #fff;
    font-family: 'SF Mono', 'Fira Code', monospace;
  }
  .modal-close {
    background: none;
    border: none;
    color: #a1a1aa;
    font-size: 24px;
    cursor: pointer;
    padding: 0;
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    transition: all 0.2s;
  }
  .modal-close:hover { background: #3f3f46; color: #fff; }
  .modal-tabs {
    display: flex;
    gap: 4px;
    padding: 0 24px;
    margin-top: 16px;
    border-bottom: 1px solid #3f3f46;
  }
  .modal-tab {
    background: none;
    border: none;
    color: #a1a1aa;
    padding: 10px 16px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    border-bottom: 2px solid transparent;
    transition: all 0.2s;
  }
  .modal-tab:hover { color: #e4e4e7; }
  .modal-tab.active {
    color: #FB326E;
    border-bottom-color: #FB326E;
  }
  .modal-body {
    padding: 24px;
    overflow-y: auto;
    flex: 1;
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 13px;
    line-height: 1.6;
    color: #e4e4e7;
    white-space: pre-wrap;
    word-break: break-all;
  }
  .modal-body::-webkit-scrollbar { width: 8px; }
  .modal-body::-webkit-scrollbar-track { background: #1b1b1d; }
  .modal-body::-webkit-scrollbar-thumb { background: #3f3f46; border-radius: 4px; }

  /* Confirmation modal */
  .confirm-modal {
    max-width: 680px;
    width: 680px;
    height: auto;
    min-height: unset;
  }
  .confirm-modal .modal-header { padding: 14px 20px 0; }
  .confirm-body {
    padding: 12px 20px 16px;
    text-align: left;
    white-space: normal;
    word-break: normal;
  }
  .confirm-icon {
    display: none;
  }
  .confirm-message {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 13px;
    line-height: 1.4;
    color: #e4e4e7;
    margin-bottom: 14px;
    white-space: pre-line;
  }
  .confirm-actions {
    display: flex;
    gap: 12px;
    justify-content: flex-end;
  }
  .confirm-input-prompt {
    font-size: 13px;
    color: #a1a1aa;
    margin-bottom: 8px;
    text-align: left;
  }
  .confirm-input {
    width: 100%;
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 6px;
    color: #e4e4e7;
    font-size: 13px;
    font-family: ui-monospace, SFMono-Regular, monospace;
    padding: 7px 10px;
    margin-bottom: 12px;
    outline: none;
    box-sizing: border-box;
  }
  .confirm-input:focus { border-color: #f87171; }

  /* Cluster topology graph */
  .cluster-graph { font-family: -apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif; white-space: normal; word-break: normal; display: flex; flex-direction: column; overflow: hidden; flex: 1; min-height: 0; }
  .cluster-graph-toolbar { display: flex; align-items: center; justify-content: flex-end; gap: 4px; padding: 10px 16px 0; flex-shrink: 0; }
  .cluster-graph-zoom-btn { background: #3f3f46; border: 1px solid #52525b; color: #a1a1aa; border-radius: 5px; width: 26px; height: 26px; font-size: 16px; line-height: 1; cursor: pointer; display: flex; align-items: center; justify-content: center; }
  .cluster-graph-zoom-btn:hover { background: #52525b; color: #e4e4e7; }
  .graph-zoom-level { font-size: 11px; color: #71717a; min-width: 34px; text-align: center; }
  .graph-toolbar-sep { width: 1px; height: 16px; background: #3f3f46; margin: 0 2px; flex-shrink: 0; }
  .fold-badge { display: inline-flex; align-items: center; justify-content: center; background: #52525b; color: #e4e4e7; border-radius: 4px; font-size: 10px; font-weight: 600; padding: 1px 5px; margin-right: 2px; letter-spacing: 0.02em; }
  .cluster-graph-canvas { flex: 1; overflow: auto; padding: 24px 20px; cursor: grab; user-select: none; }
  .cluster-graph-inner { display: inline-flex; transform-origin: top left; transition: transform 0.15s; }
  .graph-node { background: #1b1b1d; border: 1px solid #3f3f46; border-radius: 10px; padding: 14px 20px; overflow: hidden; }
  .graph-node--git { border-color: #3b82f6; }
  .graph-node-label { font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.08em; color: #52525b; margin-bottom: 6px; }
  .graph-node-name { font-size: 14px; font-weight: 600; color: #fff; margin-bottom: 3px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .graph-node-meta { font-size: 11px; color: #71717a; line-height: 1.5; }
  .graph-node-badges { display: flex; gap: 5px; margin-top: 8px; flex-wrap: wrap; }
  .graph-extensions { margin-top: 8px; padding-top: 7px; border-top: 1px solid #3f3f46; }
  .graph-extensions summary { font-size: 11px; color: #71717a; cursor: pointer; list-style: none; user-select: none; }
  .graph-extensions summary::-webkit-details-marker { display: none; }
  .graph-extensions summary::before { content: '▶ '; font-size: 9px; color: #52525b; }
  .graph-extensions[open] summary::before { content: '▼ '; }
  .graph-ext-item { font-size: 11px; color: #52525b; padding: 3px 0 0 12px; font-family: 'SF Mono','Fira Code',monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  /* DAG graph - Argo CD style left-to-right layout */
  .dag-node { display:flex; align-items:stretch; background:#18181b; border:1px solid #3f3f46; border-radius:6px; overflow:hidden; height:100px; flex-shrink:0; }
  .dag-node.clickable:hover { border-color:#71717a; }
  .dag-node-accent { width:3px; flex-shrink:0; }
  .dag-node-icon { display:flex; align-items:center; justify-content:center; width:36px; flex-shrink:0; }
  .dag-node-body { flex:1; padding:10px 10px 10px 6px; overflow:hidden; display:flex; flex-direction:column; justify-content:center; min-width:0; }
  .dag-node-kind { font-size:10px; font-weight:700; text-transform:uppercase; letter-spacing:0.07em; color:#52525b; margin-bottom:2px; }
  .dag-node-name { font-size:13px; font-weight:600; color:#e4e4e7; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
  .dag-node-meta { font-size:10px; color:#71717a; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; margin-top:2px; }
  .dag-node-badges { display:flex; gap:3px; margin-top:5px; flex-wrap:wrap; }

  /* Pagination */
  .pagination {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 4px;
    padding: 12px 20px;
    border-top: 1px solid #1b1b1d;
  }
  .page-btn {
    background: none;
    border: 1px solid #3f3f46;
    color: #a1a1aa;
    padding: 4px 10px;
    border-radius: 4px;
    font-size: 12px;
    cursor: pointer;
    min-width: 32px;
    transition: all 0.2s;
  }
  .page-btn:hover:not(:disabled) {
    border-color: #FB326E;
    color: #FB326E;
  }
  .page-btn.active {
    background: #FB326E;
    border-color: #FB326E;
    color: #fff;
  }
  .page-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  /* Dashboard */
  .dash-fleet-card { background: #27272a; border: 1px solid #3f3f46; border-radius: 12px; padding: 18px 20px; }
  .dash-fleet-title { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.08em; color: #71717a; margin-bottom: 12px; }
  .dash-fleet-bar { height: 10px; border-radius: 5px; background: #3f3f46; overflow: hidden; display: flex; margin-bottom: 12px; }
  .dash-fleet-seg { height: 100%; transition: width 0.4s; }
  .dash-fleet-seg--ready     { background: #4ade80; }
  .dash-fleet-seg--notready  { background: #f87171; }
  .dash-fleet-seg--outofsync { background: #fb923c; }
  .dash-fleet-seg--orphaned  { background: #a78bfa; }
  .dash-fleet-seg--failed    { background: #ef4444; }
  .dash-fleet-seg--unmanaged { background: #52525b; }
  .dash-fleet-legend { display: flex; align-items: center; gap: 20px; flex-wrap: wrap; }
  .dash-fleet-legend-item { display: flex; align-items: center; gap: 6px; font-size: 12px; color: #a1a1aa; }
  .dash-fleet-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .stat-strip { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 12px; }
  @media (max-width: 900px) { .stat-strip { grid-template-columns: repeat(2, 1fr); } }
  .stat-tile {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 12px;
    padding: 16px 20px;
  }
  .stat-tile-label { font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: #71717a; margin-bottom: 6px; }
  .stat-tile-value { font-size: 28px; font-weight: 700; color: #fff; line-height: 1; margin-bottom: 4px; }
  .stat-tile-sub { font-size: 12px; color: #a1a1aa; }
  .mini-bar { height: 4px; border-radius: 2px; background: #3f3f46; overflow: hidden; margin-top: 8px; }
  .mini-bar-fill { height: 100%; border-radius: 2px; background: #4ade80; transition: width 0.4s; }
  .info-row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 12px; }
  @media (max-width: 768px) { .info-row { grid-template-columns: 1fr; } }
  .info-card {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 12px;
    padding: 18px 20px;
  }
  .info-card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
  .info-card-title { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.08em; color: #71717a; }
  .info-card-value { font-size: 14px; font-weight: 600; color: #fff; margin-bottom: 8px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .info-card-sub { font-size: 12px; color: #71717a; line-height: 1.7; }
  .reconcile-bar {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 12px;
    padding: 12px 20px;
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;
    flex-wrap: wrap;
  }
  .reconcile-bar-label { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.08em; color: #71717a; flex-shrink: 0; }
  .reconcile-bar-detail { font-size: 12px; color: #71717a; flex: 1; }

  /* Sidebar layout */
  .layout { display: flex; min-height: 100vh; }
  .sidebar {
    width: 200px;
    flex-shrink: 0;
    background: #18181b;
    border-right: 1px solid #27272a;
    display: flex;
    flex-direction: column;
    position: sticky;
    top: 0;
    height: 100vh;
    overflow-y: auto;
    transition: width 0.2s ease;
    overflow-x: hidden;
  }
  .sidebar-logo {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 20px 16px;
    border-bottom: 1px solid #27272a;
  }
  .sidebar-logo-text { font-size: 16px; font-weight: 700; color: #fff; letter-spacing: -0.3px; }
  .sidebar-logo-text span { color: #FB326E; }
  .sidebar-nav { flex: 1; padding: 12px 8px; display: flex; flex-direction: column; gap: 2px; }
  .sidebar-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    border-radius: 6px;
    font-size: 13px;
    font-weight: 500;
    color: #a1a1aa;
    cursor: pointer;
    text-decoration: none;
    transition: background 0.15s, color 0.15s;
    border: none;
    background: none;
    width: 100%;
    text-align: left;
  }
  .sidebar-item:hover { background: #27272a; color: #e4e4e7; }
  .sidebar-item.active { background: #27272a; color: #fff; }
  .sidebar-item.active .sidebar-item-icon { color: #FB326E; }
  .sidebar-item-icon { font-size: 15px; width: 20px; text-align: center; }
  .sidebar-sep { height: 1px; background: #27272a; margin: 6px 8px; }
  .sidebar-group-arrow { margin-left: auto; font-size: 11px; color: #52525b; transition: transform 0.15s; }
  .sidebar-subgroup { display: none; flex-direction: column; gap: 2px; padding-left: 4px; }
  .sidebar-subgroup.open { display: flex; }
  .sidebar-subitem { padding-left: 24px !important; }
  .sidebar.collapsed .sidebar-group-arrow { display: none; }
  .sidebar.collapsed .sidebar-subgroup { display: none !important; }
  .sidebar-toggle { flex-shrink: 0; background: none; border: none; cursor: pointer; color: #71717a; font-size: 15px; padding: 2px 5px; border-radius: 4px; transition: color 0.15s, background 0.15s; margin-left: auto; line-height: 1; }
  .sidebar-toggle:hover { color: #e4e4e7; background: #27272a; }
  .sidebar.collapsed { width: 56px; }
  .sidebar.collapsed .sidebar-logo-img { display: none; }
  .sidebar.collapsed .sidebar-logo-text { display: none; }
  .sidebar.collapsed .sidebar-logo { justify-content: center; padding: 16px 0; }
  .sidebar.collapsed .sidebar-toggle { margin-left: 0; }
  .sidebar.collapsed .sidebar-item-label { display: none; }
  .sidebar.collapsed .sidebar-item { justify-content: center; padding: 9px 0; gap: 0; }
  .sidebar.collapsed .sidebar-item-icon { width: auto; }
  .sidebar.collapsed .sidebar-nav { padding: 12px 4px; }
  .sidebar.collapsed .sidebar-footer { padding: 8px 4px; }
  .sidebar-user { display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-bottom: 1px solid #27272a; }
  .sidebar-user-icon { width: 28px; height: 28px; flex-shrink: 0; opacity: 0.45; filter: invert(1); }
  .sidebar-user-text { flex: 1; min-width: 0; }
  .sidebar-user-label { font-size: 10px; color: #52525b; text-transform: uppercase; letter-spacing: 0.06em; white-space: nowrap; }
  .sidebar-user-name { font-size: 12px; font-weight: 500; color: #a1a1aa; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sidebar.collapsed .sidebar-user-text { display: none; }
  .sidebar.collapsed .sidebar-user { justify-content: center; padding: 10px 0; }
  .main-content { flex: 1; min-width: 0; padding-bottom: 36px; }
  .placeholder-page { padding: 64px 24px; text-align: center; color: #52525b; }
  .placeholder-page .placeholder-icon { font-size: 40px; margin-bottom: 16px; }
  .placeholder-page .placeholder-title { font-size: 16px; font-weight: 600; color: #71717a; margin-bottom: 8px; }
  .placeholder-page .placeholder-sub { font-size: 13px; }

  /* Repo CRUD */
  .btn-tooltip { position:relative; }
  .btn-tooltip::after { content:attr(data-tooltip); position:absolute; bottom:calc(100% + 6px); left:50%; transform:translateX(-50%); background:#FB326E; color:#fff; font-size:11px; white-space:nowrap; padding:4px 8px; border-radius:4px; pointer-events:none; opacity:0; transition:opacity 0.15s; z-index:100; }
  .btn-tooltip:hover::after { opacity:1; }
  .info-card-actions { display:flex; gap:6px; margin-top:10px; justify-content:flex-end; }
  .repo-modal-wrap { display:none; position:fixed; z-index:2000; left:0; top:0; width:100%; height:100%; background:rgba(0,0,0,0.7); align-items:center; justify-content:center; }
  .repo-modal-wrap.show { display:flex; }
  .repo-modal-box { background:#27272a; border:1px solid #3f3f46; border-radius:12px; width:480px; max-width:94vw; padding:28px; max-height:90vh; overflow-y:auto; }
  .repo-modal-title { font-size:16px; font-weight:600; color:#fff; margin-bottom:20px; }
  .repo-form-group { margin-bottom:14px; }
  .repo-form-label { display:block; font-size:12px; color:#a1a1aa; margin-bottom:5px; }
  .repo-form-input { width:100%; background:#18181b; border:1px solid #3f3f46; border-radius:6px; color:#e4e4e7; font-size:13px; padding:7px 10px; box-sizing:border-box; transition:border-color 0.15s; }
  .repo-form-input:focus { outline:none; border-color:#FB326E; }
  .repo-form-input:disabled { opacity:0.5; cursor:not-allowed; }
  .repo-token-row { display:flex; align-items:center; gap:8px; margin-bottom:6px; }
  .repo-form-actions { display:flex; justify-content:flex-end; gap:10px; margin-top:22px; }
  .pw-checks { display:flex; flex-direction:column; gap:5px; margin:0 0 16px; }
  .pw-check { display:flex; align-items:center; gap:8px; font-size:12px; color:#52525b; transition:color 0.15s; }
  .pw-check.met { color:#4ade80; }
  .pw-check-icon { font-size:11px; width:12px; flex-shrink:0; }
  .repo-form-error { color:#f87171; font-size:12px; margin-top:8px; }

  /* Cluster detail page */
  .cluster-detail-header-wrap { padding:24px 24px 0; flex-shrink:0; }
  .cluster-detail-header-wrap .header { margin-bottom:0; }
  .cluster-detail-page { display:flex; flex-direction:column; flex:1; min-height:0; overflow:hidden; }
  .cluster-detail-strip { display:flex; align-items:stretch; flex-shrink:0; border-bottom:1px solid #27272a; background:#27272a; overflow-x:auto; }
  .detail-strip-item { display:flex; flex-direction:column; justify-content:center; align-items:flex-start; padding:10px 20px; min-width:80px; }
  .detail-strip-sep { width:1px; background:#3f3f46; flex-shrink:0; }
  .detail-strip-label { font-size:10px; text-transform:uppercase; letter-spacing:0.08em; color:#71717a; margin-bottom:3px; white-space:nowrap; }
  .detail-strip-value { font-size:13px; font-weight:600; color:#e4e4e7; white-space:nowrap; }
  .cluster-detail-tabs-bar { display:flex; gap:0; flex-shrink:0; border-bottom:1px solid #27272a; padding:0 16px; background:#1b1b1d; align-items:center; }
  .cluster-detail-tab { background:none; border:none; color:#a1a1aa; padding:10px 14px; font-size:13px; font-weight:500; cursor:pointer; border-bottom:2px solid transparent; transition:all 0.15s; }
  .cluster-detail-tab:hover { color:#e4e4e7; }
  .cluster-detail-tab.active { color:#FB326E; border-bottom-color:#FB326E; }
  .cluster-detail-body { flex:1; min-height:0; overflow-y:auto; font-family:'SF Mono','Fira Code',monospace; font-size:13px; line-height:1.6; color:#e4e4e7; white-space:pre-wrap; word-break:break-all; }
  .cluster-detail-body.graph-mode { padding:0; overflow:hidden; display:flex; flex-direction:column; white-space:normal; word-break:normal; }
  .cluster-detail-body.mc-live-mode { white-space:normal; word-break:normal; }
  .cluster-detail-body::-webkit-scrollbar { width:8px; }
  .cluster-detail-body::-webkit-scrollbar-track { background:#1b1b1d; }
  .cluster-detail-body::-webkit-scrollbar-thumb { background:#3f3f46; border-radius:4px; }
</style>
</head>
<body>
<div class="loading-overlay" id="loading-overlay">
  <div class="loading-spinner-large"></div>
  <div class="loading-overlay-title"><img src="{{OMNI_LOGO_URI}}" alt="Omni" style="width:24px;height:24px;vertical-align:middle;margin-right:8px;">OmniCD is starting up...</div>
  <div class="loading-overlay-sub">This may take a moment</div>
</div>
<div class="layout">
  <nav class="sidebar" id="sidebar"></nav>
  <div class="main-content">
    <div class="container" id="app"></div>
  </div>
</div>
<div class="refresh-indicator" id="footer"></div>
<div id="modals"></div>
<div id="repo-modal" class="repo-modal-wrap">
  <div class="repo-modal-box">
    <div class="repo-modal-title" id="repo-modal-title">Add Repository</div>
    <div class="repo-form-group">
      <label class="repo-form-label" for="rm-name">Name <span style="color:#f87171">*</span></label>
      <input class="repo-form-input" id="rm-name" type="text" placeholder="my-repo" />
    </div>
    <div class="repo-form-group">
      <label class="repo-form-label" for="rm-url">URL <span style="color:#f87171">*</span></label>
      <input class="repo-form-input" id="rm-url" type="text" placeholder="https://github.com/org/repo.git" />
    </div>
    <div class="repo-form-group">
      <label class="repo-form-label" for="rm-branch">Branch</label>
      <input class="repo-form-input" id="rm-branch" type="text" placeholder="main" />
    </div>
    <div class="repo-form-group">
      <label class="repo-form-label">Access Token</label>
      <div class="repo-token-row">
        <input type="checkbox" id="rm-set-token" />
        <label for="rm-set-token" style="font-size:12px;color:#a1a1aa" id="rm-set-token-label">Set access token</label>
      </div>
      <input class="repo-form-input" id="rm-token" type="password" placeholder="Personal access token" style="display:none" />
      <div style="font-size:11px;color:#71717a;margin-top:4px" id="rm-token-hint"></div>
    </div>
    <div class="repo-form-group">
      <label class="repo-form-label" for="rm-clusters">Clusters Path</label>
      <input class="repo-form-input" id="rm-clusters" type="text" placeholder="clusters" />
    </div>
    <div class="repo-form-group">
      <label class="repo-form-label" for="rm-mc">Machine Classes Path</label>
      <input class="repo-form-input" id="rm-mc" type="text" placeholder="machineclasses" />
    </div>
    <div class="repo-form-error" id="repo-form-error" style="display:none"></div>
    <div class="repo-form-actions">
      <button class="btn-sort btn-primary" onclick="window.__closeRepoModal()">Cancel</button>
      <button class="btn-sort btn-primary" id="repo-save-btn" onclick="window.__saveRepo()">Save</button>
    </div>
  </div>
</div>
<div id="chpw-modal" class="repo-modal-wrap">
  <div class="repo-modal-box">
    <div class="repo-modal-title">Change Password</div>
    <div id="chpw-error" style="color:#f87171;font-size:13px;min-height:18px;margin-bottom:8px;"></div>
    <div class="repo-form-group">
      <label class="repo-form-label">Current Password</label>
      <input class="repo-form-input" id="chpw-current" type="password" placeholder="••••••••" autocomplete="current-password">
    </div>
    <div class="repo-form-group">
      <label class="repo-form-label">New Password</label>
      <input class="repo-form-input" id="chpw-new" type="password" placeholder="••••••••" autocomplete="new-password" oninput="updateChpwChecks(this.value);document.getElementById('chpw-confirm-msg').textContent=document.getElementById('chpw-confirm').value?(document.getElementById('chpw-confirm').value===this.value?'✓ Passwords match':'✗ Passwords do not match'):'';document.getElementById('chpw-confirm-msg').style.color=document.getElementById('chpw-confirm').value===this.value?'#4ade80':'#f87171';">
    </div>
    <div class="pw-checks" style="margin:0 0 12px;">
      <div class="pw-check" id="chpw-chk-len"><span class="pw-check-icon">✗</span>12 characters or more</div>
      <div class="pw-check" id="chpw-chk-upper"><span class="pw-check-icon">✗</span>Uppercase letter</div>
      <div class="pw-check" id="chpw-chk-num"><span class="pw-check-icon">✗</span>Number</div>
      <div class="pw-check" id="chpw-chk-special"><span class="pw-check-icon">✗</span>Special character</div>
    </div>
    <div class="repo-form-group">
      <label class="repo-form-label">Confirm New Password</label>
      <input class="repo-form-input" id="chpw-confirm" type="password" placeholder="••••••••" autocomplete="new-password" oninput="var m=document.getElementById('chpw-confirm-msg');if(!this.value){m.textContent='';return;}var match=this.value===document.getElementById('chpw-new').value;m.style.color=match?'#4ade80':'#f87171';m.textContent=match?'✓ Passwords match':'✗ Passwords do not match';">
      <div id="chpw-confirm-msg" style="font-size:12px;margin-top:6px;min-height:16px;"></div>
    </div>
    <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:8px;">
      <button class="btn-sort btn-primary" onclick="window.__closeChangePassword()">Cancel</button>
      <button class="btn-sort btn-primary" onclick="window.__submitChangePassword()">Change Password</button>
    </div>
  </div>
</div>
<div id="editprofile-modal" class="repo-modal-wrap">
  <div class="repo-modal-box">
    <div class="repo-modal-title">Edit Profile</div>
    <div id="editprofile-error" style="color:#f87171;font-size:13px;min-height:18px;margin-bottom:8px;"></div>
    <div class="repo-form-group">
      <label class="repo-form-label">Username</label>
      <input class="repo-form-input" id="editprofile-email" type="text" placeholder="admin" autocomplete="username" readonly style="opacity:0.5;cursor:not-allowed;">
    </div>
    <div class="repo-form-group">
      <label class="repo-form-label">Display Name</label>
      <input class="repo-form-input" id="editprofile-displayname" type="text" placeholder="Your name" autocomplete="name">
    </div>
    <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:8px;">
      <button class="btn-sort btn-primary" onclick="window.__closeEditProfile()">Cancel</button>
      <button class="btn-sort btn-primary" onclick="window.__submitEditProfile()">Save</button>
    </div>
  </div>
</div>
<div id="edit-oidc-user-modal" class="repo-modal-wrap">
  <div class="repo-modal-box">
    <div class="repo-modal-title">Edit SSO User</div>
    <div id="edit-oidc-user-error" style="color:#f87171;font-size:13px;min-height:18px;margin-bottom:8px;"></div>
    <input type="hidden" id="edit-oidc-user-email">
    <input type="hidden" id="edit-oidc-user-role" value="none">
    <div class="repo-form-group">
      <label class="repo-form-label">Role</label>
      <div style="display:flex;flex-direction:column;gap:8px;margin-top:4px;">
        <div id="oidc-role-opt-admin" onclick="window.__selectOIDCRole('admin')"
          style="display:flex;align-items:center;gap:10px;padding:10px 14px;border-radius:8px;border:1px solid #3f3f46;cursor:pointer;transition:border-color 0.15s,background 0.15s;">
          <div style="width:10px;height:10px;border-radius:50%;background:#FB326E;flex-shrink:0;"></div>
          <div>
            <div style="font-size:13px;font-weight:600;color:#FB326E;">Admin</div>
            <div style="font-size:11px;color:#71717a;">Full access — can change settings and trigger actions</div>
          </div>
        </div>
        <div id="oidc-role-opt-viewer" onclick="window.__selectOIDCRole('viewer')"
          style="display:flex;align-items:center;gap:10px;padding:10px 14px;border-radius:8px;border:1px solid #3f3f46;cursor:pointer;transition:border-color 0.15s,background 0.15s;">
          <div style="width:10px;height:10px;border-radius:50%;background:#22c55e;flex-shrink:0;"></div>
          <div>
            <div style="font-size:13px;font-weight:600;color:#22c55e;">Viewer</div>
            <div style="font-size:11px;color:#71717a;">Read-only access — can view clusters and logs</div>
          </div>
        </div>
        <div id="oidc-role-opt-none" onclick="window.__selectOIDCRole('none')"
          style="display:flex;align-items:center;gap:10px;padding:10px 14px;border-radius:8px;border:1px solid #3f3f46;cursor:pointer;transition:border-color 0.15s,background 0.15s;">
          <div style="width:10px;height:10px;border-radius:50%;background:#71717a;flex-shrink:0;"></div>
          <div>
            <div style="font-size:13px;font-weight:600;color:#71717a;">No Access</div>
            <div style="font-size:11px;color:#71717a;">Cannot log in — redirected to the access denied page</div>
          </div>
        </div>
      </div>
    </div>
    <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:16px;">
      <button class="btn-sort btn-primary" onclick="window.__closeEditOIDCUser()">Cancel</button>
      <button class="btn-sort btn-primary" onclick="window.__submitEditOIDCUser()">Save</button>
    </div>
  </div>
</div>
<div id="delete-oidc-user-modal" class="repo-modal-wrap">
  <div class="repo-modal-box">
    <div class="repo-modal-title">Delete SSO User</div>
    <p style="font-size:13px;color:#a1a1aa;margin-bottom:4px;">Delete SSO User <span id="delete-oidc-user-email-label" style="color:#fff;font-weight:600;"></span>?</p>
    <p style="font-size:13px;color:#71717a;margin-bottom:20px;">They will be removed from the user list and their active session will be invalidated.</p>
    <div id="delete-oidc-user-error" style="color:#f87171;font-size:13px;min-height:18px;margin-bottom:8px;"></div>
    <div style="display:flex;gap:8px;justify-content:flex-end;">
      <button class="btn-sort btn-primary" onclick="window.__closeDeleteOIDCUser()">Cancel</button>
      <button class="btn-sort btn-primary" onclick="window.__confirmDeleteOIDCUser()">Delete</button>
    </div>
  </div>
</div>
<div id="omni-instance-modal" class="repo-modal-wrap">
  <div class="repo-modal-box">
    <div class="repo-modal-title" id="omni-instance-modal-title">Add Omni Instance</div>
    <div class="repo-form-group">
      <label class="repo-form-label" for="oim-endpoint">Endpoint URL <span style="color:#f87171">*</span></label>
      <input class="repo-form-input" id="oim-endpoint" type="text" placeholder="https://your-omni-instance.example.com" />
    </div>
    <div class="repo-form-group">
      <label class="repo-form-label" for="oim-key">Service Account Key</label>
      <input class="repo-form-input" id="oim-key" type="password" placeholder="Paste service account key" />
      <div style="font-size:11px;color:#71717a;margin-top:4px" id="oim-key-hint"></div>
    </div>
    <div id="oim-test-result" style="display:none;margin-top:4px;font-size:12px"></div>
    <div class="repo-form-error" id="oim-form-error" style="display:none"></div>
    <div class="repo-form-actions">
      <button class="btn-sort btn-primary" onclick="window.__closeOmniInstanceModal()">Cancel</button>
      <button class="btn-sort btn-primary" id="oim-test-btn" onclick="window.__testOmniInstance()">Test Connection</button>
      <button class="btn-sort btn-primary" id="oim-save-btn" onclick="window.__saveOmniInstance()">Save</button>
    </div>
  </div>
</div>
<script>
(function() {
  var app      = document.getElementById('app');
  var modalsEl = document.getElementById('modals');
  var footerEl = document.getElementById('footer');
  var appVersion = '{{APP_VERSION}}';
  var loggedInAs = '{{LOGGED_IN_AS}}';
  var authDisabled = {{AUTH_DISABLED}};
  var oidcEnabled = {{OIDC_ENABLED}};
  var userRole = '{{USER_ROLE}}'; // "admin" or "viewer"
  var isAdmin = userRole === 'admin';
  if (footerEl) footerEl.textContent = 'Omni CD ' + appVersion + ' · Real-time updates';
  var state = null;
  var loadingOverlay = document.getElementById('loading-overlay');
  var appLoaded = false; // resolved after first state message confirms server start time
  var autoScroll = true;
  var machineClassPage = 1;
  var machineClassSortAZ = true;
  var mcSearch = '';
  var mcStatusFilters = {};  // { statusKey: true } — empty means "show all"
  var mcRefreshing = false;
  var gitRefreshing = false;
  var reconcileRunning = false;
  var lastHardReconcileAt = 0;  // timestamp of last Sync trigger
  var lastSoftReconcileAt = 0;  // timestamp of last Refresh/git-check trigger
  var clusterPage = 1;
  var clusterSortAZ = true;
  var clusterStatusFilter = null;
  var clusterSyncFilters = {};  // { statusKey: true } — empty means "show all"
  var clusterSearch = '';
  var pageSize = 5;
  var mcPageSize = 10;
  var clusterPageSize = 10;
  var logsModal = false;
  var logsSearch = '';
  var logsLevelFilter = ''; // '' = all, 'INFO', 'WARN', 'ERROR'
  var logsComponentFilter = ''; // '' = all, or a specific label
  var logsOrder = 'oldest'; // 'oldest' or 'newest'
  var currentRoute = window.location.pathname;
  var ws = null;
  var wsReconnectDelay = 1000;
  var wsReconnectTimer = null;

  function ts(d) {
    if (!d) return '-';
    var dt = new Date(d);
    if (isNaN(dt)) return '-';
    return dt.toLocaleTimeString();
  }

  function ago(d) {
    if (!d) return '';
    var dt = new Date(d);
    if (isNaN(dt)) return '';
    var s = Math.floor((Date.now() - dt.getTime()) / 1000);
    if (s < 5) return 'just now';
    if (s < 60) return s + 's ago';
    if (s < 3600) return Math.floor(s / 60) + 'm ago';
    return Math.floor(s / 3600) + 'h ago';
  }

  var currentModal = null;
  var confirmModal = null;
  var confirmInput = '';
  var clusterDetailId = (function() { var m = currentRoute.match(/^\/clusters\/(.+)$/); return m ? decodeURIComponent(m[1]) : null; })();
  var clusterDetailTab = 'graph';
  var clusterDetailTabExplicit = false;

  function showClusterModal(id) {
    if (!state || !state.clusters) return;
    var cluster = state.clusters.find(function(c) { return c.id === id; });
    if (!cluster) return;
    currentModal = {
      id: id,
      fileContent: cluster.fileContent || '',
      liveContent: cluster.liveContent || '',
      diff: cluster.diff || '',
      error: cluster.error || '',
      activeTab: cluster.error ? 'error' : 'graph',
      type: 'cluster'
    };
    render();
  }

  function showMachineClassModal(id) {
    if (!state || !state.machineClasses) return;
    var mc = state.machineClasses.find(function(m) { return m.id === id; });
    if (!mc) return;
    currentModal = {
      id: id,
      fileContent: mc.fileContent || '',
      liveContent: mc.liveContent || '',
      diff: mc.diff || '',
      error: mc.error || '',
      activeTab: mc.error ? 'error' : 'live',
      type: 'machineclass'
    };
    render();
  }

  function setModalTab(tab) {
    if (!currentModal) return;
    currentModal.activeTab = tab;
    // Update only the tab buttons and body content in-place to avoid a full
    // re-render (which would cause the whole page to flicker).
    var tabs = document.querySelectorAll('.drawer-tabs .drawer-tab');
    tabs.forEach(function(btn) {
      var btnTab = btn.getAttribute('onclick').match(/'([^']+)'\)/)[1];
      btn.classList.toggle('active', btnTab === tab);
    });
    var body = document.querySelector('.drawer-body');
    if (body) {
      body.classList.toggle('graph-mode', tab === 'graph');
      var isLiveType = currentModal.type === 'machineclass' || currentModal.type === 'cluster';
      body.classList.toggle('mc-live-mode', tab === 'live' && isLiveType);
      var mcToggle = document.querySelector('.mc-ignored-toggle');
      if (mcToggle) mcToggle.style.display = currentModal.type === 'machineclass' ? '' : 'none';
      if (tab === 'error') {
        body.innerHTML = '<div style="color:#f87171;white-space:pre-wrap;">' + escHtml(currentModal.error) + '</div>';
      } else if (tab === 'live') {
        body.innerHTML = currentModal.type === 'machineclass'
          ? renderMcLiveContent(currentModal.liveContent)
          : (currentModal.type === 'cluster'
            ? renderClusterLiveContent(currentModal.liveContent)
            : (currentModal.liveContent
              ? '<pre style="margin:0;white-space:pre-wrap;">' + escHtml(currentModal.liveContent) + '</pre>'
              : '<div style="color:#71717a;text-align:center;padding:40px;">No live state available</div>'));
      } else if (tab === 'diff') {
        body.innerHTML = currentModal.diff
          ? (currentModal.type === 'machineclass' ? formatMachineClassDiff(currentModal.diff) : '<pre style="margin:0;white-space:pre-wrap;">' + formatDiff(currentModal.diff) + '</pre>')
          : '<div style="color:#71717a;text-align:center;padding:40px;">' + (currentModal.status === 'unmanaged' ? 'No diff \u2014 this cluster has no template in Git.' : 'No diff available') + '</div>';
      } else if (tab === 'graph') {
        body.innerHTML = renderClusterGraph(currentModal);
        var inner = body.querySelector('.cluster-graph-inner');
        if (inner) {
          if (window.__graphTransforms && window.__graphTransforms[inner.id]) {
            restoreGraphTransform(inner.id);
          } else {
            requestAnimationFrame(function() { restoreGraphTransform(inner.id); });
          }
        }
      } else {
        body.innerHTML = '<div style="color:#71717a;text-align:center;padding:40px;">No content available</div>';
      }
      body.scrollTop = 0;
    }
  }

  function closeModal() {
    currentModal = null;
    render();
  }

  function navToCluster(id) {
    window.location.href = '/clusters/' + encodeURIComponent(id);
  }

  function applyDetailPageLayout() {
    var mc = document.querySelector('.main-content');
    var container = document.getElementById('app');
    if (mc) { mc.style.display='flex'; mc.style.flexDirection='column'; mc.style.height='100vh'; mc.style.overflow='hidden'; mc.style.paddingBottom='0'; }
    if (container) { container.style.flex='1'; container.style.minHeight='0'; container.style.display='flex'; container.style.flexDirection='column'; container.style.padding='0'; container.style.overflow='hidden'; }
  }

  function restoreDefaultLayout() {
    var mc = document.querySelector('.main-content');
    var container = document.getElementById('app');
    if (mc) { mc.style.display=''; mc.style.flexDirection=''; mc.style.height=''; mc.style.overflow=''; mc.style.paddingBottom=''; }
    if (container) { container.style.flex=''; container.style.minHeight=''; container.style.display=''; container.style.flexDirection=''; container.style.padding=''; container.style.overflow=''; }
  }

  function renderClusterDetailStrip(cluster, s) {
    function cpBadge(val) {
      if (val === 'ready') return '<span style="color:#4ade80">&#x2713; Ready</span>';
      if (val === 'not-ready') return '<span style="color:#f87171">&#x2717; Not Ready</span>';
      return '<span style="color:#52525b">&#x2014;</span>';
    }
    function statusBadge(val) {
      if (val === 'ok' || val === 'ready') return '<span style="color:#4ade80">&#x2713; OK</span>';
      if (val === 'not-ready') return '<span style="color:#f87171">&#x2717; Not Ready</span>';
      return '<span style="color:#52525b">&#x2014;</span>';
    }
    function syncBadge(status) {
      if (status === 'success' || status === 'applied') return '<span style="color:#4ade80">&#x25cf; Synced</span>';
      if (status === 'outofsync') return '<span style="color:#fb923c">&#x25cf; Out of Sync</span>';
      if (status === 'orphaned')  return '<span style="color:#a78bfa">&#x25cf; Orphaned</span>';
      if (status === 'failed') return '<span style="color:#f87171">&#x25cf; Failed</span>';
      if (status === 'syncing') return '<span style="color:#2dd4bf">&#x25cf; Syncing</span>';
      if (status === 'unmanaged') return '<span style="color:#52525b">Unmanaged</span>';
      return '<span style="color:#52525b">&#x2014;</span>';
    }
    function formatLastSync(t) {
      if (!t || t === '0001-01-01T00:00:00Z' || t === '') return '<span style="color:#71717a">Never</span>';
      var d = new Date(t);
      if (isNaN(d.getTime())) return '<span style="color:#71717a">Never</span>';
      var diff = Date.now() - d.getTime();
      var mins = Math.floor(diff / 60000);
      var hours = Math.floor(mins / 60);
      var days = Math.floor(hours / 24);
      if (days > 0) return '<span style="color:#e4e4e7">' + days + 'd ago</span>';
      if (hours > 0) return '<span style="color:#e4e4e7">' + hours + 'h ago</span>';
      if (mins > 0) return '<span style="color:#e4e4e7">' + mins + 'm ago</span>';
      return '<span style="color:#4ade80">Just now</span>';
    }
    function si(label, value) {
      return '<div class="detail-strip-item"><div class="detail-strip-label">' + label + '</div><div class="detail-strip-value">' + value + '</div></div>';
    }
    var sep = '<div class="detail-strip-sep"></div>';
    var healthy = cluster.machinesHealthy || 0;
    var total = cluster.machinesTotal || 0;
    var machinesVal = total > 0
      ? (healthy === total
          ? '<span style="color:#4ade80">' + healthy + ' / ' + total + '</span>'
          : '<span style="color:#fb923c">' + healthy + ' / ' + total + '</span>')
      : '<span style="color:#52525b">&#x2014;</span>';
    var clusterRepo = null;
    if (cluster.repoName && s && s.repos) {
      clusterRepo = s.repos.find(function(r) { return r.name === cluster.repoName; }) || null;
    }
    if (!clusterRepo && s && s.git) clusterRepo = s.git;
    var repoDisconnected = !!(clusterRepo && clusterRepo.syncError);
    var lastSyncTime = clusterRepo ? clusterRepo.lastSync : null;
    function phaseBadge(phase) {
      if (phase === 'scaling-up')    return '<span style="color:#60a5fa">&#x2191; Scaling Up</span>';
      if (phase === 'scaling-down')  return '<span style="color:#f59e0b">&#x2193; Scaling Down</span>';
      if (phase === 'destroying')    return '<span style="color:#f43f5e">&#x2715; Destroying</span>';
      if (phase === 'reconfiguring') return '<span style="color:#a78bfa">&#x21bb; Reconfiguring</span>';
      if (phase === 'running')       return '<span style="color:#4ade80">&#x25cf; Running</span>';
      return '<span style="color:#52525b">&#x2014;</span>';
    }
    var activePhase = cluster.clusterPhase && cluster.clusterPhase !== 'running' && cluster.clusterPhase !== 'unknown';
    var phaseRow = activePhase ? si('Phase', phaseBadge(cluster.clusterPhase)) + sep : '';
    return phaseRow +
      si('Cluster', cpBadge(cluster.clusterReady)) + sep +
      si('Controlplane', cpBadge(cluster.controlplaneReady)) + sep +
      si('Kubernetes API', cpBadge(cluster.kubernetesApiReady)) + sep +
      si('ETCD', statusBadge(cluster.etcdStatus)) + sep +
      si('Wireguard', statusBadge(cluster.wireGuardStatus)) + sep +
      si('Machines', machinesVal) + sep +
      si('Sync', cluster.status === 'unmanaged'
        ? '<span style="color:#52525b">&#x2014;</span>'
        : repoDisconnected
          ? '<span class="spinner" style="width:10px;height:10px;display:inline-block;vertical-align:middle"></span>'
          : syncBadge(cluster.status)) + sep +
      si('Last Sync', cluster.status === 'unmanaged'
        ? '<span style="color:#52525b">&#x2014;</span>'
        : repoDisconnected
          ? '<span style="color:#f87171">Failed</span>'
          : formatLastSync(lastSyncTime));
  }

  function setClusterDetailTab(tab) {
    clusterDetailTab = tab;
    clusterDetailTabExplicit = true;
    document.querySelectorAll('.cluster-detail-tab').forEach(function(btn) {
      btn.classList.toggle('active', btn.getAttribute('data-tab') === tab);
    });
    var body = document.getElementById('cluster-detail-body');
    if (!body || !state) return;
    var cluster = state.clusters && state.clusters.find(function(c) { return c.id === clusterDetailId; });
    if (!cluster) return;
    var modal = { id: cluster.id, fileContent: cluster.fileContent || '', liveContent: cluster.liveContent || '', diff: cluster.diff || '', error: cluster.error || '', activeTab: tab, type: 'cluster' };
    if (tab === 'graph') {
      body.classList.add('graph-mode');
      body.innerHTML = renderClusterGraph(modal);
      var gid = 'g' + cluster.id.replace(/[^a-zA-Z0-9]/g, '_');
      if (window.__graphTransforms && window.__graphTransforms[gid]) {
        restoreGraphTransform(gid);
      } else {
        requestAnimationFrame(function() { restoreGraphTransform(gid); });
      }
    } else {
      body.classList.remove('graph-mode');
      body.classList.toggle('mc-live-mode', tab === 'live');
      if (tab === 'error') {
        body.innerHTML = '<div style="padding:24px;color:#f87171;white-space:pre-wrap;">' + escHtml(cluster.error || '') + '</div>';
      } else if (tab === 'live') {
        body.innerHTML = renderClusterLiveContent(cluster.liveContent);
      } else if (tab === 'diff') {
        body.innerHTML = cluster.diff
          ? '<pre style="margin:0;padding:24px;white-space:pre-wrap;">' + formatDiff(cluster.diff) + '</pre>'
          : '<div style="color:#71717a;text-align:center;padding:40px;">' + (cluster.status === 'unmanaged' ? 'No diff \u2014 this cluster has no template in Git.' : 'No diff available') + '</div>';
      }
      body.scrollTop = 0;
    }
  }

  function updateClusterDetailInPlace() {
    if (!state || !clusterDetailId) return;
    var cluster = state.clusters && state.clusters.find(function(c) { return c.id === clusterDetailId; });
    if (!cluster) return;
    // Page structure not yet rendered — do a full render first
    if (!document.getElementById('cluster-detail-strip')) { render(); return; }
    var stripEl = document.getElementById('cluster-detail-strip');
    if (stripEl) stripEl.innerHTML = renderClusterDetailStrip(cluster, state);
    var body = document.getElementById('cluster-detail-body');
    if (body) {
      if (clusterDetailTab === 'graph') {
        var modal = { id: cluster.id, fileContent: cluster.fileContent || '', liveContent: cluster.liveContent || '', diff: cluster.diff || '', error: cluster.error || '', activeTab: 'graph', type: 'cluster' };
        var inner = body.querySelector('.cluster-graph-inner');
        var gid = inner ? inner.id : null;
        body.innerHTML = renderClusterGraph(modal);
        // Restore transform synchronously if we have a saved position — avoids
        // the one-frame jump that requestAnimationFrame would cause. Fall back
        // to RAF only for the first render (no saved transform) which needs
        // offsetWidth/offsetHeight for centering.
        if (gid && window.__graphTransforms && window.__graphTransforms[gid]) {
          restoreGraphTransform(gid);
        } else {
          requestAnimationFrame(function() { if (gid) restoreGraphTransform(gid); });
        }
      } else if (clusterDetailTab === 'live') {
        body.classList.add('mc-live-mode');
        body.innerHTML = renderClusterLiveContent(cluster.liveContent);
      } else if (clusterDetailTab === 'diff') {
        body.innerHTML = cluster.diff
          ? '<pre style="margin:0;padding:24px;white-space:pre-wrap;">' + formatDiff(cluster.diff) + '</pre>'
          : '<div style="color:#71717a;text-align:center;padding:40px;">' + (cluster.status === 'unmanaged' ? 'No diff \u2014 this cluster has no template in Git.' : 'No diff available') + '</div>';
      }
    }
  }

  function renderClusterDetailPage(s) {
    var cluster = s.clusters && s.clusters.find(function(c) { return c.id === clusterDetailId; });
    if (!cluster) {
      return '<div style="padding:40px;text-align:center;color:#52525b">Cluster not found: ' + escHtml(clusterDetailId || '') + '</div>';
    }
    var statusText = '', statusColor = '#71717a';
    if (cluster.status === 'unmanaged')                                    { statusText = 'unmanaged';     statusColor = '#52525b'; }
    else if (cluster.status === 'outofsync')                               { statusText = '● out of sync'; statusColor = '#fb923c'; }
    else if (cluster.status === 'orphaned')                                { statusText = '● orphaned';    statusColor = '#a78bfa'; }
    else if (cluster.status === 'failed')                                  { statusText = '● failed';      statusColor = '#f87171'; }
    else if (cluster.status === 'syncing')                                 { statusText = '● syncing';     statusColor = '#2dd4bf'; }
    else if (cluster.status === 'success' || cluster.status === 'applied') { statusText = '● synced';      statusColor = '#4ade80'; }
    var isRunning = s.lastReconcile && s.lastReconcile.status === 'running';
    var hasError = cluster.error && cluster.error.length > 0;
    var tabs = ['graph', 'live', 'diff'];
    if (hasError) tabs.unshift('error');
    var defaultTab = hasError ? 'error' : 'graph';
    if (!clusterDetailTabExplicit || tabs.indexOf(clusterDetailTab) < 0) clusterDetailTab = defaultTab;
    var modal = { id: cluster.id, fileContent: cluster.fileContent || '', liveContent: cluster.liveContent || '', diff: cluster.diff || '', error: cluster.error || '', activeTab: clusterDetailTab, type: 'cluster' };
    var tabLabels = { error: 'Error', graph: 'Graph', live: 'Live', diff: 'Diff' };
    var tabsHtml = tabs.map(function(t) {
      return '<button class="cluster-detail-tab' + (clusterDetailTab === t ? ' active' : '') + '" data-tab="' + t + '" onclick="window.__setClusterDetailTab(\'' + t + '\')">' + (tabLabels[t] || t) + '</button>';
    }).join('');
    var bodyClass = 'cluster-detail-body' + (clusterDetailTab === 'graph' ? ' graph-mode' : '') + (clusterDetailTab === 'live' ? ' mc-live-mode' : '');
    var bodyContent;
    if (clusterDetailTab === 'graph') {
      bodyContent = renderClusterGraph(modal);
    } else if (clusterDetailTab === 'error') {
      bodyContent = '<div style="padding:24px;color:#f87171;white-space:pre-wrap;">' + escHtml(cluster.error || '') + '</div>';
    } else if (clusterDetailTab === 'live') {
      bodyContent = renderClusterLiveContent(cluster.liveContent);
    } else {
      bodyContent = cluster.diff
        ? '<pre style="margin:0;padding:24px;white-space:pre-wrap;">' + formatDiff(cluster.diff) + '</pre>'
        : '<div style="color:#71717a;text-align:center;padding:40px;">' + (cluster.status === 'unmanaged' ? 'No diff — this cluster has no template in Git.' : 'No diff available') + '</div>';
    }
    var clusterActions = '<div class="cluster-card-actions" style="padding:10px 0 10px 24px">' +
      (isAdmin ? '<button class="btn-sort btn-primary" onclick="window.__refreshCurrentCluster()">&#8635; Refresh</button>' : '') +
      (isAdmin && cluster.status !== 'deleting' && (cluster.status === 'unmanaged' || cluster.status === 'orphaned')
        ? '<button class="btn-sort btn-primary" onclick="window.__exportCluster(\'' + cluster.id + '\', event)">&#8595; Export</button>'
        : '') +
      (isAdmin && cluster.status !== 'deleting' && cluster.status !== 'unmanaged' && cluster.status !== 'orphaned'
        ? '<button class="btn-sort btn-primary" onclick="window.__syncCluster(\'' + cluster.id + '\', event)">&#8645; Sync</button>'
        : '') +
      (isAdmin && cluster.status !== 'deleting' && cluster.status !== 'unmanaged' && cluster.status !== 'orphaned'
        ? '<button class="btn-sort btn-primary auto-sync ' + (cluster.autoSync === false ? '' : 'active') + '" onclick="window.__setClusterAutoSync(\'' + cluster.id + '\', ' + (cluster.autoSync === false ? 'true' : 'false') + ', event)">' + (cluster.autoSync === false ? '○ Auto-Sync: Off' : '● Auto-Sync: On') + '</button>'
        : '') +
      (isAdmin && cluster.status !== 'deleting' && cluster.status !== 'unmanaged'
        ? '<button class="btn-sort btn-primary" onclick="window.__deleteCluster(\'' + cluster.id + '\', event)">&#10005; Delete</button>'
        : '') +
    '</div>';
    return '<div style="display:flex;flex-direction:column;height:100%;overflow:hidden;">' +
      '<div class="cluster-detail-header-wrap">' + renderHeader(s, cluster) + '</div>' +
      '<div class="cluster-detail-page">' +
        clusterActions +
        '<div class="cluster-detail-strip" id="cluster-detail-strip">' + renderClusterDetailStrip(cluster, s) + '</div>' +
        '<div class="cluster-detail-tabs-bar">' + tabsHtml + '</div>' +
        '<div class="' + bodyClass + '" id="cluster-detail-body">' + bodyContent + '</div>' +
      '</div>' +
    '</div>';
  }

  async function refreshCurrentCluster() {
    if (!clusterDetailId) return;
    try {
      var r = await fetch('/api/refresh-cluster', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: clusterDetailId })
      });
      var d = await r.json();
      if (d.status === 'already running') alert('Refresh already in progress');
    } catch(e) {
      alert('Failed to refresh cluster');
    }
  }

  function formatDiff(raw) {
    if (!raw) return '';
    var text = raw.replace(/\\n/g, '\n');
    return text.split('\n').map(function(line) {
      if (line.startsWith('+')) return '<span class="diff-add">' + escHtml(line) + '</span>';
      if (line.startsWith('-')) return '<span class="diff-del">' + escHtml(line) + '</span>';
      if (line.startsWith('@@') || line.startsWith('---') || line.startsWith('+++')) return '<span class="diff-hdr">' + escHtml(line) + '</span>';
      return escHtml(line);
    }).join('\n');
  }

  function formatMachineClassDiff(raw) {
    if (!raw) return '';
    var text = raw.replace(/\\n/g, '\n');
    var lines = text.split('\n');
    var blocks = [];
    var cur = null;
    var inLive = false, inDesired = false;
    for (var i = 0; i < lines.length; i++) {
      var line = lines[i];
      if (line.startsWith('~ changed:') || line.startsWith('+ new:')) {
        if (cur) blocks.push(cur);
        cur = { header: line, isNew: line.startsWith('+ new:'), live: [], desired: [] };
        inLive = false; inDesired = false;
      } else if (cur && line === '--- live') {
        inLive = true; inDesired = false;
      } else if (cur && line === '+++ desired') {
        inLive = false; inDesired = true;
      } else if (cur && inLive) {
        cur.live.push(line);
      } else if (cur && inDesired) {
        cur.desired.push(line);
      }
    }
    if (cur) blocks.push(cur);
    if (blocks.length === 0) return '<pre style="margin:0;white-space:pre-wrap;">' + formatDiff(raw) + '</pre>';
    var CONTEXT = 2;
    var html = '';
    blocks.forEach(function(block) {
      var cells = '';
      if (block.isNew) {
        if (block.desired.length > 0) {
          // Two-column layout: left empty, right shows desired content with a + marker per line
          // Runtime-only metadata fields are hidden when "Show Ignored Fields" is unchecked
          var runtimeField = /^\s*(version|owner|phase|created|updated):/;
          var lineNum = 0;
          for (var i = 0; i < block.desired.length; i++) {
            var isIgnored = runtimeField.test(block.desired[i]);
            if (isIgnored && !mcLiveShowMeta) continue;
            lineNum++;
            var ln = lineNum + '.';
            var rCls = 'sbs-cell' + (isIgnored ? ' sbs-ignored' : '');
            cells += '<div class="sbs-cell"><span class="sbs-ln">' + ln + '</span></div>';
            cells += '<div class="' + rCls + '"><span class="sbs-ln">' + ln + '</span><span style="color:#4ade80;margin-right:6px;">+</span>' + escHtml(block.desired[i]) + '</div>';
          }
        }
      } else if (block.live.length > 0 || block.desired.length > 0) {
        var maxLen = Math.max(block.live.length, block.desired.length);
        // Collect changed line indices
        var changedIdx = [];
        for (var i = 0; i < maxLen; i++) {
          var l = block.live[i] !== undefined ? block.live[i] : '';
          var d = block.desired[i] !== undefined ? block.desired[i] : '';
          if (l !== d) changedIdx.push(i);
        }
        // Build set of lines to show (changed ± CONTEXT)
        var toShow = {};
        for (var k = 0; k < changedIdx.length; k++) {
          for (var j = Math.max(0, changedIdx[k] - CONTEXT); j <= Math.min(maxLen - 1, changedIdx[k] + CONTEXT); j++) {
            toShow[j] = true;
          }
        }
        var shown = Object.keys(toShow).map(Number).sort(function(a, b) { return a - b; });
        var prev = -1;
        for (var s = 0; s < shown.length; s++) {
          var i = shown[s];
          if (prev >= 0 && i > prev + 1) {
            cells += '<div class="sbs-cell sbs-ignored" style="grid-column:1/-1;border-right:none;text-align:center;">...</div>';
          }
          var lLine = block.live[i] !== undefined ? block.live[i] : '';
          var dLine = block.desired[i] !== undefined ? block.desired[i] : '';
          var isChanged = lLine !== dLine;
          var ln = (i + 1) + '.';
          cells += '<div class="sbs-cell' + (isChanged ? ' sbs-del' : '') + '"><span class="sbs-ln">' + ln + '</span>' + escHtml(lLine) + '</div>';
          cells += '<div class="sbs-cell' + (isChanged ? ' sbs-add' : '') + '"><span class="sbs-ln">' + ln + '</span>' + escHtml(dLine) + '</div>';
          prev = i;
        }
      }
      if (cells) {
        html += '<div class="sbs-block"><div class="sbs-table" style="border-radius:4px;">' + cells + '</div></div>';
      }
    });
    return html;
  }

  function escHtml(s) {
    return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
  }

  var mcLiveShowMeta = false;

  function toggleMcLiveMeta() {
    mcLiveShowMeta = !mcLiveShowMeta;
    // Sync all visible checkboxes (drawer + cluster detail page)
    document.querySelectorAll('.mc-ignored-cb').forEach(function(cb) { cb.checked = mcLiveShowMeta; });
    // Drawer re-render (machineclass only)
    if (currentModal && currentModal.type === 'machineclass') {
      var drawerBody = document.querySelector('.drawer-body');
      if (drawerBody) {
        if (currentModal.activeTab === 'live') {
          drawerBody.innerHTML = renderMcLiveContent(currentModal.liveContent);
        } else if (currentModal.activeTab === 'diff' && currentModal.diff) {
          drawerBody.innerHTML = formatMachineClassDiff(currentModal.diff);
        }
      }
    }
  }

  // buildMcRows returns an array of row objects from a YAML content string,
  // applying metadata hiding. Each row: {lineNum, content, isPlaceholder, isDim}
  function buildMcRows(content) {
    var text = (content || '').replace(/\\n/g, '\n');
    var lines = text.split('\n');
    var metaStart = -1, metaEnd = lines.length;
    for (var i = 0; i < lines.length; i++) {
      if (/^metadata:/.test(lines[i])) { metaStart = i; break; }
    }
    if (metaStart >= 0) {
      for (var j = metaStart + 1; j < lines.length; j++) {
        if (lines[j].length > 0 && !/^\s/.test(lines[j])) { metaEnd = j; break; }
      }
    }
    var ALWAYS_SHOW_META = /^\s+(namespace|id|type):/;
    var rows = [];
    var hiddenCount = 0;
    for (var i = 0; i < lines.length; i++) {
      var isMeta = metaStart >= 0 && i >= metaStart && i < metaEnd;
      var isAlwaysShown = i === metaStart || ALWAYS_SHOW_META.test(lines[i]);
      if (isMeta && !isAlwaysShown && !mcLiveShowMeta) {
        hiddenCount++;
        continue;
      }
      if (hiddenCount > 0) {
        rows.push({ lineNum: null, content: '(' + hiddenCount + ' fields hidden)', isPlaceholder: true, isDim: false });
        hiddenCount = 0;
      }
      rows.push({ lineNum: i + 1, content: lines[i], isPlaceholder: false, isDim: isMeta && !isAlwaysShown });
    }
    if (hiddenCount > 0) {
      rows.push({ lineNum: null, content: '(' + hiddenCount + ' fields hidden)', isPlaceholder: true, isDim: false });
    }
    return rows;
  }

  function renderMcLiveContent(content) {
    if (!content) return '<div style="color:#71717a;text-align:center;padding:40px;">No live state available</div>';
    var rows = buildMcRows(content);
    var html = '';
    for (var i = 0; i < rows.length; i++) {
      var r = rows[i];
      var cls = 'sbs-cell' + (r.isPlaceholder ? ' sbs-ignored' : r.isDim ? ' sbs-meta-dim' : '');
      html += '<div class="' + cls + '"><span class="sbs-ln">' + (r.lineNum ? r.lineNum + '.' : '') + '</span>' + escHtml(r.content) + '</div>';
    }
    return '<div style="padding:24px;"><div class="sbs-table-single">' + html + '</div></div>';
  }

  function renderClusterLiveContent(content) {
    if (!content) return '<div style="color:#71717a;text-align:center;padding:40px;">No live state available</div>';
    var lines = (content || '').replace(/\\n/g, '\n').split('\n');
    var html = '';
    for (var i = 0; i < lines.length; i++) {
      html += '<div class="sbs-cell"><span class="sbs-ln">' + (i+1) + '.</span>' + escHtml(lines[i]) + '</div>';
    }
    return '<div style="padding:24px;"><div class="sbs-table-single">' + html + '</div></div>';
  }

  function renderMcDiffFull(liveContent, fileContent) {
    var lRows = buildMcRows(liveContent);
    var rRows = buildMcRows(fileContent);
    // Align: when one side has a placeholder, pair it with an empty slot on the other
    var lAligned = [], rAligned = [];
    var li = 0, ri = 0;
    while (li < lRows.length || ri < rRows.length) {
      var l = lRows[li], r = rRows[ri];
      if (l && l.isPlaceholder) {
        lAligned.push(l); rAligned.push(null); li++;
      } else if (r && r.isPlaceholder) {
        lAligned.push(null); rAligned.push(r); ri++;
      } else {
        lAligned.push(l || null); rAligned.push(r || null);
        if (l) li++; if (r) ri++;
      }
    }
    var cells = '';
    for (var i = 0; i < lAligned.length; i++) {
      var l = lAligned[i], r = rAligned[i];
      var lc = l ? l.content : '', rc = r ? r.content : '';
      var changed = l && r && !l.isPlaceholder && !r.isPlaceholder && lc !== rc;
      var lCls = 'sbs-cell' + (l && l.isPlaceholder ? ' sbs-ignored' : l && l.isDim ? ' sbs-meta-dim' : '') + (changed ? ' sbs-del' : '');
      var rCls = 'sbs-cell' + (r && r.isPlaceholder ? ' sbs-ignored' : r && r.isDim ? ' sbs-meta-dim' : '') + (changed ? ' sbs-add' : '');
      cells += '<div class="' + lCls + '"><span class="sbs-ln">' + (l && l.lineNum ? l.lineNum + '.' : '') + '</span>' + escHtml(lc) + '</div>';
      cells += '<div class="' + rCls + '"><span class="sbs-ln">' + (r && r.lineNum ? r.lineNum + '.' : '') + '</span>' + escHtml(rc) + '</div>';
    }
    return '<div style="padding:16px;"><div class="sbs-table" style="border-radius:4px;">' + cells + '</div></div>';
  }

  window.__showClusterModal = showClusterModal;
  window.__setModalTab = setModalTab;
  window.__toggleMcLiveMeta = toggleMcLiveMeta;
  window.__closeModal = closeModal;

  // Pan + zoom via CSS transform: translate(tx,ty) scale(z). No scroll involved.
  // Also persists position to __graphTransforms so it survives DOM replacement.
  if (!window.__graphTransforms) window.__graphTransforms = {};
  function applyGraphTransform(inner, tx, ty, z) {
    inner.dataset.tx   = tx;
    inner.dataset.ty   = ty;
    inner.dataset.zoom = z;
    if (inner.id) window.__graphTransforms[inner.id] = {tx: tx, ty: ty, z: z};
    inner.style.transform = 'translate(' + tx.toFixed(1) + 'px,' + ty.toFixed(1) + 'px) scale(' + z + ')';
    var lvl = inner.closest('.cluster-graph').querySelector('.graph-zoom-level');
    if (lvl) lvl.textContent = Math.round(z * 100) + '%';
  }
  // Centre the graph inner element within its canvas at scale 1.
  window.__graphCentre = function(canvasEl) {
    var inner = canvasEl.querySelector('.cluster-graph-inner');
    if (!inner) return;
    var cw = canvasEl.offsetWidth;
    var ch = canvasEl.offsetHeight;
    var iw = inner.offsetWidth;
    var ih = inner.offsetHeight;
    var tx = Math.max(0, (cw - iw) / 2);
    var ty = Math.max(0, (ch - ih) / 2);
    applyGraphTransform(inner, tx, ty, 1);
  };
  window.__graphZoom = function(dir, id) {
    var el = document.getElementById(id);
    if (!el) return;
    var z  = parseFloat(el.dataset.zoom || '1');
    var tx = parseFloat(el.dataset.tx   || '0');
    var ty = parseFloat(el.dataset.ty   || '0');
    if (dir === 'in')       z = Math.min(2.5, +(z + 0.15).toFixed(2));
    else if (dir === 'out') z = Math.max(0.25, +(z - 0.15).toFixed(2));
    else { window.__graphCentre(el.closest('.cluster-graph-canvas') || el.parentElement); return; }
    applyGraphTransform(el, tx, ty, z);
  };
  // Zoom toward the cursor so the point under it stays fixed.
  window.__graphZoomWheel = function(e, canvasEl) {
    e.preventDefault();
    var inner = canvasEl.querySelector('.cluster-graph-inner');
    if (!inner) return;
    var z    = parseFloat(inner.dataset.zoom || '1');
    var tx   = parseFloat(inner.dataset.tx   || '0');
    var ty   = parseFloat(inner.dataset.ty   || '0');
    var newZ = Math.min(2.5, Math.max(0.25, +(z + (e.deltaY < 0 ? 0.08 : -0.08)).toFixed(2)));
    var rect = canvasEl.getBoundingClientRect();
    var mx   = e.clientX - rect.left;
    var my   = e.clientY - rect.top;
    applyGraphTransform(inner, mx - (mx - tx) / z * newZ, my - (my - ty) / z * newZ, newZ);
  };
  window.__graphDragStart = function(e, canvasEl) {
    if (e.button !== 0) return;
    e.preventDefault();
    var inner = canvasEl.querySelector('.cluster-graph-inner');
    if (!inner) return;
    canvasEl._drag = {
      x: e.clientX, y: e.clientY,
      tx: parseFloat(inner.dataset.tx || '0'),
      ty: parseFloat(inner.dataset.ty || '0'),
      inner: inner
    };
    canvasEl.style.cursor = 'grabbing';
  };
  window.__graphDragMove = function(e, canvasEl) {
    if (!canvasEl._drag) return;
    applyGraphTransform(
      canvasEl._drag.inner,
      canvasEl._drag.tx + (e.clientX - canvasEl._drag.x),
      canvasEl._drag.ty + (e.clientY - canvasEl._drag.y),
      parseFloat(canvasEl._drag.inner.dataset.zoom || '1')
    );
  };
  window.__graphDragEnd = function(canvasEl) {
    canvasEl._drag = null;
    canvasEl.style.cursor = 'grab';
  };
  // Persisted graph transforms keyed by graphId — survive DOM replacement.
  function restoreGraphTransform(graphId) {
    var el = document.getElementById(graphId);
    if (!el) return;
    var t = window.__graphTransforms[graphId];
    if (t) {
      applyGraphTransform(el, t.tx, t.ty, t.z);
    } else {
      var canvas = el.closest('.cluster-graph-canvas');
      if (canvas) window.__graphCentre(canvas);
    }
  }
  // Toggle column-level fold (collapses everything to the right of an edge).
  if (!window.__graphColFolded) window.__graphColFolded = {};
  function reRenderGraph() {
    if (clusterDetailId && currentRoute.startsWith('/clusters/')) {
      var body = document.getElementById('cluster-detail-body');
      if (!body || !body.classList.contains('graph-mode')) return;
      var cluster = state && state.clusters && state.clusters.find(function(c) { return c.id === clusterDetailId; });
      if (!cluster) return;
      var modal = { id: cluster.id, fileContent: cluster.fileContent || '', liveContent: cluster.liveContent || '', diff: cluster.diff || '', error: cluster.error || '', activeTab: 'graph', type: 'cluster' };
      var inner = body.querySelector('.cluster-graph-inner');
      var gid = inner ? inner.id : null;
      body.innerHTML = renderClusterGraph(modal);
      if (gid && window.__graphTransforms && window.__graphTransforms[gid]) {
        restoreGraphTransform(gid);
      } else {
        requestAnimationFrame(function() { if (gid) restoreGraphTransform(gid); });
      }
      return;
    }
    var body = document.querySelector('.drawer-body');
    if (!body || !body.classList.contains('graph-mode')) return;
    var inner = body.querySelector('.cluster-graph-inner');
    var gid = inner ? inner.id : null;
    body.innerHTML = renderClusterGraph(currentModal);
    if (gid && window.__graphTransforms && window.__graphTransforms[gid]) {
      restoreGraphTransform(gid);
    } else {
      requestAnimationFrame(function() { if (gid) restoreGraphTransform(gid); });
    }
  }
  window.__graphToggleColFold = function(key) {
    window.__graphColFolded[key] = !window.__graphColFolded[key];
    reRenderGraph();
  };
  window.__graphCollapseAll = function(graphId) {
    var inner = document.getElementById(graphId);
    if (!inner) return;
    inner.querySelectorAll('.dag-node.clickable').forEach(function(el) {
      var m = (el.getAttribute('onclick') || '').match(/__graphToggleColFold\('([^']+)'\)/);
      if (m) window.__graphColFolded[m[1]] = true;
    });
    reRenderGraph();
  };
  window.__graphExpandAll = function(graphId) {
    Object.keys(window.__graphColFolded).forEach(function(k) {
      if (k.indexOf(graphId + ':') === 0) window.__graphColFolded[k] = false;
    });
    reRenderGraph();
  };

  function badgeClass(st) {
    if (!st) return 'badge-idle';
    if (st === 'success' || st === 'applied' || st === 'synced') return 'badge-success';
    if (st === 'running') return 'badge-running';
    if (st === 'failed') return 'badge-failed';
    if (st === 'outofsync' || st === 'out of sync') return 'badge-outofsync';
    if (st === 'orphaned') return 'badge-orphaned';
    if (st === 'unmanaged') return 'badge-unmanaged';
    if (st === 'syncing') return 'badge-syncing';
    if (st === 'deleting') return 'badge-deleting';
    return 'badge-idle';
  }

  function getOmniHealth(s) {
    if (!s || !s.omniHealth || !s.omniHealth.lastCheck) return { status: 'unknown', label: 'Unknown' };
    if (s.omniHealth.status === 'healthy') return { status: 'healthy', label: 'Healthy' };
    if (s.omniHealth.status === 'failed') return { status: 'failed', label: 'Unreachable' };
    return { status: 'unknown', label: 'Unknown' };
  }

  function getRepoHealth(gitInfo, s) {
    if (!gitInfo) return { status: 'unknown', label: 'Unknown' };
    if (gitInfo.syncError) return { status: 'connecting', label: 'Failed' };
    if (!gitInfo.sha || !gitInfo.lastSync) return { status: 'disconnected', label: 'Disconnected' };
    var lastSync = new Date(gitInfo.lastSync);
    var minutesSinceSync = Math.floor((Date.now() - lastSync.getTime()) / 1000 / 60);
    if (minutesSinceSync > 10) return { status: 'stale', label: 'Stale' };
    return { status: 'healthy', label: 'Healthy' };
  }

  function getGitHealth(s) {
    var repos = (s && s.repos && s.repos.length > 0) ? s.repos : (s && s.git ? [s.git] : []);
    if (repos.length === 0) return { status: 'unknown', label: 'Unknown' };

    var priority = { 'unknown': 4, 'disconnected': 3, 'connecting': 3, 'stale': 2, 'degraded': 2, 'healthy': 0 };
    var worst = { status: 'healthy', label: 'Healthy' };
    for (var i = 0; i < repos.length; i++) {
      var h = getRepoHealth(repos[i], s);
      if ((priority[h.status] || 0) > (priority[worst.status] || 0)) {
        worst = h;
      }
    }
    return worst;
  }

  function gitHealthBadgeClass(status) {
    if (status === 'healthy') return 'badge-success';
    if (status === 'degraded') return 'badge-outofsync';
    if (status === 'stale') return 'badge-outofsync';
    if (status === 'disconnected') return 'badge-failed';
    if (status === 'connecting') return 'badge-connecting';
    return 'badge-idle';
  }

  function logClass(level) {
    if (level === 'DEBUG') return 'log-debug';
    if (level === 'WARN') return 'log-warn';
    if (level === 'ERROR') return 'log-error';
    return 'log-info';
  }

  function hideOverlay(animate) {
    if (loadingOverlay) {
      if (animate) {
        loadingOverlay.classList.add('hidden');
        setTimeout(function() { loadingOverlay.style.display = 'none'; }, 350);
      } else {
        loadingOverlay.style.display = 'none';
      }
    }
  }

  function markAppLoaded() {
    if (appLoaded) return;
    if (!state) return;
    // If the server restarted (new serverStartedAt), require reconcile to finish.
    var serverStart = state.serverStartedAt || '';
    var knownStart  = localStorage.getItem('omniCdServerStart');
    if (serverStart !== knownStart) {
      // New server session — wait for reconcile to complete, or show the UI
      // immediately if the app is idle (Omni not yet configured, waiting for UI input).
      var status = state.lastReconcile && state.lastReconcile.status;
      if (status !== 'success' && status !== 'failed' && status !== 'idle') return;
      localStorage.setItem('omniCdServerStart', serverStart);
      hideOverlay(true);
    } else {
      // Same server session as last page load — skip overlay immediately.
      hideOverlay(false);
    }
    appLoaded = true;
  }

  async function fetchState() {
    try {
      var r = await fetch('/api/state');
      state = await r.json();
      markAppLoaded();
      // Don't re-render if modal is open to prevent flashing
      if (!currentModal && !confirmModal) {
        render();
      }
    } catch(e) {}
  }

  async function checkGit() {
    gitRefreshing = true;
    lastSoftReconcileAt = Date.now();
    renderMainOnly();
    try {
      var r = await fetch('/api/check', { method: 'POST' });
      var d = await r.json();
      if (d.status === 'already running') alert('Reconcile already in progress');
      await fetchState();
    } catch(e) {
      alert('Failed to trigger git check');
    } finally {
      gitRefreshing = false;
      renderMainOnly();
    }
  }

  async function refreshMC() {
    mcRefreshing = true;
    lastSoftReconcileAt = Date.now();
    renderMainOnly();
    try {
      var r = await fetch('/api/refresh-mc', { method: 'POST' });
      var d = await r.json();
      if (d.status === 'already running') alert('Refresh already in progress');
      await fetchState();
    } catch(e) {
      alert('Failed to trigger machine class refresh');
    } finally {
      mcRefreshing = false;
      renderMainOnly();
    }
  }

  async function triggerReconcile() {
    reconcileRunning = true;
    lastHardReconcileAt = Date.now();
    renderMainOnly();
    try {
      var r = await fetch('/api/reconcile', { method: 'POST' });
      var d = await r.json();
      if (d.status === 'already running') alert('Reconcile already in progress');
      await fetchState();
    } catch(e) {
      alert('Failed to trigger reconcile');
    } finally {
      reconcileRunning = false;
      renderMainOnly();
    }
  }

  async function setClusterAutoSync(clusterId, enabled, event) {
    if (event) event.stopPropagation();
    try {
      await fetch('/api/set-cluster-autosync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: clusterId, autoSync: enabled })
      });
      fetchState();
    } catch(e) {
      alert('Failed to update auto sync setting');
    }
  }

  async function refreshSingleMC(mcId, event) {
    if (event) event.stopPropagation();
    try {
      var r = await fetch('/api/refresh-single-mc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: mcId })
      });
      var d = await r.json();
      if (d.status === 'already running') alert('Refresh already in progress');
      fetchState();
    } catch(e) {
      console.error('refresh single MC failed', e);
    }
  }

  async function syncMachineClass(mcId, event) {
    if (event) event.stopPropagation();
    try {
      await fetch('/api/sync-machineclass', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: mcId })
      });
      // Trigger a reconcile so the force-sync is picked up immediately.
      await fetch('/api/reconcile', { method: 'POST' });
    } catch(e) {
      console.error('sync machineclass failed', e);
    }
  }

  async function setMCAutoSync(mcId, enabled, event) {
    if (event) event.stopPropagation();
    try {
      await fetch('/api/set-mc-autosync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: mcId, autoSync: enabled })
      });
      fetchState();
    } catch(e) {
      alert('Failed to update auto sync setting');
    }
  }

  function forceSync(clusterId, event) {
    // Prevent event bubbling
    event.stopPropagation();

    // Show confirmation modal
    confirmModal = {
      clusterId: clusterId,
      title: 'Force Sync Cluster',
      message: 'Are you sure you want to force sync cluster "' + clusterId + '"?\n\nThis will immediately sync the cluster with the configuration from Git.',
      onConfirm: function() {
        confirmModal = null;
        render();
        doForceSync(clusterId);
      }
    };
    render();
  }

  async function doForceSync(clusterId) {
    try {
      // First, set the cluster ID to force sync
      await fetch('/api/force-cluster', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: clusterId })
      });

      // Then trigger reconcile
      var r = await fetch('/api/reconcile', { method: 'POST' });
      var d = await r.json();
      if (d.status === 'blocked') {
        alert('Sync blocked: ' + d.reason);
      } else if (d.status === 'already running') {
        alert('Reconcile already in progress');
      } else {
        fetchState();
      }
    } catch(e) {
      alert('Failed to trigger sync');
    }
  }

  async function refreshCluster(clusterId, event) {
    event.stopPropagation();
    try {
      var r = await fetch('/api/refresh-cluster', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: clusterId })
      });
      var d = await r.json();
      if (d.status === 'already running') {
        alert('Refresh already in progress');
      }
    } catch(e) {
      alert('Failed to refresh cluster');
    }
  }

  async function syncCluster(clusterId, event) {
    event.stopPropagation();
    doForceSync(clusterId);
  }

  function deleteCluster(clusterId, event) {
    event.stopPropagation();
    confirmInput = '';
    confirmModal = {
      title: 'Delete Cluster',
      message: 'Are you sure you want to delete the Cluster <b>' + escHtml(clusterId) + '</b>?\n\nDeleting the Cluster will delete all the cluster\'s managed resources, which can be dangerous.\nBe sure you understand the effects of deleting this resource before continuing.',
      requireInput: clusterId,
      inputPrompt: 'Please type \u2018' + clusterId + '\u2019 to confirm the deletion of the cluster',
      onConfirm: function() {
        confirmModal = null;
        confirmInput = '';
        render();
        doDeleteCluster(clusterId);
      }
    };
    render();
  }

  async function doDeleteCluster(clusterId) {
    try {
      await fetch('/api/delete-cluster', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: clusterId })
      });
      fetchState();
    } catch(e) {
      alert('Failed to trigger delete: ' + e.message);
    }
  }

  function closeConfirmModal() {
    confirmModal = null;
    confirmInput = '';
    render();
  }

  function deleteMachineClass(mcId, event) {
    if (event) event.stopPropagation();
    confirmInput = '';
    confirmModal = {
      title: 'Delete Machine Class',
      message: 'Are you sure you want to delete the Machine Class <b>' + escHtml(mcId) + '</b>?\n\nThis will permanently remove it from Omni.',
      requireInput: mcId,
      inputPrompt: 'Please type \u2018' + mcId + '\u2019 to confirm the deletion',
      onConfirm: function() {
        confirmModal = null;
        confirmInput = '';
        render();
        doDeleteMachineClass(mcId);
      }
    };
    render();
  }

  async function doDeleteMachineClass(mcId) {
    try {
      await fetch('/api/delete-machineclass', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: mcId })
      });
      fetchState();
    } catch(e) {
      alert('Failed to trigger delete: ' + e.message);
    }
  }

  function exportMachineClass(mcId, event) {
    if (event) event.stopPropagation();
    var mc = state && state.machineClasses && state.machineClasses.find(function(m) { return m.id === mcId; });
    var content = mc && (mc.liveContent || mc.fileContent);
    if (!content) { alert('No content available to export'); return; }
    var blob = new Blob([content], { type: 'application/x-yaml' });
    var url = window.URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = mcId + '.yaml';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
  }

  async function exportCluster(clusterId, event) {
    // Prevent event bubbling
    event.stopPropagation();

    try {
      var r = await fetch('/api/export-cluster', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: clusterId })
      });

      if (!r.ok) {
        alert('Failed to export cluster: ' + r.statusText);
        return;
      }

      // Get the YAML content
      var yamlContent = await r.text();

      // Create a blob and download link
      var blob = new Blob([yamlContent], { type: 'application/x-yaml' });
      var url = window.URL.createObjectURL(blob);
      var a = document.createElement('a');
      a.href = url;
      a.download = clusterId + '.yaml';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);
    } catch(e) {
      alert('Failed to export cluster: ' + e.message);
    }
  }

  function confirmAction() {
    if (confirmModal && confirmModal.requireInput && confirmInput !== confirmModal.requireInput) return;
    if (confirmModal && confirmModal.onConfirm) {
      confirmModal.onConfirm();
    }
  }

  function changeMachineClassPage(page) {
    machineClassPage = page;
    render();
  }

  function changeClusterPage(page) {
    clusterPage = page;
    render();
  }

  function toggleMachineClassSort() {
    machineClassSortAZ = !machineClassSortAZ;
    machineClassPage = 1;
    render();
  }

  function setMcSearch(val) {
    mcSearch = val.toLowerCase();
    machineClassPage = 1;
    render();
  }

  function toggleMcStatusFilter(key) {
    if (mcStatusFilters[key]) {
      delete mcStatusFilters[key];
    } else {
      mcStatusFilters[key] = true;
    }
    machineClassPage = 1;
    render();
  }

  function toggleClusterSyncFilter(key) {
    if (clusterSyncFilters[key]) {
      delete clusterSyncFilters[key];
    } else {
      clusterSyncFilters[key] = true;
    }
    clusterPage = 1;
    render();
  }

  function toggleClusterSort() {
    clusterSortAZ = !clusterSortAZ;
    clusterPage = 1;
    render();
  }

  function setClusterFilter(status) {
    clusterStatusFilter = (clusterStatusFilter === status) ? null : status;
    clusterPage = 1;
    render();
  }

  function clearClusterFilter() {
    clusterStatusFilter = null;
    clusterPage = 1;
    render();
  }

  function setClusterSearch(val) {
    clusterSearch = val.toLowerCase();
    clusterPage = 1;
    render();
  }

  function showClustersView() {
    window.location.href = '/clusters';
  }

  function hideClustersView() {
    window.location.href = '/clusters';
  }

  var settingsOpen = localStorage.getItem('sidebar-settings-open') === '1';
  var settingsRoutes = (authDisabled || !isAdmin) ? ['/instances', '/repos'] : ['/instances', '/repos', '/users'];

  window.__toggleSettings = function() {
    settingsOpen = !settingsOpen;
    localStorage.setItem('sidebar-settings-open', settingsOpen ? '1' : '0');
    var subgroup = document.getElementById('settings-subgroup');
    var arrow    = document.getElementById('settings-arrow');
    if (subgroup) subgroup.classList.toggle('open', settingsOpen);
    if (arrow)    arrow.textContent = settingsOpen ? '▾' : '▸';
  };

  function renderSidebar() {
    // Auto-expand Settings when on a settings sub-route (read-only, don't mutate settingsOpen).
    var showSettings = settingsOpen || settingsRoutes.indexOf(currentRoute) >= 0;

    var navItems = [
      { label: 'Clusters',        icon: '◈', href: '/clusters' },
      { label: 'Machine Classes', icon: '▦', href: '/machineclasses' },
      null,
      { label: 'Logs',            icon: '≡', href: '/logs' },
    ];
    var html = '<div class="sidebar-logo">' +
      '<img class="logo sidebar-logo-img" src="{{OMNI_LOGO_URI}}" alt="Omni">' +
      '<div class="sidebar-logo-text">Omni <span>CD</span></div>' +
      '<button class="sidebar-toggle" id="sidebar-toggle" onclick="window.__toggleSidebar()" title="Toggle sidebar">‹</button>' +
    '</div>' +
    (loggedInAs ? '<div class="sidebar-user"><img class="sidebar-user-icon" src="{{PROFILE_ICON_URI}}" alt="user"><div class="sidebar-user-text"><div class="sidebar-user-label">Logged in as:</div><div class="sidebar-user-name">' + escHtml(loggedInAs) + '</div></div></div>' : '') +
    '<div class="sidebar-nav">';
    navItems.forEach(function(item) {
      if (!item) { html += '<div class="sidebar-sep"></div>'; return; }
      var isActive = currentRoute === item.href || (item.href !== '/' && currentRoute.startsWith(item.href));
      html += '<a class="sidebar-item' + (isActive ? ' active' : '') + '" href="' + item.href + '" title="' + item.label + '">' +
        '<span class="sidebar-item-icon">' + item.icon + '</span>' +
        '<span class="sidebar-item-label">' + item.label + '</span>' +
      '</a>';
    });

    // Settings collapsible group.
    var settingsActive = settingsRoutes.indexOf(currentRoute) >= 0;
    var instanceLabel = 'Instances' + (state && !state.omniConfigured ? ' <span style="color:#fb923c">●</span>' : '');
    var subItems = [
      { label: instanceLabel, icon: '⬡', href: '/instances' },
      { label: 'Repos',       icon: '⎇', href: '/repos' },
    ];
    if (!authDisabled && isAdmin) subItems.push({ label: 'Users', icon: '◉', href: '/users' });
    html += '<button class="sidebar-item' + (settingsActive ? ' active' : '') + '" onclick="window.__toggleSettings()" title="Settings" style="width:100%;background:none;border:none;cursor:pointer;">' +
      '<span class="sidebar-item-icon">⚙</span>' +
      '<span class="sidebar-item-label">Settings</span>' +
      '<span id="settings-arrow" class="sidebar-group-arrow">' + (showSettings ? '▾' : '▸') + '</span>' +
    '</button>';
    html += '<div id="settings-subgroup" class="sidebar-subgroup' + (showSettings ? ' open' : '') + '">';
    subItems.forEach(function(item) {
      var isActive = currentRoute === item.href;
      html += '<a class="sidebar-item sidebar-subitem' + (isActive ? ' active' : '') + '" href="' + item.href + '" title="' + item.label + '">' +
        '<span class="sidebar-item-icon">' + item.icon + '</span>' +
        '<span class="sidebar-item-label">' + item.label + '</span>' +
      '</a>';
    });
    html += '</div>';

    html += '</div>' +
      '<div class="sidebar-footer">' +
        '<a class="sidebar-item" href="/logout" style="color:#71717a;" title="Sign out">' +
          '<span class="sidebar-item-icon">⏻</span>' +
          '<span class="sidebar-item-label">Sign out</span>' +
        '</a>' +
      '</div>';
    return html;
  }

  function renderLogsView(s) {
    var allLogs = (s.logs && s.logs.length > 0) ? s.logs : [];

    // Collect unique component labels for the dropdown.
    var components = [];
    allLogs.forEach(function(l) {
      if (l.label && components.indexOf(l.label) < 0) components.push(l.label);
    });
    components.sort();

    // Apply filters.
    var search = logsSearch.toLowerCase();
    var filtered = allLogs.filter(function(l) {
      if (logsLevelFilter && l.level !== logsLevelFilter) return false;
      if (logsComponentFilter && l.label !== logsComponentFilter) return false;
      if (search && l.message.toLowerCase().indexOf(search) < 0 && (l.label || '').toLowerCase().indexOf(search) < 0) return false;
      return true;
    });
    if (logsOrder === 'newest') filtered = filtered.slice().reverse();

    var logsHtml = filtered.length > 0
      ? filtered.map(function(l) {
          return '<div class="log-entry">' +
            '<span class="log-ts">' + ts(l.timestamp) + '</span> ' +
            '<span class="' + logClass(l.level) + '" style="font-size:10px;min-width:36px;display:inline-block">' + escHtml(l.level || 'INFO') + '</span> ' +
            (l.label ? '<span class="log-label">[' + escHtml(l.label) + ']</span> ' : '') +
            '<span class="log-msg">' + escHtml(l.message) + '</span>' +
          '</div>';
        }).join('')
      : '<div class="log-entry" style="color:#52525b">' + (allLogs.length > 0 ? 'No logs match the current filters' : 'No logs yet') + '</div>';

    var levels = (s.logLevel === 'DEBUG' ? ['DEBUG'] : []).concat(['INFO', 'WARN', 'ERROR']);
    var levelBtns = levels.map(function(lv) {
      var active = logsLevelFilter === lv ? ' active' : '';
      return '<button class="btn-sort btn-primary' + active + '" onclick="window.__setLogsLevel(\'' + lv + '\')">' + lv + '</button>';
    }).join('');

    var componentOpts = '<option value="">All components</option>' +
      components.map(function(c) {
        return '<option value="' + escHtml(c) + '"' + (logsComponentFilter === c ? ' selected' : '') + '>' + escHtml(c) + '</option>';
      }).join('');
    var componentSelect = '<select onchange="window.__setLogsComponent(this.value)" style="background:#18181b;border:1px solid #3f3f46;border-radius:4px;color:#e4e4e7;font-size:12px;padding:3px 8px;outline:none;font-family:inherit;cursor:pointer">' + componentOpts + '</select>';

    var searchInput = '<input type="text" placeholder="Search logs..." value="' + escHtml(logsSearch) + '" oninput="window.__setLogsSearch(this.value)" style="background:#18181b;border:1px solid #3f3f46;border-radius:4px;color:#e4e4e7;font-size:12px;padding:3px 8px;outline:none;width:180px;font-family:inherit;" />';

    var orderSelect = '<select onchange="window.__setLogsOrder(this.value)" style="background:#18181b;border:1px solid #3f3f46;border-radius:4px;color:#e4e4e7;font-size:12px;padding:3px 8px;outline:none;font-family:inherit;cursor:pointer">' +
      '<option value="oldest"' + (logsOrder === 'oldest' ? ' selected' : '') + '>Oldest first</option>' +
      '<option value="newest"' + (logsOrder === 'newest' ? ' selected' : '') + '>Newest first</option>' +
      '</select>';

    var clearBtn = (logsSearch || logsLevelFilter || logsComponentFilter)
      ? '<button class="btn-sort" onclick="window.__clearLogsFilters()" title="Clear filters">✕ Clear</button>'
      : '';

    var countHint = '<span style="font-size:11px;color:#52525b;margin-left:4px">' + filtered.length + ' / ' + allLogs.length + '</span>';

    var logsHeader = '<div class="header">' +
      '<h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Logs</h1>' +
      '<button class="btn-sort btn-primary" onclick="window.__openLogFilesModal()" style="margin-left:auto">Show Logs</button>' +
      '<button class="btn-sort btn-primary" onclick="window.__downloadTodaysLogs()" style="margin-left:8px">Download Today\'s Logs</button>' +
    '</div>' +
    '<div class="logs-filters">' +
      searchInput +
      componentSelect +
      orderSelect +
      levelBtns +
      clearBtn +
      countHint +
    '</div>';

    return logsHeader +
      '<div class="logs-page" style="height:calc(100vh - 160px);padding:0 0 12px;margin-top:8px">' +
        '<div class="logs-page-body" id="logs-page-container">' +
          logsHtml +
        '</div>' +
      '</div>';
  }

  function renderReposView(s) {
    var repos = (s.repos && s.repos.length > 0) ? s.repos : (s.git ? [s.git] : []);
    // repoConfigs is the single source of truth for which repos exist.
    // repos (live git data) is only used to enrich display info.
    var repoConfigs = (s.repoConfigs || []).slice().sort(function(a, b) { return (a.name || '').localeCompare(b.name || ''); });
    var cardsHtml = '';
    if (repoConfigs.length === 0) {
      cardsHtml = '<div class="placeholder-page">' +
        '<div class="placeholder-icon">⎇</div>' +
        '<div class="placeholder-title">Git Repositories</div>' +
        '<div class="placeholder-sub">No repositories configured — add one using the button above</div>' +
      '</div>';
    } else {
      var cards = repoConfigs.map(function(rc) {
        var name = rc.name || '';
        var r = repos.find(function(x) { return x.name === name; }) || {};
        var repoHealth = r.name ? getRepoHealth(r, s) : { status: 'idle', label: 'Not synced' };
        var url = r.repo || rc.url || '';
        var safeName = name.replace(/\\/g,'\\\\').replace(/'/g,"\\'");
        return '<div class="info-card">' +
          '<div class="info-card-header">' +
            '<span class="info-card-title">' + escHtml(name) + '</span>' +
            '<span class="badge ' + gitHealthBadgeClass(repoHealth.status) + '">' + repoHealth.label + '</span>' +
          '</div>' +
          '<div class="info-card-value">' +
            (url ? '<a href="' + escHtml(url) + '" target="_blank" style="color:#FB326E;text-decoration:none">' + escHtml(url) + '</a>' : '—') +
          '</div>' +
          '<div class="info-card-sub">' +
            'Branch: <b style="color:#a1a1aa">' + escHtml(r.branch || rc.branch || '—') + '</b>' +
            (r.shortSha ? ' &nbsp;&middot;&nbsp; SHA <b style="color:#a1a1aa">' + r.shortSha + '</b>' : '') +
            (rc.hasToken ? ' &nbsp;&middot;&nbsp; <span style="color:#4ade80;font-size:11px">&#128273; token set</span>' : '') + '<br>' +
            (r.commitMessage ? escHtml(r.commitMessage) + '<br>' : '') +
            (r.lastSync ? 'Last sync: ' + ago(r.lastSync) : '<span style="color:#52525b">Never synced</span>') +
          '</div>' +
          (isAdmin ? '<div class="info-card-actions">' +
            '<button class="btn-sort btn-primary" onclick="window.__openRepoModal(\'' + safeName + '\')">Edit</button>' +
            '<button class="btn-sort btn-primary" onclick="window.__deleteRepo(\'' + safeName + '\')">Delete</button>' +
          '</div>' : '') +
        '</div>';
      }).join('');
      cardsHtml = '<div class="info-row">' + cards + '</div>';
    }
    var repoHeader = '<div class="header">' +
      '<h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Repositories</h1>' +
      (isAdmin ? '<button class="btn-sort btn-primary" onclick="window.__openRepoModal(null)" style="margin-left:auto"' + ((!s.omniConfigured && !s.omniEnvLocked) ? ' disabled title="Configure an Omni instance before adding a repository"' : '') + '>Add Repository</button>' : '') +
    '</div>';
    return repoHeader + cardsHtml;
  }

  function renderInstancesView(s) {
    var omniHealth = getOmniHealth(s);
    var isConfigured = s.omniConfigured || s.omniEnvLocked;
    var disabledAttr = s.omniEnvLocked ? ' disabled title="Configured via environment variables"' : '';

    var cardsHtml = '';
    if (isConfigured) {
      cardsHtml = '<div class="info-row">' +
        '<div class="info-card">' +
          '<div class="info-card-header">' +
            '<span class="info-card-title">Omni Instance</span>' +
            '<span class="badge ' + gitHealthBadgeClass(omniHealth.status) + '">' + omniHealth.label + '</span>' +
          '</div>' +
          '<div class="info-card-value">' + (s.omniEndpoint ? '<a href="' + escHtml(s.omniEndpoint) + '" target="_blank" style="color:#FB326E;text-decoration:none">' + escHtml(s.omniEndpoint) + '</a>' : '<span style="color:#71717a">Not configured</span>') + '</div>' +
          '<div class="info-card-sub">' +
            (s.omniConfigured ? 'Version: <b style="color:#a1a1aa">' + escHtml(s.omniVersion || '?') + '</b><br>' : '') +
            (s.omniConfigured ? 'Last check: ' + ago(s.omniHealth && s.omniHealth.lastCheck) : '') +
            (s.omniHealth && s.omniHealth.error ? '<br><span style="color:#f87171">' + escHtml(s.omniHealth.error) + '</span>' : '') +
          '</div>' +
          (isAdmin ? '<div class="info-card-actions">' +
            '<button class="btn-sort btn-primary" onclick="window.__refreshOmniConnection()">Refresh</button>' +
            '<button class="btn-sort btn-primary"' + disabledAttr + ' onclick="window.__openOmniInstanceModal()">Edit</button>' +
            '<button class="btn-sort btn-primary"' + disabledAttr + ' onclick="window.__deleteOmniInstance()">Delete</button>' +
          '</div>' : '') +
        '</div>' +
      '</div>';
    } else {
      cardsHtml = '<div class="placeholder-page">' +
        '<div class="placeholder-icon">⚡</div>' +
        '<div class="placeholder-title">Omni Instance</div>' +
        '<div class="placeholder-sub">No Omni instance configured — add one using the button above</div>' +
      '</div>';
    }

    var addDisabled = isConfigured ? ' disabled' : '';
    var header = '<div class="header">' +
      '<h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Instances</h1>' +
      (isAdmin ? '<button class="btn-sort btn-primary"' + addDisabled + ' onclick="window.__openOmniInstanceModal()" style="margin-left:auto">Add Omni Instance</button>' : '') +
    '</div>';

    return header + cardsHtml;
  }

  function openOmniInstanceModal() {
    var isEdit = state && state.omniConfigured;
    document.getElementById('omni-instance-modal-title').textContent = isEdit ? 'Edit Omni Instance' : 'Add Omni Instance';
    document.getElementById('oim-endpoint').value = (state && state.omniEndpoint) || '';
    document.getElementById('oim-key').value = '';
    document.getElementById('oim-key-hint').textContent = (state && state.omniHasStoredKey) ? 'Stored — leave blank to keep current key' : '';
    document.getElementById('oim-test-result').style.display = 'none';
    document.getElementById('oim-form-error').style.display = 'none';
    document.getElementById('omni-instance-modal').classList.add('show');
  }

  function closeOmniInstanceModal() {
    document.getElementById('omni-instance-modal').classList.remove('show');
  }

  async function saveOmniInstance() {
    var endpoint = document.getElementById('oim-endpoint').value.trim();
    var key = document.getElementById('oim-key').value.trim();
    var errEl = document.getElementById('oim-form-error');
    errEl.style.display = 'none';
    if (!endpoint) { errEl.textContent = 'Endpoint is required'; errEl.style.display = ''; return; }
    if (!key && !(state && state.omniHasStoredKey)) { errEl.textContent = 'Service account key is required'; errEl.style.display = ''; return; }
    var btn = document.getElementById('oim-save-btn');
    btn.disabled = true;
    try {
      var body = { endpoint: endpoint };
      if (key) body.serviceAccountKey = key;
      var r = await fetch('/api/omni-instance', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      });
      var d = await r.json();
      if (!r.ok) {
        errEl.textContent = 'Save failed: ' + (d.error || r.status);
        errEl.style.display = '';
      } else {
        closeOmniInstanceModal();
        fetchState();
      }
    } catch(e) {
      errEl.textContent = 'Network error: ' + e.message;
      errEl.style.display = '';
    } finally {
      btn.disabled = false;
    }
  }

  async function testOmniInstance() {
    var endpoint = document.getElementById('oim-endpoint').value.trim();
    var key = document.getElementById('oim-key').value.trim();
    var resultEl = document.getElementById('oim-test-result');
    if (!endpoint || !key) {
      resultEl.textContent = 'Endpoint and service account key are required for testing';
      resultEl.style.color = '#f87171';
      resultEl.style.display = '';
      return;
    }
    var btn = document.getElementById('oim-test-btn');
    btn.disabled = true;
    resultEl.style.display = 'none';
    try {
      var r = await fetch('/api/omni-instance/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ endpoint: endpoint, serviceAccountKey: key })
      });
      var d = await r.json();
      resultEl.textContent = r.ok ? 'Connection successful' : (d.error || 'Test failed');
      resultEl.style.color = r.ok ? '#4ade80' : '#f87171';
      resultEl.style.display = '';
    } catch(e) {
      resultEl.textContent = 'Network error: ' + e.message;
      resultEl.style.color = '#f87171';
      resultEl.style.display = '';
    } finally {
      btn.disabled = false;
    }
  }

  function deleteOmniInstance() {
    confirmModal = {
      message: 'Delete the Omni instance configuration?',
      confirmText: 'Delete',
      onConfirm: function() {
        confirmModal = null;
        render();
        fetch('/api/omni-instance', { method: 'DELETE' })
          .then(function(r) {
            if (!r.ok) { r.text().then(function(t) { alert('Delete failed: ' + (t || r.status)); }); }
            else { fetchState(); }
          }).catch(function(e) { alert('Network error: ' + e.message); });
      }
    };
    render();
  }

  function refreshOmniConnection() {
    fetch('/api/omni-instance/refresh', { method: 'POST' })
      .then(function(r) { return r.json().then(function(d) { return { ok: r.ok, data: d }; }); })
      .then(function(res) {
        if (!res.ok) { alert('Refresh failed: ' + (res.data.error || 'unknown error')); }
        fetchState();
      }).catch(function(e) { alert('Network error: ' + e.message); });
  }

  var _usersData = null;
  var _oidcUsersLoaded = false;
  var _oidcUsersData = null;

  function renderUsersView(s) {
    var header = '<div class="header">' +
      '<h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Users</h1>' +
    '</div>';

    var content = '';
    if (!_usersData) {
      content = '<div style="color:#71717a;font-size:13px;padding:24px 0;">Loading...</div>';
      fetchUsers();
    } else if (_usersData.length > 0) {
      var u = _usersData[0];
      content =
        '<div style="font-size:13px;font-weight:600;color:#a1a1aa;text-transform:uppercase;letter-spacing:0.06em;margin-bottom:12px;">Local Admin Account</div>' +
        '<div style="display:flex;align-items:center;gap:12px;background:#27272a;border:1px solid #3f3f46;border-radius:10px;padding:14px 16px;max-width:480px;">' +
          '<img src="{{PROFILE_ICON_URI}}" style="width:18px;height:18px;opacity:0.5;filter:invert(1);flex-shrink:0;" alt="">' +
          '<div style="flex:1;min-width:0;">' +
            '<div style="font-size:13px;font-weight:600;color:#fff;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">' + escHtml(u.displayName || u.username) + '</div>' +
          '</div>' +
          '<div style="display:flex;gap:8px;">' +
            '<button class="btn-sort btn-primary" onclick="window.__openEditProfile()">Edit Profile</button>' +
            '<button class="btn-sort btn-primary" onclick="window.__openChangePassword()">Change Password</button>' +
          '</div>' +
        '</div>';
    }

    // SSO users section (shown when any OIDC users have logged in).
    var oidcSection = '';
    if (_oidcUsersData !== null) {
      var oidcRows = _oidcUsersData.length === 0
        ? '<div style="padding:12px 14px;font-size:13px;color:#71717a;">No SSO users have logged in yet.</div>'
        : _oidcUsersData.map(function(u) {
            var roleLabel = u.role === 'admin' ? 'Admin' : u.role === 'viewer' ? 'Viewer' : 'No Access';
            var roleColor = u.role === 'admin' ? '#FB326E' : u.role === 'viewer' ? '#22c55e' : '#71717a';
            return '<div style="display:flex;align-items:center;gap:12px;padding:10px 14px;border-bottom:1px solid #3f3f46;">' +
              '<div style="flex:1;min-width:0;">' +
                '<div style="font-size:13px;font-weight:600;color:#fff;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">' + escHtml(u.displayName || u.email) + '</div>' +
                '<div style="font-size:12px;color:#71717a;">' + escHtml(u.email) + '</div>' +
                '<div style="font-size:11px;color:#52525b;margin-top:2px">Last seen: ' + new Date(u.lastSeen).toLocaleString() + '</div>' +
              '</div>' +
              '<span style="font-size:12px;font-weight:600;color:' + roleColor + ';margin-right:8px;">' + roleLabel + '</span>' +
              '<button class="btn-sort btn-primary" onclick="window.__openEditOIDCUser(\'' + escHtml(u.email).replace(/\\/g,'\\\\').replace(/\'/g,"\\'") + '\', \'' + escHtml(u.role).replace(/\\/g,'\\\\').replace(/\'/g,"\\'") + '\')">Edit Role</button>' +
              '<button class="btn-sort btn-primary" onclick="window.__deleteOIDCUser(\'' + escHtml(u.email).replace(/\\/g,'\\\\').replace(/\'/g,"\\'") + '\')">Delete</button>' +
            '</div>';
          }).join('');
      oidcSection = '<div style="margin-top:28px">' +
        '<div style="font-size:13px;font-weight:600;color:#a1a1aa;text-transform:uppercase;letter-spacing:0.06em;margin-bottom:12px;">SSO Users</div>' +
        '<div style="background:#27272a;border:1px solid #3f3f46;border-radius:10px;overflow:hidden;max-width:560px">' +
          oidcRows +
        '</div>' +
      '</div>';
    }
    if (!_oidcUsersLoaded) fetchOIDCUsers();

    return header + content + oidcSection;
  }

  async function fetchOIDCUsers() {
    try {
      var r = await fetch('/api/users/oidc');
      if (!r.ok) return;
      _oidcUsersData = await r.json();
      _oidcUsersLoaded = true;
      if (currentRoute === '/users') renderMainOnly();
    } catch(e) {}
  }

  window.__selectOIDCRole = function(role) {
    document.getElementById('edit-oidc-user-role').value = role;
    ['admin','viewer','none'].forEach(function(r) {
      var el = document.getElementById('oidc-role-opt-' + r);
      if (!el) return;
      el.style.background = r === role ? '#27272a' : '';
      el.style.borderColor = r === role ? (r === 'admin' ? '#FB326E' : r === 'viewer' ? '#22c55e' : '#71717a') : '#3f3f46';
    });
  };

  window.__openEditOIDCUser = function(email, currentRole) {
    document.getElementById('edit-oidc-user-email').value = email;
    document.getElementById('edit-oidc-user-error').textContent = '';
    window.__selectOIDCRole(currentRole || 'none');
    document.getElementById('edit-oidc-user-modal').classList.add('show');
  };

  window.__closeEditOIDCUser = function() {
    document.getElementById('edit-oidc-user-modal').classList.remove('show');
  };

  window.__submitEditOIDCUser = async function() {
    var email = document.getElementById('edit-oidc-user-email').value;
    var role = document.getElementById('edit-oidc-user-role').value;
    var errEl = document.getElementById('edit-oidc-user-error');
    errEl.textContent = '';
    try {
      var r = await fetch('/api/users/oidc', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email, role: role }),
      });
      if (!r.ok) { errEl.textContent = 'Failed to update role'; return; }
      window.__closeEditOIDCUser();
      _oidcUsersLoaded = false;
      _oidcUsersData = null;
      fetchOIDCUsers();
    } catch(e) { errEl.textContent = 'Network error'; }
  };

  window.__deleteOIDCUser = function(email) {
    document.getElementById('delete-oidc-user-email-label').textContent = email;
    document.getElementById('delete-oidc-user-error').textContent = '';
    document.getElementById('delete-oidc-user-modal').dataset.email = email;
    document.getElementById('delete-oidc-user-modal').classList.add('show');
  };

  window.__closeDeleteOIDCUser = function() {
    document.getElementById('delete-oidc-user-modal').classList.remove('show');
  };

  window.__confirmDeleteOIDCUser = async function() {
    var email = document.getElementById('delete-oidc-user-modal').dataset.email;
    var errEl = document.getElementById('delete-oidc-user-error');
    errEl.textContent = '';
    try {
      var r = await fetch('/api/users/oidc', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email }),
      });
      if (!r.ok) { errEl.textContent = 'Failed to delete user'; return; }
      window.__closeDeleteOIDCUser();
      _oidcUsersLoaded = false;
      _oidcUsersData = null;
      fetchOIDCUsers();
    } catch(e) { errEl.textContent = 'Network error'; }
  };

  async function fetchUsers() {
    try {
      var r = await fetch('/api/users');
      _usersData = await r.json();
      if (currentRoute === '/users') renderMainOnly();
    } catch(e) {}
  }

  window.__openChangePassword = function() {
    document.getElementById('chpw-current').value = '';
    document.getElementById('chpw-new').value = '';
    document.getElementById('chpw-confirm').value = '';
    document.getElementById('chpw-error').textContent = '';
    document.getElementById('chpw-confirm-msg').textContent = '';
    updateChpwChecks('');
    document.getElementById('chpw-modal').classList.add('show');
    document.getElementById('chpw-current').focus();
  };

  window.__closeChangePassword = function() {
    document.getElementById('chpw-modal').classList.remove('show');
  };

  window.__openEditProfile = function() {
    var u = _usersData && _usersData[0];
    document.getElementById('editprofile-email').value = u ? (u.username || '') : '';
    document.getElementById('editprofile-displayname').value = u ? (u.displayName || '') : '';
    document.getElementById('editprofile-error').textContent = '';
    document.getElementById('editprofile-modal').classList.add('show');
    document.getElementById('editprofile-email').focus();
  };

  window.__closeEditProfile = function() {
    document.getElementById('editprofile-modal').classList.remove('show');
  };

  window.__submitEditProfile = async function() {
    var newDisplayName = document.getElementById('editprofile-displayname').value.trim();
    var errEl = document.getElementById('editprofile-error');
    errEl.textContent = '';
    try {
      var r = await fetch('/api/users/update-profile', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ newDisplayName: newDisplayName })
      });
      var d = await r.json();
      if (!r.ok) { errEl.textContent = d.error || 'Failed to update profile'; return; }
      window.__closeEditProfile();
      _usersData = null;
      fetchUsers();
    } catch(e) { errEl.textContent = 'Request failed'; }
  };

  window.__submitChangePassword = async function() {
    var current = document.getElementById('chpw-current').value;
    var newPw   = document.getElementById('chpw-new').value;
    var confirm = document.getElementById('chpw-confirm').value;
    var errEl   = document.getElementById('chpw-error');
    errEl.textContent = '';
    if (newPw !== confirm) { errEl.textContent = 'New passwords do not match'; return; }
    try {
      var r = await fetch('/api/users/change-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ currentPassword: current, newPassword: newPw })
      });
      var d = await r.json();
      if (!r.ok) { errEl.textContent = d.error || 'Failed to change password'; return; }
      window.__closeChangePassword();
    } catch(e) { errEl.textContent = 'Request failed'; }
  };

  function updateChpwChecks(v) {
    var checks = {
      'chpw-chk-len':     v.length >= 12,
      'chpw-chk-upper':   /[A-Z]/.test(v),
      'chpw-chk-num':     /[0-9]/.test(v),
      'chpw-chk-special': /[^a-zA-Z0-9]/.test(v)
    };
    for (var id in checks) {
      var el = document.getElementById(id);
      if (!el) continue;
      var icon = el.querySelector('.pw-check-icon');
      if (checks[id]) { el.classList.add('met'); icon.textContent = '✓'; }
      else { el.classList.remove('met'); icon.textContent = '✗'; }
    }
  }

  function renderHeader(s, clusterOverride) {
    var titles = { '/clusters': 'Clusters', '/machineclasses': 'Machine Classes', '/instances': 'Instances', '/repos': 'Repositories', '/users': 'Users', '/logs': 'Logs' };
    var pageTitle = titles[currentRoute] || 'Clusters';
    var titleHtml;
    if (clusterOverride) {
      titleHtml = '<span class="breadcrumb"><a class="breadcrumb-link" href="/clusters">Clusters</a>' +
        '<span class="breadcrumb-sep">/</span>' +
        '<span class="breadcrumb-current">' + escHtml(clusterOverride.id) + '</span></span>';
    } else {
      titleHtml = pageTitle;
    }
    return '<div class="header">' +
      '<h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">' + titleHtml + '</h1>' +
    '</div>';
  }

  function renderMachineClassesView(s) {
    var mcs = (s.machineClasses || []).slice().sort(function(a, b) {
      return machineClassSortAZ ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id);
    });
    var serverRunning = !!(s.lastReconcile && s.lastReconcile.status === 'running');
    var syncRunning = reconcileRunning || (lastHardReconcileAt >= lastSoftReconcileAt && serverRunning);
    var mcRunning = mcRefreshing || (lastSoftReconcileAt > lastHardReconcileAt && serverRunning);
    var isRunning = mcRunning || syncRunning;
    var activeFilters = Object.keys(mcStatusFilters);
    var filteredMcs = mcs.filter(function(m) {
      if (mcSearch && m.id.toLowerCase().indexOf(mcSearch) < 0) return false;
      if (activeFilters.length === 0) return true;
      var key = (m.status === 'success' || m.status === 'applied') ? 'synced' : (m.status || 'unknown');
      return !!mcStatusFilters[key];
    });
    var pageMcs = mcPageSize === 0 ? filteredMcs : paginateWithSize(filteredMcs, machineClassPage, mcPageSize);
    var mcRepoSyncErrors = {};
    (s.repos || []).forEach(function(r) { if (r.syncError) mcRepoSyncErrors[r.name] = true; });
    var cardsHtml = pageMcs.length > 0
      ? pageMcs.map(function(m) {
          var mcContent = m.fileContent || m.liveContent || '';
          var spec = parseMachineClassSpec(mcContent, m.id);
          var displayStatus = m.status === 'success' ? 'synced' : m.status;
          var hasDiff = m.diff && m.diff.length > 0;
          var hasFile = m.fileContent && m.fileContent.length > 0;
          var hasLive = m.liveContent && m.liveContent.length > 0;
          var hasDetails = hasDiff || hasFile || hasLive;
          // Detect provisionType from content when not set by server (e.g. unmanaged)
          var provisionType = m.provisionType;
          if (!provisionType && mcContent) {
            provisionType = mcContent.toLowerCase().indexOf('providerid:') >= 0 ? 'auto' : 'manual';
          }
          var provisionLabel = provisionType === 'auto' ? 'Auto-Provision' : provisionType === 'manual' ? 'Manual' : 'Unknown';
          var provisionColor = provisionType === 'auto' ? '#60a5fa' : '#a1a1aa';
          var statusDot = '';
          if (m.status === 'success' || m.status === 'applied') statusDot = '#4ade80';
          else if (m.status === 'failed') statusDot = '#f87171';
          else if (m.status === 'outofsync') statusDot = '#fb923c';
          else if (m.status === 'syncing') statusDot = '#2dd4bf';
          else statusDot = '#52525b';

          var isAuto = provisionType === 'auto';
          var pd = (isAuto && spec) ? spec.providerData : {};
          var pdCores    = pd.cores    || pd.vcpu || pd.cpu   || '';
          var pdSockets  = pd.sockets  || '';
          var pdMemory   = pd.memory   || pd.ram  || '';
          var pdDisk     = pd.disk_size || pd.diskSize || pd.disk || '';
          var provider   = (isAuto && spec) ? spec.providerId : '';

          var infoRows =
            '<div class="mc-info-row"><span class="mc-info-label">Mode</span><span class="mc-info-value" style="color:' + provisionColor + '">' + provisionLabel + '</span></div>' +
            (provider   ? '<div class="mc-info-row"><span class="mc-info-label">Provider</span><span class="mc-info-value">' + escHtml(provider) + '</span></div>' : '') +
            (pdCores    ? '<div class="mc-info-row"><span class="mc-info-label">Cores</span><span class="mc-info-value">' + escHtml(String(pdCores)) + '</span></div>' : '') +
            (pdSockets  ? '<div class="mc-info-row"><span class="mc-info-label">Sockets</span><span class="mc-info-value">' + escHtml(String(pdSockets)) + '</span></div>' : '') +
            (pdMemory   ? '<div class="mc-info-row"><span class="mc-info-label">Memory</span><span class="mc-info-value">' + escHtml(String(pdMemory)) + '</span></div>' : '') +
            (pdDisk     ? '<div class="mc-info-row"><span class="mc-info-label">Disk Size</span><span class="mc-info-value">' + escHtml(String(pdDisk)) + '</span></div>' : '');

          // Compute which clusters reference this machine class.
          var usedByClusters = [];
          if (state && state.clusters) {
            state.clusters.forEach(function(cl) {
              var cp = cl.controlPlane || {};
              var wks = Array.isArray(cl.workers) ? cl.workers : (cl.workers ? [cl.workers] : []);
              var refs = [cp].concat(wks);
              for (var ri = 0; ri < refs.length; ri++) {
                if (refs[ri].machineClass === m.id) {
                  usedByClusters.push(cl.id);
                  break;
                }
              }
            });
          }
          usedByClusters.sort();
          var usedByHtml = '<div class="mc-info-row" style="align-items:flex-start;margin-top:4px">' +
            '<span class="mc-info-label">Used by</span>' +
            '<span class="mc-info-value">' +
              (usedByClusters.length > 0
                ? '<div class="mc-used-by">' +
                    usedByClusters.map(function(id) { return '<span class="mc-used-by-chip" title="' + escHtml(id) + '" onclick="event.stopPropagation();window.__navToCluster(\'' + escHtml(id) + '\')">' + escHtml(id) + '</span>'; }).join('') +
                  '</div>'
                : '<span class="mc-used-by-none">none</span>') +
            '</span>' +
          '</div>';

          var mlKeys = spec ? Object.keys(spec.matchLabels) : [];
          var mlHtml = mlKeys.length > 0
            ? '<div class="mc-info-row" style="align-items:flex-start;margin-top:4px">' +
                '<span class="mc-info-label">Match Labels</span>' +
                '<span class="mc-info-value">' +
                  mlKeys.map(function(k) {
                    return '<div style="padding:1px 0"><span style="color:#71717a;margin-right:5px">•</span>' + escHtml(k) + ' = ' + escHtml(spec.matchLabels[k]) + '</div>';
                  }).join('') +
                '</span>' +
              '</div>'
            : '';

          var mcStatusText = '', mcStatusColor = '#71717a';
          if (m.status === 'success' || m.status === 'applied') { mcStatusText = '● synced';        mcStatusColor = '#4ade80'; }
          else if (m.status === 'failed')                        { mcStatusText = '● failed';       mcStatusColor = '#f87171'; }
          else if (m.status === 'outofsync')                     { mcStatusText = '● out of sync';  mcStatusColor = '#fb923c'; }
          else if (m.status === 'syncing')                       { mcStatusText = '● syncing';      mcStatusColor = '#2dd4bf'; }
          else if (m.status === 'unmanaged')                     { mcStatusText = '● unmanaged';    mcStatusColor = '#52525b'; }
          if (m.repoName && mcRepoSyncErrors[m.repoName] && m.status !== 'unmanaged') {
            mcStatusText = '<span class="spinner" style="width:10px;height:10px;display:inline-block;vertical-align:middle"></span>';
            mcStatusColor = '#2dd4bf';
          }
          return '<div class="mc-card' + (hasDetails ? ' clickable' : '') + '" data-status="' + (m.status || 'idle') + '"' + (hasDetails ? ' onclick="window.__showMachineClassModal(\'' + m.id + '\')"' : '') + '>' +
            '<div class="mc-card-accent"></div>' +
            '<div class="mc-card-header">' +
              '<div class="mc-card-title-row">' +
                '<span class="mc-card-name">' +
                  m.id +
                '</span>' +
                (mcStatusText ? '<span class="mc-card-status" style="color:' + mcStatusColor + ';">' + mcStatusText + '</span>' : '') +
              '</div>' +
              '<div class="mc-card-divider"></div>' +
              infoRows +
              mlHtml +
              '<div class="mc-card-divider" style="margin-top:4px"></div>' +
              usedByHtml +
              (m.status === 'unmanaged' ?
                '<div style="margin-top:auto">' +
                  '<div class="mc-card-divider" style="margin-top:8px"></div>' +
                  '<div class="cluster-card-actions">' +
                    (isAdmin ? '<button class="btn-sort btn-primary" onclick="window.__exportMachineClass(\'' + m.id + '\', event);event.stopPropagation()" title="Export machine class as YAML">\u2193 Export</button>' : '') +
                    (isAdmin ? (usedByClusters.length > 0
                      ? '<button class="btn-sort btn-primary" disabled title="Cannot delete \u2014 in use by: ' + usedByClusters.join(', ') + '">\u2715 Delete</button>'
                      : '<button class="btn-sort btn-primary" onclick="window.__deleteMachineClass(\'' + m.id + '\', event);event.stopPropagation()" title="Delete this machine class from Omni">\u2715 Delete</button>') : '') +
                  '</div>' +
                '</div>'
              : '<div style="margin-top:auto">' +
                  '<div class="mc-card-divider" style="margin-top:8px"></div>' +
                  '<div class="cluster-card-actions">' +
                    (isAdmin ? '<button class="btn-sort" onclick="window.__refreshSingleMC(\'' + m.id + '\', event);event.stopPropagation()" title="Refresh this machine class from Git">\u21bb Refresh</button>' : '') +
                    (isAdmin ? '<button class="btn-sort" onclick="window.__syncMachineClass(\'' + m.id + '\', event);event.stopPropagation()" title="Force sync this machine class from Git">\u21c5 Sync</button>' : '') +
                    (isAdmin ? '<button class="btn-sort btn-primary auto-sync ' + (m.autoSync === true ? 'active' : '') + '" onclick="window.__setMCAutoSync(\'' + m.id + '\', ' + (m.autoSync === true ? 'false' : 'true') + ', event);event.stopPropagation()" title="Toggle per-machine-class auto sync">' + (m.autoSync === true ? '● Auto-Sync: On' : '○ Auto-Sync: Off') + '</button>' : '') +
                    (isAdmin ? (usedByClusters.length > 0
                      ? '<button class="btn-sort btn-primary" disabled title="Cannot delete \u2014 in use by: ' + usedByClusters.join(', ') + '">\u2715 Delete</button>'
                      : '<button class="btn-sort btn-primary" onclick="window.__deleteMachineClass(\'' + m.id + '\', event);event.stopPropagation()" title="Delete this machine class from Omni">\u2715 Delete</button>') : '') +
                  '</div>' +
                '</div>') +
            '</div>' +
          '</div>';
        }).join('')
      : '<div style="padding:40px;text-align:center;color:#52525b">No machine classes</div>';

    var mcActionBar = '<div style="display:flex;align-items:center;gap:8px">' +
      (isRunning ? '<span class="spinner"></span>' : '') +
      (isAdmin ? '<button class="btn-sort btn-primary" onclick="window.__refreshMC()" ' + (isRunning ? 'disabled' : '') + '>' + (mcRunning ? 'Refreshing...' : 'Refresh') + '</button>' : '') +
      (isAdmin ? '<button class="btn-sort btn-primary" onclick="window.__triggerReconcile()" ' + (isRunning ? 'disabled' : '') + '>' + (syncRunning ? 'Syncing...' : 'Sync') + '</button>' : '') +
      '<input type="text" placeholder="Search machine classes..." value="' + mcSearch + '" oninput="window.__setMcSearch(this.value)" style="background:#18181b;border:1px solid #3f3f46;border-radius:4px;color:#e4e4e7;font-size:12px;padding:3px 8px;outline:none;width:180px;font-family:inherit;margin-left:8px;" />' +
    '</div>';
    var mcHeader = '<div class="header">' +
      '<h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Machine Classes</h1>' +
      mcActionBar +
    '</div>';
    // Status filter checkboxes — only show statuses present in data
    var mcPresentKeys = {};
    mcs.forEach(function(m) {
      var key = (m.status === 'success' || m.status === 'applied') ? 'synced' : (m.status || null);
      if (key) mcPresentKeys[key] = true;
    });
    var statusDefs = [
      { key: 'synced',    label: 'Synced'      },
      { key: 'outofsync', label: 'Out of sync' },
      { key: 'failed',    label: 'Failed'      },
      { key: 'unmanaged', label: 'Unmanaged'   },
    ];
    var filterCheckboxes = statusDefs.filter(function(d) { return mcPresentKeys[d.key]; }).map(function(d) {
      var checked = !!mcStatusFilters[d.key];
      return '<label style="display:flex;align-items:center;gap:5px;cursor:pointer;font-size:12px;color:#a1a1aa;user-select:none">' +
        '<input type="checkbox"' + (checked ? ' checked' : '') + ' onchange="window.__toggleMcStatusFilter(\'' + d.key + '\')" style="accent-color:#fb923c;cursor:pointer">' +
        d.label +
      '</label>';
    }).join('');
    var mcToolbar = '<div style="display:flex;align-items:center;gap:16px;padding:0 0 16px">' +
      '<div style="display:flex;align-items:center;gap:12px">' + filterCheckboxes + '</div>' +
      '<div style="margin-left:auto;display:flex;align-items:center;gap:8px">' +
        '<button class="btn-sort ' + (machineClassSortAZ ? 'active' : '') + '" onclick="window.__toggleMachineClassSort()">' + (machineClassSortAZ ? 'A→Z' : 'Z→A') + '</button>' +
        '<div class="page-size-bar">' + renderPageSizeBar([10, 25, 50, 0], mcPageSize, 'window.__setMcPageSize') + '</div>' +
      '</div>' +
    '</div>';
    return mcHeader + mcToolbar +
      '<div class="mc-grid">' + cardsHtml + '</div>' +
      (mcPageSize > 0 && filteredMcs.length > mcPageSize ? renderPaginationSized(filteredMcs, machineClassPage, 'window.__changeMachineClassPage', mcPageSize) : '');
  }

  function renderClustersView(s) {
    var clusters = (s.clusters || []).slice().sort(function(a, b) {
      return clusterSortAZ ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id);
    });
    var total = clusters.length;
    var countReady = 0, countNotReady = 0;
    var countScalingUp = 0, countScalingDown = 0, countDestroying = 0, countReconfiguring = 0;
    clusters.forEach(function(c) {
      var phase = c.clusterPhase || '';
      if (phase === 'scaling-up')    { countScalingUp++;     return; }
      if (phase === 'scaling-down')  { countScalingDown++;   return; }
      if (phase === 'destroying')    { countDestroying++;    return; }
      if (phase === 'reconfiguring') { countReconfiguring++; return; }
      if (!c.clusterReady || c.clusterReady === 'unknown') return;  // no Omni health data
      if (c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready') { countReady++; }
      else { countNotReady++; }
    });
    var healthTotal = countReady + countNotReady + countScalingUp + countScalingDown + countDestroying + countReconfiguring;
    var healthBar = '';
    {
      var hasFilter   = clusterStatusFilter !== null;
      var summaryParts = [];
      if (countReady)         summaryParts.push('<span style="color:#4ade80;cursor:pointer' + (hasFilter && clusterStatusFilter === 'ready'         ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'ready\')">'         + countReady        + ' ready</span>');
      if (countNotReady)      summaryParts.push('<span style="color:#f87171;cursor:pointer' + (hasFilter && clusterStatusFilter === 'not-ready'     ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'not-ready\')">'     + countNotReady     + ' not ready</span>');
      if (countScalingUp)     summaryParts.push('<span style="color:#60a5fa;cursor:pointer' + (hasFilter && clusterStatusFilter === 'scaling-up'    ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'scaling-up\')">'    + countScalingUp    + ' scaling up</span>');
      if (countScalingDown)   summaryParts.push('<span style="color:#f59e0b;cursor:pointer' + (hasFilter && clusterStatusFilter === 'scaling-down'  ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'scaling-down\')">'  + countScalingDown  + ' scaling down</span>');
      if (countDestroying)    summaryParts.push('<span style="color:#f43f5e;cursor:pointer' + (hasFilter && clusterStatusFilter === 'destroying'    ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'destroying\')">'    + countDestroying   + ' destroying</span>');
      if (countReconfiguring) summaryParts.push('<span style="color:#a78bfa;cursor:pointer' + (hasFilter && clusterStatusFilter === 'reconfiguring' ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'reconfiguring\')">' + countReconfiguring + ' reconfiguring</span>');
      var clearBtn = hasFilter ? ' &nbsp;<span style="cursor:pointer;color:#a1a1aa;text-decoration:underline;font-size:11px" onclick="window.__clearClusterFilter()">clear</span>' : '';
      healthBar = '<div class="cluster-health-bar-wrap">' +
        '<div class="cluster-health-bar' + (hasFilter ? ' has-filter' : '') + '">' +
          (countReady        ? '<div class="cluster-health-bar-seg cluster-health-bar-seg--ready'         + (hasFilter && clusterStatusFilter === 'ready'         ? ' active' : '') + '" style="width:' + (countReady        / healthTotal * 100).toFixed(1) + '%" onclick="window.__setClusterFilter(\'ready\')"         title="' + countReady        + ' ready"></div>'         : '') +
          (countNotReady     ? '<div class="cluster-health-bar-seg cluster-health-bar-seg--notready'      + (hasFilter && clusterStatusFilter === 'not-ready'     ? ' active' : '') + '" style="width:' + (countNotReady     / healthTotal * 100).toFixed(1) + '%" onclick="window.__setClusterFilter(\'not-ready\')"     title="' + countNotReady     + ' not ready"></div>'    : '') +
          (countScalingUp    ? '<div class="cluster-health-bar-seg cluster-health-bar-seg--scalingup'     + (hasFilter && clusterStatusFilter === 'scaling-up'    ? ' active' : '') + '" style="width:' + (countScalingUp    / healthTotal * 100).toFixed(1) + '%" onclick="window.__setClusterFilter(\'scaling-up\')"    title="' + countScalingUp    + ' scaling up"></div>'    : '') +
          (countScalingDown  ? '<div class="cluster-health-bar-seg cluster-health-bar-seg--scalingdown'   + (hasFilter && clusterStatusFilter === 'scaling-down'  ? ' active' : '') + '" style="width:' + (countScalingDown  / healthTotal * 100).toFixed(1) + '%" onclick="window.__setClusterFilter(\'scaling-down\')"  title="' + countScalingDown  + ' scaling down"></div>'  : '') +
          (countDestroying   ? '<div class="cluster-health-bar-seg cluster-health-bar-seg--destroying'    + (hasFilter && clusterStatusFilter === 'destroying'    ? ' active' : '') + '" style="width:' + (countDestroying   / healthTotal * 100).toFixed(1) + '%" onclick="window.__setClusterFilter(\'destroying\')"    title="' + countDestroying   + ' destroying"></div>'    : '') +
          (countReconfiguring? '<div class="cluster-health-bar-seg cluster-health-bar-seg--reconfiguring' + (hasFilter && clusterStatusFilter === 'reconfiguring' ? ' active' : '') + '" style="width:' + (countReconfiguring/ healthTotal * 100).toFixed(1) + '%" onclick="window.__setClusterFilter(\'reconfiguring\')" title="' + countReconfiguring+ ' reconfiguring"></div>': '') +
        '</div>' +
        '<div class="cluster-health-summary">' + total + ' clusters &nbsp;·&nbsp; ' + summaryParts.join(' &nbsp;·&nbsp; ') + clearBtn + '</div>' +
      '</div>';
    }
    var activeSyncKeys = Object.keys(clusterSyncFilters);
    var displayClusters = clusters.filter(function(c) {
      var st = c.status || '';
      var phase = c.clusterPhase || '';
      // Phase/health filter (from health bar click — single select)
      if (clusterStatusFilter) {
        if (clusterStatusFilter === 'ready')         { if (!(c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready') || (phase && phase !== 'running')) return false; }
        else if (clusterStatusFilter === 'not-ready'){ if (!((c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') && (!phase || phase === 'running'))) return false; }
        else if (clusterStatusFilter === 'scaling-up')    { if (phase !== 'scaling-up')    return false; }
        else if (clusterStatusFilter === 'scaling-down')  { if (phase !== 'scaling-down')  return false; }
        else if (clusterStatusFilter === 'destroying')    { if (phase !== 'destroying')    return false; }
        else if (clusterStatusFilter === 'reconfiguring') { if (phase !== 'reconfiguring') return false; }
      }
      // Sync status filter (from checkboxes — multi select)
      if (activeSyncKeys.length > 0) {
        var syncKey = (st === 'success' || st === 'applied' || st === 'synced') ? 'synced'
          : (st === 'outofsync' ? 'outofsync' : (st === 'failed' ? 'failed' : (st === 'unmanaged' ? 'unmanaged' : (st === 'orphaned' ? 'orphaned' : null))));
        if (!syncKey || !clusterSyncFilters[syncKey]) return false;
      }
      return true;
    });
    if (clusterSearch) {
      displayClusters = displayClusters.filter(function(c) {
        return c.id.toLowerCase().indexOf(clusterSearch) >= 0;
      });
    }
    var pageDisplayClusters = clusterPageSize === 0 ? displayClusters : paginateWithSize(displayClusters, clusterPage, clusterPageSize);
    // Build a lookup of repo names that are currently failing to sync so each
    // cluster card can show a 'refreshing' indicator instead of a stale status.
    var repoSyncErrors = {};
    (s.repos || []).forEach(function(r) { if (r.syncError) repoSyncErrors[r.name] = true; });
    var cards = pageDisplayClusters.map(function(c) {
      var cp = c.controlPlane || {};
      var cpCount = cp.count || 0;
      var cpMC = cp.machineClass || '';
      var hasDiff = c.diff && c.diff.length > 0;
      var hasFile = c.fileContent && c.fileContent.length > 0;
      var isFailed = c.status === 'failed';
      var hasError = c.error && c.error.length > 0;
      var hasDetails = true; // All clusters get a detail page
      var workers = Array.isArray(c.workers) ? c.workers : (c.workers ? [c.workers] : []);
      // Controlplane is the first column; worker pools follow — all rendered in 2-column rows
      var sections = [{ label: 'Controlplane', count: cpCount, mc: cpMC }];
      workers.forEach(function(wk) {
        sections.push({ label: wk.name || 'Workers', count: wk.count || 0, mc: wk.machineClass || '' });
      });
      if (workers.length === 0) {
        sections.push({ label: 'Workers', count: 0, mc: '' });
      }
      var sectionsHtml = sections.map(function(sec) {
        return '<div class="cluster-pool-row">' +
          '<div class="cluster-pool-row-label">' + escHtml(sec.label) + '</div>' +
          '<div class="cluster-pool-row-count">' + sec.count + '</div>' +
          '<div class="cluster-pool-row-mc">' + (sec.mc ? escHtml(sec.mc) : '<span style="color:#3f3f46">—</span>') + '</div>' +
        '</div>';
      }).join('');
      var statusText = '', statusColor = '#71717a';
      if (c.status === 'unmanaged')                              { statusText = 'unmanaged';      statusColor = '#52525b'; }
      else if (c.status === 'outofsync')                         { statusText = '● out of sync';  statusColor = '#fb923c'; }
      else if (c.status === 'orphaned')                          { statusText = '● orphaned';     statusColor = '#a78bfa'; }
      else if (c.status === 'failed')                            { statusText = '● failed';       statusColor = '#f87171'; }
      else if (c.status === 'syncing')                           { statusText = '● syncing';      statusColor = '#2dd4bf'; }
      else if (c.status === 'success' || c.status === 'applied') { statusText = '● synced';       statusColor = '#4ade80'; }
      // Override: if the cluster's repo is failing to sync, show refreshing
      if (c.repoName && repoSyncErrors[c.repoName] && c.status !== 'unmanaged') {
        statusText = '<span class="spinner" style="width:10px;height:10px;display:inline-block;vertical-align:middle"></span>';
        statusColor = '#2dd4bf';
      }
      var healthText = '', healthColor = '';
      var phase = c.clusterPhase || '';
      if (phase === 'scaling-up')        { healthText = '↑ scaling up';    healthColor = '#60a5fa'; }
      else if (phase === 'scaling-down')  { healthText = '↓ scaling down';  healthColor = '#f59e0b'; }
      else if (phase === 'destroying')    { healthText = '✕ destroying';    healthColor = '#f43f5e'; }
      else if (phase === 'reconfiguring') { healthText = '↻ reconfiguring'; healthColor = '#a78bfa'; }
      else if (c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready') {
        healthText = '✓ ready'; healthColor = '#4ade80';
      } else if (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') {
        healthText = '✗ not ready'; healthColor = '#f87171';
      }
      var cardHealth = ((c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready')) ? ' data-health="not-ready"' : '';
      var activePhases = ['scaling-up','scaling-down','destroying','reconfiguring'];
      var cardPhase = (c.clusterPhase && activePhases.indexOf(c.clusterPhase) >= 0) ? ' data-phase="' + c.clusterPhase + '"' : '';
      return '<div class="cluster-card clickable" data-status="' + (c.status || 'idle') + '"' + cardHealth + cardPhase + ' onclick="window.__navToCluster(\'' + c.id + '\')">' +
        '<div class="cluster-card-accent"></div>' +
        '<div class="cluster-card-body">' +
          '<div class="cluster-card-header">' +
            '<span class="cluster-card-title">' +
              c.id +
            '</span>' +
            '<div style="display:flex;align-items:center;gap:6px;flex-shrink:0;">' +
              (statusText ? '<span class="cluster-card-status" style="color:' + statusColor + ';">' + statusText + '</span>' : '') +
            '</div>' +
          '</div>' +
          '<div class="cluster-card-versions">' +
            'Talos ' + escHtml(c.talosVersion || '—') + ' · K8s ' + escHtml(c.kubernetesVersion || '—') +
            (healthText ? '&ensp;<span style="color:' + healthColor + ';">' + healthText + '</span>' : '') +
          '</div>' +
          '<div class="cluster-card-divider"></div>' +
          sectionsHtml +
          '<div class="cluster-card-divider" style="margin-top:8px"></div>' +
          '<div class="cluster-card-actions">' +
            (isAdmin ? '<button class="btn-sort btn-primary" onclick="window.__refreshCluster(\'' + c.id + '\', event);event.stopPropagation()" title="Re-read live state from Omni">↺ Refresh</button>' : '') +
            (isAdmin && c.status !== 'deleting' && (c.status === 'unmanaged' || c.status === 'orphaned') ? '<button class="btn-sort btn-primary" onclick="window.__exportCluster(\'' + c.id + '\', event);event.stopPropagation()" title="Export cluster as YAML template">↓ Export</button>' : '') +
            (isAdmin && c.status !== 'deleting' && c.status !== 'unmanaged' && c.status !== 'orphaned' ? '<button class="btn-sort btn-primary" onclick="window.__syncCluster(\'' + c.id + '\', event);event.stopPropagation()" title="Force sync this cluster from Git">⇅ Sync</button>' : '') +
            (isAdmin && c.status !== 'deleting' && c.status !== 'unmanaged' && c.status !== 'orphaned' ? '<button class="btn-sort btn-primary auto-sync ' + (c.autoSync === false ? '' : 'active') + '" onclick="window.__setClusterAutoSync(\'' + c.id + '\', ' + (c.autoSync === false ? 'true' : 'false') + ', event);event.stopPropagation()" title="Toggle per-cluster auto sync">' + (c.autoSync === false ? '○ Auto-Sync: Off' : '● Auto-Sync: On') + '</button>' : '') +
            (isAdmin && c.status !== 'deleting' && c.status !== 'unmanaged' ? '<button class="btn-sort btn-primary" onclick="window.__deleteCluster(\'' + c.id + '\', event);event.stopPropagation()" title="Delete this cluster from Omni">\u2715 Delete</button>' : '') +
          '</div>' +
        '</div>' +
      '</div>';
    }).join('');

    var serverRunningC = !!(s.lastReconcile && s.lastReconcile.status === 'running');
    var syncRunningC = reconcileRunning || (lastHardReconcileAt >= lastSoftReconcileAt && serverRunningC);
    var gitRunningC = gitRefreshing || (lastSoftReconcileAt > lastHardReconcileAt && serverRunningC);
    var isRunningC = gitRunningC || syncRunningC;
    // Sync status filter checkboxes — only show statuses present in data
    var clusterPresentKeys = {};
    clusters.forEach(function(c) {
      var st = c.status || '';
      var key = (st === 'success' || st === 'applied' || st === 'synced') ? 'synced'
        : (st === 'outofsync' ? 'outofsync' : (st === 'failed' ? 'failed' : (st === 'unmanaged' ? 'unmanaged' : (st === 'orphaned' ? 'orphaned' : null))));
      if (key) clusterPresentKeys[key] = true;
    });
    var clusterSyncDefs = [
      { key: 'synced',    label: 'Synced'      },
      { key: 'outofsync', label: 'Out of sync' },
      { key: 'failed',    label: 'Failed'      },
      { key: 'unmanaged', label: 'Unmanaged'   },
      { key: 'orphaned',  label: 'Orphaned'    },
    ];
    var clusterSyncCheckboxes = clusterSyncDefs.filter(function(d) { return clusterPresentKeys[d.key]; }).map(function(d) {
      var checked = !!clusterSyncFilters[d.key];
      return '<label style="display:flex;align-items:center;gap:5px;cursor:pointer;font-size:12px;color:#a1a1aa;user-select:none">' +
        '<input type="checkbox"' + (checked ? ' checked' : '') + ' onchange="window.__toggleClusterSyncFilter(\'' + d.key + '\')" style="accent-color:#fb923c;cursor:pointer">' +
        d.label +
      '</label>';
    }).join('');
    var clusterToolbar = '<div style="display:flex;align-items:center;gap:12px;padding:0 0 12px">' +
      '<div style="display:flex;align-items:center;gap:12px">' + clusterSyncCheckboxes + '</div>' +
      '<div style="margin-left:auto;display:flex;align-items:center;gap:8px">' +
        '<button class="btn-sort active" onclick="window.__toggleClusterSort()">' + (clusterSortAZ ? 'A→Z' : 'Z→A') + '</button>' +
        '<div class="page-size-bar">' + renderPageSizeBar([5, 10, 15, 20, 0], clusterPageSize, 'window.__setClusterPageSize') + '</div>' +
      '</div>' +
    '</div>';
    var bottomBar = '<div style="display:flex;align-items:center;gap:12px;padding:12px 0 0">' +
      '<div style="margin-left:auto;display:flex;align-items:center;gap:8px">' +
        '<button class="btn-sort active" onclick="window.__toggleClusterSort()">' + (clusterSortAZ ? 'A→Z' : 'Z→A') + '</button>' +
        '<div class="page-size-bar">' + renderPageSizeBar([5, 10, 15, 20, 0], clusterPageSize, 'window.__setClusterPageSize') + '</div>' +
      '</div>' +
    '</div>';
    var actionBar = '<div style="display:flex;align-items:center;gap:8px">' +
      (isRunningC ? '<span class="spinner"></span>' : '') +
      (isAdmin ? '<button class="btn-sort btn-primary" onclick="window.__checkGit()" ' + (isRunningC ? 'disabled' : '') + '>' + (gitRunningC ? 'Refreshing...' : 'Refresh') + '</button>' : '') +
      (isAdmin ? '<button class="btn-sort btn-primary" onclick="window.__triggerReconcile()" ' + (isRunningC ? 'disabled' : '') + '>' + (syncRunningC ? 'Syncing...' : 'Sync') + '</button>' : '') +
      '<input type="text" placeholder="Search clusters..." value="' + clusterSearch + '" oninput="window.__setClusterSearch(this.value)" style="background:#18181b;border:1px solid #3f3f46;border-radius:4px;color:#e4e4e7;font-size:12px;padding:3px 8px;outline:none;width:180px;font-family:inherit;margin-left:8px;" />' +
    '</div>';
    var clusterHeader = '<div class="header">' +
      '<h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Clusters</h1>' +
      actionBar +
    '</div>';
    return clusterHeader + healthBar + clusterToolbar +
      (clusters.length > 0
        ? '<div class="cluster-grid">' + cards + '</div>' +
          (clusterPageSize > 0 && displayClusters.length > clusterPageSize ? renderPaginationSized(displayClusters, clusterPage, 'window.__changeClusterPage', clusterPageSize) : '') +
          healthBar + bottomBar
        : '<div style="padding:24px;color:#52525b">No clusters found</div>');
  }

  function renderClusterGraph(modal) {
    var cluster = state && state.clusters && state.clusters.find(function(c) { return c.id === modal.id; });
    if (!cluster) return '<div style="color:#71717a;text-align:center;padding:40px;">No cluster data available</div>';

    var git = (state && state.git) || {};
    // Use the repo that owns this cluster for accurate branch/sha/sync info.
    // Conflicted clusters have no single owner — show a neutral placeholder.
    var isConflict = !cluster.repoName && cluster.status === 'failed' && cluster.error;
    var clusterRepo = git;
    if (cluster.repoName && state && state.repos) {
      clusterRepo = state.repos.find(function(r) { return r.name === cluster.repoName; }) || git;
    }
    var gitRepoName = isConflict ? 'Multiple repositories' : (cluster.repoName || clusterRepo.name || 'Repository');
    var gitBranch = isConflict ? '' : (clusterRepo.branch || '');
    var gitSha = isConflict ? '' : (clusterRepo.shortSha || (clusterRepo.sha || '').slice(0, 7));
    var gitLastSync = isConflict ? '' : ago(clusterRepo.lastSync);

    var omniEndpoint = (state && state.omniEndpoint) || '';
    var omniVersion  = (state && state.omniVersion)  || '';
    var omniHealth   = (state && state.omniHealth)   || {};
    var omniStatus   = omniHealth.status || 'unknown';
    var omniColor    = omniStatus === 'healthy' ? '#fb923c' : (omniStatus === 'failed' ? '#f87171' : '#71717a');
    var omniEndpointDisplay = omniEndpoint.replace(/^https?:\/\//, '');

    var cp = cluster.controlPlane || {};
    var workers = Array.isArray(cluster.workers) ? cluster.workers : (cluster.workers ? [cluster.workers] : []);

    var st = cluster.status || '';
    var borderColor = '#3f3f46';
    if (st === 'success' || st === 'applied') borderColor = '#4ade80';
    else if (st === 'failed')    borderColor = '#f87171';
    else if (st === 'outofsync') borderColor = '#fb923c';
    else if (st === 'syncing')   borderColor = '#2dd4bf';
    else if (st === 'unmanaged') borderColor = '#52525b';
    if ((st === 'success' || st === 'applied') && (cluster.clusterReady === 'not-ready' || cluster.kubernetesApiReady === 'not-ready'))
      borderColor = '#f87171';

    // Build node groups (col 3)
    var nodeGroups = [];
    var cpMachines = cp.machines || [];
    nodeGroups.push({
      kind: 'MachineSet',
      label: cp.name || 'control-planes',
      machineClass: cp.machineClass || '',
      machines: cpMachines,
      count: cp.count || 0,
      exts: cp.extensions || [],
      color: '#FB326E',
      isPool: cpMachines.length === 0
    });
    workers.forEach(function(wk) {
      var wkMachines = wk.machines || [];
      nodeGroups.push({
        kind: 'MachineSet',
        label: wk.name || 'workers',
        machineClass: wk.machineClass || '',
        machines: wkMachines,
        count: wk.count || 0,
        exts: wk.extensions || [],
        color: '#8b5cf6',
        isPool: wkMachines.length === 0
      });
    });

    var hasIndividualMachines = nodeGroups.some(function(g) { return g.machines.length > 0; });

    // Stable graph ID — used for fold-key namespacing and zoom buttons
    var graphId = 'g' + modal.id.replace(/[^a-zA-Z0-9]/g, '_');

    // Layout constants
    var NH = 100, NW = 220, NW_MACH = 330, EW = 60;
    // Column-fold state — keyed by graphId+':col3' / graphId+':col4'
    var colFoldedMap = window.__graphColFolded || {};
    function colH(n) { return n * NH + Math.max(0, n - 1) * 12; }

    // spreadMidY: vertical centre of node[idx] in a column of n nodes,
    // spread evenly across the full maxH instead of their compact block.
    // n==1 → centred; n>1 → first node near top, last near bottom.
    function spreadMidY(n, idx) {
      if (n <= 1) return maxH / 2;
      return idx * (maxH - NH) / (n - 1) + NH / 2;
    }

    // DAG node builder — optional width, nameStyle, onclickExpr params
    // onclickExpr: raw JS expression string placed in an onclick attribute
    function dagNode(kind, name, metaHtml, badgesHtml, accentColor, iconSvg, width, nameStyle, onclickExpr) {
      var w = width || NW;
      var cls = 'dag-node' + (onclickExpr ? ' clickable' : '');
      var clickAttr = onclickExpr ? ' onclick="' + onclickExpr + '"' : '';
      return '<div class="' + cls + '" style="border-color:' + accentColor + ';width:' + w + 'px;"' + clickAttr + '>' +
        '<div class="dag-node-icon">' + iconSvg + '</div>' +
        '<div class="dag-node-body">' +
          '<div class="dag-node-kind">' + escHtml(kind) + '</div>' +
          '<div class="dag-node-name" title="' + escHtml(name) + '"' + (nameStyle ? ' style="' + nameStyle + '"' : '') + '>' + escHtml(name) + '</div>' +
          (metaHtml ? '<div class="dag-node-meta">' + metaHtml + '</div>' : '') +
          (badgesHtml ? '<div class="dag-node-badges">' + badgesHtml + '</div>' : '') +
        '</div>' +
      '</div>';
    }

    // Icons
    var gitIcon = '<svg width="16" height="16" viewBox="0 0 16 16" fill="none"><circle cx="4.5" cy="4.5" r="2" stroke="#3b82f6" stroke-width="1.5"/><circle cx="11.5" cy="4.5" r="2" stroke="#3b82f6" stroke-width="1.5"/><circle cx="4.5" cy="11.5" r="2" stroke="#3b82f6" stroke-width="1.5"/><line x1="4.5" y1="6.5" x2="4.5" y2="9.5" stroke="#3b82f6" stroke-width="1.5" stroke-linecap="round"/><line x1="4.5" y1="6.5" x2="11.5" y2="6.5" stroke="#3b82f6" stroke-width="1.5" stroke-linecap="round"/></svg>';
    var omniIcon = '<svg width="16" height="16" viewBox="0 0 1000 1008" fill="none" xmlns="http://www.w3.org/2000/svg"><defs><linearGradient id="og0" x1="499" y1="1171" x2="499" y2="-27" gradientUnits="userSpaceOnUse"><stop stop-color="#E8312C"/><stop offset="0.615" stop-color="#E2335A"/><stop offset="1" stop-color="#F77216"/></linearGradient><linearGradient id="og1" x1="1016" y1="1017" x2="93" y2="94" gradientUnits="userSpaceOnUse"><stop stop-color="#E8312C"/><stop offset="0.615" stop-color="#E2335A"/><stop offset="1" stop-color="#F77216"/></linearGradient></defs><path d="M388 84.8C449.6 23.3 549.4 23.3 610.9 84.8L915.8 389.8C977.4 451.3 977.4 551.1 915.8 612.6L610.9 917.6C549.4 979.1 449.6 979.1 388 917.6L83.1 612.6C21.6 551.1 21.6 451.3 83.1 389.8Z" stroke="url(#og0)" stroke-width="72"/><path d="M132 251.9C132 186.5 185 133.4 250.5 133.4H749.1C814.5 133.4 867.6 186.5 867.6 251.9V750.5C867.6 815.9 814.5 869 749.1 869H250.5C185 869 132 815.9 132 750.5Z" stroke="url(#og1)" stroke-width="78"/></svg>';
    // Kubernetes logo (blue hexagon + 6-spoke helm wheel, 100x100 viewBox for crisp rendering)
    var k8sIcon = '<svg width="16" height="16" viewBox="0 0 100 100"><polygon points="50,4 91,27 91,73 50,96 9,73 9,27" fill="#326CE5"/><g transform="translate(50,50)"><circle r="24" stroke="#fff" stroke-width="5.5" fill="none"/><circle r="9" fill="#fff"/><line x1="0" y1="-9" x2="0" y2="-21.2" stroke="#fff" stroke-width="5" stroke-linecap="round"/><line x1="7.79" y1="-4.5" x2="18.36" y2="-10.6" stroke="#fff" stroke-width="5" stroke-linecap="round"/><line x1="7.79" y1="4.5" x2="18.36" y2="10.6" stroke="#fff" stroke-width="5" stroke-linecap="round"/><line x1="0" y1="9" x2="0" y2="21.2" stroke="#fff" stroke-width="5" stroke-linecap="round"/><line x1="-7.79" y1="4.5" x2="-18.36" y2="10.6" stroke="#fff" stroke-width="5" stroke-linecap="round"/><line x1="-7.79" y1="-4.5" x2="-18.36" y2="-10.6" stroke="#fff" stroke-width="5" stroke-linecap="round"/></g></svg>';
    var cpIcon  = k8sIcon;
    var wpIcon  = k8sIcon;
    var extIcon = '<svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M8 1v2.5M8 12.5V15M1 8h2.5M12.5 8H15M3.4 3.4l1.77 1.77M10.83 10.83l1.77 1.77M3.4 12.6l1.77-1.77M10.83 5.17l1.77-1.77" stroke="#6366f1" stroke-width="1.5" stroke-linecap="round"/></svg>';
    // Talos logo — actual SVG paths scaled to 16x16 via viewBox (three sweeping arms, red→orange gradient)
    var talosIcon = '<svg width="16" height="16" viewBox="0 0 1000 1000" xmlns="http://www.w3.org/2000/svg"><defs>' +
      '<linearGradient id="tg0" x1="70" y1="182" x2="839" y2="182" gradientUnits="userSpaceOnUse"><stop stop-color="#E8312C"/><stop offset="0.615" stop-color="#E2335A"/><stop offset="1" stop-color="#F77216"/></linearGradient>' +
      '<linearGradient id="tg1" x1="30" y1="221" x2="414" y2="886" gradientUnits="userSpaceOnUse"><stop stop-color="#E8312C"/><stop offset="0.615" stop-color="#E2335A"/><stop offset="1" stop-color="#F77216"/></linearGradient>' +
      '<linearGradient id="tg2" x1="540" y1="965" x2="924" y2="300" gradientUnits="userSpaceOnUse"><stop stop-color="#E8312C"/><stop offset="0.615" stop-color="#E2335A"/><stop offset="1" stop-color="#F77216"/></linearGradient>' +
      '</defs>' +
      '<path d="M161.5 100C161.5 102.6 162.6 105.2 164.4 107.1C169.3 112.1 174.3 117.1 179.3 122.1C311.6 253.5 416.4 317.7 499.6 318.5C582.4 319.3 688.3 254.7 823.2 121C827.2 117.2 831 113.3 834.7 109.5L835.7 108.5C837.6 106.6 838.6 104 838.6 101.4C838.6 101 838.6 100.7 838.6 100.4C838.4 97.4 836.8 94.7 834.3 93C809.2 75.5 782.8 59.9 755.9 46.8C752 44.9 747.4 45.7 744.3 48.7C636.8 154.1 550.2 211.8 500.6 211.4C449.7 210.9 362.9 152.7 256.2 47.6C253.1 44.6 248.5 43.8 244.6 45.7C217.8 58.6 191.3 74.1 166 91.5C163.5 93.3 161.9 96 161.7 99L161.5 100Z" fill="url(#tg0)"/>' +
      '<path d="M5.1 340.5C7.4 339.1 10.1 338.7 12.7 339.4C19.5 341.1 26.3 343 33.1 344.8C213.1 393.7 321.1 452.2 363.4 523.9C405.4 595.3 402.4 719.2 354.2 903C352.8 908.3 351.4 913.5 349.9 918.7L349.6 920C348.8 922.6 347.2 924.8 344.8 926.1C344.6 926.3 344.3 926.4 344 926.6C341.3 927.9 338.2 927.9 335.5 926.6C307.7 913.5 281 898.5 256.2 881.8C252.7 879.3 251 874.9 252.1 870.8C289.5 725 296.3 621.1 271.1 578.4C245.2 534.5 151.4 488.4 7 448.6C2.9 447.5 -0.1 443.8 -0.4 439.6C-2.6 409.9 -2.5 379.2 0 348.5C0.3 345.5 1.8 342.8 4.3 341.1C4.5 340.9 4.8 340.7 5.1 340.5Z" fill="url(#tg1)"/>' +
      '<path d="M656.3 926.8C654 925.5 652.3 923.3 651.6 920.8C649.7 914 647.9 907.2 646.1 900.4C598.4 720.1 595.1 597.3 636.1 524.8C676.8 452.7 785.7 393.3 968.9 343.3C974.2 341.8 979.4 340.4 984.7 339.1L986 338.7C988.6 338.1 991.3 338.4 993.7 339.8C993.9 339.9 994.2 340.1 994.5 340.2C997 341.9 998.5 344.7 998.7 347.7C1001.3 378.2 1001.6 408.8 999.5 438.7C999.2 443 996.2 446.6 992.1 447.8C847.1 488.2 753.8 534.3 729.3 577.5C704.3 621.8 711.3 726.1 749 871.1C750.1 875.2 748.4 879.6 744.9 882C720.3 898.8 693.6 914 665.8 927.2C663.1 928.5 660 928.5 657.2 927.2C657 927.1 656.7 926.9 656.4 926.8Z" fill="url(#tg2)"/>' +
      '</svg>';
    function machIcon(c) {
      return '<svg width="16" height="16" viewBox="0 0 16 16" fill="none"><rect x="1" y="4" width="14" height="9" rx="1.5" stroke="' + c + '" stroke-width="1.5"/><path d="M4 4V3M8 4V2M12 4V3" stroke="' + c + '" stroke-width="1.5" stroke-linecap="round"/></svg>';
    }

    // Col 1: Git + Omni
    var gitMeta = (gitBranch ? escHtml(gitBranch) : '') +
      (gitSha ? (gitBranch ? ' &middot; ' : '') + gitSha : '') +
      (gitLastSync ? ' &middot; ' + escHtml(gitLastSync) : '');
    var omniMeta = (omniVersion ? escHtml(omniVersion) : '') +
      (omniStatus ? (omniVersion ? ' &middot; ' : '') + '<span style="color:' + omniColor + '">' + escHtml(omniStatus) + '</span>' : '');
    var col1Nodes = [
      dagNode('Git', gitRepoName, gitMeta, '', '#3b82f6', gitIcon),
      dagNode('Omni', omniEndpointDisplay || 'Omni Instance', omniMeta, '', omniColor, omniIcon)
    ];

    // Col 3: Node groups (ControlPlane + Workers) + cluster-level extensions (same visual level)
    // Each MachineSet card is independently clickable to collapse/expand its own machines.
    // Built before col2 so we can show the fold-badge count on the Cluster card.
    var clusterExtsList = cluster.clusterExtensions || [];
    var col3Nodes = nodeGroups.map(function(g, gi) {
      var mcLabel = g.machineClass ? escHtml(g.machineClass) : 'Manual';
      var meta = g.isPool
        ? (g.count + ' node' + (g.count !== 1 ? 's' : '') + ' - ' + mcLabel)
        : (g.machines.length + ' machine' + (g.machines.length !== 1 ? 's' : '') + ' - ' + mcLabel);
      var msonclick = g.machines.length > 0 ? 'window.__graphToggleColFold(\'' + graphId + ':col4:' + gi + '\')' : '';
      var giFolded = g.machines.length > 0 && !!colFoldedMap[graphId + ':col4:' + gi];
      var foldBadge = giFolded ? '<span class="fold-badge">+' + g.machines.length + '</span>' : '';
      return dagNode(g.kind, g.label, meta, foldBadge, g.color, cpIcon, undefined, undefined, msonclick);
    }).concat(clusterExtsList.map(function(ext) {
      return dagNode('Extension', ext, '', '', '#6366f1', extIcon);
    }));

    // Col 2: Cluster — clicking collapses/expands the MachineSet column
    var clusterMeta = 'Talos ' + escHtml(cluster.talosVersion || '\u2014') + ' &middot; K8s ' + escHtml(cluster.kubernetesVersion || '\u2014');
    var col2onclick = 'window.__graphToggleColFold(\'' + graphId + ':col3\')';
    var col3foldedEarly = !!(col3Nodes.length > 0 && colFoldedMap[graphId + ':col3']);
    var clusterFoldBadge = col3foldedEarly ? '<span class="fold-badge">+' + col3Nodes.length + '</span>' : '';
    var col2Nodes = [dagNode('Cluster', cluster.id, clusterMeta, clusterFoldBadge, borderColor, talosIcon, undefined, undefined, col2onclick)];

    // Col 4 (optional): Individual machines — per-MachineSet fold, full UUID, monospace name, wider card
    var col4Nodes = [], machineConnections = [], col4Uuids = [];
    var col4GiOf = [], col4GroupSizes = {}, groupStartYMap = {};
    var INTRA_GAP = 8, INTER_GAP = 32;
    var machY = [], groupOrder = [], col4TotalH = 0;
    var hostnames = cluster.machineHostnames || {};
    if (hasIndividualMachines) {
      var yOff = 0;
      nodeGroups.forEach(function(g, gi) {
        var giFolded = !!colFoldedMap[graphId + ':col4:' + gi];
        if (giFolded) return;
        if (groupOrder.length > 0) yOff += INTER_GAP;
        groupOrder.push(gi);
        groupStartYMap[gi] = yOff;
        var localIdx = 0;
        g.machines.forEach(function(mid) {
          var flatIdx = col4Nodes.length;
          col4GroupSizes[gi] = (col4GroupSizes[gi] || 0) + 1;
          col4GiOf.push(gi);
          machY.push(yOff + localIdx * (NH + INTRA_GAP) + NH / 2);
          machineConnections.push([gi, flatIdx]);
          col4Uuids.push(mid);
          var hn = hostnames[mid] ? escHtml(hostnames[mid]) : '';
          col4Nodes.push(dagNode('Machine', mid, hn, '', g.color, machIcon(g.color), NW_MACH,
            'font-family:\'SF Mono\',\'Fira Code\',monospace;font-size:11px;'));
          localIdx++;
        });
        var gs = g.machines.length;
        yOff += gs * NH + Math.max(0, gs - 1) * INTRA_GAP;
      });
      col4TotalH = yOff;
    }

    // Extensions: collect unique names from nodeGroup and machine sources.
    // Cluster-level extensions are placed directly in col3 (alongside nodeGroups).
    // extConns stores {srcKeys, srcIdx, tgtIdx} for variable-height SVG routing.
    var extNodes = [], extNameToIdx = {}, extConns = [];
    function addExt(name) {
      if (!(name in extNameToIdx)) {
        var eidx = extNodes.length;
        extNameToIdx[name] = eidx;
        extNodes.push(dagNode('Extension', name, '', '', '#6366f1', extIcon));
      }
      return extNameToIdx[name];
    }
    // NodeGroup-level extensions → connect from each visible machine in that group.
    // If the MachineSet is collapsed its machines aren't in machineConnections, so
    // no extension links are drawn for it.
    machineConnections.forEach(function(mc) {
      var gi = mc[0], mi = mc[1];
      nodeGroups[gi].exts.forEach(function(ext) {
        extConns.push({srcCol: 4, srcIdx: mi, tgtIdx: addExt(ext)});
      });
    });
    // Machine-level extensions → connect from that specific machine card.
    var machExtMap = cluster.machineExtensions || {};
    col4Uuids.forEach(function(uuid, mi) {
      (machExtMap[uuid] || []).forEach(function(ext) {
        extConns.push({srcCol: 4, srcIdx: mi, tgtIdx: addExt(ext)});
      });
    });
    var hasExtensions = extNodes.length > 0;

    // Column-fold flags — computed after all node arrays are built
    var col3folded = !!(col3Nodes.length > 0 && colFoldedMap[graphId + ':col3']);
    // col4 is visible as long as at least one MachineSet has machines AND isn't folded
    var anyMachinesVisible = col4Nodes.length > 0;

    // Max height across visible columns only
    var visCounts = [2, 1];
    if (!col3folded) {
      visCounts.push(col3Nodes.length);
      if (hasExtensions) visCounts.push(extNodes.length);
    }
    var maxH = Math.max.apply(null, visCounts.map(colH).concat([NH]));
    if (!col3folded && anyMachinesVisible) {
      maxH = Math.max(maxH, col4TotalH);
    }

    // col3NodeY[i]: absolute vertical centre of col3 node i.
    // For MachineSet nodes that have visible machines, align to the centre of their group in col4.
    // Everything else (pool-based, folded, cluster exts) falls back to even spread.
    var col3NodeY = (function() {
      var col4Off = Math.max(0, (maxH - col4TotalH) / 2);
      return col3Nodes.map(function(_, i) {
        if (i < nodeGroups.length) {
          var gi = i;
          if (!colFoldedMap[graphId + ':col4:' + gi] && nodeGroups[gi].machines && nodeGroups[gi].machines.length > 0 && (gi in groupStartYMap)) {
            var gs = nodeGroups[gi].machines.length;
            var blockH = gs * NH + Math.max(0, gs - 1) * INTRA_GAP;
            return col4Off + groupStartYMap[gi] + blockH / 2;
          }
        }
        return spreadMidY(col3Nodes.length, i);
      });
    })();

    // Enforce minimum card separation in col3: when some MachineSets are col4-folded their
    // Y falls back to spreadMidY, which can collide with col4-anchored neighbours.
    (function() {
      var MIN_GAP = 8;
      for (var ii = 1; ii < col3NodeY.length; ii++) {
        var minCenter = col3NodeY[ii - 1] + NH + MIN_GAP;
        if (col3NodeY[ii] < minCenter) col3NodeY[ii] = minCenter;
      }
      // Expand maxH so the container is tall enough to contain all adjusted cards
      if (col3NodeY.length > 0) {
        var lastBottom = col3NodeY[col3NodeY.length - 1] + NH / 2;
        if (lastBottom > maxH) maxH = lastBottom;
      }
    })();

    // Standard SVG edge — uses spreadMidY so connection points match wrapCol positions
    function edgeSvg(leftCount, rightCount, connections) {
      var paths = '';
      connections.forEach(function(conn) {
        var ly = spreadMidY(leftCount, conn[0]);
        var ry = spreadMidY(rightCount, conn[1]);
        var bx = EW * 0.5;
        paths += '<path d="M0,' + ly.toFixed(1) + ' C' + bx.toFixed(1) + ',' + ly.toFixed(1) + ' ' + bx.toFixed(1) + ',' + ry.toFixed(1) + ' ' + EW + ',' + ry.toFixed(1) + '" stroke="#3f3f46" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>';
        paths += '<polygon points="' + EW + ',' + ry.toFixed(1) + ' ' + (EW-5) + ',' + (ry-3).toFixed(1) + ' ' + (EW-5) + ',' + (ry+3).toFixed(1) + '" fill="#52525b"/>';
      });
      return '<svg width="' + EW + '" height="' + maxH + '" style="flex-shrink:0;">' + paths + '</svg>';
    }

    // Mixed-source SVG edge: conn = {srcCol, srcIdx, tgtIdx}
    function edgeSvgMixed(connections, rightCount) {
      var col4Off = Math.max(0, (maxH - col4TotalH) / 2);
      var paths = '';
      connections.forEach(function(conn) {
        var ly = conn.srcCol === 4
          ? machY[conn.srcIdx] + col4Off
          : spreadMidY(col3Nodes.length, conn.srcIdx);
        var ry = spreadMidY(rightCount, conn.tgtIdx);
        var bx = EW * 0.5;
        paths += '<path d="M0,' + ly.toFixed(1) + ' C' + bx.toFixed(1) + ',' + ly.toFixed(1) + ' ' + bx.toFixed(1) + ',' + ry.toFixed(1) + ' ' + EW + ',' + ry.toFixed(1) + '" stroke="#3f3f46" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>';
        paths += '<polygon points="' + EW + ',' + ry.toFixed(1) + ' ' + (EW-5) + ',' + (ry-3).toFixed(1) + ' ' + (EW-5) + ',' + (ry+3).toFixed(1) + '" fill="#52525b"/>';
      });
      return '<svg width="' + EW + '" height="' + maxH + '" style="flex-shrink:0;">' + paths + '</svg>';
    }

    // Spread each column evenly across maxH
    function wrapCol(nodes) {
      var n = nodes.length;
      if (n === 1) {
        var mt = Math.round((maxH - NH) / 2);
        return '<div style="display:flex;flex-direction:column;margin-top:' + mt + 'px;">' + nodes.join('') + '</div>';
      }
      var gap = Math.max(4, Math.round((maxH - n * NH) / (n - 1)));
      return '<div style="display:flex;flex-direction:column;gap:' + gap + 'px;">' + nodes.join('') + '</div>';
    }

    // Compact column: fixed gap, centered in maxH
    function wrapColCompact(nodes, gapPx) {
      var n = nodes.length;
      var totalH = n * NH + (n - 1) * gapPx;
      var mt = Math.round(Math.max(0, (maxH - totalH) / 2));
      if (n === 1) return '<div style="display:flex;flex-direction:column;margin-top:' + mt + 'px;">' + nodes.join('') + '</div>';
      return '<div style="display:flex;flex-direction:column;gap:' + gapPx + 'px;margin-top:' + mt + 'px;">' + nodes.join('') + '</div>';
    }

    // Cluster → col3 (uses col3NodeY so arrows land at the MachineSet's true centre)
    function edgeSvgCol2ToCol3() {
      var paths = '';
      col3Nodes.forEach(function(_, i) {
        var ly = spreadMidY(1, 0), ry = col3NodeY[i], bx = EW * 0.5;
        paths += '<path d="M0,' + ly.toFixed(1) + ' C' + bx + ',' + ly.toFixed(1) + ' ' + bx + ',' + ry.toFixed(1) + ' ' + EW + ',' + ry.toFixed(1) + '" stroke="#3f3f46" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>';
        paths += '<polygon points="' + EW + ',' + ry.toFixed(1) + ' ' + (EW-5) + ',' + (ry-3).toFixed(1) + ' ' + (EW-5) + ',' + (ry+3).toFixed(1) + '" fill="#52525b"/>';
      });
      return '<svg width="' + EW + '" height="' + maxH + '" style="flex-shrink:0;">' + paths + '</svg>';
    }

    // Col4: render each MachineSet's machines as a grouped block, centered vertically
    function wrapCol4Grouped() {
      var col4Off = Math.round(Math.max(0, (maxH - col4TotalH) / 2));
      var html = '<div style="display:flex;flex-direction:column;margin-top:' + col4Off + 'px;">';
      groupOrder.forEach(function(gi, idx) {
        if (idx > 0) html += '<div style="height:' + INTER_GAP + 'px;flex-shrink:0;"></div>';
        var groupNodes = [];
        col4Nodes.forEach(function(node, fi) { if (col4GiOf[fi] === gi) groupNodes.push(node); });
        html += '<div style="display:flex;flex-direction:column;gap:' + INTRA_GAP + 'px;">' + groupNodes.join('') + '</div>';
      });
      html += '</div>';
      return html;
    }

    // Edge col3 → col4: source from col3NodeY (MachineSet centred on its group), target from machY
    function edgeSvgGrouped() {
      var col4Off = Math.max(0, (maxH - col4TotalH) / 2);
      var paths = '';
      machineConnections.forEach(function(mc) {
        var gi = mc[0], fi = mc[1];
        var ly = col3NodeY[gi];
        var ry = machY[fi] + col4Off;
        var bx = EW * 0.5;
        paths += '<path d="M0,' + ly.toFixed(1) + ' C' + bx.toFixed(1) + ',' + ly.toFixed(1) + ' ' + bx.toFixed(1) + ',' + ry.toFixed(1) + ' ' + EW + ',' + ry.toFixed(1) + '" stroke="#3f3f46" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>';
        paths += '<polygon points="' + EW + ',' + ry.toFixed(1) + ' ' + (EW-5) + ',' + (ry-3).toFixed(1) + ' ' + (EW-5) + ',' + (ry+3).toFixed(1) + '" fill="#52525b"/>';
      });
      return '<svg width="' + EW + '" height="' + maxH + '" style="flex-shrink:0;">' + paths + '</svg>';
    }

    var COL1_GAP = 16;
    var col1TotalH = col1Nodes.length * NH + (col1Nodes.length - 1) * COL1_GAP;
    var col1Off = Math.max(0, (maxH - col1TotalH) / 2);
    var col1Y = col1Nodes.map(function(_, i) { return col1Off + i * (NH + COL1_GAP) + NH / 2; });
    var edge1 = (function() {
      var paths = '';
      var ry = spreadMidY(1, 0);
      col1Y.forEach(function(ly) {
        var bx = EW * 0.5;
        paths += '<path d="M0,' + ly.toFixed(1) + ' C' + bx.toFixed(1) + ',' + ly.toFixed(1) + ' ' + bx.toFixed(1) + ',' + ry.toFixed(1) + ' ' + EW + ',' + ry.toFixed(1) + '" stroke="#3f3f46" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>';
        paths += '<polygon points="' + EW + ',' + ry.toFixed(1) + ' ' + (EW-5) + ',' + (ry-3).toFixed(1) + ' ' + (EW-5) + ',' + (ry+3).toFixed(1) + '" fill="#52525b"/>';
      });
      return '<svg width="' + EW + '" height="' + maxH + '" style="flex-shrink:0;">' + paths + '</svg>';
    })();
    var edge2 = col3folded ? '' : edgeSvgCol2ToCol3();
    var edge3 = (!col3folded && anyMachinesVisible) ? edgeSvgGrouped() : '';
    var edge4 = (!col3folded && hasExtensions) ? edgeSvgMixed(extConns, extNodes.length) : '';

    // col3: absolutely positioned so each MachineSet sits at its computed Y centre
    function wrapCol3Positioned() {
      var html = '<div style="position:relative;width:' + NW + 'px;height:' + maxH + 'px;flex-shrink:0;">';
      col3Nodes.forEach(function(node, i) {
        var top = Math.round(col3NodeY[i] - NH / 2);
        html += '<div style="position:absolute;top:' + top + 'px;left:0;">' + node + '</div>';
      });
      html += '</div>';
      return html;
    }

    // col3 segment — hidden when col3folded
    var col3segment = col3folded ? '' : (
      wrapCol3Positioned() +
      (anyMachinesVisible ? edge3 + wrapCol4Grouped() : '') +
      (hasExtensions ? edge4 + wrapCol(extNodes) : '')
    );

    return '<div class="cluster-graph">' +
      '<div class="cluster-graph-toolbar">' +
        '<button class="cluster-graph-zoom-btn" title="Collapse all" onclick="window.__graphCollapseAll(\'' + graphId + '\')">&#8991;</button>' +
        '<button class="cluster-graph-zoom-btn" title="Expand all" onclick="window.__graphExpandAll(\'' + graphId + '\')">&#8990;</button>' +
        '<span class="graph-toolbar-sep"></span>' +
        '<button class="cluster-graph-zoom-btn" onclick="window.__graphZoom(\'out\',\'' + graphId + '\')">&#8722;</button>' +
        '<span class="graph-zoom-level">100%</span>' +
        '<button class="cluster-graph-zoom-btn" onclick="window.__graphZoom(\'reset\',\'' + graphId + '\')">&#8635;</button>' +
        '<button class="cluster-graph-zoom-btn" onclick="window.__graphZoom(\'in\',\'' + graphId + '\')">&#43;</button>' +
      '</div>' +
      '<div class="cluster-graph-canvas" onwheel="window.__graphZoomWheel(event,this)" onmousedown="window.__graphDragStart(event,this)" onmousemove="window.__graphDragMove(event,this)" onmouseup="window.__graphDragEnd(this)" onmouseleave="window.__graphDragEnd(this)">' +
        '<div id="' + graphId + '" class="cluster-graph-inner" style="align-items:flex-start;gap:0;">' +
          wrapColCompact(col1Nodes, COL1_GAP) + edge1 +
          wrapCol(col2Nodes) + edge2 +
          col3segment +
        '</div>' +
      '</div>' +
    '</div>';
  }

  function renderDrawer() {
    return '<div class="drawer-backdrop' + (currentModal ? ' show' : '') + '" onclick="window.__closeModal()"></div>' +
    '<div class="drawer' + (currentModal ? ' show' : '') + '">' +
      '<div class="drawer-header">' +
        '<div class="drawer-title">' + (currentModal ? escHtml(currentModal.id) : '') + '</div>' +
        '<button class="drawer-close" onclick="window.__closeModal()">&times;</button>' +
      '</div>' +
      (currentModal ?
        '<div class="drawer-tabs">' +
          (currentModal.error ? '<button class="drawer-tab ' + (currentModal.activeTab === 'error' ? 'active' : '') + '" onclick="window.__setModalTab(\'error\')">Error</button>' : '') +
          '<button class="drawer-tab ' + (currentModal.activeTab === 'live' ? 'active' : '') + '" onclick="window.__setModalTab(\'live\')">Live</button>' +
          (currentModal.diff ? '<button class="drawer-tab ' + (currentModal.activeTab === 'diff' ? 'active' : '') + '" onclick="window.__setModalTab(\'diff\')">Diff</button>' : '') +
          (currentModal.type === 'cluster' ? '<button class="drawer-tab ' + (currentModal.activeTab === 'graph' ? 'active' : '') + '" onclick="window.__setModalTab(\'graph\')">Graph</button>' : '') +
          (currentModal.type === 'machineclass' ? '<label class="mc-ignored-toggle"><input type="checkbox" class="mc-ignored-cb"' + (mcLiveShowMeta ? ' checked' : '') + ' onchange="window.__toggleMcLiveMeta()"> Show Ignored Fields</label>' : '') +
        '</div>' : '') +
      '<div class="drawer-body' + (currentModal && currentModal.activeTab === 'graph' ? ' graph-mode' : '') + (currentModal && currentModal.activeTab === 'live' && (currentModal.type === 'machineclass' || currentModal.type === 'cluster') ? ' mc-live-mode' : '') + '">' +
        (currentModal ?
          (currentModal.activeTab === 'error' ? '<div style="color:#f87171;white-space:pre-wrap;">' + escHtml(currentModal.error) + '</div>' :
           currentModal.activeTab === 'live' ? (currentModal.type === 'machineclass' ? renderMcLiveContent(currentModal.liveContent) : (currentModal.type === 'cluster' ? renderClusterLiveContent(currentModal.liveContent) : (currentModal.liveContent ? '<pre style="margin:0;white-space:pre-wrap;">' + escHtml(currentModal.liveContent) + '</pre>' : '<div style="color:#71717a;text-align:center;padding:40px;">No live state available</div>'))) :
           currentModal.activeTab === 'diff' ? (currentModal.diff ? (currentModal.type === 'machineclass' ? formatMachineClassDiff(currentModal.diff) : '<pre style="margin:0;white-space:pre-wrap;">' + formatDiff(currentModal.diff) + '</pre>') : '<div style="color:#71717a;text-align:center;padding:40px;">' + (currentModal.status === 'unmanaged' ? 'No diff \u2014 this cluster has no template in Git.' : 'No diff available') + '</div>') :
           currentModal.activeTab === 'graph' ? renderClusterGraph(currentModal) :
           '<div style="color:#71717a;text-align:center;padding:40px;">No content available</div>')
        : '') +
      '</div>' +
    '</div>' +
    '<div class="modal ' + (confirmModal ? 'show' : '') + '" onclick="if(event.target === this) window.__closeConfirmModal()">' +
      '<div class="modal-content confirm-modal" onclick="event.stopPropagation()">' +
        '<div class="modal-header">' +
          '<div class="modal-title">' + (confirmModal ? confirmModal.title : '') + '</div>' +
          '<button class="modal-close" onclick="window.__closeConfirmModal()">&times;</button>' +
        '</div>' +
        '<div class="modal-body confirm-body">' +
          '<div class="confirm-icon">⚠️</div>' +
          '<div class="confirm-message">' + (confirmModal ? confirmModal.message : '') + '</div>' +
          (confirmModal && confirmModal.requireInput
            ? '<input class="confirm-input" type="text" value="' + escHtml(confirmInput) + '" oninput="window.__setConfirmInput(this.value)" placeholder="' + escHtml(confirmModal.inputPrompt || confirmModal.requireInput) + '" />'
            : '') +
          '<div class="confirm-actions">' +
            '<button class="btn-sort btn-primary" onclick="window.__closeConfirmModal()">Cancel</button>' +
            '<button id="confirm-ok-btn" class="btn-sort btn-primary" ' + (confirmModal && confirmModal.requireInput && confirmInput !== confirmModal.requireInput ? 'disabled' : '') + ' onclick="window.__confirmAction()">OK</button>' +
          '</div>' +
        '</div>' +
      '</div>' +
    '</div>' +
    '<div class="modal ' + (logsModal ? 'show' : '') + '" onclick="if(event.target === this) window.__closeLogsModal()">' +
      '<div class="modal-content" style="max-width:1000px" onclick="event.stopPropagation()">' +
        '<div class="logs-modal-header">' +
          '<div class="logs-modal-title">Logs</div>' +
          '<div class="logs-modal-actions">' +
            '<button class="btn-sort btn-primary" onclick="window.__downloadLogs()">Download Logs</button>' +
            '<button class="modal-close" onclick="window.__closeLogsModal()">&times;</button>' +
          '</div>' +
        '</div>' +
        '<div class="logs-container" id="logs-modal-container" style="height:600px;padding:12px 0">' +
          (state && state.logs && state.logs.length > 0
            ? state.logs.map(function(l) {
                return '<div class="log-entry">' +
                  '<span class="log-msg">' + l.message + '</span></div>';
              }).join('')
            : '<div class="log-entry" style="color:#52525b">No logs yet</div>') +
        '</div>' +
      '</div>' +
    '</div>' +
    '<div class="modal ' + (logFilesList !== null || logFilesLoading ? 'show' : '') + '" onclick="if(event.target===this)window.__closeLogFilesModal()">' +
      '<div class="modal-content" style="max-width:560px" onclick="event.stopPropagation()">' +
        '<div class="logs-modal-header">' +
          '<div class="logs-modal-title">Log Files</div>' +
          '<button class="modal-close" onclick="window.__closeLogFilesModal()">&times;</button>' +
        '</div>' +
        '<div style="padding:16px 24px;min-height:80px">' +
          (logFilesLoading
            ? '<div style="color:#71717a;text-align:center;padding:24px">Loading...</div>'
            : (logFilesList && logFilesList.length > 0
                ? '<table style="width:100%;border-collapse:collapse;font-size:12px">' +
                    '<thead><tr>' +
                      '<th style="text-align:left;color:#71717a;font-weight:400;padding:4px 8px">Date</th>' +
                      '<th style="text-align:right;color:#71717a;font-weight:400;padding:4px 8px">Size</th>' +
                      '<th style="padding:4px 8px"></th>' +
                    '</tr></thead><tbody>' +
                    (logFilesList || []).map(function(f) {
                      return '<tr style="border-top:1px solid #27272a">' +
                        '<td style="padding:6px 8px;color:#e4e4e7">' + escHtml(f.date) + '</td>' +
                        '<td style="padding:6px 8px;color:#71717a;text-align:right">' + formatBytes(f.size) + '</td>' +
                        '<td style="padding:6px 8px;text-align:right"><button class="btn-sort btn-primary" onclick="window.__downloadLogFile(\'' + escHtml(f.date) + '\')">Download</button></td>' +
                      '</tr>';
                    }).join('') +
                    '</tbody></table>'
                : '<div style="color:#71717a;text-align:center;padding:24px">No log files found</div>')) +
        '</div>' +
      '</div>' +
    '</div>';
  }

  function showLogsModal() {
    logsModal = true;
    render();
  }

  function closeLogsModal() {
    logsModal = false;
    render();
  }

  function updateLogsInPlace() {
    var ids = ['logs-modal-container', 'logs-page-container'];
    ids.forEach(function(id) {
      var el = document.getElementById(id);
      if (!el) return;
      var atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
      el.innerHTML = state && state.logs && state.logs.length > 0
        ? state.logs.map(function(l) {
            return '<div class="log-entry"><span class="log-msg">' + l.message + '</span></div>';
          }).join('')
        : '<div class="log-entry" style="color:#52525b">No logs yet</div>';
      if (atBottom) el.scrollTop = el.scrollHeight;
    });
  }

  function renderMainOnly() {
    // Re-render only the main app content, leaving the modals container untouched
    if (!state) return;
    var s = state;
    if (clusterDetailId && currentRoute.startsWith('/clusters/')) {
      // Use surgical in-place update to avoid replacing the whole page DOM
      // (which causes the graph to jump before the transform can be restored).
      updateClusterDetailInPlace();
      return;
    } else if (currentRoute === '/clusters') {
      restoreDefaultLayout();
      var activeIsSearch2 = document.activeElement && document.activeElement.tagName === 'INPUT' && document.activeElement.placeholder === 'Search clusters...';
      var ss2 = activeIsSearch2 ? document.activeElement.selectionStart : null;
      var se2 = activeIsSearch2 ? document.activeElement.selectionEnd   : null;
      app.innerHTML = renderClustersView(s);
      if (activeIsSearch2) {
        var inp2 = app.querySelector('input[placeholder="Search clusters..."]');
        if (inp2) { inp2.focus(); inp2.setSelectionRange(ss2, se2); }
      }
    } else if (currentRoute === '/machineclasses') {
      restoreDefaultLayout();
      var activeIsMcSearch = document.activeElement && document.activeElement.tagName === 'INPUT' && document.activeElement.placeholder === 'Search machine classes...';
      var mss = activeIsMcSearch ? document.activeElement.selectionStart : null;
      var mse = activeIsMcSearch ? document.activeElement.selectionEnd   : null;
      app.innerHTML = renderMachineClassesView(s);
      if (activeIsMcSearch) {
        var mci = app.querySelector('input[placeholder="Search machine classes..."]');
        if (mci) { mci.focus(); mci.setSelectionRange(mss, mse); }
      }
    } else if (currentRoute === '/instances') {
      restoreDefaultLayout();
      app.innerHTML = renderInstancesView(s);
    } else if (currentRoute === '/repos') {
      restoreDefaultLayout();
      app.innerHTML = renderReposView(s);
    } else if (currentRoute === '/users') {
      restoreDefaultLayout();
      app.innerHTML = renderUsersView(s);
    } else if (currentRoute === '/logs') {
      restoreDefaultLayout();
      app.innerHTML = renderLogsView(s);
    } else {
      window.location.replace('/clusters');
    }
  }

  function downloadTodaysLogs() {
    var today = new Date().toISOString().slice(0, 10);
    window.location.href = '/api/logs/download?date=' + today;
  }

  function downloadLogFile(date) {
    window.location.href = '/api/logs/download?date=' + date;
  }

  var logFilesList = null;
  var logFilesLoading = false;

  function openLogFilesModal() {
    logFilesList = null;
    logFilesLoading = true;
    render();
    fetch('/api/logs/files')
      .then(function(r) { return r.json(); })
      .then(function(d) { logFilesList = d; logFilesLoading = false; render(); })
      .catch(function() { logFilesList = []; logFilesLoading = false; render(); });
  }

  function closeLogFilesModal() {
    logFilesList = null;
    logFilesLoading = false;
    render();
  }

  function formatBytes(b) {
    if (b < 1024) return b + ' B';
    if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB';
    return (b / (1024 * 1024)).toFixed(1) + ' MB';
  }

  function paginateItems(items, page) {
    var start = (page - 1) * pageSize;
    var end = start + pageSize;
    return items.slice(start, end);
  }

  function paginateWithSize(items, page, size) {
    var start = (page - 1) * size;
    return items.slice(start, start + size);
  }

  function renderPaginationSized(items, currentPage, onPageChange, size) {
    var totalPages = Math.ceil(items.length / size);
    if (totalPages <= 1) return '';
    var pages = '';
    for (var i = 1; i <= totalPages; i++) {
      pages += '<button class="page-btn ' + (i === currentPage ? 'active' : '') + '" onclick="' + onPageChange + '(' + i + ')">' + i + '</button>';
    }
    return '<div class="pagination">' +
      '<button class="page-btn" onclick="' + onPageChange + '(' + (currentPage - 1) + ')" ' + (currentPage === 1 ? 'disabled' : '') + '>&laquo;</button>' +
      pages +
      '<button class="page-btn" onclick="' + onPageChange + '(' + (currentPage + 1) + ')" ' + (currentPage === totalPages ? 'disabled' : '') + '>&raquo;</button>' +
    '</div>';
  }

  function renderPageSizeBar(sizes, current, onChangeFn) {
    return sizes.map(function(n) {
      var label = n === 0 ? 'All' : String(n);
      var isActive = n === current;
      return '<button class="page-size-btn' + (isActive ? ' active' : '') + '" onclick="' + onChangeFn + '(' + n + ')">' + label + '</button>';
    }).join('');
  }

  // diffMatchLabels extracts added/removed matchLabel entries from a unified
  // diff string. Returns { removed: {key: val}, added: {key: val} } or null.
  // parseMatchLabelsFromBlock extracts matchLabel key=value pairs from a plain
  // YAML text block (no unified-diff prefixes).
  function parseMatchLabelsFromBlock(text) {
    var labels = {};
    var lines = text.split('\n');
    var inML = false;
    for (var i = 0; i < lines.length; i++) {
      var line = lines[i];
      var trimmed = line.trim();
      if (!trimmed) continue;
      if (/^matchlabels:/i.test(trimmed)) { inML = true; continue; }
      if (inML) {
        var indent = line.search(/\S/);
        // Exit section when we reach a key at the same or lower indentation
        if (indent === 0 && trimmed && !/^-/.test(trimmed)) { inML = false; continue; }
        var li = trimmed.match(/^-\s+(.+)$/);
        if (li) {
          var item = li[1].trim();
          var eq = item.match(/^([^=]+?)\s*=\s*(.*)$/);
          if (eq) labels[eq[1].trim()] = eq[2].trim();
        }
      }
    }
    return labels;
  }

  // diffMatchLabels parses the Omni diff format which contains two full YAML
  // blocks separated by "--- live" and "+++ desired" section headers.
  // Returns { removed: {k:v}, added: {k:v} } for matchLabel changes, or null.
  function diffMatchLabels(diff) {
    if (!diff) return null;
    var text = diff.replace(/\\n/g, '\n');
    // Split into live and desired blocks
    var liveStart   = text.search(/^---\s+live\s*$/m);
    var desiredStart = text.search(/^\+\+\+\s+desired\s*$/m);
    if (liveStart < 0 || desiredStart < 0) return null;
    var liveBlock    = text.slice(liveStart, desiredStart);
    var desiredBlock = text.slice(desiredStart);
    var liveLabels    = parseMatchLabelsFromBlock(liveBlock);
    var desiredLabels = parseMatchLabelsFromBlock(desiredBlock);
    var removed = {}, added = {};
    // Labels in live but missing or changed in desired
    Object.keys(liveLabels).forEach(function(k) {
      if (liveLabels[k] !== desiredLabels[k]) removed[k] = liveLabels[k];
    });
    // Labels in desired but missing or changed in live
    Object.keys(desiredLabels).forEach(function(k) {
      if (desiredLabels[k] !== liveLabels[k]) added[k] = desiredLabels[k];
    });
    var hasChanges = Object.keys(removed).length > 0 || Object.keys(added).length > 0;
    return hasChanges ? { removed: removed, added: added } : null;
  }

  function parseMachineClassSpec(yaml, id) {
    if (!yaml) return null;
    var text = yaml.replace(/\\n/g, '\n');
    // If this is a multi-doc YAML file, isolate the document for this MC
    if (id) {
      var docs = text.split(/\n---/);
      for (var d = 0; d < docs.length; d++) {
        if (docs[d].indexOf('id: ' + id) >= 0) { text = docs[d]; break; }
      }
    }
    var lines = text.split('\n');
    var result = { matchLabels: {}, providerId: '', providerData: {} };
    var i = 0;
    while (i < lines.length) {
      var line = lines[i];
      var trimmed = line.trim();
      if (!trimmed) { i++; continue; }
      var indent = line.search(/\S/);
      if (/^matchlabels:/i.test(trimmed)) {
        var base = indent;
        i++;
        while (i < lines.length) {
          var sl = lines[i]; var st = sl.trim();
          if (!st) { i++; continue; }
          if (sl.search(/\S/) <= base) break;
          var li = st.match(/^-\s+(.+)$/);
          if (li) {
            var item = li[1].trim();
            var eq = item.match(/^([^=]+?)\s*=\s*(.*)$/);
            if (eq) { result.matchLabels[eq[1].trim()] = eq[2].trim(); }
            else { var co = item.match(/^([^:]+?):\s*(.*)$/); if (co) result.matchLabels[co[1].trim()] = co[2].trim(); }
          } else {
            var kv = st.match(/^([^:]+?):\s*(.*)$/);
            if (kv) result.matchLabels[kv[1].trim()] = kv[2].trim();
          }
          i++;
        }
        continue;
      }
      if (/^providerId:/i.test(trimmed)) {
        result.providerId = trimmed.replace(/^providerId:\s*/i, '').trim();
      }
      if (/^providerData:\s*\|/i.test(trimmed)) {
        var base2 = indent;
        i++;
        while (i < lines.length) {
          var sl2 = lines[i]; var st2 = sl2.trim();
          if (!st2) { i++; continue; }
          if (sl2.search(/\S/) <= base2) break;
          var kv2 = st2.match(/^([^:]+):\s*(.*)$/);
          if (kv2) result.providerData[kv2[1].trim()] = kv2[2].trim();
          i++;
        }
        continue;
      }
      i++;
    }
    return result;
  }

  function setMcPageSize(n) {
    mcPageSize = n;
    machineClassPage = 1;
    render();
  }

  function setClusterPageSize(n) {
    clusterPageSize = n;
    clusterPage = 1;
    render();
  }

  function renderPagination(items, currentPage, onPageChange) {
    var totalPages = Math.ceil(items.length / pageSize);
    if (totalPages <= 1) return '';

    var pages = '';
    for (var i = 1; i <= totalPages; i++) {
      pages += '<button class="page-btn ' + (i === currentPage ? 'active' : '') + '" onclick="' + onPageChange + '(' + i + ')">' + i + '</button>';
    }

    return '<div class="pagination">' +
      '<button class="page-btn" onclick="' + onPageChange + '(' + (currentPage - 1) + ')" ' + (currentPage === 1 ? 'disabled' : '') + '>&laquo;</button>' +
      pages +
      '<button class="page-btn" onclick="' + onPageChange + '(' + (currentPage + 1) + ')" ' + (currentPage === totalPages ? 'disabled' : '') + '>&raquo;</button>' +
    '</div>';
  }

  function render() {
    if (!state) {
      return;
    }
    var s = state;

    if (clusterDetailId && currentRoute.startsWith('/clusters/')) {
      app.innerHTML = renderClusterDetailPage(s);
      applyDetailPageLayout();
      modalsEl.innerHTML = renderDrawer();
      if (clusterDetailTab === 'graph') {
        requestAnimationFrame(function() {
          var inner = document.querySelector('#cluster-detail-body .cluster-graph-inner');
          if (inner) restoreGraphTransform(inner.id);
        });
      }
      return;
    }

    restoreDefaultLayout();
    if (currentRoute === '/clusters') {
      var activeIsSearch = document.activeElement && document.activeElement.tagName === 'INPUT' && document.activeElement.placeholder === 'Search clusters...';
      var searchSelStart = activeIsSearch ? document.activeElement.selectionStart : null;
      var searchSelEnd   = activeIsSearch ? document.activeElement.selectionEnd   : null;
      app.innerHTML = renderClustersView(s);
      if (activeIsSearch) {
        var inp = app.querySelector('input[placeholder="Search clusters..."]');
        if (inp) { inp.focus(); inp.setSelectionRange(searchSelStart, searchSelEnd); }
      }
    } else if (currentRoute === '/machineclasses') {
      var activeIsMcSearch2 = document.activeElement && document.activeElement.tagName === 'INPUT' && document.activeElement.placeholder === 'Search machine classes...';
      var mss2 = activeIsMcSearch2 ? document.activeElement.selectionStart : null;
      var mse2 = activeIsMcSearch2 ? document.activeElement.selectionEnd   : null;
      app.innerHTML = renderMachineClassesView(s);
      if (activeIsMcSearch2) {
        var mci2 = app.querySelector('input[placeholder="Search machine classes..."]');
        if (mci2) { mci2.focus(); mci2.setSelectionRange(mss2, mse2); }
      }
    } else if (currentRoute === '/instances') {
      app.innerHTML = renderInstancesView(s);
    } else if (currentRoute === '/repos') {
      app.innerHTML = renderReposView(s);
    } else if (currentRoute === '/users') {
      app.innerHTML = renderUsersView(s);
    } else if (currentRoute === '/logs') {
      var activeIsLogSearch = document.activeElement && document.activeElement.tagName === 'INPUT' && document.activeElement.placeholder === 'Search logs...';
      var lss = activeIsLogSearch ? document.activeElement.selectionStart : null;
      var lse = activeIsLogSearch ? document.activeElement.selectionEnd   : null;
      app.innerHTML = renderLogsView(s);
      if (activeIsLogSearch) {
        var li = app.querySelector('input[placeholder="Search logs..."]');
        if (li) { li.focus(); li.setSelectionRange(lss, lse); }
      }
    } else {
      window.location.replace('/clusters');
    }

    modalsEl.innerHTML = renderDrawer();

    // Restore (or initially centre) the graph transform after a state re-render.
    if (currentModal && currentModal.activeTab === 'graph') {
      requestAnimationFrame(function() {
        var inner = document.querySelector('.cluster-graph-inner');
        if (inner) restoreGraphTransform(inner.id);
      });
    }

    if (logsModal) {
      var el = document.getElementById('logs-modal-container');
      if (el) el.scrollTop = el.scrollHeight;
    }
  }

  window.__triggerReconcile = triggerReconcile;
  window.__checkGit = checkGit;
  window.__refreshMC = refreshMC;
  window.__setClusterAutoSync = setClusterAutoSync;
  window.__setMCAutoSync = setMCAutoSync;
  window.__syncMachineClass = syncMachineClass;
  window.__refreshSingleMC = refreshSingleMC;
  window.__forceSync = forceSync;
  window.__refreshCluster = refreshCluster;
  window.__syncCluster = syncCluster;
  window.__deleteCluster = deleteCluster;
  window.__deleteMachineClass = deleteMachineClass;
  window.__exportMachineClass = exportMachineClass;
  window.__exportCluster = exportCluster;
  window.__closeConfirmModal = closeConfirmModal;
  window.__confirmAction = confirmAction;
  window.__setConfirmInput = function(v) {
    confirmInput = v;
    // Update the OK button disabled state directly — no full re-render needed.
    var btn = document.getElementById('confirm-ok-btn');
    if (btn && confirmModal && confirmModal.requireInput) {
      btn.disabled = (v !== confirmModal.requireInput);
    }
  };
  window.__changeMachineClassPage = changeMachineClassPage;
  window.__changeClusterPage = changeClusterPage;
  window.__toggleMachineClassSort = toggleMachineClassSort;
  window.__setMcSearch = setMcSearch;
  window.__toggleMcStatusFilter = toggleMcStatusFilter;
  window.__toggleClusterSyncFilter = toggleClusterSyncFilter;
  window.__toggleClusterSort = toggleClusterSort;
  window.__setClusterFilter = setClusterFilter;
  window.__clearClusterFilter = clearClusterFilter;
  window.__setClusterSearch = setClusterSearch;
  window.__showClustersView = showClustersView;
  window.__hideClustersView = hideClustersView;
  window.__showLogsModal = showLogsModal;
  window.__closeLogsModal = closeLogsModal;
  window.__downloadTodaysLogs  = downloadTodaysLogs;
  window.__openLogFilesModal   = openLogFilesModal;
  window.__closeLogFilesModal  = closeLogFilesModal;
  window.__downloadLogFile     = downloadLogFile;
  window.__setLogsSearch = function(v) { logsSearch = v; render(); };
  window.__setLogsLevel = function(v) { logsLevelFilter = logsLevelFilter === v ? '' : v; render(); };
  window.__setLogsComponent = function(v) { logsComponentFilter = v; render(); };
  window.__setLogsOrder = function(v) { logsOrder = v; render(); };
  window.__clearLogsFilters = function() { logsSearch = ''; logsLevelFilter = ''; logsComponentFilter = ''; render(); };
  // ── Repo CRUD ──────────────────────────────────────────────────────────────
  var _repoModalIsEdit = false;

  function openRepoModal(nameOrNull) {
    var rc = null;
    if (nameOrNull && state && state.repoConfigs) {
      rc = state.repoConfigs.find(function(x) { return x.name === nameOrNull; }) || null;
    }
    _repoModalIsEdit = !!(rc && rc.name);
    document.getElementById('repo-modal-title').textContent = _repoModalIsEdit ? 'Edit Repository' : 'Add Repository';
    var nameEl = document.getElementById('rm-name');
    nameEl.value    = rc ? (rc.name || '') : '';
    nameEl.disabled = _repoModalIsEdit;
    document.getElementById('rm-url').value      = rc ? (rc.url || '') : '';
    document.getElementById('rm-branch').value   = rc ? (rc.branch || '') : '';
    document.getElementById('rm-clusters').value = rc ? (rc.clustersPath || '') : '';
    document.getElementById('rm-mc').value        = rc ? (rc.mcPath || '') : '';
    var setTokenCb  = document.getElementById('rm-set-token');
    var tokenInput  = document.getElementById('rm-token');
    var tokenHint   = document.getElementById('rm-token-hint');
    var tokenLabel  = document.getElementById('rm-set-token-label');
    if (_repoModalIsEdit) {
      setTokenCb.checked         = false;
      tokenLabel.textContent     = rc.hasToken ? 'Replace existing token' : 'Set a token';
      tokenHint.textContent      = rc.hasToken ? 'Leave unchecked to keep the existing token. Check to replace it, or check and leave blank to clear it.' : '';
      tokenInput.style.display   = 'none';
      tokenInput.value           = '';
    } else {
      setTokenCb.checked         = false;
      tokenLabel.textContent     = 'Set access token';
      tokenHint.textContent      = '';
      tokenInput.style.display   = 'none';
      tokenInput.value           = '';
    }
    setTokenCb.onchange = function() {
      tokenInput.style.display = setTokenCb.checked ? '' : 'none';
      if (!setTokenCb.checked) tokenInput.value = '';
    };
    var errEl = document.getElementById('repo-form-error');
    errEl.style.display = 'none';
    errEl.textContent   = '';
    var btn = document.getElementById('repo-save-btn');
    btn.disabled    = false;
    btn.textContent = 'Save';
    document.getElementById('repo-modal').classList.add('show');
  }

  function closeRepoModal() {
    document.getElementById('repo-modal').classList.remove('show');
  }

  function saveRepo() {
    var name    = document.getElementById('rm-name').value.trim();
    var url     = document.getElementById('rm-url').value.trim();
    var branch  = document.getElementById('rm-branch').value.trim() || 'main';
    var clPath  = document.getElementById('rm-clusters').value.trim() || 'clusters';
    var mcPath  = document.getElementById('rm-mc').value.trim() || 'machineclasses';
    var setTokenCb = document.getElementById('rm-set-token');
    var tokenInput = document.getElementById('rm-token');
    var errEl = document.getElementById('repo-form-error');
    errEl.style.display = 'none';
    if (!name) { errEl.textContent = 'Name is required.'; errEl.style.display = ''; return; }
    if (!url)  { errEl.textContent = 'URL is required.';  errEl.style.display = ''; return; }
    var body = { name: name, url: url, branch: branch, clustersPath: clPath, mcPath: mcPath };
    if (_repoModalIsEdit) {
      body.clearToken = false;
      if (setTokenCb.checked) {
        var tokenVal = tokenInput.value;
        if (!tokenVal) {
          body.clearToken = true;
        } else {
          body.token = tokenVal;
        }
      }
    } else {
      if (setTokenCb.checked && tokenInput.value) {
        body.token = tokenInput.value;
      }
    }
    var method = _repoModalIsEdit ? 'PUT' : 'POST';
    var btn = document.getElementById('repo-save-btn');
    btn.disabled    = true;
    btn.textContent = 'Saving…';
    fetch('/api/repos', {
      method: method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    }).then(function(r) {
      btn.disabled    = false;
      btn.textContent = 'Save';
      if (r.ok) {
        closeRepoModal();
      } else {
        r.text().then(function(t) {
          errEl.textContent   = t || ('Error: ' + r.status);
          errEl.style.display = '';
        });
      }
    }).catch(function(e) {
      btn.disabled    = false;
      btn.textContent = 'Save';
      errEl.textContent   = 'Network error: ' + e.message;
      errEl.style.display = '';
    });
  }

  function deleteRepo(name) {
    confirmInput = '';
    var s = state || {};
    var clusterIDs  = (s.repoClusterMap      && s.repoClusterMap[name])      || [];
    var mcIDs       = (s.repoMachineClassMap && s.repoMachineClassMap[name]) || [];
    // Build cluster lines with auto-sync indicator
    var clusterLines = clusterIDs.map(function(id) { return escHtml(id); });
    var mcLines = mcIDs.map(function(id) { return escHtml(id); });
    var clusterSection = clusterLines.length
      ? '\n\u2022 Clusters\n' + clusterLines.map(function(l) { return '<span style="display:block;margin-left:16px">\u2022 ' + l + '</span>'; }).join('')
      : '';
    var mcSection = mcLines.length
      ? '\n\u2022 Machine Classes\n' + mcLines.map(function(l) { return '<span style="display:block;margin-left:16px">\u2022 ' + l + '</span>'; }).join('')
      : '';
    confirmModal = {
      title: 'Remove Repository',
      message: 'Are you sure you want to remove <b>' + escHtml(name) + '</b>?\n\n' +
               'Deleting this repository will remove associated Clusters &amp; MachineClasses from Omni, where Auto-Sync is enabled. ' +
               'The remaining resources will remain in Omni, but are marked as \u201cUnmanaged\u201d or \u201cOrphaned\u201d in OmniCD.' +
               (clusterSection || mcSection ? '\n' + clusterSection + mcSection : '') +
               '\n\n<i>Note: MachineClasses with Auto-Sync enabled that are associated with any Cluster will not be deleted.</i>',
      requireInput: name,
      inputPrompt: 'Please type \u2018' + name + '\u2019 to confirm deletion of the repository',
      onConfirm: function() {
        confirmModal = null;
        confirmInput = '';
        render();
        fetch('/api/repos', {
          method: 'DELETE',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: name })
        }).then(function(r) {
          if (!r.ok) { r.text().then(function(t) { alert('Delete failed: ' + (t || r.status)); }); }
        }).catch(function(e) { alert('Network error: ' + e.message); });
      }
    };
    render();
  }

  window.__openOmniInstanceModal  = openOmniInstanceModal;
  window.__closeOmniInstanceModal = closeOmniInstanceModal;
  window.__saveOmniInstance       = saveOmniInstance;
  window.__testOmniInstance       = testOmniInstance;
  window.__deleteOmniInstance     = deleteOmniInstance;
  window.__refreshOmniConnection  = refreshOmniConnection;

  window.__openRepoModal  = openRepoModal;
  window.__closeRepoModal = closeRepoModal;
  window.__saveRepo       = saveRepo;
  window.__deleteRepo     = deleteRepo;

  // ── End Repo CRUD ───────────────────────────────────────────────────────────
  window.__showMachineClassModal = showMachineClassModal;
  window.__setMcPageSize = setMcPageSize;
  window.__setClusterPageSize = setClusterPageSize;
  window.__navToCluster = navToCluster;
  window.__setClusterDetailTab = setClusterDetailTab;
  window.__refreshCurrentCluster = refreshCurrentCluster;

  // WebSocket connection
  function connectWebSocket() {
    var protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    var wsUrl = protocol + '//' + window.location.host + '/ws';

    try {
      ws = new WebSocket(wsUrl);

      ws.onopen = function() {
        console.log('WebSocket connected');
        wsReconnectDelay = 1000; // Reset reconnect delay on successful connection
      };

      ws.onmessage = function(event) {
        try {
          state = JSON.parse(event.data);
          markAppLoaded();
          var isDetailPage = clusterDetailId && currentRoute.startsWith('/clusters/');
          if (logsModal) {
            updateLogsInPlace();
          } else if (isDetailPage) {
            updateClusterDetailInPlace();
          } else if (currentModal || confirmModal) {
            renderMainOnly();
          } else {
            render();
          }
        } catch(e) {
          console.error('Failed to parse WebSocket message:', e);
        }
      };

      ws.onclose = function() {
        console.log('WebSocket disconnected, reconnecting...');
        ws = null;
        // Exponential backoff with max 10 seconds
        wsReconnectDelay = Math.min(wsReconnectDelay * 1.5, 10000);
        wsReconnectTimer = setTimeout(connectWebSocket, wsReconnectDelay);
      };

      ws.onerror = function(error) {
        console.error('WebSocket error:', error);
      };
    } catch(e) {
      console.error('Failed to create WebSocket:', e);
      wsReconnectTimer = setTimeout(connectWebSocket, wsReconnectDelay);
    }
  }

  // Close modal on ESC key
  document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
      if (confirmModal) {
        closeConfirmModal();
      } else if (logsModal) {
        closeLogsModal();
      } else if (currentModal) {
        closeModal();
      }
    }
  });

  // Render sidebar immediately (doesn't depend on state)
  document.getElementById('sidebar').innerHTML = renderSidebar();

  // Apply flex layout immediately on cluster detail pages (avoids layout flash)
  if (clusterDetailId) applyDetailPageLayout();

  // Sidebar collapse state
  var sidebarEl = document.getElementById('sidebar');
  function applySidebarCollapse(collapsed) {
    sidebarEl.classList.toggle('collapsed', collapsed);
    document.documentElement.style.setProperty('--sidebar-w', collapsed ? '56px' : '200px');
    var btn = document.getElementById('sidebar-toggle');
    if (btn) btn.textContent = collapsed ? '›' : '‹';
  }
  window.__toggleSidebar = function() {
    var collapsed = !sidebarEl.classList.contains('collapsed');
    localStorage.setItem('sidebarCollapsed', collapsed ? '1' : '0');
    applySidebarCollapse(collapsed);
  };

  applySidebarCollapse(localStorage.getItem('sidebarCollapsed') === '1');

  // Start WebSocket connection
  connectWebSocket();

  // Fallback polling (only if WebSocket is disconnected)
  setInterval(function() {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      fetchState();
    }
  }, 5000);
})();
</script>
</body>
</html>`
