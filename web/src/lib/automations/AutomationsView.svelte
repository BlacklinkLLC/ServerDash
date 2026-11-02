<script>
	import Icon from '../Icon.svelte';
	import WorkflowsPanel from './WorkflowsPanel.svelte';
	import ActivityPanel from './ActivityPanel.svelte';
	import ScriptsPanel from './ScriptsPanel.svelte';
	import RulesPanel from './RulesPanel.svelte';

	const tabs = [
		['workflows', 'Workflows', 'Lightning'],
		['activity', 'Activity', 'Clock'],
		['scripts', 'Scripts', 'Code'],
		['rules', 'Built-in rules', 'Settings'],
	];
	let tab = $state('workflows');
</script>

<section>
	<header>
		<h2>Automations</h2>
		<p class="muted">Workflows run on a schedule, when something happens on the server, or when a webhook is called.</p>
	</header>
	<nav>
		{#each tabs as [key, label, icon] (key)}
			<button class:active={tab === key} onclick={() => (tab = key)}><Icon name={icon} size={14} /> {label}</button>
		{/each}
	</nav>
	{#if tab === 'workflows'}<WorkflowsPanel />
	{:else if tab === 'activity'}<ActivityPanel />
	{:else if tab === 'scripts'}<ScriptsPanel />
	{:else}<RulesPanel />{/if}
</section>

<style>
	header h2 {
		margin: 0 0 4px;
	}
	.muted {
		color: var(--muted);
		margin: 0 0 14px;
	}
	nav {
		display: flex;
		gap: 4px;
		margin-bottom: 16px;
		border-bottom: 1px solid var(--border);
	}
	nav button {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 8px 14px;
		background: transparent;
		border: 0;
		border-bottom: 2px solid transparent;
		color: var(--muted);
	}
	nav button.active {
		color: var(--text);
		border-bottom-color: var(--accent);
	}
</style>
