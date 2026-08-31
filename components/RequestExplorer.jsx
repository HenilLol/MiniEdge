"use client";

import React, { useState, useEffect } from 'react';
import { state } from '../lib/state.js';
import { Icon } from '../utils/svgIcons.jsx';
import { formatMs, formatTime, getStatusClass } from '../utils/formatters.js';
import { RequestModal } from './RequestModal.jsx';

export function RequestExplorer() {
  const [requests, setRequests] = useState([...state.requests]);
  const [services, setServices] = useState([...state.services]);
  const [selectedService, setSelectedService] = useState(state.selectedService);
  const [selectedMethod, setSelectedMethod] = useState(state.selectedMethod);
  const [selectedStatusCategory, setSelectedStatusCategory] = useState(state.selectedStatusCategory);
  const [selectedLatencyFilter, setSelectedLatencyFilter] = useState(state.selectedLatencyFilter);
  const [sortOrder, setSortOrder] = useState(state.sortOrder);
  const [searchQuery, setSearchQuery] = useState(state.searchQuery);
  const [selectedRequestDetail, setSelectedRequestDetail] = useState(state.selectedRequestDetail);
  const [hoveredServiceId, setHoveredServiceId] = useState(state.hoveredServiceId);

  useEffect(() => {
    const handleRequests = () => setRequests([...state.requests]);
    const handleServices = () => setServices([...state.services]);
    const handleFilters = () => {
      setSelectedService(state.selectedService);
      setSelectedMethod(state.selectedMethod);
      setSelectedStatusCategory(state.selectedStatusCategory);
      setSelectedLatencyFilter(state.selectedLatencyFilter);
      setSortOrder(state.sortOrder);
      setSearchQuery(state.searchQuery);
    };
    const handleHover = ({ serviceId }) => setHoveredServiceId(serviceId);

    const unsubRequests = state.subscribe('requests', handleRequests);
    const unsubServices = state.subscribe('services', handleServices);
    const unsubFilters = state.subscribe('filters', handleFilters);
    const unsubHover = state.subscribe('hoveredService', handleHover);

    return () => {
      unsubRequests();
      unsubServices();
      unsubFilters();
      unsubHover();
    };
  }, []);

  const getFilteredRequests = () => {
    let list = requests.filter(req => {
      if (selectedService !== 'all' && req.service !== selectedService) return false;
      if (selectedMethod !== 'all' && req.method !== selectedMethod) return false;
      if (selectedStatusCategory !== 'all') {
        const code = Number(req.status);
        if (selectedStatusCategory === '2xx' && (code < 200 || code >= 300)) return false;
        if (selectedStatusCategory === '4xx' && (code < 400 || code >= 500)) return false;
        if (selectedStatusCategory === '5xx' && code < 500) return false;
      }
      if (selectedLatencyFilter !== 'all') {
        if (selectedLatencyFilter === '<50' && req.latency >= 50) return false;
        if (selectedLatencyFilter === '50-200' && (req.latency < 50 || req.latency > 200)) return false;
        if (selectedLatencyFilter === '>200' && req.latency <= 200) return false;
      }
      if (searchQuery) {
        const query = searchQuery.toLowerCase();
        const matchPath = req.path.toLowerCase().includes(query);
        const matchId = req.id.toLowerCase().includes(query);
        const matchService = req.service.toLowerCase().includes(query);
        if (!matchPath && !matchId && !matchService) return false;
      }
      return true;
    });

    if (sortOrder === 'latency-desc') {
      list.sort((a, b) => b.latency - a.latency);
    } else if (sortOrder === 'status-desc') {
      list.sort((a, b) => b.status - a.status);
    } else {
      list.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
    }

    return list;
  };

  const filtered = getFilteredRequests();
  const hasActiveFilters = selectedService !== 'all' || 
                           selectedMethod !== 'all' || 
                           selectedStatusCategory !== 'all' || 
                           selectedLatencyFilter !== 'all' || 
                           searchQuery;

  const handleResetFilters = () => {
    state.setSelectedService('all');
    state.setSelectedMethod('all');
    state.setSelectedStatusCategory('all');
    state.setSelectedLatencyFilter('all');
    state.setSearchQuery('');
    state.setSortOrder('newest');
  };

  const handleInspect = (req) => {
    setSelectedRequestDetail(req);
    state.selectedRequestDetail = req;
  };

  return (
    <>
      <div className="section-header">
        <div className="section-title-wrap">
          <div className="section-icon">
            <Icon name="search" />
          </div>
          <div>
            <h2 className="section-title">Request Explorer</h2>
            <p className="section-desc">Live stream of incoming client HTTP traffic routed through MiniEdge gateway</p>
          </div>
        </div>
        <div className="request-header-right">
          <div className="request-count-badge">
            <span>Showing {filtered.length} of {requests.length} requests</span>
          </div>
        </div>
      </div>

      {/* Advanced Filter Toolbar */}
      <div className="filter-toolbar">
        <div className="search-input-wrap">
          <Icon name="search" className="search-icon icon" />
          <input
            type="text"
            id="req-search-input"
            className="form-control form-control-sm"
            placeholder="Filter path, request ID, service..."
            value={searchQuery}
            onChange={(e) => state.setSearchQuery(e.target.value)}
          />
          {searchQuery && (
            <button className="btn-clear-search" id="btn-clear-req-search" onClick={() => state.setSearchQuery('')} title="Clear">
              ×
            </button>
          )}
        </div>

        <div className="filter-group">
          <label className="filter-label">Service:</label>
          <select
            id="filter-service-select"
            className="form-select form-select-sm"
            value={selectedService}
            onChange={(e) => state.setSelectedService(e.target.value)}
          >
            <option value="all">All Services</option>
            {services.map(s => (
              <option key={s.id} value={s.id}>{s.name} (:{s.port})</option>
            ))}
          </select>
        </div>

        <div className="filter-group">
          <label className="filter-label">Method:</label>
          <select
            id="filter-method-select"
            className="form-select form-select-sm"
            value={selectedMethod}
            onChange={(e) => state.setSelectedMethod(e.target.value)}
          >
            <option value="all">All</option>
            <option value="GET">GET</option>
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="DELETE">DELETE</option>
          </select>
        </div>

        <div className="filter-group">
          <label className="filter-label">Status:</label>
          <select
            id="filter-status-select"
            className="form-select form-select-sm"
            value={selectedStatusCategory}
            onChange={(e) => state.setSelectedStatusCategory(e.target.value)}
          >
            <option value="all">All Statuses</option>
            <option value="2xx">2xx Success</option>
            <option value="4xx">4xx Client Error</option>
            <option value="5xx">5xx Server Error</option>
          </select>
        </div>

        <div className="filter-group">
          <label className="filter-label">Latency:</label>
          <select
            id="filter-latency-select"
            className="form-select form-select-sm"
            value={selectedLatencyFilter}
            onChange={(e) => state.setSelectedLatencyFilter(e.target.value)}
          >
            <option value="all">All Speeds</option>
            <option value="<50">&lt; 50ms (Fast)</option>
            <option value="50-200">50 - 200ms</option>
            <option value=">200">&gt; 200ms (Slow)</option>
          </select>
        </div>

        <div className="filter-group">
          <label className="filter-label">Sort:</label>
          <select
            id="filter-sort-select"
            className="form-select form-select-sm"
            value={sortOrder}
            onChange={(e) => state.setSortOrder(e.target.value)}
          >
            <option value="newest">Newest First</option>
            <option value="latency-desc">Latency (High to Low)</option>
            <option value="status-desc">Status Code</option>
          </select>
        </div>

        {hasActiveFilters && (
          <button className="btn btn-xs btn-outline" id="btn-reset-filters" onClick={handleResetFilters}>
            Reset Filters
          </button>
        )}
      </div>

      {/* Requests Table Container */}
      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th style={{ width: '75px' }}>Method</th>
              <th>Path & Request ID</th>
              <th style={{ width: '90px' }}>Status</th>
              <th style={{ width: '100px' }}>Latency</th>
              <th style={{ width: '130px' }}>Target Service</th>
              <th style={{ width: '120px' }}>Timestamp</th>
              <th style={{ width: '60px', textAlign: 'right' }}>Inspect</th>
            </tr>
          </thead>
          <tbody>
            {filtered.length === 0 ? (
              <tr>
                <td colSpan={7} className="empty-state-cell">
                  <div className="empty-terminal-msg">
                    No requests match the selected filters.
                  </div>
                </td>
              </tr>
            ) : (
              filtered.map(req => {
                const statusClass = getStatusClass(req.status);
                const isSlow = req.latency > 500;
                const isDown = req.status >= 500;
                const isRowHighlighted = hoveredServiceId === req.service;
                const isRowDimmed = hoveredServiceId && hoveredServiceId !== req.service;

                return (
                  <tr
                    key={req.id}
                    className={`request-row ${isRowHighlighted ? 'is-row-highlighted' : ''} ${isRowDimmed ? 'is-row-dimmed' : ''}`}
                    data-request-id={req.id}
                    data-service-id={req.service}
                    tabIndex={0}
                    role="button"
                    aria-label={`Request ${req.method} ${req.path}`}
                    onClick={() => handleInspect(req)}
                    onMouseEnter={() => state.setHoveredService(req.service)}
                    onMouseLeave={() => state.setHoveredService(null)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        handleInspect(req);
                      }
                    }}
                  >
                    <td>
                      <span className={`method-badge method-${req.method.toLowerCase()}`}>{req.method}</span>
                    </td>
                    <td className="cell-path">
                      <span className="req-path-text">{req.path}</span>
                      <span className="req-id-sub font-mono">{req.id}</span>
                    </td>
                    <td>
                      <span className={`status-pill ${statusClass}`}>
                        {req.status}
                      </span>
                    </td>
                    <td>
                      <span className={`latency-pill ${isDown ? 'latency-down' : isSlow ? 'latency-slow' : 'latency-fast'} font-mono`}>
                        {formatMs(req.latency)}
                      </span>
                    </td>
                    <td>
                      <span className={`service-tag tag-${req.service}`}>
                        {req.serviceName}
                      </span>
                    </td>
                    <td className="cell-time font-mono">
                      {formatTime(req.timestamp)}
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <button
                        className="btn-inspect-req"
                        data-id={req.id}
                        onClick={(e) => {
                          e.stopPropagation();
                          handleInspect(req);
                        }}
                        title="Inspect full request"
                      >
                        <Icon name="externalLink" />
                      </button>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Request Inspector Modal */}
      <RequestModal request={selectedRequestDetail} onClose={() => { setSelectedRequestDetail(null); state.selectedRequestDetail = null; }} />
    </>
  );
}
