<script>
	import { onDestroy, onMount } from 'svelte';
	import { api } from './lib/api.js';
	import SystemOverview from './lib/SystemOverview.svelte';
	import ContainerList from './lib/ContainerList.svelte';
	import KubernetesView from './lib/KubernetesView.svelte';
	import AdminPanel from './lib/admin/AdminPanel.svelte';
	import SetupPage from './lib/SetupPage.svelte';
	import LoginPage from './lib/LoginPage.svelte';

	// 'loading' | 'setup' | 'login' | 'ready'
	let authState = $state('loading');
	let user = $state(null);

	let page = $state('dashboard'); // 'dashboard' | 'kubernetes' | 'admin'

	let snapshot = $state(null);
	let containers = $state([]);
	let error = $state(null);

	async function resolveAuth() {
		try {
			user = await api.me();
			authState = 'ready';
			return;
		} catch (e) {
			if (e.status !== 401) {
				error = e.message;
			}
		}
		try {
			const { needsSetup } = await api.setupStatus();
			authState = needsSetup ? 'setup' : 'login';
		} catch (e) {
			error = e.message;
		}
	}

	function onAuthed(u) {
		user = u;
		authState = 'ready';
	}

	async function logout() {
		await api.logout();
		user = null;
		page = 'dashboard';
		authState = 'login';
	}

	async function refresh() {
		try {
			const [sys, list] = await Promise.all([api.system(), api.containers()]);
			snapshot = sys;
			containers = list;
			error = null;
		} catch (e) {
			if (e.status === 401) {
				authState = 'login';
				return;
			}
			error = e.message;
		}
	}

	let timer;
	onMount(async () => {
		await resolveAuth();
	});
	$effect(() => {
		if (authState === 'ready') {
			refresh();
			timer = setInterval(refresh, 5000);
			return () => clearInterval(timer);
		}
	});
	onDestroy(() => clearInterval(timer));

	let canOperate = $derived(user?.role === 'admin' || user?.role === 'operator');
	let isAdmin = $derived(user?.role === 'admin');
</script>

{#if authState === 'loading'}
	<div class="center"><p class="muted">Loading…</p></div>
{:else if authState === 'setup'}
	<SetupPage onready={onAuthed} />
{:else if authState === 'login'}
	<LoginPage onready={onAuthed} />
{:else}
	<header>
		<h1>ServerDash</h1>
		<nav>
			<button class:active={page === 'dashboard'} onclick={() => (page = 'dashboard')}>Dashboard</button>
			<button class:active={page === 'kubernetes'} onclick={() => (page = 'kubernetes')}>Kubernetes</button>
			{#if isAdmin}
				<button class:active={page === 'admin'} onclick={() => (page = 'admin')}>Admin</button>
			{/if}
		</nav>
		<div class="user">
			<span>{user.username} <span class="muted">({user.role})</span></span>
			<button onclick={logout}>Sign out</button>
		</div>
	</header>

	<main>
		{#if error}
			<p class="error">Couldn't reach the ServerDash API: {error}</p>
		{/if}

		{#if page === 'dashboard'}
			<SystemOverview {snapshot} />
			<ContainerList {containers} {canOperate} onaction={refresh} />
		{:else if page === 'kubernetes'}
			<KubernetesView {canOperate} />
		{:else if page === 'admin'}
			<AdminPanel currentUserId={user.id} />
		{/if}
	</main>
{/if}

<style>
	.center {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.muted {
		color: var(--text-muted);
	}
	header {
		display: flex;
		align-items: center;
		gap: 24px;
		padding: 16px 24px;
		border-bottom: 1px solid var(--gridline);
	}
	h1 {
		margin: 0;
		font-size: 18px;
		letter-spacing: -0.01em;
	}
	nav {
		display: flex;
		gap: 4px;
		flex: 1;
	}
	nav button {
		background: none;
		border: none;
		border-radius: 6px;
		padding: 6px 12px;
		font-size: 13px;
		color: var(--text-secondary);
	}
	nav button.active {
		background: var(--surface-1);
		color: var(--text-primary);
		font-weight: 600;
	}
	.user {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 13px;
	}
	.user button {
		background: var(--surface-1);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 5px 10px;
		font-size: 12px;
		color: var(--text-primary);
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
