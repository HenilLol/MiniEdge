/**
 * MiniEdge Component - Service Dependency Topology Hero Visual
 * Visual interactive dependency topology: Client -> MiniEdge Gateway -> Microservices.
 * Pure zero-dependency SVG with animated traffic particles scaled by latency,
 * interactive zoom controls, latency labels, and rich hover tooltips.
 */

import { getIcon } from '../utils/svgIcons.js';
import { formatMs, formatNumber } from '../utils/formatters.js';

export function renderServiceMap(container, state) {
  let zoomLevel = 1.0;
  let hoveredNodeId = null;

  function getSvgHtml() {
    const services = state.services;
    const isSimRunning = state.isSimulationRunning;

    const getNodeColor = (status) => {
      if (status === 'UP') return { fill: '#10b981', border: '#059669', bg: 'rgba(16, 185, 129, 0.12)', text: '#34d399' };
      if (status === 'SLOW') return { fill: '#f59e0b', border: '#d97706', bg: 'rgba(245, 158, 11, 0.12)', text: '#fbbf24' };
      return { fill: '#ef4444', border: '#dc2626', bg: 'rgba(239, 68, 68, 0.12)', text: '#f87171' };
    };

    // Precise SVG Geometry (920 x 290 Viewport, Centered at Y=145)
    // Client: x=24, y=110, w=140, h=70 -> Right Anchor (164, 145)
    // Gateway: x=270, y=90, w=180, h=110 -> Left Anchor (270, 145), Right Anchor (450, 145)
    // Services (Users, Events, Orders, Payments): w=240, h=52, x=640
    const servicePositions = [
      { id: 'users', y: 16, cy: 42 },
      { id: 'events', y: 82, cy: 108 },
      { id: 'orders', y: 148, cy: 174 },
      { id: 'payments', y: 214, cy: 240 }
    ];

    const gwStatus = state.getSystemStatus();
    const gwColor = getNodeColor(gwStatus.color === 'up' ? 'UP' : gwStatus.color === 'slow' ? 'SLOW' : 'DOWN');

    return `
      <div class="section-header">
        <div class="section-title-wrap">
          <div class="section-icon">${getIcon('layers')}</div>
          <div>
            <h2 class="section-title">Service Dependency Topology</h2>
            <p class="section-desc">Real-time architectural flow from client edge through MiniEdge proxy to upstream microservices</p>
          </div>
        </div>

        <div class="map-header-right">
          <!-- Compact Legend -->
          <div class="map-legend">
            <span class="legend-item"><span class="legend-dot up"></span> Healthy</span>
            <span class="legend-item"><span class="legend-dot slow"></span> Degraded</span>
            <span class="legend-item"><span class="legend-dot down"></span> Down</span>
            <span class="legend-item"><span class="legend-dot traffic"></span> Traffic Pulse</span>
            <span class="legend-item"><span class="legend-dot dropped"></span> Dropped</span>
          </div>

          <!-- Map Zoom Controls -->
          <div class="map-controls-group" aria-label="Map Zoom Controls">
            <button class="btn btn-xs btn-outline map-ctrl-btn" id="map-zoom-in" title="Zoom In">+</button>
            <button class="btn btn-xs btn-outline map-ctrl-btn" id="map-zoom-out" title="Zoom Out">−</button>
            <button class="btn btn-xs btn-outline map-ctrl-btn" id="map-zoom-reset" title="Reset View">↺</button>
          </div>
        </div>
      </div>

      <div class="service-map-container" id="service-map-viewport">
        <!-- Interactive Tooltip Overlay -->
        <div class="map-hover-tooltip" id="map-hover-tooltip" style="display: none;"></div>

        <div class="service-map-scaler" id="service-map-scaler" style="transform: scale(${zoomLevel}); transform-origin: center center; transition: transform 0.2s ease;">
          <svg viewBox="0 0 920 290" class="service-map-svg" preserveAspectRatio="xMidYMid meet" aria-label="Microservice Topology Map">
            <defs>
              <!-- Arrowhead Markers -->
              <marker id="arrow-client" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#38bdf8" />
              </marker>
              <marker id="arrow-up" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#10b981" />
              </marker>
              <marker id="arrow-slow" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#f59e0b" />
              </marker>
              <marker id="arrow-down" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#ef4444" />
              </marker>
            </defs>

            <!-- ================= 1. CLIENT -> GATEWAY CONNECTION ================= -->
            <line x1="164" y1="145" x2="264" y2="145" stroke="#38bdf8" stroke-width="2" stroke-dasharray="3 3" marker-end="url(#arrow-client)" />

            <!-- ================= 2. GATEWAY -> SERVICES CONNECTIONS ================= -->
            ${servicePositions.map(pos => {
              const service = services.find(s => s.id === pos.id);
              const status = service?.status || 'UP';
              const colors = getNodeColor(status);
              const isDown = status === 'DOWN';
              const isSlow = status === 'SLOW';
              const isDimmed = hoveredNodeId && hoveredNodeId !== service.id;

              // Cubic bezier from Gateway right edge (450, 145) to Service left edge (634, pos.cy)
              const pathD = `M 450 145 C 520 145, 555 ${pos.cy}, 634 ${pos.cy}`;
              const marker = isDown ? 'url(#arrow-down)' : isSlow ? 'url(#arrow-slow)' : 'url(#arrow-up)';

              // Latency badge center on curve
              const badgeX = 535;
              const badgeY = (145 + pos.cy) / 2 + (pos.cy < 145 ? -10 : 10);

              // Particle speed: healthy 1.2s, slow 3.2s
              const animDur = isSlow ? '3.2s' : '1.2s';

              return `
                <g class="map-edge-group ${isDimmed ? 'is-dimmed' : ''}" data-service-id="${service.id}">
                  <!-- Connecting Bezier Path -->
                  <path d="${pathD}" 
                        fill="none" 
                        stroke="${colors.fill}" 
                        stroke-width="${isDown ? '1.5' : hoveredNodeId === service.id ? '2.8' : '2'}" 
                        stroke-dasharray="${isDown ? '4 4' : 'none'}" 
                        stroke-opacity="${isDown ? '0.45' : isDimmed ? '0.25' : '0.85'}" 
                        marker-end="${marker}" />

                  ${!isDown && isSimRunning ? `
                    <!-- Animated Traffic Particle Dot -->
                    <circle r="${hoveredNodeId === service.id ? '4' : '3'}" fill="#ffffff">
                      <animateMotion path="${pathD}" dur="${animDur}" repeatCount="indefinite" />
                    </circle>
                  ` : ''}

                  <!-- Edge Latency Pill Badge -->
                  <g class="map-latency-pill" transform="translate(${badgeX - 27}, ${badgeY - 9})">
                    <rect width="54" height="18" rx="3" fill="#080c14" stroke="${colors.border}" stroke-width="1" />
                    <text x="27" y="12" fill="${colors.text}" font-size="9" font-weight="700" text-anchor="middle" font-family="monospace">
                      ${isDown ? 'DROP' : formatMs(service.latency)}
                    </text>
                  </g>
                </g>
              `;
            }).join('')}

            <!-- ================= 3. CLIENT NODE ================= -->
            <g class="map-node client-node" transform="translate(24, 110)">
              <rect width="140" height="70" rx="6" fill="#131e33" stroke="#38bdf8" stroke-width="1.5" />
              
              <!-- Inbound Icon Indicator -->
              <circle cx="22" cy="35" r="10" fill="rgba(56, 189, 248, 0.15)" stroke="#38bdf8" stroke-width="1.5" />
              <circle cx="22" cy="35" r="4" fill="#38bdf8" />

              <text x="42" y="31" fill="#f8fafc" font-size="13" font-weight="700">Client</text>
              <text x="42" y="47" fill="#94a3b8" font-size="9.5" font-family="monospace">HTTP Inbound</text>
              <text x="42" y="60" fill="#38bdf8" font-size="9" font-family="monospace">External Traffic</text>
            </g>

            <!-- ================= 4. MINIEDGE GATEWAY NODE (Focal Point) ================= -->
            <g class="map-node gateway-node" transform="translate(270, 90)">
              <rect width="180" height="110" rx="8" fill="#0d1524" stroke="${gwColor.fill}" stroke-width="2" />
              <rect x="6" y="6" width="168" height="98" rx="6" fill="rgba(19, 30, 51, 0.5)" stroke="#1f2d47" stroke-width="1" />
              
              <!-- Gateway Icon Indicator -->
              <circle cx="26" cy="30" r="10" fill="${gwColor.bg}" stroke="${gwColor.fill}" stroke-width="1.5" />
              <circle cx="26" cy="30" r="4" fill="${gwColor.fill}" />
              
              <text x="44" y="28" fill="#ffffff" font-size="14" font-weight="800">MiniEdge</text>
              <text x="44" y="42" fill="#38bdf8" font-size="10" font-family="monospace">:8080 Core Proxy</text>
              
              <!-- Separator -->
              <line x1="14" y1="58" x2="166" y2="58" stroke="#1f2d47" stroke-width="1" />

              <text x="14" y="74" fill="#94a3b8" font-size="9.5">Local Dev Gateway</text>
              <text x="14" y="89" fill="${gwColor.text}" font-size="9.5" font-weight="700" font-family="monospace">${gwStatus.text}</text>
            </g>

            <!-- ================= 5. SERVICE NODES (Users, Events, Orders, Payments) ================= -->
            ${servicePositions.map(pos => {
              const service = services.find(s => s.id === pos.id);
              const status = service?.status || 'UP';
              const colors = getNodeColor(status);
              const isSelected = state.selectedService === service.id;
              const isDown = status === 'DOWN';
              const isDimmed = hoveredNodeId && hoveredNodeId !== service.id;

              return `
                <g class="map-node service-node ${isSelected ? 'is-selected' : ''} ${isDimmed ? 'is-dimmed' : ''}" 
                   transform="translate(640, ${pos.y})" 
                   data-service-id="${service.id}"
                   tabindex="0"
                   role="button"
                   aria-label="${service.name} (${status})">
                  
                  <!-- Node Card Rectangle -->
                  <rect width="240" height="52" rx="6" 
                        fill="${isSelected ? '#1c2b47' : '#131e33'}" 
                        stroke="${colors.fill}" 
                        stroke-width="${isSelected ? '2.5' : '1.2'}" />
                  
                  <!-- Left accent border stripe -->
                  <rect width="4" height="52" rx="2" fill="${colors.fill}" />

                  <!-- Status Dot Indicator -->
                  <circle cx="20" cy="26" r="7" fill="${colors.bg}" stroke="${colors.fill}" stroke-width="1.5" />
                  <circle cx="20" cy="26" r="3" fill="${colors.fill}" />

                  <!-- Service Name & Port -->
                  <text x="34" y="22" fill="#f8fafc" font-size="12" font-weight="700">${service.name}</text>
                  <text x="34" y="38" fill="#94a3b8" font-size="9.5" font-family="monospace">:${service.port} • ${service.endpoints.length} routes • ${isDown ? 'TIMEOUT' : formatMs(service.latency)}</text>
                  
                  <!-- Status Badge -->
                  <rect x="180" y="16" width="48" height="20" rx="3" fill="${colors.bg}" stroke="${colors.border}" stroke-width="1" />
                  <text x="204" y="30" fill="${colors.text}" font-size="10" font-weight="800" text-anchor="middle" font-family="monospace">${status}</text>
                </g>
              `;
            }).join('')}
          </svg>
        </div>
      </div>
    `;
  }

  function render() {
    container.innerHTML = getSvgHtml();
    attachListeners();
  }

  function attachListeners() {
    const scaler = container.querySelector('#service-map-scaler');
    const tooltip = container.querySelector('#map-hover-tooltip');
    const viewport = container.querySelector('#service-map-viewport');

    // Zoom In
    container.querySelector('#map-zoom-in')?.addEventListener('click', () => {
      zoomLevel = Math.min(1.4, zoomLevel + 0.1);
      if (scaler) scaler.style.transform = `scale(${zoomLevel})`;
    });

    // Zoom Out
    container.querySelector('#map-zoom-out')?.addEventListener('click', () => {
      zoomLevel = Math.max(0.7, zoomLevel - 0.1);
      if (scaler) scaler.style.transform = `scale(${zoomLevel})`;
    });

    // Reset Zoom
    container.querySelector('#map-zoom-reset')?.addEventListener('click', () => {
      zoomLevel = 1.0;
      if (scaler) scaler.style.transform = `scale(${zoomLevel})`;
    });

    // Service Nodes Click & Hover
    container.querySelectorAll('.service-node').forEach(node => {
      const selectService = () => {
        const id = node.getAttribute('data-service-id');
        state.setSelectedService(state.selectedService === id ? 'all' : id);
      };

      node.addEventListener('click', selectService);
      node.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          selectService();
        }
      });

      // Hover Tooltip & Dimming
      node.addEventListener('mouseenter', () => {
        const id = node.getAttribute('data-service-id');
        hoveredNodeId = id;
        state.setHoveredService(id);

        // Apply dimming classes
        container.querySelectorAll('.map-edge-group, .service-node').forEach(el => {
          if (el.getAttribute('data-service-id') !== id) {
            el.classList.add('is-dimmed');
          }
        });

        const service = state.services.find(s => s.id === id);
        if (!service || !tooltip) return;

        const isDown = service.status === 'DOWN';
        const isSlow = service.status === 'SLOW';

        tooltip.innerHTML = `
          <div class="tooltip-card">
            <div class="tooltip-header">
              <span class="tooltip-title">${service.name}</span>
              <span class="status-pill status-${service.status === 'UP' ? '2xx' : service.status === 'SLOW' ? '4xx' : '5xx'}">${service.status}</span>
            </div>
            <div class="tooltip-grid">
              <div class="tooltip-item"><span class="tooltip-label">Port:</span> <span class="font-mono">:${service.port}</span></div>
              <div class="tooltip-item"><span class="tooltip-label">Latency:</span> <span class="font-mono ${isDown ? 'text-danger' : isSlow ? 'text-warning' : 'text-success'}">${isDown ? 'TIMEOUT' : formatMs(service.latency)}</span></div>
              <div class="tooltip-item"><span class="tooltip-label">Requests:</span> <span class="font-mono">${formatNumber(service.requestCount)}</span></div>
              <div class="tooltip-item"><span class="tooltip-label">Errors:</span> <span class="font-mono ${service.errorCount > 0 ? 'text-danger' : ''}">${formatNumber(service.errorCount)}</span></div>
              <div class="tooltip-item"><span class="tooltip-label">Uptime:</span> <span class="font-mono">${service.uptimePercent}%</span></div>
              <div class="tooltip-item"><span class="tooltip-label">Routes:</span> <span class="font-mono">${service.endpoints.length} endpoints</span></div>
            </div>
            <div class="tooltip-footer">Click to filter dashboard traffic</div>
          </div>
        `;
        tooltip.style.display = 'block';

        // Position tooltip relative to viewport
        const rect = node.getBoundingClientRect();
        const viewRect = viewport.getBoundingClientRect();
        const topPos = rect.top - viewRect.top - 10;
        const leftPos = Math.max(10, rect.left - viewRect.left - 230);
        tooltip.style.top = `${topPos}px`;
        tooltip.style.left = `${leftPos}px`;
      });

      node.addEventListener('mouseleave', () => {
        hoveredNodeId = null;
        state.setHoveredService(null);
        container.querySelectorAll('.is-dimmed').forEach(el => el.classList.remove('is-dimmed'));
        if (tooltip) tooltip.style.display = 'none';
      });
    });
  }

  render();

  state.subscribe('services', render);
  state.subscribe('filters', render);
  state.subscribe('simulationStateChange', render);

  // Cross-Component Connected Hover Highlighting
  state.subscribe('hoveredService', ({ serviceId }) => {
    container.querySelectorAll('.map-edge-group, .service-node').forEach(el => {
      const sId = el.getAttribute('data-service-id');
      if (!serviceId) {
        el.classList.remove('is-dimmed');
      } else if (sId === serviceId) {
        el.classList.remove('is-dimmed');
      } else {
        el.classList.add('is-dimmed');
      }
    });
  });
}
