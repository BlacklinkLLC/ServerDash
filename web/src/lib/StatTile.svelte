<script>
	let { label, value, percent = null } = $props();
</script>

<div class="tile app-card">
	<span class="label">{label}</span>
	<span class="value">{value}</span>
	{#if percent !== null}
		<div class="bar-track" role="progressbar" aria-valuenow={Math.round(percent)} aria-valuemin="0" aria-valuemax="100">
			<div class="bar-fill" style="width: {Math.min(100, Math.max(0, percent))}%"></div>
		</div>
	{/if}
</div>

<style>
	.tile {
		display: flex;
		flex-direction: column;
		gap: 6px;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 14px;
		padding: 20px 18px 16px;
		min-width: 160px;
	}
	.label {
		font-size: 10px;
		font-weight: 500;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--muted);
	}
	.value {
		font-family: var(--font-display);
		font-size: 22px;
		font-weight: 800;
		color: var(--text);
		font-variant-numeric: proportional-nums;
	}
	.bar-track {
		height: 6px;
		border-radius: 3px;
		background: var(--sur2);
		overflow: hidden;
		margin-top: 4px;
	}
	.bar-fill {
		height: 100%;
		background: var(--accent);
		border-radius: 3px;
		transition: width 0.3s ease;
	}
</style>
