"use client";

import React, { useState, useEffect } from 'react';
import { state } from '../lib/state.js';
import { Icon } from '../utils/svgIcons.jsx';

export function SimulationControl({ onShowToast }) {
  const [services, setServices] = useState([...state.services]);
  const [simulations, setSimulations] = useState({ ...state.simulations });
  const [selectedTargetService, setSelectedTargetService] = useState('users');

  useEffect(() => {
    const handleServices = () => setServices([...state.services]);
    const handleSimulations = () => setSimulations({ ...state.simulations });

    const unsubServices = state.subscribe('services', handleServices);
    const unsubSimulations = state.subscribe('simulations', handleSimulations);

    return () => {
      unsubServices();
      unsubSimulations();
    };
  }, []);

  const handleInjectLatency = (ms) => {
    state.injectLatency(selectedTargetService, ms);
    if (onShowToast) {
      onShowToast(`[SIMULATION] Injected +${ms}ms latency to ${selectedTargetService}`, 'warning');
    }
  };

  const handleTriggerFail = () => {
    state.simulateFailure(selectedTargetService, 'DOWN');
    if (onShowToast) {
      onShowToast(`[SIMULATION] Simulated 500 Outage on ${selectedTargetService}`, 'danger');
    }
  };

  const handleTriggerSlow = () => {
    state.injectLatency(selectedTargetService, 750);
    if (onShowToast) {
      onShowToast(`[SIMULATION] Degraded ${selectedTargetService} to SLOW`, 'warning');
    }
  };

  const handleRestoreSelected = () => {
    state.restoreService(selectedTargetService);
    if (onShowToast) {
      onShowToast(`[RESTORED] ${selectedTargetService} service is now UP`, 'success');
    }
  };

  const handleRestoreItem = (id) => {
    state.restoreService(id);
    if (onShowToast) {
      onShowToast(`[RESTORED] ${id} service is now UP`, 'success');
    }
  };

  const handleResetAllSims = () => {
    state.restoreAllServices();
    if (onShowToast) {
      onShowToast(`[RESTORED] All services restored to Healthy UP`, 'success');
    }
  };

  const handlePreset = (preset) => {
    if (preset === 'baseline') {
      state.restoreAllServices();
      if (onShowToast) onShowToast(`Preset Applied: All Healthy (100% UP)`, 'success');
    } else if (preset === 'default') {
      state.restoreAllServices();
      state.simulateFailure('orders', 'DOWN');
      state.injectLatency('events', 750);
      if (onShowToast) onShowToast(`Preset Applied: Orders DOWN, Events SLOW`, 'warning');
    } else if (preset === 'chaos') {
      state.triggerChaosPreset();
      if (onShowToast) onShowToast(`Preset Applied: Random Chaos Mode`, 'danger');
    }
  };

  return (
    <div className="simulation-panel">
      <div className="simulation-header">
        <div className="section-title-wrap">
          <div className="section-icon text-warning">
            <Icon name="sliders" />
          </div>
          <div>
            <h2 className="section-title">Fault Injection & Chaos Studio</h2>
            <p className="section-desc">Test circuit breakers, timeout resiliency, and alert triggers under simulated upstream degradation</p>
          </div>
        </div>

        <div className="simulation-notice-badge">
          <Icon name="shieldAlert" className="notice-icon icon" />
          <span>LOCAL SIMULATION / INTENTIONAL FAILURE</span>
        </div>
      </div>

      <div className="simulation-body-grid">
        {/* Column 1: Target Selector & Action Builder */}
        <div className="sim-builder-card">
          <h3 className="sim-card-title">1. Target Upstream Service</h3>

          <div className="service-selector-radio">
            {services.map(s => (
              <label
                key={s.id}
                className={`service-radio-label ${selectedTargetService === s.id ? 'active' : ''}`}
              >
                <input
                  type="radio"
                  name="sim-target"
                  value={s.id}
                  checked={selectedTargetService === s.id}
                  onChange={() => setSelectedTargetService(s.id)}
                />
                <span className="radio-service-name">{s.name}</span>
                <span className="radio-service-port font-mono">:{s.port}</span>
                <span className={`radio-status-tag ${s.status.toLowerCase()}`}>{s.status}</span>
              </label>
            ))}
          </div>

          <h3 className="sim-card-title" style={{ marginTop: '14px' }}>2. Inject Fault Scenario</h3>
          <div className="sim-actions-container">
            {/* Latency Actions */}
            <div className="action-group">
              <span className="action-group-label">Inject Artificial Latency:</span>
              <div className="btn-group-row">
                <button className="btn btn-sm btn-outline btn-inject-latency" onClick={() => handleInjectLatency(200)} data-ms="200">+200ms</button>
                <button className="btn btn-sm btn-outline btn-inject-latency" onClick={() => handleInjectLatency(800)} data-ms="800">+800ms</button>
                <button className="btn btn-sm btn-outline btn-inject-latency" onClick={() => handleInjectLatency(2500)} data-ms="2500">+2.5s (Lag)</button>
              </div>
            </div>

            {/* Failure Actions */}
            <div className="action-group">
              <span className="action-group-label">Simulate Failure / Error:</span>
              <div className="btn-group-row">
                <button className="btn btn-sm btn-danger-subtle btn-trigger-fail" onClick={handleTriggerFail} data-type="DOWN">500 Outage / Drop</button>
                <button className="btn btn-sm btn-warning-subtle btn-trigger-slow" onClick={handleTriggerSlow} data-type="SLOW">Degrade to SLOW</button>
              </div>
            </div>

            {/* Restore Action */}
            <div className="action-group" style={{ marginTop: '6px' }}>
              <button className="btn btn-sm btn-success-subtle btn-restore-target" onClick={handleRestoreSelected} style={{ width: '100%' }}>
                <Icon name="checkCircle" />
                <span>Restore Selected Service to Healthy (UP)</span>
              </button>
            </div>
          </div>
        </div>

        {/* Column 2: Active Fault Status & Presets */}
        <div className="sim-status-card">
          <div className="sim-status-header">
            <h3 className="sim-card-title">Active Fault Overrides</h3>
            <button className="btn btn-xs btn-outline" id="btn-reset-all-sims" onClick={handleResetAllSims}>
              Restore All
            </button>
          </div>

          <div className="active-sims-list">
            {services.map(s => {
              const sim = simulations[s.id];
              const hasOverride = sim && (sim.latencyAdd > 0 || sim.forceStatus !== null);

              return (
                <div key={s.id} className={`active-sim-item ${hasOverride ? 'has-override' : ''}`}>
                  <div className="sim-item-info">
                    <span className="sim-item-name">{s.name} (:{s.port})</span>
                    <span className={`sim-item-status status-${s.status.toLowerCase()}`}>{s.status}</span>
                  </div>
                  <div className="sim-item-details">
                    {sim?.latencyAdd > 0 && <span className="sim-tag latency">+{sim.latencyAdd}ms Latency</span>}
                    {sim?.forceStatus && <span className="sim-tag status">Forced {sim.forceStatus}</span>}
                    {!hasOverride && <span className="sim-tag healthy">Healthy</span>}
                  </div>
                  {hasOverride && (
                    <button
                      className="btn btn-xs btn-outline btn-item-restore"
                      data-id={s.id}
                      onClick={() => handleRestoreItem(s.id)}
                      title="Remove override and restore"
                    >
                      Restore
                    </button>
                  )}
                </div>
              );
            })}
          </div>

          {/* Demo Presets */}
          <div className="chaos-presets-wrap">
            <h4 className="preset-title">Demo Presets:</h4>
            <div className="preset-buttons">
              <button className="btn btn-xs btn-outline btn-preset" onClick={() => handlePreset('baseline')} data-preset="baseline">
                All Healthy (100% UP)
              </button>
              <button className="btn btn-xs btn-outline btn-preset" onClick={() => handlePreset('default')} data-preset="default">
                Default Demo (Orders DOWN, Events SLOW)
              </button>
              <button className="btn btn-xs btn-outline btn-preset" onClick={() => handlePreset('chaos')} data-preset="chaos">
                <Icon name="flame" />
                <span>Random Chaos</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
