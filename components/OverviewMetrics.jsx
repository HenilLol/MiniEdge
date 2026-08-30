"use client";

import React, { useState, useEffect } from 'react';
import { state } from '../lib/state.js';
import { Icon } from '../utils/svgIcons.jsx';
import { formatNumber, formatMs } from '../utils/formatters.js';

export function OverviewMetrics() {
  const [metrics, setMetrics] = useState(state.getOverviewMetrics());
  const [statusInfo, setStatusInfo] = useState(state.getSystemStatus());

  useEffect(() => {
    const updateMetrics = () => {
      setMetrics(state.getOverviewMetrics());
      setStatusInfo(state.getSystemStatus());
    };

    const unsubTick = state.subscribe('tick', updateMetrics);
    const unsubServices = state.subscribe('services', updateMetrics);

    return () => {
      unsubTick();
      unsubServices();
    };
  }, []);

  const healthPercent = Math.round((metrics.healthyServices / metrics.totalServices) * 100);

  return (
    <>
      {/* 1. Command Center Quick-Glance Status Strip */}
      <div className="mission-status-strip" aria-label="Command Center Quick Status">
        <div className="status-strip-item">
          <span className="strip-label">SYSTEM</span>
          <div className="strip-val-wrap">
            <span className={`strip-dot status-${statusInfo.color}`}></span>
            <span className={`strip-val font-mono ${statusInfo.color === 'up' ? 'text-success' : statusInfo.color === 'slow' ? 'text-warning' : 'text-danger'}`}>
              {statusInfo.level}
            </span>
          </div>
        </div>

        <div className="status-strip-divider"></div>

        <div className="status-strip-item">
          <span className="strip-label">GATEWAY</span>
          <div className="strip-val-wrap">
            <span className="strip-val font-mono text-info">:8080 Proxy</span>
            <span className="strip-sub">0.8ms ovh</span>
          </div>
        </div>

        <div className="status-strip-divider"></div>

        <div className="status-strip-item">
          <span className="strip-label">SERVICES</span>
          <div className="strip-val-wrap">
            <span className="strip-val font-mono">{metrics.totalServices} Nodes</span>
            <span className="strip-sub">({metrics.healthyServices} UP, {metrics.slowServices} SLOW, {metrics.downServices} DOWN)</span>
          </div>
        </div>

        <div className="status-strip-divider"></div>

        <div className="status-strip-item">
          <span className="strip-label">TRAFFIC</span>
          <div className="strip-val-wrap">
            <span className="strip-val font-mono text-info">{formatNumber(metrics.totalRequests)}</span>
            <span className="strip-sub">~40 req/s</span>
          </div>
        </div>

        <div className="status-strip-divider"></div>

        <div className="status-strip-item">
          <span className="strip-label">ERRORS</span>
          <div className="strip-val-wrap">
            <span className={`strip-val font-mono ${Number(metrics.errorRate) > 3 ? 'text-danger' : Number(metrics.errorRate) > 0 ? 'text-warning' : 'text-success'}`}>
              {metrics.errorRate}%
            </span>
            <span className="strip-sub">({metrics.totalErrors} errs)</span>
          </div>
        </div>

        <div className="status-strip-divider"></div>

        <div className="status-strip-item">
          <span className="strip-label">LATENCY</span>
          <div className="strip-val-wrap">
            <span className={`strip-val font-mono ${metrics.avgLatency > 300 ? 'text-warning' : 'text-success'}`}>
              {formatMs(metrics.avgLatency)}
            </span>
            <span className="strip-sub">rolling avg</span>
          </div>
        </div>
      </div>

      {/* 2. Overview Metrics Grid */}
      <div className="metrics-grid">
        {/* Card 1: Total Services */}
        <div className="metric-card">
          <div className="metric-header">
            <span className="metric-title">Total Services</span>
            <div className="metric-icon-box info">
              <Icon name="server" />
            </div>
          </div>
          <div className="metric-body">
            <div className="metric-value-row">
              <span className="metric-value font-mono">{metrics.totalServices}</span>
              <span className="metric-unit">Nodes</span>
            </div>
            <div className="metric-subtext">
              <span className="subtext-pill up">{metrics.healthyServices} UP</span>
              {metrics.slowServices > 0 && <span className="subtext-pill slow">{metrics.slowServices} SLOW</span>}
              {metrics.downServices > 0 && <span className="subtext-pill down">{metrics.downServices} DOWN</span>}
            </div>
          </div>
        </div>

        {/* Card 2: Healthy Services */}
        <div className="metric-card">
          <div className="metric-header">
            <span className="metric-title">Health Ratio</span>
            <div className={`metric-icon-box ${healthPercent === 100 ? 'success' : healthPercent >= 75 ? 'warning' : 'danger'}`}>
              <Icon name="checkCircle" />
            </div>
          </div>
          <div className="metric-body">
            <div className="metric-value-row">
              <span className={`metric-value font-mono ${healthPercent < 100 ? (healthPercent >= 75 ? 'text-warning' : 'text-danger') : 'text-success'}`}>
                {metrics.healthyServices}/{metrics.totalServices}
              </span>
              <span className="metric-unit">({healthPercent}%)</span>
            </div>
            <div className="metric-progress-bar">
              <div
                className={`progress-fill ${healthPercent === 100 ? 'bg-success' : healthPercent >= 75 ? 'bg-warning' : 'bg-danger'}`}
                style={{ width: `${healthPercent}%` }}
              ></div>
            </div>
          </div>
        </div>

        {/* Card 3: Total Requests */}
        <div className="metric-card">
          <div className="metric-header">
            <span className="metric-title">Total Requests</span>
            <div className="metric-icon-box info">
              <Icon name="activity" />
            </div>
          </div>
          <div className="metric-body">
            <div className="metric-value-row">
              <span className="metric-value font-mono">{formatNumber(metrics.totalRequests)}</span>
              <span className="metric-unit">reqs</span>
            </div>
            <div className="metric-subtext text-muted font-mono">
              <span>~35-50 req/s live rate</span>
            </div>
          </div>
        </div>

        {/* Card 4: Error Rate */}
        <div className="metric-card">
          <div className="metric-header">
            <span className="metric-title">Errors & Rate</span>
            <div className={`metric-icon-box ${Number(metrics.errorRate) > 5 ? 'danger' : Number(metrics.errorRate) > 1 ? 'warning' : 'success'}`}>
              <Icon name="alertTriangle" />
            </div>
          </div>
          <div className="metric-body">
            <div className="metric-value-row">
              <span className={`metric-value font-mono ${Number(metrics.errorRate) > 3 ? 'text-danger' : Number(metrics.errorRate) > 1 ? 'text-warning' : 'text-success'}`}>
                {metrics.errorRate}%
              </span>
              <span className="metric-unit">({formatNumber(metrics.totalErrors)} errs)</span>
            </div>
            <div className="metric-subtext text-muted font-mono">
              <span>{metrics.downServices > 0 ? 'Elevated (outage active)' : 'Nominal threshold'}</span>
            </div>
          </div>
        </div>

        {/* Card 5: Average Latency */}
        <div className="metric-card">
          <div className="metric-header">
            <span className="metric-title">Average Latency</span>
            <div className={`metric-icon-box ${metrics.avgLatency > 300 ? 'warning' : 'success'}`}>
              <Icon name="clock" />
            </div>
          </div>
          <div className="metric-body">
            <div className="metric-value-row">
              <span className={`metric-value font-mono ${metrics.avgLatency > 300 ? 'text-warning' : 'text-success'}`}>
                {formatMs(metrics.avgLatency)}
              </span>
              <span className="metric-unit">rolling</span>
            </div>
            <div className="metric-subtext text-muted font-mono">
              <span>p50: ~30ms • p95: ~{metrics.slowServices > 0 ? '820ms' : '65ms'}</span>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
