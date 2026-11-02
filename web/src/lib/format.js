export function formatBytes(bytes) {
	if (!bytes) return '0 B';
	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
	let i = 0;
	let n = bytes;
	while (n >= 1024 && i < units.length - 1) {
		n /= 1024;
		i++;
	}
	return `${n.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

export function formatUptime(seconds) {
	if (!seconds) return '—';
	const days = Math.floor(seconds / 86400);
	const hours = Math.floor((seconds % 86400) / 3600);
	const minutes = Math.floor((seconds % 3600) / 60);
	if (days > 0) return `${days}d ${hours}h`;
	if (hours > 0) return `${hours}h ${minutes}m`;
	return `${minutes}m`;
}

export function formatPercent(value) {
	if (value === null || value === undefined) return '—';
	return `${value.toFixed(1)}%`;
}

export function formatRate(bytesPerSec) {
	return `${formatBytes(bytesPerSec)}/s`;
}

/** Maps a percent to a status level using the admin-configured thresholds. */
export function statusFor(percent, warn = 75, crit = 90) {
	if (percent === null || percent === undefined) return 'unknown';
	if (percent >= crit) return 'critical';
	if (percent >= warn) return 'warning';
	return 'good';
}

export const statusLabel = { good: 'Healthy', warning: 'Elevated', critical: 'Critical', unknown: 'Unknown' };

export function formatClock(date) {
	return new Date(date).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}
