<script>
	import UsersTab from './UsersTab.svelte';
	import AutomationTab from './AutomationTab.svelte';
	import ScriptsTab from './ScriptsTab.svelte';
	import WorkflowsTab from './WorkflowsTab.svelte';
	import AppearanceTab from './AppearanceTab.svelte';
	import UpdatesTab from './UpdatesTab.svelte';

	let { currentUserId, branding, onbrandingchange } = $props();

	let tab = $state('users');
</script>

<section>
	<h2>Admin</h2>
	<nav class="tabs">
		<button class:active={tab === 'users'} onclick={() => (tab = 'users')}>Users</button>
		<button class:active={tab === 'automation'} onclick={() => (tab = 'automation')}>Automation</button>
		<button class:active={tab === 'scripts'} onclick={() => (tab = 'scripts')}>Scripts</button>
		<button class:active={tab === 'workflows'} onclick={() => (tab = 'workflows')}>Workflows</button>
		<button class:active={tab === 'appearance'} onclick={() => (tab = 'appearance')}>Appearance</button>
		<button class:active={tab === 'updates'} onclick={() => (tab = 'updates')}>Updates</button>
	</nav>

	{#if tab === 'users'}
		<UsersTab {currentUserId} />
	{:else if tab === 'automation'}
		<AutomationTab />
	{:else if tab === 'scripts'}
		<ScriptsTab />
	{:else if tab === 'workflows'}
		<WorkflowsTab />
	{:else if tab === 'appearance'}
		<AppearanceTab {branding} {onbrandingchange} />
	{:else if tab === 'updates'}
		<UpdatesTab />
	{/if}
</section>

<style>
	h2 {
		margin: 0 0 14px;
		font-size: 22px;
	}
	.tabs {
		display: flex;
		gap: 4px;
		margin-bottom: 18px;
		border-bottom: 1px solid var(--border);
		flex-wrap: wrap;
	}
	.tabs button {
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		padding: 8px 4px;
		margin-right: 18px;
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--muted);
	}
	.tabs button.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
		font-weight: 500;
	}
</style>
