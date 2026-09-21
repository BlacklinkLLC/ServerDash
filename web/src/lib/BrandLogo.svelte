<script>
	let { branding, size = 22 } = $props();
</script>

{#if branding?.logoUrl}
	<img class="logo" src={branding.logoUrl} alt={branding.appName} style="height: {size}px" />
{:else}
	<span class="mark" style="width: {size}px; height: {size}px; font-size: {size * 0.55}px">
		{(branding?.appName ?? 'S').charAt(0).toUpperCase()}
	</span>
{/if}

<style>
	.logo {
		width: auto;
		object-fit: contain;
	}
	.mark {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		border-radius: 8px;
		background: var(--accent);
		color: #1f1400;
		font-family: var(--font-display);
		font-weight: 800;
	}
</style>
