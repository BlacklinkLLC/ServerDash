<script>
	import StatusBadge from './StatusBadge.svelte';
	import LogViewer from './LogViewer.svelte';
	import { api } from './api.js';

	let { containers, onaction } = $props();

	let logsFor = $state(null);
	let pending = $state(new Set());

	async function run(action, id) {
		pending = new Set([...pending, id]);
		try {
			if (action === 'start') await api.startContainer(id);
			if (action === 'stop') await api.stopContainer(id);
			if (action === 'restart') await api.restartContainer(id);
			onaction?.();
		} finally {
			pending = new Set([...pending].filter((p) => p !== id));
		}
	}
</script>

<section>
	<h2>Services</h2>
	<table>
		<thead>
			<tr>
				<th>Name</th>
				<th>Status</th>
				<th>Image</th>
				<th>Ports</th>
				<th></th>
			</tr>
		</thead>
		<tbody>
			{#each containers as c (c.id)}
				<tr>
					<td>{c.name}</td>
					<td><StatusBadge state={c.state} /></td>
					<td class="muted">{c.image}</td>
					<td class="muted">{c.ports.map((p) => `${p.publicPort || ''}${p.publicPort ? ':' : ''}${p.privatePort}`).join(', ') || '—'}</td>
					<td class="actions">
						<button disabled={pending.has(c.id)} onclick={() => run(c.state === 'running' ? 'stop' : 'start', c.id)}>
							{c.state === 'running' ? 'Stop' : 'Start'}
						</button>
						<button disabled={pending.has(c.id)} onclick={() => run('restart', c.id)}>Restart</button>
						<button onclick={() => (logsFor = c)}>Logs</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</section>

{#if logsFor}
	<LogViewer container={logsFor} onclose={() => (logsFor = null)} />
{/if}

<style>
	h2 {
		margin: 0 0 12px;
		font-size: 20px;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		background: var(--surface-1);
		border: 1px solid var(--border);
		border-radius: 10px;
		overflow: hidden;
	}
	th,
	td {
		text-align: left;
		padding: 10px 14px;
		font-size: 13px;
		border-bottom: 1px solid var(--gridline);
	}
	th {
		color: var(--text-muted);
		font-weight: 600;
		text-transform: uppercase;
		font-size: 11px;
		letter-spacing: 0.02em;
	}
	tr:last-child td {
		border-bottom: none;
	}
	.muted {
		color: var(--text-secondary);
	}
	.actions {
		display: flex;
		gap: 6px;
	}
	.actions button {
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 5px 10px;
		font-size: 12px;
		color: var(--text-primary);
	}
	.actions button:disabled {
		opacity: 0.5;
		cursor: default;
	}
</style>
