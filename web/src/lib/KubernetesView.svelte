<script>
	import { onDestroy, onMount } from 'svelte';
	import LogViewer from './LogViewer.svelte';
	import NicknameLabel from './NicknameLabel.svelte';
	import { api } from './api.js';

	let { canOperate } = $props();

	let status = $state(null);
	let selectedContext = $state('');
	let pods = $state([]);
	let error = $state(null);
	let logsFor = $state(null);
	let pending = $state(new Set());

	async function loadStatus() {
		status = await api.k8sStatus();
		if (!selectedContext) selectedContext = status.currentContext;
	}

	async function loadPods() {
		try {
			pods = await api.k8sPods({ context: selectedContext });
			error = null;
		} catch (e) {
			error = e.message;
		}
	}

	async function restart(pod) {
		const key = pod.namespace + '/' + pod.name;
		pending = new Set([...pending, key]);
		try {
			await api.restartPod(pod.namespace, pod.name, pod.context || selectedContext);
			await loadPods();
		} finally {
			pending = new Set([...pending].filter((p) => p !== key));
		}
	}

	let timer;
	onMount(async () => {
		await loadStatus();
		if (status?.available) {
			await loadPods();
			timer = setInterval(loadPods, 8000);
		}
	});
	onDestroy(() => clearInterval(timer));

	$effect(() => {
		if (selectedContext) loadPods();
	});
</script>

<section>
	<h2>Kubernetes</h2>

	{#if !status}
		<p class="muted">Loading…</p>
	{:else if !status.available}
		<p class="muted">
			Kubernetes isn't configured on this host{status.error ? ` (${status.error})` : ''}. Mount a kubeconfig and the
			<code>kubectl</code> binary to enable this.
		</p>
	{:else}
		{#if status.contexts.length > 1}
			<label class="context-picker">
				Context
				<select bind:value={selectedContext}>
					{#each status.contexts as ctx (ctx)}
						<option value={ctx}>{ctx}</option>
					{/each}
				</select>
			</label>
		{/if}

		{#if error}
			<p class="error">{error}</p>
		{/if}

		<table>
			<thead>
				<tr>
					<th>Pod</th>
					<th>Namespace</th>
					<th>Phase</th>
					<th>Ready</th>
					<th>Restarts</th>
					<th>Image</th>
					<th></th>
				</tr>
			</thead>
			<tbody>
				{#each pods as p (p.namespace + '/' + p.name)}
					<tr>
						<td>
							<NicknameLabel
								name={p.name}
								nickname={p.nickname}
								editable={canOperate}
								onsave={async (nick) => {
									await api.setPodNickname(p.namespace, p.name, nick, selectedContext);
									await loadPods();
								}}
								ondelete={async () => {
									await api.deletePodNickname(p.namespace, p.name, selectedContext);
									await loadPods();
								}}
							/>
						</td>
						<td class="muted">{p.namespace}</td>
						<td class="muted">{p.phase}</td>
						<td class="muted">{p.ready}</td>
						<td class="muted">{p.restartCount}</td>
						<td class="muted mono">{p.image}</td>
						<td class="actions">
							{#if canOperate}
								<button disabled={pending.has(p.namespace + '/' + p.name)} onclick={() => restart(p)}>Restart</button>
							{/if}
							<button onclick={() => (logsFor = p)}>Logs</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</section>

{#if logsFor}
	<LogViewer
		title={logsFor.name}
		socketUrl={api.podLogsSocketUrl(logsFor.namespace, logsFor.name, selectedContext)}
		onclose={() => (logsFor = null)}
	/>
{/if}

<style>
	h2 {
		margin: 0 0 12px;
		font-size: 20px;
	}
	.muted {
		color: var(--text-secondary);
		font-size: 13px;
	}
	.error {
		color: var(--status-critical);
		font-size: 13px;
	}
	.context-picker {
		display: block;
		font-size: 12px;
		color: var(--text-muted);
		margin-bottom: 12px;
	}
	.context-picker select {
		margin-left: 6px;
		font: inherit;
		background: var(--surface-1);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 4px 8px;
		color: var(--text-primary);
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
	.mono {
		font-family: ui-monospace, monospace;
		font-size: 11px;
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
