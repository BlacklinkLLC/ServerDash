<script>
	import { api } from '../api.js';

	let scripts = $state([]);
	let error = $state(null);
	let editing = $state(null); // null = not editing, {} = new, script object = editing existing
	let expandedRuns = $state(null); // script id whose run history is shown
	let runs = $state([]);
	let running = $state(new Set());

	async function load() {
		try {
			scripts = await api.scripts();
			error = null;
		} catch (e) {
			error = e.message;
		}
	}
	load();

	function startNew() {
		editing = { name: '', content: '#!/bin/sh\n', schedule: '', enabled: true };
	}

	async function save(e) {
		e.preventDefault();
		try {
			if (editing.id) {
				await api.updateScript(editing.id, editing);
			} else {
				await api.createScript(editing);
			}
			editing = null;
			await load();
		} catch (e) {
			error = e.message;
		}
	}

	async function remove(sc) {
		if (!confirm(`Delete script "${sc.name}"?`)) return;
		try {
			await api.deleteScript(sc.id);
			await load();
		} catch (e) {
			error = e.message;
		}
	}

	async function run(sc) {
		running = new Set([...running, sc.id]);
		try {
			await api.runScript(sc.id);
			if (expandedRuns === sc.id) await showRuns(sc);
		} catch (e) {
			error = e.message;
		} finally {
			running = new Set([...running].filter((id) => id !== sc.id));
		}
	}

	async function showRuns(sc) {
		expandedRuns = expandedRuns === sc.id ? null : sc.id;
		if (expandedRuns) runs = await api.scriptRuns(sc.id);
	}
</script>

<div class="tab">
	{#if error}
		<p class="error">{error}</p>
	{/if}

	<table>
		<thead>
			<tr>
				<th>Name</th>
				<th>Schedule</th>
				<th>Enabled</th>
				<th></th>
			</tr>
		</thead>
		<tbody>
			{#each scripts as sc (sc.id)}
				<tr>
					<td>{sc.name}</td>
					<td class="muted mono">{sc.schedule || 'manual only'}</td>
					<td class="muted">{sc.enabled ? 'yes' : 'no'}</td>
					<td class="actions">
						<button disabled={running.has(sc.id)} onclick={() => run(sc)}>{running.has(sc.id) ? 'Running…' : 'Run now'}</button>
						<button onclick={() => showRuns(sc)}>History</button>
						<button onclick={() => (editing = { ...sc })}>Edit</button>
						<button class="danger" onclick={() => remove(sc)}>Delete</button>
					</td>
				</tr>
				{#if expandedRuns === sc.id}
					<tr>
						<td colspan="4">
							{#if runs.length === 0}
								<p class="muted">No runs yet.</p>
							{:else}
								<ul class="runs">
									{#each runs as run (run.id)}
										<li>
											<div class="run-meta">
												<span class={run.exitCode === 0 ? 'ok' : 'fail'}>exit {run.exitCode ?? '—'}</span>
												<span class="muted">{new Date(run.startedAt).toLocaleString()} · {run.triggeredBy}</span>
											</div>
											<pre>{run.output}</pre>
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
			<h3>{editing.id ? 'Edit script' : 'New script'}</h3>
			<label>
				Name
				<input bind:value={editing.name} required />
			</label>
			<label>
				Content
				<textarea bind:value={editing.content} rows="8" class="mono" required></textarea>
			</label>
			<label>
				Cron schedule (blank = manual run only)
				<input bind:value={editing.schedule} placeholder="0 */6 * * *" />
			</label>
			<label class="checkbox">
				<input type="checkbox" bind:checked={editing.enabled} />
				Enabled
			</label>
			<div class="row">
				<button type="submit">Save</button>
				<button type="button" onclick={() => (editing = null)}>Cancel</button>
			</div>
		</form>
	{:else}
		<button onclick={startNew}>New script</button>
	{/if}
</div>

<style>
	.tab {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		background: var(--surface-1);
		border: 1px solid var(--border);
		border-radius: 10px;
		overflow: hidden;
	}
	th,
	td {
		text-align: left;
		padding: 10px 14px;
		font-size: 13px;
		border-bottom: 1px solid var(--gridline);
	}
	th {
		color: var(--text-muted);
		font-weight: 600;
		text-transform: uppercase;
		font-size: 11px;
	}
	.muted {
		color: var(--text-secondary);
	}
	.mono {
		font-family: ui-monospace, monospace;
		font-size: 12px;
	}
	.error {
		color: var(--status-critical);
		font-size: 13px;
	}
	.actions {
		display: flex;
		gap: 6px;
	}
	button {
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 6px 10px;
		font-size: 12px;
		color: var(--text-primary);
	}
	button.danger {
		color: var(--status-critical);
		border-color: var(--status-critical);
	}
	.editor {
		background: var(--surface-1);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.editor h3 {
		margin: 0;
		font-size: 14px;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12px;
		color: var(--text-muted);
	}
	label.checkbox {
		flex-direction: row;
		align-items: center;
	}
	input,
	textarea {
		font: inherit;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 6px 8px;
		color: var(--text-primary);
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
		color: var(--status-good);
		font-weight: 600;
	}
	.fail {
		color: var(--status-critical);
		font-weight: 600;
	}
	.runs pre {
		margin: 0;
		background: var(--surface-2);
		border-radius: 6px;
		padding: 8px 10px;
		font-size: 11px;
		white-space: pre-wrap;
		max-height: 200px;
		overflow-y: auto;
	}
</style>
