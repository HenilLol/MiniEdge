/**
 * MiniEdge Component - Advanced Log Viewer Console & Live Event Feed
 * Streaming terminal log console with real-time architectural event feed,
 * log level filters, service filter, error-only toggle, search, auto-scroll,
 * and single-click copy feedback.
 */

import { getIcon } from '../utils/svgIcons.js';
import { formatTime, formatMs, getStatusClass, escapeHtml } from '../utils/formatters.js';

export function renderLogViewer(container, state) {
  function showToast(message, type = 'info') {
    let toastContainer = document.getElementById('global-toast-container');
    if (!toastContainer) {
      toastContainer = document.createElement('div');
      toastContainer.id = 'global-toast-container';
      toastContainer.className = 'toast-container';
      document.body.appendChild(toastContainer);
    }

    const toast = document.createElement('div');
    toast.className = `toast-msg toast-${type}`;
    toast.innerHTML = `
      <span>${getIcon(type === 'success' ? 'checkCircle' : 'alertTriangle')}</span>
      <span>${message}</span>
    `;
    toastContainer.appendChild(toast);

    setTimeout(() => {
      toast.remove();
    }, 2400);
  }

  function getFilteredLogs() {
    return state.logs.filter(log => {
      // Level filter
      if (state.logLevelFilter !== 'ALL' && log.level !== state.logLevelFilter) {
        return false;
      }
      // Error-only filter toggle
      if (state.logErrorOnly && log.level !== 'ERROR' && (!log.status || log.status < 400)) {
        return false;
      }
      // Service filter
      if (state.selectedService !== 'all' && log.service !== state.selectedService && log.service !== 'gateway') {
        return false;
      }
      // Text search query
      if (state.logSearchQuery) {
        const q = state.logSearchQuery.toLowerCase();
        const matchMsg = log.message?.toLowerCase().includes(q);
        const matchPath = log.path?.toLowerCase().includes(q);
        const matchReqId = log.requestId?.toLowerCase().includes(q);
        if (!matchMsg && !matchPath && !matchReqId) return false;
      }
      return true;
    });
  }

  function getHtml() {
    const logs = getFilteredLogs();

    return `
      <div class="section-header">
        <div class="section-title-wrap">
          <div class="section-icon">${getIcon('terminal')}</div>
          <div>
            <h2 class="section-title">Gateway & Service Logs</h2>
            <p class="section-desc">Structured real-time stdout diagnostics, upstream status codes, and live architectural events</p>
          </div>
        </div>
        <div class="terminal-controls">
          <button class="btn btn-xs btn-outline" id="btn-copy-logs" title="Copy visible logs to clipboard">
            ${getIcon('copy')}
            <span>Copy Logs</span>
          </button>
          <button class="btn btn-xs btn-outline" id="btn-clear-logs" title="Clear log console">
            ${getIcon('trash')}
            <span>Clear</span>
          </button>
        </div>
      </div>

      <!-- Live Event Feed Banner -->
      <div class="live-event-feed-strip">
        <div class="feed-header-label">
          <span class="live-indicator"></span>
          <span>LIVE EVENTS:</span>
        </div>
        <div class="feed-items-scroll">
          ${state.logs.slice(0, 3).map(log => `
            <div class="feed-event-item">
              <span class="feed-time font-mono">${formatTime(log.timestamp)}</span>
              <span class="feed-level log-lvl-${log.level.toLowerCase()}">[${log.level}]</span>
              <span class="feed-msg font-mono">${escapeHtml(log.message)}</span>
            </div>
          `).join('')}
        </div>
      </div>

      <!-- Log Toolbar -->
      <div class="log-toolbar">
        <div class="log-level-pills">
          ${['ALL', 'INFO', 'WARN', 'ERROR'].map(lvl => `
            <button class="log-level-btn ${lvl.toLowerCase()} ${state.logLevelFilter === lvl ? 'active' : ''}" 
                    data-level="${lvl}">
              ${lvl}
            </button>
          `).join('')}
        </div>

        <button class="btn btn-xs btn-outline ${state.logErrorOnly ? 'active' : ''}" id="btn-toggle-error-only" title="Show only errors">
          ${getIcon('alertTriangle')}
          <span>Errors Only</span>
        </button>

        <div class="log-search-wrap">
          ${getIcon('search', 'search-icon')}
          <input type="text" 
                 id="log-search-input" 
                 class="form-control form-control-sm" 
                 placeholder="Filter log messages, request ID, path..." 
                 value="${escapeHtml(state.logSearchQuery)}">
        </div>

        <div class="log-autoscroll-toggle">
          <label class="toggle-label">
            <input type="checkbox" id="chk-autoscroll" ${state.isLogAutoScroll ? 'checked' : ''}>
            <span>Auto-scroll</span>
          </label>
        </div>
      </div>

      <!-- Log Terminal Box -->
      <div class="log-terminal-window" id="log-terminal-window">
        <div class="log-terminal-header">
          <div class="terminal-dots">
            <span class="dot red"></span>
            <span class="dot yellow"></span>
            <span class="dot green"></span>
          </div>
          <span class="terminal-title">miniedge-stdout.log</span>
          <span class="terminal-badge">${logs.length} entries</span>
        </div>

        <div class="log-entries-list" id="log-entries-list">
          ${logs.length === 0 ? `
            <div class="empty-terminal-msg">
              No log entries match the current filter.
            </div>
          ` : logs.map(log => {
            const lvlClass = `log-lvl-${log.level.toLowerCase()}`;
            const statusClass = log.status ? getStatusClass(log.status) : '';

            return `
              <div class="log-line ${lvlClass}" data-log-id="${log.id}" data-service-id="${log.service}" title="Click to copy line">
                <span class="log-col-time font-mono">${formatTime(log.timestamp)}</span>
                <span class="log-col-level ${lvlClass}">[${log.level}]</span>
                <span class="log-col-reqid font-mono">${log.requestId || '-'}</span>
                <span class="log-col-service">[${log.service}]</span>
                ${log.method ? `<span class="log-col-method method-${log.method.toLowerCase()}">${log.method}</span>` : ''}
                ${log.path ? `<span class="log-col-path font-mono">${escapeHtml(log.path)}</span>` : ''}
                ${log.status ? `<span class="log-col-status ${statusClass}">${log.status}</span>` : ''}
                ${log.latency ? `<span class="log-col-latency font-mono">${formatMs(log.latency)}</span>` : ''}
                <span class="log-col-msg">${escapeHtml(log.message)}</span>
              </div>
            `;
          }).join('')}
        </div>
      </div>
    `;
  }

  function render() {
    container.innerHTML = getHtml();
    attachListeners();
  }

  function attachListeners() {
    container.querySelectorAll('.log-level-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        const lvl = btn.getAttribute('data-level');
        state.setLogLevelFilter(lvl);
      });
    });

    container.querySelector('#btn-toggle-error-only')?.addEventListener('click', () => {
      state.setLogErrorOnly(!state.logErrorOnly);
    });

    const searchInput = container.querySelector('#log-search-input');
    searchInput?.addEventListener('input', (e) => {
      state.setLogSearchQuery(e.target.value);
    });

    const chkAutoScroll = container.querySelector('#chk-autoscroll');
    chkAutoScroll?.addEventListener('change', (e) => {
      state.isLogAutoScroll = e.target.checked;
    });

    container.querySelector('#btn-clear-logs')?.addEventListener('click', () => {
      state.clearLogs();
      showToast('Log console cleared', 'info');
    });

    // Copy all logs
    container.querySelector('#btn-copy-logs')?.addEventListener('click', () => {
      const logs = getFilteredLogs();
      const text = logs.map(l => 
        `[${formatTime(l.timestamp)}] [${l.level}] [${l.service}] ${l.method || ''} ${l.path || ''} ${l.status || ''} - ${l.message}`
      ).join('\n');
      
      navigator.clipboard.writeText(text).then(() => {
        showToast(`Copied ${logs.length} logs to clipboard`, 'success');
      });
    });

    // Single click to copy line & connected hover
    container.querySelectorAll('.log-line').forEach(line => {
      const sId = line.getAttribute('data-service-id');
      line.addEventListener('mouseenter', () => {
        if (sId && sId !== 'gateway') state.setHoveredService(sId);
      });
      line.addEventListener('mouseleave', () => {
        state.setHoveredService(null);
      });

      line.addEventListener('click', () => {
        const text = line.innerText;
        navigator.clipboard.writeText(text).then(() => {
          showToast('Log line copied to clipboard', 'info');
        });
      });
    });

    // Auto-scroll
    if (state.isLogAutoScroll) {
      const list = container.querySelector('#log-entries-list');
      if (list) list.scrollTop = 0;
    }
  }

  render();

  state.subscribe('logs', render);
  state.subscribe('logFilters', render);
  state.subscribe('filters', render);

  // Cross-Component Connected Hover Highlighting
  state.subscribe('hoveredService', ({ serviceId }) => {
    container.querySelectorAll('.log-line').forEach(line => {
      const sId = line.getAttribute('data-service-id');
      if (!serviceId) {
        line.classList.remove('is-log-dimmed', 'is-log-highlighted');
      } else if (sId === serviceId) {
        line.classList.add('is-log-highlighted');
        line.classList.remove('is-log-dimmed');
      } else {
        line.classList.add('is-log-dimmed');
        line.classList.remove('is-log-highlighted');
      }
    });
  });
}
