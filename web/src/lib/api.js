// Thin client for ServerDash's Go API. Requests go through the same origin
// the dashboard is served from; the Node server (dev or prod) proxies /api
// through to the Go backend, so no base URL configuration is needed here.

async function request(path, options = {}) {
	const res = await fetch(path, {
		headers: { 'Content-Type': 'application/json' },
		...options,
	});
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new Error(body.error || `${res.status} ${res.statusText}`);
	}
	return res.json();
}

export const api = {
	system: () => request('/api/system'),
	containers: () => request('/api/containers'),
	startContainer: (id) => request(`/api/containers/${id}/start`, { method: 'POST' }),
	stopContainer: (id) => request(`/api/containers/${id}/stop`, { method: 'POST' }),
	restartContainer: (id) => request(`/api/containers/${id}/restart`, { method: 'POST' }),
	logsSocketUrl: (id) => {
		const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		return `${proto}//${window.location.host}/api/containers/${id}/logs`;
	},
};
