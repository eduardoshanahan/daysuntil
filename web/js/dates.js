(() => {
  function today() {
    const d = new Date();
    return new Date(d.getFullYear(), d.getMonth(), d.getDate());
  }

  function startOfDay(date) {
    return new Date(date.getFullYear(), date.getMonth(), date.getDate());
  }

  // parseDate/formatISODate/isValidISODate operate on plain YYYY-MM-DD
  // strings — still needed for the calendar date-picker widget, which
  // always works in local, timezone-free calendar days. app.js combines a
  // picked date (+ optional time, + the interval's timezone) into the
  // full start_at/end_at instants the API expects.
  function parseDate(str) {
    const [y, m, d] = str.split('-').map(Number);
    return new Date(y, m - 1, d);
  }

  function isValidISODate(str) {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(str)) return false;
    const date = parseDate(str);
    return formatISODate(date) === str;
  }

  function formatISODate(date) {
    const year = date.getFullYear();
    const month = `${date.getMonth() + 1}`.padStart(2, '0');
    const day = `${date.getDate()}`.padStart(2, '0');
    return `${year}-${month}-${day}`;
  }

  function diffDays(a, b) {
    return Math.round((b - a) / 86400000);
  }

  function formatDate(date) {
    const d = `${date.getDate()}`.padStart(2, '0');
    const m = `${date.getMonth() + 1}`.padStart(2, '0');
    const y = date.getFullYear();
    return `${d}/${m}/${y}`;
  }

  function monthLabel(date) {
    return date.toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
  }

  // resolveOccurrence rolls a recurring interval's start/end forward by
  // whole periods until the occurrence's end is in the future, without
  // mutating iv.start_at/end_at — mirrors the server's nextOccurrence
  // (server/recurrence.go), used by the reminder dispatcher, so both sides
  // agree on "what's the current occurrence" without either persisting the
  // rolled-forward date. Non-recurring intervals pass through unchanged.
  function resolveOccurrence(iv, now) {
    let start = new Date(iv.start_at);
    let end = new Date(iv.end_at);
    const rule = iv.recurrence_rule;
    if (!rule || rule === 'none' || end > now) return { start, end };

    const advance = date => {
      const d = new Date(date);
      if (rule === 'weekly') d.setDate(d.getDate() + 7);
      else if (rule === 'monthly') d.setMonth(d.getMonth() + 1);
      else if (rule === 'yearly') d.setFullYear(d.getFullYear() + 1);
      return d;
    };

    while (end <= now) {
      start = advance(start);
      end = advance(end);
    }
    return { start, end };
  }

  // computeProgress works in whole days for all-day intervals (so a
  // multi-day trip's progress bar fills day-by-day, matching how it always
  // has) and in milliseconds for timed intervals (continuous fill). Either
  // way, past/left/total/pct describe the same status categories
  // (upcoming/active/ended); formatByUnit turns the underlying instants
  // into the seconds/minutes/hours/weeks/months/years/"sleeps" display the
  // user picked, independent of how the bar itself is computed.
  function computeProgress(iv, now) {
    now = now || new Date();
    const { start, end } = resolveOccurrence(iv, now);

    if (iv.all_day) {
      const dayNow = startOfDay(now);
      const dayStart = startOfDay(start);
      const dayEnd = startOfDay(end);
      const total = diffDays(dayStart, dayEnd) + 1;

      if (dayNow < dayStart) {
        return { status: 'upcoming', past: 0, left: total, total, untilStart: diffDays(dayNow, dayStart), pct: 0, start, end };
      }
      if (dayNow > dayEnd) {
        return { status: 'ended', past: diffDays(dayEnd, dayNow), left: 0, total, untilStart: 0, pct: 100, start, end };
      }
      const past = diffDays(dayStart, dayNow);
      const left = diffDays(dayNow, dayEnd) + 1;
      const pct = total > 1 ? Math.round((past / (total - 1)) * 100) : 100;
      return { status: 'active', past, left, total, untilStart: 0, pct, start, end };
    }

    const totalMs = end - start;
    if (now < start) {
      return { status: 'upcoming', past: 0, left: totalMs, total: totalMs, untilStart: start - now, pct: 0, start, end };
    }
    if (now > end) {
      return { status: 'ended', past: now - end, left: 0, total: totalMs, untilStart: 0, pct: 100, start, end };
    }
    const pastMs = now - start;
    const leftMs = end - now;
    const pct = totalMs > 0 ? Math.round((pastMs / totalMs) * 100) : 100;
    return { status: 'active', past: pastMs, left: leftMs, total: totalMs, untilStart: 0, pct, start, end };
  }

  function statusLabel(status, progress) {
    if (status === 'upcoming') return `starts in ${progress.untilStart} day${progress.untilStart !== 1 ? 's' : ''}`;
    if (status === 'ended') return `ended ${progress.past} day${progress.past !== 1 ? 's' : ''} ago`;
    return 'in progress';
  }

  const UNIT_MS = {
    seconds: 1000,
    minutes: 60000,
    hours: 3600000,
    days: 86400000,
    weeks: 604800000,
    months: 2629746000, // average month (365.2425 / 12 days) — approximate by design, see formatByUnit
    years: 31556952000, // average year (365.2425 days) — approximate by design, see formatByUnit
    sleeps: 86400000,
  };

  // pickAutoUnit chooses a display unit so the resulting number reads
  // naturally (not "3600 seconds", not "0 weeks") — same heuristic a
  // countdown app's "auto" mode uses.
  function pickAutoUnit(ms) {
    if (ms < UNIT_MS.minutes) return 'seconds';
    if (ms < UNIT_MS.hours) return 'minutes';
    if (ms < UNIT_MS.days) return 'hours';
    if (ms < UNIT_MS.weeks) return 'days';
    if (ms < UNIT_MS.months) return 'weeks';
    if (ms < UNIT_MS.years) return 'months';
    return 'years';
  }

  // formatByUnit renders a millisecond distance (e.g. progress.left or
  // progress.untilStart from computeProgress) in the requested unit.
  // "months"/"years" use average lengths, not calendar-exact ones — exact
  // calendar math (respecting variable month/year lengths) isn't worth the
  // added complexity for a rounded display figure like "in about 3 months".
  function formatByUnit(ms, unit) {
    const resolved = !unit || unit === 'auto' ? pickAutoUnit(Math.abs(ms)) : unit;
    const divisor = UNIT_MS[resolved] || UNIT_MS.days;
    const value = Math.round(Math.abs(ms) / divisor);
    const singular = resolved.slice(0, -1);
    return `${value} ${value === 1 ? singular : resolved}`;
  }

  window.DaysUntilDates = {
    computeProgress,
    diffDays,
    formatByUnit,
    formatDate,
    formatISODate,
    isValidISODate,
    monthLabel,
    parseDate,
    resolveOccurrence,
    statusLabel,
    today,
  };
})();
