<script>
	import { api } from '../api.js';

	let status = $state(null);
	let error = $state(null);
	let checking = $state(true);
	let applying = $state(false);
	let applied = $state(false);

	async function check() {
		checking = true;
		error = null;
		try {
			status = await api.updateStatus();
		} catch (e) {
			error = e.message;
		} finally {
			checking = false;
		}
	}
	check();

	async function apply() {
		if (!confirm('Pull the latest code and rebuild? ServerDash will restart — this page will briefly disconnect.')) return;
		applying = true;
		error = null;
		try {
			await api.applyUpdate();
			applied = true;
		} catch (e) {
			error = e.message;
			applying = false;
		}
	}
</script>

<div class="tab">
	<p class="intro">
		Checks ServerDash's own git checkout against its remote. Applying an update pulls the latest commit, rebuilds the
		image, and restarts the service — your users, settings, scripts, and workflows live in a separate persistent
		volume and aren't affected.
	</p>

	{#if error}
		<p class="error">{error}</p>
	{/if}

	{#if checking}
		<p class="muted">Checking…</p>
	{:else if !status?.enabled}
		<p class="muted">
			Self-update isn't configured. Set <code>SERVERDASH_INSTALL_DIR</code> to ServerDash's git checkout path on the host,
			mounted into this container at the same path, to enable this.
		</p>
	{:else if status.error}
		<p class="error">{status.error}</p>
		<button class="btn subtle" onclick={check}>Retry</button>
	{:else if applied}
		<div class="block">
			<p>Update triggered — rebuilding and restarting now. This page will reconnect once it's back.</p>
		</div>
	{:else}
		<div class="block">
			<div class="row">
				<span class="badge-pill {status.commitsBehind > 0 ? 'accent' : 'success'}">
					{status.commitsBehind > 0 ? `${status.commitsBehind} commit${status.commitsBehind === 1 ? '' : 's'} behind` : 'up to date'}
				</span>
				<span class="muted mono">{status.currentCommit} → {status.remoteCommit}</span>
				<span class="muted">on {status.branch}</span>
			</div>

			{#if status.log?.length}
				<ul class="log">
					{#each status.log as line (line)}
						<li>{line}</li>
					{/each}
				</ul>
			{/if}

			<div class="row">
				<button class="btn subtle" onclick={check}>Check again</button>
				{#if status.commitsBehind > 0}
					<button class="btn primary" onclick={apply} disabled={applying}>{applying ? 'Updating…' : 'Update now'}</button>
				{/if}
			</div>
		</div>
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
		font-size: 12px;
	}
	.mono {
		font-family: var(--font-mono);
	}
	.error {
		color: var(--danger);
		font-size: 13px;
	}
	.block {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 14px;
		padding: 18px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
	}
	.log {
		margin: 0;
		padding: 0 0 0 18px;
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--muted-strong);
		max-height: 200px;
		overflow-y: auto;
	}
</style>
