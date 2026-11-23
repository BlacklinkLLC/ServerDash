<script>
	import { onDestroy, onMount } from 'svelte';
	import { api, setTargetNode } from './lib/api.js';
	import { applyAppearance, mergeSettings, resolveWidgetLayout } from './lib/preferences.js';
	import SystemOverview from './lib/SystemOverview.svelte';
	import ContainerList from './lib/ContainerList.svelte';
	import ContainersView from './lib/containers/ContainersView.svelte';
	import ToolsView from './lib/ToolsView.svelte';
	import ClusterView from './lib/ClusterView.svelte';
	import PluginsView from './lib/plugins/PluginsView.svelte';
	import PreferencesPanel from './lib/PreferencesPanel.svelte';
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
	let globalSettings = $state({ appName: 'ServerDash', logoUrl: null, accentColor: '#ffb020', showSystem: true, showContainers: true, refreshSeconds: 5, widgets: [] });
	let preferences = $state({});
	let branding = $derived(mergeSettings(globalSettings, preferences));
	let pluginWidgets = $state([]);
	let runtime = $state(null);

	const pages = ['dashboard', 'tools', 'containers', 'cluster', 'plugins', 'automations', 'admin', 'preferences'];
	// The page lives in the URL hash so a refresh or shared link lands on it.
	const initialHash = location.hash.slice(1).replace('kubernetes', 'containers');
	let page = $state(pages.includes(initialHash) ? initialHash : 'dashboard');
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
			globalSettings = await api.settings();
		} catch {
		}
	}

	async function loadPreferences() {
		try {
			preferences = await api.preferences();
		} catch {
			preferences = {};
		}
	}

	async function loadPluginWidgets() {
		try {
			const result = await api.plugins();
			pluginWidgets = result.plugins
				.filter((p) => p.enabled && p.dashboard)
				.flatMap((p) => (p.widgets ?? []).map((widget) => ({ id: `plugin:${p.id}:${widget.id}`, plugin: p, widget })));
		} catch {
			pluginWidgets = [];
		}
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
		preferences = {};
		pluginWidgets = [];
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
		branding;
		applyAppearance(branding);
	});
	$effect(() => {
		if (authState === 'ready') {
			loadPreferences();
			loadPluginWidgets();
		}
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
	let dashboardLayout = $derived(resolveWidgetLayout(branding, pluginWidgets));

	function onBrandingChange(updated) {
		globalSettings = updated;
	}

	function onPreferencesChange(updated) {
		preferences = updated ?? {};
	}

	function onPreferencesReset() {
		preferences = {};
	}
</script>

{#if authState === 'loading'}
	<div class="center"><p class="muted">Loading…</p></div>
{:else if authState === 'setup'}
	<SetupPage {branding} onready={onAuthed} onbrandingchange={onBrandingChange} />
{:else if authState === 'login'}
	<LoginPage {branding} onready={onAuthed} />
{:else}
	<header class="topbar">
		<div class="brand">
			<BrandLogo {branding} size={branding.logoHeight || 22} />
			<div class="brand-copy">
				<span class="eyebrow">Server dashboard</span>
				<h1>{branding.appName}</h1>
			</div>
		</div>
		<nav class="primary-nav" aria-label="Primary">
			<button class:active={page === 'dashboard'} title="Dashboard" onclick={() => (page = 'dashboard')}><Icon name="Home" size={14} /> <span>Dashboard</span></button>
			<button class:active={page === 'tools'} title="Tools" onclick={() => (page = 'tools')}><Icon name="Settings" size={14} /> <span>Tools</span></button>
			<button class:active={page === 'containers'} title="Containers" onclick={() => (page = 'containers')}><Icon name="Container" size={14} /> <span>Containers</span></button>
			<button class:active={page === 'cluster'} title="Cluster" onclick={() => (page = 'cluster')}><Icon name="Organization" size={14} /> <span>Cluster</span></button>
			{#if !target}
				<button class:active={page === 'plugins'} title="Plugins" onclick={() => (page = 'plugins')}><Icon name="Puzzle" size={14} /> <span>Plugins</span></button>
			{/if}
			<button class:active={page === 'preferences'} title="Preferences" onclick={() => (page = 'preferences')}><Icon name="Brush" size={14} /> <span>Preferences</span></button>
			{#if isAdmin && !target}
				<span class="nav-divider" aria-hidden="true"></span>
				<button class:active={page === 'automations'} title="Automations" onclick={() => (page = 'automations')}><Icon name="Bolt" size={14} /> <span>Automations</span></button>
				<button class:active={page === 'admin'} title="Admin" onclick={() => (page = 'admin')}><Icon name="Shield" size={14} /> <span>Admin</span></button>
			{/if}
		</nav>
		<div class="topbar-actions">
			{#if cluster?.nodes?.length}
				<label class="switcher" title="Choose which server to manage">
					<Icon name="Server" size={14} />
					<select value={target ?? ''} onchange={(e) => openServer(e.currentTarget.value || null)}>
						<option value="">{cluster.self?.hostname ?? 'This server'} (this server)</option>
						{#each cluster.nodes as n (n.id)}
							<option value={n.id} disabled={!n.online || !n.url}>{n.name}{n.online ? '' : ' (offline)'}{n.url ? '' : ' (no address)'}</option>
						{/each}
					</select>
				</label>
			{/if}
			<div class="user">
				<span class="avatar"><Icon name="User" size={14} /></span>
				<span class="who"><b>{user.username}</b><small>{user.role}</small></span>
				<button class="btn subtle signout" title="Sign out" onclick={logout}><Icon name="Exit" size={13} /> <span>Sign out</span></button>
			</div>
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
					settings={branding}
					widgetLayout={host?.containersRuntime === false ? dashboardLayout.filter((item) => item.id !== 'containers') : dashboardLayout}
					{pluginWidgets}
					onnavigate={(p) => (page = p)}
				/>
			{/if}
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
		{:else if page === 'containers'}
			<ContainersView {containers} {canOperate} {isAdmin} {host} onaction={refresh} />
		{:else if page === 'plugins'}
			<PluginsView {isAdmin} onchange={() => { pluginsVersion++; loadPluginWidgets(); }} />
		{:else if page === 'automations'}
			<AutomationsView />
		{:else if page === 'preferences'}
			<PreferencesPanel {branding} onpreferences={onPreferencesChange} onreset={onPreferencesReset} />
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
	.topbar {
		position: sticky;
		top: 0;
		z-index: 20;
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) auto;
		align-items: center;
		gap: 18px;
		padding: 11px 24px;
		border-bottom: 1px solid var(--border);
		background: var(--surface);
	}
	.brand {
		display: flex;
		align-items: center;
		gap: 11px;
		min-width: 0;
	}
	.brand-copy {
		display: flex;
		flex-direction: column;
		gap: 1px;
		min-width: 0;
	}
	.eyebrow {
		font-size: 9px;
		font-weight: 600;
		letter-spacing: 0.14em;
		text-transform: uppercase;
		color: var(--muted);
	}
	h1 {
		max-width: 220px;
		margin: 0;
		font-size: 16px;
		letter-spacing: -0.01em;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.primary-nav {
		display: flex;
		align-items: center;
		gap: 3px;
		min-width: 0;
		padding: 3px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--sur2);
		overflow-x: auto;
		scrollbar-width: none;
	}
	.primary-nav::-webkit-scrollbar {
		display: none;
	}
	.primary-nav button {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		flex: 0 0 auto;
		white-space: nowrap;
		background: none;
		border: none;
		border-radius: 9px;
		padding: 8px 12px;
		font-family: var(--font-sans);
		font-size: 12px;
		color: var(--muted);
		transition:
			background 0.15s ease,
			color 0.15s ease;
	}
	.primary-nav button:hover {
		background: rgba(255, 255, 255, 0.04);
		color: var(--text);
	}
	.primary-nav button.active {
		background: var(--accent-soft);
		color: var(--accent);
		font-weight: 500;
		box-shadow: inset 0 0 0 1px var(--border-strong);
	}
	.nav-divider {
		width: 1px;
		height: 20px;
		margin: 0 4px;
		background: var(--border2);
	}
	.topbar-actions {
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.switcher {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		color: var(--muted);
	}
	.switcher select {
		max-width: 190px;
		padding: 7px 10px;
		font-size: 12px;
		border-radius: 10px;
	}
	.user {
		display: flex;
		align-items: center;
		gap: 9px;
		font-size: 12px;
	}
	.avatar {
		width: 30px;
		height: 30px;
		display: grid;
		place-items: center;
		border-radius: 50%;
		border: 1px solid var(--border-strong);
		background: var(--accent-soft);
		color: var(--accent);
	}
	.who {
		display: flex;
		flex-direction: column;
		line-height: 1.15;
	}
	.who b {
		font-size: 12px;
		font-weight: 600;
	}
	.who small {
		font-size: 9px;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--muted);
	}
	.signout {
		white-space: nowrap;
	}
	@media (max-width: 1120px) {
		.topbar {
			grid-template-columns: auto minmax(0, 1fr);
			row-gap: 10px;
		}
		.topbar-actions {
			grid-column: 2;
			justify-content: flex-end;
		}
		.primary-nav {
			grid-column: 1 / -1;
			grid-row: 2;
		}
	}
	@media (max-width: 640px) {
		.topbar {
			padding: 10px 14px;
		}
		.topbar-actions {
			grid-column: 2;
		}
		.who small,
		.signout span,
		.primary-nav button span {
			display: none;
		}
		.primary-nav button {
			padding: 8px 10px;
		}
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
