<script>
	import { onDestroy, onMount } from 'svelte';
	import LogViewer from './LogViewer.svelte';
	import NicknameLabel from './NicknameLabel.svelte';
	import { api } from './api.js';

	let { canOperate = false } = $props();

	let status = $state(null);
	let selectedContext = $state('');
	let tab = $state('pods');
	let pods = $state([]);
	let deployments = $state([]);
	let services = $state([]);
	let scaleValues = $state({});
	let error = $state(null);
	let logsFor = $state(null);
	let pending = $state(new Set());
	let timer;

	async function loadPods() {
		return api.k8sPods({ context: selectedContext });
	}

	async function loadAll() {
		try {
			const [p, d, s] = await Promise.all([
				loadPods(),
				api.k8sDeployments({ context: selectedContext }),
				api.k8sServices({ context: selectedContext }),
			]);
			pods = p;
			deployments = d;
			services = s;
			for (const item of d) {
				const key = `${item.namespace}/${item.name}`;
				if (scaleValues[key] === undefined) scaleValues[key] = item.replicas;
			}
			error = null;
		} catch (e) {
			error = e.message;
		}
	}

	async function run(key, fn) {
		pending = new Set([...pending, key]);
		error = null;
		try {
			await fn();
			await loadAll();
		} catch (e) {
			error = e.message;
		} finally {
			pending = new Set([...pending].filter((p) => p !== key));
		}
	}

	onMount(async () => {
		status = await api.k8sStatus();
		if (!status.currentContext) status.currentContext = '';
		selectedContext = status.currentContext;
		if (status.available) {
			await loadAll();
			timer = setInterval(loadAll, 8000);
		}
	});
	onDestroy(() => clearInterval(timer));

	$effect(() => {
		if (selectedContext && status?.available) loadAll();
	});
</script>

<section>
	<div class="section-head">
		<div>
			<h2>Kubernetes</h2>
			<p class="muted">Pods, deployments, and services in the selected context.</p>
		</div>
		{#if status?.contexts?.length > 1}
			<label class="context-picker">
				Context
				<select bind:value={selectedContext}>
					{#each status.contexts as ctx (ctx)}
						<option value={ctx}>{ctx}</option>
					{/each}
				</select>
			</label>
		{/if}
	</div>

	{#if !status}
		<p class="muted">Loading…</p>
	{:else if !status.available}
		<p class="muted">
			Kubernetes isn't configured on this host{status.error ? ` (${status.error})` : ''}. Mount a kubeconfig and the
			<code>kubectl</code> binary to enable this.
		</p>
	{:else}
		<div class="tabs" role="tablist" aria-label="Kubernetes views">
			<button class:active={tab === 'pods'} role="tab" aria-selected={tab === 'pods'} onclick={() => (tab = 'pods')}>Pods</button>
			<button class:active={tab === 'deployments'} role="tab" aria-selected={tab === 'deployments'} onclick={() => (tab = 'deployments')}>Deployments</button>
			<button class:active={tab === 'services'} role="tab" aria-selected={tab === 'services'} onclick={() => (tab = 'services')}>Services</button>
		</div>

		{#if error}
			<p class="error">{error}</p>
		{/if}

		{#if tab === 'pods'}
			<table class="token-table">
				<thead>
					<tr>
						<th>Pod</th><th>Namespace</th><th>Phase</th><th>Ready</th><th>Restarts</th><th>Image</th><th></th>
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
										await loadAll();
									}}
									ondelete={async () => {
										await api.deletePodNickname(p.namespace, p.name, selectedContext);
										await loadAll();
									}}
								/>
							</td>
							<td class="muted">{p.namespace}</td>
							<td class="muted">{p.phase}</td>
							<td class="muted">{p.ready}</td>
							<td class="muted">{p.restartCount}</td>
							<td class="muted">{p.image}</td>
							<td>
								<div class="actions">
									{#if canOperate}
										<button class="btn subtle" disabled={pending.has(p.namespace + '/' + p.name)} onclick={() => run(p.namespace + '/' + p.name, () => api.restartPod(p.namespace, p.name, selectedContext))}>Restart</button>
										<button class="btn subtle" disabled={pending.has(p.namespace + '/' + p.name)} onclick={() => run(p.namespace + '/' + p.name, () => api.deletePod(p.namespace, p.name, selectedContext))}>Delete</button>
									{/if}
									<button class="btn subtle" onclick={() => (logsFor = p)}>Logs</button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{:else if tab === 'deployments'}
			<table class="token-table">
				<thead>
					<tr><th>Deployment</th><th>Namespace</th><th>Replicas</th><th>Image</th><th></th></tr>
				</thead>
				<tbody>
					{#each deployments as d (d.namespace + '/' + d.name)}
						<tr>
							<td><b>{d.name}</b></td>
							<td class="muted">{d.namespace}</td>
							<td class="muted">{d.ready}/{d.replicas} ready</td>
							<td class="muted">{d.image}</td>
							<td>
								<div class="actions">
									{#if canOperate}
										<input class="scale" type="number" min="0" max="1000" bind:value={scaleValues[d.namespace + '/' + d.name]} aria-label="Replica count" />
										<button class="btn subtle" disabled={pending.has(d.namespace + '/' + d.name)} onclick={() => run(d.namespace + '/' + d.name, () => api.scaleDeployment(d.namespace, d.name, Number(scaleValues[d.namespace + '/' + d.name]), selectedContext))}>Scale</button>
										<button class="btn subtle" disabled={pending.has(d.namespace + '/' + d.name)} onclick={() => run(d.namespace + '/' + d.name, () => api.restartDeployment(d.namespace, d.name, selectedContext))}>Restart</button>
									{/if}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{:else}
			<table class="token-table">
				<thead>
					<tr><th>Service</th><th>Namespace</th><th>Type</th><th>Cluster IP</th><th>Ports</th></tr>
				</thead>
				<tbody>
					{#each services as svc (svc.namespace + '/' + svc.name)}
						<tr>
							<td><b>{svc.name}</b></td>
							<td class="muted">{svc.namespace}</td>
							<td class="muted">{svc.type}</td>
							<td class="muted">{svc.clusterIp}</td>
							<td class="muted">{svc.ports?.join(', ') || '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
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
	.section-head {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 16px;
	}
	h2 {
		margin: 0 0 4px;
		font-size: 18px;
	}
	.muted {
		margin: 0;
		color: var(--muted-strong);
		font-size: 12px;
	}
	.error {
		color: var(--danger);
		font-size: 13px;
	}
	.context-picker {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--muted);
	}
	.context-picker select {
		padding: 5px 8px;
		font-size: 12px;
	}
	.tabs {
		display: inline-flex;
		gap: 4px;
		padding: 3px;
		margin: 14px 0 12px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--sur2);
	}
	.tabs button {
		padding: 7px 14px;
		border: none;
		border-radius: 7px;
		background: none;
		color: var(--muted);
		font-family: var(--font-sans);
		font-size: 12px;
	}
	.tabs button.active {
		background: var(--accent-soft);
		color: var(--accent);
		font-weight: 600;
	}
	.actions {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.scale {
		width: 64px;
		padding: 6px 8px;
		font-size: 12px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--sur2);
		color: var(--text);
	}
</style>
