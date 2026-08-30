"use client";

import React, { useState, useEffect } from 'react';
import { state } from '../lib/state.js';
import { Icon } from '../utils/svgIcons.jsx';
import { formatNumber, formatMs, getServiceHealthClass } from '../utils/formatters.js';

export function ServiceCards() {
  const [services, setServices] = useState([...state.services]);
  const [selectedService, setSelectedService] = useState(state.selectedService);
  const [hoveredServiceId, setHoveredServiceId] = useState(state.hoveredServiceId);
  const [, setTick] = useState(0);

  useEffect(() => {
    const handleServices = () => setServices([...state.services]);
    const handleFilters = () => setSelectedService(state.selectedService);
    const handleHover = ({ serviceId }) => setHoveredServiceId(serviceId);
    const handleTick = () => setTick(t => t + 1);

    const unsubServices = state.subscribe('services', handleServices);
    const unsubFilters = state.subscribe('filters', handleFilters);
    const unsubHover = state.subscribe('hoveredService', handleHover);
    const unsubTick = state.subscribe('tick', handleTick);

    return () => {
      unsubServices();
      unsubFilters();
      unsubHover();
      unsubTick();
    };
  }, []);

  const generateSparkline = (service) => {
    const isDown = service.status === 'DOWN';
    const isSlow = service.status === 'SLOW';
    const color = isDown ? '#ef4444' : isSlow ? '#f59e0b' : '#10b981';
    const baseVal = isDown ? 0 : isSlow ? 80 : 25;

    const points = [];
    for (let i = 0; i < 8; i++) {
      const jitter = isDown ? 0 : Math.sin(i + Date.now() / 3000) * 12 + (i % 2 === 0 ? 5 : -5);
      const val = Math.max(2, baseVal + jitter);
      points.push(val);
    }

    const max = Math.max(...points, 100);
    const w = 120;
    const h = 22;

    const coords = points.map((p, i) => {
      const x = (w / (points.length - 1)) * i;
      const y = isDown ? h - 2 : Math.max(3, h - (p / max) * (h - 6));
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    });

    const polylineStr = coords.join(' ');
    const polygonStr = `0,${h} ${polylineStr} ${w},${h}`;

    return (
      <div className="service-sparkline-wrap" title="Live Activity Sparkline">
        <svg viewBox={`0 0 ${w} ${h}`} className="service-sparkline-svg" preserveAspectRatio="none">
          <polygon points={polygonStr} fill={color} fillOpacity="0.15" />
          <polyline points={polylineStr} fill="none" stroke={color} strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </div>
    );
  };

  const handleFilterClick = (e, id) => {
    e.stopPropagation();
    state.setSelectedService(selectedService === id ? 'all' : id);
  };

  const handleCardClick = (id) => {
    state.setSelectedService(selectedService === id ? 'all' : id);
  };

  const handleRestore = (e, id) => {
    e.stopPropagation();
    state.restoreService(id);
  };

  const handleSlow = (e, id) => {
    e.stopPropagation();
    state.injectLatency(id, 800);
  };

  const handleFail = (e, id) => {
    e.stopPropagation();
    state.simulateFailure(id, 'DOWN');
  };

  const handleRestoreAll = () => {
    state.restoreAllServices();
  };

  return (
    <>
      <div className="section-header">
        <div className="section-title-wrap">
          <div className="section-icon">
            <Icon name="server" />
          </div>
          <div>
            <h2 className="section-title">Microservices Status & Health</h2>
            <p className="section-desc">Upstream service nodes, health states, live activity sparklines, and fault triggers</p>
          </div>
        </div>
        <div className="section-actions">
          <button className="btn btn-sm btn-outline" id="btn-restore-all" onClick={handleRestoreAll} title="Restore all services to healthy UP status">
            <Icon name="checkCircle" />
            <span>Restore All to Healthy</span>
          </button>
        </div>
      </div>

      <div className="service-cards-grid">
        {services.map(service => {
          const healthClass = getServiceHealthClass(service.status);
          const isSelected = selectedService === service.id;
          const isDown = service.status === 'DOWN';
          const isSlow = service.status === 'SLOW';

          const isHovered = hoveredServiceId === service.id;
          const isDimmed = hoveredServiceId && hoveredServiceId !== service.id;

          return (
            <div
              key={service.id}
              className={`service-card ${healthClass} ${isSelected ? 'selected' : ''} ${isHovered ? 'is-hovered' : ''} ${isDimmed ? 'is-dimmed' : ''}`}
              data-service-id={service.id}
              tabIndex={0}
              role="button"
              aria-label={`${service.name} status ${service.status}`}
              onClick={() => handleCardClick(service.id)}
              onMouseEnter={() => state.setHoveredService(service.id)}
              onMouseLeave={() => state.setHoveredService(null)}
            >
              <div className="service-card-header">
                <div className="service-identity">
                  <div className={`service-icon-wrap ${healthClass}`}>
                    <Icon name={isDown ? 'xCircle' : isSlow ? 'alertTriangle' : 'cpu'} />
                  </div>
                  <div>
                    <h3 className="service-name">{service.name}</h3>
                    <div className="service-meta">
                      <span className="service-port font-mono">:{service.port}</span>
                      <span className="service-version font-mono">{service.version}</span>
                    </div>
                  </div>
                </div>

                <div className={`status-badge ${healthClass}`}>
                  <span className="badge-dot"></span>
                  <span className="badge-label font-mono">{service.status}</span>
                </div>
              </div>

              <div className="service-card-body">
                <div className="service-stat-grid">
                  <div className="service-stat-item">
                    <span className="stat-label">Latency</span>
                    <span className={`stat-val ${isDown ? 'text-danger' : isSlow ? 'text-warning' : 'text-success'}`}>
                      {isDown ? 'TIMEOUT' : formatMs(service.latency)}
                    </span>
                  </div>

                  <div className="service-stat-item">
                    <span className="stat-label">Requests</span>
                    <span className="stat-val font-mono">{formatNumber(service.requestCount)}</span>
                  </div>

                  <div className="service-stat-item">
                    <span className="stat-label">Errors</span>
                    <span className={`stat-val font-mono ${service.errorCount > 100 ? 'text-danger' : service.errorCount > 0 ? 'text-warning' : ''}`}>
                      {formatNumber(service.errorCount)}
                    </span>
                  </div>

                  <div className="service-stat-item">
                    <span className="stat-label">Uptime</span>
                    <span className={`stat-val font-mono ${service.uptimePercent < 90 ? 'text-danger' : service.uptimePercent < 98 ? 'text-warning' : 'text-success'}`}>
                      {service.uptimePercent}%
                    </span>
                  </div>
                </div>

                {/* Live Sparkline Activity Graph */}
                <div className="service-sparkline-container">
                  <div className="sparkline-header">
                    <span className="sparkline-label">Activity Trend</span>
                    <span className="sparkline-status font-mono">{isDown ? 'OUTAGE' : isSlow ? 'DEGRADED' : 'OPTIMAL'}</span>
                  </div>
                  {generateSparkline(service)}
                </div>

                <div className="service-endpoints-list">
                  <span className="endpoints-title">Endpoints ({service.endpoints.length}):</span>
                  <div className="endpoints-tags">
                    {service.endpoints.map((ep, idx) => (
                      <span className="endpoint-tag" key={idx}>
                        <span className={`ep-method method-${ep.method.toLowerCase()}`}>{ep.method}</span>
                        <span className="ep-path font-mono">{ep.path}</span>
                      </span>
                    ))}
                  </div>
                </div>
              </div>

              <div className="service-card-footer">
                <button
                  className="btn btn-xs btn-outline btn-filter-service"
                  data-id={service.id}
                  onClick={(e) => handleFilterClick(e, service.id)}
                  title="Filter Explorer and Logs"
                >
                  <Icon name="filter" />
                  <span>{isSelected ? 'Filtering' : 'Filter'}</span>
                </button>

                <div className="card-quick-actions">
                  {service.status !== 'UP' ? (
                    <button
                      className="btn btn-xs btn-success-subtle btn-quick-restore"
                      data-id={service.id}
                      onClick={(e) => handleRestore(e, service.id)}
                      title="Restore to healthy UP"
                    >
                      Restore UP
                    </button>
                  ) : (
                    <>
                      <button
                        className="btn btn-xs btn-warning-subtle btn-quick-slow"
                        data-id={service.id}
                        onClick={(e) => handleSlow(e, service.id)}
                        title="Inject 800ms latency"
                      >
                        +800ms
                      </button>
                      <button
                        className="btn btn-xs btn-danger-subtle btn-quick-fail"
                        data-id={service.id}
                        onClick={(e) => handleFail(e, service.id)}
                        title="Simulate 500 failure"
                      >
                        Outage
                      </button>
                    </>
                  )}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </>
  );
}
