<script>
	import { onDestroy, onMount } from 'svelte';
	import { api, setTargetNode } from './lib/api.js';
	import { applyAccentColor } from './lib/color.js';
	import SystemOverview from './lib/SystemOverview.svelte';
	import ContainerList from './lib/ContainerList.svelte';
	import KubernetesView from './lib/KubernetesView.svelte';
	import ToolsView from './lib/ToolsView.svelte';
	import ClusterView from './lib/ClusterView.svelte';
	import PluginsView from './lib/plugins/PluginsView.svelte';
	import DashboardPlugins from './lib/plugins/DashboardPlugins.svelte';
	import AutomationsView from './lib/automations/AutomationsView.svelte';
	import AdminPanel from './lib/admin/AdminPanel.svelte';
	import SetupPage from './lib/SetupPage.svelte';
	import LoginPage from './lib/LoginPage.svelte';
	import BrandLogo from './lib/BrandLogo.svelte';
	import Icon from './lib/Icon.svelte';
	import Footer from './lib/Footer.svelte';

	// 'loading' | 'setup' | 'login' | 'ready'
	let authState = $state('loading');
	let user = $state(null);
	let branding = $state({ appName: 'ServerDash', logoUrl: null, accentColor: '#ffb020', showSystem: true, showContainers: true, refreshSeconds: 5, widgets: [] });
	let runtime = $state(null);

	const pages = ['dashboard', 'tools', 'kubernetes', 'cluster', 'plugins', 'automations', 'admin'];
	// The page lives in the URL hash so a refresh or shared link lands on it.
	let page = $state(pages.includes(location.hash.slice(1)) ? location.hash.slice(1) : 'dashboard');
	$effect(() => {
		history.replaceState(null, '', `#${page}`);
	});

	let snapshot = $state(null);
	let containers = $state([]);
	let metricsHistory = $state([]);
	let topProcesses = $state(null);
	let host = $state(null);
	let cluster = $state(null);
	// The cluster node whose dashboard is being viewed (null = this server).
	let target = $state(null);
	let targetNode = $derived(cluster?.nodes?.find((n) => n.id === target) ?? null);

	function loadCluster() {
		api.cluster()
			.then((c) => (cluster = c))
			.catch(() => {});
	}

	function openServer(id) {
		target = id;
		setTargetNode(id);
		snapshot = null;
		containers = [];
		metricsHistory = [];
		topProcesses = null;
		runtime = null;
		host = null;
		if (page === 'cluster' || page === 'admin' || page === 'automations' || page === 'plugins') page = 'dashboard';
		refresh();
	}
	let error = $state(null);
	let pluginsVersion = $state(0);

	async function loadBranding() {
		try {
			branding = await api.settings();
		} catch {
			// Falls back to the default name/no logo — branding is cosmetic,
			// never worth blocking the rest of the app over.
		}
		applyBranding();
	}

	function applyBranding() {
		document.title = branding.appName;
		applyAccentColor(branding.accentColor || '#ffb020');
		document.documentElement.dataset.theme = branding.theme || 'dark';
		document.documentElement.dataset.density = branding.density || 'comfortable';
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
		if (page === 'cluster' || !cluster) loadCluster();
		if (page === 'dashboard') {
			api.systemHistory()
				.then((h) => (metricsHistory = h))
				.catch(() => {});
			if (canOperate) {
				api.processes('cpu', 6)
					.then((p) => (topProcesses = p))
					.catch(() => {});
			}
		}
		api.runtime()
			.then((r) => (runtime = r))
			.catch(() => {});
	}

	let timer;
	onMount(async () => {
		await Promise.all([loadBranding(), resolveAuth()]);
	});
	$effect(() => {
		if (authState === 'ready') {
			page;
			target;
			if (!host) {
				api.host()
					.then((h) => (host = h))
					.catch(() => {});
			}
			refresh();
			timer = setInterval(refresh, (branding.refreshSeconds || 5) * 1000);
			return () => clearInterval(timer);
		}
	});
	onDestroy(() => clearInterval(timer));

	let canOperate = $derived(user?.role === 'admin' || user?.role === 'operator');
	let isAdmin = $derived(user?.role === 'admin');

	function onBrandingChange(updated) {
		branding = updated;
		applyBranding();
	}
</script>

{#if authState === 'loading'}
	<div class="center"><p class="muted">Loading…</p></div>
{:else if authState === 'setup'}
	<SetupPage {branding} onready={onAuthed} onbrandingchange={onBrandingChange} />
{:else if authState === 'login'}
	<LoginPage {branding} onready={onAuthed} />
{:else}
	<header>
		<div class="brand">
			<BrandLogo {branding} size={22} />
			<h1>{branding.appName}</h1>
		</div>
		<nav>
			<button class:active={page === 'dashboard'} onclick={() => (page = 'dashboard')}><Icon name="Home" size={14} /> Dashboard</button>
			<button class:active={page === 'tools'} onclick={() => (page = 'tools')}><Icon name="Settings" size={14} /> Tools</button>
			<button class:active={page === 'cluster'} onclick={() => (page = 'cluster')}><Icon name="Connect" size={14} /> Cluster</button>
			<button class:active={page === 'kubernetes'} onclick={() => (page = 'kubernetes')}><Icon name="Cloud" size={14} /> Kubernetes</button>
			{#if !target}
				<button class:active={page === 'plugins'} onclick={() => (page = 'plugins')}><Icon name="Puzzle" size={14} /> Plugins</button>
			{/if}
			{#if isAdmin && !target}
				<button class:active={page === 'automations'} onclick={() => (page = 'automations')}><Icon name="Lightning" size={14} /> Automations</button>
				<button class:active={page === 'admin'} onclick={() => (page = 'admin')}><Icon name="Shield" size={14} /> Admin</button>
			{/if}
		</nav>
		{#if cluster?.nodes?.length}
			<label class="switcher" title="Choose which server to manage">
				<Icon name="Desktop" size={14} />
				<select value={target ?? ''} onchange={(e) => openServer(e.currentTarget.value || null)}>
					<option value="">{cluster.self?.hostname ?? 'This server'} (this server)</option>
					{#each cluster.nodes as n (n.id)}
						<option value={n.id} disabled={!n.online || !n.url}>{n.name}{n.online ? '' : ' (offline)'}{n.url ? '' : ' (no address)'}</option>
					{/each}
				</select>
			</label>
		{/if}
		<div class="user">
			<span class="who"><Icon name="User" size={14} /> {user.username} <span class="muted">({user.role})</span></span>
			<button class="btn subtle signout" onclick={logout}><Icon name="Exit" size={13} /> Sign out</button>
		</div>
	</header>

	<main>
		{#if targetNode}
			<div class="remote-banner">
				<Icon name="Connect" size={15} />
				<span>Managing <b>{targetNode.name}</b> through this server{targetNode.role === 'replica' ? ' · replica' : ''}</span>
				<button class="btn subtle" onclick={() => openServer(null)}>Back to this server</button>
			</div>
		{/if}
		{#if error}
			<p class="error">Couldn't reach {branding.appName}'s API: {error}</p>
		{/if}

		{#if page === 'dashboard'}
			{#if branding.showSystem}
				<SystemOverview
					{snapshot}
					{runtime}
					history={metricsHistory}
					{containers}
					processes={topProcesses}
					settings={host?.containersRuntime === false ? { ...branding, widgets: (branding.widgets ?? []).filter((w) => w !== 'containers') } : branding}
					onnavigate={(p) => (page = p)}
				/>
			{/if}
			{#if !target}<DashboardPlugins version={pluginsVersion} />{/if}
			{#if branding.showContainers && host?.containersRuntime !== false}
				<ContainerList {containers} {canOperate} {isAdmin} onaction={refresh} />
			{/if}
		{:else if page === 'tools'}
			<ToolsView {isAdmin} {host} refreshSeconds={branding.refreshSeconds || 5} />
		{:else if page === 'cluster'}
			<ClusterView
				{cluster}
				{isAdmin}
				settings={branding}
				onopen={(id) => {
					openServer(id);
					page = 'dashboard';
				}}
				onchange={loadCluster}
			/>
		{:else if page === 'kubernetes'}
			<KubernetesView {canOperate} />
		{:else if page === 'plugins'}
			<PluginsView {isAdmin} onchange={() => pluginsVersion++} />
		{:else if page === 'automations'}
			<AutomationsView />
		{:else if page === 'admin'}
			<AdminPanel currentUserId={user.id} {branding} onbrandingchange={onBrandingChange} />
		{/if}
	</main>

	<Footer {branding} />
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
		display: inline-flex;
		align-items: center;
		gap: 7px;
		background: none;
		border: none;
		border-radius: 10px;
		padding: 7px 14px;
		font-family: var(--font-sans);
		font-size: 12px;
		color: var(--muted);
	}
	nav button.active {
		background: var(--accent-soft);
		color: var(--accent);
		font-weight: 500;
	}
	.who,
	.signout {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}
	.switcher {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		color: var(--muted);
	}
	.switcher select {
		padding: 7px 10px;
		font-size: 12px;
		border-radius: 10px;
	}
	.remote-banner {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 10px 14px;
		border-radius: 12px;
		border: 1px solid var(--border-strong);
		background: var(--accent-soft);
		font-size: 12px;
	}
	.remote-banner .btn {
		margin-left: auto;
		padding: 6px 12px;
	}
	.user {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 12px;
	}
	main {
		max-width: 1180px;
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
