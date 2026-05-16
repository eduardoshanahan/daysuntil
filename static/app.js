'use strict';

const routePath = window.location.pathname;
const publicMatch = routePath.match(/^\/u\/([^/]+)$/);
const publicUsername = publicMatch ? decodeURIComponent(publicMatch[1]).toLowerCase() : '';
const isPublicView = publicUsername !== '';

const authView = document.getElementById('auth-view');
const appView = document.getElementById('app-view');
const list = document.getElementById('interval-list');
const profilePanel = document.getElementById('profile-panel');
const profileForm = document.getElementById('profile-form');
const profileDisplayName = document.getElementById('profile-display-name');
const profileSave = document.getElementById('profile-save');
const profileError = document.getElementById('profile-error');
const profileLink = document.getElementById('profile-link');
const publicProfileHeader = document.getElementById('public-profile-header');
const publicProfileName = document.getElementById('public-profile-name');
const publicProfileUsername = document.getElementById('public-profile-username');
const overlay = document.getElementById('modal-overlay');
const form = document.getElementById('interval-form');
const modalTitle = document.getElementById('modal-title');
const fieldId = document.getElementById('field-id');
const fieldName = document.getElementById('field-name');
const fieldStart = document.getElementById('field-start');
const fieldEnd = document.getElementById('field-end');
const fieldColor = document.getElementById('field-color');
const fieldVisibility = document.getElementById('field-visibility');
const btnPickStart = document.getElementById('btn-pick-start');
const btnPickEnd = document.getElementById('btn-pick-end');
const formError = document.getElementById('form-error');
const btnSave = document.getElementById('btn-save');
const btnAdd = document.getElementById('btn-add');
const btnCancel = document.getElementById('btn-cancel');
const btnLogout = document.getElementById('btn-logout');
const btnRetry = document.getElementById('btn-retry');
const loadingMsg = document.getElementById('loading-msg');
const appStatus = document.getElementById('app-status');
const statusMessage = document.getElementById('status-message');
const colorSwatches = document.getElementById('color-swatches');
const userBadge = document.getElementById('user-badge');
const datePicker = document.getElementById('date-picker');
const datePickerTitle = document.getElementById('date-picker-title');
const datePickerGrid = document.getElementById('date-picker-grid');
const datePrev = document.getElementById('date-prev');
const dateNext = document.getElementById('date-next');

const authForm = document.getElementById('auth-form');
const authUsername = document.getElementById('auth-username');
const authPassword = document.getElementById('auth-password');
const authError = document.getElementById('auth-error');
const authSubmit = document.getElementById('auth-submit');
const authSwitch = document.getElementById('auth-switch');
const authSwitchLabel = document.getElementById('auth-switch-label');
const authKicker = document.getElementById('auth-kicker');
const authTitle = document.getElementById('auth-title');
const authSubtitle = document.getElementById('auth-subtitle');
const authOAuth = document.getElementById('auth-oauth');
const authGithub = document.getElementById('auth-github');

let isSubmitting = false;
let isAuthSubmitting = false;
let isProfileSubmitting = false;
let isRegisterMode = false;
let activeLoadToken = 0;
const pendingDeleteIds = new Set();
let lastFocusedElement = null;
let currentUser = null;
let activeDateField = null;
let visibleMonth = null;

class ApiError extends Error {
  constructor(message, status) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

async function apiFetch(path, options = {}) {
  let res;
  try {
    res = await fetch(path, {
      headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
      ...options,
    });
  } catch {
    throw new ApiError('Network error. Check your connection and try again.', 0);
  }

  if (!res.ok) {
    const msg = (await res.text()).trim();
    throw new ApiError(msg || `Request failed with status ${res.status}.`, res.status);
  }
  if (res.status === 204) return null;
  return res.json();
}

const api = {
  providers: () => apiFetch('/api/auth/providers'),
  me: () => apiFetch('/api/me'),
  updateProfile: data => apiFetch('/api/me/profile', { method: 'PUT', body: JSON.stringify(data) }),
  register: data => apiFetch('/api/register', { method: 'POST', body: JSON.stringify(data) }),
  login: data => apiFetch('/api/login', { method: 'POST', body: JSON.stringify(data) }),
  logout: () => apiFetch('/api/logout', { method: 'POST' }),
  list: () => apiFetch('/api/intervals'),
  create: data => apiFetch('/api/intervals', { method: 'POST', body: JSON.stringify(data) }),
  update: (id, data) => apiFetch(`/api/intervals/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: id => apiFetch(`/api/intervals/${id}`, { method: 'DELETE' }),
  publicProfile: username => apiFetch(`/api/public/users/${encodeURIComponent(username)}`),
};

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
  const total = diffDays(start, end);
  const past = diffDays(start, now);
  const left = diffDays(now, end);

  if (now < start) {
    return { status: 'upcoming', past: 0, left: diffDays(now, end), total, pct: 0 };
  }
  if (now > end) {
    return { status: 'ended', past: diffDays(end, now), left: 0, total, pct: 100 };
  }
  const pct = total > 0 ? Math.round((past / total) * 100) : 100;
  return { status: 'active', past, left, total, pct };
}

function statusLabel(status, progress) {
  if (status === 'upcoming') return `starts in ${progress.left} day${progress.left !== 1 ? 's' : ''}`;
  if (status === 'ended') return `ended ${progress.past} day${progress.past !== 1 ? 's' : ''} ago`;
  return 'in progress';
}

function escHtml(str) {
  return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function renderCard(iv, options = {}) {
  const progress = computeProgress(iv);
  const isDeleting = pendingDeleteIds.has(iv.id);
  const showActions = options.showActions === true;
  const showVisibility = options.showVisibility === true;

  const card = document.createElement('div');
  card.className = 'card';
  card.dataset.id = iv.id;

  const pastText = progress.status === 'upcoming'
    ? '0 days past'
    : `${progress.past} day${progress.past !== 1 ? 's' : ''} past`;
  const leftText = progress.status === 'ended'
    ? '0 days left'
    : `${progress.left} day${progress.left !== 1 ? 's' : ''} left`;

  const color = iv.color || '#4f8ef7';
  card.style.setProperty('--card-color', color);

  const visibilityBadge = showVisibility
    ? `<span class="status-badge ${iv.visibility === 'public' ? 'active' : 'upcoming'}">${iv.visibility}</span>`
    : '';

  const actions = showActions
    ? `<div class="card-actions">
        <button class="btn-icon btn-edit" title="Edit" aria-label="Edit ${escHtml(iv.name)}" ${isDeleting ? 'disabled' : ''}>Edit</button>
        <button class="btn-icon danger btn-delete" title="Delete" aria-label="${isDeleting ? `Deleting ${escHtml(iv.name)}` : `Delete ${escHtml(iv.name)}`}" ${isDeleting ? 'disabled' : ''}>${isDeleting ? 'Deleting...' : 'Delete'}</button>
      </div>`
    : '';

  card.innerHTML = `
    <div class="card-header">
      <div>
        <div class="card-name">${escHtml(iv.name)} ${visibilityBadge}<span class="status-badge ${progress.status}">${statusLabel(progress.status, progress)}</span></div>
        <div class="card-dates">${formatDate(iv.start_date)} &ndash; ${formatDate(iv.end_date)} <span class="total-days">${progress.total} days</span></div>
      </div>
      ${actions}
    </div>
    <div class="progress-row">
      <span class="day-label past">${pastText}</span>
      <div class="bar-track"><div class="bar-fill" style="width:${progress.pct}%"></div></div>
      <span class="day-label left">${leftText}</span>
    </div>
  `;

  if (showActions) {
    card.querySelector('.btn-edit').addEventListener('click', () => openEdit(iv));
    card.querySelector('.btn-delete').addEventListener('click', () => confirmDelete(iv));
  }

  return card;
}

function setCurrentUser(user) {
  currentUser = user;
  const isAuthenticated = Boolean(user);

  authView.classList.toggle('hidden', isAuthenticated || isPublicView);
  appView.classList.toggle('hidden', !isAuthenticated && !isPublicView);
  btnAdd.classList.toggle('hidden', !isAuthenticated || isPublicView);
  btnLogout.classList.toggle('hidden', !isAuthenticated || isPublicView);
  userBadge.classList.toggle('hidden', !isAuthenticated || isPublicView);
  profilePanel.classList.toggle('hidden', !isAuthenticated || isPublicView);
  publicProfileHeader.classList.toggle('hidden', !isPublicView);

  if (isAuthenticated) {
    userBadge.textContent = user.display_name || user.username;
    profileDisplayName.value = user.display_name || user.username;
    profileLink.textContent = `${window.location.origin}/u/${user.username}`;
    authPassword.value = '';
    clearAuthError();
  } else {
    userBadge.textContent = '';
    btnAdd.disabled = false;
    closeModal(true);
  }
}

function renderIntervals(intervals, options = {}) {
  list.innerHTML = '';
  if (!intervals.length) {
    list.innerHTML = `<p class="empty-msg">${options.emptyMessage || 'No intervals yet.'}</p>`;
    return;
  }
  intervals.forEach(iv => list.appendChild(renderCard(iv, options)));
}

async function loadIntervals(options = {}) {
  const preserveStatus = options.preserveStatus === true;
  const loadToken = ++activeLoadToken;
  if (!currentUser) return;

  if (!list.children.length) {
    loadingMsg?.classList.remove('hidden');
  }

  try {
    const intervals = await api.list();
    if (loadToken !== activeLoadToken) return;
    renderIntervals(intervals, {
      showActions: true,
      showVisibility: true,
      emptyMessage: 'No intervals yet. Add your first one above.',
    });
    if (!preserveStatus) clearStatus();
  } catch (err) {
    if (loadToken !== activeLoadToken) return;
    if (err.status === 401) {
      handleUnauthorized('Your session has ended. Log in again.');
      return;
    }
    if (!list.children.length) {
      list.innerHTML = '<p class="empty-msg">Unable to load intervals. Check the connection or try again.</p>';
    }
    showStatus(err.message, { retry: true, tone: 'error' });
  }
}

async function loadPublicProfile() {
  list.innerHTML = '<p class="empty-msg">Loading public profile...</p>';
  try {
    const profile = await api.publicProfile(publicUsername);
    document.title = `${profile.display_name || profile.username} | Days Until`;
    publicProfileName.textContent = profile.display_name || profile.username;
    publicProfileUsername.textContent = `@${profile.username}`;
    renderIntervals(profile.intervals || [], {
      showActions: false,
      showVisibility: false,
      emptyMessage: 'No public intervals to show yet.',
    });
    clearStatus();
  } catch (err) {
    list.innerHTML = `<p class="empty-msg">${err.status === 404 ? 'Public profile not found.' : 'Unable to load public profile.'}</p>`;
    if (err.status !== 404) {
      showStatus(err.message, { tone: 'error' });
    }
  }
}

function openAdd() {
  lastFocusedElement = document.activeElement;
  modalTitle.textContent = 'Add Interval';
  fieldId.value = '';
  form.reset();
  fieldColor.value = '#4f8ef7';
  fieldVisibility.value = 'private';
  hideError();
  document.body.classList.add('modal-open');
  overlay.classList.remove('hidden');
  fieldName.focus();
}

function openEdit(iv) {
  lastFocusedElement = document.activeElement;
  modalTitle.textContent = 'Edit Interval';
  fieldId.value = iv.id;
  fieldName.value = iv.name;
  fieldStart.value = iv.start_date;
  fieldEnd.value = iv.end_date;
  fieldColor.value = iv.color || '#4f8ef7';
  fieldVisibility.value = iv.visibility || 'private';
  hideError();
  document.body.classList.add('modal-open');
  overlay.classList.remove('hidden');
  fieldName.focus();
}

function closeModal(force = false) {
  if (isSubmitting && !force) return;
  hideDatePicker();
  overlay.classList.add('hidden');
  document.body.classList.remove('modal-open');
  if (lastFocusedElement instanceof HTMLElement) {
    lastFocusedElement.focus();
  }
}

function showError(message) {
  formError.textContent = message;
  formError.classList.remove('hidden');
}

function hideError() {
  formError.textContent = '';
  formError.classList.add('hidden');
}

function showProfileError(message) {
  profileError.textContent = message;
  profileError.classList.remove('hidden');
}

function clearProfileError() {
  profileError.textContent = '';
  profileError.classList.add('hidden');
}

function openDatePicker(field) {
  activeDateField = field;
  const currentValue = field.value;
  const baseDate = isValidISODate(currentValue) ? parseDate(currentValue) : today();
  visibleMonth = new Date(baseDate.getFullYear(), baseDate.getMonth(), 1);
  renderDatePicker();
  datePicker.classList.remove('hidden');
}

function hideDatePicker() {
  datePicker.classList.add('hidden');
  activeDateField = null;
}

function renderDatePicker() {
  if (!visibleMonth) return;
  datePickerTitle.textContent = monthLabel(visibleMonth);
  datePickerGrid.innerHTML = '';

  const year = visibleMonth.getFullYear();
  const month = visibleMonth.getMonth();
  const firstDay = new Date(year, month, 1);
  const startOffset = (firstDay.getDay() + 6) % 7;
  const gridStart = new Date(year, month, 1 - startOffset);
  const selectedValue = activeDateField && isValidISODate(activeDateField.value) ? activeDateField.value : '';
  const todayValue = formatISODate(today());

  for (let index = 0; index < 42; index += 1) {
    const date = new Date(gridStart.getFullYear(), gridStart.getMonth(), gridStart.getDate() + index);
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'date-day';
    button.textContent = `${date.getDate()}`;
    button.dataset.value = formatISODate(date);
    if (date.getMonth() !== month) button.classList.add('muted');
    if (button.dataset.value === todayValue) button.classList.add('today');
    if (button.dataset.value === selectedValue) button.classList.add('selected');
    button.addEventListener('click', () => {
      if (!activeDateField) return;
      activeDateField.value = button.dataset.value;
      hideError();
      hideDatePicker();
      activeDateField.focus();
    });
    datePickerGrid.appendChild(button);
  }
}

function showAuthError(message) {
  authError.textContent = message;
  authError.classList.remove('hidden');
}

function clearAuthError() {
  authError.textContent = '';
  authError.classList.add('hidden');
}

function setAuthMode(registerMode) {
  isRegisterMode = registerMode;
  authKicker.textContent = registerMode ? 'Create account' : 'Login';
  authTitle.textContent = registerMode ? 'Create your account' : 'Sign in to your account';
  authSubtitle.textContent = registerMode
    ? 'Each account gets its own private interval list, with optional public cards.'
    : 'Your intervals stay private unless you mark them public.';
  authSubmit.textContent = registerMode ? 'Create account' : 'Log in';
  authSwitchLabel.textContent = registerMode ? 'Already have an account?' : 'Need an account?';
  authSwitch.textContent = registerMode ? 'Log in instead' : 'Create one';
  authPassword.autocomplete = registerMode ? 'new-password' : 'current-password';
  clearAuthError();
}

function setAuthSubmitting(nextValue) {
  isAuthSubmitting = nextValue;
  authSubmit.disabled = nextValue;
  authSwitch.disabled = nextValue;
  authUsername.disabled = nextValue;
  authPassword.disabled = nextValue;
  authGithub.classList.toggle('disabled', nextValue);
  authGithub.setAttribute('aria-disabled', nextValue ? 'true' : 'false');
}

function setProfileSubmitting(nextValue) {
  isProfileSubmitting = nextValue;
  profileSave.disabled = nextValue;
  profileDisplayName.disabled = nextValue;
}

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
  if (options.tone === 'error') appStatus.classList.add('error');
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
  const focusable = overlay.querySelectorAll('button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [href], [tabindex]:not([tabindex="-1"])');
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

function handleUnauthorized(message) {
  setCurrentUser(null);
  clearStatus();
  list.innerHTML = '<p id="loading-msg" class="empty-msg">Loading intervals...</p>';
  setAuthMode(false);
  showAuthError(message);
  authUsername.focus();
}

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
    if (err.status === 401) {
      handleUnauthorized('Your session has ended. Log in again.');
      return;
    }
    keepStatus = true;
    showStatus(err.message, { retry: true, tone: 'error' });
  } finally {
    pendingDeleteIds.delete(iv.id);
    if (currentUser) {
      await loadIntervals({ preserveStatus: keepStatus });
    }
  }
}

btnAdd.addEventListener('click', openAdd);
btnCancel.addEventListener('click', closeModal);
btnLogout.addEventListener('click', async () => {
  try {
    await api.logout();
  } catch {
    // Keep going even if server-side cleanup fails.
  }
  handleUnauthorized('Logged out.');
});
btnRetry.addEventListener('click', () => {
  clearStatus();
  if (isPublicView) {
    loadPublicProfile();
  } else {
    loadIntervals();
  }
});
authSwitch.addEventListener('click', () => {
  setAuthMode(!isRegisterMode);
  authUsername.focus();
});
overlay.addEventListener('click', event => {
  if (event.target === overlay) closeModal();
});
datePrev.addEventListener('click', () => {
  if (!visibleMonth) return;
  visibleMonth = new Date(visibleMonth.getFullYear(), visibleMonth.getMonth() - 1, 1);
  renderDatePicker();
});
dateNext.addEventListener('click', () => {
  if (!visibleMonth) return;
  visibleMonth = new Date(visibleMonth.getFullYear(), visibleMonth.getMonth() + 1, 1);
  renderDatePicker();
});
btnPickStart.addEventListener('click', () => openDatePicker(fieldStart));
btnPickEnd.addEventListener('click', () => openDatePicker(fieldEnd));
fieldStart.addEventListener('focus', () => openDatePicker(fieldStart));
fieldEnd.addEventListener('focus', () => openDatePicker(fieldEnd));
document.addEventListener('click', event => {
  if (datePicker.classList.contains('hidden')) return;
  const clickedInsidePicker = datePicker.contains(event.target);
  const clickedTrigger = [fieldStart, fieldEnd, btnPickStart, btnPickEnd].includes(event.target);
  if (!clickedInsidePicker && !clickedTrigger) {
    hideDatePicker();
  }
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape') closeModal();
  if (event.key === 'Tab' && !overlay.classList.contains('hidden')) trapModalFocus(event);
});

authForm.addEventListener('submit', async event => {
  event.preventDefault();
  if (isAuthSubmitting) return;

  clearAuthError();
  const username = authUsername.value.trim();
  const password = authPassword.value;
  if (!username) return showAuthError('Username is required.');
  if (!password) return showAuthError('Password is required.');

  try {
    setAuthSubmitting(true);
    const user = isRegisterMode
      ? await api.register({ username, password })
      : await api.login({ username, password });
    setCurrentUser(user);
    clearStatus();
    list.innerHTML = '<p id="loading-msg" class="empty-msg">Loading intervals...</p>';
    await loadIntervals();
  } catch (err) {
    showAuthError(err.message);
  } finally {
    setAuthSubmitting(false);
  }
});

profileForm.addEventListener('submit', async event => {
  event.preventDefault();
  if (isProfileSubmitting || !currentUser) return;

  clearProfileError();
  const displayName = profileDisplayName.value.trim();
  if (!displayName) return showProfileError('Display name is required.');

  try {
    setProfileSubmitting(true);
    const updatedUser = await api.updateProfile({ display_name: displayName });
    setCurrentUser(updatedUser);
    showStatus('Display name updated.', { tone: 'status' });
  } catch (err) {
    if (err.status === 401) {
      handleUnauthorized('Your session has ended. Log in again.');
      return;
    }
    showProfileError(err.message);
  } finally {
    setProfileSubmitting(false);
  }
});

form.addEventListener('submit', async event => {
  event.preventDefault();
  if (isSubmitting) return;
  hideError();

  const name = fieldName.value.trim();
  const start = fieldStart.value;
  const end = fieldEnd.value;
  const visibility = fieldVisibility.value;

  if (!name) return showError('Name is required.');
  if (!start) return showError('Start date is required.');
  if (!end) return showError('End date is required.');
  if (!isValidISODate(start)) return showError('Start date must be in YYYY-MM-DD format.');
  if (!isValidISODate(end)) return showError('End date must be in YYYY-MM-DD format.');
  if (start >= end) return showError('End date must be after start date.');

  const data = { name, start_date: start, end_date: end, color: fieldColor.value, visibility };

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
    if (err.status === 401) {
      handleUnauthorized('Your session has ended. Log in again.');
      return;
    }
    showError(err.message);
  } finally {
    setSubmitting(false);
  }
});

colorSwatches.addEventListener('click', event => {
  const btn = event.target.closest('.swatch');
  if (btn) fieldColor.value = btn.dataset.color;
});

async function initPrivateApp() {
  setAuthMode(false);
  const params = new URLSearchParams(window.location.search);
  const authErrorMessage = params.get('auth_error');

  try {
    const providers = await api.providers();
    authOAuth.classList.toggle('hidden', !providers.github_enabled);
  } catch {
    authOAuth.classList.add('hidden');
  }

  try {
    const user = await api.me();
    setCurrentUser(user);
    await loadIntervals();
  } catch (err) {
    setCurrentUser(null);
    authView.classList.remove('hidden');
    if (authErrorMessage) {
      showAuthError(authErrorMessage);
      window.history.replaceState({}, '', window.location.pathname);
    } else if (err.status !== 401) {
      showAuthError(err.message);
    }
    authUsername.focus();
  }
}

function initPublicView() {
  authView.classList.add('hidden');
  btnAdd.classList.add('hidden');
  btnLogout.classList.add('hidden');
  userBadge.classList.add('hidden');
  profilePanel.classList.add('hidden');
  publicProfileHeader.classList.remove('hidden');
  appView.classList.remove('hidden');
  loadPublicProfile();
}

if (isPublicView) {
  initPublicView();
} else {
  initPrivateApp();
}
