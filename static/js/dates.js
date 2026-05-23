(() => {
  function today() {
    const d = new Date();
    return new Date(d.getFullYear(), d.getMonth(), d.getDate());
  }

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

  function formatDate(str) {
    const [y, m, d] = str.split('-');
    return `${d}/${m}/${y}`;
  }

  function monthLabel(date) {
    return date.toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
  }

  function computeProgress(iv) {
    const now = today();
    const start = parseDate(iv.start_date);
    const end = parseDate(iv.end_date);
    const total = diffDays(start, end) + 1;

    if (now < start) {
      return { status: 'upcoming', past: 0, left: total, total, untilStart: diffDays(now, start), pct: 0 };
    }
    if (now > end) {
      return { status: 'ended', past: diffDays(end, now), left: 0, total, untilStart: 0, pct: 100 };
    }
    const past = diffDays(start, now);
    const left = diffDays(now, end) + 1;
    const pct = total > 1 ? Math.round((past / (total - 1)) * 100) : 100;
    return { status: 'active', past, left, total, untilStart: 0, pct };
  }

  function statusLabel(status, progress) {
    if (status === 'upcoming') return `starts in ${progress.untilStart} day${progress.untilStart !== 1 ? 's' : ''}`;
    if (status === 'ended') return `ended ${progress.past} day${progress.past !== 1 ? 's' : ''} ago`;
    return 'in progress';
  }

  window.DaysUntilDates = {
    computeProgress,
    diffDays,
    formatDate,
    formatISODate,
    isValidISODate,
    monthLabel,
    parseDate,
    statusLabel,
    today,
  };
})();
