/**
 * Privacy-focused analytics tracker
 * Tracks page views, engagement time, and custom events
 * Uses sendBeacon for reliable delivery, respects DNT
 *
 * Embed: <script src="/analytics.js" data-endpoint="/api/analytics" data-api-key="key"></script>
 * Or configure via window.ANALYTICS_CONFIG before loading.
 */
(function () {
  'use strict';

  var STORAGE_KEY = 'analytics_queue';
  var SESSION_KEY = 'analytics_session';

  var DEFAULTS = {
    endpoint: '/api/analytics',
    apiKey: null,
    batchSize: 5,
    flushInterval: 10000,
    respectDNT: true,
    trackEngagement: true,
    trackScrollDepth: true,
    scrollDepthThresholds: [25, 50, 75, 90, 100],
    debug: false
  };

  function throttle(fn, limit) {
    var inThrottle = false;
    return function () {
      var args = arguments;
      var ctx = this;
      if (!inThrottle) {
        fn.apply(ctx, args);
        inThrottle = true;
        setTimeout(function () { inThrottle = false; }, limit);
      }
    };
  }

  function generateUUID() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
      var r = Math.random() * 16 | 0;
      var v = c === 'x' ? r : (r & 0x3 | 0x8);
      return v.toString(16);
    });
  }

  function sanitizeEventName(name) {
    return String(name).replace(/[^a-zA-Z0-9_.-]/g, '_');
  }

  function safeJsonParse(str, fallback) {
    try { return JSON.parse(str); }
    catch (e) { return fallback; }
  }

  function readConfig() {
    var script = document.currentScript;
    var cfg = {};
    if (script) {
      if (script.dataset.endpoint) cfg.endpoint = script.dataset.endpoint;
      if (script.dataset.apiKey) cfg.apiKey = script.dataset.apiKey;
      if (script.dataset.debug) cfg.debug = script.dataset.debug === 'true';
    }
    if (window.ANALYTICS_CONFIG) {
      for (var k in window.ANALYTICS_CONFIG) {
        if (window.ANALYTICS_CONFIG.hasOwnProperty(k)) {
          cfg[k] = window.ANALYTICS_CONFIG[k];
        }
      }
    }
    return cfg;
  }

  function Analytics(options) {
    this._options = {};
    for (var k in DEFAULTS) {
      if (DEFAULTS.hasOwnProperty(k)) this._options[k] = DEFAULTS[k];
    }
    for (var k in options) {
      if (options.hasOwnProperty(k)) this._options[k] = options[k];
    }

    this._queue = [];
    this._engagementTime = 0;
    this._isPageVisible = !document.hidden;
    this._engagementTimer = null;
    this._flushTimer = null;
    this._scrollDepthsTracked = {};
    this._maxScrollDepth = 0;
    this._isEnabled = false;
    this._isDisposed = false;
    this._listeners = [];

    if (!this._shouldEnable()) {
      if (this._options.debug) console.info('[Analytics] Disabled (DNT or opted out)');
      return;
    }

    this._isEnabled = true;
    this._sessionId = this._getOrCreateSessionId();
    this._pageLoadTime = performance.now();

    this._restoreQueue();
    this._setupEventListeners();
    this._startTimers();

    if (this._options.debug) {
      console.info('[Analytics] Initialized', { sessionId: this._sessionId, endpoint: this._options.endpoint });
    }
  }

  Analytics.prototype._shouldEnable = function () {
    if (this._options.respectDNT &&
        (navigator.doNotTrack === '1' || window.doNotTrack === '1' || navigator.msDoNotTrack === '1')) {
      return false;
    }
    try {
      if (localStorage.getItem('analytics_optout') === 'true') return false;
    } catch (e) { /* storage unavailable */ }
    return true;
  };

  Analytics.prototype._getOrCreateSessionId = function () {
    try {
      var sid = sessionStorage.getItem(SESSION_KEY);
      if (sid) return sid;
      sid = generateUUID();
      sessionStorage.setItem(SESSION_KEY, sid);
      return sid;
    } catch (e) {
      return generateUUID();
    }
  };

  Analytics.prototype._getBaseContext = function () {
    return {
      screenWidth: window.screen.width,
      screenHeight: window.screen.height,
      viewportWidth: window.innerWidth,
      viewportHeight: window.innerHeight,
      scrollDepth: this._maxScrollDepth,
      engagementTime: Math.round(this._engagementTime)
    };
  };

  Analytics.prototype._enqueue = function (event) {
    this._queue.push(event);
    if (this._queue.length >= this._options.batchSize) {
      this._flush();
    } else {
      this._persistQueue();
    }
  };

  Analytics.prototype.pageview = function (data) {
    if (!this._isEnabled || this._isDisposed) return;
    data = data || {};
    this._enqueue({
      sessionId: this._sessionId,
      type: 'pageview',
      url: window.location.pathname,
      referrer: document.referrer || null,
      title: document.title,
      timestamp: Date.now(),
      userAgent: navigator.userAgent,
      data: merge(this._getBaseContext(), data)
    });
    if (this._options.debug) console.log('[Analytics] Page view tracked', window.location.pathname);
  };

  Analytics.prototype.track = function (event, data) {
    if (!this._isEnabled || this._isDisposed) return;
    data = data || {};
    this._enqueue({
      sessionId: this._sessionId,
      type: 'event',
      event: sanitizeEventName(event),
      url: window.location.pathname,
      timestamp: Date.now(),
      userAgent: navigator.userAgent,
      data: merge(this._getBaseContext(), data)
    });
    if (this._options.debug) console.log('[Analytics] Event tracked: ' + event, data);
  };

  Analytics.prototype.flush = function (useBeacon) {
    if (!this._isEnabled || this._isDisposed) return;
    if (this._queue.length === 0) return;

    var eventsToSend = this._queue.splice(0, this._options.batchSize);
    if (this._queue.length > 0) {
      this._persistQueue();
    } else {
      try { localStorage.removeItem(STORAGE_KEY); } catch (e) { /* ignore */ }
    }

    this._send(eventsToSend, useBeacon);
  };

  Analytics.prototype._flush = function (useBeacon) {
    this.flush(useBeacon);
  };

  Analytics.prototype._send = function (events, useBeacon) {
    if (events.length === 0) return;

    var payload = JSON.stringify(events);
    var endpoint = this._options.endpoint;
    var apiKey = this._options.apiKey;
    var self = this;

    if (useBeacon || this._isPageUnload()) {
      if (navigator.sendBeacon) {
        var beaconUrl = endpoint;
        if (apiKey) {
          beaconUrl += (beaconUrl.indexOf('?') === -1 ? '?' : '&') + 'api_key=' + encodeURIComponent(apiKey);
        }
        var blob = new Blob([payload], { type: 'application/json' });
        var sent = navigator.sendBeacon(beaconUrl, blob);
        if (sent) {
          if (this._options.debug) console.log('[Analytics] Sent via sendBeacon', events.length);
          return;
        }
      }
    }

    var headers = { 'Content-Type': 'application/json' };
    if (apiKey) headers['X-API-Key'] = apiKey;

    fetch(endpoint, {
      method: 'POST',
      headers: headers,
      body: payload,
      keepalive: true
    }).catch(function (error) {
      if (self._options.debug) console.error('[Analytics] Send failed, re-queueing', error);
      self._queue.unshift.apply(self._queue, events);
      self._persistQueue();
    });
  };

  Analytics.prototype._isPageUnload = function () {
    return document.visibilityState === 'hidden' || this._isDisposed;
  };

  Analytics.prototype._persistQueue = function () {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this._queue.slice(-100)));
    } catch (e) {
      if (this._options.debug) console.warn('[Analytics] Could not persist queue', e);
    }
  };

  Analytics.prototype._restoreQueue = function () {
    try {
      var stored = localStorage.getItem(STORAGE_KEY);
      if (stored) {
        this._queue = safeJsonParse(stored, []);
        localStorage.removeItem(STORAGE_KEY);
        if (this._queue.length > 0) this._flush();
      }
    } catch (e) {
      if (this._options.debug) console.warn('[Analytics] Could not restore queue', e);
    }
  };

  Analytics.prototype._setupEventListeners = function () {
    var self = this;

    this._addListener(document, 'visibilitychange', function () {
      var wasVisible = self._isPageVisible;
      self._isPageVisible = !document.hidden;
      if (wasVisible && document.hidden) self._flush(true);
    });

    if (this._options.trackScrollDepth) {
      var scrollFn = throttle(function () {
        self._handleScroll();
      }, 100);
      this._addListener(window, 'scroll', scrollFn, { passive: true });
    }

    this._addListener(window, 'beforeunload', function () { self._flush(true); });
    this._addListener(window, 'pagehide', function () { self._flush(true); });
  };

  Analytics.prototype._addListener = function (target, type, fn, options) {
    target.addEventListener(type, fn, options);
    this._listeners.push({ target: target, type: type, fn: fn, options: options });
  };

  Analytics.prototype._removeAllListeners = function () {
    for (var i = 0; i < this._listeners.length; i++) {
      var l = this._listeners[i];
      l.target.removeEventListener(l.type, l.fn, l.options);
    }
    this._listeners = [];
  };

  Analytics.prototype._startTimers = function () {
    var self = this;
    if (this._options.trackEngagement) {
      this._engagementTimer = setInterval(function () {
        if (self._isPageVisible && !document.hidden) {
          self._engagementTime++;
        }
      }, 1000);
    }
    this._flushTimer = setInterval(function () {
      self._flush();
    }, this._options.flushInterval);
  };

  Analytics.prototype._stopTimers = function () {
    if (this._engagementTimer) { clearInterval(this._engagementTimer); this._engagementTimer = null; }
    if (this._flushTimer) { clearInterval(this._flushTimer); this._flushTimer = null; }
  };

  Analytics.prototype._handleScroll = function () {
    if (!this._options.trackScrollDepth || this._isDisposed) return;

    var scrollHeight = document.documentElement.scrollHeight - window.innerHeight;
    var scrolled = window.scrollY;
    var depth = scrollHeight > 0 ? Math.min(100, Math.round((scrolled / scrollHeight) * 100)) : 100;

    if (depth > this._maxScrollDepth) this._maxScrollDepth = depth;

    var thresholds = this._options.scrollDepthThresholds;
    for (var i = 0; i < thresholds.length; i++) {
      var t = thresholds[i];
      if (depth >= t && !this._scrollDepthsTracked[t]) {
        this._scrollDepthsTracked[t] = true;
        this.track('scroll_depth', { depth: t });
      }
    }
  };

  Analytics.prototype.getSessionId = function () { return this._sessionId; };
  Analytics.prototype.getEngagementTime = function () { return this._engagementTime; };

  Analytics.prototype.dispose = function () {
    if (this._isDisposed) return;
    this._isDisposed = true;
    this._flush(true);
    this._stopTimers();
    this._removeAllListeners();
    this._queue = [];
    this._scrollDepthsTracked = {};
    if (this._options.debug) console.info('[Analytics] Disposed');
  };

  function merge(target, source) {
    var result = {};
    for (var k in target) { if (target.hasOwnProperty(k)) result[k] = target[k]; }
    for (var k in source) { if (source.hasOwnProperty(k)) result[k] = source[k]; }
    return result;
  }

  var instance = null;

  window.AnalyticsTracker = {
    init: function (options) {
      if (instance) {
        if ((options || {}).debug) console.warn('[Analytics] Already initialized');
        return instance;
      }
      var cfg = merge(readConfig(), options || {});
      instance = new Analytics(cfg);
      if (instance._isEnabled) instance.pageview();
      return instance;
    },
    get: function () { return instance; },
    dispose: function () {
      if (instance) { instance.dispose(); instance = null; }
    },
    pageview: function (data) { if (instance) instance.pageview(data); },
    track: function (event, data) { if (instance) instance.track(event, data); },
    optOut: function () {
      try { localStorage.setItem('analytics_optout', 'true'); } catch (e) { /* ignore */ }
      if (instance) { instance.dispose(); instance = null; }
    },
    optIn: function () {
      try { localStorage.removeItem('analytics_optout'); } catch (e) { /* ignore */ }
    }
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () { window.AnalyticsTracker.init(); });
  } else {
    window.AnalyticsTracker.init();
  }
})();
