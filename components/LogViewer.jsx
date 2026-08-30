"use client";

import React, { useState, useEffect, useRef } from 'react';
import { state } from '../lib/state.js';
import { Icon } from '../utils/svgIcons.jsx';
import { formatTime, formatMs } from '../utils/formatters.js';

export function LogViewer({ onShowToast }) {
  const [logs, setLogs] = useState([...state.logs]);
  const [logLevelFilter, setLogLevelFilter] = useState(state.logLevelFilter);
  const [logSearchQuery, setLogSearchQuery] = useState(state.logSearchQuery);
  const [logErrorOnly, setLogErrorOnly] = useState(state.logErrorOnly);
  const [isAutoScroll, setIsAutoScroll] = useState(state.isLogAutoScroll);
  const [hoveredServiceId, setHoveredServiceId] = useState(state.hoveredServiceId);

  const terminalBodyRef = useRef(null);

  useEffect(() => {
    const handleLogs = () => {
      setLogs([...state.logs]);
    };
    const handleLogFilters = () => {
      setLogLevelFilter(state.logLevelFilter);
      setLogSearchQuery(state.logSearchQuery);
      setLogErrorOnly(state.logErrorOnly);
    };
    const handleHover = ({ serviceId }) => setHoveredServiceId(serviceId);

    const unsubLogs = state.subscribe('logs', handleLogs);
    const unsubLogFilters = state.subscribe('logFilters', handleLogFilters);
    const unsubHover = state.subscribe('hoveredService', handleHover);

    return () => {
      unsubLogs();
      unsubLogFilters();
      unsubHover();
    };
  }, []);

  useEffect(() => {
    if (isAutoScroll && terminalBodyRef.current) {
      terminalBodyRef.current.scrollTop = 0; // newest are on top
    }
  }, [logs, isAutoScroll]);

  const getFilteredLogs = () => {
    return logs.filter(log => {
      if (logErrorOnly && log.level !== 'ERROR') return false;
      if (logLevelFilter !== 'ALL' && log.level !== logLevelFilter) return false;
      if (logSearchQuery) {
        const q = logSearchQuery.toLowerCase();
        const matchMsg = log.message?.toLowerCase().includes(q);
        const matchPath = log.path?.toLowerCase().includes(q);
        const matchService = log.service?.toLowerCase().includes(q);
        const matchReqId = log.requestId?.toLowerCase().includes(q);
        if (!matchMsg && !matchPath && !matchService && !matchReqId) return false;
      }
      return true;
    });
  };

  const handleCopyLog = (text) => {
    navigator.clipboard?.writeText(text);
    if (onShowToast) {
      onShowToast('Copied log line to clipboard', 'info');
    }
  };

  const handleClearLogs = () => {
    state.clearLogs();
    if (onShowToast) {
      onShowToast('Cleared log terminal output', 'info');
    }
  };

  const filteredLogs = getFilteredLogs();
  const recentEvents = logs.slice(0, 5);

  return (
    <>
      <div className="section-header">
        <div className="section-title-wrap">
          <div className="section-icon">
            <Icon name="terminal" />
          </div>
          <div>
            <h2 className="section-title">Developer Log Console</h2>
            <p className="section-desc">Monospace stdout stream of gateway routing decisions, circuit break events, and proxy errors</p>
          </div>
        </div>
      </div>

      {/* Live Event Feed Strip */}
      <div className="live-event-feed-strip" aria-label="Live System Event Stream">
        <div className="feed-header-label">
          <Icon name="radio" />
          <span>LIVE ARCHITECTURAL EVENTS:</span>
        </div>
        <div className="feed-items-scroll">
          {recentEvents.map(evt => (
            <div className="feed-event-item" key={evt.id}>
              <span className="feed-time font-mono">{formatTime(evt.timestamp)}</span>
              <span className={`feed-level log-col-level ${evt.level === 'ERROR' ? 'text-danger' : evt.level === 'WARN' ? 'text-warning' : 'text-info'}`}>
                [{evt.level}]
              </span>
              <span className="feed-msg font-mono">{evt.message}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Log Console Toolbar */}
      <div className="log-toolbar">
        <div className="log-level-pills" role="radiogroup" aria-label="Log Level Filter">
          {['ALL', 'INFO', 'WARN', 'ERROR'].map(lvl => (
            <button
              key={lvl}
              className={`log-level-btn ${lvl.toLowerCase()} ${logLevelFilter === lvl ? 'active' : ''}`}
              onClick={() => state.setLogLevelFilter(lvl)}
              data-level={lvl}
            >
              {lvl}
            </button>
          ))}
        </div>

        <div className="log-search-wrap">
          <Icon name="search" className="search-icon icon" />
          <input
            type="text"
            id="log-search-input"
            className="form-control form-control-sm"
            placeholder="Search logs, request IDs, paths..."
            value={logSearchQuery}
            onChange={(e) => state.setLogSearchQuery(e.target.value)}
          />
        </div>

        <div className="log-autoscroll-toggle">
          <label className="toggle-label">
            <input
              type="checkbox"
              id="log-errors-only-toggle"
              checked={logErrorOnly}
              onChange={(e) => state.setLogErrorOnly(e.target.checked)}
            />
            <span>Errors Only</span>
          </label>
        </div>

        <div className="log-autoscroll-toggle">
          <label className="toggle-label">
            <input
              type="checkbox"
              id="log-autoscroll-toggle"
              checked={isAutoScroll}
              onChange={(e) => {
                setIsAutoScroll(e.target.checked);
                state.isLogAutoScroll = e.target.checked;
              }}
            />
            <span>Auto-Scroll</span>
          </label>
        </div>

        <button className="btn btn-xs btn-outline" id="btn-clear-logs" onClick={handleClearLogs} title="Clear terminal view">
          <Icon name="trash" />
          <span>Clear</span>
        </button>
      </div>

      {/* Monospace Terminal Window */}
      <div className="log-terminal-window">
        <div className="log-terminal-header">
          <div className="terminal-dots">
            <span className="dot red"></span>
            <span className="dot yellow"></span>
            <span className="dot green"></span>
          </div>
          <span className="terminal-title">miniedge-gateway-stdout.log</span>
          <span className="terminal-badge font-mono">{filteredLogs.length} lines</span>
        </div>

        <div className="log-entries-list" id="log-terminal-body" ref={terminalBodyRef} tabIndex={0} aria-label="Terminal Log Stream">
          {filteredLogs.length === 0 ? (
            <div className="empty-terminal-msg">
              No log records match the current filter criteria.
            </div>
          ) : (
            filteredLogs.map(log => {
              const rawText = `${log.timestamp} [${log.level}] [${log.requestId || '-'}] [${log.service}] ${log.method || ''} ${log.path || ''} ${log.status || ''} - ${log.message}`;
              const isHighlighted = hoveredServiceId === log.service;
              const isDimmed = hoveredServiceId && hoveredServiceId !== log.service;

              return (
                <div
                  key={log.id}
                  className={`log-line log-lvl-${log.level.toLowerCase()} ${isHighlighted ? 'is-log-highlighted' : ''} ${isDimmed ? 'is-log-dimmed' : ''}`}
                  data-service-id={log.service}
                  title="Click to copy log line"
                  onClick={() => handleCopyLog(rawText)}
                  onMouseEnter={() => log.service && state.setHoveredService(log.service)}
                  onMouseLeave={() => state.setHoveredService(null)}
                >
                  <span className="log-col-time">{formatTime(log.timestamp)}</span>
                  <span className="log-col-level">[{log.level}]</span>
                  {log.requestId && <span className="log-col-reqid">[{log.requestId}]</span>}
                  <span className="log-col-service">[{log.service}]</span>
                  {log.method && <span className={`log-col-method method-${log.method.toLowerCase()}`}>{log.method}</span>}
                  {log.path && <span className="log-col-path">{log.path}</span>}
                  {log.status && <span className="log-col-status font-mono">HTTP {log.status}</span>}
                  {log.latency && <span className="log-col-latency font-mono">({formatMs(log.latency)})</span>}
                  <span className="log-col-msg">{log.message}</span>
                </div>
              );
            })
          )}
        </div>
      </div>
    </>
  );
}
