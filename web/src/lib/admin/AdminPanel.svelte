<script>
	import UsersTab from './UsersTab.svelte';
	import AutomationTab from './AutomationTab.svelte';
	import ScriptsTab from './ScriptsTab.svelte';

	let { currentUserId } = $props();

	let tab = $state('users');
</script>

<section>
	<h2>Admin</h2>
	<nav class="tabs">
		<button class:active={tab === 'users'} onclick={() => (tab = 'users')}>Users</button>
		<button class:active={tab === 'automation'} onclick={() => (tab = 'automation')}>Automation</button>
		<button class:active={tab === 'scripts'} onclick={() => (tab = 'scripts')}>Scripts</button>
	</nav>

	{#if tab === 'users'}
		<UsersTab {currentUserId} />
	{:else if tab === 'automation'}
		<AutomationTab />
	{:else if tab === 'scripts'}
		<ScriptsTab />
	{/if}
</section>

<style>
	h2 {
		margin: 0 0 12px;
		font-size: 20px;
	}
	.tabs {
		display: flex;
		gap: 4px;
		margin-bottom: 16px;
		border-bottom: 1px solid var(--gridline);
	}
	.tabs button {
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		padding: 8px 4px;
		margin-right: 16px;
		font-size: 13px;
		color: var(--text-secondary);
	}
	.tabs button.active {
		color: var(--text-primary);
		border-bottom-color: var(--sequential-500);
		font-weight: 600;
	}
</style>
