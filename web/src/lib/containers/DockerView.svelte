<script>
	import { onMount } from 'svelte';
	import { api } from '../api.js';
	import ContainerList from '../ContainerList.svelte';
	import ContainerForm from './ContainerForm.svelte';
	import ImageManager from './ImageManager.svelte';
	import ManagedContainersView from './ManagedContainersView.svelte';

	let { containers = [], canOperate = false, isAdmin = false, host = null, onaction } = $props();

	let tab = $state('containers');
	let showCreate = $state(false);
	let runtime = $state(null);
	let busy = $state(false);
	let error = $state(null);

	onMount(() => {
		api.runtime()
			.then((r) => (runtime = r))
			.catch(() => {});
	});

	async function create(spec) {
		busy = true;
		error = null;
		try {
			await api.createContainer(spec);
			showCreate = false;
			await onaction?.();
		} catch (e) {
			error = e.message;
			throw e;
		} finally {
			busy = false;
		}
	}

	const tabs = [
		{ id: 'containers', label: 'Containers', icon: 'Container' },
		{ id: 'images', label: 'Images', icon: 'Layers' },
		{ id: 'managed', label: 'Managed', icon: 'Cube' },
	];
</script>

<section>
	<div class="docker-head">
		<div>
			<h2>{runtime?.name ?? 'Docker / Podman'}</h2>
			<p class="muted">{runtime?.version ? `Engine ${runtime.version}` : 'Container runtime'}</p>
		</div>
		{#if canOperate && tab === 'containers'}
			<button class="btn primary" onclick={() => (showCreate = !showCreate)}>
				{showCreate ? 'Close form' : 'New container'}
			</button>
		{/if}
	</div>

	<div class="tabs" role="tablist" aria-label="Docker views">
		{#each tabs as item (item.id)}
			{#if item.id !== 'managed' || host?.managedContainers}
				<button
					class:active={tab === item.id}
					role="tab"
					aria-selected={tab === item.id}
					onclick={() => (tab = item.id)}
				>
					{item.label}
				</button>
			{/if}
		{/each}
	</div>

	{#if error}
		<p class="error">{error}</p>
	{/if}

	{#if tab === 'containers'}
		{#if showCreate && canOperate}
			<div class="editor">
				<ContainerForm submitLabel="Create container" {busy} onsubmit={create} oncancel={() => (showCreate = false)} />
			</div>
		{/if}
		<ContainerList {containers} {canOperate} {isAdmin} onaction={onaction} />
	{:else if tab === 'images'}
		<ImageManager {canOperate} onchange={onaction} />
	{:else if tab === 'managed'}
		<ManagedContainersView {canOperate} onchange={onaction} />
	{/if}
</section>

<style>
	h2 {
		margin: 0 0 4px;
		font-size: 18px;
	}
	.docker-head {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 16px;
		margin-bottom: 14px;
	}
	.muted {
		margin: 0;
		color: var(--muted);
		font-size: 12px;
	}
	.tabs {
		display: inline-flex;
		gap: 4px;
		padding: 3px;
		margin-bottom: 16px;
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
</style>
