<script>
	import { api } from '../api.js';
	import DockerView from './DockerView.svelte';
	import KubernetesView from '../KubernetesView.svelte';
	import Icon from '../Icon.svelte';

	let { containers = [], canOperate = false, isAdmin = false, host = null, onaction } = $props();

	const storageKey = 'serverdash.containers.view';
	let mode = $state(localStorage.getItem(storageKey) || 'auto');
	let k8sAvailable = $state(false);

	$effect(() => {
		if (mode !== 'auto') localStorage.setItem(storageKey, mode);
	});

	$effect(() => {
		api.k8sStatus()
			.then((s) => (k8sAvailable = s.available))
			.catch(() => (k8sAvailable = false));
	});

	let activeMode = $derived(
		mode === 'auto' ? (host?.containersRuntime !== false ? 'docker' : k8sAvailable ? 'kubernetes' : 'docker') : mode,
	);
</script>

<div class="switcher" role="tablist" aria-label="Container platform">
	<button
		class:active={activeMode === 'docker'}
		role="tab"
		aria-selected={activeMode === 'docker'}
		onclick={() => (mode = 'docker')}
	>
		<Icon name="Container" size={14} /> Docker / Podman
	</button>
	<button
		class:active={activeMode === 'kubernetes'}
		role="tab"
		aria-selected={activeMode === 'kubernetes'}
		onclick={() => (mode = 'kubernetes')}
	>
		<Icon name="Organization" size={14} /> Kubernetes
	</button>
</div>

{#if activeMode === 'docker'}
	<DockerView {containers} {canOperate} {isAdmin} {host} {onaction} />
{:else}
	<KubernetesView {canOperate} />
{/if}

<style>
	.switcher {
		display: inline-flex;
		gap: 4px;
		padding: 4px;
		margin-bottom: 18px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--surface);
	}
	.switcher button {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 8px 16px;
		border: none;
		border-radius: 8px;
		background: none;
		color: var(--muted);
		font-family: var(--font-sans);
		font-size: 12px;
	}
	.switcher button.active {
		background: var(--accent-soft);
		color: var(--accent);
		font-weight: 600;
		box-shadow: inset 0 0 0 1px var(--border-strong);
	}
</style>
