<script>
	import { api } from '../api.js';
	import RunLog from './RunLog.svelte';

	let runs = $state([]);
	let error = $state(null);
	let only = $state('all');

	async function load() {
		try {
			runs = await api.recentWorkflowRuns(200);
			error = null;
		} catch (e) {
			error = e.message;
		}
	}
	load();
	$effect(() => {
		const t = setInterval(load, 15000);
		return () => clearInterval(t);
	});

	let shown = $derived(runs.filter((r) => only === 'all' || (only === 'failed' ? r.success === false : r.success === true)));
	let failed = $derived(runs.filter((r) => r.success === false).length);
</script>

<div class="toolbar">
	<div class="seg">
		{#each [['all', 'All'], ['failed', `Failed (${failed})`], ['ok', 'Succeeded']] as [k, label] (k)}
			<button class:active={only === k} onclick={() => (only = k)}>{label}</button>
		{/each}
	</div>
	<button class="btn subtle" onclick={load}>Refresh</button>
</div>
{#if error}<p class="error">{error}</p>{/if}
<div class="list">
	{#each shown as run (run.id)}
		<RunLog {run} showName />
	{:else}
		<p class="muted">No workflow runs yet.</p>
	{/each}
</div>

<style>
	.toolbar {
		display: flex;
		justify-content: space-between;
		margin-bottom: 12px;
	}
	.seg {
		display: flex;
		gap: 4px;
	}
	.seg button {
		padding: 6px 12px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: transparent;
		color: var(--muted);
		font-size: 12px;
	}
	.seg button.active {
		color: var(--text);
		border-color: var(--accent);
	}
	.list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.muted {
		color: var(--muted);
	}
	.error {
		color: var(--danger);
	}
</style>
