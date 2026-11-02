<script>
	import { api } from '../api.js';
	import Icon from '../Icon.svelte';
	import JournalViewer from './JournalViewer.svelte';

	let { isAdmin } = $props();

	let units = $state([]);
	let loading = $state(true);
	let error = $state(null);
	let flash = $state(null);
	let filter = $state('');
	let show = $state('loaded'); // 'loaded' | 'running' | 'failed' | 'all'
	let pending = $state(new Set());
	let logsFor = $state(null);
	let confirming = $state(null); // { name, action }

	async function load() {
		try {
			units = await api.services();
			error = null;
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}
	load();

	let counts = $derived({
		running: units.filter((u) => u.sub === 'running').length,
		failed: units.filter((u) => u.active === 'failed').length,
		loaded: units.filter((u) => u.load === 'loaded').length,
	});

	let visible = $derived.by(() => {
		const q = filter.trim().toLowerCase();
		return units.filter((u) => {
			if (show === 'loaded' && u.load !== 'loaded') return false;
			if (show === 'running' && u.sub !== 'running') return false;
			if (show === 'failed' && u.active !== 'failed') return false;
			return !q || u.name.toLowerCase().includes(q) || u.description.toLowerCase().includes(q);
		});
	});

	function stateClass(u) {
		if (u.active === 'failed') return 'danger';
		if (u.active === 'active') return 'success';
		if (u.active === 'activating' || u.active === 'reloading') return 'info';
		return 'muted';
	}

	// Stopping/disabling can take a host service offline, so those ask first.
	function request(name, action) {
		if (action === 'stop' || action === 'disable') confirming = { name, action };
		else act(name, action);
	}

	async function act(name, action) {
		confirming = null;
		pending = new Set([...pending, name]);
		try {
			await api.serviceAction(name, action);
			flash = `${action[0].toUpperCase()}${action.slice(1)} ${name}: done`;
			setTimeout(() => (flash = null), 4000);
			await load();
		} catch (e) {
			error = e.message;
		} finally {
			pending = new Set([...pending].filter((p) => p !== name));
		}
	}
</script>

<div class="summary">
	<button class="chip" class:on={show === 'running'} onclick={() => (show = 'running')}>
		<span class="status-dot good"></span><b>{counts.running}</b> running
	</button>
	<button class="chip" class:on={show === 'failed'} onclick={() => (show = 'failed')}>
		<span class="status-dot {counts.failed ? 'critical' : ''}"></span><b>{counts.failed}</b> failed
	</button>
	<button class="chip" class:on={show === 'loaded'} onclick={() => (show = 'loaded')}><b>{counts.loaded}</b> loaded</button>
	<button class="chip" class:on={show === 'all'} onclick={() => (show = 'all')}><b>{units.length}</b> installed</button>
</div>

<div class="toolbar">
	<input placeholder="Filter services…" bind:value={filter} />
	<button class="btn subtle" onclick={load}><Icon name="Refresh" size={13} /> Refresh</button>
	<span class="muted count">{visible.length} shown</span>
</div>

{#if error}<p class="error">{error}</p>{/if}
{#if flash}<p class="flash">{flash}</p>{/if}

{#if loading}
	<p class="muted">Loading services…</p>
{:else}
	<table class="token-table">
		<thead>
			<tr><th>Service</th><th>State</th><th>Boot</th><th></th></tr>
		</thead>
		<tbody>
			{#each visible as u (u.name)}
				<tr>
					<td>
						<span class="uname">{u.name.replace(/\.service$/, '')}</span>
						{#if u.description}<span class="desc">{u.description}</span>{/if}
					</td>
					<td><span class="badge-pill {stateClass(u)}">{u.active} · {u.sub}</span></td>
					<td class="muted">{u.enabled || '—'}</td>
					<td class="actions">
						<button class="btn subtle" onclick={() => (logsFor = u.name)}>Logs</button>
						{#if isAdmin}
							{#if u.active === 'active'}
								<button class="btn subtle" disabled={pending.has(u.name)} onclick={() => request(u.name, 'restart')}>Restart</button>
								<button class="btn subtle" disabled={pending.has(u.name)} onclick={() => request(u.name, 'stop')}>Stop</button>
							{:else}
								<button class="btn subtle" disabled={pending.has(u.name)} onclick={() => request(u.name, 'start')}>Start</button>
							{/if}
							{#if u.enabled === 'enabled'}
								<button class="btn subtle" disabled={pending.has(u.name)} onclick={() => request(u.name, 'disable')}>Disable</button>
							{:else if u.enabled === 'disabled'}
								<button class="btn subtle" disabled={pending.has(u.name)} onclick={() => request(u.name, 'enable')}>Enable</button>
							{/if}
						{/if}
					</td>
				</tr>
			{:else}
				<tr><td colspan="4" class="muted">No services match.</td></tr>
			{/each}
		</tbody>
	</table>
{/if}

{#if logsFor}
	<div class="modal-backdrop" role="presentation" onclick={() => (logsFor = null)}>
		<div class="modal wide panel" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && (logsFor = null)}>
			<div class="modal-head">
				<h3><Icon name="Document" size={16} /> {logsFor}</h3>
				<button class="btn subtle" onclick={() => (logsFor = null)}>Close</button>
			</div>
			<JournalViewer unit={logsFor} />
		</div>
	</div>
{/if}

{#if confirming}
	<div class="modal-backdrop" role="presentation" onclick={() => (confirming = null)}>
		<div class="modal panel" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && (confirming = null)}>
			<h3>{confirming.action === 'stop' ? 'Stop' : 'Disable'} {confirming.name}?</h3>
			<p class="muted">
				{confirming.action === 'stop'
					? 'Anything depending on this service will stop working until it is started again.'
					: 'It will no longer start automatically when the server boots.'}
			</p>
			<div class="modal-actions">
				<button class="btn subtle" onclick={() => (confirming = null)}>Cancel</button>
				<button class="btn danger" onclick={() => act(confirming.name, confirming.action)}>
					{confirming.action === 'stop' ? 'Stop service' : 'Disable service'}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.summary {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
		margin-bottom: 12px;
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 8px 12px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--sur2);
		color: var(--muted-strong);
		font-size: 12px;
	}
	.chip.on {
		border-color: var(--accent);
		color: var(--text);
	}
	.chip b {
		font-family: var(--font-display);
		color: var(--text);
	}
	.toolbar {
		display: flex;
		gap: 8px;
		align-items: center;
		margin-bottom: 12px;
		flex-wrap: wrap;
	}
	.toolbar input {
		flex: 1;
		min-width: 220px;
	}
	.toolbar .btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}
	.count {
		font-size: 11px;
	}
	.muted {
		color: var(--muted);
	}
	.error {
		color: var(--danger);
	}
	.flash {
		color: var(--success);
	}
	.uname {
		display: block;
		font-weight: 500;
	}
	.desc {
		display: block;
		color: var(--muted);
		font-size: 11px;
	}
	.actions {
		display: flex;
		gap: 6px;
		justify-content: flex-end;
		flex-wrap: wrap;
	}
	.actions .btn {
		padding: 6px 10px;
		font-size: 11px;
	}
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.55);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 50;
		padding: 24px;
	}
	.modal {
		max-width: 420px;
		width: 100%;
		box-shadow: var(--card-shadow);
	}
	.modal.wide {
		max-width: 1000px;
	}
	.modal h3 {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 0 0 8px;
	}
	.modal-head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 10px;
	}
	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 16px;
	}
</style>
