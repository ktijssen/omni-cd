package web

import (
	"fmt"
	"net/http"
	"strings"
)

// handleUI serves the embedded UI.
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, strings.ReplaceAll(uiHTML, "{{APP_VERSION}}", s.version))
}

const uiHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Omni CD</title>
<link rel="icon" type="image/svg+xml" href="https://mintlify.s3.us-west-1.amazonaws.com/siderolabs-fe86397c/images/omni.svg">
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
  .badge-unmanaged { background: #27272a; color: #71717a; border: 1px solid #3f3f46; }
  .badge-deleting { background: #4d1500; color: #fb7a37; }
  .badge-syncing { background: #0d2d2a; color: #2dd4bf; }
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
  .btn-sort { background: none; border: 1px solid #3f3f46; color: #71717a; padding: 2px 8px; border-radius: 4px; font-size: 11px; cursor: pointer; font-family: 'SF Mono', 'Fira Code', monospace; }
  .btn-sort:hover { border-color: #a1a1aa; color: #a1a1aa; }
  .btn-sort.active { border-color: #FB326E; color: #FB326E; background: rgba(251, 50, 110, 0.1); }
  .btn-back { display: inline-block; background: none; border: 1px solid #3f3f46; color: #a1a1aa; padding: 4px 12px; border-radius: 6px; font-size: 12px; cursor: pointer; text-decoration: none; }
  .btn-back:hover { border-color: #a1a1aa; color: #fff; }
  .panel-nav-link { cursor: pointer; transition: color 0.15s; }
  .panel-nav-link:hover { color: #FB326E; }
  .cluster-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 16px; padding: 24px; }
  .cluster-card { background: #27272a; border: 1px solid #3f3f46; border-radius: 12px; overflow: hidden; display: flex; }
  .cluster-card.clickable { cursor: pointer; }
  .cluster-card.clickable:hover { border-color: #71717a; }
  .cluster-card-accent { width: 4px; flex-shrink: 0; background: #3f3f46; }
  .cluster-card[data-status="success"] .cluster-card-accent,
  .cluster-card[data-status="applied"] .cluster-card-accent { background: #4ade80; }
  .cluster-card[data-status="failed"] .cluster-card-accent { background: #f87171; }
  .cluster-card[data-status="outofsync"] .cluster-card-accent { background: #fb923c; }
  .cluster-card[data-status="syncing"] .cluster-card-accent { background: #2dd4bf; }
  .cluster-card[data-status="unmanaged"] .cluster-card-accent { background: #52525b; }
  /* Override accent to red when synced but cluster not ready */
  .cluster-card[data-status="success"][data-health="not-ready"] .cluster-card-accent,
  .cluster-card[data-status="applied"][data-health="not-ready"] .cluster-card-accent { background: #f87171; }
  .cluster-health-bar-wrap { padding: 0 24px 16px; display: flex; align-items: center; gap: 12px; }
  .cluster-health-bar { flex: 1; height: 8px; border-radius: 4px; background: #3f3f46; overflow: hidden; display: flex; }
  .cluster-health-bar-seg { height: 100%; cursor: pointer; transition: width 0.3s, opacity 0.15s; opacity: 0.85; }
  .cluster-health-bar-seg:hover { opacity: 1; }
  .cluster-health-bar.has-filter .cluster-health-bar-seg { opacity: 0.3; }
  .cluster-health-bar.has-filter .cluster-health-bar-seg.active { opacity: 1; }
  .cluster-health-bar-seg--ready { background: #4ade80; }
  .cluster-health-bar-seg--notready { background: #f87171; }
  .cluster-health-bar-seg--failed { background: #ef4444; }
  .cluster-health-bar-seg--outofsync { background: #fb923c; }
  .cluster-health-bar-seg--unmanaged { background: #52525b; }
  .cluster-health-summary { font-size: 12px; color: #71717a; white-space: nowrap; }
  .cluster-card-body { flex: 1; padding: 12px 14px; min-width: 0; }
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

  .btn-logs {
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
  .btn-logs:hover { background: #e0285f; }
  .btn-logs:active { background: #c92255; }
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
  .btn-download {
    background: #3f3f46;
    color: #e4e4e7;
    border: none;
    padding: 6px 14px;
    border-radius: 6px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
  }
  .btn-download:hover { background: #52525b; }
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
  .log-entry { padding: 2px 20px; }
  .log-entry:hover { background: #323235; }
  .log-ts { color: #52525b; }
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
  }
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
  .mc-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 12px; }
  .mc-card { background: #27272a; border: 1px solid #3f3f46; border-radius: 12px; overflow: hidden; display: flex; }
  .mc-card.clickable { cursor: pointer; }
  .mc-card.clickable:hover { border-color: #71717a; }
  .mc-card-accent { width: 4px; flex-shrink: 0; background: #3f3f46; }
  .mc-card[data-status="success"] .mc-card-accent,
  .mc-card[data-status="applied"] .mc-card-accent { background: #4ade80; }
  .mc-card[data-status="failed"] .mc-card-accent { background: #f87171; }
  .mc-card[data-status="outofsync"] .mc-card-accent { background: #fb923c; }
  .mc-card[data-status="syncing"] .mc-card-accent { background: #2dd4bf; }
  .mc-card-header { flex: 1; padding: 12px 14px; min-width: 0; }
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
    max-width: 500px;
  }
  .confirm-body {
    padding: 32px 24px;
    text-align: center;
    white-space: normal;
  }
  .confirm-icon {
    font-size: 48px;
    margin-bottom: 16px;
  }
  .confirm-message {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 14px;
    line-height: 1.6;
    color: #e4e4e7;
    margin-bottom: 24px;
    white-space: pre-line;
  }
  .confirm-actions {
    display: flex;
    gap: 12px;
    justify-content: center;
  }
  .btn-cancel, .btn-confirm {
    padding: 8px 24px;
    border-radius: 6px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    border: none;
    transition: all 0.2s;
  }
  .btn-cancel {
    background: #3f3f46;
    color: #e4e4e7;
  }
  .btn-cancel:hover {
    background: #52525b;
  }
  .btn-confirm {
    background: #FB326E;
    color: #fff;
  }
  .btn-confirm:hover {
    background: #e91e63;
  }

  /* Cluster topology graph */
  .cluster-graph { font-family: -apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif; white-space: normal; word-break: normal; display: flex; flex-direction: column; overflow: hidden; flex: 1; min-height: 0; }
  .cluster-graph-toolbar { display: flex; align-items: center; justify-content: flex-end; gap: 4px; padding: 10px 16px 0; flex-shrink: 0; }
  .cluster-graph-zoom-btn { background: #3f3f46; border: 1px solid #52525b; color: #a1a1aa; border-radius: 5px; width: 26px; height: 26px; font-size: 16px; line-height: 1; cursor: pointer; display: flex; align-items: center; justify-content: center; }
  .cluster-graph-zoom-btn:hover { background: #52525b; color: #e4e4e7; }
  .graph-zoom-level { font-size: 11px; color: #71717a; min-width: 34px; text-align: center; }
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
  .dash-fleet-card { background: #27272a; border: 1px solid #3f3f46; border-radius: 12px; padding: 18px 20px; margin-bottom: 12px; }
  .dash-fleet-title { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.08em; color: #71717a; margin-bottom: 12px; }
  .dash-fleet-bar { height: 10px; border-radius: 5px; background: #3f3f46; overflow: hidden; display: flex; margin-bottom: 12px; }
  .dash-fleet-seg { height: 100%; transition: width 0.4s; }
  .dash-fleet-seg--ready     { background: #4ade80; }
  .dash-fleet-seg--notready  { background: #f87171; }
  .dash-fleet-seg--outofsync { background: #fb923c; }
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
  .main-content { flex: 1; min-width: 0; padding-bottom: 36px; }
  .placeholder-page { padding: 64px 24px; text-align: center; color: #52525b; }
  .placeholder-page .placeholder-icon { font-size: 40px; margin-bottom: 16px; }
  .placeholder-page .placeholder-title { font-size: 16px; font-weight: 600; color: #71717a; margin-bottom: 8px; }
  .placeholder-page .placeholder-sub { font-size: 13px; }
</style>
</head>
<body>
<div class="layout">
  <nav class="sidebar" id="sidebar"></nav>
  <div class="main-content">
    <div class="container" id="app"></div>
  </div>
</div>
<div class="refresh-indicator" id="footer"></div>
<div id="modals"></div>
<script>
(function() {
  var app      = document.getElementById('app');
  var modalsEl = document.getElementById('modals');
  var footerEl = document.getElementById('footer');
  var appVersion = '{{APP_VERSION}}';
  if (footerEl) footerEl.textContent = 'Omni CD ' + appVersion + ' · Real-time updates';
  var state = null;
  var autoScroll = true;
  var machineClassPage = 1;
  var machineClassSortAZ = true;
  var clusterPage = 1;
  var clusterSortAZ = true;
  var clusterStatusFilter = null;
  var pageSize = 5;
  var mcPageSize = 10;
  var clusterPageSize = 10;
  var logsModal = false;
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
      if (tab === 'error') {
        body.innerHTML = '<div style="color:#f87171;white-space:pre-wrap;">' + escHtml(currentModal.error) + '</div>';
      } else if (tab === 'live') {
        body.innerHTML = currentModal.liveContent
          ? '<pre style="margin:0;white-space:pre-wrap;">' + escHtml(currentModal.liveContent) + '</pre>'
          : '<div style="color:#71717a;text-align:center;padding:40px;">No live state available</div>';
      } else if (tab === 'diff') {
        body.innerHTML = currentModal.diff
          ? '<pre style="margin:0;white-space:pre-wrap;">' + formatDiff(currentModal.diff) + '</pre>'
          : '<div style="color:#71717a;text-align:center;padding:40px;">No diff available</div>';
      } else if (tab === 'graph') {
        body.innerHTML = renderClusterGraph(currentModal);
        requestAnimationFrame(function() { window.__graphCentre(body); });
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

  function escHtml(s) {
    return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
  }

  window.__showClusterModal = showClusterModal;
  window.__setModalTab = setModalTab;
  window.__closeModal = closeModal;

  // Pan + zoom via CSS transform: translate(tx,ty) scale(z). No scroll involved.
  function applyGraphTransform(inner, tx, ty, z) {
    inner.dataset.tx   = tx;
    inner.dataset.ty   = ty;
    inner.dataset.zoom = z;
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

  function badgeClass(st) {
    if (!st) return 'badge-idle';
    if (st === 'success' || st === 'applied' || st === 'synced') return 'badge-success';
    if (st === 'running') return 'badge-running';
    if (st === 'failed') return 'badge-failed';
    if (st === 'outofsync' || st === 'out of sync') return 'badge-outofsync';
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

  function getGitHealth(s) {
    if (!s || !s.git) return { status: 'unknown', label: 'Unknown' };

    // Check if we have valid git data
    if (!s.git.sha || !s.git.lastSync) {
      return { status: 'disconnected', label: 'Disconnected' };
    }

    // Check if last sync was recent (within 10 minutes)
    var lastSync = new Date(s.git.lastSync);
    var now = Date.now();
    var minutesSinceSync = Math.floor((now - lastSync.getTime()) / 1000 / 60);

    // If sync is very old, something might be wrong
    if (minutesSinceSync > 10) {
      return { status: 'stale', label: 'Stale' };
    }

    // Check if last reconcile failed
    if (s.lastReconcile && s.lastReconcile.status === 'failed') {
      return { status: 'degraded', label: 'Degraded' };
    }

    return { status: 'healthy', label: 'Healthy' };
  }

  function gitHealthBadgeClass(status) {
    if (status === 'healthy') return 'badge-success';
    if (status === 'degraded') return 'badge-outofsync';
    if (status === 'stale') return 'badge-outofsync';
    if (status === 'disconnected') return 'badge-failed';
    return 'badge-idle';
  }

  function logClass(level) {
    if (level === 'WARN') return 'log-warn';
    if (level === 'ERROR') return 'log-error';
    return 'log-info';
  }

  async function fetchState() {
    try {
      var r = await fetch('/api/state');
      state = await r.json();
      // Don't re-render if modal is open to prevent flashing
      if (!currentModal && !confirmModal) {
        render();
      }
    } catch(e) {}
  }

  async function checkGit() {
    try {
      var r = await fetch('/api/check', { method: 'POST' });
      var d = await r.json();
      if (d.status === 'already running') alert('Reconcile already in progress');
      fetchState();
    } catch(e) {
      alert('Failed to trigger git check');
    }
  }

  async function triggerReconcile() {
    try {
      var r = await fetch('/api/reconcile', { method: 'POST' });
      var d = await r.json();
      if (d.status === 'already running') alert('Reconcile already in progress');
      fetchState();
    } catch(e) {
      alert('Failed to trigger reconcile');
    }
  }

  async function toggleClusters() {
    try {
      var r = await fetch('/api/clusters-toggle', { method: 'POST' });
      var d = await r.json();
      fetchState();
    } catch(e) {
      alert('Failed to toggle clusters');
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

  function closeConfirmModal() {
    confirmModal = null;
    render();
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

  function showClustersView() {
    window.location.href = '/clusters';
  }

  function hideClustersView() {
    window.location.href = '/';
  }

  function renderSidebar() {
    var navItems = [
      { label: 'Dashboard',      icon: '⊟', href: '/' },
      { label: 'Clusters',       icon: '◈', href: '/clusters' },
      { label: 'Machine Classes', icon: '▦', href: '/machineclasses' },
      null,
      { label: 'Repos',          icon: '⎇', href: '/repos' },
      { label: 'Users',          icon: '◉', href: '/users' },
    ];
    var html = '<div class="sidebar-logo">' +
      '<img class="logo sidebar-logo-img" src="https://mintlify.s3.us-west-1.amazonaws.com/siderolabs-fe86397c/images/omni.svg" alt="Omni">' +
      '<div class="sidebar-logo-text">Omni <span>CD</span></div>' +
      '<button class="sidebar-toggle" id="sidebar-toggle" onclick="window.__toggleSidebar()" title="Toggle sidebar">‹</button>' +
    '</div>' +
    '<div class="sidebar-nav">';
    navItems.forEach(function(item) {
      if (!item) { html += '<div class="sidebar-sep"></div>'; return; }
      var isActive = currentRoute === item.href || (item.href !== '/' && currentRoute.startsWith(item.href));
      html += '<a class="sidebar-item' + (isActive ? ' active' : '') + '" href="' + item.href + '" title="' + item.label + '">' +
        '<span class="sidebar-item-icon">' + item.icon + '</span>' +
        '<span class="sidebar-item-label">' + item.label + '</span>' +
      '</a>';
    });
    html += '</div>' +
      '<div class="sidebar-footer" style="padding:8px">' +
        '<a class="sidebar-item" href="/logout" style="color:#71717a;" title="Sign out">' +
          '<span class="sidebar-item-icon">⏻</span>' +
          '<span class="sidebar-item-label">Sign out</span>' +
        '</a>' +
      '</div>';
    return html;
  }

  function renderReposView(s) {
    return renderHeader(s) +
      '<div class="placeholder-page">' +
        '<div class="placeholder-icon">⎇</div>' +
        '<div class="placeholder-title">Git Repositories</div>' +
        '<div class="placeholder-sub">Multi-repo support coming soon</div>' +
      '</div>';
  }

  function renderUsersView(s) {
    return renderHeader(s) +
      '<div class="placeholder-page">' +
        '<div class="placeholder-icon">◉</div>' +
        '<div class="placeholder-title">User Management</div>' +
        '<div class="placeholder-sub">Authentication &amp; RBAC coming soon</div>' +
      '</div>';
  }

  function renderHeader(s) {
    var isRunning = s.lastReconcile && s.lastReconcile.status === 'running';
    var mismatch = s.versionMismatch;
    var syncDisabled = isRunning || mismatch;
    var titles = { '/': 'Dashboard', '/clusters': 'Clusters', '/machineclasses': 'Machine Classes', '/repos': 'Repositories', '/users': 'Users' };
    var pageTitle = titles[currentRoute] || 'Dashboard';
    return '<div class="header">' +
      '<h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">' + pageTitle + '</h1>' +
      '<div class="header-buttons">' +
        (mismatch ?
          '<div class="version-warning">' +
            '<span class="warn-icon">&#9888;</span>' +
            'Omni ' + s.omniVersion + ' &gt; omnictl ' + s.omnictlVersion +
          '</div>' : '') +
        (isRunning ? '<span class="spinner"></span>' : '') +
        '<button class="btn-check" onclick="window.__checkGit()" ' +
          (isRunning ? 'disabled' : '') + '>Refresh</button>' +
        '<button class="btn-reconcile" onclick="window.__triggerReconcile()" ' +
          (syncDisabled ? 'disabled' : '') + '>' +
          (isRunning ? 'Syncing...' : 'Sync') +
        '</button>' +
        '<button class="btn-logs" onclick="window.__showLogsModal()">Logs</button>' +
      '</div>' +
    '</div>';
  }

  function renderMachineClassesView(s) {
    var mcs = (s.machineClasses || []).slice().sort(function(a, b) {
      return machineClassSortAZ ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id);
    });
    var pageMcs = mcPageSize === 0 ? mcs : paginateWithSize(mcs, machineClassPage, mcPageSize);
    var cardsHtml = pageMcs.length > 0
      ? pageMcs.map(function(m) {
          var spec = parseMachineClassSpec(m.fileContent, m.id);
          var displayStatus = m.status === 'success' ? 'synced' : m.status;
          var hasDiff = m.diff && m.diff.length > 0;
          var hasFile = m.fileContent && m.fileContent.length > 0;
          var hasDetails = hasDiff || hasFile;
          var provisionLabel = m.provisionType === 'auto' ? 'Auto-Provision' : m.provisionType === 'manual' ? 'Manual' : 'Unknown';
          var provisionColor = m.provisionType === 'auto' ? '#60a5fa' : '#a1a1aa';
          var statusDot = '';
          if (m.status === 'success' || m.status === 'applied') statusDot = '#4ade80';
          else if (m.status === 'failed') statusDot = '#f87171';
          else if (m.status === 'outofsync') statusDot = '#fb923c';
          else if (m.status === 'syncing') statusDot = '#2dd4bf';
          else statusDot = '#52525b';

          var isAuto = m.provisionType === 'auto';
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
          if (m.status === 'success' || m.status === 'applied') { mcStatusText = '● synced';       mcStatusColor = '#4ade80'; }
          else if (m.status === 'failed')                        { mcStatusText = '● failed';       mcStatusColor = '#f87171'; }
          else if (m.status === 'outofsync')                     { mcStatusText = '● out of sync';  mcStatusColor = '#fb923c'; }
          else if (m.status === 'syncing')                       { mcStatusText = '● syncing';      mcStatusColor = '#2dd4bf'; }
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
            '</div>' +
          '</div>';
        }).join('')
      : '<div style="padding:40px;text-align:center;color:#52525b">No machine classes</div>';

    return renderHeader(s) +
      '<div style="display:flex;align-items:center;gap:12px;padding:0 0 16px">' +
        '<div style="margin-left:auto;display:flex;align-items:center;gap:8px">' +
          '<button class="btn-sort active" onclick="window.__toggleMachineClassSort()">' + (machineClassSortAZ ? 'A→Z' : 'Z→A') + '</button>' +
          '<div class="page-size-bar">' + renderPageSizeBar([10, 25, 50, 0], mcPageSize, 'window.__setMcPageSize') + '</div>' +
        '</div>' +
      '</div>' +
      '<div class="mc-grid">' + cardsHtml + '</div>' +
      (mcPageSize > 0 && mcs.length > mcPageSize ? renderPaginationSized(mcs, machineClassPage, 'window.__changeMachineClassPage', mcPageSize) : '');
  }

  function renderClustersView(s) {
    var clusters = (s.clusters || []).slice().sort(function(a, b) {
      return clusterSortAZ ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id);
    });
    var total = clusters.length;
    var countReady = 0, countNotReady = 0, countFailed = 0, countOutofsync = 0, countUnmanaged = 0;
    clusters.forEach(function(c) {
      var st = c.status || '';
      if (st === 'unmanaged') { countUnmanaged++; return; }
      if (st === 'failed') { countFailed++; return; }
      if (st === 'outofsync') { countOutofsync++; return; }
      if (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') { countNotReady++; return; }
      if (st === 'success' || st === 'applied' || st === 'synced' || st === 'syncing') countReady++;
    });
    var healthBar = '';
    if (total > 0) {
      var pReady      = (countReady      / total * 100).toFixed(1);
      var pNotReady   = (countNotReady   / total * 100).toFixed(1);
      var pFailed     = (countFailed     / total * 100).toFixed(1);
      var pOutofsync  = (countOutofsync  / total * 100).toFixed(1);
      var pUnmanaged  = (countUnmanaged  / total * 100).toFixed(1);
      var hasFilter   = clusterStatusFilter !== null;
      var summaryParts = [];
      if (countReady)    summaryParts.push('<span style="color:#4ade80;cursor:pointer' + (hasFilter && clusterStatusFilter === 'ready'     ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'ready\')">'      + countReady    + ' ready</span>');
      if (countNotReady) summaryParts.push('<span style="color:#f87171;cursor:pointer' + (hasFilter && clusterStatusFilter === 'not-ready' ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'not-ready\')">'  + countNotReady + ' not ready</span>');
      if (countFailed)   summaryParts.push('<span style="color:#ef4444;cursor:pointer' + (hasFilter && clusterStatusFilter === 'failed'    ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'failed\')">'     + countFailed   + ' failed</span>');
      if (countOutofsync) summaryParts.push('<span style="color:#fb923c;cursor:pointer' + (hasFilter && clusterStatusFilter === 'outofsync' ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'outofsync\')">' + countOutofsync + ' out of sync</span>');
      if (countUnmanaged) summaryParts.push('<span style="color:#52525b;cursor:pointer' + (hasFilter && clusterStatusFilter === 'unmanaged' ? ';font-weight:700' : '') + '" onclick="window.__setClusterFilter(\'unmanaged\')">' + countUnmanaged + ' unmanaged</span>');
      var clearBtn = hasFilter ? ' &nbsp;<span style="cursor:pointer;color:#a1a1aa;text-decoration:underline;font-size:11px" onclick="window.__clearClusterFilter()">clear</span>' : '';
      healthBar = '<div class="cluster-health-bar-wrap">' +
        '<div class="cluster-health-bar' + (hasFilter ? ' has-filter' : '') + '">' +
          (countReady     ? '<div class="cluster-health-bar-seg cluster-health-bar-seg--ready'     + (hasFilter && clusterStatusFilter === 'ready'     ? ' active' : '') + '" style="width:' + pReady     + '%" onclick="window.__setClusterFilter(\'ready\')"     title="' + countReady     + ' ready"></div>'         : '') +
          (countNotReady  ? '<div class="cluster-health-bar-seg cluster-health-bar-seg--notready'  + (hasFilter && clusterStatusFilter === 'not-ready'  ? ' active' : '') + '" style="width:' + pNotReady  + '%" onclick="window.__setClusterFilter(\'not-ready\')"  title="' + countNotReady  + ' not ready"></div>'    : '') +
          (countFailed    ? '<div class="cluster-health-bar-seg cluster-health-bar-seg--failed'    + (hasFilter && clusterStatusFilter === 'failed'    ? ' active' : '') + '" style="width:' + pFailed    + '%" onclick="window.__setClusterFilter(\'failed\')"    title="' + countFailed    + ' failed"></div>'        : '') +
          (countOutofsync ? '<div class="cluster-health-bar-seg cluster-health-bar-seg--outofsync' + (hasFilter && clusterStatusFilter === 'outofsync' ? ' active' : '') + '" style="width:' + pOutofsync + '%" onclick="window.__setClusterFilter(\'outofsync\')" title="' + countOutofsync + ' out of sync"></div>'   : '') +
          (countUnmanaged ? '<div class="cluster-health-bar-seg cluster-health-bar-seg--unmanaged' + (hasFilter && clusterStatusFilter === 'unmanaged' ? ' active' : '') + '" style="width:' + pUnmanaged + '%" onclick="window.__setClusterFilter(\'unmanaged\')" title="' + countUnmanaged + ' unmanaged"></div>'    : '') +
        '</div>' +
        '<div class="cluster-health-summary">' + total + ' clusters &nbsp;·&nbsp; ' + summaryParts.join(' &nbsp;·&nbsp; ') + clearBtn + '</div>' +
      '</div>';
    }
    var displayClusters = clusterStatusFilter
      ? clusters.filter(function(c) {
          var st = c.status || '';
          if (clusterStatusFilter === 'ready')     return (st === 'success' || st === 'applied' || st === 'synced') && c.clusterReady !== 'not-ready' && c.kubernetesApiReady !== 'not-ready';
          if (clusterStatusFilter === 'not-ready') return st !== 'unmanaged' && (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready');
          if (clusterStatusFilter === 'failed')    return st === 'failed';
          if (clusterStatusFilter === 'outofsync') return st === 'outofsync';
          if (clusterStatusFilter === 'unmanaged') return st === 'unmanaged';
          return true;
        })
      : clusters;
    var pageDisplayClusters = clusterPageSize === 0 ? displayClusters : paginateWithSize(displayClusters, clusterPage, clusterPageSize);
    var cards = pageDisplayClusters.map(function(c) {
      var cp = c.controlPlane || {};
      var cpCount = cp.count || 0;
      var cpMC = cp.machineClass || '';
      var hasDiff = c.diff && c.diff.length > 0;
      var hasFile = c.fileContent && c.fileContent.length > 0;
      var isFailed = c.status === 'failed';
      var hasError = c.error && c.error.length > 0;
      var hasDetails = c.status !== 'unmanaged'; // Graph tab available for all managed clusters
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
      if (c.status === 'unmanaged')                              { statusText = 'unmanaged';     statusColor = '#52525b'; }
      else if (c.status === 'outofsync')                         { statusText = '● out of sync'; statusColor = '#fb923c'; }
      else if (c.status === 'failed')                            { statusText = '● failed';       statusColor = '#f87171'; }
      else if (c.status === 'syncing')                           { statusText = '● syncing';      statusColor = '#2dd4bf'; }
      else if (c.status === 'success' || c.status === 'applied') { statusText = '● synced';       statusColor = '#4ade80'; }
      else if (c.status === 'deleting')                          { statusText = '● deleting';     statusColor = '#71717a'; }
      var healthText = '', healthColor = '';
      if (c.status !== 'unmanaged') {
        if (c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready') {
          healthText = '✓ ready'; healthColor = '#4ade80';
        } else if (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') {
          healthText = '✗ not ready'; healthColor = '#f87171';
        }
      }
      var cardHealth = (c.status !== 'unmanaged' && (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready')) ? ' data-health="not-ready"' : '';
      return '<div class="cluster-card' + (hasDetails ? ' clickable' : '') + '" data-status="' + (c.status || 'idle') + '"' + cardHealth + (hasDetails ? ' onclick="window.__showClusterModal(\'' + c.id + '\')"' : '') + '>' +
        '<div class="cluster-card-accent"></div>' +
        '<div class="cluster-card-body">' +
          '<div class="cluster-card-header">' +
            '<span class="cluster-card-title">' +
              c.id +
            '</span>' +
            '<div style="display:flex;align-items:center;gap:6px;flex-shrink:0;">' +
              (c.status === 'unmanaged' ? '<button class="btn-export" onclick="window.__exportCluster(\'' + c.id + '\', event);event.stopPropagation()">export</button>' : '') +
              (c.status === 'outofsync' || c.status === 'failed' ? '<button class="btn-force-sync" onclick="window.__forceSync(\'' + c.id + '\', event)">↻ force sync</button>' : '') +
              (statusText ? '<span class="cluster-card-status" style="color:' + statusColor + ';">' + statusText + '</span>' : '') +
            '</div>' +
          '</div>' +
          '<div class="cluster-card-versions">' +
            'Talos ' + escHtml(c.talosVersion || '—') + ' · K8s ' + escHtml(c.kubernetesVersion || '—') +
            (healthText ? '&ensp;<span style="color:' + healthColor + ';">' + healthText + '</span>' : '') +
          '</div>' +
          '<div class="cluster-card-divider"></div>' +
          sectionsHtml +
        '</div>' +
      '</div>';
    }).join('');

    var clusterToolbar = '<div style="display:flex;align-items:center;gap:12px;padding:0 0 12px">' +
      '<div style="margin-left:auto;display:flex;align-items:center;gap:8px">' +
        '<button class="btn-sort active" onclick="window.__toggleClusterSort()">' + (clusterSortAZ ? 'A→Z' : 'Z→A') + '</button>' +
        '<div class="page-size-bar">' + renderPageSizeBar([10, 25, 50, 0], clusterPageSize, 'window.__setClusterPageSize') + '</div>' +
      '</div>' +
    '</div>';
    return renderHeader(s) +
      (clusters.length > 0
        ? healthBar + clusterToolbar + '<div class="cluster-grid">' + cards + '</div>' +
          (clusterPageSize > 0 && displayClusters.length > clusterPageSize ? renderPaginationSized(displayClusters, clusterPage, 'window.__changeClusterPage', clusterPageSize) : '')
        : '<div style="padding:24px;color:#52525b">No clusters</div>');
  }

  function renderClusterGraph(modal) {
    var cluster = state && state.clusters && state.clusters.find(function(c) { return c.id === modal.id; });
    if (!cluster) return '<div style="color:#71717a;text-align:center;padding:40px;">No cluster data available</div>';

    var git = (state && state.git) || {};
    var gitRepo = git.repo || '';
    var gitBranch = git.branch || '';
    var gitSha = git.shortSha || (git.sha || '').slice(0, 7);
    var gitLastSync = ago(git.lastSync);

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

    // Layout constants
    var NH = 100, NG = 12, NW = 220, NW_MACH = 330, EW = 60;
    function colH(n) { return n * NH + Math.max(0, n - 1) * NG; }

    // DAG node builder — optional width and nameStyle params
    function dagNode(kind, name, metaHtml, badgesHtml, accentColor, iconSvg, width, nameStyle) {
      var w = width || NW;
      return '<div class="dag-node" style="border-color:' + accentColor + ';width:' + w + 'px;">' +
        '<div class="dag-node-accent" style="background:' + accentColor + ';"></div>' +
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

    // Col 1: Git
    var gitMeta = (gitBranch ? escHtml(gitBranch) : '') +
      (gitSha ? (gitBranch ? ' &middot; ' : '') + gitSha : '') +
      (gitLastSync ? ' &middot; ' + escHtml(gitLastSync) : '');
    var col1Nodes = [dagNode('Git', gitRepo || 'Repository', gitMeta, '', '#3b82f6', gitIcon)];

    // Col 2: Cluster — no status/health badges, just version meta
    var clusterMeta = 'Talos ' + escHtml(cluster.talosVersion || '\u2014') + ' &middot; K8s ' + escHtml(cluster.kubernetesVersion || '\u2014');
    var col2Nodes = [dagNode('Cluster', cluster.id, clusterMeta, '', borderColor, talosIcon)];

    // Col 3: Node groups (ControlPlane + Workers) + cluster-level extensions (same visual level)
    var clusterExtsList = cluster.clusterExtensions || [];
    var col3Nodes = nodeGroups.map(function(g) {
      var mcLabel = g.machineClass ? escHtml(g.machineClass) : 'Manual';
      var meta = g.isPool
        ? (g.count + ' node' + (g.count !== 1 ? 's' : '') + ' - ' + mcLabel)
        : (g.machines.length + ' machine' + (g.machines.length !== 1 ? 's' : '') + ' - ' + mcLabel);
      return dagNode(g.kind, g.label, meta, '', g.color, cpIcon);
    }).concat(clusterExtsList.map(function(ext) {
      return dagNode('Extension', ext, '', '', '#6366f1', extIcon);
    }));

    // Col 4 (optional): Individual machines — full UUID, monospace name, wider card
    var col4Nodes = [], machineConnections = [], col4Uuids = [];
    if (hasIndividualMachines) {
      nodeGroups.forEach(function(g, gi) {
        g.machines.forEach(function(mid) {
          machineConnections.push([gi, col4Nodes.length]);
          col4Uuids.push(mid);
          col4Nodes.push(dagNode('Machine', mid, '', '', g.color, machIcon(g.color), NW_MACH,
            'font-family:\'SF Mono\',\'Fira Code\',monospace;font-size:11px;'));
        });
      });
    }

    // Extensions: collect unique names from nodeGroup and machine sources.
    // Cluster-level extensions are placed directly in col3 (alongside nodeGroups).
    // Each connection stores [srcColCount, srcIdx, tgtIdx] so the SVG can compute
    // the correct Y position regardless of which column is the source.
    var extNodes = [], extNameToIdx = {}, extConns = [];
    function addExt(name) {
      if (!(name in extNameToIdx)) {
        extNameToIdx[name] = extNodes.length;
        extNodes.push(dagNode('Extension', name, '', '', '#6366f1', extIcon));
      }
      return extNameToIdx[name];
    }
    // nodeGroup-level (kind: ControlPlane/Workers → systemExtensions) → connect from that nodeGroup in col3
    nodeGroups.forEach(function(g, gi) {
      g.exts.forEach(function(ext) {
        extConns.push([col3Nodes.length, gi, addExt(ext)]);
      });
    });
    // Source 3: machine-level (kind: Machine → systemExtensions) → connect from that machine card
    if (hasIndividualMachines) {
      var machExtMap = cluster.machineExtensions || {};
      col4Uuids.forEach(function(uuid, mi) {
        (machExtMap[uuid] || []).forEach(function(ext) {
          extConns.push([col4Nodes.length, mi, addExt(ext)]);
        });
      });
    }
    var hasExtensions = extNodes.length > 0;

    // Max height across all active columns
    var colCounts = [1, 1, col3Nodes.length];
    if (hasIndividualMachines) colCounts.push(col4Nodes.length);
    if (hasExtensions) colCounts.push(extNodes.length);
    var maxH = Math.max.apply(null, colCounts.map(colH).concat([NH]));

    // Standard SVG edge connector: all connections share the same left column count
    function edgeSvg(leftCount, rightCount, connections) {
      var lH = colH(leftCount), rH = colH(rightCount);
      var mt_l = (maxH - lH) / 2, mt_r = (maxH - rH) / 2;
      var paths = '';
      connections.forEach(function(conn) {
        var ly = mt_l + conn[0] * (NH + NG) + NH / 2;
        var ry = mt_r + conn[1] * (NH + NG) + NH / 2;
        var bx = EW * 0.5;
        paths += '<path d="M0,' + ly.toFixed(1) + ' C' + bx.toFixed(1) + ',' + ly.toFixed(1) + ' ' + bx.toFixed(1) + ',' + ry.toFixed(1) + ' ' + EW + ',' + ry.toFixed(1) + '" stroke="#3f3f46" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>';
        paths += '<polygon points="' + EW + ',' + ry.toFixed(1) + ' ' + (EW-5) + ',' + (ry-3).toFixed(1) + ' ' + (EW-5) + ',' + (ry+3).toFixed(1) + '" fill="#52525b"/>';
      });
      return '<svg width="' + EW + '" height="' + maxH + '" style="flex-shrink:0;">' + paths + '</svg>';
    }

    // Mixed-source SVG edge connector: conn = [srcColCount, srcIdx, tgtIdx]
    // Computes the left Y from the source column's own height and centering offset.
    function edgeSvgMixed(connections, rightCount) {
      var rH = colH(rightCount);
      var mt_r = (maxH - rH) / 2;
      var paths = '';
      connections.forEach(function(conn) {
        var srcCount = conn[0], srcIdx = conn[1], tgtIdx = conn[2];
        var mt_l = (maxH - colH(srcCount)) / 2;
        var ly = mt_l + srcIdx * (NH + NG) + NH / 2;
        var ry = mt_r + tgtIdx * (NH + NG) + NH / 2;
        var bx = EW * 0.5;
        paths += '<path d="M0,' + ly.toFixed(1) + ' C' + bx.toFixed(1) + ',' + ly.toFixed(1) + ' ' + bx.toFixed(1) + ',' + ry.toFixed(1) + ' ' + EW + ',' + ry.toFixed(1) + '" stroke="#3f3f46" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>';
        paths += '<polygon points="' + EW + ',' + ry.toFixed(1) + ' ' + (EW-5) + ',' + (ry-3).toFixed(1) + ' ' + (EW-5) + ',' + (ry+3).toFixed(1) + '" fill="#52525b"/>';
      });
      return '<svg width="' + EW + '" height="' + maxH + '" style="flex-shrink:0;">' + paths + '</svg>';
    }

    // Center each column vertically within maxH
    function wrapCol(nodes) {
      var ch = colH(nodes.length);
      var mt = Math.max(0, (maxH - ch) / 2);
      return '<div style="display:flex;flex-direction:column;gap:' + NG + 'px;margin-top:' + mt.toFixed(0) + 'px;">' + nodes.join('') + '</div>';
    }

    // Cluster → col3: gray lines to nodeGroups, indigo lines to cluster-level extensions
    function edgeSvgCol2ToCol3() {
      var mt_l = (maxH - colH(1)) / 2, mt_r = (maxH - colH(col3Nodes.length)) / 2;
      var paths = '';
      nodeGroups.forEach(function(_, gi) {
        var ly = mt_l + NH / 2, ry = mt_r + gi * (NH + NG) + NH / 2, bx = EW * 0.5;
        paths += '<path d="M0,' + ly.toFixed(1) + ' C' + bx + ',' + ly.toFixed(1) + ' ' + bx + ',' + ry.toFixed(1) + ' ' + EW + ',' + ry.toFixed(1) + '" stroke="#3f3f46" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>';
        paths += '<polygon points="' + EW + ',' + ry.toFixed(1) + ' ' + (EW-5) + ',' + (ry-3).toFixed(1) + ' ' + (EW-5) + ',' + (ry+3).toFixed(1) + '" fill="#52525b"/>';
      });
      clusterExtsList.forEach(function(_, ei) {
        var rIdx = nodeGroups.length + ei;
        var ly = mt_l + NH / 2, ry = mt_r + rIdx * (NH + NG) + NH / 2, bx = EW * 0.5;
        paths += '<path d="M0,' + ly.toFixed(1) + ' C' + bx + ',' + ly.toFixed(1) + ' ' + bx + ',' + ry.toFixed(1) + ' ' + EW + ',' + ry.toFixed(1) + '" stroke="#3f3f46" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>';
        paths += '<polygon points="' + EW + ',' + ry.toFixed(1) + ' ' + (EW-5) + ',' + (ry-3).toFixed(1) + ' ' + (EW-5) + ',' + (ry+3).toFixed(1) + '" fill="#52525b"/>';
      });
      return '<svg width="' + EW + '" height="' + maxH + '" style="flex-shrink:0;">' + paths + '</svg>';
    }

    var edge1 = edgeSvg(1, 1, [[0, 0]]);
    var edge2 = edgeSvgCol2ToCol3();
    var edge3 = hasIndividualMachines ? edgeSvg(col3Nodes.length, col4Nodes.length, machineConnections) : '';
    var edge4 = hasExtensions ? edgeSvgMixed(extConns, extNodes.length) : '';

    var graphId = 'g' + modal.id.replace(/[^a-zA-Z0-9]/g, '_');
    return '<div class="cluster-graph">' +
      '<div class="cluster-graph-toolbar">' +
        '<button class="cluster-graph-zoom-btn" onclick="window.__graphZoom(\'out\',\'' + graphId + '\')">&#8722;</button>' +
        '<span class="graph-zoom-level">100%</span>' +
        '<button class="cluster-graph-zoom-btn" onclick="window.__graphZoom(\'reset\',\'' + graphId + '\')">&#8635;</button>' +
        '<button class="cluster-graph-zoom-btn" onclick="window.__graphZoom(\'in\',\'' + graphId + '\')">&#43;</button>' +
      '</div>' +
      '<div class="cluster-graph-canvas" onwheel="window.__graphZoomWheel(event,this)" onmousedown="window.__graphDragStart(event,this)" onmousemove="window.__graphDragMove(event,this)" onmouseup="window.__graphDragEnd(this)" onmouseleave="window.__graphDragEnd(this)">' +
        '<div id="' + graphId + '" class="cluster-graph-inner" style="align-items:flex-start;gap:0;">' +
          wrapCol(col1Nodes) + edge1 +
          wrapCol(col2Nodes) + edge2 +
          wrapCol(col3Nodes) +
          (hasIndividualMachines ? edge3 + wrapCol(col4Nodes) : '') +
          (hasExtensions ? edge4 + wrapCol(extNodes) : '') +
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
          (currentModal.type === 'cluster' ? '<button class="drawer-tab ' + (currentModal.activeTab === 'diff' ? 'active' : '') + '" onclick="window.__setModalTab(\'diff\')">Diff</button>' : '') +
          (currentModal.type === 'cluster' ? '<button class="drawer-tab ' + (currentModal.activeTab === 'graph' ? 'active' : '') + '" onclick="window.__setModalTab(\'graph\')">Graph</button>' : '') +
        '</div>' : '') +
      '<div class="drawer-body' + (currentModal && currentModal.activeTab === 'graph' ? ' graph-mode' : '') + '">' +
        (currentModal ?
          (currentModal.activeTab === 'error' ? '<div style="color:#f87171;white-space:pre-wrap;">' + escHtml(currentModal.error) + '</div>' :
           currentModal.activeTab === 'live' ? (currentModal.liveContent ? '<pre style="margin:0;white-space:pre-wrap;">' + escHtml(currentModal.liveContent) + '</pre>' : '<div style="color:#71717a;text-align:center;padding:40px;">No live state available</div>') :
           currentModal.activeTab === 'diff' ? (currentModal.diff ? '<pre style="margin:0;white-space:pre-wrap;">' + formatDiff(currentModal.diff) + '</pre>' : '<div style="color:#71717a;text-align:center;padding:40px;">No diff available</div>') :
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
          '<div class="confirm-actions">' +
            '<button class="btn-cancel" onclick="window.__closeConfirmModal()">Cancel</button>' +
            '<button class="btn-confirm" onclick="window.__confirmAction()">Confirm</button>' +
          '</div>' +
        '</div>' +
      '</div>' +
    '</div>' +
    '<div class="modal ' + (logsModal ? 'show' : '') + '" onclick="if(event.target === this) window.__closeLogsModal()">' +
      '<div class="modal-content" style="max-width:1000px" onclick="event.stopPropagation()">' +
        '<div class="logs-modal-header">' +
          '<div class="logs-modal-title">Logs</div>' +
          '<div class="logs-modal-actions">' +
            '<button class="btn-download" onclick="window.__downloadLogs()">Download Logs</button>' +
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
    var el = document.getElementById('logs-modal-container');
    if (!el) return;
    var atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    el.innerHTML = state && state.logs && state.logs.length > 0
      ? state.logs.map(function(l) {
          return '<div class="log-entry"><span class="log-msg">' + l.message + '</span></div>';
        }).join('')
      : '<div class="log-entry" style="color:#52525b">No logs yet</div>';
    if (atBottom) el.scrollTop = el.scrollHeight;
  }

  function renderMainOnly() {
    // Re-render only the main app content, leaving the modals container untouched
    if (!state) return;
    var s = state;
    if (currentRoute === '/clusters') {
      app.innerHTML = renderClustersView(s);
    } else if (currentRoute === '/machineclasses') {
      app.innerHTML = renderMachineClassesView(s);
    } else if (currentRoute === '/repos') {
      app.innerHTML = renderReposView(s);
    } else if (currentRoute === '/users') {
      app.innerHTML = renderUsersView(s);
    } else {
      app.innerHTML = renderDashboardView(s);
    }
  }

  function downloadLogs() {
    if (!state || !state.logs) return;
    var blob = new Blob([JSON.stringify(state.logs, null, 2)], { type: 'application/json' });
    var url = window.URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = 'omni-cd-logs.json';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
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

  function renderDashboardView(s) {
    var clusters = s.clusters || [];
    var mcs = s.machineClasses || [];
    var totalClusters = clusters.length;
    var managedClusters = clusters.filter(function(c) { return c.status !== 'unmanaged'; });
    var countReady = 0, countNotReady = 0, countFailed = 0, countOutofsync = 0, countUnmanaged = 0;
    clusters.forEach(function(c) {
      var st = c.status || '';
      if (st === 'unmanaged') { countUnmanaged++; return; }
      if (st === 'failed') { countFailed++; return; }
      if (st === 'outofsync') { countOutofsync++; return; }
      if (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') { countNotReady++; return; }
      if (st === 'success' || st === 'applied' || st === 'synced' || st === 'syncing') countReady++;
    });
    var readyPct = managedClusters.length > 0 ? Math.round(countReady / managedClusters.length * 100) : 0;
    var omniHealth = getOmniHealth(s);
    var gitHealth  = getGitHealth(s);

    // ── 1. Fleet Health Bar ───────────────────────────────────────────────────
    var pReady     = totalClusters > 0 ? (countReady     / totalClusters * 100).toFixed(1) : 0;
    var pNotReady  = totalClusters > 0 ? (countNotReady  / totalClusters * 100).toFixed(1) : 0;
    var pFailed    = totalClusters > 0 ? (countFailed    / totalClusters * 100).toFixed(1) : 0;
    var pOutofsync = totalClusters > 0 ? (countOutofsync / totalClusters * 100).toFixed(1) : 0;
    var pUnmanaged = totalClusters > 0 ? (countUnmanaged / totalClusters * 100).toFixed(1) : 0;
    var legendItems = [];
    if (countReady)     legendItems.push('<span class="dash-fleet-legend-item"><span class="dash-fleet-dot" style="background:#4ade80"></span><b style="color:#e4e4e7">' + countReady     + '</b>&nbsp;Ready</span>');
    if (countNotReady)  legendItems.push('<span class="dash-fleet-legend-item"><span class="dash-fleet-dot" style="background:#f87171"></span><b style="color:#e4e4e7">' + countNotReady  + '</b>&nbsp;Not Ready</span>');
    if (countOutofsync) legendItems.push('<span class="dash-fleet-legend-item"><span class="dash-fleet-dot" style="background:#fb923c"></span><b style="color:#e4e4e7">' + countOutofsync + '</b>&nbsp;Out of Sync</span>');
    if (countFailed)    legendItems.push('<span class="dash-fleet-legend-item"><span class="dash-fleet-dot" style="background:#ef4444"></span><b style="color:#e4e4e7">' + countFailed    + '</b>&nbsp;Failed</span>');
    if (countUnmanaged) legendItems.push('<span class="dash-fleet-legend-item"><span class="dash-fleet-dot" style="background:#52525b"></span><b style="color:#e4e4e7">' + countUnmanaged + '</b>&nbsp;Unmanaged</span>');
    var fleetBar =
      '<div class="dash-fleet-card">' +
        '<div class="dash-fleet-title">Fleet Health</div>' +
        '<div class="dash-fleet-bar">' +
          (countReady     ? '<div class="dash-fleet-seg dash-fleet-seg--ready"     style="width:' + pReady     + '%" title="' + countReady     + ' ready"></div>'        : '') +
          (countNotReady  ? '<div class="dash-fleet-seg dash-fleet-seg--notready"  style="width:' + pNotReady  + '%" title="' + countNotReady  + ' not ready"></div>'   : '') +
          (countOutofsync ? '<div class="dash-fleet-seg dash-fleet-seg--outofsync" style="width:' + pOutofsync + '%" title="' + countOutofsync + ' out of sync"></div>' : '') +
          (countFailed    ? '<div class="dash-fleet-seg dash-fleet-seg--failed"    style="width:' + pFailed    + '%" title="' + countFailed    + ' failed"></div>'      : '') +
          (countUnmanaged ? '<div class="dash-fleet-seg dash-fleet-seg--unmanaged" style="width:' + pUnmanaged + '%" title="' + countUnmanaged + ' unmanaged"></div>'  : '') +
        '</div>' +
        '<div class="dash-fleet-legend">' + (legendItems.length ? legendItems.join('') : '<span style="color:#52525b;font-size:12px">No clusters</span>') + '</div>' +
      '</div>';

    // ── 2. Stat Strip ─────────────────────────────────────────────────────────
    var statStrip =
      '<div class="stat-strip">' +
        '<div class="stat-tile">' +
          '<div class="stat-tile-label">Clusters</div>' +
          '<div class="stat-tile-value">' + totalClusters + '</div>' +
          '<div class="stat-tile-sub">' + countReady + ' ready</div>' +
        '</div>' +
        '<div class="stat-tile">' +
          '<div class="stat-tile-label">Health</div>' +
          '<div class="stat-tile-value">' + readyPct + '<span style="font-size:16px;color:#71717a;font-weight:400">%</span></div>' +
          '<div class="mini-bar"><div class="mini-bar-fill" style="width:' + readyPct + '%"></div></div>' +
        '</div>' +
        '<div class="stat-tile">' +
          '<div class="stat-tile-label">Machine Classes</div>' +
          '<div class="stat-tile-value">' + mcs.length + '</div>' +
          '<div class="stat-tile-sub">Managed</div>' +
        '</div>' +
        '<div class="stat-tile">' +
          '<div class="stat-tile-label">Last Sync</div>' +
          '<div class="stat-tile-value" style="font-size:20px;letter-spacing:-0.5px">' + (ago(s.git && s.git.lastSync) || '—') + '</div>' +
          '<div class="stat-tile-sub">' + (s.git && s.git.shortSha ? 'SHA ' + s.git.shortSha : '—') + '</div>' +
        '</div>' +
      '</div>';

    // ── 3. Omni + Git cards ───────────────────────────────────────────────────
    var omniCard =
      '<div class="info-card">' +
        '<div class="info-card-header">' +
          '<span class="info-card-title">Omni Instance</span>' +
          '<span class="badge ' + gitHealthBadgeClass(omniHealth.status) + '">' + omniHealth.label + '</span>' +
        '</div>' +
        '<div class="info-card-value">' + (s.omniEndpoint ? '<a href="' + s.omniEndpoint + '" target="_blank" style="color:#FB326E;text-decoration:none">' + s.omniEndpoint + '</a>' : '—') + '</div>' +
        '<div class="info-card-sub">' +
          'Omni <b style="color:#a1a1aa">' + (s.omniVersion || '?') + '</b> &nbsp;&middot;&nbsp; omnictl <b style="color:#a1a1aa">' + (s.omnictlVersion || '?') + '</b><br>' +
          'Last check: ' + ago(s.omniHealth && s.omniHealth.lastCheck) +
          (s.omniHealth && s.omniHealth.error ? '<br><span style="color:#f87171">' + escHtml(s.omniHealth.error) + '</span>' : '') +
        '</div>' +
      '</div>';

    var gitCard =
      '<div class="info-card">' +
        '<div class="info-card-header">' +
          '<span class="info-card-title">Git</span>' +
          '<span class="badge ' + gitHealthBadgeClass(gitHealth.status) + '">' + gitHealth.label + '</span>' +
        '</div>' +
        '<div class="info-card-value">' + (s.git && s.git.repo ? '<a href="' + s.git.repo + '" target="_blank" style="color:#FB326E;text-decoration:none">' + s.git.repo + '</a>' : '—') + '</div>' +
        '<div class="info-card-sub">' +
          'Branch: <b style="color:#a1a1aa">' + (s.git && s.git.branch || '—') + '</b>' +
          (s.git && s.git.shortSha ? ' &nbsp;&middot;&nbsp; <b style="color:#a1a1aa">' + s.git.shortSha + '</b>' : '') + '<br>' +
          (s.git && s.git.commitMessage ? escHtml(s.git.commitMessage) + '<br>' : '') +
          'Last sync: ' + ago(s.git && s.git.lastSync) +
        '</div>' +
      '</div>';

    // ── 4. Reconcile Bar ──────────────────────────────────────────────────────
    var recType = s.lastReconcile.type === 'soft' ? 'Refresh' : s.lastReconcile.type === 'hard' ? 'Sync' : '—';
    var recDetail = recType;
    if (ts(s.lastReconcile.startedAt) !== '-') recDetail += ' &nbsp;·&nbsp; Started ' + ts(s.lastReconcile.startedAt);
    if (ts(s.lastReconcile.finishedAt) !== '-') recDetail += ' &nbsp;·&nbsp; Finished ' + ts(s.lastReconcile.finishedAt);
    var reconcileBar =
      '<div class="reconcile-bar">' +
        '<span class="reconcile-bar-label">Last Reconciliation</span>' +
        '<span class="badge ' + badgeClass(s.lastReconcile.status) + '">' + (s.lastReconcile.status || 'idle') + '</span>' +
        '<span class="reconcile-bar-detail">' + recDetail + '</span>' +
      '</div>';

    // ── 5. Panels (max 5 each) ────────────────────────────────────────────────
    var topClusters = clusters.slice().sort(function(a, b) { return a.id.localeCompare(b.id); }).slice(0, 5);
    var topMcs      = mcs.slice().sort(function(a, b) { return a.id.localeCompare(b.id); }).slice(0, 5);

    var clustersPanel =
      '<div class="panel">' +
        '<div class="panel-header">' +
          '<span class="panel-nav-link" onclick="window.location.href=\'/clusters\'">Clusters</span>' +
          '<div class="panel-header-right">' +
            '<span class="toggle-status ' + (s.clustersEnabled ? 'on' : 'off') + '">Auto Sync</span>' +
            '<button class="toggle-switch ' + (s.clustersEnabled ? 'on' : '') + '" onclick="window.__toggleClusters()">' +
              '<div class="toggle-knob"></div>' +
            '</button>' +
            '<span class="count">' + clusters.length + '</span>' +
          '</div>' +
        '</div>' +
        '<div class="resource-list">' +
          (topClusters.length > 0
            ? topClusters.map(function(r) {
                var isFailed = r.status === 'failed';
                var hasDiff  = r.diff && r.diff.length > 0;
                var badge = '';
                if (isFailed && hasDiff)          badge = '<span class="badge badge-outofsync">out of sync</span><span class="badge badge-failed">failed</span>';
                else if (r.status === 'outofsync') badge = '<span class="badge badge-outofsync">out of sync</span>';
                else if (r.status === 'success' || r.status === 'applied') badge = '<span class="badge badge-success">synced</span>';
                else if (isFailed)                 badge = '<span class="badge badge-failed">failed</span>';
                else if (r.status === 'unmanaged') badge = '<span class="badge badge-unmanaged">unmanaged</span>';
                else if (r.status === 'syncing')   badge = '<span class="badge badge-syncing">syncing</span>';
                else                               badge = '<span class="badge badge-idle">' + (r.status || '') + '</span>';
                var healthBadge = r.status !== 'unmanaged' && (r.clusterReady === 'ready' && r.kubernetesApiReady === 'ready')
                  ? '<span class="badge badge-ready">ready</span>'
                  : r.status !== 'unmanaged' && (r.clusterReady === 'not-ready' || r.kubernetesApiReady === 'not-ready')
                  ? '<span class="badge badge-notready">not ready</span>' : '';
                var exportBtn = r.status === 'unmanaged' ? '<button class="btn-export" onclick="window.__exportCluster(\'' + r.id + '\', event)">export</button>' : '';
                return '<div class="resource-item"><span class="resource-id">' + r.id + '</span>' +
                  '<div class="resource-right">' + exportBtn + healthBadge + badge + '</div></div>';
              }).join('') +
              (clusters.length > 5 ? '<div class="resource-item" style="justify-content:center"><a href="/clusters" style="color:#FB326E;font-size:12px;text-decoration:none">View all ' + clusters.length + ' clusters →</a></div>' : '')
            : '<div class="resource-item" style="color:#52525b">No clusters</div>') +
        '</div>' +
      '</div>';

    var mcsPanel =
      '<div class="panel">' +
        '<div class="panel-header">' +
          '<span class="panel-nav-link" onclick="window.location.href=\'/machineclasses\'">Machine Classes</span>' +
          '<span class="count">' + mcs.length + '</span>' +
        '</div>' +
        '<div class="resource-list">' +
          (topMcs.length > 0
            ? topMcs.map(function(m) {
                var displayStatus = m.status === 'success' ? 'synced' : m.status;
                var dot = m.status === 'success' || m.status === 'applied' ? '#4ade80'
                        : m.status === 'failed' ? '#f87171'
                        : m.status === 'outofsync' ? '#fb923c' : '#52525b';
                return '<div class="resource-item">' +
                  '<span class="resource-id" style="display:flex;align-items:center;gap:8px">' +
                    '<span style="width:7px;height:7px;border-radius:50%;background:' + dot + ';flex-shrink:0;display:inline-block"></span>' +
                    m.id +
                  '</span>' +
                  '<div class="resource-right">' +
                    (m.provisionType ? '<span class="provision-type ' + m.provisionType + '">' + (m.provisionType === 'auto' ? 'auto' : m.provisionType) + '</span>' : '') +
                    '<span class="badge ' + badgeClass(m.status) + '">' + displayStatus + '</span>' +
                  '</div>' +
                '</div>';
              }).join('') +
              (mcs.length > 5 ? '<div class="resource-item" style="justify-content:center"><a href="/machineclasses" style="color:#FB326E;font-size:12px;text-decoration:none">View all ' + mcs.length + ' →</a></div>' : '')
            : '<div class="resource-item" style="color:#52525b">No machine classes</div>') +
        '</div>' +
      '</div>';

    return renderHeader(s) +
      '<div class="info-row">' + omniCard + gitCard + '</div>' +
      fleetBar +
      '<div class="panels">' + clustersPanel + mcsPanel + '</div>';
  }

  function render() {
    if (!state) {
      app.innerHTML = '<div style="text-align:center;padding:60px;color:#52525b">Loading...</div>';
      modalsEl.innerHTML = '';
      return;
    }
    var s = state;

    if (currentRoute === '/clusters') {
      app.innerHTML = renderClustersView(s);
    } else if (currentRoute === '/machineclasses') {
      app.innerHTML = renderMachineClassesView(s);
    } else if (currentRoute === '/repos') {
      app.innerHTML = renderReposView(s);
    } else if (currentRoute === '/users') {
      app.innerHTML = renderUsersView(s);
    } else {
      app.innerHTML = renderDashboardView(s);
    }

    modalsEl.innerHTML = renderDrawer();

    // Auto-centre graph when the drawer opens on the graph tab
    if (currentModal && currentModal.activeTab === 'graph') {
      requestAnimationFrame(function() {
        var body = document.querySelector('.drawer-body');
        if (body) window.__graphCentre(body);
      });
    }

    if (logsModal) {
      var el = document.getElementById('logs-modal-container');
      if (el) el.scrollTop = el.scrollHeight;
    }
  }

  window.__triggerReconcile = triggerReconcile;
  window.__checkGit = checkGit;
  window.__toggleClusters = toggleClusters;
  window.__forceSync = forceSync;
  window.__exportCluster = exportCluster;
  window.__closeConfirmModal = closeConfirmModal;
  window.__confirmAction = confirmAction;
  window.__changeMachineClassPage = changeMachineClassPage;
  window.__changeClusterPage = changeClusterPage;
  window.__toggleMachineClassSort = toggleMachineClassSort;
  window.__toggleClusterSort = toggleClusterSort;
  window.__setClusterFilter = setClusterFilter;
  window.__clearClusterFilter = clearClusterFilter;
  window.__showClustersView = showClustersView;
  window.__hideClustersView = hideClustersView;
  window.__showLogsModal = showLogsModal;
  window.__closeLogsModal = closeLogsModal;
  window.__downloadLogs = downloadLogs;
  window.__showMachineClassModal = showMachineClassModal;
  window.__setMcPageSize = setMcPageSize;
  window.__setClusterPageSize = setClusterPageSize;

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
          if (logsModal) {
            // Update logs in-place to prevent flickering
            updateLogsInPlace();
          } else if (currentModal || confirmModal) {
            // Only update the main content, not the modal
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
