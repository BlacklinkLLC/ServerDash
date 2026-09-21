<script>
	import { onDestroy, onMount } from 'svelte';
	import { api } from './lib/api.js';
	import SystemOverview from './lib/SystemOverview.svelte';
	import ContainerList from './lib/ContainerList.svelte';
	import KubernetesView from './lib/KubernetesView.svelte';
	import AdminPanel from './lib/admin/AdminPanel.svelte';
	import SetupPage from './lib/SetupPage.svelte';
	import LoginPage from './lib/LoginPage.svelte';
	import BrandLogo from './lib/BrandLogo.svelte';

	// 'loading' | 'setup' | 'login' | 'ready'
	let authState = $state('loading');
	let user = $state(null);
	let branding = $state({ appName: 'ServerDash', logoUrl: null });

	let page = $state('dashboard'); // 'dashboard' | 'kubernetes' | 'admin'

	let snapshot = $state(null);
	let containers = $state([]);
	let error = $state(null);

	async function loadBranding() {
		try {
			branding = await api.settings();
		} catch {
			// Falls back to the default name/no logo — branding is cosmetic,
			// never worth blocking the rest of the app over.
		}
		document.title = branding.appName;
	}

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
		await Promise.all([loadBranding(), resolveAuth()]);
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
	<SetupPage {branding} onready={onAuthed} />
{:else if authState === 'login'}
	<LoginPage {branding} onready={onAuthed} />
{:else}
	<header>
		<div class="brand">
			<BrandLogo {branding} size={22} />
			<h1>{branding.appName}</h1>
		</div>
		<nav>
			<button class:active={page === 'dashboard'} onclick={() => (page = 'dashboard')}>Dashboard</button>
			<button class:active={page === 'kubernetes'} onclick={() => (page = 'kubernetes')}>Kubernetes</button>
			{#if isAdmin}
				<button class:active={page === 'admin'} onclick={() => (page = 'admin')}>Admin</button>
			{/if}
		</nav>
		<div class="user">
			<span>{user.username} <span class="muted">({user.role})</span></span>
			<button class="btn subtle" onclick={logout}>Sign out</button>
		</div>
	</header>

	<main>
		{#if error}
			<p class="error">Couldn't reach {branding.appName}'s API: {error}</p>
		{/if}

		{#if page === 'dashboard'}
			<SystemOverview {snapshot} />
			<ContainerList {containers} {canOperate} onaction={refresh} />
		{:else if page === 'kubernetes'}
			<KubernetesView {canOperate} />
		{:else if page === 'admin'}
			<AdminPanel currentUserId={user.id} {branding} onbrandingchange={(b) => (branding = b)} />
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
		color: var(--muted);
	}
	header {
		display: flex;
		align-items: center;
		gap: 24px;
		padding: 16px 24px;
		border-bottom: 1px solid var(--border);
		background: var(--surface);
	}
	.brand {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	h1 {
		margin: 0;
		font-size: 17px;
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
		border-radius: 10px;
		padding: 7px 14px;
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--muted);
	}
	nav button.active {
		background: var(--accent-soft);
		color: var(--accent);
		font-weight: 500;
	}
	.user {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 12px;
	}
	main {
		max-width: 980px;
		margin: 0 auto;
		padding: 28px 24px;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.error {
		color: var(--danger);
		font-size: 13px;
	}
</style>
