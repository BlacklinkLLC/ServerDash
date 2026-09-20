<script>
	let { state } = $props();

	const statusFor = (s) => {
		switch (s) {
			case 'running':
				return { color: 'var(--status-good)', label: 'Running', icon: '●' };
			case 'paused':
				return { color: 'var(--status-warning)', label: 'Paused', icon: '●' };
			case 'restarting':
				return { color: 'var(--status-warning)', label: 'Restarting', icon: '●' };
			case 'exited':
			case 'dead':
				return { color: 'var(--status-critical)', label: 'Stopped', icon: '●' };
			default:
				return { color: 'var(--text-muted)', label: s ?? 'Unknown', icon: '●' };
		}
	};

	let status = $derived(statusFor(state));
</script>

<span class="badge" style="color: {status.color}">
	<span class="dot" style="background: {status.color}"></span>
	{status.label}
</span>

<style>
	.badge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 13px;
		font-weight: 600;
	}
	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}
</style>
