<script>
	import { onDestroy } from 'svelte';
	import { api } from './api.js';
	import { formatBytes } from './format.js';
	import Icon from './Icon.svelte';
	import ServicesPanel from './tools/ServicesPanel.svelte';
	import JournalViewer from './tools/JournalViewer.svelte';

	let { isAdmin, refreshSeconds = 5, host = null } = $props();

	let tab = $state('processes');
	let error = $state(null);

	// --- Processes ---
	let processes = $state([]);
	let sortBy = $state('cpu');
	let filter = $state('');
	let paused = $state(false);
	let confirming = $state(null); // { pid, name, signal }
	let flash = $state(null);

	// --- Ports / interfaces ---
	let ports = $state([]);
	let portFilter = $state('');
	let interfaces = $state([]);

	let filteredProcs = $derived.by(() => {
		const q = filter.trim().toLowerCase();
		const list = q
			? processes.filter((p) => p.name.toLowerCase().includes(q) || p.command.toLowerCase().includes(q) || String(p.pid) === q || p.user.toLowerCase().includes(q))
			: processes;
		return list.slice(0, 200);
	});

	let filteredPorts = $derived.by(() => {
		const q = portFilter.trim().toLowerCase();
		if (!q) return ports;
		return ports.filter((p) => String(p.port).includes(q) || (p.process ?? '').toLowerCase().includes(q) || p.address.includes(q));
	});

	async function load() {
		try {
			if (tab === 'processes' && !paused) processes = await api.processes(sortBy);
			if (tab === 'ports') ports = await api.ports();
			if (tab === 'network') interfaces = await api.interfaces();
			error = null;
		} catch (e) {
			error = e.message;
		}
	}

	let timer;
	$effect(() => {
		tab;
		sortBy;
		load();
		clearInterval(timer);
		timer = setInterval(load, refreshSeconds * 1000);
		return () => clearInterval(timer);
	});
	onDestroy(() => clearInterval(timer));

	async function sendSignal() {
		const { pid, name, signal } = confirming;
		confirming = null;
		try {
			await api.signalProcess(pid, signal);
			flash = `Sent SIG${signal} to ${name} (${pid})`;
			setTimeout(() => (flash = null), 4000);
			load();
		} catch (e) {
			error = e.message;
		}
	}

	function since(ms) {
		if (!ms) return '—';
		const s = (Date.now() - ms) / 1000;
		if (s < 3600) return `${Math.floor(s / 60)}m`;
		if (s < 86400) return `${Math.floor(s / 3600)}h`;
		return `${Math.floor(s / 86400)}d`;
	}
</script>

<section>
	<h2><Icon name="Settings" size={20} /> Server tools</h2>
	<nav class="tabs">
		<button class:active={tab === 'processes'} onclick={() => (tab = 'processes')}><Icon name="Script" size={14} /> Processes</button>
		<button class:active={tab === 'ports'} onclick={() => (tab = 'ports')}><Icon name="Connect" size={14} /> Listening ports</button>
		<button class:active={tab === 'network'} onclick={() => (tab = 'network')}><Icon name="Ethernet" size={14} /> Network interfaces</button>
		<button class:active={tab === 'services'} onclick={() => (tab = 'services')}><Icon name="Blocks" size={14} /> Services</button>
		<button class:active={tab === 'logs'} onclick={() => (tab = 'logs')}><Icon name="Document" size={14} /> System logs</button>
	</nav>

	{#if error}<p class="error">{error}</p>{/if}
	{#if flash}<p class="flash">{flash}</p>{/if}

	{#if tab === 'processes'}
		<div class="toolbar">
			<input placeholder="Filter by name, command, user, or PID…" bind:value={filter} />
			<select bind:value={sortBy}>
				<option value="cpu">Sort by CPU</option>
				<option value="mem">Sort by memory</option>
			</select>
			<button class="btn subtle" onclick={() => (paused = !paused)}>{paused ? 'Resume' : 'Pause'} live updates</button>
			<span class="muted count">{filteredProcs.length} of {processes.length}</span>
		</div>
		<table class="token-table">
			<thead>
				<tr>
					<th>PID</th>
					<th>Name</th>
					<th>User</th>
					<th class="num">CPU</th>
					<th class="num">Mem</th>
					<th class="num">RSS</th>
					<th class="num">Threads</th>
					<th class="num">Age</th>
					{#if isAdmin}<th></th>{/if}
				</tr>
			</thead>
			<tbody>
				{#each filteredProcs as p (p.pid)}
					<tr>
						<td class="muted">{p.pid}</td>
						<td title={p.command}><span class="pname">{p.name}</span><span class="cmd">{p.command}</span></td>
						<td class="muted">{p.user || '—'}</td>
						<td class="num">{p.cpu.toFixed(1)}%</td>
						<td class="num">{p.mem.toFixed(1)}%</td>
						<td class="num muted">{formatBytes(p.rss)}</td>
						<td class="num muted">{p.threads}</td>
						<td class="num muted">{since(p.startedMs)}</td>
						{#if isAdmin}
							<td class="actions">
								<button class="btn subtle" onclick={() => (confirming = { pid: p.pid, name: p.name, signal: 'TERM' })}>Stop</button>
								<button class="btn danger" onclick={() => (confirming = { pid: p.pid, name: p.name, signal: 'KILL' })}>Kill</button>
							</td>
						{/if}
					</tr>
				{/each}
			</tbody>
		</table>
	{:else if tab === 'ports'}
		<div class="toolbar">
			<input placeholder="Filter by port, address, or process…" bind:value={portFilter} />
			<span class="muted count">{filteredPorts.length} sockets</span>
		</div>
		<table class="token-table">
			<thead>
				<tr><th>Port</th><th>Protocol</th><th>Address</th><th>Process</th><th>PID</th></tr>
			</thead>
			<tbody>
				{#each filteredPorts as p (`${p.protocol}${p.address}${p.port}`)}
					<tr>
						<td><b>{p.port}</b></td>
						<td><span class="badge-pill {p.protocol.startsWith('tcp') ? 'info' : 'muted'}">{p.protocol}</span></td>
						<td class="muted">{p.address === '0.0.0.0' || p.address === '::' ? `${p.address} (all)` : p.address}</td>
						<td>{p.process || '—'}</td>
						<td class="muted">{p.pid || '—'}</td>
					</tr>
				{:else}
					<tr><td colspan="5" class="muted">No listening sockets found.</td></tr>
				{/each}
			</tbody>
		</table>
	{:else if tab === 'network'}
		<div class="ifaces">
			{#each interfaces as i (i.name)}
				<div class="panel">
					<div class="panel-title">
						<span><span class="status-dot {i.up ? 'good' : ''}"></span> {i.name}</span>
						<span>{i.up ? 'Up' : 'Down'} · MTU {i.mtu}</span>
					</div>
					{#if i.mac}<div class="row"><span class="muted">MAC</span><span>{i.mac}</span></div>{/if}
					{#each i.addresses ?? [] as a}
						<div class="row"><span class="muted">Address</span><span>{a}</span></div>
					{/each}
					<div class="row"><span class="muted">Received</span><span>{formatBytes(i.rxBytes)}</span></div>
					<div class="row"><span class="muted">Sent</span><span>{formatBytes(i.txBytes)}</span></div>
					{#if i.rxErrors || i.txErrors}
						<div class="row"><span class="muted">Errors</span><span class="warn">{i.rxErrors} rx · {i.txErrors} tx</span></div>
					{/if}
				</div>
			{/each}
		</div>
	{:else if tab === 'services' || tab === 'logs'}
		{#if host && !host.systemd}
			<div class="panel note">
				<Icon name="Information" size={18} />
				<div>
					<b>Needs a native install</b>
					<p class="muted">
						Managing host services and reading the system journal needs ServerDash running directly on a systemd host. This one is
						running {host.deployment === 'container' ? 'in a container' : 'without systemd'}. Reinstall with
						<code>install.sh --runtime native</code> to turn these on.
					</p>
				</div>
			</div>
		{:else if tab === 'services'}
			<ServicesPanel {isAdmin} />
		{:else}
			<JournalViewer />
		{/if}
	{/if}
</section>

{#if confirming}
	<div class="modal-backdrop" role="presentation" onclick={() => (confirming = null)}>
		<div class="modal panel" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && (confirming = null)}>
			<h3>{confirming.signal === 'KILL' ? 'Force kill' : 'Stop'} {confirming.name}?</h3>
			<p class="muted">
				Sends SIG{confirming.signal} to PID {confirming.pid}.
				{confirming.signal === 'KILL' ? 'The process is terminated immediately and cannot clean up.' : 'The process is asked to shut down gracefully.'}
			</p>
			<div class="modal-actions">
				<button class="btn subtle" onclick={() => (confirming = null)}>Cancel</button>
				<button class="btn danger" onclick={sendSignal}>Send SIG{confirming.signal}</button>
			</div>
		</div>
	</div>
{/if}

<style>
	h2 {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0 0 14px;
		font-size: 22px;
	}
	.tabs {
		display: flex;
		gap: 18px;
		margin-bottom: 16px;
		border-bottom: 1px solid var(--border);
	}
	.tabs button {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		padding: 8px 4px;
		font-size: 12px;
		color: var(--muted);
	}
	.tabs button.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
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
	.num {
		text-align: right;
		font-variant-numeric: tabular-nums;
	}
	.pname {
		display: block;
		font-weight: 500;
	}
	.cmd {
		display: block;
		max-width: 340px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--muted);
		font-size: 11px;
	}
	.actions {
		display: flex;
		gap: 6px;
		justify-content: flex-end;
	}
	.actions .btn {
		padding: 6px 10px;
		font-size: 11px;
	}
	.note {
		display: flex;
		gap: 12px;
		align-items: flex-start;
	}
	.note p {
		margin: 4px 0 0;
	}
	.ifaces {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
		gap: 14px;
	}
	.row {
		display: flex;
		justify-content: space-between;
		gap: 10px;
		padding: 5px 0;
		font-size: 12px;
		border-bottom: 1px solid var(--border);
		word-break: break-all;
	}
	.row:last-child {
		border-bottom: none;
	}
	.warn {
		color: var(--warning);
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
		max-width: 420px;
		box-shadow: var(--card-shadow);
	}
	.modal h3 {
		margin: 0 0 8px;
	}
	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 16px;
	}
</style>
