/**
 * Dashboard UI Components
 */

import { getToday, getDaysAgo, toISODate, formatNumber, formatPercent, formatDuration, truncate, escapeHtml } from './utils.js';

// ============================================================================
// Date Range Picker
// ============================================================================

export class DatePicker {
  constructor(options = {}) {
    this.options = {
      presets: ['today', '7d', '30d', '90d'],
      defaultPreset: '30d',
      onChange: null,
      ...options
    };

    this.startDate = this.options.startDate || getDaysAgo(30);
    this.endDate = this.options.endDate || getToday();

    this.element = this.#render();
  }

  #render() {
    const container = document.createElement('div');
    container.className = 'date-picker';

    // Presets
    const presetsEl = document.createElement('div');
    presetsEl.className = 'date-picker__presets';

    this.options.presets.forEach(preset => {
      const btn = document.createElement('button');
      btn.className = 'date-picker__preset';
      btn.textContent = this.#getPresetLabel(preset);
      btn.dataset.preset = preset;

      if (preset === this.options.defaultPreset) {
        btn.classList.add('date-picker__preset--active');
      }

      btn.addEventListener('click', () => this.#selectPreset(preset, btn));
      presetsEl.appendChild(btn);
    });

    container.appendChild(presetsEl);

    // Date inputs
    const inputsEl = document.createElement('div');
    inputsEl.className = 'date-picker__inputs';

    const startInput = document.createElement('input');
    startInput.type = 'date';
    startInput.className = 'date-picker__input';
    startInput.value = this.startDate;
    startInput.setAttribute('aria-label', 'Date de début');

    const separator = document.createElement('span');
    separator.className = 'date-picker__separator';
    separator.textContent = '—';

    const endInput = document.createElement('input');
    endInput.type = 'date';
    endInput.className = 'date-picker__input';
    endInput.value = this.endDate;
    endInput.setAttribute('aria-label', 'Date de fin');

    startInput.addEventListener('change', () => {
      this.startDate = startInput.value;
      this.#clearActivePreset();
      this.#notifyChange();
    });

    endInput.addEventListener('change', () => {
      this.endDate = endInput.value;
      this.#clearActivePreset();
      this.#notifyChange();
    });

    inputsEl.append(startInput, separator, endInput);
    container.appendChild(inputsEl);

    this.container = container;
    this.startInput = startInput;
    this.endInput = endInput;
    this.presetsEl = presetsEl;

    return container;
  }

  #getPresetLabel(preset) {
    const labels = {
      today: 'Aujourd\'hui',
      '7d': '7 jours',
      '30d': '30 jours',
      '90d': '90 jours',
      '1y': '1 an'
    };
    return labels[preset] || preset;
  }

  #selectPreset(preset, btn) {
    this.#clearActivePreset();
    btn.classList.add('date-picker__preset--active');

    const today = new Date();

    switch (preset) {
      case 'today':
        this.startDate = getToday();
        this.endDate = getToday();
        break;
      case '7d':
        this.startDate = getDaysAgo(7);
        this.endDate = getToday();
        break;
      case '30d':
        this.startDate = getDaysAgo(30);
        this.endDate = getToday();
        break;
      case '90d':
        this.startDate = getDaysAgo(90);
        this.endDate = getToday();
        break;
      case '1y':
        this.startDate = getDaysAgo(365);
        this.endDate = getToday();
        break;
    }

    this.startInput.value = this.startDate;
    this.endInput.value = this.endDate;
    this.#notifyChange();
  }

  #clearActivePreset() {
    const active = this.presetsEl.querySelector('.date-picker__preset--active');
    if (active) active.classList.remove('date-picker__preset--active');
  }

  #notifyChange() {
    if (this.options.onChange) {
      this.options.onChange(this.startDate, this.endDate);
    }
  }

  getRange() {
    return { start: this.startDate, end: this.endDate };
  }

  setRange(start, end) {
    this.startDate = start;
    this.endDate = end;
    this.startInput.value = start;
    this.endInput.value = end;
    this.#clearActivePreset();
  }

  mount(parent) {
    const container = typeof parent === 'string'
      ? document.querySelector(parent)
      : parent;
    container.appendChild(this.element);
    return this;
  }
}

// ============================================================================
// Sortable Table
// ============================================================================

export class SortableTable {
  constructor(columns, options = {}) {
    this.columns = columns;
    this.options = {
      emptyMessage: 'Aucune donnée disponible',
      initialSort: null,
      ...options
    };

    this.data = [];
    this.sortColumn = null;
    this.sortDirection = 'asc';

    if (this.options.initialSort) {
      this.sortColumn = this.options.initialSort.column;
      this.sortDirection = this.options.initialSort.direction || 'asc';
    }

    this.element = this.#render();
  }

  #render() {
    const wrapper = document.createElement('div');
    wrapper.className = 'table-wrapper';

    const table = document.createElement('table');
    table.className = 'table';

    // Header
    const thead = document.createElement('thead');
    const headerRow = document.createElement('tr');

    this.columns.forEach(col => {
      const th = document.createElement('th');
      th.textContent = col.label;
      th.dataset.column = col.key;

      if (col.sortable !== false) {
        th.dataset.sortable = '';
        th.addEventListener('click', () => this.#sort(col.key));
      }

      if (col.key === this.sortColumn) {
        th.dataset.sorted = this.sortDirection;
        th.innerHTML = `${col.label} ${this.#getSortIcon(this.sortDirection)}`;
      }

      if (col.className) {
        th.classList.add(col.className);
      }

      headerRow.appendChild(th);
    });

    thead.appendChild(headerRow);
    table.appendChild(thead);

    // Body
    this.tbody = document.createElement('tbody');
    table.appendChild(this.tbody);

    wrapper.appendChild(table);
    this.tableElement = table;

    return wrapper;
  }

  #getSortIcon(direction) {
    return direction === 'asc'
      ? '<span class="sort-icon">↑</span>'
      : '<span class="sort-icon">↓</span>';
  }

  #sort(column) {
    if (this.sortColumn === column) {
      this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      this.sortColumn = column;
      this.sortDirection = 'asc';
    }

    // Update header
    const th = this.tableElement.querySelector(`th[data-column="${column}"]`);
    const allTh = this.tableElement.querySelectorAll('th[data-sortable]');
    allTh.forEach(t => {
      delete t.dataset.sorted;
      const label = this.columns.find(c => c.key === t.dataset.column)?.label || '';
      t.textContent = label;
    });

    if (th) {
      th.dataset.sorted = this.sortDirection;
      th.innerHTML = `${this.columns.find(c => c.key === column)?.label} ${this.#getSortIcon(this.sortDirection)}`;
    }

    this.render();
  }

  #getSortedData() {
    if (!this.sortColumn) return this.data;

    return [...this.data].sort((a, b) => {
      const aVal = a[this.sortColumn];
      const bVal = b[this.sortColumn];

      let comparison = 0;
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        comparison = aVal - bVal;
      } else {
        comparison = String(aVal).localeCompare(String(bVal));
      }

      return this.sortDirection === 'asc' ? comparison : -comparison;
    });
  }

  setData(data) {
    this.data = data;
    this.render();
    return this;
  }

  render() {
    this.tbody.innerHTML = '';

    const sortedData = this.#getSortedData();

    if (sortedData.length === 0) {
      const tr = document.createElement('tr');
      const td = document.createElement('td');
      td.colSpan = this.columns.length;
      td.className = 'table__empty';
      td.textContent = this.options.emptyMessage;
      tr.appendChild(td);
      this.tbody.appendChild(tr);
      return;
    }

    sortedData.forEach(row => {
      const tr = document.createElement('tr');

      this.columns.forEach(col => {
        const td = document.createElement('td');
        const value = row[col.key];

        if (col.render) {
          td.innerHTML = col.render(value, row);
        } else if (col.format) {
          td.textContent = col.format(value);
        } else {
          td.textContent = value ?? '-';
        }

        if (col.className) {
          td.classList.add(...col.className.split(' '));
        }

        if (col.truncate) {
          td.classList.add('table__cell--truncate');
          td.title = value;
        }

        tr.appendChild(td);
      });

      this.tbody.appendChild(tr);
    });
  }

  mount(parent) {
    const container = typeof parent === 'string'
      ? document.querySelector(parent)
      : parent;
    container.appendChild(this.element);
    return this;
  }
}

// ============================================================================
// Metric Card
// ============================================================================

export class MetricCard {
  constructor(config) {
    this.config = {
      label: '',
      value: null,
      previousValue: null,
      icon: null,
      format: formatNumber,
      ...config
    };

    this.element = this.#render();
  }

  #render() {
    const card = document.createElement('div');
    card.className = 'card';

    const body = document.createElement('div');
    body.className = 'card__body';

    const metric = document.createElement('div');
    metric.className = 'metric';

    // Label
    const label = document.createElement('div');
    label.className = 'metric__label';
    label.textContent = this.config.label;

    // Value row
    const valueRow = document.createElement('div');
    valueRow.className = 'flex items-end gap-sm';

    // Value
    const value = document.createElement('div');
    value.className = 'metric__value';
    value.textContent = this.config.format(this.config.value);

    // Change indicator
    const change = document.createElement('div');
    change.className = 'metric__change';

    if (this.config.previousValue !== null) {
      const percentChange = this.config.previousValue !== 0
        ? ((this.config.value - this.config.previousValue) / this.config.previousValue) * 100
        : 0;

      const isPositive = percentChange > 0;
      const isNegative = percentChange < 0;

      change.classList.add(
        isPositive ? 'metric__change--positive' :
        isNegative ? 'metric__change--negative' :
        'metric__change--neutral'
      );

      const icon = isPositive ? '↑' : isNegative ? '↓' : '–';
      change.textContent = `${icon} ${Math.abs(percentChange).toFixed(1)}%`;
    }

    valueRow.append(value, change);

    metric.append(label, valueRow);
    body.appendChild(metric);
    card.appendChild(body);

    this.valueElement = value;
    this.changeElement = change;

    return card;
  }

  update(value, previousValue = null) {
    this.config.value = value;
    this.config.previousValue = previousValue !== undefined ? previousValue : this.config.previousValue;

    this.valueElement.textContent = this.config.format(this.config.value);

    if (this.config.previousValue !== null) {
      const percentChange = this.config.previousValue !== 0
        ? ((this.config.value - this.config.previousValue) / this.config.previousValue) * 100
        : 0;

      const isPositive = percentChange > 0;
      const isNegative = percentChange < 0;

      this.changeElement.classList.remove(
        'metric__change--positive',
        'metric__change--negative',
        'metric__change--neutral'
      );

      this.changeElement.classList.add(
        isPositive ? 'metric__change--positive' :
        isNegative ? 'metric__change--negative' :
        'metric__change--neutral'
      );

      const icon = isPositive ? '↑' : isNegative ? '↓' : '–';
      this.changeElement.textContent = `${icon} ${Math.abs(percentChange).toFixed(1)}%`;
    }

    return this;
  }

  mount(parent) {
    const container = typeof parent === 'string'
      ? document.querySelector(parent)
      : parent;
    container.appendChild(this.element);
    return this;
  }
}

// ============================================================================
// Loading State
// ============================================================================

export function showLoading(container) {
  const el = typeof container === 'string'
    ? document.querySelector(container)
    : container;

  if (!el) return;

  el.innerHTML = '';
  el.classList.add('chart--loading');

  const spinner = document.createElement('div');
  spinner.className = 'chart__spinner';
  el.appendChild(spinner);
}

export function hideLoading(container) {
  const el = typeof container === 'string'
    ? document.querySelector(container)
    : container;

  if (!el) return;

  el.classList.remove('chart--loading');
  const spinner = el.querySelector('.chart__spinner');
  if (spinner) spinner.remove();
}

// ============================================================================
// Theme Toggle
// ============================================================================

export function initThemeToggle() {
  const toggle = document.getElementById('theme-toggle');
  if (!toggle) return;

  // Get saved theme or system preference
  const savedTheme = localStorage.getItem('theme');
  const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches;

  let currentTheme = savedTheme || (systemDark ? 'dark' : 'light');
  document.documentElement.setAttribute('data-theme', currentTheme);

  toggle.addEventListener('click', () => {
    currentTheme = currentTheme === 'light' ? 'dark' : 'light';
    document.documentElement.setAttribute('data-theme', currentTheme);
    localStorage.setItem('theme', currentTheme);
  });
}

// ============================================================================
// Auto Refresh
// ============================================================================

export class AutoRefresh {
  constructor(callback, options = {}) {
    this.callback = callback;
    this.options = {
      interval: 30000, // 30 seconds
      enabled: false,
      ...options
    };

    this.timer = null;
  }

  start() {
    if (this.timer) return;
    this.options.enabled = true;
    this.timer = setInterval(() => {
      this.callback();
    }, this.options.interval);
  }

  stop() {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    this.options.enabled = false;
  }

  toggle() {
    if (this.options.enabled) {
      this.stop();
    } else {
      this.start();
    }
    return this.options.enabled;
  }

  setInterval(ms) {
    this.options.interval = ms;
    if (this.options.enabled) {
      this.stop();
      this.start();
    }
  }

  isActive() {
    return this.options.enabled;
  }
}

// ============================================================================
// Status Badge
// ============================================================================

export function createStatusBadge(status) {
  const badge = document.createElement('span');
  badge.className = 'badge';

  const statusMap = {
    healthy: { class: 'badge--success', text: 'Sain' },
    unhealthy: { class: 'badge--error', text: 'Non sain' },
    loading: { class: 'badge--info', text: 'Chargement...' },
    error: { class: 'badge--error', text: 'Erreur' }
  };

  const config = statusMap[status] || statusMap.loading;
  badge.classList.add(config.class);
  badge.textContent = config.text;

  return badge;
}

// ============================================================================
// API Key Modal
// ============================================================================

export class ApiKeyModal {
  constructor(options = {}) {
    this.options = {
      title: 'Clé API requise',
      message: 'Entrez votre clé API pour accéder au tableau de bord.',
      onSave: null,
      allowCancel: true,
      ...options
    };

    this.element = null;
    this.backdrop = null;
    this.input = null;
    this.rememberCheckbox = null;
  }

  show() {
    if (this.element) return; // Already shown

    // Create backdrop
    this.backdrop = document.createElement('div');
    this.backdrop.className = 'modal-backdrop';
    this.backdrop.style.cssText = `
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.5);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: var(--z-modal);
      opacity: 0;
      transition: opacity 0.2s ease;
    `;

    // Create modal
    this.element = document.createElement('div');
    this.element.className = 'modal';
    this.element.style.cssText = `
      background: var(--color-bg-elevated);
      border: 1px solid var(--color-border);
      border-radius: var(--radius-lg);
      box-shadow: var(--shadow-xl);
      padding: var(--space-xl);
      width: 100%;
      max-width: 400px;
      margin: var(--space-md);
      transform: scale(0.95);
      transition: transform 0.2s ease;
    `;

    // Modal content
    this.element.innerHTML = `
      <div class="modal__header" style="margin-bottom: var(--space-md);">
        <h2 style="font-size: var(--text-xl); font-weight: var(--font-semibold); margin: 0;">${escapeHtml(this.options.title)}</h2>
      </div>
      <div class="modal__body">
        <p style="color: var(--color-text-muted); margin-bottom: var(--space-lg);">${escapeHtml(this.options.message)}</p>
        <div class="form-group" style="margin-bottom: var(--space-md);">
          <label for="api-key-input" style="display: block; font-size: var(--text-sm); font-weight: var(--font-medium); margin-bottom: var(--space-xs);">Clé API</label>
          <input
            type="password"
            id="api-key-input"
            class="input"
            placeholder="Entrez votre clé API"
            autocomplete="current-password"
            style="width: 100%; padding: var(--space-sm) var(--space-md); border: 1px solid var(--color-border); border-radius: var(--radius-md); font-family: var(--font-mono); font-size: var(--text-sm);"
          />
        </div>
        <div class="form-group" style="margin-bottom: var(--space-lg);">
          <label class="checkbox-label" style="display: flex; align-items: center; gap: var(--space-sm); cursor: pointer; font-size: var(--text-sm);">
            <input type="checkbox" id="remember-key" style="width: 16px; height: 16px;" />
            <span>Se souvenir de cette clé</span>
          </label>
        </div>
      </div>
      <div class="modal__footer" style="display: flex; gap: var(--space-sm); justify-content: flex-end;">
        ${this.options.allowCancel ? '<button type="button" class="btn btn--secondary" data-action="cancel">Annuler</button>' : ''}
        <button type="button" class="btn btn--primary" data-action="save">Enregistrer la clé</button>
      </div>
    `;

    // Get references
    this.input = this.element.querySelector('#api-key-input');
    this.rememberCheckbox = this.element.querySelector('#remember-key');

    // Add event listeners
    const saveBtn = this.element.querySelector('[data-action="save"]');
    const cancelBtn = this.element.querySelector('[data-action="cancel"]');

    saveBtn.addEventListener('click', () => this.#save());
    if (cancelBtn) {
      cancelBtn.addEventListener('click', () => this.hide());
    }

    // Enter key to save
    this.input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') this.#save();
    });

    // Escape to close
    document.addEventListener('keydown', this.#handleEscape);

    // Click backdrop to close (if allowed)
    if (this.options.allowCancel) {
      this.backdrop.addEventListener('click', (e) => {
        if (e.target === this.backdrop) this.hide();
      });
    }

    // Append to DOM
    this.backdrop.appendChild(this.element);
    document.body.appendChild(this.backdrop);

    // Trigger animation
    requestAnimationFrame(() => {
      this.backdrop.style.opacity = '1';
      this.element.style.transform = 'scale(1)';
    });

    // Focus input
    this.input.focus();
  }

  #handleEscape = (e) => {
    if (e.key === 'Escape' && this.options.allowCancel) {
      this.hide();
    }
  };

  #save() {
    const key = this.input.value.trim();
    const remember = this.rememberCheckbox.checked;

    if (!key) {
      this.input.focus();
      this.input.style.borderColor = 'var(--color-error)';
      return;
    }

    if (this.options.onSave) {
      this.options.onSave(key, remember);
    }

    this.hide();
  }

  hide() {
    if (!this.element) return;

    // Animate out
    this.backdrop.style.opacity = '0';
    this.element.style.transform = 'scale(0.95)';

    setTimeout(() => {
      if (this.backdrop && this.backdrop.parentNode) {
        this.backdrop.parentNode.removeChild(this.backdrop);
      }
      document.removeEventListener('keydown', this.#handleEscape);
      this.element = null;
      this.backdrop = null;
    }, 200);
  }

  setValue(value) {
    if (this.input) {
      this.input.value = value;
    }
  }

  setRemember(checked) {
    if (this.rememberCheckbox) {
      this.rememberCheckbox.checked = checked;
    }
  }
}
