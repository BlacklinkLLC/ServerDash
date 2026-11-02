<script>
	import { api } from './api.js';
	import Icon from './Icon.svelte';
	import { formatUptime, statusFor } from './format.js';

	let { cluster, isAdmin, settings, onopen, onchange } = $props();

	let warn = $derived(settings?.warnThreshold ?? 75);
	let crit = $derived(settings?.critThreshold ?? 90);
	let m = $derived(cluster?.membership ?? { mode: 'standalone' });
	let nodes = $derived(cluster?.nodes ?? []);
	let online = $derived(nodes.filter((n) => n.online).length);

	let error = $state(null);

	// --- Add node ---
	let adding = $state(false);
	let joinRole = $state('node');
	let primaryUrl = $state(location.origin);
	let ttlHours = $state(1);
	let token = $state(null);
	let copied = $state(false);

	async function createToken() {
		error = null;
		try {
			token = await api.createJoinToken(joinRole, primaryUrl, Number(ttlHours));
			onchange?.();
		} catch (e) {
			error = e.message;
		}
	}
	async function copy(text) {
		await navigator.clipboard.writeText(text);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	// --- Edit / remove node ---
	let editing = $state(null); // { id, name, url, role }
	let removing = $state(null);

	async function saveNode(e) {
		e.preventDefault();
		error = null;
		try {
			await api.updateClusterNode(editing.id, { name: editing.name, url: editing.url, role: editing.role });
			editing = null;
			onchange?.();
		} catch (e) {
			error = e.message;
		}
	}
	async function removeNode() {
		const id = removing.id;
		removing = null;
		try {
			await api.deleteClusterNode(id);
			onchange?.();
		} catch (e) {
			error = e.message;
		}
	}

	// --- This server as a node ---
	let joinToken = $state('');
	let selfName = $state('');
	let selfUrl = $state(location.origin);
	let leaving = $state(false);
	let busy = $state(false);

	// Prefill once per membership change, not on every refresh, so typing
	// into the fields isn't overwritten by the next poll.
	let prefilledFor = null;
	$effect(() => {
		const key = `${m.mode}:${m.nodeId ?? ''}`;
		if (key === prefilledFor) return;
		prefilledFor = key;
		if (m.mode === 'node') {
			selfName = m.name ?? '';
			selfUrl = m.selfUrl ?? '';
		} else {
			selfName = cluster?.self?.hostname ?? '';
		}
	});

	async function join(e) {
		e.preventDefault();
		busy = true;
		error = null;
		try {
			await api.joinCluster(joinToken, selfName, selfUrl);
			joinToken = '';
			onchange?.();
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}
	async function saveSelf(e) {
		e.preventDefault();
		error = null;
		try {
			await api.updateClusterSelf(selfName, selfUrl);
			onchange?.();
		} catch (e) {
			error = e.message;
		}
	}
	async function leave() {
		leaving = false;
		try {
			await api.leaveCluster();
			onchange?.();
		} catch (e) {
			error = e.message;
		}
	}

	function ago(t) {
		if (!t) return 'never';
		const s = Math.max(0, (Date.now() - new Date(t)) / 1000);
		if (s < 60) return `${Math.round(s)}s ago`;
		if (s < 3600) return `${Math.round(s / 60)}m ago`;
		return `${Math.round(s / 3600)}h ago`;
	}
</script>

{#snippet meters(st)}
	{#each [['CPU', st?.cpu], ['Memory', st?.mem], ['Disk', st?.disk]] as [label, v]}
		<div class="meter">
			<span class="mk">{label}</span>
			<div class="bar"><div class="fill {statusFor(v, warn, crit)}" style="width: {Math.min(100, v ?? 0)}%"></div></div>
			<span class="mv">{v === undefined || v === null ? '—' : `${Math.round(v)}%`}</span>
		</div>
	{/each}
{/snippet}

<section class="cluster">
	<div class="head">
		<div>
			<h2><Icon name="Connect" size={20} /> Cluster</h2>
			<p class="muted">
				{#if m.mode === 'primary'}
					This server is the <b>primary</b> · {online + 1} of {nodes.length + 1} servers online
				{:else if m.mode === 'node'}
					This server is a <b>{m.role}</b> of <b>{m.primaryName || m.primaryUrl}</b>
				{:else}
					This server is standalone. Add nodes to manage several servers from here.
				{/if}
			</p>
		</div>
		{#if isAdmin && m.mode !== 'node'}
			<button class="btn primary" onclick={() => ((adding = !adding), (token = null))}><Icon name="Upload" size={13} /> Add a server</button>
		{/if}
	</div>

	{#if error}<p class="error">{error}</p>{/if}

	{#if adding}
		<div class="panel add">
			<h3>Add a server</h3>
			<p class="muted">Create a single-use join token, then paste it into the new server's setup screen, or under <b>Cluster → Join a cluster</b> on an existing one.</p>
			<div class="roles">
				<button type="button" class="role" class:on={joinRole === 'node'} onclick={() => (joinRole = 'node')}>
					<Icon name="Desktop" size={18} />
					<b>Node</b>
					<span>Monitored and managed from here. Keeps its own users and settings.</span>
				</button>
				<button type="button" class="role" class:on={joinRole === 'replica'} onclick={() => (joinRole = 'replica')}>
					<Icon name="Copy" size={18} />
					<b>Replica</b>
					<span>Also mirrors this server's users and appearance, so the same logins work there. Its own users are replaced.</span>
				</button>
			</div>
			<div class="row">
				<label class="field grow">
					Address nodes use to reach this server
					<input bind:value={primaryUrl} placeholder="https://hq.example.com" />
				</label>
				<label class="field">
					Expires after
					<select bind:value={ttlHours}>
						<option value={1}>1 hour</option>
						<option value={24}>1 day</option>
						<option value={168}>7 days</option>
					</select>
				</label>
				<button class="btn primary" onclick={createToken}>Create token</button>
			</div>
			{#if token}
				<div class="token">
					<code>{token.token}</code>
					<button class="btn subtle" onclick={() => copy(token.token)}><Icon name="Copy" size={13} /> {copied ? 'Copied' : 'Copy'}</button>
				</div>
				<p class="muted small">Works once, until {new Date(token.expiresAt).toLocaleString()}. Treat it like a password.</p>
			{/if}
		</div>
	{/if}

	<div class="fleet">
		<div class="panel server self">
			<div class="srv-head">
				<span class="status-dot good"></span>
				<b>{cluster?.self?.hostname ?? 'This server'}</b>
				<span class="badge-pill accent">{m.mode === 'node' ? m.role : m.mode === 'primary' ? 'primary' : 'this server'}</span>
			</div>
			<p class="srv-sub">{cluster?.self?.platform ?? ''} · up {formatUptime(cluster?.self?.uptime)}</p>
			{@render meters(cluster?.self)}
			<div class="srv-foot">
				<span class="muted">{cluster?.self?.containersRunning ?? 0}/{cluster?.self?.containersTotal ?? 0} containers · {cluster?.self?.deployment}</span>
				<button class="btn subtle" onclick={() => onopen?.(null)}>Open</button>
			</div>
		</div>

		{#each nodes as n (n.id)}
			<div class="panel server" class:offline={!n.online}>
				<div class="srv-head">
					<span class="status-dot {n.online ? 'good' : 'critical'}"></span>
					<b>{n.name}</b>
					<span class="badge-pill {n.role === 'replica' ? 'info' : 'muted'}">{n.role}</span>
				</div>
				<p class="srv-sub">
					{n.status?.hostname ?? '—'} · {n.online ? `up ${formatUptime(n.status?.uptime)}` : `offline, last seen ${ago(n.lastSeen)}`}
				</p>
				{@render meters(n.online ? n.status : null)}
				<div class="srv-foot">
					<span class="muted">{n.status?.containersRunning ?? 0}/{n.status?.containersTotal ?? 0} containers{n.url ? '' : ' · no address'}</span>
					<span class="actions">
						{#if isAdmin}
							<button class="icon-btn" title="Edit" onclick={() => (editing = { id: n.id, name: n.name, url: n.url, role: n.role })}><Icon name="Edit" size={14} /></button>
							<button class="icon-btn" title="Remove" onclick={() => (removing = n)}><Icon name="X" size={14} /></button>
						{/if}
						<button class="btn subtle" disabled={!n.url || !n.online} title={n.url ? '' : 'Set an address to manage it from here'} onclick={() => onopen?.(n.id)}>Open</button>
					</span>
				</div>
			</div>
		{/each}
	</div>

	{#if isAdmin && m.mode === 'node'}
		<div class="panel">
			<h3>Membership</h3>
			<div class="kv">
				<span>Primary</span><span>{m.primaryName} <span class="muted">({m.primaryUrl})</span></span>
				<span>Role</span><span>{m.role}{m.role === 'replica' ? ': users and appearance are mirrored from the primary' : ''}</span>
				<span>Last heartbeat</span><span>{ago(cluster?.agent?.lastHeartbeat)}</span>
				{#if m.role === 'replica'}<span>Last sync</span><span>{ago(cluster?.agent?.lastSync)}</span>{/if}
			</div>
			{#if cluster?.agent?.lastError}<p class="error">{cluster.agent.lastError}</p>{/if}
			<form class="row" onsubmit={saveSelf}>
				<label class="field">Name shown on the primary<input bind:value={selfName} maxlength="60" /></label>
				<label class="field grow">Address the primary uses to reach this server<input bind:value={selfUrl} placeholder="https://edge-1.example.com" /></label>
				<button class="btn subtle" type="submit">Save</button>
			</form>
			<div><button class="btn danger" onclick={() => (leaving = true)}>Leave cluster</button></div>
		</div>
	{:else if isAdmin && nodes.length === 0}
		<details class="panel">
			<summary><b>Join a cluster</b> <span class="muted">· make this server a node of another ServerDash</span></summary>
			<form class="join" onsubmit={join}>
				<label class="field">Join token<input bind:value={joinToken} placeholder="sdj1.…" required /></label>
				<div class="row">
					<label class="field">Name for this server<input bind:value={selfName} maxlength="60" /></label>
					<label class="field grow">Address the primary uses to reach this server <span class="muted">(optional: without it, it only reports status)</span><input bind:value={selfUrl} /></label>
				</div>
				<button class="btn primary" type="submit" disabled={busy}>{busy ? 'Joining…' : 'Join'}</button>
			</form>
		</details>
	{/if}
</section>

{#if editing}
	<div class="modal-backdrop" role="presentation" onclick={() => (editing = null)}>
		<div class="modal panel" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && (editing = null)}>
		<form class="modal-form" onsubmit={saveNode}>
			<h3>Edit server</h3>
			<label class="field">Name<input bind:value={editing.name} maxlength="60" required /></label>
			<label class="field">Address<input bind:value={editing.url} placeholder="https://edge-1.example.com" /></label>
			<label class="field">
				Role
				<select bind:value={editing.role}>
					<option value="node">Node</option>
					<option value="replica">Replica (mirrors users &amp; appearance)</option>
				</select>
			</label>
			<div class="modal-actions">
				<button type="button" class="btn subtle" onclick={() => (editing = null)}>Cancel</button>
				<button type="submit" class="btn primary">Save</button>
			</div>
		</form>
		</div>
	</div>
{/if}

{#if removing || leaving}
	<div class="modal-backdrop" role="presentation" onclick={() => ((removing = null), (leaving = false))}>
		<div class="modal panel" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && ((removing = null), (leaving = false))}>
			{#if removing}
				<h3>Remove {removing.name}?</h3>
				<p class="muted">It stops being managed from here and its credentials are revoked. ServerDash keeps running on it; join it again later with a new token.</p>
			{:else}
				<h3>Leave the cluster?</h3>
				<p class="muted">This server becomes standalone. Users and settings it already has stay as they are.</p>
			{/if}
			<div class="modal-actions">
				<button class="btn subtle" onclick={() => ((removing = null), (leaving = false))}>Cancel</button>
				<button class="btn danger" onclick={removing ? removeNode : leave}>{removing ? 'Remove' : 'Leave'}</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.cluster {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.head {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 16px;
		flex-wrap: wrap;
	}
	h2 {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0 0 4px;
		font-size: 22px;
	}
	h3 {
		margin: 0 0 6px;
		font-size: 14px;
	}
	.head .btn,
	.token .btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}
	.muted {
		color: var(--muted);
		margin: 0;
	}
	.small {
		font-size: 11px;
	}
	.error {
		color: var(--danger);
	}
	.add,
	.join {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.roles {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
		gap: 10px;
	}
	.role {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 4px;
		text-align: left;
		padding: 14px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--sur2);
		color: var(--text);
	}
	.role span {
		color: var(--muted);
		font-size: 11px;
	}
	.role.on {
		border-color: var(--accent);
		background: var(--accent-soft);
	}
	.row {
		display: flex;
		gap: 12px;
		align-items: flex-end;
		flex-wrap: wrap;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 11px;
		color: var(--muted);
	}
	.grow {
		flex: 1;
		min-width: 240px;
	}
	.token {
		display: flex;
		gap: 8px;
		align-items: center;
	}
	.token code {
		flex: 1;
		padding: 10px 12px;
		border-radius: 10px;
		background: var(--bg);
		border: 1px solid var(--border-strong);
		color: var(--accent);
		word-break: break-all;
		font-size: 11px;
	}
	.fleet {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
		gap: 14px;
	}
	.server {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.server.self {
		border-color: var(--border-strong);
	}
	.server.offline {
		opacity: 0.7;
	}
	.srv-head {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 14px;
	}
	.srv-head .badge-pill {
		margin-left: auto;
	}
	.srv-sub {
		margin: 0 0 4px;
		font-size: 11px;
		color: var(--muted);
	}
	.meter {
		display: grid;
		grid-template-columns: 54px 1fr 38px;
		align-items: center;
		gap: 8px;
		font-size: 11px;
	}
	.mk {
		color: var(--muted);
	}
	.mv {
		text-align: right;
		font-variant-numeric: tabular-nums;
	}
	.bar {
		height: 6px;
		border-radius: 3px;
		background: var(--sur2);
		overflow: hidden;
	}
	.fill {
		height: 100%;
		background: var(--accent);
		border-radius: 3px;
		transition: width 0.4s ease;
	}
	.fill.warning {
		background: var(--warning);
	}
	.fill.critical {
		background: var(--danger);
	}
	.srv-foot {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 8px;
		margin-top: 4px;
		font-size: 11px;
	}
	.actions {
		display: flex;
		gap: 4px;
		align-items: center;
	}
	.srv-foot .btn {
		padding: 6px 12px;
	}
	.icon-btn {
		display: inline-flex;
		padding: 6px;
		border-radius: 8px;
		border: 1px solid transparent;
		background: none;
		color: var(--muted);
	}
	.icon-btn:hover {
		color: var(--text);
		border-color: var(--border);
	}
	.kv {
		display: grid;
		grid-template-columns: 140px 1fr;
		gap: 6px 12px;
		font-size: 12px;
		margin: 6px 0 12px;
	}
	.kv > span:nth-child(odd) {
		color: var(--muted);
	}
	details summary {
		cursor: pointer;
		margin-bottom: 4px;
	}
	details[open] summary {
		margin-bottom: 14px;
	}
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.55);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 50;
	}
	.modal {
		max-width: 440px;
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 10px;
		box-shadow: var(--card-shadow);
	}
	.modal-form {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 8px;
	}
</style>
