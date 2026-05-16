'use strict';

const list      = document.getElementById('interval-list');
const overlay   = document.getElementById('modal-overlay');
const form      = document.getElementById('interval-form');
const modalTitle= document.getElementById('modal-title');
const fieldId    = document.getElementById('field-id');
const fieldName  = document.getElementById('field-name');
const fieldStart = document.getElementById('field-start');
const fieldEnd   = document.getElementById('field-end');
const fieldColor = document.getElementById('field-color');
const formError  = document.getElementById('form-error');
const btnSave    = document.getElementById('btn-save');
const btnAdd    = document.getElementById('btn-add');
const btnCancel = document.getElementById('btn-cancel');
const btnRetry  = document.getElementById('btn-retry');
const loadingMsg = document.getElementById('loading-msg');
const appStatus = document.getElementById('app-status');
const statusMessage = document.getElementById('status-message');
const colorSwatches = document.getElementById('color-swatches');

let isSubmitting = false;
let activeLoadToken = 0;
const pendingDeleteIds = new Set();
let lastFocusedElement = null;

// ── API ────────────────────────────────────────────────────────────────────────

async function apiFetch(path, options = {}) {
  let res;
  try {
    res = await fetch(path, {
      headers: { 'Content-Type': 'application/json' },
      ...options,
    });
  } catch {
    throw new Error('Network error. Check your connection and try again.');
  }

  if (!res.ok) {
    const msg = (await res.text()).trim();
    throw new Error(msg || `Request failed with status ${res.status}.`);
  }
  if (res.status === 204) return null;
  return res.json();
}

const api = {
  list:   ()       => apiFetch('/api/intervals'),
  create: (data)   => apiFetch('/api/intervals',      { method: 'POST',   body: JSON.stringify(data) }),
  update: (id, d)  => apiFetch(`/api/intervals/${id}`,{ method: 'PUT',    body: JSON.stringify(d) }),
  delete: (id)     => apiFetch(`/api/intervals/${id}`,{ method: 'DELETE' }),
};

// ── Date helpers ───────────────────────────────────────────────────────────────

function today() {
  const d = new Date();
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

function parseDate(str) {
  const [y, m, d] = str.split('-').map(Number);
  return new Date(y, m - 1, d);
}

function diffDays(a, b) {
  return Math.round((b - a) / 86400000);
}

function formatDate(str) {
  const [y, m, d] = str.split('-');
  return `${d}/${m}/${y}`;
}

// ── Render ─────────────────────────────────────────────────────────────────────

function computeProgress(iv) {
  const now   = today();
  const start = parseDate(iv.start_date);
  const end   = parseDate(iv.end_date);
  const total = diffDays(start, end);
  const past  = diffDays(start, now);
  const left  = diffDays(now, end);

  if (now < start) {
    return { status: 'upcoming', past: 0, left: diffDays(now, end), total, pct: 0 };
  }
  if (now > end) {
    return { status: 'ended', past: diffDays(end, now), left: 0, total, pct: 100 };
  }
  const pct = total > 0 ? Math.round((past / total) * 100) : 100;
  return { status: 'active', past, left, total, pct };
}

function statusLabel(status, p) {
  if (status === 'upcoming') return `starts in ${p.left} day${p.left !== 1 ? 's' : ''}`;
  if (status === 'ended')    return `ended ${p.past} day${p.past !== 1 ? 's' : ''} ago`;
  return 'in progress';
}

function renderCard(iv) {
  const p = computeProgress(iv);
  const isDeleting = pendingDeleteIds.has(iv.id);

  const card = document.createElement('div');
  card.className = 'card';
  card.dataset.id = iv.id;

  const pastText = p.status === 'upcoming'
    ? '0 days past'
    : `${p.past} day${p.past !== 1 ? 's' : ''} past`;

  const leftText = p.status === 'ended'
    ? '0 days left'
    : `${p.left} day${p.left !== 1 ? 's' : ''} left`;

  const color = iv.color || '#4f8ef7';
  card.style.setProperty('--card-color', color);

  card.innerHTML = `
    <div class="card-header">
      <div>
        <div class="card-name">${escHtml(iv.name)} <span class="status-badge ${p.status}">${statusLabel(p.status, p)}</span></div>
        <div class="card-dates">${formatDate(iv.start_date)} &ndash; ${formatDate(iv.end_date)} <span class="total-days">${p.total} days</span></div>
      </div>
      <div class="card-actions">
        <button class="btn-icon btn-edit" title="Edit" aria-label="Edit ${escHtml(iv.name)}" ${isDeleting ? 'disabled' : ''}>Edit</button>
        <button class="btn-icon danger btn-delete" title="Delete" aria-label="${isDeleting ? `Deleting ${escHtml(iv.name)}` : `Delete ${escHtml(iv.name)}`}" ${isDeleting ? 'disabled' : ''}>${isDeleting ? 'Deleting...' : 'Delete'}</button>
      </div>
    </div>
    <div class="progress-row">
      <span class="day-label past">${pastText}</span>
      <div class="bar-track"><div class="bar-fill" style="width:${p.pct}%"></div></div>
      <span class="day-label left">${leftText}</span>
    </div>
  `;

  card.querySelector('.btn-edit').addEventListener('click', () => openEdit(iv));
  card.querySelector('.btn-delete').addEventListener('click', () => confirmDelete(iv));

  return card;
}

function escHtml(str) {
  return str.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

async function loadIntervals(options = {}) {
  const preserveStatus = options.preserveStatus === true;
  const loadToken = ++activeLoadToken;

  if (!list.children.length) {
    loadingMsg?.classList.remove('hidden');
  }

  try {
    const intervals = await api.list();
    if (loadToken !== activeLoadToken) return;

    list.innerHTML = '';
    if (!intervals.length) {
      list.innerHTML = '<p class="empty-msg">No intervals yet. Add your first one above.</p>';
      if (!preserveStatus) {
        clearStatus();
      }
      return;
    }
    intervals.forEach(iv => list.appendChild(renderCard(iv)));
    if (!preserveStatus) {
      clearStatus();
    }
  } catch (err) {
    if (loadToken !== activeLoadToken) return;
    if (!list.children.length) {
      list.innerHTML = '<p class="empty-msg">Unable to load intervals. Check the connection or try again.</p>';
    }
    showStatus(err.message, { retry: true, tone: 'error' });
  }
}

// ── Modal ──────────────────────────────────────────────────────────────────────

function openAdd() {
  lastFocusedElement = document.activeElement;
  modalTitle.textContent = 'Add Interval';
  fieldId.value    = '';
  form.reset();
  fieldColor.value = '#4f8ef7';
  hideError();
  document.body.classList.add('modal-open');
  overlay.classList.remove('hidden');
  fieldName.focus();
}

function openEdit(iv) {
  lastFocusedElement = document.activeElement;
  modalTitle.textContent = 'Edit Interval';
  fieldId.value    = iv.id;
  fieldName.value  = iv.name;
  fieldStart.value = iv.start_date;
  fieldEnd.value   = iv.end_date;
  fieldColor.value = iv.color || '#4f8ef7';
  hideError();
  document.body.classList.add('modal-open');
  overlay.classList.remove('hidden');
  fieldName.focus();
}

function closeModal(force = false) {
  if (isSubmitting && !force) return;
  overlay.classList.add('hidden');
  document.body.classList.remove('modal-open');
  if (lastFocusedElement instanceof HTMLElement) {
    lastFocusedElement.focus();
  }
}

function showError(msg) {
  formError.textContent = msg;
  formError.classList.remove('hidden');
}

function hideError() {
  formError.textContent = '';
  formError.classList.add('hidden');
}

// ── Delete ─────────────────────────────────────────────────────────────────────

async function confirmDelete(iv) {
  if (pendingDeleteIds.has(iv.id)) return;
  if (!confirm(`Delete "${iv.name}"?`)) return;

  pendingDeleteIds.add(iv.id);
  await loadIntervals();
  let keepStatus = false;

  try {
    await api.delete(iv.id);
    clearStatus();
  } catch (err) {
    keepStatus = true;
    showStatus(err.message, { retry: true, tone: 'error' });
  } finally {
    pendingDeleteIds.delete(iv.id);
    await loadIntervals({ preserveStatus: keepStatus });
  }
}

// ── Events ─────────────────────────────────────────────────────────────────────

btnAdd.addEventListener('click', openAdd);
btnCancel.addEventListener('click', closeModal);
btnRetry.addEventListener('click', () => {
  clearStatus();
  loadIntervals();
});
overlay.addEventListener('click', e => { if (e.target === overlay) closeModal(); });

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') closeModal();
  if (e.key === 'Tab' && !overlay.classList.contains('hidden')) trapModalFocus(e);
});

form.addEventListener('submit', async e => {
  e.preventDefault();
  if (isSubmitting) return;
  hideError();

  const name  = fieldName.value.trim();
  const start = fieldStart.value;
  const end   = fieldEnd.value;

  if (!name)  { showError('Name is required.'); return; }
  if (!start) { showError('Start date is required.'); return; }
  if (!end)   { showError('End date is required.'); return; }
  if (start >= end) { showError('End date must be after start date.'); return; }

  const data = { name, start_date: start, end_date: end, color: fieldColor.value };

  try {
    setSubmitting(true);
    const id = fieldId.value;
    if (id) {
      await api.update(id, data);
    } else {
      await api.create(data);
    }
    clearStatus();
    closeModal(true);
    await loadIntervals();
  } catch (err) {
    showError(err.message);
  } finally {
    setSubmitting(false);
  }
});

// ── Color swatches ─────────────────────────────────────────────────────────────

colorSwatches.addEventListener('click', e => {
  const btn = e.target.closest('.swatch');
  if (btn) fieldColor.value = btn.dataset.color;
});

function setSubmitting(nextValue) {
  isSubmitting = nextValue;
  btnSave.disabled = nextValue;
  btnCancel.disabled = nextValue;
  btnAdd.disabled = nextValue;
  btnSave.textContent = nextValue ? 'Saving...' : 'Save';
}

function showStatus(message, options = {}) {
  statusMessage.textContent = message;
  appStatus.classList.remove('hidden', 'error');
  appStatus.setAttribute('role', options.tone === 'error' ? 'alert' : 'status');
  appStatus.setAttribute('aria-live', options.tone === 'error' ? 'assertive' : 'polite');
  if (options.tone === 'error') {
    appStatus.classList.add('error');
  }
  btnRetry.classList.toggle('hidden', !options.retry);
}

function clearStatus() {
  statusMessage.textContent = '';
  appStatus.classList.add('hidden');
  appStatus.classList.remove('error');
  appStatus.setAttribute('role', 'status');
  appStatus.setAttribute('aria-live', 'polite');
  btnRetry.classList.add('hidden');
}

function trapModalFocus(event) {
  const focusable = overlay.querySelectorAll(
    'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [href], [tabindex]:not([tabindex="-1"])'
  );
  if (!focusable.length) return;

  const first = focusable[0];
  const last = focusable[focusable.length - 1];

  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
    return;
  }

  if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

// ── Init ───────────────────────────────────────────────────────────────────────

loadIntervals();
