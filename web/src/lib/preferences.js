import { applyAccentColor } from './color.js';

export const preferenceKeys = [
	'theme',
	'density',
	'fontFamily',
	'borderRadius',
	'backgroundPattern',
	'refreshSeconds',
	'warnThreshold',
	'critThreshold',
	'showSystem',
	'showContainers',
	'widgetLayout',
];

const defaultWidths = {
	health: 'full',
	gauges: 'full',
	cpuChart: 'half',
	memChart: 'half',
	netChart: 'full',
	cores: 'half',
	disks: 'half',
	containers: 'half',
	processes: 'half',
};

export function mergeSettings(global, preferences = {}) {
	const merged = { ...global };
	for (const key of preferenceKeys) {
		if (preferences[key] !== undefined && preferences[key] !== null) merged[key] = preferences[key];
	}
	return merged;
}

export function resolveWidgetLayout(settings, pluginWidgets = []) {
	const catalog = settings?.widgetCatalog ?? [];
	const builtins = ['health', 'gauges', 'cpuChart', 'memChart', 'netChart', 'cores', 'disks', 'containers', 'processes'];
	const known = new Set([...builtins, ...catalog.map((item) => item.id)]);
	if (Array.isArray(settings?.widgetLayout) && settings.widgetLayout.length) {
		return settings.widgetLayout.filter((item) => known.has(item.id));
	}
	const items = builtins.map((id) => ({
		id,
		enabled: true,
		width: catalog.find((item) => item.id === id)?.width || defaultWidths[id] || 'half',
	}));
	for (const item of pluginWidgets) {
		items.push({ id: item.id, enabled: true, width: item.widget?.wide ? 'full' : 'half' });
	}
	return items;
}

export function applyAppearance(settings) {
	document.title = settings?.appName || 'ServerDash';
	applyAccentColor(settings?.accentColor || '#ffb020');
	const root = document.documentElement;
	root.dataset.theme = settings?.theme || 'dark';
	root.dataset.density = settings?.density || 'comfortable';
	root.dataset.font = settings?.fontFamily || 'modern';
	root.dataset.radius = settings?.borderRadius || 'default';
	root.dataset.pattern = settings?.backgroundPattern || 'none';
	updateFavicon(settings?.logoUrl, settings?.faviconFromLogo !== false);
}

function updateFavicon(logoUrl, enabled) {
	let link = document.querySelector('link[data-serverdash-favicon]');
	if (enabled && logoUrl) {
		if (!link) {
			link = document.createElement('link');
			link.rel = 'icon';
			link.dataset.serverdashFavicon = 'true';
			document.head.appendChild(link);
		}
		link.href = logoUrl;
		return;
	}
	link?.remove();
}
