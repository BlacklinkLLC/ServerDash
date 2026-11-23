<script module>
	const ICONS_URL = 'https://cdn.blacklink.net/blacklink-icons.js';
	let libPromise;

	function loadIcons() {
		libPromise ??= import(/* @vite-ignore */ ICONS_URL)
			.then((m) => {
				m.defineIconElement?.();
				return m;
			})
			.catch((e) => {
				libPromise = undefined;
				throw e;
			});
		return libPromise;
	}
</script>

<script>
	let { name, size = 16, variant = 'mono', label = undefined } = $props();
	let ready = $state(false);

	$effect(() => {
		let cancelled = false;
		loadIcons().then(
			() => {
				if (!cancelled) ready = true;
			},
			() => {
				if (!cancelled) ready = false;
			},
		);
		return () => {
			cancelled = true;
		};
	});
</script>

<span
	class="icon"
	style="width: {size}px; height: {size}px"
	role={label ? 'img' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
>
	{#if ready}
		<blacklink-icon {name} {variant} {size} label={label}></blacklink-icon>
	{/if}
</span>

<style>
	.icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		line-height: 0;
	}
	.icon :global(svg),
	.icon :global(img) {
		width: 100%;
		height: 100%;
	}
</style>
