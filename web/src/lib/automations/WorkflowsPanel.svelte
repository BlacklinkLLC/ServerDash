<script>
	import { api } from '../api.js';
	import Icon from '../Icon.svelte';
	import WorkflowEditor from './WorkflowEditor.svelte';
	import RunLog from './RunLog.svelte';
	import { TEMPLATES, blankTrigger, triggerMeta } from './catalog.js';

	let workflows = $state([]);
	let lastRuns = $state({});
	let containers = $state([]);
	let scripts = $state([]);
	let error = $state(null);
	let loading = $state(true);
	let filter = $state('');

	let editing = $state(null); // workflow being edited/created
	let picking = $state(false); // template gallery open
	let historyFor = $state(null);
	let history = $state([]);
	let running = $state(new Set());
	let confirmDelete = $state(null);

	async function load() {
		try {
			const [wfs, runs] = await Promise.all([api.workflows(), api.recentWorkflowRuns(200)]);
			workflows = wfs;
			const latest = {};
			for (const r of runs) latest[r.workflowId] ??= r;
			lastRuns = latest;
			error = null;
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}
	load();
	api.containers()
		.then((l) => (containers = l.map((c) => c.name.replace(/^\//, ''))))
		.catch(() => {});
	api.scripts()
		.then((l) => (scripts = l))
		.catch(() => {});

	let visible = $derived(workflows.filter((w) => !filter.trim() || w.name.toLowerCase().includes(filter.trim().toLowerCase())));

	function normalize(wf) {
		return {
			...wf,
			trigger: { ...blankTrigger(wf.trigger?.type ?? 'daily'), ...(wf.trigger ?? {}) },
			onFailure: { format: 'discord', url: '', ...(wf.onFailure ?? {}) },
			blocks: wf.blocks ?? [],
		};
	}

	function fromTemplate(t) {
		picking = false;
		editing = normalize({ name: t?.name ?? '', enabled: true, trigger: t?.trigger ?? { type: 'daily' }, onFailure: t?.onFailure, blocks: t?.blocks ?? [] });
	}

	async function save(payload) {
		if (editing.id) await api.updateWorkflow(editing.id, payload);
		else await api.createWorkflow(payload);
		editing = null;
		await load();
	}

	async function toggleEnabled(wf) {
		try {
			await api.updateWorkflow(wf.id, { name: wf.name, enabled: !wf.enabled, trigger: wf.trigger, onFailure: wf.onFailure, blocks: wf.blocks });
			await load();
		} catch (e) {
			error = e.message;
		}
	}

	function duplicate(wf) {
		const { id, ...rest } = normalize(wf);
		editing = { ...rest, name: `${wf.name} (copy)`, trigger: { ...rest.trigger, token: undefined } };
	}

	async function runNow(wf) {
		running = new Set([...running, wf.id]);
		try {
			await api.runWorkflow(wf.id);
			await load();
			if (historyFor?.id === wf.id) history = await api.workflowRuns(wf.id);
		} catch (e) {
			error = e.message;
		} finally {
			running = new Set([...running].filter((x) => x !== wf.id));
		}
	}

	async function openHistory(wf) {
		historyFor = historyFor?.id === wf.id ? null : wf;
		if (historyFor) history = await api.workflowRuns(wf.id);
	}

	async function remove() {
		const wf = confirmDelete;
		confirmDelete = null;
		try {
			await api.deleteWorkflow(wf.id);
			await load();
		} catch (e) {
			error = e.message;
		}
	}

	function ago(t) {
		const s = (Date.now() - new Date(t)) / 1000;
		if (s < 60) return 'just now';
		if (s < 3600) return `${Math.round(s / 60)}m ago`;
		if (s < 86400) return `${Math.round(s / 3600)}h ago`;
		return `${Math.round(s / 86400)}d ago`;
	}
</script>

{#if editing}
	<div class="panel">
		<WorkflowEditor workflow={editing} {containers} {scripts} onsave={save} oncancel={() => (editing = null)} />
	</div>
{:else}
	<div class="toolbar">
		<input placeholder="Search workflows…" bind:value={filter} />
		<button class="btn primary" onclick={() => (picking = !picking)}>+ New workflow</button>
	</div>

	{#if error}<p class="error">{error}</p>{/if}

	{#if picking || (!loading && workflows.length === 0)}
		<div class="gallery">
			<button class="tpl blank" onclick={() => fromTemplate(null)}>
				<Icon name="Pencil" size={20} />
				<b>Start from scratch</b>
				<span>Pick a trigger and add steps yourself.</span>
			</button>
			{#each TEMPLATES as t (t.name)}
				<button class="tpl" onclick={() => fromTemplate(JSON.parse(JSON.stringify(t)))}>
					<Icon name={t.icon} size={20} />
					<b>{t.name}</b>
					<span>{t.description}</span>
				</button>
			{/each}
		</div>
	{/if}

	<div class="list">
		{#each visible as wf (wf.id)}
			{@const last = lastRuns[wf.id]}
			<div class="wf panel" class:disabled={!wf.enabled}>
				<div class="wf-main">
					<span class="ticon"><Icon name={triggerMeta(wf.trigger?.type).icon} size={18} /></span>
					<div class="wf-text">
						<b>{wf.name}</b>
						<span class="muted">{wf.triggerSummary} · {wf.blocks.length} step{wf.blocks.length === 1 ? '' : 's'}{wf.onFailure?.url ? ' · alerts on failure' : ''}</span>
					</div>
					<div class="wf-last">
						{#if running.has(wf.id)}
							<span class="badge-pill info">running…</span>
						{:else if last}
							<span class="badge-pill {last.success ? 'success' : last.success === false ? 'danger' : 'info'}">
								{last.success ? 'ok' : last.success === false ? 'failed' : 'running'}
							</span>
							<span class="muted small">{ago(last.startedAt)} · {last.triggeredBy}</span>
						{:else}
							<span class="muted small">never run</span>
						{/if}
					</div>
					<label class="toggle" title={wf.enabled ? 'Enabled' : 'Disabled'}>
						<input type="checkbox" checked={wf.enabled} onchange={() => toggleEnabled(wf)} />
						<span></span>
					</label>
				</div>
				<div class="wf-actions">
					<button class="btn subtle" disabled={running.has(wf.id)} onclick={() => runNow(wf)}><Icon name="Play" size={12} /> Run now</button>
					<button class="btn subtle" onclick={() => openHistory(wf)}><Icon name="Clock" size={12} /> History</button>
					<button class="btn subtle" onclick={() => (editing = normalize(wf))}><Icon name="Edit" size={12} /> Edit</button>
					<button class="btn subtle" onclick={() => duplicate(wf)}><Icon name="Copy" size={12} /> Duplicate</button>
					{#if wf.trigger?.type === 'webhook' && wf.trigger.token}
						<button class="btn subtle" onclick={() => navigator.clipboard.writeText(api.workflowHookUrl(wf.trigger.token))}><Icon name="Connect" size={12} /> Copy webhook URL</button>
					{/if}
					<button class="btn danger" onclick={() => (confirmDelete = wf)}>Delete</button>
				</div>
				{#if historyFor?.id === wf.id}
					<div class="history">
						{#each history as run (run.id)}
							<RunLog {run} />
						{:else}
							<p class="muted">No runs yet.</p>
						{/each}
					</div>
				{/if}
			</div>
		{/each}
	</div>
{/if}

{#if confirmDelete}
	<div class="modal-backdrop" role="presentation" onclick={() => (confirmDelete = null)}>
		<div class="modal panel" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && (confirmDelete = null)}>
			<h3>Delete “{confirmDelete.name}”?</h3>
			<p class="muted">Its run history is deleted too.{confirmDelete.trigger?.type === 'webhook' ? ' Its webhook URL stops working.' : ''}</p>
			<div class="modal-actions">
				<button class="btn subtle" onclick={() => (confirmDelete = null)}>Cancel</button>
				<button class="btn danger" onclick={remove}>Delete</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.toolbar {
		display: flex;
		gap: 8px;
		margin-bottom: 14px;
	}
	.toolbar input {
		flex: 1;
	}
	.error {
		color: var(--danger);
	}
	.muted {
		color: var(--muted);
	}
	.small {
		font-size: 11px;
	}
	.gallery {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
		gap: 10px;
		margin-bottom: 18px;
	}
	.tpl {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 6px;
		padding: 14px;
		border-radius: 14px;
		border: 1px solid var(--border);
		background: var(--surface);
		color: var(--text);
		text-align: left;
		transition:
			border-color 0.15s ease,
			transform 0.15s ease;
	}
	.tpl:hover {
		border-color: var(--border-strong);
		transform: translateY(-1px);
	}
	.tpl :global(.icon) {
		color: var(--accent);
	}
	.tpl span {
		color: var(--muted);
		font-size: 11px;
	}
	.tpl.blank {
		border-style: dashed;
	}
	.list {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.wf {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.wf.disabled {
		opacity: 0.6;
	}
	.wf-main {
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.ticon {
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
	.wf-text {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.wf-text .muted {
		font-size: 11px;
	}
	.wf-last {
		display: flex;
		align-items: center;
		gap: 8px;
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
	.wf-actions {
		display: flex;
		gap: 6px;
		flex-wrap: wrap;
		padding-left: 48px;
	}
	.wf-actions .btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		font-size: 11px;
	}
	.history {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding-left: 48px;
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
