const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'dates.js'), 'utf8');
const RealDate = Date;

function loadDates(fixedNow) {
  class MockDate extends RealDate {
    constructor(...args) {
      if (args.length === 0) {
        super(fixedNow.getTime());
        return;
      }
      super(...args);
    }

    static now() {
      return fixedNow.getTime();
    }
  }

  const context = {
    window: {},
    Date: MockDate,
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  return context.window.DaysUntilDates;
}

// --- all-day intervals: day-granularity progress, unchanged behavior ---

{
  const dates = loadDates(new RealDate(2026, 4, 18, 12, 0, 0, 0));
  const progress = dates.computeProgress({
    start_at: '2026-05-18T00:00:00Z',
    end_at: '2026-05-24T00:00:00Z',
    all_day: true,
    recurrence_rule: 'none',
  });

  assert.equal(progress.status, 'active');
  assert.equal(progress.total, 7);
  assert.equal(progress.past, 0);
  assert.equal(progress.left, 7);
  assert.equal(progress.pct, 0);
}

{
  const dates = loadDates(new RealDate(2026, 4, 16, 12, 0, 0, 0));
  const progress = dates.computeProgress({
    start_at: '2026-05-18T00:00:00Z',
    end_at: '2026-05-24T00:00:00Z',
    all_day: true,
    recurrence_rule: 'none',
  });

  assert.equal(progress.status, 'upcoming');
  assert.equal(progress.total, 7);
  assert.equal(progress.left, 7);
  assert.equal(progress.untilStart, 2);
  assert.equal(dates.statusLabel(progress.status, progress), 'starts in 2 days');
}

{
  const dates = loadDates(new RealDate(2026, 4, 20, 12, 0, 0, 0));
  const progress = dates.computeProgress({
    start_at: '2026-05-20T00:00:00Z',
    end_at: '2026-05-20T00:00:00Z',
    all_day: true,
    recurrence_rule: 'none',
  });

  assert.equal(progress.status, 'active');
  assert.equal(progress.total, 1);
  assert.equal(progress.past, 0);
  assert.equal(progress.left, 1);
  assert.equal(progress.pct, 100);
}

// --- timed (non-all-day) intervals: millisecond-precision progress ---

{
  const dates = loadDates(new RealDate('2026-05-20T11:00:00Z'));
  const progress = dates.computeProgress({
    start_at: '2026-05-20T10:00:00Z',
    end_at: '2026-05-20T14:00:00Z',
    all_day: false,
    recurrence_rule: 'none',
  });

  assert.equal(progress.status, 'active');
  assert.equal(progress.left, 3 * 3600000);
  assert.equal(dates.formatByUnit(progress.left, 'hours'), '3 hours');
}

{
  const dates = loadDates(new RealDate('2026-05-20T09:00:00Z'));
  const progress = dates.computeProgress({
    start_at: '2026-05-20T10:00:00Z',
    end_at: '2026-05-20T14:00:00Z',
    all_day: false,
    recurrence_rule: 'none',
  });

  assert.equal(progress.status, 'upcoming');
  assert.equal(progress.untilStart, 3600000);
  assert.equal(dates.formatByUnit(progress.untilStart, 'minutes'), '60 minutes');
}

// --- formatByUnit: auto-unit selection and singular/plural ---

{
  const dates = loadDates(new RealDate());
  assert.equal(dates.formatByUnit(30000, 'auto'), '30 seconds');
  assert.equal(dates.formatByUnit(1000, 'auto'), '1 second');
  assert.equal(dates.formatByUnit(2 * 3600000, 'auto'), '2 hours');
  assert.equal(dates.formatByUnit(10 * 86400000, 'auto'), '1 week');
  assert.equal(dates.formatByUnit(86400000, 'sleeps'), '1 sleep');
  assert.equal(dates.formatByUnit(3 * 86400000, 'sleeps'), '3 sleeps');
}

// --- resolveOccurrence: recurring intervals roll forward without mutating iv ---

{
  const dates = loadDates(new RealDate(2026, 7, 3, 12, 0, 0, 0));
  const iv = {
    start_at: '2020-01-01T00:00:00Z',
    end_at: '2020-01-01T00:00:00Z',
    all_day: true,
    recurrence_rule: 'yearly',
  };
  const progress = dates.computeProgress(iv, new RealDate(2026, 7, 3, 12, 0, 0, 0));

  // The occurrence resolves to Jan 1 2027 (the next occurrence after "now"),
  // and iv itself is left untouched.
  assert.equal(progress.status, 'upcoming');
  assert.equal(progress.start.getFullYear(), 2027);
  assert.equal(iv.start_at, '2020-01-01T00:00:00Z');
}
