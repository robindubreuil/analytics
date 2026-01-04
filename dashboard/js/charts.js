/**
 * SVG Chart Components
 * Lightweight, vanilla JS charting library
 */

import { formatNumber } from './utils.js';

const SVG_NS = 'http://www.w3.org/2000/svg';

/**
 * Create SVG element with attributes
 */
function createSVGElement(tag, attrs = {}) {
  const el = document.createElementNS(SVG_NS, tag);
  Object.entries(attrs).forEach(([key, value]) => {
    if (key === 'className') {
      el.setAttribute('class', value);
    } else if (key.startsWith('data-')) {
      el.setAttribute(key, String(value));
    } else {
      el.setAttribute(key, String(value));
    }
  });
  return el;
}

/**
 * Base Chart class
 */
class Chart {
  constructor(container, options = {}) {
    this.container = typeof container === 'string'
      ? document.querySelector(container)
      : container;

    if (!this.container) {
      throw new Error(`Chart container not found: ${container}`);
    }

    this.options = {
      padding: { top: 20, right: 20, bottom: 40, left: 60 },
      ...options
    };

    this.data = [];
    this.svg = null;
    this.tooltip = null;
    this.dimensions = { width: 0, height: 0 };
    this.scales = { x: null, y: null };

    this.#init();
  }

  #init() {
    // Create SVG element
    this.svg = createSVGElement('svg', {
      className: 'chart__svg',
      preserveAspectRatio: 'none'
    });

    // Create tooltip
    this.tooltip = document.createElement('div');
    this.tooltip.className = 'chart__tooltip';
    document.body.appendChild(this.tooltip);

    // Get initial dimensions
    this.#updateDimensions();

    // Observe resize
    const resizeObserver = new ResizeObserver(() => {
      this.#updateDimensions();
      this.render();
    });
    resizeObserver.observe(this.container);

    // Clear container and append SVG
    this.container.innerHTML = '';
    this.container.appendChild(this.svg);

    // Add loading state
    this.container.classList.add('chart');
  }

  #updateDimensions() {
    const rect = this.container.getBoundingClientRect();
    this.dimensions = {
      width: rect.width || 600,
      height: rect.height || 300
    };
  }

  get chartWidth() {
    return this.dimensions.width - this.options.padding.left - this.options.padding.right;
  }

  get chartHeight() {
    return this.dimensions.height - this.options.padding.top - this.options.padding.bottom;
  }

  setData(data) {
    this.data = data;
    this.render();
    return this;
  }

  render() {
    // Override in subclass
    this.svg.innerHTML = '';
  }

  destroy() {
    if (this.tooltip && this.tooltip.parentNode) {
      this.tooltip.parentNode.removeChild(this.tooltip);
    }
  }

  /**
   * Create x and y scales
   */
  #createScales() {
    throw new Error('Must implement #createScales in subclass');
  }

  /**
   * Show tooltip at position
   */
  showTooltip(x, y, content, anchorRect = null) {
    this.tooltip.innerHTML = content;
    this.tooltip.classList.add('chart__tooltip--visible');

    let left, top;

    if (anchorRect) {
      left = anchorRect.left + anchorRect.width / 2;
      top = anchorRect.top - 8;
    } else {
      const containerRect = this.container.getBoundingClientRect();
      left = containerRect.left + x;
      top = containerRect.top + y;
    }

    const tooltipRect = this.tooltip.getBoundingClientRect();
    const padding = 8;

    // Adjust horizontal position
    if (left + tooltipRect.width / 2 > window.innerWidth - padding) {
      left = window.innerWidth - tooltipRect.width / 2 - padding;
    } else if (left - tooltipRect.width / 2 < padding) {
      left = tooltipRect.width / 2 + padding;
    }

    // Adjust vertical position
    if (top - tooltipRect.height < padding) {
      top = anchorRect ? anchorRect.bottom + 8 : top + tooltipRect.height + padding;
    }

    this.tooltip.style.left = `${left - tooltipRect.width / 2}px`;
    this.tooltip.style.top = `${top - tooltipRect.height}px`;
  }

  hideTooltip() {
    this.tooltip.classList.remove('chart__tooltip--visible');
  }
}

/**
 * Line Chart
 */
export class LineChart extends Chart {
  #xScale = null;
  #yScale = null;

  constructor(container, options = {}) {
    super(container, {
      ...options,
      smooth: options.smooth ?? true
    });
  }

  #createScales() {
    if (this.data.length === 0) return;

    const values = this.data.map(d => d.value);
    const minValue = Math.min(...values, 0);
    const maxValue = Math.max(...values);

    const xDomain = this.data.map(d => d.date);
    const yDomain = [minValue, maxValue];

    this.#xScale = (index) => {
      return (index / (this.data.length - 1 || 1)) * this.chartWidth;
    };

    this.#yScale = (value) => {
      const range = yDomain[1] - yDomain[0] || 1;
      return this.chartHeight - ((value - yDomain[0]) / range) * this.chartHeight;
    };

    return { xDomain, yDomain };
  }

  render() {
    this.svg.innerHTML = '';

    if (this.data.length === 0) {
      this.renderEmpty();
      return;
    }

    const { xDomain, yDomain } = this.#createScales();

    // Create group for chart content
    const g = createSVGElement('g', {
      transform: `translate(${this.options.padding.left}, ${this.options.padding.top})`
    });

    // Draw grid lines (horizontal)
    this.#drawGrid(g, yDomain);

    // Draw area (gradient fill)
    this.#drawArea(g);

    // Draw line
    this.#drawLine(g);

    // Draw dots
    this.#drawDots(g);

    // Draw axes labels
    this.#drawLabels(g, xDomain, yDomain);

    this.svg.appendChild(g);
  }

  #drawGrid(g, yDomain) {
    const gridCount = 5;
    for (let i = 0; i <= gridCount; i++) {
      const value = yDomain[0] + (yDomain[1] - yDomain[0]) * (i / gridCount);
      const y = this.#yScale(value);

      const line = createSVGElement('line', {
        className: 'chart__grid',
        x1: 0,
        y1: y,
        x2: this.chartWidth,
        y2: y
      });
      g.appendChild(line);
    }
  }

  #drawArea(g) {
    const points = this.data.map((d, i) => {
      return `${this.#xScale(i)},${this.#yScale(d.value)}`;
    });

    const areaPath = createSVGElement('path', {
      className: 'chart__area',
      d: `M ${this.#xScale(0)},${this.chartHeight} L ${points.join(' L ')} L ${this.#xScale(this.data.length - 1)},${this.chartHeight} Z`
    });
    g.appendChild(areaPath);
  }

  #drawLine(g) {
    if (this.data.length === 0) return;

    const points = this.data.map((d, i) => {
      return `${this.#xScale(i)},${this.#yScale(d.value)}`;
    });

    const linePath = createSVGElement('path', {
      className: 'chart__line',
      d: `M ${points.join(' L ')}`
    });
    g.appendChild(linePath);
  }

  #drawDots(g) {
    this.data.forEach((d, i) => {
      const x = this.#xScale(i);
      const y = this.#yScale(d.value);

      const circle = createSVGElement('circle', {
        className: 'chart__dot',
        cx: x,
        cy: y,
        'data-date': d.date,
        'data-value': d.value
      });

      // Tooltip interaction
      circle.addEventListener('mouseenter', (e) => {
        this.showTooltip(x, y, `
          <div class="chart__tooltip-label">${formatDateShort(d.date)}</div>
          <div class="chart__tooltip-value">${formatNumber(d.value)}</div>
        `, e.target.getBoundingClientRect());
      });

      circle.addEventListener('mouseleave', () => {
        this.hideTooltip();
      });

      g.appendChild(circle);
    });
  }

  #drawLabels(g, xDomain, yDomain) {
    // Y-axis labels
    const gridCount = 5;
    for (let i = 0; i <= gridCount; i++) {
      const value = yDomain[0] + (yDomain[1] - yDomain[0]) * (i / gridCount);
      const y = this.#yScale(value);

      const text = createSVGElement('text', {
        className: 'chart__label chart__label--y',
        x: -10,
        y: y,
        'text-anchor': 'end'
      });
      text.textContent = formatCompactNumber(value);
      g.appendChild(text);
    }

    // X-axis labels (show first, middle, last)
    const xLabels = [0, Math.floor(xDomain.length / 2), xDomain.length - 1];
    xLabels.forEach((i, idx) => {
      if (i >= 0 && i < xDomain.length) {
        const x = this.#xScale(i);

        const text = createSVGElement('text', {
          className: 'chart__label chart__label--x',
          x: x,
          y: this.chartHeight + 20
        });
        text.textContent = formatDateShort(xDomain[i]);
        g.appendChild(text);
      }
    });
  }

  renderEmpty() {
    const text = createSVGElement('text', {
      x: this.dimensions.width / 2,
      y: this.dimensions.height / 2,
      'text-anchor': 'middle',
      className: 'chart__label'
    });
    text.textContent = 'Aucune donnée disponible';
    this.svg.appendChild(text);
  }
}

/**
 * Bar Chart
 */
export class BarChart extends Chart {
  constructor(container, options = {}) {
    super(container, {
      ...options,
      barPadding: options.barPadding ?? 0.2
    });
  }

  #createScales() {
    if (this.data.length === 0) return;

    const values = this.data.map(d => d.value);
    const maxValue = Math.max(...values, 0);

    this.#xScale = (index) => {
      const barWidth = this.chartWidth / this.data.length;
      return barWidth * index;
    };

    this.#yScale = (value) => {
      return this.chartHeight - (value / maxValue) * this.chartHeight;
    };

    const barWidth = this.chartWidth / this.data.length;
    return { maxValue, barWidth };
  }

  #xScale = null;
  #yScale = null;

  render() {
    this.svg.innerHTML = '';

    if (this.data.length === 0) {
      this.renderEmpty();
      return;
    }

    const { maxValue, barWidth } = this.#createScales();
    const actualBarWidth = barWidth * (1 - this.options.barPadding);
    const barOffset = barWidth * this.options.barPadding / 2;

    const g = createSVGElement('g', {
      transform: `translate(${this.options.padding.left}, ${this.options.padding.top})`
    });

    // Draw grid lines
    const gridCount = 5;
    for (let i = 0; i <= gridCount; i++) {
      const value = maxValue * (i / gridCount);
      const y = this.#yScale(value);

      const line = createSVGElement('line', {
        className: 'chart__grid',
        x1: 0,
        y1: y,
        x2: this.chartWidth,
        y2: y
      });
      g.appendChild(line);
    }

    // Draw bars
    this.data.forEach((d, i) => {
      const x = this.#xScale(i) + barOffset;
      const y = this.#yScale(d.value);
      const height = this.chartHeight - y;

      const rect = createSVGElement('rect', {
        className: 'chart__bar',
        x: x,
        y: y,
        width: actualBarWidth,
        height: height,
        rx: 2
      });

      rect.addEventListener('mouseenter', (e) => {
        this.showTooltip(x, y, `
          <div class="chart__tooltip-label">${d.label || d.date}</div>
          <div class="chart__tooltip-value">${formatNumber(d.value)}</div>
        `, e.target.getBoundingClientRect());
      });

      rect.addEventListener('mouseleave', () => {
        this.hideTooltip();
      });

      g.appendChild(rect);
    });

    // Draw axes labels
    for (let i = 0; i <= gridCount; i++) {
      const value = maxValue * (i / gridCount);
      const y = this.#yScale(value);

      const text = createSVGElement('text', {
        className: 'chart__label chart__label--y',
        x: -10,
        y: y,
        'text-anchor': 'end'
      });
      text.textContent = formatCompactNumber(value);
      g.appendChild(text);
    }

    this.svg.appendChild(g);
  }

  renderEmpty() {
    const text = createSVGElement('text', {
      x: this.dimensions.width / 2,
      y: this.dimensions.height / 2,
      'text-anchor': 'middle',
      className: 'chart__label'
    });
    text.textContent = 'Aucune donnée disponible';
    this.svg.appendChild(text);
  }
}

/**
 * Mini Sparkline (for inline trend indicators)
 */
export class Sparkline {
  constructor(container, options = {}) {
    this.container = typeof container === 'string'
      ? document.querySelector(container)
      : container;

    if (!this.container) {
      throw new Error(`Sparkline container not found: ${container}`);
    }

    this.options = {
      width: options.width || 60,
      height: options.height || 20,
      strokeWidth: options.strokeWidth || 1.5,
      ...options
    };

    this.data = [];
  }

  setData(data) {
    this.data = data;
    this.render();
    return this;
  }

  render() {
    if (this.data.length < 2) return;

    const values = this.data;
    const min = Math.min(...values);
    const max = Math.max(...values);
    const range = max - min || 1;

    const points = values.map((v, i) => {
      const x = (i / (values.length - 1)) * this.options.width;
      const y = this.options.height - ((v - min) / range) * this.options.height;
      return `${x},${y}`;
    }).join(' ');

    const svg = createSVGElement('svg', {
      className: 'sparkline',
      width: this.options.width,
      height: this.options.height,
      viewBox: `0 0 ${this.options.width} ${this.options.height}`,
      overflow: 'visible'
    });

    // Determine trend color
    const trend = values[values.length - 1] >= values[0] ? 'up' : 'down';
    svg.classList.add(`sparkline--${trend}`);

    const path = createSVGElement('path', {
      d: `M ${points}`,
      fill: 'none',
      stroke: 'currentColor',
      'stroke-width': this.options.strokeWidth,
      'stroke-linecap': 'round',
      'stroke-linejoin': 'round'
    });

    svg.appendChild(path);

    this.container.innerHTML = '';
    this.container.appendChild(svg);
  }
}

// Helper functions
function formatDateShort(dateStr) {
  const date = new Date(dateStr);
  if (isNaN(date)) return dateStr;
  return date.toLocaleDateString('fr-FR', { month: 'short', day: 'numeric' });
}

function formatCompactNumber(num) {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
  return num.toString();
}

// Export factory function for easy creation
export function createLineChart(container, data, options = {}) {
  const chart = new LineChart(container, options);
  if (data) chart.setData(data);
  return chart;
}

export function createBarChart(container, data, options = {}) {
  const chart = new BarChart(container, options);
  if (data) chart.setData(data);
  return chart;
}

export function createSparkline(container, data, options = {}) {
  const sparkline = new Sparkline(container, options);
  if (data) sparkline.setData(data);
  return sparkline;
}
