/**
 * MiniEdge Component - Failure Simulation & Chaos Studio
 * Interactive fault injection controls for simulating latency spikes, 500/503 outages,
 * and service restoration with real-time propagation across the entire command center.
 * Labeled: "LOCAL SIMULATION / INTENTIONAL FAILURE".
 */

import { getIcon } from '../utils/svgIcons.js';

export function renderSimulationControl(container, state) {
  let selectedTargetService = 'users';

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
      <span>${getIcon(type === 'success' ? 'checkCircle' : type === 'danger' ? 'xCircle' : 'alertTriangle')}</span>
      <span>${message}</span>
    `;
    toastContainer.appendChild(toast);

    setTimeout(() => {
      toast.remove();
    }, 2800);
  }

  function getHtml() {
    return `
      <div class="simulation-panel">
        <div class="simulation-header">
          <div class="section-title-wrap">
            <div class="section-icon text-warning">${getIcon('sliders')}</div>
            <div>
              <h2 class="section-title">Fault Injection & Chaos Studio</h2>
              <p class="section-desc">Test circuit breakers, timeout resiliency, and alert triggers under simulated upstream degradation</p>
            </div>
          </div>

          <div class="simulation-notice-badge">
            ${getIcon('shieldAlert', 'notice-icon')}
            <span>LOCAL SIMULATION / INTENTIONAL FAILURE</span>
          </div>
        </div>

        <div class="simulation-body-grid">
          <!-- Column 1: Target Selector & Action Builder -->
          <div class="sim-builder-card">
            <h3 class="sim-card-title">1. Target Upstream Service</h3>
            
            <div class="service-selector-radio">
              ${state.services.map(s => `
                <label class="service-radio-label ${selectedTargetService === s.id ? 'active' : ''}">
                  <input type="radio" name="sim-target" value="${s.id}" ${selectedTargetService === s.id ? 'checked' : ''}>
                  <span class="radio-service-name">${s.name}</span>
                  <span class="radio-service-port font-mono">:${s.port}</span>
                  <span class="radio-status-tag ${s.status.toLowerCase()}">${s.status}</span>
                </label>
              `).join('')}
            </div>

            <h3 class="sim-card-title" style="margin-top: 14px;">2. Inject Fault Scenario</h3>
            <div class="sim-actions-container">
              <!-- Latency Actions -->
              <div class="action-group">
                <span class="action-group-label">Inject Artificial Latency:</span>
                <div class="btn-group-row">
                  <button class="btn btn-sm btn-outline btn-inject-latency" data-ms="200">+200ms</button>
                  <button class="btn btn-sm btn-outline btn-inject-latency" data-ms="800">+800ms</button>
                  <button class="btn btn-sm btn-outline btn-inject-latency" data-ms="2500">+2.5s (Lag)</button>
                </div>
              </div>

              <!-- Failure Actions -->
              <div class="action-group">
                <span class="action-group-label">Simulate Failure / Error:</span>
                <div class="btn-group-row">
                  <button class="btn btn-sm btn-danger-subtle btn-trigger-fail" data-type="DOWN">500 Outage / Drop</button>
                  <button class="btn btn-sm btn-warning-subtle btn-trigger-slow" data-type="SLOW">Degrade to SLOW</button>
                </div>
              </div>

              <!-- Restore Action -->
              <div class="action-group" style="margin-top: 6px;">
                <button class="btn btn-sm btn-success-subtle btn-restore-target" style="width: 100%;">
                  ${getIcon('checkCircle')}
                  <span>Restore Selected Service to Healthy (UP)</span>
                </button>
              </div>
            </div>
          </div>

          <!-- Column 2: Active Fault Status & Presets -->
          <div class="sim-status-card">
            <div class="sim-status-header">
              <h3 class="sim-card-title">Active Fault Overrides</h3>
              <button class="btn btn-xs btn-outline" id="btn-reset-all-sims">
                Restore All
              </button>
            </div>

            <div class="active-sims-list">
              ${state.services.map(s => {
                const sim = state.simulations[s.id];
                const hasOverride = sim && (sim.latencyAdd > 0 || sim.forceStatus !== null);

                return `
                  <div class="active-sim-item ${hasOverride ? 'has-override' : ''}">
                    <div class="sim-item-info">
                      <span class="sim-item-name">${s.name} (:${s.port})</span>
                      <span class="sim-item-status status-${s.status.toLowerCase()}">${s.status}</span>
                    </div>
                    <div class="sim-item-details">
                      ${sim?.latencyAdd > 0 ? `<span class="sim-tag latency">+${sim.latencyAdd}ms Latency</span>` : ''}
                      ${sim?.forceStatus ? `<span class="sim-tag status">Forced ${sim.forceStatus}</span>` : ''}
                      ${!hasOverride ? `<span class="sim-tag healthy">Healthy</span>` : ''}
                    </div>
                    ${hasOverride ? `
                      <button class="btn btn-xs btn-outline btn-item-restore" data-id="${s.id}" title="Remove override and restore">
                        Restore
                      </button>
                    ` : ''}
                  </div>
                `;
              }).join('')}
            </div>

            <!-- Demo Presets -->
            <div class="chaos-presets-wrap">
              <h4 class="preset-title">Demo Presets:</h4>
              <div class="preset-buttons">
                <button class="btn btn-xs btn-outline btn-preset" data-preset="baseline">
                  All Healthy (100% UP)
                </button>
                <button class="btn btn-xs btn-outline btn-preset" data-preset="default">
                  Default Demo (Orders DOWN, Events SLOW)
                </button>
                <button class="btn btn-xs btn-outline btn-preset" data-preset="chaos">
                  ${getIcon('flame')}
                  <span>Random Chaos</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    `;
  }

  function render() {
    container.innerHTML = getHtml();
    attachListeners();
  }

  function attachListeners() {
    container.querySelectorAll('input[name="sim-target"]').forEach(radio => {
      radio.addEventListener('change', (e) => {
        selectedTargetService = e.target.value;
        render();
      });
    });

    container.querySelectorAll('.btn-inject-latency').forEach(btn => {
      btn.addEventListener('click', () => {
        const ms = Number(btn.getAttribute('data-ms'));
        state.injectLatency(selectedTargetService, ms);
        showToast(`[SIMULATION] Injected +${ms}ms latency to ${selectedTargetService}`, 'warning');
      });
    });

    container.querySelectorAll('.btn-trigger-fail').forEach(btn => {
      btn.addEventListener('click', () => {
        state.simulateFailure(selectedTargetService, 'DOWN');
        showToast(`[SIMULATION] Simulated 500 Outage on ${selectedTargetService}`, 'danger');
      });
    });

    container.querySelectorAll('.btn-trigger-slow').forEach(btn => {
      btn.addEventListener('click', () => {
        state.injectLatency(selectedTargetService, 750);
        showToast(`[SIMULATION] Degraded ${selectedTargetService} to SLOW`, 'warning');
      });
    });

    container.querySelector('.btn-restore-target')?.addEventListener('click', () => {
      state.restoreService(selectedTargetService);
      showToast(`[RESTORED] ${selectedTargetService} service is now UP`, 'success');
    });

    container.querySelectorAll('.btn-item-restore').forEach(btn => {
      btn.addEventListener('click', () => {
        const id = btn.getAttribute('data-id');
        state.restoreService(id);
        showToast(`[RESTORED] ${id} service is now UP`, 'success');
      });
    });

    container.querySelector('#btn-reset-all-sims')?.addEventListener('click', () => {
      state.restoreAllServices();
      showToast(`[RESTORED] All services restored to Healthy UP`, 'success');
    });

    container.querySelectorAll('.btn-preset').forEach(btn => {
      btn.addEventListener('click', () => {
        const preset = btn.getAttribute('data-preset');
        if (preset === 'baseline') {
          state.restoreAllServices();
          showToast(`Preset Applied: All Healthy (100% UP)`, 'success');
        } else if (preset === 'default') {
          state.restoreAllServices();
          state.simulateFailure('orders', 'DOWN');
          state.injectLatency('events', 750);
          showToast(`Preset Applied: Orders DOWN, Events SLOW`, 'warning');
        } else if (preset === 'chaos') {
          state.triggerChaosPreset();
          showToast(`Preset Applied: Random Chaos Mode`, 'danger');
        }
      });
    });
  }

  render();

  state.subscribe('services', render);
  state.subscribe('simulations', render);
}
