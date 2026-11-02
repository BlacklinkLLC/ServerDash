<script>
	let { run, showName = false } = $props();
	let open = $state(false);
	let status = $derived(run.success === true ? 'success' : run.success === false ? 'danger' : 'info');
	let duration = $derived.by(() => {
		if (!run.finishedAt) return null;
		const ms = new Date(run.finishedAt) - new Date(run.startedAt);
		return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`;
	});
</script>

<div class="run">
	<button class="head" onclick={() => (open = !open)}>
		<span class="badge-pill {status}">{run.success === true ? 'ok' : run.success === false ? 'failed' : 'running'}</span>
		{#if showName}<b>{run.workflowName}</b>{/if}
		<span class="muted">{new Date(run.startedAt).toLocaleString()}</span>
		<span class="muted">· {run.triggeredBy}</span>
		{#if duration}<span class="muted">· {duration}</span>{/if}
		<span class="chev">{open ? '▾' : '▸'}</span>
	</button>
	{#if open}<pre>{run.log || '(no output)'}</pre>{/if}
</div>

<style>
	.run {
		border: 1px solid var(--border);
		border-radius: 10px;
		overflow: hidden;
	}
	.head {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 8px 10px;
		background: transparent;
		border: 0;
		color: var(--text);
		text-align: left;
		font-size: 12px;
	}
	.muted {
		color: var(--muted);
	}
	.chev {
		margin-left: auto;
		color: var(--muted);
	}
	pre {
		margin: 0;
		padding: 10px;
		max-height: 320px;
		overflow: auto;
		font-family: var(--font-mono);
		font-size: 11px;
		white-space: pre-wrap;
		border-top: 1px solid var(--border);
		background: var(--bg);
	}
</style>
