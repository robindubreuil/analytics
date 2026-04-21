/**
 * Utility functions for the analytics dashboard
 */

// DOM selection helpers
export const $ = (selector, parent = document) =>
  parent.querySelector(selector);

export const $$ = (selector, parent = document) =>
  [...parent.querySelectorAll(selector)];

// Safe DOM query - only runs callback if element exists
export function safeQuery(selector, callback, parent = document) {
  const el = parent.querySelector(selector);
  if (el) callback(el);
  return el;
}

// Format numbers with locale
export function formatNumber(num, locale = 'fr-FR', options = {}) {
  if (num == null) return '-';
  return Number(num).toLocaleString(locale, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
    ...options
  });
}

// Format percentage
export function formatPercent(value, decimals = 1) {
  if (value == null) return '-';
  return `${value.toFixed(decimals)}%`;
}

// Format duration (seconds to human-readable)
export function formatDuration(seconds) {
  if (seconds == null) return '-';
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60);
    const secs = Math.round(seconds % 60);
    return `${mins}m ${secs}s`;
  }
  const hours = Math.floor(seconds / 3600);
  const mins = Math.round((seconds % 3600) / 60);
  return `${hours}h ${mins}m`;
}

// Format date
export function formatDate(date, locale = 'fr-FR', options = {}) {
  const defaultOptions = {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    ...options
  };
  return new Date(date).toLocaleDateString(locale, defaultOptions);
}

// Format date to ISO (YYYY-MM-DD)
export function toISODate(date) {
  const d = new Date(date);
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

// Get today's date in ISO format
export function getToday() {
  return toISODate(new Date());
}

// Get date N days ago in ISO format
export function getDaysAgo(days) {
  const date = new Date();
  date.setDate(date.getDate() - days);
  return toISODate(date);
}

// Parse date from ISO format
export function parseISODate(str) {
  const [year, month, day] = str.split('-').map(Number);
  return new Date(year, month - 1, day);
}

// Get date range as array of dates
export function getDateRange(start, end) {
  const dates = [];
  let current = parseISODate(start);
  const endDate = parseISODate(end);

  while (current <= endDate) {
    dates.push(toISODate(current));
    current.setDate(current.getDate() + 1);
  }

  return dates;
}

// Debounce function
export function debounce(fn, delay) {
  let timeoutId;
  return function (...args) {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn.apply(this, args), delay);
  };
}

// Throttle function
export function throttle(fn, limit) {
  let inThrottle;
  return function (...args) {
    if (!inThrottle) {
      fn.apply(this, args);
      inThrottle = true;
      setTimeout(() => (inThrottle = false), limit);
    }
  };
}

// Create element with attributes and children
export function createElement(tag, attrs = {}, children = []) {
  const el = document.createElement(tag);

  Object.entries(attrs).forEach(([key, value]) => {
    if (key === 'className') {
      el.className = value;
    } else if (key === 'dataset') {
      Object.entries(value).forEach(([k, v]) => {
        el.dataset[k] = v;
      });
    } else if (key.startsWith('on') && typeof value === 'function') {
      const event = key.slice(2).toLowerCase();
      el.addEventListener(event, value);
    } else {
      el[key] = value;
    }
  });

  children.forEach(child => {
    if (typeof child === 'string') {
      el.appendChild(document.createTextNode(child));
    } else if (child instanceof Node) {
      el.appendChild(child);
    }
  });

  return el;
}

// Escape HTML to prevent XSS
export function escapeHtml(str) {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

// Truncate text
export function truncate(str, length = 50, suffix = '...') {
  if (str.length <= length) return str;
  return str.slice(0, length) + suffix;
}

// Get relative time (e.g., "il y a 2 heures")
export function relativeTime(date) {
  const seconds = Math.floor((Date.now() - new Date(date).getTime()) / 1000);
  const intervals = {
    year: { seconds: 31536000, singular: 'an', plural: 'ans' },
    month: { seconds: 2592000, singular: 'mois', plural: 'mois' },
    week: { seconds: 604800, singular: 'semaine', plural: 'semaines' },
    day: { seconds: 86400, singular: 'jour', plural: 'jours' },
    hour: { seconds: 3600, singular: 'heure', plural: 'heures' },
    minute: { seconds: 60, singular: 'minute', plural: 'minutes' }
  };

  for (const [unit, data] of Object.entries(intervals)) {
    const interval = Math.floor(seconds / data.seconds);
    if (interval >= 1) {
      const label = interval > 1 ? data.plural : data.singular;
      return `il y a ${interval} ${label}`;
    }
  }

  return 'à l\'instant';
}

// Calculate percent change
export function calculatePercentChange(oldValue, newValue) {
  if (oldValue === 0) return newValue > 0 ? 100 : 0;
  return ((newValue - oldValue) / oldValue) * 100;
}

// Parse URL params
export function getURLParams() {
  return Object.fromEntries(
    new URLSearchParams(window.location.search)
  );
}

// Update URL params without reloading
export function updateURLParams(params) {
  const url = new URL(window.location);
  Object.entries(params).forEach(([key, value]) => {
    if (value == null || value === '') {
      url.searchParams.delete(key);
    } else {
      url.searchParams.set(key, value);
    }
  });
  window.history.replaceState({}, '', url);
}

// Local storage helpers with error handling
export const storage = {
  get(key) {
    try {
      const item = localStorage.getItem(key);
      return item ? JSON.parse(item) : null;
    } catch {
      return null;
    }
  },
  set(key, value) {
    try {
      localStorage.setItem(key, JSON.stringify(value));
    } catch (e) {
      console.warn('Failed to save to localStorage:', e);
    }
  },
  remove(key) {
    try {
      localStorage.removeItem(key);
    } catch (e) {
      console.warn('Failed to remove from localStorage:', e);
    }
  }
};
