/**
 * MiniEdge Component - Advanced Request Explorer
 * Real-time filterable request table with live search, latency range filtering,
 * sort controls, and deep request inspection drawer.
 */

import { getIcon } from '../utils/svgIcons.js';
import { formatMs, formatTime, getStatusClass, escapeHtml } from '../utils/formatters.js';

export function renderRequestExplorer(container, state) {
  function getFilteredRequests() {
    let list = state.requests.filter(req => {
      // Service filter
      if (state.selectedService !== 'all' && req.service !== state.selectedService) {
        return false;
      }
      // Method filter
      if (state.selectedMethod !== 'all' && req.method !== state.selectedMethod) {
        return false;
      }
      // Status category filter
      if (state.selectedStatusCategory !== 'all') {
        const code = Number(req.status);
        if (state.selectedStatusCategory === '2xx' && (code < 200 || code >= 300)) return false;
        if (state.selectedStatusCategory === '4xx' && (code < 400 || code >= 500)) return false;
        if (state.selectedStatusCategory === '5xx' && code < 500) return false;
      }
      // Latency range filter
      if (state.selectedLatencyFilter !== 'all') {
        if (state.selectedLatencyFilter === '<50' && req.latency >= 50) return false;
        if (state.selectedLatencyFilter === '50-200' && (req.latency < 50 || req.latency > 200)) return false;
        if (state.selectedLatencyFilter === '>200' && req.latency <= 200) return false;
      }
      // Search query
      if (state.searchQuery) {
        const query = state.searchQuery.toLowerCase();
        const matchPath = req.path.toLowerCase().includes(query);
        const matchId = req.id.toLowerCase().includes(query);
        const matchService = req.service.toLowerCase().includes(query);
        if (!matchPath && !matchId && !matchService) return false;
      }
      return true;
    });

    // Sort order
    if (state.sortOrder === 'latency-desc') {
      list.sort((a, b) => b.latency - a.latency);
    } else if (state.sortOrder === 'status-desc') {
      list.sort((a, b) => b.status - a.status);
    } else {
      // newest first
      list.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
    }

    return list;
  }

  function getHtml() {
    const filtered = getFilteredRequests();
    const hasActiveFilters = state.selectedService !== 'all' || 
                             state.selectedMethod !== 'all' || 
                             state.selectedStatusCategory !== 'all' || 
                             state.selectedLatencyFilter !== 'all' || 
                             state.searchQuery;

    return `
      <div class="section-header">
        <div class="section-title-wrap">
          <div class="section-icon">${getIcon('search')}</div>
          <div>
            <h2 class="section-title">Request Explorer</h2>
            <p class="section-desc">Live stream of incoming client HTTP traffic routed through MiniEdge gateway</p>
          </div>
        </div>
        <div class="request-header-right">
          <div class="request-count-badge">
            <span>Showing ${filtered.length} of ${state.requests.length} requests</span>
          </div>
        </div>
      </div>

      <!-- Advanced Filter Toolbar -->
      <div class="filter-toolbar">
        <div class="search-input-wrap">
          ${getIcon('search', 'search-icon')}
          <input type="text" 
                 id="req-search-input" 
                 class="form-control form-control-sm" 
                 placeholder="Filter path, request ID, service..." 
                 value="${escapeHtml(state.searchQuery)}">
          ${state.searchQuery ? `<button class="btn-clear-search" id="btn-clear-req-search" title="Clear">×</button>` : ''}
        </div>

        <div class="filter-group">
          <label class="filter-label">Service:</label>
          <select id="filter-service-select" class="form-select form-select-sm">
            <option value="all" ${state.selectedService === 'all' ? 'selected' : ''}>All Services</option>
            ${state.services.map(s => `
              <option value="${s.id}" ${state.selectedService === s.id ? 'selected' : ''}>${s.name} (:${s.port})</option>
            `).join('')}
          </select>
        </div>

        <div class="filter-group">
          <label class="filter-label">Method:</label>
          <select id="filter-method-select" class="form-select form-select-sm">
            <option value="all" ${state.selectedMethod === 'all' ? 'selected' : ''}>All</option>
            <option value="GET" ${state.selectedMethod === 'GET' ? 'selected' : ''}>GET</option>
            <option value="POST" ${state.selectedMethod === 'POST' ? 'selected' : ''}>POST</option>
            <option value="PUT" ${state.selectedMethod === 'PUT' ? 'selected' : ''}>PUT</option>
            <option value="DELETE" ${state.selectedMethod === 'DELETE' ? 'selected' : ''}>DELETE</option>
          </select>
        </div>

        <div class="filter-group">
          <label class="filter-label">Status:</label>
          <select id="filter-status-select" class="form-select form-select-sm">
            <option value="all" ${state.selectedStatusCategory === 'all' ? 'selected' : ''}>All Statuses</option>
            <option value="2xx" ${state.selectedStatusCategory === '2xx' ? 'selected' : ''}>2xx Success</option>
            <option value="4xx" ${state.selectedStatusCategory === '4xx' ? 'selected' : ''}>4xx Client Error</option>
            <option value="5xx" ${state.selectedStatusCategory === '5xx' ? 'selected' : ''}>5xx Server Error</option>
          </select>
        </div>

        <div class="filter-group">
          <label class="filter-label">Latency:</label>
          <select id="filter-latency-select" class="form-select form-select-sm">
            <option value="all" ${state.selectedLatencyFilter === 'all' ? 'selected' : ''}>All Speeds</option>
            <option value="<50" ${state.selectedLatencyFilter === '<50' ? 'selected' : ''}>&lt; 50ms (Fast)</option>
            <option value="50-200" ${state.selectedLatencyFilter === '50-200' ? 'selected' : ''}>50 - 200ms</option>
            <option value=">200" ${state.selectedLatencyFilter === '>200' ? 'selected' : ''}>&gt; 200ms (Slow)</option>
          </select>
        </div>

        <div class="filter-group">
          <label class="filter-label">Sort:</label>
          <select id="filter-sort-select" class="form-select form-select-sm">
            <option value="newest" ${state.sortOrder === 'newest' ? 'selected' : ''}>Newest First</option>
            <option value="latency-desc" ${state.sortOrder === 'latency-desc' ? 'selected' : ''}>Latency (High to Low)</option>
            <option value="status-desc" ${state.sortOrder === 'status-desc' ? 'selected' : ''}>Status Code</option>
          </select>
        </div>

        ${hasActiveFilters ? `
          <button class="btn btn-xs btn-outline" id="btn-reset-filters">
            Reset Filters
          </button>
        ` : ''}
      </div>

      <!-- Requests Table Container -->
      <div class="table-container">
        <table class="data-table">
          <thead>
            <tr>
              <th style="width: 75px;">Method</th>
              <th>Path & Request ID</th>
              <th style="width: 90px;">Status</th>
              <th style="width: 100px;">Latency</th>
              <th style="width: 130px;">Target Service</th>
              <th style="width: 120px;">Timestamp</th>
              <th style="width: 60px; text-align: right;">Inspect</th>
            </tr>
          </thead>
          <tbody>
            ${filtered.length === 0 ? `
              <tr>
                <td colspan="7" class="empty-state-cell">
                  <div class="empty-terminal-msg">
                    No requests match the selected filters.
                  </div>
                </td>
              </tr>
            ` : filtered.map(req => {
              const statusClass = getStatusClass(req.status);
              const isSlow = req.latency > 500;
              const isDown = req.status >= 500;

              return `
                <tr class="request-row" data-request-id="${req.id}" data-service-id="${req.service}" tabindex="0" role="button" aria-label="Request ${req.method} ${req.path}">
                  <td>
                    <span class="method-badge method-${req.method.toLowerCase()}">${req.method}</span>
                  </td>
                  <td class="cell-path">
                    <span class="req-path-text">${escapeHtml(req.path)}</span>
                    <span class="req-id-sub font-mono">${req.id}</span>
                  </td>
                  <td>
                    <span class="status-pill ${statusClass}">
                      ${req.status}
                    </span>
                  </td>
                  <td>
                    <span class="latency-pill ${isDown ? 'latency-down' : isSlow ? 'latency-slow' : 'latency-fast'} font-mono">
                      ${formatMs(req.latency)}
                    </span>
                  </td>
                  <td>
                    <span class="service-tag tag-${req.service}">
                      ${escapeHtml(req.serviceName)}
                    </span>
                  </td>
                  <td class="cell-time font-mono">
                    ${formatTime(req.timestamp)}
                  </td>
                  <td style="text-align: right;">
                    <button class="btn-inspect-req" data-id="${req.id}" title="Inspect full request">
                      ${getIcon('externalLink')}
                    </button>
                  </td>
                </tr>
              `;
            }).join('')}
          </tbody>
        </table>
      </div>

      <!-- Request Inspector Modal Placeholder -->
      <div id="request-modal-placeholder"></div>
    `;
  }

  function renderModal(req) {
    const modalPlaceholder = container.querySelector('#request-modal-placeholder');
    if (!modalPlaceholder) return;

    if (!req) {
      modalPlaceholder.innerHTML = '';
      return;
    }

    const statusClass = getStatusClass(req.status);
    const serviceObj = state.services.find(s => s.id === req.service);

    modalPlaceholder.innerHTML = `
      <div class="modal-backdrop" id="modal-backdrop">
        <div class="modal-card">
          <div class="modal-header">
            <div class="modal-title-wrap">
              <span class="method-badge method-${req.method.toLowerCase()}">${req.method}</span>
              <span class="modal-path">${escapeHtml(req.path)}</span>
              <span class="status-pill ${statusClass}">${req.status}</span>
            </div>
            <button class="modal-close-btn" id="btn-close-modal">×</button>
          </div>

          <div class="modal-body">
            <!-- Timing & Routing summary -->
            <div class="modal-section">
              <h4 class="modal-section-title">Routing & Timing Breakdown</h4>
              <div class="modal-grid-3">
                <div class="info-box">
                  <span class="info-label">Upstream Node</span>
                  <span class="info-value font-mono">${escapeHtml(req.serviceName)} (:${serviceObj?.port || 3000})</span>
                </div>
                <div class="info-box">
                  <span class="info-label">Total Latency</span>
                  <span class="info-value font-mono ${req.latency > 500 ? 'text-warning' : 'text-success'}">${formatMs(req.latency)}</span>
                </div>
                <div class="info-box">
                  <span class="info-label">Timestamp</span>
                  <span class="info-value font-mono">${formatTime(req.timestamp)}</span>
                </div>
              </div>
            </div>

            <!-- Request Headers -->
            <div class="modal-section">
              <h4 class="modal-section-title">Gateway Inbound Headers</h4>
              <div class="code-block">
                <pre><code>${escapeHtml(JSON.stringify(req.headers, null, 2))}</code></pre>
              </div>
            </div>

            <!-- Response Body -->
            <div class="modal-section">
              <h4 class="modal-section-title">Upstream Response Payload</h4>
              <div class="code-block">
                <pre><code>${escapeHtml(JSON.stringify(req.responseBody, null, 2))}</code></pre>
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button class="btn btn-secondary btn-sm" id="btn-modal-close-footer">Close</button>
          </div>
        </div>
      </div>
    `;

    const closeModal = () => {
      state.selectedRequestDetail = null;
      renderModal(null);
    };

    modalPlaceholder.querySelector('#btn-close-modal')?.addEventListener('click', closeModal);
    modalPlaceholder.querySelector('#btn-modal-close-footer')?.addEventListener('click', closeModal);
    modalPlaceholder.querySelector('#modal-backdrop')?.addEventListener('click', (e) => {
      if (e.target.id === 'modal-backdrop') closeModal();
    });
  }

  function render() {
    container.innerHTML = getHtml();
    attachListeners();
    if (state.selectedRequestDetail) {
      renderModal(state.selectedRequestDetail);
    }
  }

  function attachListeners() {
    const searchInput = container.querySelector('#req-search-input');
    searchInput?.addEventListener('input', (e) => {
      state.setSearchQuery(e.target.value);
    });

    container.querySelector('#btn-clear-req-search')?.addEventListener('click', () => {
      state.setSearchQuery('');
    });

    container.querySelector('#filter-service-select')?.addEventListener('change', (e) => {
      state.setSelectedService(e.target.value);
    });

    container.querySelector('#filter-method-select')?.addEventListener('change', (e) => {
      state.setSelectedMethod(e.target.value);
    });

    container.querySelector('#filter-status-select')?.addEventListener('change', (e) => {
      state.setSelectedStatusCategory(e.target.value);
    });

    container.querySelector('#filter-latency-select')?.addEventListener('change', (e) => {
      state.setSelectedLatencyFilter(e.target.value);
    });

    container.querySelector('#filter-sort-select')?.addEventListener('change', (e) => {
      state.setSortOrder(e.target.value);
    });

    container.querySelector('#btn-reset-filters')?.addEventListener('click', () => {
      state.setSelectedService('all');
      state.setSelectedMethod('all');
      state.setSelectedStatusCategory('all');
      state.setSelectedLatencyFilter('all');
      state.setSearchQuery('');
      state.setSortOrder('newest');
    });

    container.querySelectorAll('.request-row').forEach(row => {
      const sId = row.getAttribute('data-service-id');
      row.addEventListener('mouseenter', () => {
        if (sId) state.setHoveredService(sId);
      });
      row.addEventListener('mouseleave', () => {
        state.setHoveredService(null);
      });
    });

    container.querySelectorAll('.btn-inspect-req, .request-row').forEach(el => {
      const inspect = () => {
        const id = el.getAttribute('data-id') || el.getAttribute('data-request-id');
        const req = state.requests.find(r => r.id === id);
        if (req) {
          state.selectedRequestDetail = req;
          renderModal(req);
        }
      };

      el.addEventListener('click', inspect);
      el.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          inspect();
        }
      });
    });
  }

  render();

  state.subscribe('requests', render);
  state.subscribe('filters', render);

  // Cross-Component Connected Hover Highlighting
  state.subscribe('hoveredService', ({ serviceId }) => {
    container.querySelectorAll('.request-row').forEach(row => {
      const sId = row.getAttribute('data-service-id');
      if (!serviceId) {
        row.classList.remove('is-row-dimmed', 'is-row-highlighted');
      } else if (sId === serviceId) {
        row.classList.add('is-row-highlighted');
        row.classList.remove('is-row-dimmed');
      } else {
        row.classList.add('is-row-dimmed');
        row.classList.remove('is-row-highlighted');
      }
    });
  });
}
