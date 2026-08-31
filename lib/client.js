/**
 * MiniEdge - API Client & Data Access Layer
 */

class ApiClient {
  constructor() {
    this.mode = 'mock'; // 'mock' | 'real'
    this.baseUrl = 'http://localhost:8080';
  }

  setMode(mode) {
    this.mode = mode;
  }

  setBaseUrl(url) {
    this.baseUrl = url.replace(/\/$/, '');
  }

  // GET /api/services
  async getServices() {
    if (this.mode === 'real') {
      const res = await fetch(`${this.baseUrl}/api/services`);
      if (!res.ok) throw new Error(`HTTP error ${res.status}`);
      return await res.json();
    }
    return null;
  }

  // GET /api/services/:id
  async getServiceById(id) {
    if (this.mode === 'real') {
      const res = await fetch(`${this.baseUrl}/api/services/${id}`);
      if (!res.ok) throw new Error(`HTTP error ${res.status}`);
      return await res.json();
    }
    return null;
  }

  // GET /api/metrics
  async getMetrics() {
    if (this.mode === 'real') {
      const res = await fetch(`${this.baseUrl}/api/metrics`);
      if (!res.ok) throw new Error(`HTTP error ${res.status}`);
      return await res.json();
    }
    return null;
  }

  // GET /api/logs
  async getLogs(filter = {}) {
    if (this.mode === 'real') {
      const query = new URLSearchParams(filter).toString();
      const res = await fetch(`${this.baseUrl}/api/logs?${query}`);
      if (!res.ok) throw new Error(`HTTP error ${res.status}`);
      return await res.json();
    }
    return null;
  }

  // GET /api/service-map
  async getServiceMap() {
    if (this.mode === 'real') {
      const res = await fetch(`${this.baseUrl}/api/service-map`);
      if (!res.ok) throw new Error(`HTTP error ${res.status}`);
      return await res.json();
    }
    return null;
  }

  // GET /api/health
  async getHealth() {
    if (this.mode === 'real') {
      const res = await fetch(`${this.baseUrl}/api/health`);
      if (!res.ok) throw new Error(`HTTP error ${res.status}`);
      return await res.json();
    }
    return { status: 'healthy', timestamp: new Date().toISOString() };
  }

  // POST /api/simulations
  async applySimulation(simulationData) {
    if (this.mode === 'real') {
      const res = await fetch(`${this.baseUrl}/api/simulations`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(simulationData)
      });
      if (!res.ok) throw new Error(`HTTP error ${res.status}`);
      return await res.json();
    }
    return { success: true, simulated: true, applied: simulationData };
  }
}

export const apiClient = new ApiClient();
