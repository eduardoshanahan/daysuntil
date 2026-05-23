(() => {
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

  const daysUntilApiClient = {
    version: () => apiFetch('/api/version'),
    providers: () => apiFetch('/api/auth/providers'),
    me: () => apiFetch('/api/me'),
    deleteAccount: () => apiFetch('/api/me', { method: 'DELETE' }),
    updateProfile: data => apiFetch('/api/me/profile', { method: 'PUT', body: JSON.stringify(data) }),
    register: data => apiFetch('/api/register', { method: 'POST', body: JSON.stringify(data) }),
    login: data => apiFetch('/api/login', { method: 'POST', body: JSON.stringify(data) }),
    requestLoginLink: data => apiFetch('/api/login/link', { method: 'POST', body: JSON.stringify(data) }),
    consumeLoginLink: data => apiFetch('/api/login/link/consume', { method: 'POST', body: JSON.stringify(data) }),
    logout: () => apiFetch('/api/logout', { method: 'POST' }),
    list: () => apiFetch('/api/intervals'),
    create: data => apiFetch('/api/intervals', { method: 'POST', body: JSON.stringify(data) }),
    move: (id, direction) => apiFetch(`/api/intervals/${id}/move`, { method: 'POST', body: JSON.stringify({ direction }) }),
    update: (id, data) => apiFetch(`/api/intervals/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: id => apiFetch(`/api/intervals/${id}`, { method: 'DELETE' }),
    listGroups: () => apiFetch('/api/share-groups'),
    createGroup: data => apiFetch('/api/share-groups', { method: 'POST', body: JSON.stringify(data) }),
    updateGroup: (id, data) => apiFetch(`/api/share-groups/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    deleteGroup: id => apiFetch(`/api/share-groups/${id}`, { method: 'DELETE' }),
    rotateGroup: id => apiFetch(`/api/share-groups/${id}/rotate`, { method: 'POST' }),
    publicGroup: slug => apiFetch(`/api/public/groups/${encodeURIComponent(slug)}`),
  };

  window.DaysUntilApi = { ApiError, api: daysUntilApiClient };
})();
