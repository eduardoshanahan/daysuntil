const { api } = window.DaysUntilApi;
const {
  computeProgress,
  formatDate,
  formatISODate,
  isValidISODate,
  monthLabel,
  parseDate,
  statusLabel,
  today,
} = window.DaysUntilDates;
const {
  accountSummary,
  appStatus,
  appVersion,
  appView,
  authEmail,
  authError,
  authForm,
  authGithub,
  authKicker,
  authOAuth,
  authPassword,
  authSubmit,
  authSubtitle,
  authSwitch,
  authSwitchLabel,
  authTitle,
  authUsername,
  authUsernameRow,
  authView,
  btnAdd,
  btnCancel,
  btnLogout,
  btnMenu,
  mobileMenu,
  menuGroupsBadge,
  menuLogout,
  menuUserBadge,
  menuVersion,
  btnPickEnd,
  btnPickStart,
  btnRetry,
  btnSave,
  colorSwatches,
  dateNext,
  datePicker,
  datePickerGrid,
  datePickerTitle,
  datePrev,
  fieldColor,
  fieldEnd,
  fieldId,
  fieldName,
  fieldShareGroup,
  fieldStart,
  form,
  formError,
  groupsBadge,
  intervalFilterBar,
  intervalTools,
  isPublicView,
  list,
  loadingMsg,
  modalTitle,
  overlay,
  profileDeleteAccount,
  profileDisplayName,
  profileError,
  profileForm,
  profilePanel,
  profileSave,
  publicGroupHeader,
  publicGroupName,
  publicGroupOwner,
  publicGroupSlug,
  shareGroupCreate,
  shareGroupError,
  shareGroupForm,
  shareGroupName,
  shareGroupsList,
  shareGroupsPanel,
  shareGroupsSummary,
  statusMessage,
  userBadge,
} = window.DaysUntilDom;
const { escHtml } = window.DaysUntilUtils;

let isSubmitting = false;
let isAuthSubmitting = false;
let isProfileSubmitting = false;
let isShareGroupSubmitting = false;
let isRegisterMode = false;
let activeLoadToken = 0;
const pendingDeleteIds = new Set();
let lastFocusedElement = null;
let currentUser = null;
let currentShareGroups = [];
let currentIntervals = [];
let activeIntervalFilter = 'all';
let activeDateField = null;
let visibleMonth = null;
let statusTimer = null;
let midnightTimer = null;
let currentDayStamp = formatISODate(today());
let currentPublicGroup = null;

function nextMidnightDelay() {
  const now = new Date();
  const nextMidnight = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
  return Math.max(1000, nextMidnight.getTime() - now.getTime() + 1000);
}

function scheduleMidnightRefresh() {
  if (midnightTimer) {
    clearTimeout(midnightTimer);
  }
  midnightTimer = window.setTimeout(() => {
    refreshForDateChange();
  }, nextMidnightDelay());
}

function refreshForDateChange() {
  const nextDayStamp = formatISODate(today());
  if (nextDayStamp === currentDayStamp) {
    scheduleMidnightRefresh();
    return;
  }

  currentDayStamp = nextDayStamp;
  if (isPublicView) {
    if (currentPublicGroup) {
      renderPublicGroup(currentPublicGroup);
    }
  } else if (currentUser) {
    renderCurrentIntervals();
  }

  if (!datePicker.classList.contains('hidden')) {
    renderDatePicker();
  }
  scheduleMidnightRefresh();
}

function renderShareGroupOptions(selectedID = null) {
  fieldShareGroup.innerHTML = '<option value="">Private</option>';
  currentShareGroups.forEach(group => {
    const option = document.createElement('option');
    option.value = String(group.id);
    option.textContent = group.name;
    fieldShareGroup.appendChild(option);
  });
  fieldShareGroup.value = selectedID == null ? '' : String(selectedID);
}

function intervalCountsByGroup() {
  const counts = new Map();
  let privateCount = 0;
  for (const interval of currentIntervals) {
    if (interval.share_group_id == null) {
      privateCount += 1;
      continue;
    }
    counts.set(interval.share_group_id, (counts.get(interval.share_group_id) || 0) + 1);
  }
  return { counts, privateCount, total: currentIntervals.length };
}

function filteredIntervals() {
  if (activeIntervalFilter === 'all') return currentIntervals;
  if (activeIntervalFilter === 'private') {
    return currentIntervals.filter(interval => interval.share_group_id == null);
  }
  const groupID = Number(activeIntervalFilter);
  return currentIntervals.filter(interval => interval.share_group_id === groupID);
}

function renderIntervalFilters() {
  if (!currentUser || isPublicView) {
    intervalTools.classList.add('hidden');
    intervalFilterBar.innerHTML = '';
    return;
  }

  intervalTools.classList.remove('hidden');
  const { counts, privateCount, total } = intervalCountsByGroup();
  const filterItems = [
    { key: 'all', label: `All (${total})` },
    { key: 'private', label: `Private (${privateCount})` },
    ...currentShareGroups.map(group => ({
      key: String(group.id),
      label: `${group.name} (${counts.get(group.id) || 0})`,
    })),
  ];

  if (activeIntervalFilter !== 'all' && activeIntervalFilter !== 'private' && !currentShareGroups.some(group => String(group.id) === activeIntervalFilter)) {
    activeIntervalFilter = 'all';
  }

  intervalFilterBar.innerHTML = '';
  for (const item of filterItems) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `filter-chip${activeIntervalFilter === item.key ? ' active' : ''}`;
    button.textContent = item.label;
    button.addEventListener('click', () => {
      if (activeIntervalFilter === item.key) return;
      activeIntervalFilter = item.key;
      renderIntervalFilters();
      renderIntervals(filteredIntervals(), {
        showActions: true,
        emptyMessage: 'No intervals match this filter.',
      });
    });
    intervalFilterBar.appendChild(button);
  }
}

function updateManagementSummaries() {
  if (!currentUser) {
    accountSummary.textContent = 'Manage your display name and account settings.';
    shareGroupsSummary.textContent = 'Create public collections of related intervals.';
    profilePanel.classList.add('hidden');
    profilePanel.open = false;
    shareGroupsPanel.classList.add('hidden');
    shareGroupsPanel.open = false;
    userBadge.setAttribute('aria-expanded', 'false');
    groupsBadge.setAttribute('aria-expanded', 'false');
    return;
  }

  accountSummary.textContent = `Signed in as ${currentUser.display_name || currentUser.username}.`;
  const { counts, total } = intervalCountsByGroup();
  let sharedIntervals = 0;
  for (const count of counts.values()) sharedIntervals += count;
  const groupCount = currentShareGroups.length;
  shareGroupsSummary.textContent = `${groupCount} group${groupCount !== 1 ? 's' : ''}, ${sharedIntervals} shared interval${sharedIntervals !== 1 ? 's' : ''}, ${total - sharedIntervals} private.`;
  userBadge.setAttribute('aria-expanded', profilePanel.open ? 'true' : 'false');
  groupsBadge.setAttribute('aria-expanded', shareGroupsPanel.open ? 'true' : 'false');
}

function renderCard(iv, options = {}) {
  const progress = computeProgress(iv);
  const isDeleting = pendingDeleteIds.has(iv.id);
  const showActions = options.showActions === true;
  const showShareBadge = options.showShareBadge !== false;
  const canMoveUp = options.canMoveUp === true;
  const canMoveDown = options.canMoveDown === true;

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

  const shareBadge = !showShareBadge
    ? ''
    : iv.share_group_name
      ? `<span class="status-badge share-badge">${escHtml(iv.share_group_name)}</span>`
      : `<span class="status-badge private-badge">private</span>`;

  const actions = showActions
    ? `<div class="card-actions">
        <button type="button" class="btn-icon btn-move-up" title="Move up" aria-label="Move ${escHtml(iv.name)} up" ${!canMoveUp || isDeleting ? 'disabled' : ''}>Up</button>
        <button type="button" class="btn-icon btn-move-down" title="Move down" aria-label="Move ${escHtml(iv.name)} down" ${!canMoveDown || isDeleting ? 'disabled' : ''}>Down</button>
        <button type="button" class="btn-icon btn-edit" title="Edit" aria-label="Edit ${escHtml(iv.name)}" ${isDeleting ? 'disabled' : ''}>Edit</button>
        <button type="button" class="btn-icon danger btn-delete" title="Delete" aria-label="${isDeleting ? `Deleting ${escHtml(iv.name)}` : `Delete ${escHtml(iv.name)}`}" ${isDeleting ? 'disabled' : ''}>${isDeleting ? 'Deleting...' : 'Delete'}</button>
      </div>
      <button type="button" class="btn-icon btn-card-menu" aria-label="Actions for ${escHtml(iv.name)}" aria-expanded="false" aria-haspopup="true">&#8942;</button>`
    : '';

  const cardMenu = showActions
    ? `<div class="card-menu hidden" role="menu">
        <button type="button" class="card-menu-item btn-move-up" role="menuitem" ${!canMoveUp || isDeleting ? 'disabled' : ''}>&#8593; Move up</button>
        <button type="button" class="card-menu-item btn-move-down" role="menuitem" ${!canMoveDown || isDeleting ? 'disabled' : ''}>&#8595; Move down</button>
        <div class="card-menu-divider"></div>
        <button type="button" class="card-menu-item btn-edit" role="menuitem" ${isDeleting ? 'disabled' : ''}>&#9998; Edit</button>
        <button type="button" class="card-menu-item danger btn-delete" role="menuitem" ${isDeleting ? 'disabled' : ''}>${isDeleting ? 'Deleting...' : '&#10005; Delete'}</button>
      </div>`
    : '';

  card.innerHTML = `
    <div class="card-header">
      <div>
        <div class="card-name">${escHtml(iv.name)} ${shareBadge}<span class="status-badge ${progress.status}">${statusLabel(progress.status, progress)}</span></div>
        <div class="card-dates">${formatDate(iv.start_date)} &ndash; ${formatDate(iv.end_date)} <span class="total-days">${progress.total} days</span></div>
      </div>
      ${actions}
    </div>
    <div class="progress-row">
      <span class="day-label past">${pastText}</span>
      <div class="bar-track"><div class="bar-fill"></div></div>
      <span class="day-label left">${leftText}</span>
    </div>
    ${cardMenu}
  `;
  card.querySelector('.bar-fill').style.width = `${progress.pct}%`;
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
  groupsBadge.classList.toggle('hidden', !isAuthenticated || isPublicView);
  btnMenu.classList.toggle('hidden', !isAuthenticated || isPublicView);
  publicGroupHeader.classList.toggle('hidden', !isPublicView);

  if (isAuthenticated) {
    profilePanel.classList.add('hidden');
    profilePanel.open = false;
    shareGroupsPanel.classList.add('hidden');
    shareGroupsPanel.open = false;
    userBadge.textContent = user.display_name || user.username;
    menuUserBadge.textContent = user.display_name || user.username;
    profileDisplayName.value = user.display_name || user.username;
    updateManagementSummaries();
    authEmail.value = '';
    authUsername.value = '';
    authPassword.value = '';
    clearAuthError();
  } else {
    userBadge.textContent = '';
    menuUserBadge.textContent = '';
    closeMobileMenu();
    currentShareGroups = [];
    currentIntervals = [];
    activeIntervalFilter = 'all';
    renderShareGroupOptions();
    shareGroupsList.innerHTML = '';
    intervalTools.classList.add('hidden');
    intervalFilterBar.innerHTML = '';
    updateManagementSummaries();
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
  intervals.forEach((iv, index) => list.appendChild(renderCard(iv, {
    ...options,
    canMoveUp: options.showActions === true && index > 0,
    canMoveDown: options.showActions === true && index < intervals.length - 1,
  })));
}

function renderCurrentIntervals(options = {}) {
  renderIntervalFilters();
  updateManagementSummaries();
  renderIntervals(filteredIntervals(), {
    showActions: true,
    emptyMessage: options.emptyMessage || 'No intervals yet. Add your first one above.',
  });
}

function setVersionLabel(label) {
  appVersion.textContent = label || 'dev';
  menuVersion.textContent = label || 'dev';
}

function closeMobileMenu() {
  mobileMenu.classList.add('hidden');
  btnMenu.setAttribute('aria-expanded', 'false');
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
    currentIntervals = intervals || [];
    renderCurrentIntervals();
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

function renderShareGroups() {
  if (!currentShareGroups.length) {
    shareGroupsList.innerHTML = '<p class="empty-msg">No share groups yet. Create one to start sharing selected intervals.</p>';
    return;
  }

  const { counts } = intervalCountsByGroup();
  shareGroupsList.innerHTML = '';
  currentShareGroups.forEach(group => {
    const card = document.createElement('div');
    card.className = 'share-group-card';
    const publicURL = `${window.location.origin}/g/${group.public_slug}`;
    const groupCount = counts.get(group.id) || 0;
    card.innerHTML = `
      <label class="share-group-label" for="share-group-edit-${group.id}">Group name</label>
      <div class="share-group-edit-row">
        <input id="share-group-edit-${group.id}" type="text" value="${escHtml(group.name)}" maxlength="80" ${isShareGroupSubmitting ? 'disabled' : ''} />
        <button type="button" class="btn-secondary btn-group-save" ${isShareGroupSubmitting ? 'disabled' : ''}>Save</button>
      </div>
      <p class="share-group-meta">${groupCount} interval${groupCount !== 1 ? 's' : ''} in this group</p>
      <p><a class="profile-link" target="_blank" rel="noopener noreferrer" href="${publicURL}">${publicURL}</a></p>
      <div class="share-group-actions">
        <button type="button" class="btn-secondary btn-group-open" ${isShareGroupSubmitting ? 'disabled' : ''}>Open</button>
        <button type="button" class="btn-secondary btn-group-filter" ${isShareGroupSubmitting ? 'disabled' : ''}>Show in list</button>
        <button type="button" class="btn-secondary btn-group-rotate" ${isShareGroupSubmitting ? 'disabled' : ''}>Rotate link</button>
        <button type="button" class="btn-secondary danger btn-group-delete" ${isShareGroupSubmitting ? 'disabled' : ''}>Delete group</button>
      </div>
    `;

    const nameInput = card.querySelector(`#share-group-edit-${group.id}`);
    card.querySelector('.btn-group-save').addEventListener('click', () => saveShareGroup(group.id, nameInput.value.trim()));
    card.querySelector('.btn-group-open').addEventListener('click', () => window.open(publicURL, '_blank', 'noopener,noreferrer'));
    card.querySelector('.btn-group-filter').addEventListener('click', () => {
      activeIntervalFilter = String(group.id);
      renderIntervalFilters();
      renderIntervals(filteredIntervals(), {
        showActions: true,
        emptyMessage: 'No intervals match this filter.',
      });
    });
    card.querySelector('.btn-group-rotate').addEventListener('click', () => rotateShareGroup(group));
    card.querySelector('.btn-group-delete').addEventListener('click', () => removeShareGroup(group));
    shareGroupsList.appendChild(card);
  });
}

async function loadShareGroups() {
  if (!currentUser) return;
  const groups = await api.listGroups();
  currentShareGroups = groups || [];
  if (currentShareGroups.length === 0) {
    shareGroupsPanel.open = true;
  }
  renderShareGroupOptions();
  renderIntervalFilters();
  updateManagementSummaries();
  renderShareGroups();
}

async function loadPublicGroup() {
  list.innerHTML = '<p class="empty-msg">Loading shared group...</p>';
  try {
    const group = await api.publicGroup(publicGroupSlug);
    currentPublicGroup = group;
    renderPublicGroup(group);
    clearStatus();
  } catch (err) {
    list.innerHTML = `<p class="empty-msg">${err.status === 404 ? 'Shared group not found.' : 'Unable to load shared group.'}</p>`;
    if (err.status !== 404) {
      showStatus(err.message, { tone: 'error' });
    }
  }
}

function renderPublicGroup(group) {
  document.title = `${group.name} | Days Until`;
  publicGroupName.textContent = group.name;
  publicGroupOwner.textContent = group.owner_name
    ? `Shared by ${group.owner_name}`
    : `Shared by @${group.owner_username}`;
  renderIntervals(group.intervals || [], {
      showActions: false,
      showShareBadge: false,
      emptyMessage: 'No shared intervals to show yet.',
  });
}

function openAdd() {
  lastFocusedElement = document.activeElement;
  modalTitle.textContent = 'Add Interval';
  fieldId.value = '';
  form.reset();
  fieldColor.value = '#4f8ef7';
  renderShareGroupOptions();
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
  renderShareGroupOptions(iv.share_group_id);
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

function showShareGroupError(message) {
  shareGroupError.textContent = message;
  shareGroupError.classList.remove('hidden');
}

function clearShareGroupError() {
  shareGroupError.textContent = '';
  shareGroupError.classList.add('hidden');
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
    ? 'Use your email to sign in, and choose a separate public username.'
    : 'Sign in with your email. Share groups stay separate from your account identity.';
  authSubmit.textContent = registerMode ? 'Create account' : 'Log in';
  authSwitchLabel.textContent = registerMode ? 'Already have an account?' : 'Need an account?';
  authSwitch.textContent = registerMode ? 'Log in instead' : 'Create one';
  authUsernameRow.classList.toggle('hidden', !registerMode);
  authUsername.required = registerMode;
  authEmail.autocomplete = 'email';
  authPassword.autocomplete = registerMode ? 'new-password' : 'current-password';
  clearAuthError();
}

function setAuthSubmitting(nextValue) {
  isAuthSubmitting = nextValue;
  authSubmit.disabled = nextValue;
  authSwitch.disabled = nextValue;
  authEmail.disabled = nextValue;
  authUsername.disabled = nextValue;
  authPassword.disabled = nextValue;
  authGithub.classList.toggle('disabled', nextValue);
  authGithub.setAttribute('aria-disabled', nextValue ? 'true' : 'false');
}

function setProfileSubmitting(nextValue) {
  isProfileSubmitting = nextValue;
  profileSave.disabled = nextValue;
  profileDeleteAccount.disabled = nextValue;
  profileDisplayName.disabled = nextValue;
}

function setShareGroupSubmitting(nextValue) {
  isShareGroupSubmitting = nextValue;
  shareGroupName.disabled = nextValue;
  shareGroupCreate.disabled = nextValue;
  renderShareGroups();
}

function setSubmitting(nextValue) {
  isSubmitting = nextValue;
  btnSave.disabled = nextValue;
  btnCancel.disabled = nextValue;
  btnAdd.disabled = nextValue;
  btnSave.textContent = nextValue ? 'Saving...' : 'Save';
}

function showStatus(message, options = {}) {
  if (statusTimer) {
    clearTimeout(statusTimer);
    statusTimer = null;
  }
  statusMessage.textContent = message;
  appStatus.classList.remove('hidden', 'error');
  appStatus.setAttribute('role', options.tone === 'error' ? 'alert' : 'status');
  appStatus.setAttribute('aria-live', options.tone === 'error' ? 'assertive' : 'polite');
  if (options.tone === 'error') appStatus.classList.add('error');
  btnRetry.classList.toggle('hidden', !options.retry);
  if (options.autoHide !== false && options.tone !== 'error') {
    statusTimer = window.setTimeout(() => {
      clearStatus();
    }, options.duration || 2500);
  }
}

function clearStatus() {
  if (statusTimer) {
    clearTimeout(statusTimer);
    statusTimer = null;
  }
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
  authEmail.focus();
}

async function confirmDelete(iv) {
  if (pendingDeleteIds.has(iv.id)) return;
  if (!confirm(`Delete "${iv.name}"?`)) return;

  pendingDeleteIds.add(iv.id);
  await loadIntervals({ preserveStatus: true });
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

async function moveIntervalInList(iv, direction) {
  if (!currentUser) return;
  const currentIndex = currentIntervals.findIndex(interval => interval.id === iv.id);
  if (currentIndex === -1) return;

  const swapIndex = direction === 'up' ? currentIndex - 1 : currentIndex + 1;
  if (swapIndex < 0 || swapIndex >= currentIntervals.length) return;

  const reordered = [...currentIntervals];
  [reordered[currentIndex], reordered[swapIndex]] = [reordered[swapIndex], reordered[currentIndex]];
  currentIntervals = reordered;
  renderCurrentIntervals();
  showStatus(`Moved "${iv.name}" ${direction}.`, { tone: 'status' });

  try {
    await api.move(iv.id, direction);
    await loadIntervals({ preserveStatus: true });
  } catch (err) {
    if (err.status === 401) {
      handleUnauthorized('Your session has ended. Log in again.');
      return;
    }
    await loadIntervals({ preserveStatus: true });
    showStatus(err.message, { retry: true, tone: 'error' });
  }
}

async function saveShareGroup(id, name) {
  if (isShareGroupSubmitting) return;
  if (!name) return showShareGroupError('Group name is required.');
  clearShareGroupError();
  try {
    setShareGroupSubmitting(true);
    await api.updateGroup(id, { name });
    await loadShareGroups();
    await loadIntervals({ preserveStatus: true });
    showStatus('Share group updated.', { tone: 'status' });
  } catch (err) {
    if (err.status === 401) {
      handleUnauthorized('Your session has ended. Log in again.');
      return;
    }
    showShareGroupError(err.message);
  } finally {
    setShareGroupSubmitting(false);
  }
}

async function rotateShareGroup(group) {
  if (isShareGroupSubmitting) return;
  if (!confirm(`Rotate the public link for "${group.name}"? The current group link will stop working immediately.`)) return;
  clearShareGroupError();
  try {
    setShareGroupSubmitting(true);
    await api.rotateGroup(group.id);
    await loadShareGroups();
    showStatus('Share-group link rotated.', { tone: 'status' });
  } catch (err) {
    if (err.status === 401) {
      handleUnauthorized('Your session has ended. Log in again.');
      return;
    }
    showShareGroupError(err.message);
  } finally {
    setShareGroupSubmitting(false);
  }
}

async function removeShareGroup(group) {
  if (isShareGroupSubmitting) return;
  if (!confirm(`Delete the share group "${group.name}"? Intervals in it will become private.`)) return;
  clearShareGroupError();
  try {
    setShareGroupSubmitting(true);
    await api.deleteGroup(group.id);
    await loadShareGroups();
    await loadIntervals({ preserveStatus: true });
    showStatus('Share group deleted. Its intervals are private again.', { tone: 'status' });
  } catch (err) {
    if (err.status === 401) {
      handleUnauthorized('Your session has ended. Log in again.');
      return;
    }
    showShareGroupError(err.message);
  } finally {
    setShareGroupSubmitting(false);
  }
}

btnMenu.addEventListener('click', event => {
  event.stopPropagation();
  const isOpen = !mobileMenu.classList.contains('hidden');
  mobileMenu.classList.toggle('hidden', isOpen);
  btnMenu.setAttribute('aria-expanded', isOpen ? 'false' : 'true');
});
menuUserBadge.addEventListener('click', () => {
  closeMobileMenu();
  userBadge.click();
});
menuGroupsBadge.addEventListener('click', () => {
  closeMobileMenu();
  groupsBadge.click();
});
menuLogout.addEventListener('click', () => {
  closeMobileMenu();
  btnLogout.click();
});
btnAdd.addEventListener('click', openAdd);
btnCancel.addEventListener('click', closeModal);
list.addEventListener('click', event => {
  const button = event.target.closest('button');
  if (!button || button.disabled) return;

  const card = button.closest('.card');
  if (!card) return;

  if (button.classList.contains('btn-card-menu')) {
    event.preventDefault();
    event.stopPropagation();
    document.querySelectorAll('.card-menu:not(.hidden)').forEach(m => {
      if (m.closest('.card') !== card) {
        m.classList.add('hidden');
        const mb = m.closest('.card').querySelector('.btn-card-menu');
        if (mb) mb.setAttribute('aria-expanded', 'false');
      }
    });
    const menu = card.querySelector('.card-menu');
    if (menu) {
      const isOpen = !menu.classList.contains('hidden');
      menu.classList.toggle('hidden', isOpen);
      button.setAttribute('aria-expanded', isOpen ? 'false' : 'true');
    }
    return;
  }

  const openMenu = card.querySelector('.card-menu:not(.hidden)');
  if (openMenu) {
    openMenu.classList.add('hidden');
    const mb = card.querySelector('.btn-card-menu');
    if (mb) mb.setAttribute('aria-expanded', 'false');
  }

  const id = Number(card.dataset.id);
  const interval = currentIntervals.find(item => item.id === id);
  if (!interval) return;

  if (button.classList.contains('btn-move-up')) {
    event.preventDefault();
    moveIntervalInList(interval, 'up');
    return;
  }
  if (button.classList.contains('btn-move-down')) {
    event.preventDefault();
    moveIntervalInList(interval, 'down');
    return;
  }
  if (button.classList.contains('btn-edit')) {
    event.preventDefault();
    openEdit(interval);
    return;
  }
  if (button.classList.contains('btn-delete')) {
    event.preventDefault();
    confirmDelete(interval);
  }
});
userBadge.addEventListener('click', () => {
  if (!currentUser || isPublicView) return;
  const nextHidden = !profilePanel.classList.contains('hidden');
  profilePanel.classList.toggle('hidden', nextHidden);
  profilePanel.open = !nextHidden;
  userBadge.setAttribute('aria-expanded', profilePanel.open ? 'true' : 'false');
  if (profilePanel.open) {
    profilePanel.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }
});
groupsBadge.addEventListener('click', () => {
  if (!currentUser || isPublicView) return;
  const nextHidden = !shareGroupsPanel.classList.contains('hidden');
  shareGroupsPanel.classList.toggle('hidden', nextHidden);
  shareGroupsPanel.open = !nextHidden;
  groupsBadge.setAttribute('aria-expanded', shareGroupsPanel.open ? 'true' : 'false');
  if (shareGroupsPanel.open) {
    shareGroupsPanel.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }
});
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
    loadPublicGroup();
  } else {
    loadShareGroups()
      .then(() => loadIntervals())
      .catch(err => showStatus(err.message, { retry: true, tone: 'error' }));
  }
});
authSwitch.addEventListener('click', () => {
  setAuthMode(!isRegisterMode);
  if (isRegisterMode) {
    authUsername.focus();
  } else {
    authEmail.focus();
  }
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
  if (!mobileMenu.classList.contains('hidden')) {
    if (!mobileMenu.contains(event.target) && event.target !== btnMenu) {
      closeMobileMenu();
    }
  }
  document.querySelectorAll('.card-menu:not(.hidden)').forEach(menu => {
    const menuBtn = menu.closest('.card').querySelector('.btn-card-menu');
    if (!menu.contains(event.target) && event.target !== menuBtn) {
      menu.classList.add('hidden');
      if (menuBtn) menuBtn.setAttribute('aria-expanded', 'false');
    }
  });
  if (datePicker.classList.contains('hidden')) return;
  const clickedInsidePicker = datePicker.contains(event.target);
  const clickedTrigger = [fieldStart, fieldEnd, btnPickStart, btnPickEnd].includes(event.target);
  if (!clickedInsidePicker && !clickedTrigger) {
    hideDatePicker();
  }
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape') {
    closeModal();
    closeMobileMenu();
    document.querySelectorAll('.card-menu:not(.hidden)').forEach(menu => {
      menu.classList.add('hidden');
      const mb = menu.closest('.card').querySelector('.btn-card-menu');
      if (mb) mb.setAttribute('aria-expanded', 'false');
    });
  }
  if (event.key === 'Tab' && !overlay.classList.contains('hidden')) trapModalFocus(event);
});
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) {
    refreshForDateChange();
  }
});
window.addEventListener('focus', refreshForDateChange);
profilePanel.addEventListener('toggle', () => {
  userBadge.setAttribute('aria-expanded', profilePanel.open ? 'true' : 'false');
  if (!profilePanel.open) {
    profilePanel.classList.add('hidden');
  }
});
shareGroupsPanel.addEventListener('toggle', () => {
  groupsBadge.setAttribute('aria-expanded', shareGroupsPanel.open ? 'true' : 'false');
  if (!shareGroupsPanel.open) {
    shareGroupsPanel.classList.add('hidden');
  }
});

authForm.addEventListener('submit', async event => {
  event.preventDefault();
  if (isAuthSubmitting) return;

  clearAuthError();
  const email = authEmail.value.trim();
  const username = authUsername.value.trim();
  const password = authPassword.value;
  if (!email) return showAuthError('Email is required.');
  if (isRegisterMode && !username) return showAuthError('Username is required.');
  if (!password) return showAuthError('Password is required.');

  try {
    setAuthSubmitting(true);
    const user = isRegisterMode
      ? await api.register({ email, username, password })
      : await api.login({ email, password });
    setCurrentUser(user);
    clearStatus();
    list.innerHTML = '<p id="loading-msg" class="empty-msg">Loading intervals...</p>';
    await loadShareGroups();
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

shareGroupForm.addEventListener('submit', async event => {
  event.preventDefault();
  if (isShareGroupSubmitting || !currentUser) return;

  clearShareGroupError();
  const name = shareGroupName.value.trim();
  if (!name) return showShareGroupError('Group name is required.');

  try {
    setShareGroupSubmitting(true);
    await api.createGroup({ name });
    shareGroupName.value = '';
    await loadShareGroups();
    showStatus('Share group created.', { tone: 'status' });
  } catch (err) {
    if (err.status === 401) {
      handleUnauthorized('Your session has ended. Log in again.');
      return;
    }
    showShareGroupError(err.message);
  } finally {
    setShareGroupSubmitting(false);
  }
});

profileDeleteAccount.addEventListener('click', async () => {
  if (isProfileSubmitting || !currentUser) return;
  if (!confirm(`Delete account "${currentUser.username}"? This will permanently remove your intervals, groups, and session.`)) return;

  clearProfileError();
  try {
    setProfileSubmitting(true);
    await api.deleteAccount();
    handleUnauthorized('Account deleted.');
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
  const shareGroupID = fieldShareGroup.value ? Number(fieldShareGroup.value) : null;

  if (!name) return showError('Name is required.');
  if (!start) return showError('Start date is required.');
  if (!end) return showError('End date is required.');
  if (!isValidISODate(start)) return showError('Start date must be in YYYY-MM-DD format.');
  if (!isValidISODate(end)) return showError('End date must be in YYYY-MM-DD format.');
  if (start > end) return showError('End date must be on or after start date.');

  const data = { name, start_date: start, end_date: end, color: fieldColor.value, share_group_id: shareGroupID };

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
    const build = await api.version();
    setVersionLabel(build.version);
  } catch {
    setVersionLabel('dev');
  }

  try {
    const providers = await api.providers();
    authOAuth.classList.toggle('hidden', !providers.github_enabled);
  } catch {
    authOAuth.classList.add('hidden');
  }

  try {
    const user = await api.me();
    setCurrentUser(user);
    await loadShareGroups();
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
    authEmail.focus();
  }
}

function initPublicView() {
  authView.classList.add('hidden');
  btnAdd.classList.add('hidden');
  btnLogout.classList.add('hidden');
  btnMenu.classList.add('hidden');
  userBadge.classList.add('hidden');
  profilePanel.classList.add('hidden');
  shareGroupsPanel.classList.add('hidden');
  publicGroupHeader.classList.remove('hidden');
  appView.classList.remove('hidden');
  api.version()
    .then(build => setVersionLabel(build.version))
    .catch(() => setVersionLabel('dev'));
  loadPublicGroup();
}

if (isPublicView) {
  scheduleMidnightRefresh();
  initPublicView();
} else {
  scheduleMidnightRefresh();
  initPrivateApp();
}
