<script>
	import { onMount } from 'svelte';
	import StatusBadge from '../StatusBadge.svelte';
	import { api } from '../api.js';
	import ContainerForm from './ContainerForm.svelte';

	let { canOperate = false, onchange } = $props();

	let items = $state([]);
	let showForm = $state(false);
	let editing = $state(null);
	let busy = $state(null);
	let error = $state(null);

	async function load() {
		try {
			items = await api.managedContainers();
			error = null;
		} catch (e) {
			error = e.message;
		}
	}

	onMount(load);

	async function run(id, fn) {
		busy = id;
		error = null;
		try {
			await fn();
			await load();
			onchange?.();
		} catch (e) {
			error = e.message;
		} finally {
			busy = null;
		}
	}

	async function save(spec) {
		if (editing) await api.updateManagedContainer(editing.id, spec);
		else await api.createManagedContainer(spec);
		showForm = false;
		editing = null;
		await load();
		onchange?.();
	}

	function beginCreate() {
		editing = null;
		showForm = true;
	}

	function beginEdit(item) {
		editing = item;
		showForm = true;
	}
</script>

<section>
	<div class="section-head">
		<div>
			<h2>ServerDash Containers</h2>
			<p class="muted">
				Native-only managed containers. ServerDash keeps the definition, image history, and deploy lifecycle, then runs
				the container through Docker or Podman.
			</p>
		</div>
		{#if canOperate}
			<button class="btn primary" onclick={beginCreate}>New managed container</button>
		{/if}
	</div>

	{#if error}
		<p class="error">{error}</p>
	{/if}

	{#if showForm && canOperate}
		{#key editing?.id ?? 'new'}
			<div class="editor">
				<ContainerForm
					initial={editing}
					managed={true}
					submitLabel={editing ? 'Save definition' : 'Create definition'}
					onsubmit={save}
					oncancel={() => (showForm = false)}
				/>
			</div>
		{/key}
	{/if}

	{#if items.length === 0}
		<p class="muted empty">No managed containers yet.</p>
	{:else}
		<table class="token-table">
			<thead>
				<tr>
					<th>Name</th>
					<th>Image</th>
					<th>Status</th>
					<th>Ports</th>
					<th>Deploys</th>
					<th></th>
				</tr>
			</thead>
			<tbody>
				{#each items as item (item.id)}
					<tr>
						<td><b>{item.name}</b><br /><span class="muted">sd-{item.name}</span></td>
						<td class="muted">{item.image}</td>
						<td><StatusBadge state={item.status || 'unknown'} /></td>
						<td class="muted">
							{item.ports?.map((p) => `${p.hostPort}:${p.containerPort}/${p.protocol || 'tcp'}`).join(', ') || '—'}
						</td>
						<td class="muted">{item.history?.length ?? 0}</td>
						<td>
							<div class="actions">
								{#if canOperate}
									<button class="btn subtle" disabled={busy === item.id} onclick={() => run(item.id, () => api.deployManagedContainer(item.id))}>Deploy</button>
									{#if item.status === 'running'}
										<button class="btn subtle" disabled={busy === item.id} onclick={() => run(item.id, () => api.managedContainerAction(item.id, 'stop'))}>Stop</button>
										<button class="btn subtle" disabled={busy === item.id} onclick={() => run(item.id, () => api.managedContainerAction(item.id, 'restart'))}>Restart</button>
									{:else}
										<button class="btn subtle" disabled={busy === item.id} onclick={() => run(item.id, () => api.managedContainerAction(item.id, 'start'))}>Start</button>
									{/if}
									<button class="btn subtle" disabled={busy === item.id} onclick={() => run(item.id, () => api.rollbackManagedContainer(item.id))}>Rollback</button>
									<button class="btn subtle" onclick={() => beginEdit(item)}>Edit</button>
									<button class="btn danger" disabled={busy === item.id} onclick={() => run(item.id, () => api.deleteManagedContainer(item.id))}>Delete</button>
								{/if}
							</div>
						</td>
					</tr>
					{#if item.history?.length}
						<tr class="history">
							<td colspan="6">
								<div class="history-list">
									{#each [...item.history].reverse().slice(0, 5) as entry (entry.id)}
										<span class:failed={entry.status === 'error'}>
											{new Date(entry.at).toLocaleString()} · {entry.action} · {entry.image} · {entry.status}
										</span>
									{/each}
								</div>
							</td>
						</tr>
					{/if}
				{/each}
			</tbody>
		</table>
	{/if}
</section>

<style>
	.section-head {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 16px;
		margin-bottom: 12px;
	}
	h2 {
		margin: 0 0 4px;
		font-size: 18px;
	}
	.muted {
		margin: 0;
		color: var(--muted);
		font-size: 12px;
	}
	.editor {
		padding: 16px;
		margin-bottom: 18px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--sur2);
	}
	.error {
		color: var(--danger);
		font-size: 12px;
	}
	.empty {
		padding: 18px;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.history td {
		background: var(--sur2);
	}
	.history-list {
		display: flex;
		flex-direction: column;
		gap: 3px;
		font-size: 10px;
		color: var(--muted);
	}
	.history-list .failed {
		color: var(--danger);
	}
</style>
