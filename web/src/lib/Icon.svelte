<script module>
	// Blacklink's icon set, loaded once from its CDN at runtime so the
	// dashboard always shows the current brand icons without a rebuild.
	const ICONS_URL = 'https://cdn.blacklink.net/blacklink-icons.js';
	let lib;
	function loadLib() {
		lib ??= import(/* @vite-ignore */ ICONS_URL).catch((e) => {
			lib = undefined; // retry on next mount, e.g. after a network blip
			throw e;
		});
		return lib;
	}
</script>

<script>
	// variant="mono" (default) inlines the SVG with fill=currentColor so it
	// follows text color across themes; "color" keeps Blacklink's own paint.
	let { name, size = 16, variant = 'mono', label = undefined } = $props();

	let svg = $state('');

	$effect(() => {
		const n = name;
		let cancelled = false;
		loadLib()
			.then((m) => (m.hasIcon(n) ? m.getInlineSvg(n, { variant, title: label }) : ''))
			.then((markup) => {
				if (!cancelled) svg = markup;
			})
			.catch(() => {
				if (!cancelled) svg = '';
			});
		return () => (cancelled = true);
	});
</script>

<span class="icon" style="width: {size}px; height: {size}px" aria-hidden={label ? undefined : 'true'}>
	<!-- Markup comes from Blacklink's own CDN, the same origin as the script. -->
	{@html svg}
</span>

<style>
	.icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		line-height: 0;
	}
	.icon :global(svg) {
		width: 100%;
		height: 100%;
	}
</style>
