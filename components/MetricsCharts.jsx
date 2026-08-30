"use client";

import React, { useState, useEffect, useRef, useCallback } from 'react';
import { state } from '../lib/state.js';
import { Icon } from '../utils/svgIcons.jsx';
import { formatMs } from '../utils/formatters.js';

export function MetricsCharts() {
  const [timeRange, setTimeRange] = useState(state.metricsTimeRange);
  const [hoveredIndex, setHoveredIndex] = useState({ throughput: -1, errors: -1, latency: -1 });

  const canvasThroughputRef = useRef(null);
  const canvasErrorsRef = useRef(null);
  const canvasLatencyRef = useRef(null);

  const cardThroughputRef = useRef(null);
  const cardErrorsRef = useRef(null);
  const cardLatencyRef = useRef(null);

  const tooltipThroughputRef = useRef(null);
  const tooltipErrorsRef = useRef(null);
  const tooltipLatencyRef = useRef(null);

  const liveValThroughputRef = useRef(null);
  const peakValThroughputRef = useRef(null);
  const liveValErrorsRef = useRef(null);
  const peakValErrorsRef = useRef(null);

  // Setup Canvas with High-DPI 2x scaling
  const setupCanvas = (canvas) => {
    if (!canvas) return null;
    const dpr = typeof window !== 'undefined' ? (window.devicePixelRatio || 1) : 1;
    const rect = canvas.getBoundingClientRect();
    const height = 135;
    canvas.width = Math.round(rect.width * dpr);
    canvas.height = Math.round(height * dpr);
    const ctx = canvas.getContext('2d');
    ctx.scale(dpr, dpr);
    return { ctx, width: rect.width, height };
  };

  const drawGrid = (ctx, width, height, maxVal, unit = '', padding = { top: 10, bottom: 18, left: 34, right: 10 }) => {
    ctx.clearRect(0, 0, width, height);
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.05)';
    ctx.lineWidth = 1;
    ctx.fillStyle = '#64748b';
    ctx.font = '9px monospace';
    ctx.textAlign = 'right';

    const steps = 3;
    for (let i = 0; i <= steps; i++) {
      const y = padding.top + ((height - padding.top - padding.bottom) / steps) * i;
      const val = Math.round(maxVal - (maxVal / steps) * i);

      ctx.beginPath();
      ctx.moveTo(padding.left, y);
      ctx.lineTo(width - padding.right, y);
      ctx.stroke();

      ctx.fillText(`${val}${unit}`, padding.left - 4, y + 3);
    }
  };

  const drawCrosshair = (ctx, x, height, padding) => {
    ctx.save();
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.2)';
    ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(x, padding.top);
    ctx.lineTo(x, height - padding.bottom);
    ctx.stroke();
    ctx.restore();
  };

  const updateTooltip = (tooltipEl, cardEl, x, y, htmlContent) => {
    if (!tooltipEl) return;
    tooltipEl.innerHTML = htmlContent;
    tooltipEl.style.display = 'block';

    const cardRect = cardEl ? cardEl.getBoundingClientRect() : { width: 300 };
    const tooltipWidth = 170;
    let left = x + 12;
    if (left + tooltipWidth > cardRect.width - 10) {
      left = x - tooltipWidth - 12;
    }
    tooltipEl.style.left = `${Math.max(8, left)}px`;
    tooltipEl.style.top = `${Math.max(10, y - 20)}px`;
  };

  // 1. Draw Throughput Spline Area Chart
  const drawThroughputChart = useCallback(() => {
    const canvas = canvasThroughputRef.current;
    const card = cardThroughputRef.current;
    const tooltip = tooltipThroughputRef.current;
    if (!canvas) return;

    const setup = setupCanvas(canvas);
    if (!setup) return;
    const { ctx, width, height } = setup;

    const data = state.metricsHistory.throughput;
    const timestamps = state.metricsHistory.timestamps;
    if (!data.length) return;

    const peak = Math.max(...data, 1);
    if (liveValThroughputRef.current) liveValThroughputRef.current.textContent = `${data[data.length - 1]} req/s`;
    if (peakValThroughputRef.current) peakValThroughputRef.current.textContent = `peak: ${peak} req/s`;

    const padding = { top: 10, bottom: 18, left: 34, right: 10 };
    const chartW = width - padding.left - padding.right;
    const chartH = height - padding.top - padding.bottom;
    const maxVal = Math.max(...data, 60);

    drawGrid(ctx, width, height, maxVal, '', padding);

    const getX = (i) => padding.left + (chartW / (data.length - 1)) * i;
    const getY = (val) => padding.top + chartH - (val / maxVal) * chartH;

    // Gradient fill
    const grad = ctx.createLinearGradient(0, padding.top, 0, padding.top + chartH);
    grad.addColorStop(0, 'rgba(56, 189, 248, 0.28)');
    grad.addColorStop(1, 'rgba(56, 189, 248, 0.00)');

    ctx.beginPath();
    ctx.moveTo(getX(0), getY(data[0]));
    for (let i = 1; i < data.length; i++) {
      const cx = (getX(i - 1) + getX(i)) / 2;
      ctx.bezierCurveTo(cx, getY(data[i - 1]), cx, getY(data[i]), getX(i), getY(data[i]));
    }
    ctx.lineTo(getX(data.length - 1), padding.top + chartH);
    ctx.lineTo(getX(0), padding.top + chartH);
    ctx.closePath();
    ctx.fillStyle = grad;
    ctx.fill();

    // Spline stroke
    ctx.beginPath();
    ctx.moveTo(getX(0), getY(data[0]));
    for (let i = 1; i < data.length; i++) {
      const cx = (getX(i - 1) + getX(i)) / 2;
      ctx.bezierCurveTo(cx, getY(data[i - 1]), cx, getY(data[i]), getX(i), getY(data[i]));
    }
    ctx.strokeStyle = '#38bdf8';
    ctx.lineWidth = 2;
    ctx.stroke();

    const hIdx = hoveredIndex.throughput;
    if (hIdx >= 0 && hIdx < data.length) {
      const hX = getX(hIdx);
      const hY = getY(data[hIdx]);
      drawCrosshair(ctx, hX, height, padding);

      ctx.beginPath();
      ctx.arc(hX, hY, 4, 0, Math.PI * 2);
      ctx.fillStyle = '#38bdf8';
      ctx.fill();
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 1.5;
      ctx.stroke();

      const val = data[hIdx];
      const time = timestamps[hIdx] || '--:--:--';
      const statusText = val > 40 ? 'Heavy Load' : val > 20 ? 'Optimal' : 'Quiet';
      const statusClass = val > 40 ? 'text-warning' : 'text-success';

      const tooltipHtml = `
        <div class="hud-tooltip-card">
          <div class="hud-tooltip-header">
            <span class="hud-time font-mono">${time}</span>
            <span class="hud-badge status-up">Throughput</span>
          </div>
          <div class="hud-tooltip-grid">
            <div class="hud-row"><span class="hud-label">Requests:</span> <span class="hud-val font-mono">${val} req/s</span></div>
            <div class="hud-row"><span class="hud-label">Window Peak:</span> <span class="hud-val font-mono">${peak} req/s</span></div>
          </div>
          <div class="hud-tooltip-footer">
            <span class="hud-label">Status:</span>
            <span class="hud-status ${statusClass}">${statusText}</span>
          </div>
        </div>
      `;
      updateTooltip(tooltip, card, hX, hY, tooltipHtml);
    } else {
      if (tooltip) tooltip.style.display = 'none';
      const lastX = getX(data.length - 1);
      const lastY = getY(data[data.length - 1]);
      ctx.beginPath();
      ctx.arc(lastX, lastY, 3, 0, Math.PI * 2);
      ctx.fillStyle = '#38bdf8';
      ctx.fill();
    }
  }, [hoveredIndex.throughput]);

  // 2. Draw Error Spikes Bar Chart
  const drawErrorsChart = useCallback(() => {
    const canvas = canvasErrorsRef.current;
    const card = cardErrorsRef.current;
    const tooltip = tooltipErrorsRef.current;
    if (!canvas) return;

    const setup = setupCanvas(canvas);
    if (!setup) return;
    const { ctx, width, height } = setup;

    const data = state.metricsHistory.errors;
    const timestamps = state.metricsHistory.timestamps;
    if (!data.length) return;

    const m = state.getOverviewMetrics();
    const curVal = data[data.length - 1] || 0;
    const prevVal = data[data.length - 2] || 0;

    const trendText = curVal > prevVal ? '↑ Elevated' : curVal < prevVal ? '↓ Lower' : '● Stable';
    if (liveValErrorsRef.current) {
      liveValErrorsRef.current.textContent = `${curVal} err/s`;
      liveValErrorsRef.current.className = `telemetry-live-val ${curVal > 6 ? 'text-danger' : curVal > 0 ? 'text-warning' : 'text-success'}`;
    }
    if (peakValErrorsRef.current) {
      peakValErrorsRef.current.textContent = `${trendText} • Rate: ${m.errorRate}%`;
    }

    const padding = { top: 20, bottom: 20, left: 34, right: 10 };
    const chartW = width - padding.left - padding.right;
    const chartH = height - padding.top - padding.bottom;
    const maxVal = Math.max(...data, 8);
    const peakVal = Math.max(...data);
    const peakIdx = data.lastIndexOf(peakVal);

    drawGrid(ctx, width, height, maxVal, '', padding);

    const dataLen = data.length;
    const slotW = chartW / dataLen;
    const gap = 1.5;
    const barW = Math.max(2.5, slotW - gap);
    const hIdx = hoveredIndex.errors;

    ctx.fillStyle = 'rgba(255, 255, 255, 0.08)';
    ctx.fillRect(padding.left, padding.top + chartH, chartW, 1);

    data.forEach((val, i) => {
      const x = padding.left + slotW * i;
      const isHovered = (i === hIdx);

      if (isHovered) {
        ctx.fillStyle = 'rgba(255, 255, 255, 0.08)';
        ctx.fillRect(x, padding.top, slotW, chartH);
      }

      if (val > 0) {
        const barH = Math.max(4, (val / maxVal) * chartH);
        const y = padding.top + chartH - barH;

        if (val > 6) {
          const grad = ctx.createLinearGradient(0, y, 0, padding.top + chartH);
          grad.addColorStop(0, isHovered ? '#fca5a5' : '#f87171');
          grad.addColorStop(1, isHovered ? '#ef4444' : '#b91c1c');

          ctx.save();
          ctx.shadowColor = 'rgba(239, 68, 68, 0.4)';
          ctx.shadowBlur = isHovered ? 8 : 4;
          ctx.fillStyle = grad;
          ctx.fillRect(x, y, barW, barH);
          ctx.restore();

          ctx.fillStyle = '#ffffff';
          ctx.fillRect(x, y, barW, 1.5);
        } else {
          const grad = ctx.createLinearGradient(0, y, 0, padding.top + chartH);
          grad.addColorStop(0, isHovered ? '#fde68a' : '#fbbf24');
          grad.addColorStop(1, isHovered ? '#f59e0b' : '#b45309');
          ctx.fillStyle = grad;
          ctx.fillRect(x, y, barW, barH);

          ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
          ctx.fillRect(x, y, barW, 1);
        }
      } else {
        ctx.fillStyle = isHovered ? '#34d399' : 'rgba(16, 185, 129, 0.2)';
        ctx.fillRect(x, padding.top + chartH - 1.5, barW, 1.5);
      }
    });

    if (peakVal >= 4 && peakIdx >= 0) {
      const pX = padding.left + slotW * peakIdx + slotW / 2;
      const pBarH = Math.max(4, (peakVal / maxVal) * chartH);
      const pY = padding.top + chartH - pBarH;

      ctx.strokeStyle = '#ef4444';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(pX, pY - 2);
      ctx.lineTo(pX, pY - 6);
      ctx.stroke();

      ctx.beginPath();
      ctx.arc(pX, pY - 6, 2.5, 0, Math.PI * 2);
      ctx.fillStyle = '#ef4444';
      ctx.fill();

      const tagW = 68;
      const tagH = 13;
      const tagX = Math.max(padding.left, Math.min(width - padding.right - tagW, pX - tagW / 2));
      const tagY = pY - 18;

      if (tagY >= 2) {
        ctx.fillStyle = '#080c14';
        ctx.strokeStyle = '#ef4444';
        ctx.lineWidth = 1;
        ctx.fillRect(tagX, tagY, tagW, tagH);
        ctx.strokeRect(tagX, tagY, tagW, tagH);

        ctx.fillStyle = '#f87171';
        ctx.font = 'bold 8px monospace';
        ctx.textAlign = 'center';
        ctx.fillText(`▲ PEAK ${peakVal}e/s`, tagX + tagW / 2, tagY + 9.5);
      }
    }

    ctx.fillStyle = '#64748b';
    ctx.font = '8.5px monospace';
    ctx.textAlign = 'left';
    ctx.fillText(timestamps[0] || '', padding.left, padding.top + chartH + 13);
    ctx.textAlign = 'center';
    const midIdx = Math.floor(dataLen / 2);
    ctx.fillText(timestamps[midIdx] || '', padding.left + chartW / 2, padding.top + chartH + 13);
    ctx.textAlign = 'right';
    ctx.fillText('now', padding.left + chartW, padding.top + chartH + 13);

    if (hIdx >= 0 && hIdx < dataLen) {
      const hX = padding.left + slotW * hIdx + slotW / 2;
      drawCrosshair(ctx, hX, height, padding);

      const val = data[hIdx];
      const time = timestamps[hIdx] || '--:--:--';
      const downService = state.services.find(s => s.status === 'DOWN');
      const slowService = state.services.find(s => s.status === 'SLOW');
      const affectedService = downService ? downService.name : slowService ? slowService.name : 'None (Healthy)';
      const severity = val > 6 ? 'CRITICAL' : val > 0 ? 'WARNING' : 'NOMINAL';
      const severityClass = val > 6 ? 'text-danger' : val > 0 ? 'text-warning' : 'text-success';
      const statusTitle = val > 6 ? 'INCIDENT DETECTED' : val > 0 ? 'DEGRADED TRAFFIC' : 'ALL SYSTEMS NOMINAL';

      const tooltipHtml = `
        <div class="hud-tooltip-card">
          <div class="hud-tooltip-header">
            <span class="hud-time font-mono">${time}</span>
            <span class="hud-badge ${val > 6 ? 'status-down' : val > 0 ? 'status-slow' : 'status-up'}">${severity}</span>
          </div>
          <div class="hud-tooltip-grid">
            <div class="hud-row"><span class="hud-label">ERRORS:</span> <span class="hud-val font-mono ${val > 0 ? 'text-danger' : ''}">${val} err/s</span></div>
            <div class="hud-row"><span class="hud-label">ERROR RATE:</span> <span class="hud-val font-mono">${m.errorRate}%</span></div>
            <div class="hud-row"><span class="hud-label">SEVERITY:</span> <span class="hud-val font-mono ${severityClass}">${severity}</span></div>
            <div class="hud-row"><span class="hud-label">SERVICE:</span> <span class="hud-val font-mono">${affectedService}</span></div>
          </div>
          <div class="hud-tooltip-footer">
            <span class="hud-label">STATUS:</span>
            <span class="hud-status ${severityClass}">${statusTitle}</span>
          </div>
        </div>
      `;
      updateTooltip(tooltip, card, hX, padding.top + chartH / 2, tooltipHtml);
    } else {
      if (tooltip) tooltip.style.display = 'none';
    }
  }, [hoveredIndex.errors]);

  // 3. Draw Latency Trends Dual-Curve Chart
  const drawLatencyChart = useCallback(() => {
    const canvas = canvasLatencyRef.current;
    const card = cardLatencyRef.current;
    const tooltip = tooltipLatencyRef.current;
    if (!canvas) return;

    const setup = setupCanvas(canvas);
    if (!setup) return;
    const { ctx, width, height } = setup;

    const p50Data = state.metricsHistory.latencyP50;
    const p95Data = state.metricsHistory.latencyP95;
    const timestamps = state.metricsHistory.timestamps;
    if (!p50Data.length || !p95Data.length) return;

    const padding = { top: 10, bottom: 18, left: 38, right: 10 };
    const chartW = width - padding.left - padding.right;
    const chartH = height - padding.top - padding.bottom;
    const maxVal = Math.max(...p95Data, ...p50Data, 300);

    drawGrid(ctx, width, height, maxVal, 'ms', padding);

    const getX = (i) => padding.left + (chartW / (p50Data.length - 1)) * i;
    const getY = (val) => padding.top + chartH - (val / maxVal) * chartH;

    // p95 curve (amber)
    ctx.beginPath();
    ctx.moveTo(getX(0), getY(p95Data[0]));
    for (let i = 1; i < p95Data.length; i++) {
      const cx = (getX(i - 1) + getX(i)) / 2;
      ctx.bezierCurveTo(cx, getY(p95Data[i - 1]), cx, getY(p95Data[i]), getX(i), getY(p95Data[i]));
    }
    ctx.strokeStyle = '#f59e0b';
    ctx.lineWidth = 1.8;
    ctx.stroke();

    // p50 curve (cyan)
    ctx.beginPath();
    ctx.moveTo(getX(0), getY(p50Data[0]));
    for (let i = 1; i < p50Data.length; i++) {
      const cx = (getX(i - 1) + getX(i)) / 2;
      ctx.bezierCurveTo(cx, getY(p50Data[i - 1]), cx, getY(p50Data[i]), getX(i), getY(p50Data[i]));
    }
    ctx.strokeStyle = '#06b6d4';
    ctx.lineWidth = 1.8;
    ctx.stroke();

    const hIdx = hoveredIndex.latency;
    if (hIdx >= 0 && hIdx < p50Data.length) {
      const hX = getX(hIdx);
      const yP50 = getY(p50Data[hIdx]);
      const yP95 = getY(p95Data[hIdx]);

      drawCrosshair(ctx, hX, height, padding);

      ctx.beginPath();
      ctx.arc(hX, yP50, 3.5, 0, Math.PI * 2);
      ctx.fillStyle = '#06b6d4';
      ctx.fill();
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 1;
      ctx.stroke();

      ctx.beginPath();
      ctx.arc(hX, yP95, 3.5, 0, Math.PI * 2);
      ctx.fillStyle = '#f59e0b';
      ctx.fill();
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 1;
      ctx.stroke();

      const vP50 = p50Data[hIdx];
      const vP95 = p95Data[hIdx];
      const time = timestamps[hIdx] || '--:--:--';
      const statusText = vP95 > 600 ? 'High Jitter / Tail Lag' : vP95 > 250 ? 'Moderate' : 'Optimal Low Latency';
      const statusClass = vP95 > 600 ? 'text-warning' : 'text-success';

      const tooltipHtml = `
        <div class="hud-tooltip-card">
          <div class="hud-tooltip-header">
            <span class="hud-time font-mono">${time}</span>
            <span class="hud-badge status-slow">Latency HUD</span>
          </div>
          <div class="hud-tooltip-grid">
            <div class="hud-row"><span class="hud-label" style="color: #06b6d4;">p50 Median:</span> <span class="hud-val font-mono">${formatMs(vP50)}</span></div>
            <div class="hud-row"><span class="hud-label" style="color: #f59e0b;">p95 Tail:</span> <span class="hud-val font-mono">${formatMs(vP95)}</span></div>
          </div>
          <div class="hud-tooltip-footer">
            <span class="hud-label">Health:</span>
            <span class="hud-status ${statusClass}">${statusText}</span>
          </div>
        </div>
      `;
      updateTooltip(tooltip, card, hX, (yP50 + yP95) / 2, tooltipHtml);
    } else {
      if (tooltip) tooltip.style.display = 'none';
    }
  }, [hoveredIndex.latency]);

  const renderAllCharts = useCallback(() => {
    drawThroughputChart();
    drawErrorsChart();
    drawLatencyChart();
  }, [drawThroughputChart, drawErrorsChart, drawLatencyChart]);

  useEffect(() => {
    renderAllCharts();

    const unsubMetrics = state.subscribe('metrics', renderAllCharts);
    const unsubTick = state.subscribe('tick', renderAllCharts);

    const handleResize = () => renderAllCharts();
    window.addEventListener('resize', handleResize);

    return () => {
      unsubMetrics();
      unsubTick();
      window.removeEventListener('resize', handleResize);
    };
  }, [renderAllCharts]);

  const handleMouseMove = (e, key, paddingLeft, paddingRight) => {
    const canvas = e.currentTarget;
    const rect = canvas.getBoundingClientRect();
    const mouseX = e.clientX - rect.left;
    const chartW = rect.width - paddingLeft - paddingRight;
    const dataLen = state.metricsHistory.throughput.length;
    if (dataLen <= 1) return;

    let index;
    if (key === 'errors') {
      index = Math.floor(((mouseX - paddingLeft) / chartW) * dataLen);
    } else {
      index = Math.round(((mouseX - paddingLeft) / chartW) * (dataLen - 1));
    }
    index = Math.max(0, Math.min(dataLen - 1, index));

    setHoveredIndex(prev => ({ ...prev, [key]: index }));
  };

  const handleMouseLeave = (key) => {
    setHoveredIndex(prev => ({ ...prev, [key]: -1 }));
  };

  const handleRangeChange = (range) => {
    setTimeRange(range);
    state.setMetricsTimeRange(range);
  };

  return (
    <>
      <div className="section-header">
        <div className="section-title-wrap">
          <div className="section-icon">
            <Icon name="activity" />
          </div>
          <div>
            <h2 className="section-title">Telemetry & Metrics</h2>
            <p className="section-desc">Zero-dependency Canvas visualizers for gateway throughput, error spikes, and upstream latency distribution</p>
          </div>
        </div>
        <div className="metrics-header-controls">
          <div className="time-range-pills" role="radiogroup" aria-label="Metrics Time Range">
            {['5m', '15m', '1h'].map(r => (
              <button
                key={r}
                className={`btn btn-xs range-btn ${timeRange === r ? 'active' : ''}`}
                onClick={() => handleRangeChange(r)}
                data-range={r}
              >
                {r}
              </button>
            ))}
          </div>
          <div className="charts-badge">
            <span className="live-indicator"></span>
            <span>Live 24-Point Buffer</span>
          </div>
        </div>
      </div>

      <div className="telemetry-grid">
        {/* 1. Throughput Chart Panel */}
        <div className="telemetry-card" id="card-throughput" ref={cardThroughputRef}>
          <div className="telemetry-card-header">
            <div className="telemetry-title-group">
              <span className="telemetry-title">Request Throughput</span>
              <span className="telemetry-subtitle">Gateway requests per second</span>
            </div>
            <div className="telemetry-stat-box">
              <span className="telemetry-live-val text-info" id="live-val-throughput" ref={liveValThroughputRef}>-- req/s</span>
              <span className="telemetry-peak-val" id="peak-val-throughput" ref={peakValThroughputRef}>peak: --</span>
            </div>
          </div>
          <div className="telemetry-canvas-wrap">
            <canvas
              ref={canvasThroughputRef}
              id="canvas-throughput"
              height="135"
              onMouseMove={(e) => handleMouseMove(e, 'throughput', 34, 10)}
              onMouseLeave={() => handleMouseLeave('throughput')}
            />
            <div className="chart-hud-tooltip" id="tooltip-throughput" ref={tooltipThroughputRef} />
          </div>
        </div>

        {/* 2. Error Spikes Chart Panel */}
        <div className="telemetry-card" id="card-errors" ref={cardErrorsRef}>
          <div className="telemetry-card-header">
            <div className="telemetry-title-group">
              <span className="telemetry-title">Error Spikes & Incidents</span>
              <span className="telemetry-subtitle">4xx / 5xx frequency • Live timeline</span>
            </div>
            <div className="telemetry-stat-box">
              <span className="telemetry-live-val text-danger" id="live-val-errors" ref={liveValErrorsRef}>-- err/s</span>
              <span className="telemetry-peak-val" id="peak-val-errors" ref={peakValErrorsRef}>Rate: --</span>
            </div>
          </div>
          <div className="telemetry-canvas-wrap">
            <canvas
              ref={canvasErrorsRef}
              id="canvas-errors"
              height="135"
              onMouseMove={(e) => handleMouseMove(e, 'errors', 34, 10)}
              onMouseLeave={() => handleMouseLeave('errors')}
            />
            <div className="chart-hud-tooltip" id="tooltip-errors" ref={tooltipErrorsRef} />
          </div>
        </div>

        {/* 3. Latency Trends Chart Panel */}
        <div className="telemetry-card" id="card-latency" ref={cardLatencyRef}>
          <div className="telemetry-card-header">
            <div className="telemetry-title-group">
              <span className="telemetry-title">Latency Trends</span>
              <span className="telemetry-subtitle">p50 median & p95 tail response times</span>
            </div>
            <div className="telemetry-latency-legend">
              <span className="legend-tag p50"><span className="legend-dot-cyan"></span> p50</span>
              <span className="legend-tag p95"><span className="legend-dot-amber"></span> p95</span>
            </div>
          </div>
          <div className="telemetry-canvas-wrap">
            <canvas
              ref={canvasLatencyRef}
              id="canvas-latency"
              height="135"
              onMouseMove={(e) => handleMouseMove(e, 'latency', 38, 10)}
              onMouseLeave={() => handleMouseLeave('latency')}
            />
            <div className="chart-hud-tooltip" id="tooltip-latency" ref={tooltipLatencyRef} />
          </div>
        </div>
      </div>
    </>
  );
}
