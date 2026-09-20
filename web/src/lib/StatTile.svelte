<script>
	let { label, value, percent = null } = $props();
</script>

<div class="tile">
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
		background: var(--surface-1);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 16px 18px;
		min-width: 160px;
	}
	.label {
		font-size: 12px;
		font-weight: 600;
		letter-spacing: 0.02em;
		text-transform: uppercase;
		color: var(--text-muted);
	}
	.value {
		font-size: 26px;
		font-weight: 600;
		color: var(--text-primary);
		font-variant-numeric: proportional-nums;
	}
	.bar-track {
		height: 6px;
		border-radius: 3px;
		background: var(--sequential-100);
		overflow: hidden;
		margin-top: 4px;
	}
	.bar-fill {
		height: 100%;
		background: var(--sequential-500);
		border-radius: 3px;
		transition: width 0.3s ease;
	}
</style>
