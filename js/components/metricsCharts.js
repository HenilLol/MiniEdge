/**
 * MiniEdge Component - Telemetry & Analytics Dashboard
 * Zero-dependency HTML5 Canvas visualizers with High-DPI 2x retina scaling,
 * interactive vertical crosshairs, point/bar snapping, time-range selection,
 * and universal rich HUD tooltips.
 */

import { getIcon } from '../utils/svgIcons.js';
import { formatMs } from '../utils/formatters.js';

export function renderMetricsCharts(container, state) {
  // Active hover states per chart
  let hoveredIndex = {
    throughput: -1,
    errors: -1,
    latency: -1
  };

  container.innerHTML = `
    <div class="section-header">
      <div class="section-title-wrap">
        <div class="section-icon">${getIcon('activity')}</div>
        <div>
          <h2 class="section-title">Telemetry & Metrics</h2>
          <p class="section-desc">Zero-dependency Canvas visualizers for gateway throughput, error spikes, and upstream latency distribution</p>
        </div>
      </div>
      <div class="metrics-header-controls">
        <div class="time-range-pills" role="radiogroup" aria-label="Metrics Time Range">
          <button class="btn btn-xs range-btn ${state.metricsTimeRange === '5m' ? 'active' : ''}" data-range="5m">5m</button>
          <button class="btn btn-xs range-btn ${state.metricsTimeRange === '15m' ? 'active' : ''}" data-range="15m">15m</button>
          <button class="btn btn-xs range-btn ${state.metricsTimeRange === '1h' ? 'active' : ''}" data-range="1h">1h</button>
        </div>
        <div class="charts-badge">
          <span class="live-indicator"></span>
          <span>Live 24-Point Buffer</span>
        </div>
      </div>
    </div>

    <div class="telemetry-grid">
      <!-- 1. Throughput Chart Panel -->
      <div class="telemetry-card" id="card-throughput">
        <div class="telemetry-card-header">
          <div class="telemetry-title-group">
            <span class="telemetry-title">Request Throughput</span>
            <span class="telemetry-subtitle">Gateway requests per second</span>
          </div>
          <div class="telemetry-stat-box">
            <span class="telemetry-live-val text-info" id="live-val-throughput">-- req/s</span>
            <span class="telemetry-peak-val" id="peak-val-throughput">peak: --</span>
          </div>
        </div>
        <div class="telemetry-canvas-wrap">
          <canvas id="canvas-throughput" height="135"></canvas>
          <div class="chart-hud-tooltip" id="tooltip-throughput"></div>
        </div>
      </div>

      <!-- 2. Error Spikes Chart Panel -->
      <div class="telemetry-card" id="card-errors">
        <div class="telemetry-card-header">
          <div class="telemetry-title-group">
            <span class="telemetry-title">Error Spikes & Incidents</span>
            <span class="telemetry-subtitle">4xx / 5xx frequency • Live timeline</span>
          </div>
          <div class="telemetry-stat-box">
            <span class="telemetry-live-val text-danger" id="live-val-errors">-- err/s</span>
            <span class="telemetry-peak-val" id="peak-val-errors">Rate: --</span>
          </div>
        </div>
        <div class="telemetry-canvas-wrap">
          <canvas id="canvas-errors" height="135"></canvas>
          <div class="chart-hud-tooltip" id="tooltip-errors"></div>
        </div>
      </div>

      <!-- 3. Latency Trends Chart Panel -->
      <div class="telemetry-card" id="card-latency">
        <div class="telemetry-card-header">
          <div class="telemetry-title-group">
            <span class="telemetry-title">Latency Trends</span>
            <span class="telemetry-subtitle">p50 median & p95 tail response times</span>
          </div>
          <div class="telemetry-latency-legend">
            <span class="legend-tag p50"><span class="legend-dot-cyan"></span> p50</span>
            <span class="legend-tag p95"><span class="legend-dot-amber"></span> p95</span>
          </div>
        </div>
        <div class="telemetry-canvas-wrap">
          <canvas id="canvas-latency" height="135"></canvas>
          <div class="chart-hud-tooltip" id="tooltip-latency"></div>
        </div>
      </div>
    </div>
  `;

  // Canvas helper with High-DPI 2x scaling
  function setupCanvas(canvas) {
    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.getBoundingClientRect();
    const height = 135;
    canvas.width = Math.round(rect.width * dpr);
    canvas.height = Math.round(height * dpr);
    const ctx = canvas.getContext('2d');
    ctx.scale(dpr, dpr);
    return { ctx, width: rect.width, height };
  }

  // Draw Grid Lines & Y-Axis Scale
  function drawGrid(ctx, width, height, maxVal, unit = '', padding = { top: 10, bottom: 18, left: 34, right: 10 }) {
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
  }

  // Draw Crosshair Vertical Guide
  function drawCrosshair(ctx, x, height, padding) {
    ctx.save();
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.2)';
    ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(x, padding.top);
    ctx.lineTo(x, height - padding.bottom);
    ctx.stroke();
    ctx.restore();
  }

  // Update HUD Tooltip Position & HTML
  function updateTooltip(tooltipEl, cardEl, x, y, htmlContent) {
    if (!tooltipEl) return;
    tooltipEl.innerHTML = htmlContent;
    tooltipEl.style.display = 'block';

    const cardRect = cardEl.getBoundingClientRect();
    const tooltipRect = tooltipEl.getBoundingClientRect();

    let left = x + 12;
    let top = y - 20;

    // Flip horizontally if near right boundary
    if (left + tooltipRect.width > cardRect.width - 10) {
      left = x - tooltipRect.width - 12;
    }
    // Constrain vertically
    if (top < 10) top = 10;
    if (top + tooltipRect.height > cardRect.height - 10) {
      top = cardRect.height - tooltipRect.height - 10;
    }

    tooltipEl.style.left = `${Math.max(6, left)}px`;
    tooltipEl.style.top = `${Math.max(6, top)}px`;
  }

  // 1. Draw Throughput Area Chart
  function drawThroughputChart() {
    const canvas = container.querySelector('#canvas-throughput');
    const card = container.querySelector('#card-throughput');
    const tooltip = container.querySelector('#tooltip-throughput');
    if (!canvas) return;

    const { ctx, width, height } = setupCanvas(canvas);
    const data = state.metricsHistory.throughput;
    const timestamps = state.metricsHistory.timestamps;
    if (!data.length) return;

    const liveValEl = container.querySelector('#live-val-throughput');
    const peakValEl = container.querySelector('#peak-val-throughput');
    const peak = Math.max(...data);
    if (liveValEl) liveValEl.textContent = `${data[data.length - 1]} req/s`;
    if (peakValEl) peakValEl.textContent = `peak: ${peak} req/s`;

    const padding = { top: 10, bottom: 18, left: 34, right: 10 };
    const chartW = width - padding.left - padding.right;
    const chartH = height - padding.top - padding.bottom;
    const maxVal = Math.max(...data, 40);

    drawGrid(ctx, width, height, maxVal, '', padding);

    const getX = (i) => padding.left + (chartW / (data.length - 1 || 1)) * i;
    const getY = (v) => padding.top + chartH - (v / maxVal) * chartH;

    // Gradient Area Fill
    const gradient = ctx.createLinearGradient(0, padding.top, 0, height - padding.bottom);
    gradient.addColorStop(0, 'rgba(56, 189, 248, 0.28)');
    gradient.addColorStop(1, 'rgba(56, 189, 248, 0.0)');

    ctx.beginPath();
    ctx.moveTo(getX(0), getY(data[0]));
    for (let i = 1; i < data.length; i++) {
      const xc = (getX(i) + getX(i - 1)) / 2;
      const yc = (getY(data[i]) + getY(data[i - 1])) / 2;
      ctx.quadraticCurveTo(getX(i - 1), getY(data[i - 1]), xc, yc);
    }
    ctx.lineTo(getX(data.length - 1), getY(data[data.length - 1]));
    ctx.lineTo(getX(data.length - 1), padding.top + chartH);
    ctx.lineTo(getX(0), padding.top + chartH);
    ctx.closePath();
    ctx.fillStyle = gradient;
    ctx.fill();

    // Spline Line
    ctx.beginPath();
    ctx.moveTo(getX(0), getY(data[0]));
    for (let i = 1; i < data.length; i++) {
      const xc = (getX(i) + getX(i - 1)) / 2;
      const yc = (getY(data[i]) + getY(data[i - 1])) / 2;
      ctx.quadraticCurveTo(getX(i - 1), getY(data[i - 1]), xc, yc);
    }
    ctx.lineTo(getX(data.length - 1), getY(data[data.length - 1]));
    ctx.strokeStyle = '#38bdf8';
    ctx.lineWidth = 1.8;
    ctx.stroke();

    // Hover Highlight & Crosshair
    const hIdx = hoveredIndex.throughput;
    if (hIdx >= 0 && hIdx < data.length) {
      const hX = getX(hIdx);
      const hY = getY(data[hIdx]);

      drawCrosshair(ctx, hX, height, padding);

      // Outer glow circle
      ctx.beginPath();
      ctx.arc(hX, hY, 6, 0, Math.PI * 2);
      ctx.fillStyle = 'rgba(56, 189, 248, 0.3)';
      ctx.fill();

      // Center solid dot
      ctx.beginPath();
      ctx.arc(hX, hY, 3.5, 0, Math.PI * 2);
      ctx.fillStyle = '#ffffff';
      ctx.fill();
      ctx.strokeStyle = '#0284c7';
      ctx.lineWidth = 1.5;
      ctx.stroke();

      const time = timestamps[hIdx] || '--:--:--';
      const val = data[hIdx];
      const statusText = val >= peak * 0.85 ? 'Peak Traffic' : val <= 15 ? 'Low Volume' : 'Nominal';
      const statusClass = val >= peak * 0.85 ? 'text-info' : 'text-success';

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

      // Default last dot
      const lastX = getX(data.length - 1);
      const lastY = getY(data[data.length - 1]);
      ctx.beginPath();
      ctx.arc(lastX, lastY, 3, 0, Math.PI * 2);
      ctx.fillStyle = '#38bdf8';
      ctx.fill();
    }
  }

  // 2. Draw Error Spikes & Incident Timeline Chart
  function drawErrorsChart() {
    const canvas = container.querySelector('#canvas-errors');
    const card = container.querySelector('#card-errors');
    const tooltip = container.querySelector('#tooltip-errors');
    if (!canvas) return;

    const { ctx, width, height } = setupCanvas(canvas);
    const data = state.metricsHistory.errors;
    const timestamps = state.metricsHistory.timestamps;
    if (!data.length) return;

    const liveValEl = container.querySelector('#live-val-errors');
    const peakValEl = container.querySelector('#peak-val-errors');
    const m = state.getOverviewMetrics();
    const curVal = data[data.length - 1] || 0;
    const prevVal = data[data.length - 2] || 0;

    // Trend indicator
    const trendText = curVal > prevVal ? '↑ Elevated' : curVal < prevVal ? '↓ Lower' : '● Stable';
    if (liveValEl) {
      liveValEl.textContent = `${curVal} err/s`;
      liveValEl.className = `telemetry-live-val ${curVal > 6 ? 'text-danger' : curVal > 0 ? 'text-warning' : 'text-success'}`;
    }
    if (peakValEl) {
      peakValEl.textContent = `${trendText} • Rate: ${m.errorRate}%`;
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
    const gap = 1.5; // Small consistent 1.5px gap between adjacent bars
    const barW = Math.max(2.5, slotW - gap);
    const hIdx = hoveredIndex.errors;

    // Continuous baseline line
    ctx.fillStyle = 'rgba(255, 255, 255, 0.08)';
    ctx.fillRect(padding.left, padding.top + chartH, chartW, 1);

    // Render bars
    data.forEach((val, i) => {
      const x = padding.left + slotW * i;
      const isHovered = (i === hIdx);

      if (isHovered) {
        // Translucent column hover guide behind the slot
        ctx.fillStyle = 'rgba(255, 255, 255, 0.08)';
        ctx.fillRect(x, padding.top, slotW, chartH);
      }

      if (val > 0) {
        const barH = Math.max(4, (val / maxVal) * chartH);
        const y = padding.top + chartH - barH;

        if (val > 6) {
          // Critical 5xx spike (Crimson gradient + subtle glow)
          const grad = ctx.createLinearGradient(0, y, 0, padding.top + chartH);
          grad.addColorStop(0, isHovered ? '#fca5a5' : '#f87171');
          grad.addColorStop(1, isHovered ? '#ef4444' : '#b91c1c');
          
          ctx.save();
          ctx.shadowColor = 'rgba(239, 68, 68, 0.4)';
          ctx.shadowBlur = isHovered ? 8 : 4;
          ctx.fillStyle = grad;
          ctx.fillRect(x, y, barW, barH);
          ctx.restore();

          // Crisp top cap
          ctx.fillStyle = '#ffffff';
          ctx.fillRect(x, y, barW, 1.5);
        } else {
          // Warning 4xx error (Amber gradient)
          const grad = ctx.createLinearGradient(0, y, 0, padding.top + chartH);
          grad.addColorStop(0, isHovered ? '#fde68a' : '#fbbf24');
          grad.addColorStop(1, isHovered ? '#f59e0b' : '#b45309');
          ctx.fillStyle = grad;
          ctx.fillRect(x, y, barW, barH);

          // Top cap
          ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
          ctx.fillRect(x, y, barW, 1);
        }
      } else {
        // Quiet Zero-error period: subtle 1.5px baseline marker preserving continuous timeline
        ctx.fillStyle = isHovered ? '#34d399' : 'rgba(16, 185, 129, 0.2)';
        ctx.fillRect(x, padding.top + chartH - 1.5, barW, 1.5);
      }
    });

    // Peak Incident Pin / Anomaly Callout (only if significant spike >= 4 err/s)
    if (peakVal >= 4 && peakIdx >= 0) {
      const pX = padding.left + slotW * peakIdx + slotW / 2;
      const pBarH = Math.max(4, (peakVal / maxVal) * chartH);
      const pY = padding.top + chartH - pBarH;

      // Small vertical pointer tick
      ctx.strokeStyle = '#ef4444';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(pX, pY - 2);
      ctx.lineTo(pX, pY - 6);
      ctx.stroke();

      // Beacon Dot
      ctx.beginPath();
      ctx.arc(pX, pY - 6, 2.5, 0, Math.PI * 2);
      ctx.fillStyle = '#ef4444';
      ctx.fill();

      // Incident Flag Pill
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

    // Time Axis: clean timestamps along bottom
    ctx.fillStyle = '#64748b';
    ctx.font = '8.5px monospace';
    ctx.textAlign = 'left';
    ctx.fillText(timestamps[0] || '', padding.left, padding.top + chartH + 13);
    ctx.textAlign = 'center';
    const midIdx = Math.floor(dataLen / 2);
    ctx.fillText(timestamps[midIdx] || '', padding.left + chartW / 2, padding.top + chartH + 13);
    ctx.textAlign = 'right';
    ctx.fillText('now', padding.left + chartW, padding.top + chartH + 13);

    // Interactive Hover & HUD Tooltip
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
  }

  // 3. Draw Latency Profile Line Chart (p50 & p95)
  function drawLatencyChart() {
    const canvas = container.querySelector('#canvas-latency');
    const card = container.querySelector('#card-latency');
    const tooltip = container.querySelector('#tooltip-latency');
    if (!canvas) return;

    const { ctx, width, height } = setupCanvas(canvas);
    const p50 = state.metricsHistory.latencyP50;
    const p95 = state.metricsHistory.latencyP95;
    const timestamps = state.metricsHistory.timestamps;
    if (!p50.length) return;

    const padding = { top: 10, bottom: 18, left: 38, right: 10 };
    const chartW = width - padding.left - padding.right;
    const chartH = height - padding.top - padding.bottom;
    const maxVal = Math.max(...p95, 150);

    drawGrid(ctx, width, height, maxVal, 'ms', padding);

    const getX = (i) => padding.left + (chartW / (p50.length - 1 || 1)) * i;
    const getY = (v) => padding.top + chartH - (v / maxVal) * chartH;

    // p95 Line (Amber)
    ctx.beginPath();
    ctx.moveTo(getX(0), getY(p95[0]));
    for (let i = 1; i < p95.length; i++) {
      ctx.lineTo(getX(i), getY(p95[i]));
    }
    ctx.strokeStyle = '#f59e0b';
    ctx.lineWidth = 1.6;
    ctx.stroke();

    // p50 Line (Cyan)
    ctx.beginPath();
    ctx.moveTo(getX(0), getY(p50[0]));
    for (let i = 1; i < p50.length; i++) {
      ctx.lineTo(getX(i), getY(p50[i]));
    }
    ctx.strokeStyle = '#06b6d4';
    ctx.lineWidth = 1.6;
    ctx.stroke();

    // Hover Highlight & Dual-Point Snapping
    const hIdx = hoveredIndex.latency;
    if (hIdx >= 0 && hIdx < p50.length) {
      const hX = getX(hIdx);
      const yP50 = getY(p50[hIdx]);
      const yP95 = getY(p95[hIdx]);

      drawCrosshair(ctx, hX, height, padding);

      // p95 dot
      ctx.beginPath();
      ctx.arc(hX, yP95, 4, 0, Math.PI * 2);
      ctx.fillStyle = '#f59e0b';
      ctx.fill();
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 1.5;
      ctx.stroke();

      // p50 dot
      ctx.beginPath();
      ctx.arc(hX, yP50, 4, 0, Math.PI * 2);
      ctx.fillStyle = '#06b6d4';
      ctx.fill();
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 1.5;
      ctx.stroke();

      const time = timestamps[hIdx] || '--:--:--';
      const val50 = p50[hIdx];
      const val95 = p95[hIdx];
      const diff = val95 - val50;
      const statusText = val95 > 400 ? 'High Latency Tail' : val95 > 150 ? 'Degraded' : 'Optimal Response';
      const statusClass = val95 > 400 ? 'text-danger' : val95 > 150 ? 'text-warning' : 'text-success';

      const tooltipHtml = `
        <div class="hud-tooltip-card">
          <div class="hud-tooltip-header">
            <span class="hud-time font-mono">${time}</span>
            <span class="hud-badge status-slow">Latency Distribution</span>
          </div>
          <div class="hud-tooltip-grid">
            <div class="hud-row"><span class="hud-label">p50 Median:</span> <span class="hud-val font-mono text-info">${formatMs(val50)}</span></div>
            <div class="hud-row"><span class="hud-label">p95 Tail:</span> <span class="hud-val font-mono text-warning">${formatMs(val95)}</span></div>
            <div class="hud-row"><span class="hud-label">Spread (Diff):</span> <span class="hud-val font-mono">+${diff}ms</span></div>
          </div>
          <div class="hud-tooltip-footer">
            <span class="hud-label">Status:</span>
            <span class="hud-status ${statusClass}">${statusText}</span>
          </div>
        </div>
      `;
      updateTooltip(tooltip, card, hX, (yP50 + yP95) / 2, tooltipHtml);
    } else {
      if (tooltip) tooltip.style.display = 'none';
    }
  }

  function renderAllCharts() {
    drawThroughputChart();
    drawErrorsChart();
    drawLatencyChart();
  }

  // Attach Canvas Mouse Interaction Handlers
  function attachChartHoverListeners() {
    const bindHover = (canvasId, key, drawFn) => {
      const canvas = container.querySelector(canvasId);
      if (!canvas) return;

      const paddingLeft = key === 'latency' ? 38 : 34;
      const paddingRight = 10;

      canvas.addEventListener('mousemove', (e) => {
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

        if (hoveredIndex[key] !== index) {
          hoveredIndex[key] = index;
          drawFn();
        }
      });

      canvas.addEventListener('mouseleave', () => {
        hoveredIndex[key] = -1;
        drawFn();
      });
    };

    bindHover('#canvas-throughput', 'throughput', drawThroughputChart);
    bindHover('#canvas-errors', 'errors', drawErrorsChart);
    bindHover('#canvas-latency', 'latency', drawLatencyChart);
  }

  // Time-Range switchers
  container.querySelectorAll('.range-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const range = btn.getAttribute('data-range');
      state.setMetricsTimeRange(range);
      container.querySelectorAll('.range-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
    });
  });

  setTimeout(() => {
    renderAllCharts();
    attachChartHoverListeners();
  }, 40);

  state.subscribe('metrics', renderAllCharts);
  state.subscribe('tick', renderAllCharts);

  window.addEventListener('resize', renderAllCharts);
}
