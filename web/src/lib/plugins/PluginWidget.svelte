<script>
	import { api } from '../api.js';
	import Icon from '../Icon.svelte';

	// One plugin widget: runs on the server, refreshed on the widget's interval.
	let { plugin, widget } = $props();
	let res = $state(null);
	let loading = $state(false);
	let error = $state(null);

	async function load(refresh = false) {
		loading = true;
		try {
			res = await api.pluginWidget(plugin.id, widget.id, refresh);
			error = null;
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}
	$effect(() => {
		load();
		const t = setInterval(load, Math.max(widget.interval || 60, 10) * 1000);
		return () => clearInterval(t);
	});

	let data = $derived(res?.data ?? null);
	let problem = $derived(error || res?.error || null);
</script>

<div class="widget panel" class:wide={widget.wide}>
	<header>
		<span class="icon-wrap"><Icon name={plugin.icon || 'Puzzle'} size={14} /></span>
		<b>{widget.title}</b>
		<span class="src">{plugin.name}</span>
		<button class="refresh" title="Refresh" disabled={loading} onclick={() => load(true)} class:spin={loading}>↻</button>
	</header>

	{#if problem}
		<p class="err">{problem}</p>
	{:else if !res}
		<p class="muted">Loading…</p>
	{:else if res.format === 'stats'}
		<div class="stats">
			{#each data?.items ?? [] as item, i (i)}
				<div class="stat {item.status ?? ''}">
					<span class="label">{item.label}</span>
					<span class="value">{item.value}</span>
					{#if item.hint}<span class="hint">{item.hint}</span>{/if}
				</div>
			{:else}
				<p class="muted">Nothing to show.</p>
			{/each}
		</div>
	{:else if res.format === 'table'}
		{#if data?.rows?.length}
			<div class="table-wrap">
				<table>
					<thead><tr>{#each data.columns ?? [] as c, i (i)}<th>{c}</th>{/each}</tr></thead>
					<tbody>
						{#each data.rows as row, r (r)}
							<tr class={data.status?.[r] ?? ''}>
								{#each row as cell, c (c)}<td>{#if c === 0 && data.status?.[r]}<span class="dot"></span>{/if}{cell}</td>{/each}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{:else}
			<p class="muted">Nothing to show yet. Check this plugin's settings.</p>
		{/if}
	{:else}
		<pre>{res.text || '(no output)'}</pre>
	{/if}

	{#if res?.updatedAt}<footer>Updated {new Date(res.updatedAt).toLocaleTimeString()}</footer>{/if}
</div>

<style>
	.widget {
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-width: 0;
	}
	.widget.wide {
		grid-column: 1 / -1;
	}
	header {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.icon-wrap {
		display: inline-flex;
		color: var(--accent);
	}
	.src {
		color: var(--muted);
		font-size: 11px;
	}
	.refresh {
		margin-left: auto;
		background: transparent;
		border: 0;
		color: var(--muted);
		font-size: 15px;
		line-height: 1;
	}
	.refresh:hover:not(:disabled) {
		color: var(--text);
	}
	.spin {
		animation: spin 1s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	.muted {
		color: var(--muted);
		margin: 0;
	}
	.err {
		margin: 0;
		color: var(--danger);
		white-space: pre-wrap;
		font-size: 12px;
	}
	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
		gap: 8px;
	}
	.stat {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 10px 12px;
		border-radius: 10px;
		background: var(--sur2);
		border: 1px solid var(--border);
	}
	.stat .label {
		color: var(--muted);
		font-size: 11px;
	}
	.stat .value {
		font-size: 17px;
		font-weight: 700;
		overflow-wrap: anywhere;
	}
	.stat .hint {
		color: var(--muted);
		font-size: 11px;
	}
	.stat.warn {
		border-color: var(--warning);
	}
	.stat.crit {
		border-color: var(--danger);
	}
	.table-wrap {
		overflow-x: auto;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 12px;
	}
	th {
		text-align: left;
		color: var(--muted);
		font-weight: 500;
		padding: 4px 8px;
		border-bottom: 1px solid var(--border);
	}
	td {
		padding: 6px 8px;
		border-bottom: 1px solid var(--border);
		overflow-wrap: anywhere;
	}
	tr:last-child td {
		border-bottom: 0;
	}
	.dot {
		display: inline-block;
		width: 7px;
		height: 7px;
		border-radius: 50%;
		margin-right: 7px;
		background: var(--success);
	}
	tr.warn .dot {
		background: var(--warning);
	}
	tr.crit .dot {
		background: var(--danger);
	}
	tr.crit td:not(:first-child) {
		color: var(--danger);
	}
	pre {
		margin: 0;
		max-height: 260px;
		overflow: auto;
		font-family: var(--font-mono);
		font-size: 11px;
		white-space: pre-wrap;
	}
	footer {
		color: var(--muted);
		font-size: 10px;
	}
</style>
