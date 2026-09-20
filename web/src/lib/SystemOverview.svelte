<script>
	import StatTile from './StatTile.svelte';
	import { formatBytes, formatUptime, formatPercent } from './format.js';

	let { snapshot } = $props();
</script>

<section>
	<h2>{snapshot?.hostname ?? 'Server'}</h2>
	<p class="subtitle">{snapshot?.platform ?? ''}</p>

	<div class="tiles">
		<StatTile label="CPU" value={formatPercent(snapshot?.cpuPercent)} percent={snapshot?.cpuPercent ?? 0} />
		<StatTile
			label="Memory"
			value={`${formatBytes(snapshot?.memUsedBytes)} / ${formatBytes(snapshot?.memTotalBytes)}`}
			percent={snapshot?.memUsedPercent ?? 0}
		/>
		<StatTile label="Uptime" value={formatUptime(snapshot?.uptimeSeconds)} />
		<StatTile
			label="Load average"
			value={snapshot ? `${snapshot.loadAvg1.toFixed(2)} · ${snapshot.loadAvg5.toFixed(2)} · ${snapshot.loadAvg15.toFixed(2)}` : '—'}
		/>
	</div>

	{#if snapshot?.disks?.length}
		<div class="tiles">
			{#each snapshot.disks as disk (disk.path)}
				<StatTile
					label={`Disk ${disk.path}`}
					value={`${formatBytes(disk.usedBytes)} / ${formatBytes(disk.totalBytes)}`}
					percent={disk.usedPercent}
				/>
			{/each}
		</div>
	{/if}
</section>

<style>
	h2 {
		margin: 0 0 2px;
		font-size: 20px;
	}
	.subtitle {
		margin: 0 0 16px;
		color: var(--text-secondary);
		font-size: 13px;
	}
	.tiles {
		display: flex;
		flex-wrap: wrap;
		gap: 12px;
		margin-bottom: 12px;
	}
</style>
