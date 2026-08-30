/**
 * MiniEdge Utilities - Formatters
 * Zero-dependency helper functions
 */

export function formatMs(ms) {
  if (ms === undefined || ms === null || isNaN(ms)) return '0ms';
  if (ms < 1) return `${(ms * 1000).toFixed(0)}µs`;
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

export function formatNumber(num) {
  if (num === undefined || num === null || isNaN(num)) return '0';
  return new Intl.NumberFormat('en-US').format(num);
}

export function formatTime(timestamp) {
  const date = timestamp instanceof Date ? timestamp : new Date(timestamp);
  return date.toTimeString().split(' ')[0] + '.' + String(date.getMilliseconds()).padStart(3, '0');
}

export function formatRelativeTime(timestamp) {
  const date = timestamp instanceof Date ? timestamp : new Date(timestamp);
  const now = new Date();
  const diffSec = Math.floor((now - date) / 1000);
  if (diffSec < 5) return 'just now';
  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  return `${Math.floor(diffMin / 60)}h ago`;
}

export function getStatusCategory(status) {
  const code = Number(status);
  if (code >= 200 && code < 300) return 'success';
  if (code >= 300 && code < 400) return 'redirect';
  if (code >= 400 && code < 500) return 'client-error';
  if (code >= 500) return 'server-error';
  return 'unknown';
}

export function getStatusClass(status) {
  const cat = getStatusCategory(status);
  switch (cat) {
    case 'success': return 'status-2xx';
    case 'redirect': return 'status-3xx';
    case 'client-error': return 'status-4xx';
    case 'server-error': return 'status-5xx';
    default: return 'status-unknown';
  }
}

export function getServiceHealthClass(health) {
  switch (health?.toUpperCase()) {
    case 'UP': return 'health-up';
    case 'SLOW': return 'health-slow';
    case 'DOWN': return 'health-down';
    default: return 'health-unknown';
  }
}

export function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}
