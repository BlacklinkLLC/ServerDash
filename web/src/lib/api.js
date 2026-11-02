// Thin client for ServerDash's Go API. Requests go through the same origin
// the dashboard is served from; the Node server (dev or prod) proxies /api
// through to the Go backend, so no base URL configuration is needed here.

export class ApiError extends Error {
	constructor(status, message) {
		super(message);
		this.status = status;
	}
}

// --- Cluster target ---
// When a cluster node is selected in the server switcher, server-scoped
// calls (metrics, containers, pods, tools) are routed through this
// primary's proxy to that node. Everything else (auth, settings, admin)
// always stays on the server you're signed in to. Mirrors the backend's
// proxyablePrefixes.
const proxyable = ['/api/system', '/api/runtime', '/api/host', '/api/containers', '/api/k8s', '/api/services', '/api/journal'];
let targetNode = null;

export function setTargetNode(id) {
	targetNode = id || null;
}

function route(path) {
	if (!targetNode) return path;
	const bare = path.split('?')[0];
	if (!proxyable.some((p) => bare === p || bare.startsWith(p + '/'))) return path;
	return `/api/cluster/nodes/${encodeURIComponent(targetNode)}/proxy${path}`;
}

async function request(path, options = {}) {
	const res = await fetch(route(path), {
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
	return `${proto}//${window.location.host}${route(path)}`;
}

export const api = {
	// --- Auth / setup ---
	setupStatus: () => request('/api/setup/status'),
	setup: (username, password, setupCode) => post('/api/setup', { username, password, setupCode }),
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
	systemHistory: () => request('/api/system/history'),
	processes: (sort = 'cpu', limit = 0) => request(`/api/system/processes?sort=${sort}&limit=${limit}`),
	signalProcess: (pid, signal) => post(`/api/system/processes/${pid}/signal`, { signal }),
	ports: () => request('/api/system/ports'),
	interfaces: () => request('/api/system/interfaces'),
	host: () => request('/api/host'),
	services: () => request('/api/services'),
	serviceAction: (name, action) => post(`/api/services/${encodeURIComponent(name)}/${action}`),
	journal: (opts = {}) => {
		const params = new URLSearchParams();
		for (const [k, v] of Object.entries(opts)) if (v !== undefined && v !== '' && v !== null) params.set(k, v);
		return request(`/api/journal?${params}`);
	},

	// --- Containers (Docker/Podman) ---
	containers: () => request('/api/containers'),
	startContainer: (id) => post(`/api/containers/${id}/start`),
	stopContainer: (id) => post(`/api/containers/${id}/stop`),
	restartContainer: (id) => post(`/api/containers/${id}/restart`),
	setContainerNickname: (id, nickname) => put(`/api/containers/${id}/nickname`, { nickname }),
	deleteContainerNickname: (id) => del(`/api/containers/${id}/nickname`),
	containerLogsSocketUrl: (id) => wsUrl(`/api/containers/${id}/logs`),
	containerTerminalSocketUrl: (id) => wsUrl(`/api/containers/${id}/terminal`),

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

	// --- Workflows: block-based deployment automations (admin) ---
	workflows: () => request('/api/workflows'),
	createWorkflow: (workflow) => post('/api/workflows', workflow),
	updateWorkflow: (id, workflow) => put(`/api/workflows/${id}`, workflow),
	deleteWorkflow: (id) => del(`/api/workflows/${id}`),
	runWorkflow: (id) => post(`/api/workflows/${id}/run`),
	workflowRuns: (id) => request(`/api/workflows/${id}/runs`),

	// --- Cluster ---
	cluster: () => request('/api/cluster'),
	setClusterMode: (mode) => put('/api/cluster/mode', { mode }),
	createJoinToken: (role, primaryUrl, ttlHours) => post('/api/cluster/tokens', { role, primaryUrl, ttlHours }),
	updateClusterNode: (id, node) => put(`/api/cluster/nodes/${id}`, node),
	deleteClusterNode: (id) => del(`/api/cluster/nodes/${id}`),
	joinCluster: (token, name, selfUrl) => post('/api/cluster/join', { token, name, selfUrl }),
	leaveCluster: () => post('/api/cluster/leave'),
	updateClusterSelf: (name, selfUrl) => put('/api/cluster/self', { name, selfUrl }),

	// --- Settings / branding / appearance ---
	settings: () => request('/api/settings'),
	updateSettings: (patch) => put('/api/settings', patch),
	uploadLogo: async (file) => {
		const form = new FormData();
		form.append('logo', file);
		const res = await fetch('/api/settings/logo', { method: 'POST', credentials: 'include', body: form });
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			throw new ApiError(res.status, body.error || `${res.status} ${res.statusText}`);
		}
		return res.json();
	},
	deleteLogo: () => del('/api/settings/logo'),

	// --- Self-update (admin) ---
	updateStatus: () => request('/api/updates/status'),
	applyUpdate: () => post('/api/updates/apply'),
};
