<script>
	import { api } from '../api.js';
	import Icon from '../Icon.svelte';
	import PluginWidget from './PluginWidget.svelte';
	import PluginActions from './PluginActions.svelte';
	import PluginSettings from './PluginSettings.svelte';

	let { isAdmin = false, onchange } = $props();
	let list = $state([]);
	let userDir = $state('');
	let tab = $state('overview');
	let error = $state(null);
	let loaded = $state(false);
	let openSettings = $state(null);
	let reloadMsg = $state(null);

	async function load() {
		try {
			const r = await api.plugins();
			list = r.plugins;
			userDir = r.userDir;
			error = null;
		} catch (e) {
			error = e.message;
		} finally {
			loaded = true;
		}
	}
	load();

	let enabled = $derived(list.filter((p) => p.enabled));

	async function setState(p, patch) {
		try {
			await api.updatePlugin(p.id, { enabled: p.enabled, dashboard: p.dashboard, settings: p.values ?? {}, ...patch });
			await load();
			onchange?.();
		} catch (e) {
			error = e.message;
		}
	}

	async function reload() {
		const r = await api.reloadPlugins();
		reloadMsg = r.errors.length ? `Found ${r.count} plugins. Skipped: ${r.errors.join('; ')}` : `Found ${r.count} plugins.`;
		await load();
	}
</script>

<section>
	<header class="top">
		<div>
			<h2>Plugins</h2>
			<p class="muted">Extra widgets and tools. Each plugin is a folder with a <code>plugin.json</code> and a few scripts.</p>
		</div>
		{#if isAdmin}
			<nav>
				<button class:active={tab === 'overview'} onclick={() => (tab = 'overview')}>Overview</button>
				<button class:active={tab === 'manage'} onclick={() => (tab = 'manage')}>Manage ({list.length})</button>
			</nav>
		{/if}
	</header>

	{#if error}<p class="err">{error}</p>{/if}

	{#if tab === 'overview'}
		{#if loaded && enabled.length === 0}
			<div class="empty panel">
				<Icon name="Puzzle" size={28} />
				<b>No plugins are turned on</b>
				{#if isAdmin}
					<span class="muted">Turn some on under Manage.</span>
					<button class="btn primary" onclick={() => (tab = 'manage')}>Browse plugins</button>
				{:else}
					<span class="muted">An admin can turn plugins on.</span>
				{/if}
			</div>
		{/if}
		{#each enabled as p (p.id)}
			<div class="plugin-block">
				{#if p.actions?.length}
					<div class="actions-row"><PluginActions plugin={p} /></div>
				{/if}
				<div class="grid">
					{#each p.widgets ?? [] as w (w.id)}<PluginWidget plugin={p} widget={w} />{/each}
				</div>
			</div>
		{/each}
	{:else}
		<div class="toolbar">
			<span class="muted small">Add your own plugins by putting their folders in <code>{userDir}</code>, then click Rescan.</span>
			<button class="btn subtle" onclick={reload}>Rescan folders</button>
		</div>
		{#if reloadMsg}<p class="muted small">{reloadMsg}</p>{/if}
		<div class="manage">
			{#each list as p (p.id)}
				<div class="card panel" class:off={!p.enabled}>
					<div class="card-top">
						<span class="picon"><Icon name={p.icon || 'Puzzle'} size={18} /></span>
						<div class="meta">
							<b>{p.name} <span class="ver">v{p.version || '0'}</span> {#if p.source === 'user'}<span class="badge-pill accent">yours</span>{/if}</b>
							<span class="muted">{p.description}</span>
						</div>
						<label class="toggle" title={p.enabled ? 'On' : 'Off'}>
							<input type="checkbox" checked={p.enabled} onchange={() => setState(p, { enabled: !p.enabled })} />
							<span></span>
						</label>
					</div>
					{#if p.missing?.length}
						<p class="warn">Needs {p.missing.join(', ')}, which isn't installed on this server.</p>
					{/if}
					<div class="details">
						{#if p.widgets?.length}<span class="muted small">Widgets: {p.widgets.map((w) => w.title).join(', ')}</span>{/if}
						{#if p.actions?.length}<span class="muted small">Actions: {p.actions.map((a) => a.label).join(', ')}</span>{/if}
					</div>
					<div class="card-actions">
						<label class="inline">
							<input type="checkbox" checked={p.dashboard} disabled={!p.enabled || !p.widgets?.length} onchange={() => setState(p, { dashboard: !p.dashboard })} />
							Show on dashboard
						</label>
						{#if p.settings?.length}
							<button class="btn subtle" onclick={() => (openSettings = openSettings === p.id ? null : p.id)}>
								<Icon name="Settings" size={12} /> Settings
							</button>
						{/if}
					</div>
					{#if openSettings === p.id}
						<div class="settings"><PluginSettings plugin={p} onsaved={load} /></div>
					{/if}
				</div>
			{:else}
				<p class="muted">No plugins found.</p>
			{/each}
		</div>
		<p class="muted small note">
			Plugins run their commands with ServerDash's own permissions, so only turn on plugins you trust. Only admins can turn plugins on or change their settings, and each action says whether operators may run it.
		</p>
	{/if}
</section>

<style>
	.top {
		display: flex;
		justify-content: space-between;
		align-items: flex-end;
		gap: 16px;
		flex-wrap: wrap;
		margin-bottom: 16px;
	}
	h2 {
		margin: 0 0 4px;
	}
	.muted {
		color: var(--muted);
		margin: 0;
	}
	.small {
		font-size: 11px;
	}
	.err {
		color: var(--danger);
	}
	nav {
		display: flex;
		gap: 4px;
	}
	nav button {
		padding: 7px 14px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: transparent;
		color: var(--muted);
	}
	nav button.active {
		color: var(--text);
		border-color: var(--accent);
	}
	.empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
		padding: 40px;
		text-align: center;
		color: var(--muted);
	}
	.empty b {
		color: var(--text);
	}
	.plugin-block {
		display: flex;
		flex-direction: column;
		gap: 10px;
		margin-bottom: 18px;
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
		gap: 12px;
	}
	.toolbar {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
		margin-bottom: 10px;
	}
	.manage {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
		gap: 12px;
	}
	.card {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.card.off .card-top {
		opacity: 0.75;
	}
	.card-top {
		display: flex;
		gap: 12px;
		align-items: flex-start;
	}
	.picon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		border-radius: 10px;
		background: var(--accent-soft);
		color: var(--accent);
		flex-shrink: 0;
	}
	.meta {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 3px;
		font-size: 12px;
	}
	.meta b {
		font-size: 14px;
	}
	.ver {
		color: var(--muted);
		font-weight: 400;
		font-size: 11px;
	}
	.warn {
		margin: 0;
		font-size: 12px;
		color: var(--warning);
	}
	.details {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.card-actions {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}
	.card-actions .btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		font-size: 11px;
	}
	.inline {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
	}
	.settings {
		border-top: 1px solid var(--border);
		padding-top: 12px;
	}
	.note {
		margin-top: 16px;
	}
	.toggle {
		position: relative;
		width: 36px;
		height: 20px;
		flex-shrink: 0;
	}
	.toggle input {
		opacity: 0;
		width: 0;
		height: 0;
	}
	.toggle span {
		position: absolute;
		inset: 0;
		border-radius: 999px;
		background: var(--sur3);
		cursor: pointer;
		transition: background 0.15s ease;
	}
	.toggle span::after {
		content: '';
		position: absolute;
		left: 3px;
		top: 3px;
		width: 14px;
		height: 14px;
		border-radius: 50%;
		background: var(--text);
		transition: transform 0.15s ease;
	}
	.toggle input:checked + span {
		background: var(--accent);
	}
	.toggle input:checked + span::after {
		transform: translateX(16px);
		background: #1f1400;
	}
	.toggle input:focus-visible + span {
		box-shadow: 0 0 0 3px var(--accent-soft);
	}
</style>
