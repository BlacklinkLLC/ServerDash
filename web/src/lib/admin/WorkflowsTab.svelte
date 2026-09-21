<script>
	import { api } from '../api.js';

	const TIMEZONES = [
		'UTC',
		'America/New_York',
		'America/Chicago',
		'America/Denver',
		'America/Los_Angeles',
		'America/Anchorage',
		'Europe/London',
		'Europe/Berlin',
		'Asia/Tokyo',
		'Asia/Shanghai',
		'Australia/Sydney',
	];

	const BLOCK_TYPES = [
		{ type: 'git_pull', label: 'Git Pull' },
		{ type: 'compose_up', label: 'Rebuild & Run (docker compose up)' },
		{ type: 'restart_container', label: 'Restart Container' },
		{ type: 'shell', label: 'Run Command' },
		{ type: 'wait', label: 'Wait' },
	];
	const blockLabel = (type) => BLOCK_TYPES.find((b) => b.type === type)?.label ?? type;

	function blankBlock(type) {
		switch (type) {
			case 'git_pull':
				return { type, dir: '' };
			case 'compose_up':
				return { type, dir: '', build: true };
			case 'restart_container':
				return { type, container: '' };
			case 'shell':
				return { type, command: '', dir: '' };
			case 'wait':
				return { type, seconds: 30 };
			default:
				return { type };
		}
	}

	let workflows = $state([]);
	let containerNames = $state([]);
	let error = $state(null);
	let editing = $state(null); // null | workflow-shaped object being edited/created
	let addBlockType = $state('git_pull');
	let expandedRuns = $state(null);
	let runs = $state([]);
	let running = $state(new Set());

	async function load() {
		try {
			workflows = await api.workflows();
			error = null;
		} catch (e) {
			error = e.message;
		}
	}
	load();

	api.containers()
		.then((list) => (containerNames = list.map((c) => c.name)))
		.catch(() => {});

	function startNew() {
		editing = {
			name: '',
			triggerTime: '00:00',
			triggerTz: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
			enabled: true,
			blocks: [],
		};
	}

	function startEdit(wf) {
		editing = { ...wf, blocks: JSON.parse(JSON.stringify(wf.blocks)) };
	}

	function addBlock() {
		editing.blocks = [...editing.blocks, blankBlock(addBlockType)];
	}
	function removeBlock(i) {
		editing.blocks = editing.blocks.filter((_, idx) => idx !== i);
	}
	function moveBlock(i, dir) {
		const j = i + dir;
		if (j < 0 || j >= editing.blocks.length) return;
		const blocks = [...editing.blocks];
		[blocks[i], blocks[j]] = [blocks[j], blocks[i]];
		editing.blocks = blocks;
	}

	async function save(e) {
		e.preventDefault();
		try {
			const payload = { ...editing, blocks: editing.blocks };
			if (editing.id) {
				await api.updateWorkflow(editing.id, payload);
			} else {
				await api.createWorkflow(payload);
			}
			editing = null;
			await load();
		} catch (e) {
			error = e.message;
		}
	}

	async function remove(wf) {
		if (!confirm(`Delete workflow "${wf.name}"?`)) return;
		try {
			await api.deleteWorkflow(wf.id);
			await load();
		} catch (e) {
			error = e.message;
		}
	}

	async function run(wf) {
		running = new Set([...running, wf.id]);
		try {
			await api.runWorkflow(wf.id);
			if (expandedRuns === wf.id) await showRuns(wf);
		} catch (e) {
			error = e.message;
		} finally {
			running = new Set([...running].filter((id) => id !== wf.id));
		}
	}

	async function showRuns(wf) {
		expandedRuns = expandedRuns === wf.id ? null : wf.id;
		if (expandedRuns) runs = await api.workflowRuns(wf.id);
	}
</script>

<div class="tab">
	<p class="intro">
		Scheduled, block-based deployment automations — e.g. "at midnight, pull the latest code and rebuild the container."
		Each block is a single typed step; blocks run top to bottom and stop at the first failure.
	</p>

	{#if error}
		<p class="error">{error}</p>
	{/if}

	<table class="token-table">
		<thead>
			<tr>
				<th>Name</th>
				<th>Trigger</th>
				<th>Blocks</th>
				<th>Enabled</th>
				<th></th>
			</tr>
		</thead>
		<tbody>
			{#each workflows as wf (wf.id)}
				<tr>
					<td>{wf.name}</td>
					<td class="muted">At {wf.triggerTime} {wf.triggerTz}</td>
					<td class="muted">{wf.blocks.length}</td>
					<td class="muted">{wf.enabled ? 'yes' : 'no'}</td>
					<td class="actions">
						<button class="btn subtle" disabled={running.has(wf.id)} onclick={() => run(wf)}>
							{running.has(wf.id) ? 'Running…' : 'Run now'}
						</button>
						<button class="btn subtle" onclick={() => showRuns(wf)}>History</button>
						<button class="btn subtle" onclick={() => startEdit(wf)}>Edit</button>
						<button class="btn danger" onclick={() => remove(wf)}>Delete</button>
					</td>
				</tr>
				{#if expandedRuns === wf.id}
					<tr>
						<td colspan="5">
							{#if runs.length === 0}
								<p class="muted">No runs yet.</p>
							{:else}
								<ul class="runs">
									{#each runs as run (run.id)}
										<li>
											<div class="run-meta">
												<span class={run.success ? 'ok' : 'fail'}>{run.success ? 'success' : 'failed'}</span>
												<span class="muted">{new Date(run.startedAt).toLocaleString()} · {run.triggeredBy}</span>
											</div>
											<pre>{run.log}</pre>
										</li>
									{/each}
								</ul>
							{/if}
						</td>
					</tr>
				{/if}
			{/each}
		</tbody>
	</table>

	{#if editing}
		<form class="editor" onsubmit={save}>
			<h3>{editing.id ? 'Edit workflow' : 'New workflow'}</h3>
			<label>
				Name
				<input bind:value={editing.name} required placeholder="Nightly rebuild" />
			</label>

			<div class="trigger-block">
				<span class="block-kind">TRIGGER</span>
				<span>At time</span>
				<input type="time" bind:value={editing.triggerTime} required />
				<input list="tz-options" bind:value={editing.triggerTz} placeholder="America/Chicago" required />
				<datalist id="tz-options">
					{#each TIMEZONES as tz (tz)}
						<option value={tz}></option>
					{/each}
				</datalist>
			</div>

			<div class="blocks">
				{#each editing.blocks as block, i (i)}
					<div class="block-card">
						<div class="block-head">
							<span class="block-kind">{blockLabel(block.type)}</span>
							<div class="block-controls">
								<button type="button" class="btn subtle" onclick={() => moveBlock(i, -1)} disabled={i === 0} aria-label="Move up">↑</button>
								<button
									type="button"
									class="btn subtle"
									onclick={() => moveBlock(i, 1)}
									disabled={i === editing.blocks.length - 1}
									aria-label="Move down">↓</button
								>
								<button type="button" class="btn danger" onclick={() => removeBlock(i)} aria-label="Remove block">✕</button>
							</div>
						</div>

						{#if block.type === 'git_pull'}
							<label>
								Working directory
								<input bind:value={block.dir} placeholder="/opt/myapp" required />
							</label>
						{:else if block.type === 'compose_up'}
							<label>
								Working directory
								<input bind:value={block.dir} placeholder="/opt/myapp" required />
							</label>
							<label class="checkbox">
								<input type="checkbox" bind:checked={block.build} />
								Rebuild image (--build)
							</label>
						{:else if block.type === 'restart_container'}
							<label>
								Container name or ID
								<input bind:value={block.container} list="container-options" required />
								<datalist id="container-options">
									{#each containerNames as name (name)}
										<option value={name}></option>
									{/each}
								</datalist>
							</label>
						{:else if block.type === 'shell'}
							<label>
								Command
								<textarea bind:value={block.command} rows="2" class="mono" required></textarea>
							</label>
							<label>
								Working directory (optional)
								<input bind:value={block.dir} placeholder="defaults to ServerDash's own directory" />
							</label>
						{:else if block.type === 'wait'}
							<label>
								Seconds
								<input type="number" bind:value={block.seconds} min="1" required />
							</label>
						{/if}
					</div>
				{/each}

				<div class="add-block">
					<select bind:value={addBlockType}>
						{#each BLOCK_TYPES as bt (bt.type)}
							<option value={bt.type}>{bt.label}</option>
						{/each}
					</select>
					<button type="button" class="btn subtle" onclick={addBlock}>+ Add block</button>
				</div>
			</div>

			<label class="checkbox">
				<input type="checkbox" bind:checked={editing.enabled} />
				Enabled
			</label>

			<div class="row">
				<button class="btn primary" type="submit">Save</button>
				<button class="btn subtle" type="button" onclick={() => (editing = null)}>Cancel</button>
			</div>
		</form>
	{:else}
		<button class="btn primary" onclick={startNew}>New workflow</button>
	{/if}
</div>

<style>
	.tab {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.intro {
		margin: 0;
		color: var(--muted);
		font-size: 12px;
		max-width: 640px;
	}
	.muted {
		color: var(--muted-strong);
	}
	.mono {
		font-family: var(--font-mono);
		font-size: 12px;
	}
	.error {
		color: var(--danger);
		font-size: 13px;
	}
	.actions {
		display: flex;
		gap: 6px;
	}
	.editor {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 20px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.editor h3 {
		margin: 0;
		font-size: 15px;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 11px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--muted);
	}
	label.checkbox {
		flex-direction: row;
		align-items: center;
		text-transform: none;
		letter-spacing: normal;
		font-size: 12px;
	}
	.trigger-block {
		display: flex;
		align-items: center;
		gap: 8px;
		background: var(--accent-soft);
		border: 1px solid var(--border-strong);
		border-radius: 12px;
		padding: 10px 14px;
		font-size: 12px;
		color: var(--text);
	}
	.trigger-block input[type='time'] {
		width: 110px;
	}
	.trigger-block input:not([type]) {
		width: 180px;
	}
	.block-kind {
		font-family: var(--font-mono);
		font-size: 10px;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--accent);
		font-weight: 500;
	}
	.blocks {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.block-card {
		background: var(--sur2);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 12px 14px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.block-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.block-controls {
		display: flex;
		gap: 4px;
	}
	.block-controls .btn {
		padding: 4px 8px;
		font-size: 11px;
	}
	.add-block {
		display: flex;
		gap: 8px;
	}
	.row {
		display: flex;
		gap: 8px;
	}
	.runs {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.run-meta {
		display: flex;
		gap: 8px;
		align-items: center;
		font-size: 12px;
		margin-bottom: 4px;
	}
	.ok {
		color: var(--success);
		font-weight: 600;
	}
	.fail {
		color: var(--danger);
		font-weight: 600;
	}
	.runs pre {
		margin: 0;
		background: var(--sur3);
		border-radius: 8px;
		padding: 10px 12px;
		font-size: 11px;
		white-space: pre-wrap;
		max-height: 240px;
		overflow-y: auto;
	}
</style>
