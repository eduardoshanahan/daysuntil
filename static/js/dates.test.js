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

{
  const dates = loadDates(new RealDate(2026, 4, 18, 12, 0, 0, 0));
  const progress = dates.computeProgress({
    start_date: '2026-05-18',
    end_date: '2026-05-24',
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
    start_date: '2026-05-18',
    end_date: '2026-05-24',
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
    start_date: '2026-05-20',
    end_date: '2026-05-20',
  });

  assert.equal(progress.status, 'active');
  assert.equal(progress.total, 1);
  assert.equal(progress.past, 0);
  assert.equal(progress.left, 1);
  assert.equal(progress.pct, 100);
}
