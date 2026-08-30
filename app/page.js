"use client";

import React, { useState, useEffect, useCallback } from 'react';
import { state } from '../lib/state.js';
import { Header } from '../components/Header.jsx';
import { OverviewMetrics } from '../components/OverviewMetrics.jsx';
import { ServiceCards } from '../components/ServiceCards.jsx';
import { ServiceMap } from '../components/ServiceMap.jsx';
import { MetricsCharts } from '../components/MetricsCharts.jsx';
import { RequestExplorer } from '../components/RequestExplorer.jsx';
import { LogViewer } from '../components/LogViewer.jsx';
import { SimulationControl } from '../components/SimulationControl.jsx';
import { ToastContainer } from '../components/ToastContainer.jsx';

export default function DashboardPage() {
  const [toasts, setToasts] = useState([]);

  const showToast = useCallback((message, type = 'info') => {
    const id = `toast_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`;
    setToasts(prev => [...prev, { id, message, type }]);

    setTimeout(() => {
      setToasts(prev => prev.filter(t => t.id !== id));
    }, 2800);
  }, []);

  // Global Keyboard Shortcuts
  useEffect(() => {
    const handleKeyDown = (e) => {
      // Ignore when user is typing in inputs or selects
      const tag = document.activeElement ? document.activeElement.tagName.toLowerCase() : '';
      if (tag === 'input' || tag === 'textarea' || tag === 'select') {
        return;
      }

      if (e.code === 'Space') {
        e.preventDefault();
        state.toggleSimulation();
        showToast(
          state.isSimulationRunning ? 'Resumed live telemetry simulation' : 'Paused live telemetry simulation',
          'info'
        );
      } else if (e.key === 'r' || e.key === 'R') {
        e.preventDefault();
        state.manualRefresh();
        showToast('Immediate telemetry tick executed', 'info');
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [showToast]);

  return (
    <div className="app-wrapper">
      {/* Header Section */}
      <div id="header-container" className="header-container">
        <Header />
      </div>

      {/* Main Dashboard Container */}
      <main className="main-container">
        {/* 1. Overview KPIs */}
        <section id="overview-metrics-container" aria-label="System Overview Metrics">
          <OverviewMetrics />
        </section>

        {/* 2. Microservices Status Cards */}
        <section id="service-cards-container" className="dashboard-section" aria-label="Microservice Nodes">
          <ServiceCards />
        </section>

        {/* 3. Service Dependency Topology Map */}
        <section id="service-map-container" className="dashboard-section" aria-label="Service Dependency Map">
          <ServiceMap />
        </section>

        {/* 4. Real-time Telemetry & Charts */}
        <section id="metrics-charts-container" className="dashboard-section" aria-label="Telemetry and Metrics">
          <MetricsCharts />
        </section>

        {/* 5. Request Explorer Table */}
        <section id="request-explorer-container" className="dashboard-section" aria-label="Request Explorer">
          <RequestExplorer />
        </section>

        {/* 6. Developer Log Viewer Console */}
        <section id="log-viewer-container" className="dashboard-section" aria-label="Developer Log Stream">
          <LogViewer onShowToast={showToast} />
        </section>

        {/* 7. Fault Injection & Simulation Studio */}
        <section id="simulation-control-container" className="dashboard-section" aria-label="Failure Simulation Studio">
          <SimulationControl onShowToast={showToast} />
        </section>
      </main>

      {/* Footer */}
      <footer className="dashboard-footer">
        <div className="footer-content">
          <div className="footer-left">
            <span>MiniEdge Gateway v1.0.0-dev • Built for Hackathon (Zero Runtime Dependencies)</span>
          </div>
          <div className="footer-links">
            <span className="footer-shortcut">Shortcuts: <kbd className="font-mono">[Space]</kbd> Pause/Resume • <kbd className="font-mono">[R]</kbd> Refresh</span>
          </div>
        </div>
      </footer>

      {/* Global Toast Container */}
      <ToastContainer toasts={toasts} />
    </div>
  );
}
