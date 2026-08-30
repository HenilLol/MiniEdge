/**
 * MiniEdge Component - Advanced Microservices Status Cards
 * Displays upstream nodes with UP/SLOW/DOWN distinction, live activity sparklines,
 * telemetry metrics, route endpoints, interactive fault triggers, and cross-component connected highlighting.
 */

import { getIcon } from '../utils/svgIcons.js';
import { formatNumber, formatMs, getServiceHealthClass } from '../utils/formatters.js';

export function renderServiceCards(container, state) {
  // Generate mini SVG sparkline for a service based on its latency and health
  function generateServiceSparkline(service) {
    const isDown = service.status === 'DOWN';
    const isSlow = service.status === 'SLOW';
    const color = isDown ? '#ef4444' : isSlow ? '#f59e0b' : '#10b981';
    const baseVal = isDown ? 0 : isSlow ? 80 : 25;

    // Generate 8 data points representing recent rolling response times
    const points = [];
    for (let i = 0; i < 8; i++) {
      const jitter = isDown ? 0 : Math.sin(i + Date.now() / 3000) * 12 + (i % 2 === 0 ? 5 : -5);
      const val = Math.max(2, baseVal + jitter);
      points.push(val);
    }

    const max = Math.max(...points, 100);
    const w = 120;
    const h = 22;

    const coords = points.map((p, i) => {
      const x = (w / (points.length - 1)) * i;
      const y = isDown ? h - 2 : Math.max(3, h - (p / max) * (h - 6));
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    });

    const polylineStr = coords.join(' ');
    const polygonStr = `0,${h} ${polylineStr} ${w},${h}`;

    return `
      <div class="service-sparkline-wrap" title="Live Activity Sparkline">
        <svg viewBox="0 0 ${w} ${h}" class="service-sparkline-svg" preserveAspectRatio="none">
          <polygon points="${polygonStr}" fill="${color}" fill-opacity="0.15" />
          <polyline points="${polylineStr}" fill="none" stroke="${color}" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </div>
    `;
  }

  function getHtml() {
    return `
      <div class="section-header">
        <div class="section-title-wrap">
          <div class="section-icon">${getIcon('server')}</div>
          <div>
            <h2 class="section-title">Microservices Status & Health</h2>
            <p class="section-desc">Upstream service nodes, health states, live activity sparklines, and fault triggers</p>
          </div>
        </div>
        <div class="section-actions">
          <button class="btn btn-sm btn-outline" id="btn-restore-all" title="Restore all services to healthy UP status">
            ${getIcon('checkCircle')}
            <span>Restore All to Healthy</span>
          </button>
        </div>
      </div>

      <div class="service-cards-grid">
        ${state.services.map(service => {
          const healthClass = getServiceHealthClass(service.status);
          const isSelected = state.selectedService === service.id;
          const isDown = service.status === 'DOWN';
          const isSlow = service.status === 'SLOW';

          return `
            <div class="service-card ${healthClass} ${isSelected ? 'selected' : ''}" data-service-id="${service.id}" tabindex="0" role="button" aria-label="${service.name} status ${service.status}">
              <div class="service-card-header">
                <div class="service-identity">
                  <div class="service-icon-wrap ${healthClass}">
                    ${getIcon(isDown ? 'xCircle' : isSlow ? 'alertTriangle' : 'cpu')}
                  </div>
                  <div>
                    <h3 class="service-name">${service.name}</h3>
                    <div class="service-meta">
                      <span class="service-port font-mono">:${service.port}</span>
                      <span class="service-version font-mono">${service.version}</span>
                    </div>
                  </div>
                </div>

                <div class="status-badge ${healthClass}">
                  <span class="badge-dot"></span>
                  <span class="badge-label font-mono">${service.status}</span>
                </div>
              </div>

              <div class="service-card-body">
                <div class="service-stat-grid">
                  <div class="service-stat-item">
                    <span class="stat-label">Latency</span>
                    <span class="stat-val ${isDown ? 'text-danger' : isSlow ? 'text-warning' : 'text-success'}">
                      ${isDown ? 'TIMEOUT' : formatMs(service.latency)}
                    </span>
                  </div>

                  <div class="service-stat-item">
                    <span class="stat-label">Requests</span>
                    <span class="stat-val font-mono">${formatNumber(service.requestCount)}</span>
                  </div>

                  <div class="service-stat-item">
                    <span class="stat-label">Errors</span>
                    <span class="stat-val font-mono ${service.errorCount > 100 ? 'text-danger' : service.errorCount > 0 ? 'text-warning' : ''}">
                      ${formatNumber(service.errorCount)}
                    </span>
                  </div>

                  <div class="service-stat-item">
                    <span class="stat-label">Uptime</span>
                    <span class="stat-val font-mono ${service.uptimePercent < 90 ? 'text-danger' : service.uptimePercent < 98 ? 'text-warning' : 'text-success'}">
                      ${service.uptimePercent}%
                    </span>
                  </div>
                </div>

                <!-- Live Sparkline Activity Graph -->
                <div class="service-sparkline-container">
                  <div class="sparkline-header">
                    <span class="sparkline-label">Activity Trend</span>
                    <span class="sparkline-status font-mono">${isDown ? 'OUTAGE' : isSlow ? 'DEGRADED' : 'OPTIMAL'}</span>
                  </div>
                  ${generateServiceSparkline(service)}
                </div>

                <div class="service-endpoints-list">
                  <span class="endpoints-title">Endpoints (${service.endpoints.length}):</span>
                  <div class="endpoints-tags">
                    ${service.endpoints.map(ep => `
                      <span class="endpoint-tag">
                        <span class="ep-method method-${ep.method.toLowerCase()}">${ep.method}</span>
                        <span class="ep-path font-mono">${ep.path}</span>
                      </span>
                    `).join('')}
                  </div>
                </div>
              </div>

              <div class="service-card-footer">
                <button class="btn btn-xs btn-outline btn-filter-service" data-id="${service.id}" title="Filter Explorer and Logs">
                  ${getIcon('filter')}
                  <span>${isSelected ? 'Filtering' : 'Filter'}</span>
                </button>

                <div class="card-quick-actions">
                  ${service.status !== 'UP' ? `
                    <button class="btn btn-xs btn-success-subtle btn-quick-restore" data-id="${service.id}" title="Restore to healthy UP">
                      Restore UP
                    </button>
                  ` : `
                    <button class="btn btn-xs btn-warning-subtle btn-quick-slow" data-id="${service.id}" title="Inject 800ms latency">
                      +800ms
                    </button>
                    <button class="btn btn-xs btn-danger-subtle btn-quick-fail" data-id="${service.id}" title="Simulate 500 failure">
                      Outage
                    </button>
                  `}
                </div>
              </div>
            </div>
          `;
        }).join('')}
      </div>
    `;
  }

  function render() {
    container.innerHTML = getHtml();
    attachListeners();
  }

  function attachListeners() {
    // Filter button click
    container.querySelectorAll('.btn-filter-service').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const id = btn.getAttribute('data-id');
        state.setSelectedService(state.selectedService === id ? 'all' : id);
      });
    });

    // Card selection & Connected Hover
    container.querySelectorAll('.service-card').forEach(card => {
      const id = card.getAttribute('data-service-id');
      card.addEventListener('click', () => {
        state.setSelectedService(state.selectedService === id ? 'all' : id);
      });

      card.addEventListener('mouseenter', () => {
        state.setHoveredService(id);
      });

      card.addEventListener('mouseleave', () => {
        state.setHoveredService(null);
      });
    });

    // Quick restore
    container.querySelectorAll('.btn-quick-restore').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const id = btn.getAttribute('data-id');
        state.restoreService(id);
      });
    });

    // Quick slow
    container.querySelectorAll('.btn-quick-slow').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const id = btn.getAttribute('data-id');
        state.injectLatency(id, 800);
      });
    });

    // Quick fail
    container.querySelectorAll('.btn-quick-fail').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const id = btn.getAttribute('data-id');
        state.simulateFailure(id, 'DOWN');
      });
    });

    // Restore all
    container.querySelector('#btn-restore-all')?.addEventListener('click', () => {
      state.restoreAllServices();
    });
  }

  render();

  state.subscribe('services', render);
  state.subscribe('filters', render);

  // Cross-Component Connected Hover Highlighting
  state.subscribe('hoveredService', ({ serviceId }) => {
    container.querySelectorAll('.service-card').forEach(card => {
      const id = card.getAttribute('data-service-id');
      if (!serviceId) {
        card.classList.remove('is-dimmed', 'is-hovered');
      } else if (id === serviceId) {
        card.classList.add('is-hovered');
        card.classList.remove('is-dimmed');
      } else {
        card.classList.add('is-dimmed');
        card.classList.remove('is-hovered');
      }
    });
  });
}
