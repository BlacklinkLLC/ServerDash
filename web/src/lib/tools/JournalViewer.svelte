<script>
	import { onDestroy } from 'svelte';
	import { api } from '../api.js';
	import Icon from '../Icon.svelte';

	// unit = '' shows the whole system journal.
	let { unit = '' } = $props();

	const priorities = [
		{ v: '', label: 'All levels' },
		{ v: '3', label: 'Errors & worse' },
		{ v: '4', label: 'Warnings & worse' },
		{ v: '6', label: 'Info & worse' },
	];
	const levelName = ['emerg', 'alert', 'crit', 'err', 'warn', 'notice', 'info', 'debug'];

	let entries = $state([]);
	let priority = $state('');
	let grep = $state('');
	let lines = $state(300);
	let follow = $state(true);
	let error = $state(null);
	let loading = $state(false);
	let box = $state(null);

	async function load() {
		loading = true;
		try {
			entries = await api.journal({ unit, priority, grep, lines });
			error = null;
			if (follow && box) requestAnimationFrame(() => (box.scrollTop = box.scrollHeight));
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	let timer;
	$effect(() => {
		unit;
		priority;
		lines;
		load();
		clearInterval(timer);
		if (follow) timer = setInterval(load, 5000);
		return () => clearInterval(timer);
	});
	onDestroy(() => clearInterval(timer));

	function onSearch(e) {
		e.preventDefault();
		load();
	}

	function download() {
		const text = entries.map((e) => `${e.time} ${e.unit} [${levelName[e.priority] ?? e.priority}] ${e.message}`).join('\n');
		const a = document.createElement('a');
		a.href = URL.createObjectURL(new Blob([text], { type: 'text/plain' }));
		a.download = `${unit || 'system'}-journal.log`;
		a.click();
		URL.revokeObjectURL(a.href);
	}

	function time(t) {
		return t ? new Date(t).toLocaleString([], { month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '';
	}
</script>

<form class="toolbar" onsubmit={onSearch}>
	<input placeholder="Search messages (regex)…" bind:value={grep} />
	<select bind:value={priority}>
		{#each priorities as p (p.v)}<option value={p.v}>{p.label}</option>{/each}
	</select>
	<select bind:value={lines}>
		<option value={100}>Last 100</option>
		<option value={300}>Last 300</option>
		<option value={1000}>Last 1000</option>
		<option value={2000}>Last 2000</option>
	</select>
	<label class="follow"><input type="checkbox" bind:checked={follow} /> Live</label>
	<button class="btn subtle" type="submit"><Icon name="Search" size={13} /> Search</button>
	<button class="btn subtle" type="button" onclick={download} disabled={!entries.length}><Icon name="Download" size={13} /> Save</button>
</form>

{#if error}<p class="error">{error}</p>{/if}

<div class="log" bind:this={box}>
	{#each entries as e, i (i)}
		<div class="line p{e.priority}">
			<span class="t">{time(e.time)}</span>
			{#if !unit}<span class="u">{e.unit}</span>{/if}
			<span class="lvl">{levelName[e.priority] ?? e.priority}</span>
			<span class="m">{e.message}</span>
		</div>
	{:else}
		<div class="empty">{loading ? 'Loading…' : 'No log entries match.'}</div>
	{/each}
</div>

<style>
	.toolbar {
		display: flex;
		gap: 8px;
		align-items: center;
		margin-bottom: 10px;
		flex-wrap: wrap;
	}
	.toolbar input:not([type='checkbox']) {
		flex: 1;
		min-width: 200px;
	}
	.toolbar .btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}
	.follow {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
	}
	.error {
		color: var(--danger);
	}
	.log {
		height: 60vh;
		overflow: auto;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 8px 0;
		font-family: var(--font-mono);
		font-size: 11.5px;
		line-height: 1.55;
	}
	.line {
		display: flex;
		gap: 10px;
		padding: 1px 12px;
		white-space: pre-wrap;
		word-break: break-word;
	}
	.line:hover {
		background: var(--sur2);
	}
	.t {
		color: var(--muted);
		flex-shrink: 0;
	}
	.u {
		color: var(--blue);
		flex-shrink: 0;
		max-width: 200px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.lvl {
		flex-shrink: 0;
		width: 46px;
		color: var(--muted);
		text-transform: uppercase;
		font-size: 10px;
		padding-top: 1px;
	}
	.p0 .lvl,
	.p1 .lvl,
	.p2 .lvl,
	.p3 .lvl {
		color: var(--danger);
	}
	.p0 .m,
	.p1 .m,
	.p2 .m,
	.p3 .m {
		color: var(--danger);
	}
	.p4 .lvl {
		color: var(--warning);
	}
	.empty {
		padding: 20px;
		text-align: center;
		color: var(--muted);
	}
</style>
