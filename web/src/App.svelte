<script>
	import { onDestroy, onMount } from 'svelte';
	import { api } from './lib/api.js';
	import SystemOverview from './lib/SystemOverview.svelte';
	import ContainerList from './lib/ContainerList.svelte';

	let snapshot = $state(null);
	let containers = $state([]);
	let error = $state(null);

	async function refresh() {
		try {
			const [sys, list] = await Promise.all([api.system(), api.containers()]);
			snapshot = sys;
			containers = list;
			error = null;
		} catch (e) {
			error = e.message;
		}
	}

	let timer;
	onMount(() => {
		refresh();
		timer = setInterval(refresh, 5000);
	});
	onDestroy(() => clearInterval(timer));
</script>

<header>
	<h1>ServerDash</h1>
</header>

<main>
	{#if error}
		<p class="error">Couldn't reach the ServerDash API: {error}</p>
	{/if}
	<SystemOverview {snapshot} />
	<ContainerList {containers} onaction={refresh} />
</main>

<style>
	header {
		padding: 20px 24px;
		border-bottom: 1px solid var(--gridline);
	}
	h1 {
		margin: 0;
		font-size: 18px;
		letter-spacing: -0.01em;
	}
	main {
		max-width: 960px;
		margin: 0 auto;
		padding: 24px;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.error {
		color: var(--status-critical);
		font-size: 13px;
	}
</style>
