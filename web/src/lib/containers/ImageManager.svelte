<script>
	import { onMount } from 'svelte';
	import { api } from '../api.js';
	import Icon from '../Icon.svelte';

	let { canOperate = false, onchange } = $props();

	let images = $state([]);
	let pullTarget = $state('');
	let busy = $state(false);
	let error = $state(null);

	async function load() {
		try {
			images = await api.images();
			error = null;
		} catch (e) {
			error = e.message;
		}
	}

	onMount(load);

	async function pull(e) {
		e.preventDefault();
		if (!pullTarget.trim()) return;
		busy = true;
		error = null;
		try {
			await api.pullImage(pullTarget.trim());
			pullTarget = '';
			await load();
			onchange?.();
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}

	async function remove(image) {
		busy = true;
		try {
			await api.removeImage(image.id.replace('sha256:', ''));
			await load();
			onchange?.();
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}

	function size(bytes) {
		if (!bytes || bytes < 0) return '—';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let value = bytes;
		let unit = 0;
		while (value >= 1024 && unit < units.length - 1) {
			value /= 1024;
			unit++;
		}
		return `${value.toFixed(unit ? 1 : 0)} ${units[unit]}`;
	}
</script>

<section>
	<div class="section-head">
		<div>
			<h2>Images</h2>
			<p class="muted">Pull, inspect, and remove images cached by Docker or Podman.</p>
		</div>
		<form class="pull" onsubmit={pull}>
			<input bind:value={pullTarget} placeholder="image:tag" aria-label="Image to pull" />
			<button class="btn primary" disabled={busy || !canOperate} type="submit"><Icon name="Download" size={13} /> Pull</button>
		</form>
	</div>
	{#if error}
		<p class="error">{error}</p>
	{/if}
	<table class="token-table">
		<thead>
			<tr>
				<th>Repository</th>
				<th>ID</th>
				<th>Size</th>
				<th>Created</th>
				<th></th>
			</tr>
		</thead>
		<tbody>
			{#each images as image (image.id)}
				<tr>
					<td>{image.tags?.join(', ') || '<none>'}</td>
					<td class="muted">{image.id.replace('sha256:', '').slice(0, 16)}</td>
					<td class="muted">{size(image.size)}</td>
					<td class="muted">{new Date(image.created * 1000).toLocaleDateString()}</td>
					<td>
						<button class="btn subtle" disabled={busy || !canOperate} onclick={() => remove(image)}>Remove</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</section>

<style>
	.section-head {
		display: flex;
		justify-content: space-between;
		align-items: flex-end;
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
	.pull {
		display: flex;
		gap: 8px;
	}
	.pull input {
		min-width: 220px;
		padding: 8px 10px;
		font-size: 12px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--sur2);
		color: var(--text);
	}
	.error {
		color: var(--danger);
		font-size: 12px;
	}
</style>
