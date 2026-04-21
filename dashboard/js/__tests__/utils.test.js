import { describe, it, expect } from 'vitest';
import {
  formatNumber,
  formatPercent,
  formatDuration,
  toISODate,
  getDaysAgo,
  parseISODate,
  getDateRange,
  escapeHtml,
  truncate,
  calculatePercentChange,
  debounce,
  throttle,
  $,
  $$
} from '../utils.js';

describe('formatNumber', () => {
  it('formats numbers with locale', () => {
    const result = formatNumber(1234);
    expect(result).toMatch(/1/);
    expect(result).toMatch(/234/);
  });

  it('returns dash for null', () => {
    expect(formatNumber(null)).toBe('-');
  });

  it('returns dash for undefined', () => {
    expect(formatNumber(undefined)).toBe('-');
  });

  it('formats zero', () => {
    expect(formatNumber(0)).toBe('0');
  });

  it('formats negative numbers', () => {
    const result = formatNumber(-100);
    expect(result).toContain('100');
  });
});

describe('formatPercent', () => {
  it('formats percentage with default decimals', () => {
    expect(formatPercent(50.123)).toBe('50.1%');
  });

  it('formats percentage with custom decimals', () => {
    expect(formatPercent(50.123, 2)).toBe('50.12%');
  });

  it('returns dash for null', () => {
    expect(formatPercent(null)).toBe('-');
  });

  it('formats zero percent', () => {
    expect(formatPercent(0)).toBe('0.0%');
  });

  it('formats 100 percent', () => {
    expect(formatPercent(100)).toBe('100.0%');
  });
});

describe('formatDuration', () => {
  it('formats seconds', () => {
    expect(formatDuration(30)).toBe('30s');
  });

  it('formats zero seconds', () => {
    expect(formatDuration(0)).toBe('0s');
  });

  it('formats minutes and seconds', () => {
    expect(formatDuration(125)).toBe('2m 5s');
  });

  it('formats hours and minutes', () => {
    expect(formatDuration(3661)).toBe('1h 1m');
  });

  it('returns dash for null', () => {
    expect(formatDuration(null)).toBe('-');
  });

  it('formats exactly one minute', () => {
    expect(formatDuration(60)).toBe('1m 0s');
  });
});

describe('toISODate', () => {
  it('converts date to ISO format', () => {
    const date = new Date(2024, 0, 15);
    expect(toISODate(date)).toBe('2024-01-15');
  });

  it('handles end of year', () => {
    const date = new Date(2024, 11, 31);
    expect(toISODate(date)).toBe('2024-12-31');
  });
});

describe('parseISODate', () => {
  it('parses ISO date string', () => {
    const result = parseISODate('2024-01-15');
    expect(result.getFullYear()).toBe(2024);
    expect(result.getMonth()).toBe(0);
    expect(result.getDate()).toBe(15);
  });

  it('parses different months', () => {
    const result = parseISODate('2024-06-30');
    expect(result.getMonth()).toBe(5);
    expect(result.getDate()).toBe(30);
  });
});

describe('getDateRange', () => {
  it('returns array of dates', () => {
    const dates = getDateRange('2024-01-01', '2024-01-03');
    expect(dates).toEqual(['2024-01-01', '2024-01-02', '2024-01-03']);
  });

  it('returns single date when start equals end', () => {
    const dates = getDateRange('2024-01-01', '2024-01-01');
    expect(dates).toEqual(['2024-01-01']);
  });

  it('returns empty for start after end', () => {
    const dates = getDateRange('2024-01-03', '2024-01-01');
    expect(dates).toEqual([]);
  });
});

describe('escapeHtml', () => {
  it('escapes angle brackets', () => {
    expect(escapeHtml('<script>')).toBe('&lt;script&gt;');
  });

  it('escapes ampersands', () => {
    expect(escapeHtml('a&b')).toBe('a&amp;b');
  });

  it('escapes quotes', () => {
    expect(escapeHtml('"hello"')).toBe('&quot;hello&quot;');
  });

  it('returns plain text unchanged', () => {
    expect(escapeHtml('hello world')).toBe('hello world');
  });

  it('handles empty string', () => {
    expect(escapeHtml('')).toBe('');
  });
});

describe('truncate', () => {
  it('does not truncate short strings', () => {
    expect(truncate('hello')).toBe('hello');
  });

  it('truncates long strings with default length', () => {
    const long = 'a'.repeat(60);
    const result = truncate(long);
    expect(result.length).toBe(53);
    expect(result.endsWith('...')).toBe(true);
  });

  it('truncates with custom length', () => {
    const result = truncate('hello world', 5);
    expect(result).toBe('hello...');
  });

  it('handles exact length string', () => {
    expect(truncate('hello', 5)).toBe('hello');
  });
});

describe('calculatePercentChange', () => {
  it('calculates positive change', () => {
    expect(calculatePercentChange(100, 200)).toBe(100);
  });

  it('calculates negative change', () => {
    expect(calculatePercentChange(200, 100)).toBe(-50);
  });

  it('returns 100 for growth from zero', () => {
    expect(calculatePercentChange(0, 100)).toBe(100);
  });

  it('returns 0 for no change from zero', () => {
    expect(calculatePercentChange(0, 0)).toBe(0);
  });

  it('handles no change', () => {
    expect(calculatePercentChange(100, 100)).toBe(0);
  });
});

describe('debounce', () => {
  it('delays function execution', async () => {
    let called = 0;
    const fn = debounce(() => { called++; }, 50);

    fn();
    fn();
    fn();

    expect(called).toBe(0);

    await new Promise(r => setTimeout(r, 100));
    expect(called).toBe(1);
  });
});

describe('throttle', () => {
  it('limits function calls', async () => {
    let called = 0;
    const fn = throttle(() => { called++; }, 50);

    fn();
    fn();
    fn();

    expect(called).toBe(1);

    await new Promise(r => setTimeout(r, 100));
    fn();
    expect(called).toBe(2);
  });
});
