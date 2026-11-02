<script>
	import { api } from '../api.js';
	import PluginWidget from './PluginWidget.svelte';

	// Widgets from plugins an admin chose to show on the dashboard.
	let { version = 0 } = $props();
	let shown = $state([]);
	$effect(() => {
		version;
		api.plugins()
			.then((r) => (shown = r.plugins.filter((p) => p.enabled && p.dashboard)))
			.catch(() => (shown = []));
	});
</script>

{#if shown.length}
	<div class="grid">
		{#each shown as p (p.id)}
			{#each p.widgets ?? [] as w (w.id)}<PluginWidget plugin={p} widget={w} />{/each}
		{/each}
	</div>
{/if}

<style>
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
		gap: 12px;
		margin: 16px 0;
	}
</style>
