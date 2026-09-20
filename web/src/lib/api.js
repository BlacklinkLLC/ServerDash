// Thin client for ServerDash's Go API. Requests go through the same origin
// the dashboard is served from; the Node server (dev or prod) proxies /api
// through to the Go backend, so no base URL configuration is needed here.

export class ApiError extends Error {
	constructor(status, message) {
		super(message);
		this.status = status;
	}
}

async function request(path, options = {}) {
	const res = await fetch(path, {
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		...options,
	});
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new ApiError(res.status, body.error || `${res.status} ${res.statusText}`);
	}
	if (res.status === 204) return null;
	return res.json();
}

const del = (path) => request(path, { method: 'DELETE' });
const post = (path, body) => request(path, { method: 'POST', body: body !== undefined ? JSON.stringify(body) : undefined });
const put = (path, body) => request(path, { method: 'PUT', body: JSON.stringify(body) });

function wsUrl(path) {
	const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
	return `${proto}//${window.location.host}${path}`;
}

export const api = {
	// --- Auth / setup ---
	setupStatus: () => request('/api/setup/status'),
	setup: (username, password) => post('/api/setup', { username, password }),
	login: (username, password) => post('/api/auth/login', { username, password }),
	logout: () => post('/api/auth/logout'),
	me: () => request('/api/auth/me'),

	// --- Users (admin) ---
	users: () => request('/api/users'),
	createUser: (username, password, role) => post('/api/users', { username, password, role }),
	setUserRole: (id, role) => put(`/api/users/${id}/role`, { role }),
	setUserPassword: (id, password) => put(`/api/users/${id}/password`, { password }),
	deleteUser: (id) => del(`/api/users/${id}`),

	// --- System / runtime ---
	system: () => request('/api/system'),
	runtime: () => request('/api/runtime'),

	// --- Containers (Docker/Podman) ---
	containers: () => request('/api/containers'),
	startContainer: (id) => post(`/api/containers/${id}/start`),
	stopContainer: (id) => post(`/api/containers/${id}/stop`),
	restartContainer: (id) => post(`/api/containers/${id}/restart`),
	setContainerNickname: (id, nickname) => put(`/api/containers/${id}/nickname`, { nickname }),
	deleteContainerNickname: (id) => del(`/api/containers/${id}/nickname`),
	containerLogsSocketUrl: (id) => wsUrl(`/api/containers/${id}/logs`),

	// --- Kubernetes ---
	k8sStatus: () => request('/api/k8s/status'),
	k8sPods: (opts = {}) => {
		const params = new URLSearchParams();
		if (opts.context) params.set('context', opts.context);
		if (opts.namespace) params.set('namespace', opts.namespace);
		const qs = params.toString();
		return request(`/api/k8s/pods${qs ? `?${qs}` : ''}`);
	},
	restartPod: (namespace, name, context) =>
		post(`/api/k8s/pods/${namespace}/${name}/restart${context ? `?context=${context}` : ''}`),
	setPodNickname: (namespace, name, nickname, context) =>
		put(`/api/k8s/pods/${namespace}/${name}/nickname${context ? `?context=${context}` : ''}`, { nickname }),
	deletePodNickname: (namespace, name, context) =>
		del(`/api/k8s/pods/${namespace}/${name}/nickname${context ? `?context=${context}` : ''}`),
	podLogsSocketUrl: (namespace, name, context) =>
		wsUrl(`/api/k8s/pods/${namespace}/${name}/logs${context ? `?context=${context}` : ''}`),

	// --- Automation (admin) ---
	automationRules: () => request('/api/automation/rules'),
	updateAutomationRule: (id, rule) => put(`/api/automation/rules/${id}`, rule),
	pruneNow: () => post('/api/automation/prune'),

	// --- Scripts (admin) ---
	scripts: () => request('/api/scripts'),
	createScript: (script) => post('/api/scripts', script),
	updateScript: (id, script) => put(`/api/scripts/${id}`, script),
	deleteScript: (id) => del(`/api/scripts/${id}`),
	runScript: (id) => post(`/api/scripts/${id}/run`),
	scriptRuns: (id) => request(`/api/scripts/${id}/runs`),
};
