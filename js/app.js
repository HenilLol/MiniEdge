/**
 * MiniEdge - Main Application Coordinator
 * Boots up the Developer Command Center UI and orchestrates reactive components.
 */

import { state } from './state.js';
import { renderHeader } from './components/header.js';
import { renderOverviewMetrics } from './components/overviewMetrics.js';
import { renderServiceCards } from './components/serviceCards.js';
import { renderServiceMap } from './components/serviceMap.js';
import { renderMetricsCharts } from './components/metricsCharts.js';
import { renderRequestExplorer } from './components/requestExplorer.js';
import { renderLogViewer } from './components/logViewer.js';
import { renderSimulationControl } from './components/simulationControl.js';

document.addEventListener('DOMContentLoaded', () => {
  console.log('🚀 MiniEdge Developer Command Center initializing...');

  try {
    // 1. Mount Header
    const headerContainer = document.getElementById('header-container');
    if (headerContainer) renderHeader(headerContainer, state);

    // 2. Mount Overview Metrics KPIs
    const metricsContainer = document.getElementById('overview-metrics-container');
    if (metricsContainer) renderOverviewMetrics(metricsContainer, state);

    // 3. Mount Microservices Status Cards
    const servicesContainer = document.getElementById('service-cards-container');
    if (servicesContainer) renderServiceCards(servicesContainer, state);

    // 4. Mount Visual Dependency Topology Map
    const mapContainer = document.getElementById('service-map-container');
    if (mapContainer) renderServiceMap(mapContainer, state);

    // 5. Mount Zero-Dependency Metrics Canvas Charts
    const chartsContainer = document.getElementById('metrics-charts-container');
    if (chartsContainer) renderMetricsCharts(chartsContainer, state);

    // 6. Mount Request Explorer Stream Table
    const requestsContainer = document.getElementById('request-explorer-container');
    if (requestsContainer) renderRequestExplorer(requestsContainer, state);

    // 7. Mount Developer Terminal Log Console
    const logsContainer = document.getElementById('log-viewer-container');
    if (logsContainer) renderLogViewer(logsContainer, state);

    // 8. Mount Fault Injection & Failure Simulation Studio
    const simulationContainer = document.getElementById('simulation-control-container');
    if (simulationContainer) renderSimulationControl(simulationContainer, state);

    // Developer Keyboard Shortcuts
    window.addEventListener('keydown', (e) => {
      // Don't trigger shortcuts if user is typing in an input
      if (['INPUT', 'SELECT', 'TEXTAREA'].includes(e.target.tagName)) return;

      if (e.key === ' ' || e.code === 'Space') {
        e.preventDefault();
        state.toggleSimulation();
      } else if (e.key === 'r' || e.key === 'R') {
        e.preventDefault();
        state.manualRefresh();
      } else if (e.key === 'Escape') {
        const modal = document.querySelector('.modal-backdrop');
        if (modal) {
          state.selectedRequestDetail = null;
          const placeholder = document.getElementById('request-modal-placeholder');
          if (placeholder) placeholder.innerHTML = '';
        }
      }
    });

    console.log('✅ MiniEdge Command Center fully mounted and running.');
  } catch (err) {
    console.error('Fatal initialization error in MiniEdge:', err);
    document.body.innerHTML = `
      <div style="padding: 40px; color: #ef4444; font-family: monospace; background: #0f172a; min-height: 100vh;">
        <h2>MiniEdge Command Center: Failed to load</h2>
        <pre>${err.stack || err.message}</pre>
      </div>
    `;
  }
});
