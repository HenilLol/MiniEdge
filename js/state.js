/**
 * MiniEdge - Centralized Reactive State Store
 * Manages services, requests, logs, metrics, simulation rules, and event subscriptions.
 */

import {
  initialServices,
  activeSimulations,
  generateSeedRequests,
  generateSeedLogs,
  generateSeedMetricsHistory,
  generateSimulatedRequest
} from './api/mockData.js';

class StateStore {
  constructor() {
    this.services = JSON.parse(JSON.stringify(initialServices));
    this.simulations = JSON.parse(JSON.stringify(activeSimulations));
    this.requests = generateSeedRequests();
    this.logs = generateSeedLogs();
    this.metricsHistory = generateSeedMetricsHistory();

    // UI & Filter states
    this.selectedService = 'all';
    this.selectedMethod = 'all';
    this.selectedStatusCategory = 'all';
    this.selectedLatencyFilter = 'all'; // 'all' | '<50' | '50-200' | '>200'
    this.sortOrder = 'newest'; // 'newest' | 'latency-desc' | 'status-desc'
    this.searchQuery = '';
    this.logLevelFilter = 'ALL';
    this.logSearchQuery = '';
    this.logErrorOnly = false;
    this.selectedRequestDetail = null;
    this.isLogAutoScroll = true;
    this.metricsTimeRange = '5m'; // '5m' | '15m' | '1h'

    // Simulation runtime state
    this.isSimulationRunning = true;
    this.simulationIntervalId = null;
    this.lastUpdated = new Date();

    // Event listeners
    this.listeners = new Map();

    // Start background traffic ticker
    this.startTrafficSimulation();
  }

  // Event Subscription System
  subscribe(event, callback) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event).add(callback);
    return () => this.listeners.get(event).delete(callback);
  }

  emit(event, data) {
    if (this.listeners.has(event)) {
      this.listeners.get(event).forEach(cb => {
        try {
          cb(data, this);
        } catch (err) {
          console.error(`Error in listener for ${event}:`, err);
        }
      });
    }
    if (event !== 'all' && this.listeners.has('all')) {
      this.listeners.get('all').forEach(cb => cb({ event, data }, this));
    }
  }

  // System Health Evaluator
  getSystemStatus() {
    const hasDown = this.services.some(s => s.status === 'DOWN');
    const hasSlow = this.services.some(s => s.status === 'SLOW');

    if (hasDown) return { level: 'CRITICAL', text: 'Critical Outage Detected', color: 'down' };
    if (hasSlow) return { level: 'DEGRADED', text: 'Degraded Performance', color: 'slow' };
    return { level: 'OPERATIONAL', text: 'All Systems Operational', color: 'up' };
  }

  // Overview metrics computation
  getOverviewMetrics() {
    const totalServices = this.services.length;
    const healthyServices = this.services.filter(s => s.status === 'UP').length;
    const slowServices = this.services.filter(s => s.status === 'SLOW').length;
    const downServices = this.services.filter(s => s.status === 'DOWN').length;

    const totalRequests = this.services.reduce((acc, s) => acc + s.requestCount, 0);
    const totalErrors = this.services.reduce((acc, s) => acc + s.errorCount, 0);
    const errorRate = totalRequests > 0 ? ((totalErrors / totalRequests) * 100).toFixed(2) : '0.00';

    // Weighted average latency of active services
    const activeServices = this.services.filter(s => s.status !== 'DOWN');
    const avgLatency = activeServices.length > 0
      ? Math.round(activeServices.reduce((acc, s) => acc + s.latency, 0) / activeServices.length)
      : 0;

    return {
      totalServices,
      healthyServices,
      slowServices,
      downServices,
      totalRequests,
      totalErrors,
      errorRate,
      avgLatency
    };
  }

  // Background traffic generator loop
  startTrafficSimulation() {
    if (this.simulationIntervalId) clearInterval(this.simulationIntervalId);
    this.isSimulationRunning = true;

    this.simulationIntervalId = setInterval(() => {
      if (!this.isSimulationRunning) return;
      this.tick();
    }, 1800);
  }

  pauseTrafficSimulation() {
    this.isSimulationRunning = false;
    this.emit('simulationStateChange', { isRunning: false });
  }

  resumeTrafficSimulation() {
    this.isSimulationRunning = true;
    this.emit('simulationStateChange', { isRunning: true });
  }

  toggleSimulation() {
    if (this.isSimulationRunning) {
      this.pauseTrafficSimulation();
    } else {
      this.resumeTrafficSimulation();
    }
  }

  // Single simulation step
  tick() {
    const batchSize = Math.floor(1 + Math.random() * 3); // 1-3 requests per tick
    let errorsInTick = 0;
    let latenciesInTick = [];

    for (let i = 0; i < batchSize; i++) {
      const { request, log, targetServiceId, isError, latency } = generateSimulatedRequest(
        this.services,
        this.simulations
      );

      // Prepend to requests list (keep max 100)
      this.requests.unshift(request);
      if (this.requests.length > 100) this.requests.pop();

      // Prepend to logs list (keep max 150)
      this.logs.unshift(log);
      if (this.logs.length > 150) this.logs.pop();

      // Update service statistics
      const sIndex = this.services.findIndex(s => s.id === targetServiceId);
      if (sIndex !== -1) {
        this.services[sIndex].requestCount += 1;
        if (isError) {
          this.services[sIndex].errorCount += 1;
          errorsInTick++;
        }
        if (this.services[sIndex].status !== 'DOWN') {
          // Smooth latency updates
          this.services[sIndex].latency = Math.round(this.services[sIndex].latency * 0.7 + latency * 0.3);
        }
      }
      latenciesInTick.push(latency);
    }

    // Update time-series metrics buffers
    const now = new Date();
    const timeStr = now.toLocaleTimeString([], { hour12: false });
    this.metricsHistory.timestamps.push(timeStr);
    this.metricsHistory.throughput.push(batchSize * 15 + Math.floor(Math.random() * 10)); // normalized req/s
    this.metricsHistory.errors.push(errorsInTick * 4);

    const avgP50 = latenciesInTick.length > 0
      ? Math.round(latenciesInTick.reduce((a, b) => a + b, 0) / latenciesInTick.length)
      : 25;
    this.metricsHistory.latencyP50.push(avgP50);
    this.metricsHistory.latencyP95.push(Math.round(avgP50 * 2.8 + Math.random() * 50));

    // Keep buffer capped at 24 points
    if (this.metricsHistory.timestamps.length > 24) {
      this.metricsHistory.timestamps.shift();
      this.metricsHistory.throughput.shift();
      this.metricsHistory.errors.shift();
      this.metricsHistory.latencyP50.shift();
      this.metricsHistory.latencyP95.shift();
    }

    this.lastUpdated = now;

    // Emit reactive notifications
    this.emit('tick', { timestamp: now });
    this.emit('requests', this.requests);
    this.emit('logs', this.logs);
    this.emit('services', this.services);
    this.emit('metrics', this.metricsHistory);
  }

  // Simulation Controls Actions
  injectLatency(serviceId, latencyMs) {
    if (!this.simulations[serviceId]) {
      this.simulations[serviceId] = { latencyAdd: 0, forceStatus: null, errorRate: 0.01 };
    }
    this.simulations[serviceId].latencyAdd = latencyMs;
    this.simulations[serviceId].forceStatus = 'SLOW';

    const s = this.services.find(item => item.id === serviceId);
    if (s) {
      s.status = 'SLOW';
      s.latency = (s.latency || 25) + latencyMs;
    }

    this.logs.unshift({
      id: `log_sim_${Date.now()}`,
      timestamp: new Date().toISOString(),
      level: 'WARN',
      service: serviceId,
      method: 'SIMULATION',
      path: '/chaos/latency',
      status: 200,
      latency: latencyMs,
      requestId: 'sim_injection',
      message: `[SIMULATION APPLIED] Injected +${latencyMs}ms artificial latency to ${serviceId} service`
    });

    this.emit('services', this.services);
    this.emit('simulations', this.simulations);
    this.emit('logs', this.logs);
  }

  simulateFailure(serviceId, statusType = 'DOWN') {
    if (!this.simulations[serviceId]) {
      this.simulations[serviceId] = { latencyAdd: 0, forceStatus: null, errorRate: 0.01 };
    }
    this.simulations[serviceId].forceStatus = statusType;
    this.simulations[serviceId].errorRate = 1.0;

    const s = this.services.find(item => item.id === serviceId);
    if (s) {
      s.status = statusType;
      if (statusType === 'DOWN') s.latency = 0;
    }

    this.logs.unshift({
      id: `log_sim_${Date.now()}`,
      timestamp: new Date().toISOString(),
      level: 'ERROR',
      service: serviceId,
      method: 'SIMULATION',
      path: '/chaos/failure',
      status: 500,
      latency: 0,
      requestId: 'sim_injection',
      message: `[SIMULATION APPLIED] Triggered forced ${statusType} failure on ${serviceId} service`
    });

    this.emit('services', this.services);
    this.emit('simulations', this.simulations);
    this.emit('logs', this.logs);
  }

  restoreService(serviceId) {
    if (this.simulations[serviceId]) {
      this.simulations[serviceId].latencyAdd = 0;
      this.simulations[serviceId].forceStatus = null;
      this.simulations[serviceId].errorRate = 0.005;
    }

    const s = this.services.find(item => item.id === serviceId);
    if (s) {
      s.status = 'UP';
      s.latency = serviceId === 'events' ? 38 : serviceId === 'orders' ? 26 : 24;
    }

    this.logs.unshift({
      id: `log_sim_${Date.now()}`,
      timestamp: new Date().toISOString(),
      level: 'INFO',
      service: serviceId,
      method: 'SIMULATION',
      path: '/chaos/restore',
      status: 200,
      latency: 15,
      requestId: 'sim_injection',
      message: `[SERVICE RESTORED] Restored ${serviceId} service to UP (Operational)`
    });

    this.emit('services', this.services);
    this.emit('simulations', this.simulations);
    this.emit('logs', this.logs);
  }

  restoreAllServices() {
    this.services.forEach(s => {
      this.restoreService(s.id);
    });
  }

  triggerChaosPreset() {
    // Randomize states for demonstration: 1 DOWN, 1 SLOW, 2 UP
    const ids = ['users', 'events', 'orders', 'payments'];
    const downTarget = ids[Math.floor(Math.random() * ids.length)];
    const remaining = ids.filter(id => id !== downTarget);
    const slowTarget = remaining[Math.floor(Math.random() * remaining.length)];
    const upTargets = remaining.filter(id => id !== slowTarget);

    this.simulateFailure(downTarget, 'DOWN');
    this.injectLatency(slowTarget, 850);
    upTargets.forEach(id => this.restoreService(id));
  }

  // Filter setters
  setSelectedService(serviceId) {
    this.selectedService = serviceId;
    this.emit('filters', { service: serviceId });
  }

  setSelectedMethod(method) {
    this.selectedMethod = method;
    this.emit('filters', { method });
  }

  setSelectedStatusCategory(cat) {
    this.selectedStatusCategory = cat;
    this.emit('filters', { statusCategory: cat });
  }

  setSelectedLatencyFilter(range) {
    this.selectedLatencyFilter = range;
    this.emit('filters', { latencyFilter: range });
  }

  setSortOrder(order) {
    this.sortOrder = order;
    this.emit('filters', { sortOrder: order });
  }

  setSearchQuery(q) {
    this.searchQuery = q.toLowerCase();
    this.emit('filters', { searchQuery: this.searchQuery });
  }

  setLogLevelFilter(level) {
    this.logLevelFilter = level;
    this.emit('logFilters', { level });
  }

  setLogSearchQuery(q) {
    this.logSearchQuery = q.toLowerCase();
    this.emit('logFilters', { searchQuery: this.logSearchQuery });
  }

  setLogErrorOnly(bool) {
    this.logErrorOnly = bool;
    this.emit('logFilters', { errorOnly: bool });
  }

  setMetricsTimeRange(range) {
    this.metricsTimeRange = range;
    this.emit('metrics', this.metricsHistory);
  }

  setHoveredService(serviceId) {
    this.hoveredServiceId = serviceId;
    this.emit('hoveredService', { serviceId });
  }

  clearLogs() {
    this.logs = [];
    this.emit('logs', this.logs);
  }

  manualRefresh() {
    this.tick();
    this.emit('refresh', { timestamp: new Date() });
  }
}

export const state = new StateStore();
