<script>
	let { state } = $props();

	const statusFor = (s) => {
		switch (s) {
			case 'running':
				return { variant: 'success', label: 'Running' };
			case 'paused':
			case 'restarting':
				return { variant: 'accent', label: s === 'paused' ? 'Paused' : 'Restarting' };
			case 'exited':
			case 'dead':
				return { variant: 'danger', label: 'Stopped' };
			default:
				return { variant: 'muted', label: s ?? 'Unknown' };
		}
	};

	let status = $derived(statusFor(state));
</script>

<span class="badge-pill {status.variant}">{status.label}</span>
