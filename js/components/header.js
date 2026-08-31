/**
 * MiniEdge Component - Header & Navigation Shell
 * Manages branding, environment indicator, system health status, connection indicator,
 * stream controls, and developer section tab navigation.
 */

import { getIcon } from '../utils/svgIcons.js';
import { formatTime } from '../utils/formatters.js';

export function renderHeader(container, state) {
  function getActiveOverridesCount() {
    return Object.values(state.simulations).filter(s => s && (s.latencyAdd > 0 || s.forceStatus !== null)).length;
  }

  function getHtml() {
    const statusInfo = state.getSystemStatus();
    const activeOverrides = getActiveOverridesCount();

    return `
      <header class="top-nav">
        <div class="nav-left">
          <div class="brand-logo">
            <div class="logo-icon-wrap">
              ${getIcon('zap', 'logo-icon')}
            </div>
            <div class="brand-text">
              <div class="brand-title">
                <span class="brand-name">MiniEdge</span>
                <span class="brand-badge">v1.0.0</span>
                <span class="env-badge" title="Local Gateway Runtime">LOCAL DEV</span>
              </div>
              <p class="brand-subtitle">Developer Command Center</p>
            </div>
          </div>
        </div>

        <div class="nav-center">
          <div class="gateway-status-tag" title="MiniEdge Local Reverse Proxy (0.8ms overhead)">
            <span class="gateway-ping-dot"></span>
            <span>127.0.0.1:8080</span>
          </div>

          <div class="system-status-indicator status-${statusInfo.color}" id="header-status-badge" title="System Health State">
            <span class="status-dot"></span>
            <span class="status-text">${statusInfo.text}</span>
          </div>
        </div>

        <div class="nav-right">
          <div class="engine-mode-pill" title="Self-contained zero-dependency mock simulation engine">
            <span class="engine-dot"></span>
            <span>Mock Engine</span>
          </div>

          <div class="header-actions">
            <button class="btn btn-secondary btn-sm" id="btn-toggle-stream" title="Pause/Resume Live Simulation [Space]">
              <span id="stream-icon">${getIcon(state.isSimulationRunning ? 'pause' : 'play')}</span>
              <span id="stream-label">${state.isSimulationRunning ? 'Live' : 'Paused'}</span>
              <span class="kbd-hint">Space</span>
            </button>

            <button class="btn btn-primary btn-sm" id="btn-manual-refresh" title="Manually trigger immediate tick [R]">
              ${getIcon('refreshCw', 'refresh-icon')}
              <span>Refresh</span>
              <span class="kbd-hint" style="color: #bae6fd; border-color: #0284c7;">R</span>
            </button>
          </div>

          <div class="last-sync" id="last-sync-time" title="Last background poll timestamp">
            ${formatTime(state.lastUpdated)}
          </div>
        </div>
      </header>

      <!-- Developer Section Navigation Tabs -->
      <nav class="section-nav-bar" aria-label="Dashboard Sections">
        <div class="nav-tabs-list">
          <a href="#overview-metrics-container" class="nav-tab-item active" data-target="overview-metrics-container">
            ${getIcon('activity')}
            <span>Overview</span>
          </a>
          <a href="#service-cards-container" class="nav-tab-item" data-target="service-cards-container">
            ${getIcon('server')}
            <span>Services</span>
            <span class="tab-badge">${state.services.length} Nodes</span>
          </a>
          <a href="#service-map-container" class="nav-tab-item" data-target="service-map-container">
            ${getIcon('layers')}
            <span>Service Map</span>
          </a>
          <a href="#metrics-charts-container" class="nav-tab-item" data-target="metrics-charts-container">
            ${getIcon('activity')}
            <span>Metrics</span>
          </a>
          <a href="#request-explorer-container" class="nav-tab-item" data-target="request-explorer-container">
            ${getIcon('search')}
            <span>Requests</span>
            <span class="tab-badge" id="nav-req-count">${state.requests.length}</span>
          </a>
          <a href="#log-viewer-container" class="nav-tab-item" data-target="log-viewer-container">
            ${getIcon('terminal')}
            <span>Logs</span>
            <span class="tab-badge" id="nav-log-count">${state.logs.length}</span>
          </a>
          <a href="#simulation-control-container" class="nav-tab-item" data-target="simulation-control-container">
            ${getIcon('sliders')}
            <span>Failure Simulation</span>
            ${activeOverrides > 0 ? `<span class="tab-badge" style="color: #fbbf24; border-color: rgba(245,158,11,0.3);">${activeOverrides} Active</span>` : ''}
          </a>
        </div>
      </nav>
    `;
  }

  function render() {
    container.innerHTML = getHtml();
    attachListeners();
  }

  function attachListeners() {
    const btnToggleStream = container.querySelector('#btn-toggle-stream');
    const btnManualRefresh = container.querySelector('#btn-manual-refresh');
    const streamIcon = container.querySelector('#stream-icon');
    const streamLabel = container.querySelector('#stream-label');

    btnToggleStream?.addEventListener('click', () => {
      state.toggleSimulation();
      if (streamIcon) streamIcon.innerHTML = getIcon(state.isSimulationRunning ? 'pause' : 'play');
      if (streamLabel) streamLabel.textContent = state.isSimulationRunning ? 'Live' : 'Paused';
    });

    btnManualRefresh?.addEventListener('click', () => {
      const icon = btnManualRefresh.querySelector('.refresh-icon');
      icon?.classList.add('spin-once');
      setTimeout(() => icon?.classList.remove('spin-once'), 500);
      state.manualRefresh();
    });

    // Tab Navigation click handlers with smooth scrolling
    container.querySelectorAll('.nav-tab-item').forEach(tab => {
      tab.addEventListener('click', (e) => {
        e.preventDefault();
        const targetId = tab.getAttribute('data-target');
        const targetEl = document.getElementById(targetId);
        if (targetEl) {
          targetEl.scrollIntoView({ behavior: 'smooth' });
          container.querySelectorAll('.nav-tab-item').forEach(t => t.classList.remove('active'));
          tab.classList.add('active');
        }
      });
    });
  }

  render();

  state.subscribe('tick', () => {
    const status = state.getSystemStatus();
    const badge = document.getElementById('header-status-badge');
    const syncTime = document.getElementById('last-sync-time');
    const reqCount = document.getElementById('nav-req-count');
    const logCount = document.getElementById('nav-log-count');

    if (badge) {
      badge.className = `system-status-indicator status-${status.color}`;
      badge.innerHTML = `
        <span class="status-dot"></span>
        <span class="status-text">${status.text}</span>
      `;
    }

    if (syncTime) syncTime.textContent = formatTime(state.lastUpdated);
    if (reqCount) reqCount.textContent = state.requests.length;
    if (logCount) logCount.textContent = state.logs.length;
  });

  state.subscribe('services', render);
  state.subscribe('simulations', render);
}
