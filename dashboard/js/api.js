/**
 * API client for analytics dashboard
 */

const DEFAULT_BASE_URL = '/api/dashboard';
const DEFAULT_TIMEOUT = 30000;

class APIError extends Error {
  constructor(message, status, code) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.code = code;
  }
}

class AnalyticsAPI {
  #baseUrl;
  #apiKey;
  #timeout;
  #authRequiredCallback;

  constructor(config = {}) {
    this.#baseUrl = config.baseUrl || DEFAULT_BASE_URL;
    this.#apiKey = config.apiKey || this.#getApiKeyFromURL() || this.#getApiKeyFromStorage();
    this.#timeout = config.timeout || DEFAULT_TIMEOUT;
    this.#authRequiredCallback = config.onAuthRequired || null;
  }

  /**
   * Get API key from URL query param (takes precedence)
   */
  #getApiKeyFromURL() {
    const params = new URLSearchParams(window.location.search);
    return params.get('api_key') || '';
  }

  /**
   * Get API key from localStorage
   */
  #getApiKeyFromStorage() {
    try {
      return localStorage.getItem('analytics_api_key') || '';
    } catch {
      return '';
    }
  }

  /**
   * Set API key and optionally persist to localStorage
   */
  setApiKey(key, persist = false) {
    this.#apiKey = key;
    if (persist) {
      try {
        localStorage.setItem('analytics_api_key', key);
      } catch (e) {
        console.warn('Failed to save API key to localStorage:', e);
      }
    }
  }

  /**
   * Clear API key from memory and localStorage
   */
  clearApiKey() {
    this.#apiKey = '';
    try {
      localStorage.removeItem('analytics_api_key');
    } catch (e) {
      console.warn('Failed to clear API key from localStorage:', e);
    }
  }

  /**
   * Check if API key is set
   */
  hasApiKey() {
    return !!this.#apiKey;
  }

  /**
   * Get current API key
   */
  getApiKey() {
    return this.#apiKey;
  }

  /**
   * Set callback for when auth is required (401 response)
   */
  onAuthRequired(callback) {
    this.#authRequiredCallback = callback;
  }

  /**
   * Build fetch options with auth
   */
  #buildOptions(options = {}) {
    const headers = {
      'Content-Type': 'application/json',
      ...options.headers
    };

    if (this.#apiKey) {
      headers['X-API-Key'] = this.#apiKey;
    }

    return {
      ...options,
      headers
    };
  }

  /**
   * Build URL with query params
   */
  #buildUrl(endpoint, params = {}) {
    const url = new URL(endpoint, window.location.origin);
    Object.entries(params).forEach(([key, value]) => {
      if (value != null && value !== '') {
        url.searchParams.set(key, value);
      }
    });
    return url.pathname + url.search;
  }

  /**
   * Make fetch request with timeout
   */
  async #fetch(endpoint, options = {}) {
    // Create a new abort controller for this request
    const abortController = new AbortController();

    const timeoutId = setTimeout(() => {
      abortController.abort();
    }, this.#timeout);

    try {
      const response = await fetch(
        this.#buildUrl(this.#baseUrl + endpoint, options.params),
        this.#buildOptions({
          ...options,
          signal: abortController.signal
        })
      );

      clearTimeout(timeoutId);

      if (!response.ok) {
        let error = 'Erreur inconnue';
        let code = 'unknown_error';

        try {
          const data = await response.json();
          error = data.message || error;
          code = data.error || code;
        } catch {
          error = response.statusText || error;
        }

        // Trigger auth callback on 401
        if (response.status === 401 && this.#authRequiredCallback) {
          this.#authRequiredCallback();
        }

        throw new APIError(error, response.status, code);
      }

      // Handle 204 No Content
      if (response.status === 204) {
        return null;
      }

      return await response.json();
    } catch (error) {
      clearTimeout(timeoutId);

      if (error.name === 'AbortError') {
        throw new APIError('Délai de requête dépassé', 408, 'timeout');
      }

      if (error instanceof APIError) {
        throw error;
      }

      throw new APIError(
        error.message || 'Erreur réseau',
        0,
        'network_error'
      );
    }
  }

  /**
   * Check API health
   */
  async health() {
    return this.#fetch('/health');
  }

  /**
   * Get summary statistics
   * @param {string} start - Start date (YYYY-MM-DD)
   * @param {string} end - End date (YYYY-MM-DD)
   */
  async getSummary(start, end) {
    return this.#fetch('/summary', {
      params: { start, end }
    });
  }

  /**
   * Get time series data
   * @param {string} start - Start date (YYYY-MM-DD)
   * @param {string} end - End date (YYYY-MM-DD)
   */
  async getTimeSeries(start, end) {
    return this.#fetch('/timeseries', {
      params: { start, end }
    });
  }

  /**
   * Get top pages
   * @param {string} start - Start date (YYYY-MM-DD)
   * @param {string} end - End date (YYYY-MM-DD)
   * @param {number} limit - Max results
   */
  async getPages(start, end, limit = 50) {
    return this.#fetch('/pages', {
      params: { start, end, limit }
    });
  }

  /**
   * Get top events
   * @param {string} start - Start date (YYYY-MM-DD)
   * @param {string} end - End date (YYYY-MM-DD)
   * @param {number} limit - Max results
   */
  async getEvents(start, end, limit = 50) {
    return this.#fetch('/events', {
      params: { start, end, limit }
    });
  }

  /**
   * Get sessions
   * @param {number} start - Start timestamp (ms)
   * @param {number} end - End timestamp (ms)
   * @param {number} limit - Max results
   * @param {number} offset - Pagination offset
   */
  async getSessions(start, end, limit = 100, offset = 0) {
    return this.#fetch('/sessions', {
      params: { start, end, limit, offset }
    });
  }

  /**
   * Fetch all dashboard data in parallel
   */
  async getAll(start, end) {
    const [summary, timeseries, pages, events] = await Promise.allSettled([
      this.getSummary(start, end),
      this.getTimeSeries(start, end),
      this.getPages(start, end),
      this.getEvents(start, end)
    ]);

    return {
      summary: summary.status === 'fulfilled' ? summary.value : null,
      timeseries: timeseries.status === 'fulfilled' ? timeseries.value : null,
      pages: pages.status === 'fulfilled' ? pages.value : null,
      events: events.status === 'fulfilled' ? events.value : null,
      errors: {
        summary: summary.status === 'rejected' ? summary.reason : null,
        timeseries: timeseries.status === 'rejected' ? timeseries.reason : null,
        pages: pages.status === 'rejected' ? pages.reason : null,
        events: events.status === 'rejected' ? events.reason : null
      }
    };
  }
}

// Create singleton instance
const api = new AnalyticsAPI();

export { AnalyticsAPI, APIError, api as default };
