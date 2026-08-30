/**
 * MiniEdge - Mock Data Engine & Traffic Simulator
 * Provides realistic microservice seed data and dynamic background traffic generation.
 */

// Initial microservices topology & status
export const initialServices = [
  {
    id: 'users',
    name: 'User Service',
    port: 3001,
    status: 'UP', // UP, SLOW, DOWN
    latency: 24,
    requestCount: 4120,
    errorCount: 12,
    uptimePercent: 99.98,
    version: 'v1.4.2',
    endpoints: [
      { method: 'GET', path: '/users' },
      { method: 'GET', path: '/users/:id' },
      { method: 'POST', path: '/users/auth' },
      { method: 'PUT', path: '/users/profile' }
    ]
  },
  {
    id: 'events',
    name: 'Event Service',
    port: 3002,
    status: 'SLOW',
    latency: 820,
    requestCount: 18450,
    errorCount: 384,
    uptimePercent: 97.40,
    version: 'v2.1.0',
    endpoints: [
      { method: 'GET', path: '/events' },
      { method: 'POST', path: '/events/publish' },
      { method: 'GET', path: '/events/stream' },
      { method: 'GET', path: '/events/analytics' }
    ]
  },
  {
    id: 'orders',
    name: 'Order Service',
    port: 3003,
    status: 'DOWN',
    latency: 0,
    requestCount: 3200,
    errorCount: 890,
    uptimePercent: 78.10,
    version: 'v1.0.8',
    endpoints: [
      { method: 'GET', path: '/orders' },
      { method: 'POST', path: '/orders/checkout' },
      { method: 'POST', path: '/orders/create' },
      { method: 'GET', path: '/orders/:id' }
    ]
  },
  {
    id: 'payments',
    name: 'Payment Service',
    port: 3004,
    status: 'UP',
    latency: 31,
    requestCount: 5690,
    errorCount: 18,
    uptimePercent: 99.85,
    version: 'v1.8.0',
    endpoints: [
      { method: 'POST', path: '/payments/charge' },
      { method: 'POST', path: '/payments/intent' },
      { method: 'GET', path: '/payments/webhook' },
      { method: 'POST', path: '/payments/refund' }
    ]
  }
];

// Active failure simulation overrides
export const activeSimulations = {
  users: { latencyAdd: 0, forceStatus: null, errorRate: 0.005 },
  events: { latencyAdd: 750, forceStatus: 'SLOW', errorRate: 0.08 },
  orders: { latencyAdd: 0, forceStatus: 'DOWN', errorRate: 1.0 },
  payments: { latencyAdd: 0, forceStatus: null, errorRate: 0.003 }
};

// Seed initial historical requests
export function generateSeedRequests() {
  const requests = [];
  const now = Date.now();
  const samplePaths = [
    { service: 'users', method: 'GET', path: '/users', status: 200, latency: 22 },
    { service: 'users', method: 'GET', path: '/users/usr_981', status: 200, latency: 26 },
    { service: 'events', method: 'GET', path: '/events', status: 200, latency: 810 },
    { service: 'events', method: 'POST', path: '/events/publish', status: 200, latency: 845 },
    { service: 'payments', method: 'POST', path: '/payments/charge', status: 200, latency: 32 },
    { service: 'orders', method: 'POST', path: '/orders/checkout', status: 500, latency: 5 },
    { service: 'users', method: 'POST', path: '/users/auth', status: 200, latency: 28 },
    { service: 'payments', method: 'POST', path: '/payments/intent', status: 200, latency: 30 },
    { service: 'orders', method: 'GET', path: '/orders', status: 503, latency: 2 },
    { service: 'events', method: 'GET', path: '/events/stream', status: 200, latency: 805 },
    { service: 'users', method: 'PUT', path: '/users/profile', status: 200, latency: 25 },
    { service: 'orders', method: 'POST', path: '/orders/create', status: 500, latency: 8 },
    { service: 'payments', method: 'POST', path: '/payments/refund', status: 200, latency: 35 },
    { service: 'events', method: 'GET', path: '/events/analytics', status: 200, latency: 830 },
    { service: 'users', method: 'GET', path: '/users', status: 200, latency: 21 }
  ];

  for (let i = samplePaths.length - 1; i >= 0; i--) {
    const item = samplePaths[i];
    const timeOffset = (samplePaths.length - i) * 1800; // spread backwards in time
    requests.push({
      id: `req_${Math.random().toString(36).substring(2, 9)}`,
      timestamp: new Date(now - timeOffset).toISOString(),
      service: item.service,
      serviceName: initialServices.find(s => s.id === item.service)?.name || item.service,
      method: item.method,
      path: item.path,
      status: item.status,
      latency: item.latency,
      ip: `127.0.0.1`,
      client: `curl/8.4.0`,
      headers: {
        'host': `localhost:${initialServices.find(s => s.id === item.service)?.port || 3000}`,
        'x-request-id': `req_${Math.random().toString(36).substring(2, 9)}`,
        'user-agent': 'MiniEdge-Gateway/1.0',
        'content-type': 'application/json'
      },
      responseBody: item.status >= 400 
        ? { error: item.status === 503 ? 'Service Unavailable - Connection Refused' : 'Internal Server Error' } 
        : { status: 'ok', data: { service: item.service, processed: true } }
    });
  }
  return requests;
}

// Seed initial logs
export function generateSeedLogs() {
  const logs = [];
  const now = Date.now();
  const seedEntries = [
    { level: 'INFO', service: 'gateway', method: 'SYSTEM', path: '/boot', status: 200, latency: 1, message: 'MiniEdge Gateway v1.0.0 listening on http://127.0.0.1:8080' },
    { level: 'INFO', service: 'users', method: 'GET', path: '/users', status: 200, latency: 22, message: 'Dispatched proxy upstream -> :3001 completed in 22ms' },
    { level: 'WARN', service: 'events', method: 'GET', path: '/events', status: 200, latency: 810, message: 'High upstream response latency detected (>500ms threshold)' },
    { level: 'ERROR', service: 'orders', method: 'POST', path: '/orders/checkout', status: 500, latency: 5, message: 'Upstream connection reset by peer on port :3003' },
    { level: 'INFO', service: 'payments', method: 'POST', path: '/payments/charge', status: 200, latency: 32, message: 'Payment intent created successfully via :3004' },
    { level: 'ERROR', service: 'orders', method: 'GET', path: '/orders', status: 503, latency: 2, message: 'Circuit Breaker OPEN: Order Service unreachable' },
    { level: 'INFO', service: 'users', method: 'POST', path: '/users/auth', status: 200, latency: 28, message: 'Token authenticated successfully' },
    { level: 'WARN', service: 'events', method: 'POST', path: '/events/publish', status: 200, latency: 845, message: 'Kafka event broker ack delayed in upstream cluster' },
    { level: 'INFO', service: 'payments', method: 'POST', path: '/payments/intent', status: 200, latency: 30, message: 'Webhook registered for stripe charge session' },
    { level: 'ERROR', service: 'orders', method: 'POST', path: '/orders/create', status: 500, latency: 8, message: 'Dial tcp 127.0.0.1:3003: connect: connection refused' }
  ];

  seedEntries.forEach((entry, idx) => {
    logs.push({
      id: `log_${Math.random().toString(36).substring(2, 9)}`,
      timestamp: new Date(now - (seedEntries.length - idx) * 2200).toISOString(),
      level: entry.level,
      service: entry.service,
      method: entry.method,
      path: entry.path,
      status: entry.status,
      latency: entry.latency,
      requestId: `req_${Math.random().toString(36).substring(2, 7)}`,
      message: entry.message
    });
  });

  return logs;
}

// Initial time-series metrics buffers (last 24 data points)
export function generateSeedMetricsHistory() {
  const points = 24;
  const history = {
    timestamps: [],
    throughput: [], // total requests / sec
    errors: [],     // error count / sec
    latencyP50: [], // ms
    latencyP95: []  // ms
  };

  const now = Date.now();
  for (let i = points - 1; i >= 0; i--) {
    const t = new Date(now - i * 3000);
    history.timestamps.push(t.toLocaleTimeString([], { hour12: false }));
    // Base values with realistic noise
    const reqs = Math.floor(25 + Math.random() * 20);
    const errs = Math.floor(Math.random() < 0.7 ? (1 + Math.random() * 3) : 0);
    const p50 = Math.round(35 + Math.random() * 15);
    const p95 = Math.round(220 + Math.random() * 180);

    history.throughput.push(reqs);
    history.errors.push(errs);
    history.latencyP50.push(p50);
    history.latencyP95.push(p95);
  }

  return history;
}

// Helper to simulate a single new dynamic request based on active service states & simulations
export function generateSimulatedRequest(services, simulations) {
  // Choose random service weighted by typical volume
  const serviceKeys = ['users', 'events', 'orders', 'payments'];
  const targetId = serviceKeys[Math.floor(Math.random() * serviceKeys.length)];
  const service = services.find(s => s.id === targetId) || services[0];
  const sim = simulations[targetId] || { latencyAdd: 0, forceStatus: null, errorRate: 0 };

  const endpoint = service.endpoints[Math.floor(Math.random() * service.endpoints.length)];
  let path = endpoint.path;
  if (path.includes(':id')) {
    path = path.replace(':id', Math.floor(100 + Math.random() * 900));
  }

  // Determine status & latency based on service state and simulation overrides
  let status = 200;
  let latency = service.latency;
  const reqId = `req_${Math.random().toString(36).substring(2, 9)}`;

  // Status computation
  const isDown = sim.forceStatus === 'DOWN' || (sim.forceStatus === null && service.status === 'DOWN');
  const isSlow = sim.forceStatus === 'SLOW' || (sim.forceStatus === null && service.status === 'SLOW');

  if (isDown) {
    status = Math.random() > 0.5 ? 503 : 500;
    latency = Math.round(2 + Math.random() * 10);
  } else if (Math.random() < sim.errorRate) {
    status = Math.random() > 0.6 ? 500 : 400;
    latency = Math.round((service.latency || 25) + (Math.random() * 40));
  } else if (endpoint.method === 'POST') {
    status = 201;
  }

  // Latency computation
  if (!isDown) {
    let baseLatency = 20 + Math.random() * 15;
    if (isSlow) baseLatency = 780 + Math.random() * 150;
    latency = Math.round(baseLatency + (sim.latencyAdd || 0));
  }

  const reqObj = {
    id: reqId,
    timestamp: new Date().toISOString(),
    service: service.id,
    serviceName: service.name,
    method: endpoint.method,
    path: path,
    status: status,
    latency: latency,
    ip: '127.0.0.1',
    client: 'MiniEdge-Gateway',
    headers: {
      'host': `localhost:${service.port}`,
      'x-request-id': reqId,
      'x-forwarded-for': '127.0.0.1',
      'user-agent': 'MiniEdge-Gateway/1.0',
      'content-type': 'application/json'
    },
    responseBody: status >= 500
      ? { error: status === 503 ? 'Service Unavailable' : 'Internal Gateway Upstream Error', service: service.name }
      : { success: true, timestamp: Date.now(), data: { service: service.id, status: 'acknowledged' } }
  };

  // Generate corresponding log entry
  let logLevel = 'INFO';
  let logMessage = `Request ${endpoint.method} ${path} -> :${service.port} returned ${status} in ${latency}ms`;

  if (status >= 500) {
    logLevel = 'ERROR';
    logMessage = `Upstream error on :${service.port} [${service.name}] -> HTTP ${status} (Latency: ${latency}ms)`;
  } else if (status >= 400) {
    logLevel = 'WARN';
    logMessage = `Client error ${status} on ${endpoint.method} ${path}`;
  } else if (latency > 500) {
    logLevel = 'WARN';
    logMessage = `Slow upstream response on :${service.port} [${service.name}] (${latency}ms)`;
  }

  const logObj = {
    id: `log_${Math.random().toString(36).substring(2, 9)}`,
    timestamp: new Date().toISOString(),
    level: logLevel,
    service: service.id,
    method: endpoint.method,
    path: path,
    status: status,
    latency: latency,
    requestId: reqId,
    message: logMessage
  };

  return { request: reqObj, log: logObj, targetServiceId: service.id, isError: status >= 400, latency };
}
