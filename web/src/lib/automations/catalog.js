// What the workflow editor offers: trigger types, block types (with their
// fields), and starter templates. Mirrors server/internal/workflow.

export const TRIGGERS = [
	{ type: 'daily', label: 'Every day', icon: 'Calendar', group: 'Schedule', hint: 'At a set time' },
	{ type: 'weekly', label: 'Weekly', icon: 'Calendar', group: 'Schedule', hint: 'On chosen days' },
	{ type: 'interval', label: 'Every N minutes', icon: 'Refresh', group: 'Schedule', hint: 'Repeating' },
	{ type: 'cron', label: 'Cron', icon: 'Code', group: 'Schedule', hint: 'Custom expression' },
	{ type: 'metric', label: 'Resource alert', icon: 'Speedometer', group: 'Event', hint: 'CPU, memory, disk, load' },
	{ type: 'container', label: 'Container event', icon: 'Blocks', group: 'Event', hint: 'Stops or turns unhealthy' },
	{ type: 'service', label: 'Service failure', icon: 'Warning', group: 'Event', hint: 'systemd unit fails' },
	{ type: 'webhook', label: 'Webhook', icon: 'Connect', group: 'Other', hint: 'From CI, GitHub, anything' },
	{ type: 'manual', label: 'Manual only', icon: 'Play', group: 'Other', hint: 'Run now button' },
];

export function blankTrigger(type) {
	const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
	switch (type) {
		case 'daily':
			return { type, time: '03:00', tz };
		case 'weekly':
			return { type, time: '09:00', tz, days: [1, 2, 3, 4, 5] };
		case 'interval':
			return { type, minutes: 15 };
		case 'cron':
			return { type, cron: '*/30 * * * *' };
		case 'metric':
			return { type, metric: 'cpu', threshold: 90, forMinutes: 5, cooldownMinutes: 30 };
		case 'container':
			return { type, pattern: '*', event: 'either', cooldownMinutes: 10 };
		case 'service':
			return { type, pattern: '*', cooldownMinutes: 10 };
		default:
			return { type };
	}
}

// group: palette section; fields: rendered by the editor in order.
export const BLOCKS = [
	{ type: 'check', label: 'Only continue if…', icon: 'Focus', group: 'Logic' },
	{ type: 'wait', label: 'Wait', icon: 'Clock', group: 'Logic', fields: [{ key: 'seconds', label: 'Seconds', kind: 'number', min: 1, max: 3600 }] },
	{
		type: 'notify',
		label: 'Send notification',
		icon: 'Bell',
		group: 'Notify',
		fields: [
			{ key: 'format', label: 'Service', kind: 'select', options: [['discord', 'Discord'], ['slack', 'Slack'], ['ntfy', 'ntfy'], ['json', 'Generic JSON']] },
			{ key: 'url', label: 'Webhook URL', kind: 'text', placeholder: 'https://discord.com/api/webhooks/…' },
			{ key: 'message', label: 'Message', kind: 'textarea', placeholder: '{{workflow}} on {{host}}: {{event}}' },
		],
	},
	{
		type: 'http_request',
		label: 'HTTP request',
		icon: 'Globe',
		group: 'Notify',
		fields: [
			{ key: 'method', label: 'Method', kind: 'select', options: [['GET', 'GET'], ['POST', 'POST'], ['PUT', 'PUT'], ['PATCH', 'PATCH'], ['DELETE', 'DELETE'], ['HEAD', 'HEAD']] },
			{ key: 'url', label: 'URL', kind: 'text', placeholder: 'https://example.com/health' },
			{ key: 'body', label: 'Body (optional)', kind: 'textarea' },
			{ key: 'expectStatus', label: 'Expect status (0 = any 2xx)', kind: 'number', min: 0, max: 599 },
		],
	},
	{ type: 'restart_container', label: 'Restart container', icon: 'Refresh', group: 'Containers', fields: [{ key: 'container', label: 'Container', kind: 'container' }] },
	{ type: 'start_container', label: 'Start container', icon: 'Play', group: 'Containers', fields: [{ key: 'container', label: 'Container', kind: 'container' }] },
	{ type: 'stop_container', label: 'Stop container', icon: 'Pause', group: 'Containers', fields: [{ key: 'container', label: 'Container', kind: 'container' }] },
	{
		type: 'compose_up',
		label: 'Rebuild & run (compose up)',
		icon: 'Stack',
		group: 'Containers',
		fields: [
			{ key: 'dir', label: 'Project directory', kind: 'text', placeholder: '/srv/app' },
			{ key: 'build', label: 'Rebuild images (--build)', kind: 'checkbox' },
		],
	},
	{ type: 'prune', label: 'Prune unused Docker data', icon: 'Droplet', group: 'Containers', fields: [] },
	{ type: 'git_pull', label: 'Git pull', icon: 'Git', group: 'Deploy', fields: [{ key: 'dir', label: 'Repository directory', kind: 'text', placeholder: '/srv/app' }] },
	{
		type: 'shell',
		label: 'Run command',
		icon: 'Terminal',
		group: 'Server',
		fields: [
			{ key: 'command', label: 'Command', kind: 'code', placeholder: 'df -h / && echo {{event}}' },
			{ key: 'dir', label: 'Working directory (optional)', kind: 'text' },
		],
	},
	{ type: 'run_script', label: 'Run saved script', icon: 'Script', group: 'Server', fields: [{ key: 'scriptId', label: 'Script', kind: 'script' }] },
	{
		type: 'service_action',
		label: 'Control a service',
		icon: 'Settings',
		group: 'Server',
		fields: [
			{ key: 'action', label: 'Action', kind: 'select', options: [['restart', 'Restart'], ['start', 'Start'], ['stop', 'Stop'], ['reload', 'Reload']] },
			{ key: 'service', label: 'Service', kind: 'text', placeholder: 'nginx' },
		],
	},
];

export const CHECKS = [
	['cpu_above', 'CPU is above', 'threshold'],
	['cpu_below', 'CPU is below', 'threshold'],
	['mem_above', 'Memory is above', 'threshold'],
	['disk_above', 'Any disk is above', 'threshold'],
	['container_running', 'Container is running', 'container'],
	['container_stopped', 'Container is stopped', 'container'],
	['http_ok', 'URL responds OK', 'url'],
	['command_ok', 'Command succeeds', 'command'],
	['output_contains', 'Previous output contains', 'text'],
];

export const blockMeta = (type) => BLOCKS.find((b) => b.type === type) ?? { type, label: type, icon: 'Blocks', fields: [] };
export const triggerMeta = (type) => TRIGGERS.find((t) => t.type === type) ?? TRIGGERS[0];

export function blankBlock(type) {
	switch (type) {
		case 'wait':
			return { type, seconds: 30 };
		case 'notify':
			return { type, format: 'discord', url: '', message: '{{workflow}} on {{host}}: {{event}}' };
		case 'http_request':
			return { type, method: 'GET', url: '', expectStatus: 0 };
		case 'compose_up':
			return { type, dir: '', build: true };
		case 'check':
			return { type, check: 'cpu_above', threshold: 80, onFalse: 'stop' };
		case 'service_action':
			return { type, action: 'restart', service: '' };
		default:
			return { type };
	}
}

export const VARIABLES = [
	['{{workflow}}', 'Workflow name'],
	['{{trigger}}', 'schedule, event, webhook, or manual'],
	['{{event}}', 'What fired it, e.g. "CPU at 93%"'],
	['{{payload}}', 'Webhook request body'],
	['{{output}}', "Previous block's output"],
	['{{host}}', 'Server hostname'],
	['{{cpu}} {{mem}} {{disk}} {{load}}', 'Live readings'],
	['{{time}} {{date}}', 'Current time / date'],
];

const discord = 'https://discord.com/api/webhooks/…';

export const TEMPLATES = [
	{
		name: 'Alert when CPU stays high',
		icon: 'Speedometer',
		description: 'Notify Discord when CPU is above 90% for 5 minutes.',
		trigger: { type: 'metric', metric: 'cpu', threshold: 90, forMinutes: 5, cooldownMinutes: 30 },
		blocks: [{ type: 'notify', format: 'discord', url: discord, message: '🔥 {{host}}: {{event}}. Memory {{mem}}, load {{load}}.' }],
	},
	{
		name: 'Auto-heal crashed containers',
		icon: 'Blocks',
		description: 'When a container stops or turns unhealthy, notify, wait, and restart it.',
		trigger: { type: 'container', pattern: '*', event: 'either', cooldownMinutes: 10 },
		blocks: [
			{ type: 'notify', format: 'discord', url: discord, message: '⚠️ {{host}}: {{event}}. Restarting it in 30s.' },
			{ type: 'wait', seconds: 30 },
			{ type: 'restart_container', container: 'my-app', continueOnError: true },
		],
	},
	{
		name: 'Deploy on push (webhook)',
		icon: 'Git',
		description: 'Point a GitHub or CI webhook here to pull and rebuild.',
		trigger: { type: 'webhook' },
		blocks: [
			{ type: 'git_pull', dir: '/srv/app' },
			{ type: 'compose_up', dir: '/srv/app', build: true },
			{ type: 'notify', format: 'discord', url: discord, message: '🚀 Deployed {{workflow}} on {{host}}.' },
		],
	},
	{
		name: 'Uptime check',
		icon: 'Globe',
		description: 'Every 5 minutes, alert if a URL stops responding.',
		trigger: { type: 'interval', minutes: 5 },
		blocks: [
			{ type: 'check', check: 'http_ok', url: 'https://example.com', onFalse: 'fail' },
		],
		onFailure: { url: discord, format: 'discord' },
	},
	{
		name: 'Disk space janitor',
		icon: 'Folder',
		description: 'Nightly: if any disk is above 85%, prune Docker and report.',
		trigger: { type: 'daily', time: '04:00' },
		blocks: [
			{ type: 'check', check: 'disk_above', threshold: 85, onFalse: 'stop' },
			{ type: 'prune' },
			{ type: 'notify', format: 'discord', url: discord, message: '🧹 {{host}}: {{output}}. Disk now {{disk}}.' },
		],
	},
	{
		name: 'Restart failed services',
		icon: 'Warning',
		description: 'When nginx fails, restart it and tell the team.',
		trigger: { type: 'service', pattern: 'nginx', cooldownMinutes: 10 },
		blocks: [
			{ type: 'service_action', action: 'restart', service: 'nginx' },
			{ type: 'notify', format: 'slack', url: 'https://hooks.slack.com/services/…', message: '{{event}}; restarted it on {{host}}.' },
		],
	},
	{
		name: 'Weekly server report',
		icon: 'Analytics',
		description: 'Monday morning summary of uptime and disk usage.',
		trigger: { type: 'weekly', time: '09:00', days: [1] },
		blocks: [
			{ type: 'shell', command: 'uptime -p; df -h --output=target,pcent -x tmpfs -x devtmpfs | tail -n +2' },
			{ type: 'notify', format: 'discord', url: discord, message: '📊 Weekly report for {{host}}\n```\n{{output}}\n```' },
		],
	},
];
