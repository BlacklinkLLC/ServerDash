<script>
	import Gauge from './charts/Gauge.svelte';
	import Icon from './Icon.svelte';
	import AreaChart from './charts/AreaChart.svelte';
	import { formatBytes, formatUptime, formatPercent, formatRate, statusFor, statusLabel } from './format.js';

	let { snapshot, runtime, history = [], containers = [], processes = null, settings, onnavigate } = $props();

	let warn = $derived(settings?.warnThreshold ?? 75);
	let crit = $derived(settings?.critThreshold ?? 90);
	let widgets = $derived(settings?.widgets ?? []);

	let hottestDisk = $derived(
		snapshot?.disks?.length ? snapshot.disks.reduce((a, b) => (b.usedPercent > a.usedPercent ? b : a)) : null,
	);
	let latest = $derived(history.at(-1));

	let containerCounts = $derived.by(() => {
		const c = { running: 0, stopped: 0, unhealthy: 0 };
		for (const x of containers) {
			if (x.state === 'running') c.running++;
			else c.stopped++;
			if (/unhealthy/i.test(x.status ?? '')) c.unhealthy++;
		}
		return c;
	});

	// Overall health is the worst of the headline resources; reasons list
	// what pushed it there so the banner says *why*, not just a color.
	let health = $derived.by(() => {
		if (!snapshot) return { level: 'unknown', reasons: [] };
		const checks = [
			['CPU', snapshot.cpuPercent],
			['Memory', snapshot.memUsedPercent],
			...(snapshot.disks ?? []).map((d) => [`Disk ${d.path}`, d.usedPercent]),
		];
		if (snapshot.swapTotalBytes) checks.push(['Swap', snapshot.swapPercent]);
		const rank = { good: 0, warning: 1, critical: 2 };
		let level = 'good';
		const reasons = [];
		for (const [name, v] of checks) {
			const s = statusFor(v, warn, crit);
			if (s !== 'good') reasons.push(`${name} at ${formatPercent(v)}`);
			if (rank[s] > rank[level]) level = s;
		}
		if (containerCounts.unhealthy) {
			reasons.push(`${containerCounts.unhealthy} unhealthy container${containerCounts.unhealthy > 1 ? 's' : ''}`);
			if (level === 'good') level = 'warning';
		}
		return { level, reasons };
	});

	const pctFmt = (v) => `${v.toFixed(1)}%`;
</script>

<section class="overview">
	{#if widgets.includes('health')}
	<div class="hero">
		<div class="identity">
			<div class="eyebrow">
				<span class="status-dot {health.level}"></span>
				{statusLabel[health.level]}
				{#if health.reasons.length}<span class="reasons">· {health.reasons.join(' · ')}</span>{/if}
			</div>
			<h2>{snapshot?.hostname ?? 'Server'}</h2>
			<div class="facts">
				{#if snapshot?.platform}<span>{snapshot.platform}</span>{/if}
				{#if snapshot?.kernelVersion}<span>Kernel {snapshot.kernelVersion}</span>{/if}
				{#if snapshot?.arch}<span>{snapshot.arch}</span>{/if}
				{#if runtime}<span>{runtime.name} {runtime.version}</span>{/if}
			</div>
		</div>
		<div class="hero-stats">
			<div><span class="k"><Icon name="Clock" size={12} /> Uptime</span><span class="v">{formatUptime(snapshot?.uptimeSeconds)}</span></div>
			<div>
				<span class="k"><Icon name="Speedometer" size={12} /> Load</span>
				<span class="v">{snapshot ? snapshot.loadAvg1.toFixed(2) : '—'}</span>
				<span class="sub">{snapshot ? `${snapshot.loadAvg5.toFixed(2)} · ${snapshot.loadAvg15.toFixed(2)}` : ''}</span>
			</div>
			<div><span class="k"><Icon name="Script" size={12} /> Processes</span><span class="v">{snapshot?.processCount ?? '—'}</span></div>
			<div>
				<span class="k"><Icon name="Globe" size={12} /> Network</span>
				<span class="v small">↓ {formatRate(latest?.rx ?? 0)}</span>
				<span class="sub">↑ {formatRate(latest?.tx ?? 0)}</span>
			</div>
		</div>
	</div>
	{/if}

	<div class="grid">
		{#each widgets as w (w)}
			{#if w === 'gauges'}
				<div class="panel span-full">
					<div class="gauges">
						<Gauge label="CPU" percent={snapshot?.cpuPercent} detail={snapshot ? `${snapshot.cpuCores} cores` : ''} {warn} {crit} />
						<Gauge
							label="Memory"
							percent={snapshot?.memUsedPercent}
							detail={snapshot ? `${formatBytes(snapshot.memUsedBytes)} / ${formatBytes(snapshot.memTotalBytes)}` : ''}
							{warn}
							{crit}
						/>
						<Gauge
							label="Swap"
							percent={snapshot?.swapTotalBytes ? snapshot.swapPercent : null}
							detail={snapshot?.swapTotalBytes ? `${formatBytes(snapshot.swapUsedBytes)} / ${formatBytes(snapshot.swapTotalBytes)}` : 'No swap'}
							{warn}
							{crit}
						/>
						<Gauge
							label="Fullest disk"
							percent={hottestDisk?.usedPercent}
							detail={hottestDisk ? `${hottestDisk.path} · ${formatBytes(hottestDisk.totalBytes - hottestDisk.usedBytes)} free` : ''}
							{warn}
							{crit}
						/>
					</div>
				</div>
			{:else if w === 'cpuChart'}
				<div class="panel span-half">
					<div class="panel-title"><span class="pt"><Icon name="Speedometer" size={14} /> CPU usage</span> <span>{formatPercent(snapshot?.cpuPercent)}</span></div>
					<AreaChart points={history} series={[{ key: 'cpu', label: 'CPU', color: 'var(--accent)' }]} max={100} format={pctFmt} />
				</div>
			{:else if w === 'memChart'}
				<div class="panel span-half">
					<div class="panel-title"><span class="pt"><Icon name="Stack" size={14} /> Memory usage</span> <span>{formatPercent(snapshot?.memUsedPercent)}</span></div>
					<AreaChart points={history} series={[{ key: 'mem', label: 'Memory', color: 'var(--accent)' }]} max={100} format={pctFmt} />
				</div>
			{:else if w === 'netChart'}
				<div class="panel span-full">
					<div class="panel-title">
						<span class="pt"><Icon name="Ethernet" size={14} /> Network throughput</span>
						<span class="legend">
							<span><i style="background: var(--accent)"></i>Download {formatRate(latest?.rx ?? 0)}</span>
							<span><i style="background: var(--blue)"></i>Upload {formatRate(latest?.tx ?? 0)}</span>
						</span>
					</div>
					<AreaChart
						points={history}
						series={[
							{ key: 'rx', label: 'Download', color: 'var(--accent)' },
							{ key: 'tx', label: 'Upload', color: 'var(--blue)' },
						]}
						format={formatRate}
						height={140}
					/>
				</div>
			{:else if w === 'cores' && snapshot?.perCoreCpu?.length}
				<div class="panel span-half">
					<div class="panel-title"><span class="pt"><Icon name="Bolt" size={14} /> Per-core load</span> <span>{snapshot.cpuModel ?? ''}</span></div>
					<div class="cores">
						{#each snapshot.perCoreCpu as c, i}
							<div class="core" title="Core {i}: {c.toFixed(1)}%">
								<div class="core-fill {statusFor(c, warn, crit)}" style="height: {Math.max(3, c)}%"></div>
							</div>
						{/each}
					</div>
				</div>
			{:else if w === 'disks' && snapshot?.disks?.length}
				<div class="panel span-half">
					<div class="panel-title"><span class="pt"><Icon name="Folder" size={14} /> Storage</span> <span>{snapshot.disks.length} volume{snapshot.disks.length > 1 ? 's' : ''}</span></div>
					<div class="disks">
						{#each snapshot.disks as d (d.path)}
							{@const s = statusFor(d.usedPercent, warn, crit)}
							<div class="disk">
								<div class="disk-head">
									<span class="mono">{d.path}</span>
									<span class="muted">{formatBytes(d.usedBytes)} / {formatBytes(d.totalBytes)} · {d.usedPercent.toFixed(0)}%</span>
								</div>
								<div class="bar"><div class="bar-fill {s}" style="width: {d.usedPercent}%"></div></div>
							</div>
						{/each}
					</div>
				</div>
			{:else if w === 'containers'}
				<div class="panel span-half">
					<div class="panel-title"><span class="pt"><Icon name="Blocks" size={14} /> Containers</span> <span>{containers.length} total</span></div>
					<div class="counts">
						<div><span class="status-dot good"></span><b>{containerCounts.running}</b> running</div>
						<div><span class="status-dot"></span><b>{containerCounts.stopped}</b> stopped</div>
						<div><span class="status-dot {containerCounts.unhealthy ? 'critical' : ''}"></span><b>{containerCounts.unhealthy}</b> unhealthy</div>
					</div>
					{#if containers.length}
						<div class="stack" aria-hidden="true">
							<span class="good" style="flex: {containerCounts.running}"></span>
							<span style="flex: {containerCounts.stopped}"></span>
						</div>
					{/if}
				</div>
			{:else if w === 'processes' && processes}
				<div class="panel span-half">
					<div class="panel-title">
						<span class="pt"><Icon name="Terminal" size={14} /> Top processes</span>
						<button class="link" onclick={() => onnavigate?.('tools')}>Open process manager →</button>
					</div>
					<table class="procs">
						<tbody>
							{#each processes as p (p.pid)}
								<tr>
									<td class="mono name" title={p.command}>{p.name}</td>
									<td class="muted">{p.user}</td>
									<td class="num">{p.cpu.toFixed(1)}%</td>
									<td class="num muted">{formatBytes(p.rss)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		{/each}
	</div>
</section>

<style>
	.overview {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.hero {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		gap: 20px;
		padding: 26px 24px;
		border-radius: 20px;
		border: 1px solid var(--border);
		background:
			radial-gradient(120% 140% at 0% 0%, var(--accent-soft), transparent 55%),
			var(--surface);
	}
	.eyebrow {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 11px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--muted-strong);
	}
	.reasons {
		text-transform: none;
		letter-spacing: normal;
		color: var(--muted);
	}
	h2 {
		margin: 8px 0 8px;
		font-size: 32px;
		letter-spacing: -0.02em;
	}
	.facts {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.facts span {
		padding: 4px 9px;
		border-radius: 999px;
		border: 1px solid var(--border);
		background: var(--sur2);
		font-size: 11px;
		color: var(--muted-strong);
	}
	.hero-stats {
		display: grid;
		grid-template-columns: repeat(4, minmax(90px, auto));
		gap: 24px;
		align-self: center;
	}
	.hero-stats > div {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.k {
		display: flex;
		align-items: center;
		gap: 5px;
		font-size: 10px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--muted);
	}
	.v {
		font-family: var(--font-display);
		font-size: 22px;
		font-weight: 800;
	}
	.v.small {
		font-size: 16px;
	}
	.sub {
		font-size: 11px;
		color: var(--muted);
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 16px;
	}
	.span-full {
		grid-column: 1 / -1;
	}
	@media (max-width: 760px) {
		.grid {
			grid-template-columns: 1fr;
		}
		.hero-stats {
			grid-template-columns: repeat(2, auto);
		}
	}
	.gauges {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
		gap: 16px;
	}
	.pt {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		color: var(--muted-strong);
	}
	.legend {
		display: flex;
		gap: 14px;
		text-transform: none;
		letter-spacing: normal;
		color: var(--muted-strong);
	}
	.legend span {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.legend i {
		width: 8px;
		height: 8px;
		border-radius: 2px;
	}
	.cores {
		display: flex;
		align-items: flex-end;
		gap: 3px;
		height: 120px;
	}
	.core {
		flex: 1;
		height: 100%;
		display: flex;
		align-items: flex-end;
		background: var(--sur2);
		border-radius: 4px;
		overflow: hidden;
	}
	.core-fill {
		width: 100%;
		background: var(--accent);
		border-radius: 4px 4px 0 0;
		transition: height 0.5s ease;
	}
	.core-fill.warning,
	.bar-fill.warning {
		background: var(--warning);
	}
	.core-fill.critical,
	.bar-fill.critical {
		background: var(--danger);
	}
	.disks {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.disk-head {
		display: flex;
		justify-content: space-between;
		gap: 8px;
		font-size: 12px;
		margin-bottom: 5px;
	}
	.bar {
		height: 6px;
		border-radius: 3px;
		background: var(--sur2);
		overflow: hidden;
	}
	.bar-fill {
		height: 100%;
		background: var(--accent);
		border-radius: 3px;
		transition: width 0.4s ease;
	}
	.muted {
		color: var(--muted);
	}
	.mono {
		font-family: var(--font-mono);
	}
	.counts {
		display: flex;
		flex-direction: column;
		gap: 8px;
		font-size: 13px;
	}
	.counts div {
		display: flex;
		align-items: center;
		gap: 8px;
		color: var(--muted-strong);
	}
	.counts b {
		font-family: var(--font-display);
		font-size: 18px;
		color: var(--text);
	}
	.stack {
		display: flex;
		gap: 2px;
		height: 8px;
		margin-top: 14px;
	}
	.stack span {
		border-radius: 4px;
		background: var(--sur3);
	}
	.stack span.good {
		background: var(--success);
	}
	.link {
		background: none;
		border: none;
		padding: 0;
		color: var(--accent);
		font-size: 10px;
		letter-spacing: 0.04em;
		text-transform: none;
	}
	.procs {
		width: 100%;
		border-collapse: collapse;
		font-size: 12px;
	}
	.procs td {
		padding: 6px 4px;
		border-bottom: 1px solid var(--border);
	}
	.procs tr:last-child td {
		border-bottom: none;
	}
	.procs .name {
		max-width: 160px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.num {
		text-align: right;
		font-variant-numeric: tabular-nums;
	}
</style>
