/**
 * MiniEdge Component - Overview Metrics & Mission Control Status Strip
 * Displays the System Health Hero, Command Center Status Strip, and high-density KPI cards.
 */

import { getIcon } from '../utils/svgIcons.js';
import { formatNumber, formatMs } from '../utils/formatters.js';

export function renderOverviewMetrics(container, state) {
  function getHtml() {
    const m = state.getOverviewMetrics();
    const statusInfo = state.getSystemStatus();
    const healthPercent = Math.round((m.healthyServices / m.totalServices) * 100);

    return `
      <!-- 1. Command Center Quick-Glance Status Strip -->
      <div class="mission-status-strip" aria-label="Command Center Quick Status">
        <div class="status-strip-item">
          <span class="strip-label">SYSTEM</span>
          <div class="strip-val-wrap">
            <span class="strip-dot status-${statusInfo.color}"></span>
            <span class="strip-val font-mono ${statusInfo.color === 'up' ? 'text-success' : statusInfo.color === 'slow' ? 'text-warning' : 'text-danger'}">${statusInfo.level}</span>
          </div>
        </div>

        <div class="status-strip-divider"></div>

        <div class="status-strip-item">
          <span class="strip-label">GATEWAY</span>
          <div class="strip-val-wrap">
            <span class="strip-val font-mono text-info">:8080 Proxy</span>
            <span class="strip-sub">0.8ms ovh</span>
          </div>
        </div>

        <div class="status-strip-divider"></div>

        <div class="status-strip-item">
          <span class="strip-label">SERVICES</span>
          <div class="strip-val-wrap">
            <span class="strip-val font-mono">${m.totalServices} Nodes</span>
            <span class="strip-sub">(${m.healthyServices} UP, ${m.slowServices} SLOW, ${m.downServices} DOWN)</span>
          </div>
        </div>

        <div class="status-strip-divider"></div>

        <div class="status-strip-item">
          <span class="strip-label">TRAFFIC</span>
          <div class="strip-val-wrap">
            <span class="strip-val font-mono text-info">${formatNumber(m.totalRequests)}</span>
            <span class="strip-sub">~40 req/s</span>
          </div>
        </div>

        <div class="status-strip-divider"></div>

        <div class="status-strip-item">
          <span class="strip-label">ERRORS</span>
          <div class="strip-val-wrap">
            <span class="strip-val font-mono ${Number(m.errorRate) > 3 ? 'text-danger' : Number(m.errorRate) > 0 ? 'text-warning' : 'text-success'}">${m.errorRate}%</span>
            <span class="strip-sub">(${m.totalErrors} errs)</span>
          </div>
        </div>

        <div class="status-strip-divider"></div>

        <div class="status-strip-item">
          <span class="strip-label">LATENCY</span>
          <div class="strip-val-wrap">
            <span class="strip-val font-mono ${m.avgLatency > 300 ? 'text-warning' : 'text-success'}">${formatMs(m.avgLatency)}</span>
            <span class="strip-sub">rolling avg</span>
          </div>
        </div>
      </div>

      <!-- 2. Overview Metrics Grid -->
      <div class="metrics-grid">
        <!-- Card 1: Total Services -->
        <div class="metric-card">
          <div class="metric-header">
            <span class="metric-title">Total Services</span>
            <div class="metric-icon-box info">
              ${getIcon('server')}
            </div>
          </div>
          <div class="metric-body">
            <div class="metric-value-row">
              <span class="metric-value font-mono">${m.totalServices}</span>
              <span class="metric-unit">Nodes</span>
            </div>
            <div class="metric-subtext">
              <span class="subtext-pill up">${m.healthyServices} UP</span>
              ${m.slowServices > 0 ? `<span class="subtext-pill slow">${m.slowServices} SLOW</span>` : ''}
              ${m.downServices > 0 ? `<span class="subtext-pill down">${m.downServices} DOWN</span>` : ''}
            </div>
          </div>
        </div>

        <!-- Card 2: Healthy Services -->
        <div class="metric-card">
          <div class="metric-header">
            <span class="metric-title">Health Ratio</span>
            <div class="metric-icon-box ${healthPercent === 100 ? 'success' : healthPercent >= 75 ? 'warning' : 'danger'}">
              ${getIcon('checkCircle')}
            </div>
          </div>
          <div class="metric-body">
            <div class="metric-value-row">
              <span class="metric-value font-mono ${healthPercent < 100 ? (healthPercent >= 75 ? 'text-warning' : 'text-danger') : 'text-success'}">
                ${m.healthyServices}/${m.totalServices}
              </span>
              <span class="metric-unit">(${healthPercent}%)</span>
            </div>
            <div class="metric-progress-bar">
              <div class="progress-fill ${healthPercent === 100 ? 'bg-success' : healthPercent >= 75 ? 'bg-warning' : 'bg-danger'}" 
                   style="width: ${healthPercent}%"></div>
            </div>
          </div>
        </div>

        <!-- Card 3: Total Requests -->
        <div class="metric-card">
          <div class="metric-header">
            <span class="metric-title">Total Requests</span>
            <div class="metric-icon-box info">
              ${getIcon('activity')}
            </div>
          </div>
          <div class="metric-body">
            <div class="metric-value-row">
              <span class="metric-value font-mono">${formatNumber(m.totalRequests)}</span>
              <span class="metric-unit">reqs</span>
            </div>
            <div class="metric-subtext text-muted font-mono">
              <span>~35-50 req/s live rate</span>
            </div>
          </div>
        </div>

        <!-- Card 4: Error Rate -->
        <div class="metric-card">
          <div class="metric-header">
            <span class="metric-title">Errors & Rate</span>
            <div class="metric-icon-box ${Number(m.errorRate) > 5 ? 'danger' : Number(m.errorRate) > 1 ? 'warning' : 'success'}">
              ${getIcon('alertTriangle')}
            </div>
          </div>
          <div class="metric-body">
            <div class="metric-value-row">
              <span class="metric-value font-mono ${Number(m.errorRate) > 3 ? 'text-danger' : Number(m.errorRate) > 1 ? 'text-warning' : 'text-success'}">
                ${m.errorRate}%
              </span>
              <span class="metric-unit">(${formatNumber(m.totalErrors)} errs)</span>
            </div>
            <div class="metric-subtext text-muted font-mono">
              <span>${m.downServices > 0 ? 'Elevated (outage active)' : 'Nominal threshold'}</span>
            </div>
          </div>
        </div>

        <!-- Card 5: Average Latency -->
        <div class="metric-card">
          <div class="metric-header">
            <span class="metric-title">Average Latency</span>
            <div class="metric-icon-box ${m.avgLatency > 300 ? 'warning' : 'success'}">
              ${getIcon('clock')}
            </div>
          </div>
          <div class="metric-body">
            <div class="metric-value-row">
              <span class="metric-value font-mono ${m.avgLatency > 300 ? 'text-warning' : 'text-success'}">${formatMs(m.avgLatency)}</span>
              <span class="metric-unit">rolling</span>
            </div>
            <div class="metric-subtext text-muted font-mono">
              <span>p50: ~30ms • p95: ~${m.slowServices > 0 ? '820ms' : '65ms'}</span>
            </div>
          </div>
        </div>
      </div>
    `;
  }

  function render() {
    container.innerHTML = getHtml();
  }

  render();

  state.subscribe('tick', render);
  state.subscribe('services', render);
}
