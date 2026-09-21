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

		<table class="token-table">
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
						<td class="muted">{p.image}</td>
						<td class="actions">
							{#if canOperate}
								<button class="btn subtle" disabled={pending.has(p.namespace + '/' + p.name)} onclick={() => restart(p)}>Restart</button>
							{/if}
							<button class="btn subtle" onclick={() => (logsFor = p)}>Logs</button>
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
		font-size: 18px;
	}
	.muted {
		color: var(--muted-strong);
		font-size: 12px;
	}
	.error {
		color: var(--danger);
		font-size: 13px;
	}
	.context-picker {
		display: block;
		font-size: 12px;
		color: var(--muted);
		margin-bottom: 12px;
	}
	.context-picker select {
		margin-left: 6px;
		padding: 4px 8px;
		font-size: 12px;
	}
	.actions {
		display: flex;
		gap: 6px;
	}
</style>
