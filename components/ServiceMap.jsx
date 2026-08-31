"use client";

import React, { useState, useEffect, useRef } from 'react';
import { state } from '../lib/state.js';
import { Icon } from '../utils/svgIcons.jsx';
import { formatMs, formatNumber } from '../utils/formatters.js';

export function ServiceMap() {
  const [services, setServices] = useState([...state.services]);
  const [selectedService, setSelectedService] = useState(state.selectedService);
  const [hoveredNodeId, setHoveredNodeId] = useState(state.hoveredServiceId);
  const [isSimRunning, setIsSimRunning] = useState(state.isSimulationRunning);
  const [zoomLevel, setZoomLevel] = useState(1.0);
  const [tooltipData, setTooltipData] = useState(null);

  const viewportRef = useRef(null);

  useEffect(() => {
    const handleServices = () => setServices([...state.services]);
    const handleFilters = () => setSelectedService(state.selectedService);
    const handleSim = ({ isRunning }) => setIsSimRunning(isRunning);
    const handleHover = ({ serviceId }) => setHoveredNodeId(serviceId);

    const unsubServices = state.subscribe('services', handleServices);
    const unsubFilters = state.subscribe('filters', handleFilters);
    const unsubSim = state.subscribe('simulationStateChange', handleSim);
    const unsubHover = state.subscribe('hoveredService', handleHover);

    return () => {
      unsubServices();
      unsubFilters();
      unsubSim();
      unsubHover();
    };
  }, []);

  const getNodeColor = (status) => {
    if (status === 'UP') return { fill: '#10b981', border: '#059669', bg: 'rgba(16, 185, 129, 0.12)', text: '#34d399' };
    if (status === 'SLOW') return { fill: '#f59e0b', border: '#d97706', bg: 'rgba(245, 158, 11, 0.12)', text: '#fbbf24' };
    return { fill: '#ef4444', border: '#dc2626', bg: 'rgba(239, 68, 68, 0.12)', text: '#f87171' };
  };

  const servicePositions = [
    { id: 'users', y: 16, cy: 42 },
    { id: 'events', y: 82, cy: 108 },
    { id: 'orders', y: 148, cy: 174 },
    { id: 'payments', y: 214, cy: 240 }
  ];

  const gwStatus = state.getSystemStatus();
  const gwColor = getNodeColor(gwStatus.color === 'up' ? 'UP' : gwStatus.color === 'slow' ? 'SLOW' : 'DOWN');

  const handleNodeMouseEnter = (e, s) => {
    state.setHoveredService(s.id);

    if (!viewportRef.current) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const viewRect = viewportRef.current.getBoundingClientRect();
    const topPos = rect.top - viewRect.top - 10;
    const leftPos = Math.max(10, rect.left - viewRect.left - 230);

    setTooltipData({
      service: s,
      top: topPos,
      left: leftPos
    });
  };

  const handleNodeMouseLeave = () => {
    state.setHoveredService(null);
    setTooltipData(null);
  };

  const handleSelectService = (id) => {
    state.setSelectedService(selectedService === id ? 'all' : id);
  };

  return (
    <>
      <div className="section-header">
        <div className="section-title-wrap">
          <div className="section-icon">
            <Icon name="layers" />
          </div>
          <div>
            <h2 className="section-title">Service Dependency Topology</h2>
            <p className="section-desc">Real-time architectural flow from client edge through MiniEdge proxy to upstream microservices</p>
          </div>
        </div>

        <div className="map-header-right">
          {/* Compact Legend */}
          <div className="map-legend">
            <span className="legend-item"><span className="legend-dot up"></span> Healthy</span>
            <span className="legend-item"><span className="legend-dot slow"></span> Degraded</span>
            <span className="legend-item"><span className="legend-dot down"></span> Down</span>
            <span className="legend-item"><span className="legend-dot traffic"></span> Traffic Pulse</span>
            <span className="legend-item"><span className="legend-dot dropped"></span> Dropped</span>
          </div>

          {/* Map Zoom Controls */}
          <div className="map-controls-group" aria-label="Map Zoom Controls">
            <button
              className="btn btn-xs btn-outline map-ctrl-btn"
              id="map-zoom-in"
              onClick={() => setZoomLevel(prev => Math.min(1.4, prev + 0.1))}
              title="Zoom In"
            >
              +
            </button>
            <button
              className="btn btn-xs btn-outline map-ctrl-btn"
              id="map-zoom-out"
              onClick={() => setZoomLevel(prev => Math.max(0.7, prev - 0.1))}
              title="Zoom Out"
            >
              −
            </button>
            <button
              className="btn btn-xs btn-outline map-ctrl-btn"
              id="map-zoom-reset"
              onClick={() => setZoomLevel(1.0)}
              title="Reset View"
            >
              ↺
            </button>
          </div>
        </div>
      </div>

      <div className="service-map-container" id="service-map-viewport" ref={viewportRef}>
        {/* Interactive Tooltip Overlay */}
        {tooltipData && (
          <div
            className="map-hover-tooltip"
            id="map-hover-tooltip"
            style={{ top: `${tooltipData.top}px`, left: `${tooltipData.left}px`, display: 'block' }}
          >
            <div className="tooltip-card">
              <div className="tooltip-header">
                <span className="tooltip-title">{tooltipData.service.name}</span>
                <span className={`status-pill status-${tooltipData.service.status === 'UP' ? '2xx' : tooltipData.service.status === 'SLOW' ? '4xx' : '5xx'}`}>
                  {tooltipData.service.status}
                </span>
              </div>
              <div className="tooltip-grid">
                <div className="tooltip-item">
                  <span className="tooltip-label">Port:</span> <span className="font-mono">:{tooltipData.service.port}</span>
                </div>
                <div className="tooltip-item">
                  <span className="tooltip-label">Latency:</span>{' '}
                  <span className={`font-mono ${tooltipData.service.status === 'DOWN' ? 'text-danger' : tooltipData.service.status === 'SLOW' ? 'text-warning' : 'text-success'}`}>
                    {tooltipData.service.status === 'DOWN' ? 'TIMEOUT' : formatMs(tooltipData.service.latency)}
                  </span>
                </div>
                <div className="tooltip-item">
                  <span className="tooltip-label">Requests:</span> <span className="font-mono">{formatNumber(tooltipData.service.requestCount)}</span>
                </div>
                <div className="tooltip-item">
                  <span className="tooltip-label">Errors:</span>{' '}
                  <span className={`font-mono ${tooltipData.service.errorCount > 0 ? 'text-danger' : ''}`}>
                    {formatNumber(tooltipData.service.errorCount)}
                  </span>
                </div>
                <div className="tooltip-item">
                  <span className="tooltip-label">Uptime:</span> <span className="font-mono">{tooltipData.service.uptimePercent}%</span>
                </div>
                <div className="tooltip-item">
                  <span className="tooltip-label">Routes:</span> <span className="font-mono">{tooltipData.service.endpoints.length} endpoints</span>
                </div>
              </div>
              <div className="tooltip-footer">Click to filter dashboard traffic</div>
            </div>
          </div>
        )}

        <div
          className="service-map-scaler"
          id="service-map-scaler"
          style={{ transform: `scale(${zoomLevel})`, transformOrigin: 'center center', transition: 'transform 0.2s ease' }}
        >
          <svg viewBox="0 0 920 290" className="service-map-svg" preserveAspectRatio="xMidYMid meet" aria-label="Microservice Topology Map">
            <defs>
              <marker id="arrow-client" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#38bdf8" />
              </marker>
              <marker id="arrow-up" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#10b981" />
              </marker>
              <marker id="arrow-slow" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#f59e0b" />
              </marker>
              <marker id="arrow-down" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#ef4444" />
              </marker>
            </defs>

            {/* 1. CLIENT -> GATEWAY CONNECTION */}
            <line x1="164" y1="145" x2="264" y2="145" stroke="#38bdf8" strokeWidth="2" strokeDasharray="3 3" markerEnd="url(#arrow-client)" />

            {/* 2. GATEWAY -> SERVICES CONNECTIONS */}
            {servicePositions.map(pos => {
              const service = services.find(s => s.id === pos.id);
              const status = service?.status || 'UP';
              const colors = getNodeColor(status);
              const isDown = status === 'DOWN';
              const isSlow = status === 'SLOW';
              const isDimmed = hoveredNodeId && hoveredNodeId !== service.id;

              const pathD = `M 450 145 C 520 145, 555 ${pos.cy}, 634 ${pos.cy}`;
              const marker = isDown ? 'url(#arrow-down)' : isSlow ? 'url(#arrow-slow)' : 'url(#arrow-up)';

              const badgeX = 535;
              const badgeY = (145 + pos.cy) / 2 + (pos.cy < 145 ? -10 : 10);
              const animDur = isSlow ? '3.2s' : '1.2s';

              return (
                <g key={pos.id} className={`map-edge-group ${isDimmed ? 'is-dimmed' : ''}`} data-service-id={service.id}>
                  <path
                    d={pathD}
                    fill="none"
                    stroke={colors.fill}
                    strokeWidth={isDown ? '1.5' : hoveredNodeId === service.id ? '2.8' : '2'}
                    strokeDasharray={isDown ? '4 4' : 'none'}
                    strokeOpacity={isDown ? '0.45' : isDimmed ? '0.25' : '0.85'}
                    markerEnd={marker}
                  />

                  {!isDown && isSimRunning && (
                    <circle r={hoveredNodeId === service.id ? '4' : '3'} fill="#ffffff">
                      <animateMotion path={pathD} dur={animDur} repeatCount="indefinite" />
                    </circle>
                  )}

                  <g className="map-latency-pill" transform={`translate(${badgeX - 27}, ${badgeY - 9})`}>
                    <rect width="54" height="18" rx="3" fill="#080c14" stroke={colors.border} strokeWidth="1" />
                    <text x="27" y="12" fill={colors.text} fontSize="9" fontWeight="700" textAnchor="middle" fontFamily="monospace">
                      {isDown ? 'DROP' : formatMs(service.latency)}
                    </text>
                  </g>
                </g>
              );
            })}

            {/* 3. CLIENT NODE */}
            <g className="map-node client-node" transform="translate(24, 110)">
              <rect width="140" height="70" rx="6" fill="#131e33" stroke="#38bdf8" strokeWidth="1.5" />
              <circle cx="22" cy="35" r="10" fill="rgba(56, 189, 248, 0.15)" stroke="#38bdf8" strokeWidth="1.5" />
              <circle cx="22" cy="35" r="4" fill="#38bdf8" />
              <text x="42" y="31" fill="#f8fafc" fontSize="13" fontWeight="700">Client</text>
              <text x="42" y="47" fill="#94a3b8" fontSize="9.5" fontFamily="monospace">HTTP Inbound</text>
              <text x="42" y="60" fill="#38bdf8" fontSize="9" fontFamily="monospace">External Traffic</text>
            </g>

            {/* 4. MINIEDGE GATEWAY NODE */}
            <g className="map-node gateway-node" transform="translate(270, 90)">
              <rect width="180" height="110" rx="8" fill="#0d1524" stroke={gwColor.fill} strokeWidth="2" />
              <rect x="6" y="6" width="168" height="98" rx="6" fill="rgba(19, 30, 51, 0.5)" stroke="#1f2d47" strokeWidth="1" />
              <circle cx="26" cy="30" r="10" fill={gwColor.bg} stroke={gwColor.fill} strokeWidth="1.5" />
              <circle cx="26" cy="30" r="4" fill={gwColor.fill} />
              <text x="44" y="28" fill="#ffffff" fontSize="14" fontWeight="800">MiniEdge</text>
              <text x="44" y="42" fill="#38bdf8" fontSize="10" fontFamily="monospace">:8080 Core Proxy</text>
              <line x1="14" y1="58" x2="166" y2="58" stroke="#1f2d47" strokeWidth="1" />
              <text x="14" y="74" fill="#94a3b8" fontSize="9.5">Local Dev Gateway</text>
              <text x="14" y="89" fill={gwColor.text} fontSize="9.5" fontWeight="700" fontFamily="monospace">{gwStatus.text}</text>
            </g>

            {/* 5. SERVICE NODES */}
            {servicePositions.map(pos => {
              const service = services.find(s => s.id === pos.id);
              const status = service?.status || 'UP';
              const colors = getNodeColor(status);
              const isSelected = selectedService === service.id;
              const isDown = status === 'DOWN';
              const isDimmed = hoveredNodeId && hoveredNodeId !== service.id;

              return (
                <g
                  key={pos.id}
                  className={`map-node service-node ${isSelected ? 'is-selected' : ''} ${isDimmed ? 'is-dimmed' : ''}`}
                  transform={`translate(640, ${pos.y})`}
                  data-service-id={service.id}
                  tabIndex={0}
                  role="button"
                  aria-label={`${service.name} (${status})`}
                  onClick={() => handleSelectService(service.id)}
                  onMouseEnter={(e) => handleNodeMouseEnter(e, service)}
                  onMouseLeave={handleNodeMouseLeave}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      handleSelectService(service.id);
                    }
                  }}
                >
                  <rect
                    width="240"
                    height="52"
                    rx="6"
                    fill={isSelected ? '#1c2b47' : '#131e33'}
                    stroke={colors.fill}
                    strokeWidth={isSelected ? '2.5' : '1.2'}
                  />
                  <rect width="4" height="52" rx="2" fill={colors.fill} />
                  <circle cx="20" cy="26" r="7" fill={colors.bg} stroke={colors.fill} strokeWidth="1.5" />
                  <circle cx="20" cy="26" r="3" fill={colors.fill} />
                  <text x="34" y="22" fill="#f8fafc" fontSize="12" fontWeight="700">{service.name}</text>
                  <text x="34" y="38" fill="#94a3b8" fontSize="9.5" fontFamily="monospace">
                    :{service.port} • {service.endpoints.length} routes • {isDown ? 'TIMEOUT' : formatMs(service.latency)}
                  </text>
                  <rect x="180" y="16" width="48" height="20" rx="3" fill={colors.bg} stroke={colors.border} strokeWidth="1" />
                  <text x="204" y="30" fill={colors.text} fontSize="10" fontWeight="800" textAnchor="middle" fontFamily="monospace">{status}</text>
                </g>
              );
            })}
          </svg>
        </div>
      </div>
    </>
  );
}
