/**
 * Analytics Dashboard - Main Entry Point
 */

import '../css/main.css';
import api, { APIError } from './api.js';
import { LineChart, createLineChart, createSparkline } from './charts.js';
import {
  DatePicker,
  SortableTable,
  MetricCard,
  showLoading,
  hideLoading,
  initThemeToggle,
  AutoRefresh,
  createStatusBadge,
  ApiKeyModal
} from './components.js';
import {
  getDaysAgo,
  getToday,
  formatNumber,
  formatPercent,
  formatDuration,
  truncate,
  escapeHtml,
  updateURLParams,
  getURLParams,
  storage
} from './utils.js';

// ============================================================================
// Dashboard Application
// ============================================================================

class Dashboard {
  constructor() {
    this.state = {
      startDate: getDaysAgo(30),
      endDate: getToday(),
      isLoading: false,
      error: null,
      data: null
    };

    this.autoRefresh = new AutoRefresh(() => this.loadData(), {
      interval: 60000, // 1 minute
      enabled: false
    });

    this.charts = {};
    this.tables = {};
    this.metrics = {};

    // Initialize API key modal
    this.apiKeyModal = new ApiKeyModal({
      title: 'Clé API requise',
      message: 'Entrez votre clé API de tableau de bord pour accéder aux statistiques.',
      onSave: (key, remember) => {
        api.setApiKey(key, remember);
        this.loadData();
      }
    });

    // Set up auth callback
    api.onAuthRequired(() => {
      this.apiKeyModal.show();
    });

    this.#init();
  }

  async #init() {
    // Initialize theme toggle
    initThemeToggle();

    // Initialize date picker
    this.#initDatePicker();

    // Initialize UI components
    this.#initMetrics();
    this.#initCharts();
    this.#initTables();
    this.#initActions();

    // Parse URL params for initial state
    this.#loadFromURL();

    // Load initial data
    await this.loadData();
  }

  #initDatePicker() {
    const datePickerContainer = document.getElementById('date-picker');
    if (!datePickerContainer) return;

    this.datePicker = new DatePicker({
      startDate: this.state.startDate,
      endDate: this.state.endDate,
      onChange: (start, end) => {
        this.state.startDate = start;
        this.state.endDate = end;
        updateURLParams({ start, end });
        this.loadData();
      }
    });

    datePickerContainer.appendChild(this.datePicker.element);
  }

  #initMetrics() {
    // Pageviews metric
    const pageviewsEl = document.getElementById('metric-pageviews');
    if (pageviewsEl) {
      this.metrics.pageviews = new MetricCard({
        label: 'Vues',
        value: 0,
        previousValue: null,
        format: formatNumber
      });
      this.metrics.pageviews.mount(pageviewsEl);
    }

    // Sessions metric
    const sessionsEl = document.getElementById('metric-sessions');
    if (sessionsEl) {
      this.metrics.sessions = new MetricCard({
        label: 'Sessions',
        value: 0,
        previousValue: null,
        format: formatNumber
      });
      this.metrics.sessions.mount(sessionsEl);
    }

    // Visitors metric
    const visitorsEl = document.getElementById('metric-visitors');
    if (visitorsEl) {
      this.metrics.visitors = new MetricCard({
        label: 'Visiteurs uniques',
        value: 0,
        previousValue: null,
        format: formatNumber
      });
      this.metrics.visitors.mount(visitorsEl);
    }

    // Engagement metric
    const engagementEl = document.getElementById('metric-engagement');
    if (engagementEl) {
      this.metrics.engagement = new MetricCard({
        label: 'Engagement moyen',
        value: 0,
        previousValue: null,
        format: (v) => formatDuration(v)
      });
      this.metrics.engagement.mount(engagementEl);
    }
  }

  #initCharts() {
    // Timeseries chart
    const chartContainer = document.getElementById('timeseries-chart');
    if (chartContainer) {
      this.charts.timeseries = createLineChart(chartContainer, [], {
        padding: { top: 10, right: 0, bottom: 30, left: 50 }
      });
    }
  }

  #initTables() {
    // Pages table
    const pagesTableEl = document.getElementById('pages-table');
    if (pagesTableEl) {
      this.tables.pages = new SortableTable(
        [
          {
            key: 'url',
            label: 'Page',
            sortable: true,
            truncate: true,
            render: (value) => {
              const safeUrl = (!value.startsWith('/') && !value.startsWith('http')) ? '#' : escapeHtml(value);
              return `<a href="${safeUrl}" class="text-truncate" style="max-width: 300px; display: block;">${escapeHtml(value)}</a>`;
            }
          },
          {
            key: 'pageviews',
            label: 'Vues',
            sortable: true,
            className: 'table__cell--numeric',
            format: formatNumber
          },
          {
            key: 'sessions',
            label: 'Sessions',
            sortable: true,
            className: 'table__cell--numeric',
            format: formatNumber
          },
          {
            key: 'avgEngagement',
            label: 'Temps moy.',
            sortable: true,
            className: 'table__cell--numeric',
            format: (v) => formatDuration(v)
          }
        ],
        { initialSort: { column: 'pageviews', direction: 'desc' } }
      );
      this.tables.pages.mount(pagesTableEl);
    }

    // Events table
    const eventsTableEl = document.getElementById('events-table');
    if (eventsTableEl) {
      this.tables.events = new SortableTable(
        [
          {
            key: 'eventName',
            label: 'Événement',
            sortable: true,
            render: (value) => `<span class="badge badge--primary">${escapeHtml(value)}</span>`
          },
          {
            key: 'count',
            label: 'Nombre',
            sortable: true,
            className: 'table__cell--numeric',
            format: formatNumber
          }
        ],
        { initialSort: { column: 'count', direction: 'desc' } }
      );
      this.tables.events.mount(eventsTableEl);
    }
  }

  #initActions() {
    // Refresh button
    const refreshBtn = document.getElementById('refresh-btn');
    if (refreshBtn) {
      refreshBtn.addEventListener('click', () => this.loadData());
    }

    // Auto-refresh toggle
    const autoRefreshBtn = document.getElementById('autorefresh-btn');
    if (autoRefreshBtn) {
      autoRefreshBtn.addEventListener('click', () => {
        const isActive = this.autoRefresh.toggle();
        autoRefreshBtn.classList.toggle('header__btn--active', isActive);
      });
    }

    // API key button - show modal with current key
    const apiKeyBtn = document.getElementById('api-key-btn');
    if (apiKeyBtn) {
      // Update button style if key is set
      if (api.hasApiKey()) {
        apiKeyBtn.classList.add('header__btn--active');
      }

      apiKeyBtn.addEventListener('click', () => {
        // Pre-fill with current key if exists
        if (api.hasApiKey()) {
          // Show a modal with options to clear or change the key
          this.#showApiKeyMenu();
        } else {
          this.apiKeyModal.show();
        }
      });
    }
  }

  #showApiKeyMenu() {
    const currentKey = api.getApiKey();
    const hasStoredKey = localStorage.getItem('analytics_api_key');

    // Create a simple confirm-like modal for key management
    const backdrop = document.createElement('div');
    backdrop.className = 'modal-backdrop';
    backdrop.style.cssText = `
      position: fixed; inset: 0; background: rgba(0,0,0,0.5);
      display: flex; align-items: center; justify-content: center;
      z-index: var(--z-modal);
    `;

    const modal = document.createElement('div');
    modal.className = 'modal';
    modal.style.cssText = `
      background: var(--color-bg-elevated);
      border: 1px solid var(--color-border);
      border-radius: var(--radius-lg);
      box-shadow: var(--shadow-xl);
      padding: var(--space-xl);
      width: 100%; max-width: 400px; margin: var(--space-md);
    `;

    modal.innerHTML = `
      <h2 style="font-size: var(--text-xl); margin-bottom: var(--space-md);">Clé API</h2>
      <p style="color: var(--color-text-muted); margin-bottom: var(--space-lg);">
        Une clé API de tableau de bord est actuellement configurée${hasStoredKey ? ' et enregistrée' : ''}.
      </p>
      <div class="flex" style="gap: var(--space-sm); justify-content: flex-end;">
        <button class="btn btn--secondary" data-action="change">Changer la clé</button>
        <button class="btn btn--secondary" data-action="clear" style="color: var(--color-error);">Effacer</button>
        <button class="btn btn--primary" data-action="close">Fermer</button>
      </div>
    `;

    modal.querySelector('[data-action="change"]').addEventListener('click', () => {
      backdrop.remove();
      this.apiKeyModal.show();
    });

    modal.querySelector('[data-action="clear"]').addEventListener('click', () => {
      api.clearApiKey();
      backdrop.remove();
      apiKeyBtn?.classList.remove('header__btn--active');
      this.loadData(); // Reload to show auth prompt if needed
    });

    modal.querySelector('[data-action="close"]').addEventListener('click', () => {
      backdrop.remove();
    });

    backdrop.addEventListener('click', (e) => {
      if (e.target === backdrop) backdrop.remove();
    });

    backdrop.appendChild(modal);
    document.body.appendChild(backdrop);
  }

  #loadFromURL() {
    const params = getURLParams();
    if (params.start) this.state.startDate = params.start;
    if (params.end) this.state.endDate = params.end;

    if (this.datePicker) {
      this.datePicker.setRange(this.state.startDate, this.state.endDate);
    }
  }

  async loadData() {
    if (this.state.isLoading) return;

    this.state.isLoading = true;
    this.#setLoadingState(true);
    this.#clearError();

    try {
      const data = await api.getAll(
        this.state.startDate,
        this.state.endDate
      );

      if (data.errors.summary || data.errors.timeseries ||
          data.errors.pages || data.errors.events) {
        this.#handlePartialError(data.errors);
      }

      this.state.data = data;
      this.#render(data);

    } catch (error) {
      console.error('Failed to load dashboard data:', error);
      this.#showError(error.message || 'Failed to load data');
    } finally {
      this.state.isLoading = false;
      this.#setLoadingState(false);
    }
  }

  #render(data) {
    // Render metrics
    if (data.summary) {
      this.#renderMetrics(data.summary);
    }

    // Render timeseries chart
    if (data.timeseries?.data) {
      this.#renderTimeseries(data.timeseries.data);
    }

    // Render tables
    if (data.pages?.pages) {
      this.tables.pages?.setData(data.pages.pages);
    }

    if (data.events?.events) {
      this.tables.events?.setData(data.events.events);
    }
  }

  #renderMetrics(summary) {
    this.metrics.pageviews?.update(summary.pageviews);
    this.metrics.sessions?.update(summary.sessions);
    this.metrics.visitors?.update(summary.uniqueVisitors);
    this.metrics.engagement?.update(summary.avgEngagement);
  }

  #renderTimeseries(timeseriesData) {
    if (!this.charts.timeseries) return;

    // Convert data to chart format
    const chartData = timeseriesData.map(d => ({
      date: d.date,
      value: d.pageviews
    }));

    this.charts.timeseries.setData(chartData);
  }

  #setLoadingState(loading) {
    const refreshBtn = document.getElementById('refresh-btn');
    if (refreshBtn) {
      if (loading) {
        refreshBtn.disabled = true;
        refreshBtn.innerHTML = `
          <div class="spinner spinner--sm"></div>
        `;
      } else {
        refreshBtn.disabled = false;
        refreshBtn.innerHTML = `
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M23 4v6h-6M1 20v-6h6"/>
            <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
          </svg>
        `;
      }
    }
  }

  #clearError() {
    const errorContainer = document.getElementById('error-alert');
    if (errorContainer) {
      errorContainer.innerHTML = '';
      errorContainer.classList.add('d-none');
    }
  }

  #showError(message) {
    const errorContainer = document.getElementById('error-alert');
    if (!errorContainer) return;

    errorContainer.innerHTML = `
      <div class="alert alert--error">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
        <span>${escapeHtml(message)}</span>
      </div>
    `;
    errorContainer.classList.remove('d-none');
  }

  #handlePartialError(errors) {
    const messages = [];

    if (errors.summary) messages.push('résumé');
    if (errors.timeseries) messages.push('série chronologique');
    if (errors.pages) messages.push('pages');
    if (errors.events) messages.push('événements');

    if (messages.length > 0) {
      this.#showError(`Erreur partielle : Impossible de charger ${messages.join(', ')}`);
    }
  }
}

// ============================================================================
// Initialize Dashboard
// ============================================================================

let dashboard;

document.addEventListener('DOMContentLoaded', () => {
  dashboard = new Dashboard();
});

// Export for testing/debugging
window.Dashboard = Dashboard;
window.dashboard = () => dashboard;
