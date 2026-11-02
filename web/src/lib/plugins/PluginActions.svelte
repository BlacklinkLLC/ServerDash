<script>
	import { api } from '../api.js';

	// The action buttons of one plugin, with a confirmation step and the last output.
	let { plugin, ondone } = $props();
	let busy = $state(null);
	let confirming = $state(null);
	let result = $state(null);

	async function run(a) {
		confirming = null;
		busy = a.id;
		result = null;
		const r = await api.runPluginAction(plugin.id, a.id);
		result = { label: a.label, ...r };
		busy = null;
		ondone?.();
	}
	function click(a) {
		if (a.confirm) confirming = a;
		else run(a);
	}
</script>

{#if plugin.actions?.length}
	<div class="actions">
		{#each plugin.actions as a (a.id)}
			<button class="btn {a.danger ? 'danger' : 'subtle'}" disabled={busy !== null} onclick={() => click(a)}>
				{busy === a.id ? 'Running…' : a.label}
			</button>
		{/each}
	</div>
	{#if confirming}
		<div class="confirm">
			<span>{confirming.confirm}</span>
			<button class="btn subtle" onclick={() => (confirming = null)}>Cancel</button>
			<button class="btn {confirming.danger ? 'danger' : 'primary'}" onclick={() => run(confirming)}>{confirming.label}</button>
		</div>
	{/if}
	{#if result}
		<div class="result" class:failed={!result.ok}>
			<b>{result.label}: {result.ok ? 'done' : 'failed'}</b>
			{#if result.error}<span>{result.error}</span>{/if}
			{#if result.output?.trim()}<pre>{result.output}</pre>{/if}
			<button class="close" onclick={() => (result = null)}>×</button>
		</div>
	{/if}
{/if}

<style>
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.actions .btn,
	.confirm .btn {
		padding: 6px 12px;
		font-size: 12px;
	}
	.confirm {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		padding: 10px;
		border-radius: 10px;
		background: var(--warning-soft);
		font-size: 12px;
	}
	.confirm span {
		flex: 1;
		min-width: 200px;
	}
	.result {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 10px 28px 10px 10px;
		border-radius: 10px;
		background: var(--success-soft);
		font-size: 12px;
	}
	.result.failed {
		background: var(--danger-soft);
	}
	.result pre {
		margin: 0;
		max-height: 200px;
		overflow: auto;
		font-family: var(--font-mono);
		font-size: 11px;
		white-space: pre-wrap;
	}
	.close {
		position: absolute;
		top: 6px;
		right: 8px;
		background: transparent;
		border: 0;
		color: var(--muted);
		font-size: 16px;
	}
</style>
