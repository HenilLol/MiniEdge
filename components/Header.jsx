"use client";

import React, { useState, useEffect } from 'react';
import { state } from '../lib/state.js';
import { Icon } from '../utils/svgIcons.jsx';
import { formatTime } from '../utils/formatters.js';

export function Header() {
  const [statusInfo, setStatusInfo] = useState(state.getSystemStatus());
  const [isRunning, setIsRunning] = useState(state.isSimulationRunning);
  const [lastUpdated, setLastUpdated] = useState(state.lastUpdated);
  const [servicesCount, setServicesCount] = useState(state.services.length);
  const [requestsCount, setRequestsCount] = useState(state.requests.length);
  const [logsCount, setLogsCount] = useState(state.logs.length);
  const [activeOverrides, setActiveOverrides] = useState(0);
  const [activeTab, setActiveTab] = useState('overview-metrics-container');

  useEffect(() => {
    const updateCounts = () => {
      setStatusInfo(state.getSystemStatus());
      setIsRunning(state.isSimulationRunning);
      setLastUpdated(new Date(state.lastUpdated));
      setServicesCount(state.services.length);
      setRequestsCount(state.requests.length);
      setLogsCount(state.logs.length);
      const overrides = Object.values(state.simulations).filter(s => s && (s.latencyAdd > 0 || s.forceStatus !== null)).length;
      setActiveOverrides(overrides);
    };

    updateCounts();

    const unsubTick = state.subscribe('tick', updateCounts);
    const unsubSim = state.subscribe('simulationStateChange', updateCounts);
    const unsubServices = state.subscribe('services', updateCounts);
    const unsubSimulations = state.subscribe('simulations', updateCounts);
    const unsubRequests = state.subscribe('requests', updateCounts);
    const unsubLogs = state.subscribe('logs', updateCounts);

    return () => {
      unsubTick();
      unsubSim();
      unsubServices();
      unsubSimulations();
      unsubRequests();
      unsubLogs();
    };
  }, []);

  const handleToggleStream = () => {
    state.toggleSimulation();
  };

  const handleManualRefresh = () => {
    state.manualRefresh();
  };

  const handleTabClick = (e, targetId) => {
    setActiveTab(targetId);
    const el = document.getElementById(targetId);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth' });
    }
  };

  return (
    <>
      <header className="top-nav">
        <div className="nav-left">
          <div className="brand-logo">
            <div className="logo-icon-wrap">
              <Icon name="zap" className="logo-icon" />
            </div>
            <div className="brand-text">
              <div className="brand-title">
                <span className="brand-name">MiniEdge</span>
                <span className="brand-badge">v1.0.0</span>
                <span className="env-badge" title="Local Gateway Runtime">LOCAL DEV</span>
              </div>
              <p className="brand-subtitle">Developer Command Center</p>
            </div>
          </div>
        </div>

        <div className="nav-center">
          <div className="gateway-status-tag" title="MiniEdge Local Reverse Proxy (0.8ms overhead)">
            <span className="gateway-ping-dot"></span>
            <span>127.0.0.1:8080</span>
          </div>

          <div className={`system-status-indicator status-${statusInfo.color}`} id="header-status-badge" title="System Health State">
            <span className="status-dot"></span>
            <span className="status-text">{statusInfo.text}</span>
          </div>
        </div>

        <div className="nav-right">
          <div className="engine-mode-pill" title="Self-contained zero-dependency mock simulation engine">
            <span className="engine-dot"></span>
            <span>Mock Engine</span>
          </div>

          <div className="header-actions">
            <button className="btn btn-secondary btn-sm" id="btn-toggle-stream" onClick={handleToggleStream} title="Pause/Resume Live Simulation [Space]">
              <span id="stream-icon">
                <Icon name={isRunning ? 'pause' : 'play'} />
              </span>
              <span id="stream-label">{isRunning ? 'Live' : 'Paused'}</span>
              <span className="kbd-hint">Space</span>
            </button>

            <button className="btn btn-primary btn-sm" id="btn-manual-refresh" onClick={handleManualRefresh} title="Manually trigger immediate tick [R]">
              <Icon name="refreshCw" className="refresh-icon icon" />
              <span>Refresh</span>
              <span className="kbd-hint" style={{ color: '#bae6fd', borderColor: '#0284c7' }}>R</span>
            </button>
          </div>

          <div className="last-sync" id="last-sync-time" title="Last background poll timestamp">
            {formatTime(lastUpdated)}
          </div>
        </div>
      </header>

      {/* Developer Section Navigation Tabs */}
      <nav className="section-nav-bar" aria-label="Dashboard Sections">
        <div className="nav-tabs-list">
          <a
            href="#overview-metrics-container"
            className={`nav-tab-item ${activeTab === 'overview-metrics-container' ? 'active' : ''}`}
            onClick={(e) => handleTabClick(e, 'overview-metrics-container')}
          >
            <Icon name="activity" />
            <span>Overview</span>
          </a>
          <a
            href="#service-cards-container"
            className={`nav-tab-item ${activeTab === 'service-cards-container' ? 'active' : ''}`}
            onClick={(e) => handleTabClick(e, 'service-cards-container')}
          >
            <Icon name="server" />
            <span>Services</span>
            <span className="tab-badge">{servicesCount} Nodes</span>
          </a>
          <a
            href="#service-map-container"
            className={`nav-tab-item ${activeTab === 'service-map-container' ? 'active' : ''}`}
            onClick={(e) => handleTabClick(e, 'service-map-container')}
          >
            <Icon name="layers" />
            <span>Service Map</span>
          </a>
          <a
            href="#metrics-charts-container"
            className={`nav-tab-item ${activeTab === 'metrics-charts-container' ? 'active' : ''}`}
            onClick={(e) => handleTabClick(e, 'metrics-charts-container')}
          >
            <Icon name="activity" />
            <span>Metrics</span>
          </a>
          <a
            href="#request-explorer-container"
            className={`nav-tab-item ${activeTab === 'request-explorer-container' ? 'active' : ''}`}
            onClick={(e) => handleTabClick(e, 'request-explorer-container')}
          >
            <Icon name="search" />
            <span>Requests</span>
            <span className="tab-badge" id="nav-req-count">{requestsCount}</span>
          </a>
          <a
            href="#log-viewer-container"
            className={`nav-tab-item ${activeTab === 'log-viewer-container' ? 'active' : ''}`}
            onClick={(e) => handleTabClick(e, 'log-viewer-container')}
          >
            <Icon name="terminal" />
            <span>Logs</span>
            <span className="tab-badge" id="nav-log-count">{logsCount}</span>
          </a>
          <a
            href="#simulation-control-container"
            className={`nav-tab-item ${activeTab === 'simulation-control-container' ? 'active' : ''}`}
            onClick={(e) => handleTabClick(e, 'simulation-control-container')}
          >
            <Icon name="sliders" />
            <span>Failure Simulation</span>
            {activeOverrides > 0 && (
              <span className="tab-badge" style={{ color: '#fbbf24', borderColor: 'rgba(245,158,11,0.3)' }}>
                {activeOverrides} Active
              </span>
            )}
          </a>
        </div>
      </nav>
    </>
  );
}
