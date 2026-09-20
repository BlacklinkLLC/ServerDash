<script>
	import { api } from '../api.js';

	let rules = $state({});
	let error = $state(null);
	let pruneReport = $state(null);
	let pruning = $state(false);

	// Backup config/schedule are edited as flat local fields, joined/split
	// against the rule's raw JSON config on load/save (Svelte can't bind
	// directly into a conditional/computed expression).
	let backupSources = $state('');
	let backupDest = $state('');
	let backupRetain = $state(7);
	let backupSchedule = $state('0 2 * * *');
	let pruneSchedule = $state('0 3 * * *');

	async function load() {
		try {
			const list = await api.automationRules();
			rules = Object.fromEntries(list.map((r) => [r.id, r]));
			const backupCfg = JSON.parse(rules.backup?.config || '{}');
			backupSources = (backupCfg.sourcePaths || []).join('\n');
			backupDest = backupCfg.destDir || '';
			backupRetain = backupCfg.retain || 7;
			backupSchedule = rules.backup?.schedule || '0 2 * * *';
			pruneSchedule = rules.prune?.schedule || '0 3 * * *';
			error = null;
		} catch (e) {
			error = e.message;
		}
	}
	load();

	async function toggleUnhealthy(enabled) {
		await api.updateAutomationRule('auto_restart_unhealthy', { enabled, config: {}, schedule: '' });
		await load();
	}

	async function savePrune(enabled, schedule) {
		await api.updateAutomationRule('prune', { enabled, config: {}, schedule });
		await load();
	}

	async function saveBackup(enabled, schedule) {
		const config = {
			sourcePaths: backupSources.split('\n').map((s) => s.trim()).filter(Boolean),
			destDir: backupDest.trim(),
			retain: Number(backupRetain) || 7,
		};
		await api.updateAutomationRule('backup', { enabled, config, schedule });
		await load();
	}

	async function pruneNow() {
		pruning = true;
		pruneReport = null;
		try {
			pruneReport = await api.pruneNow();
		} catch (e) {
			error = e.message;
		} finally {
			pruning = false;
		}
	}
</script>

<div class="tab">
	{#if error}
		<p class="error">{error}</p>
	{/if}

	<section class="rule">
		<header>
			<h3>Auto-restart unhealthy containers</h3>
			<label class="toggle">
				<input type="checkbox" checked={rules.auto_restart_unhealthy?.enabled} onchange={(e) => toggleUnhealthy(e.target.checked)} />
				Enabled
			</label>
		</header>
		<p class="muted">Checks every 30 seconds; restarts any container whose healthcheck reports unhealthy.</p>
	</section>

	<section class="rule">
		<header>
			<h3>Prune unused resources</h3>
			<label class="toggle">
				<input type="checkbox" checked={rules.prune?.enabled} onchange={(e) => savePrune(e.target.checked, pruneSchedule)} />
				Enabled
			</label>
		</header>
		<p class="muted">Removes stopped containers, dangling images, and unused volumes on a schedule.</p>
		<div class="row">
			<label>
				Cron schedule
				<input bind:value={pruneSchedule} onchange={() => savePrune(rules.prune?.enabled ?? false, pruneSchedule)} placeholder="0 3 * * *" />
			</label>
			<button onclick={pruneNow} disabled={pruning}>{pruning ? 'Pruning…' : 'Run now'}</button>
		</div>
		{#if pruneReport}
			<p class="muted">
				Reclaimed {(pruneReport.spaceReclaimedBytes / 1024 / 1024).toFixed(1)} MB
				({pruneReport.containersDeleted.length} containers, {pruneReport.imagesDeleted} images, {pruneReport.volumesDeleted.length} volumes)
			</p>
		{/if}
	</section>

	<section class="rule">
		<header>
			<h3>Scheduled backups</h3>
			<label class="toggle">
				<input type="checkbox" checked={rules.backup?.enabled} onchange={(e) => saveBackup(e.target.checked, backupSchedule)} />
				Enabled
			</label>
		</header>
		<p class="muted">Tars each source path to the destination directory on a schedule, keeping the newest N archives.</p>
		<div class="grid">
			<label>
				Cron schedule
				<input bind:value={backupSchedule} placeholder="0 2 * * *" />
			</label>
			<label>
				Retain
				<input type="number" bind:value={backupRetain} min="1" />
			</label>
			<label class="full">
				Source paths (one per line)
				<textarea bind:value={backupSources} rows="3" placeholder="/data/postgres&#10;/data/uploads"></textarea>
			</label>
			<label class="full">
				Destination directory
				<input bind:value={backupDest} placeholder="/backups" />
			</label>
		</div>
		<button onclick={() => saveBackup(rules.backup?.enabled ?? false, backupSchedule)}>Save backup config</button>
	</section>
</div>

<style>
	.tab {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.rule {
		background: var(--surface-1);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	h3 {
		margin: 0;
		font-size: 14px;
	}
	.muted {
		margin: 0;
		color: var(--text-secondary);
		font-size: 12px;
	}
	.error {
		color: var(--status-critical);
		font-size: 13px;
	}
	.toggle {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--text-secondary);
	}
	.row {
		display: flex;
		align-items: flex-end;
		gap: 10px;
	}
	.grid {
		display: grid;
		grid-template-columns: 1fr 100px;
		gap: 10px;
	}
	.grid .full {
		grid-column: 1 / -1;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12px;
		color: var(--text-muted);
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
	button {
		align-self: flex-start;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 7px 12px;
		font-size: 12px;
		color: var(--text-primary);
	}
</style>
