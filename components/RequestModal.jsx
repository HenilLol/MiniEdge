"use client";

import React, { useEffect } from 'react';
import { state } from '../lib/state.js';
import { formatMs, formatTime, getStatusClass } from '../utils/formatters.js';

export function RequestModal({ request, onClose }) {
  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  if (!request) return null;

  const statusClass = getStatusClass(request.status);
  const serviceObj = state.services.find(s => s.id === request.service);

  return (
    <div className="modal-backdrop" id="modal-backdrop" onClick={(e) => { if (e.target.id === 'modal-backdrop') onClose(); }}>
      <div className="modal-card">
        <div className="modal-header">
          <div className="modal-title-wrap">
            <span className={`method-badge method-${request.method.toLowerCase()}`}>{request.method}</span>
            <span className="modal-path">{request.path}</span>
            <span className={`status-pill ${statusClass}`}>{request.status}</span>
          </div>
          <button className="modal-close-btn" id="btn-close-modal" onClick={onClose} aria-label="Close modal">×</button>
        </div>

        <div className="modal-body">
          {/* Timing & Routing summary */}
          <div className="modal-section">
            <h4 className="modal-section-title">Routing & Timing Breakdown</h4>
            <div className="modal-grid-3">
              <div className="info-box">
                <span className="info-label">Upstream Node</span>
                <span className="info-value font-mono">{request.serviceName} (:{serviceObj?.port || 3000})</span>
              </div>
              <div className="info-box">
                <span className="info-label">Total Latency</span>
                <span className={`info-value font-mono ${request.latency > 500 ? 'text-warning' : 'text-success'}`}>{formatMs(request.latency)}</span>
              </div>
              <div className="info-box">
                <span className="info-label">Timestamp</span>
                <span className="info-value font-mono">{formatTime(request.timestamp)}</span>
              </div>
            </div>
          </div>

          {/* Request Headers */}
          <div className="modal-section">
            <h4 className="modal-section-title">Gateway Inbound Headers</h4>
            <div className="code-block">
              <pre><code>{JSON.stringify(request.headers, null, 2)}</code></pre>
            </div>
          </div>

          {/* Response Body */}
          <div className="modal-section">
            <h4 className="modal-section-title">Upstream Response Payload</h4>
            <div className="code-block">
              <pre><code>{JSON.stringify(request.responseBody, null, 2)}</code></pre>
            </div>
          </div>
        </div>

        <div className="modal-footer">
          <button className="btn btn-secondary btn-sm" id="btn-modal-close-footer" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  );
}
