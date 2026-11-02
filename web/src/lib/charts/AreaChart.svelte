<script>
	// Single- or dual-series time chart with a crosshair + tooltip. Series
	// share one y-axis (same unit), so callers never mix measures here.
	import { formatClock } from '../format.js';

	let {
		points = [], // [{ at, ...values }]
		series = [], // [{ key, label, color }]
		max = null, // fixed y max (e.g. 100 for percent); null = auto
		format = (v) => v.toFixed(1),
		height = 120,
	} = $props();

	const W = 600;
	let hover = $state(null);
	let el = $state(null);

	let yMax = $derived.by(() => {
		if (max !== null) return max;
		let m = 0;
		for (const p of points) for (const s of series) m = Math.max(m, p[s.key] ?? 0);
		return m > 0 ? m * 1.15 : 1;
	});

	function x(i) {
		return points.length > 1 ? (i / (points.length - 1)) * W : 0;
	}
	function y(v) {
		return height - 4 - ((v ?? 0) / yMax) * (height - 8);
	}
	function linePath(key) {
		return points.map((p, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(p[key]).toFixed(1)}`).join('');
	}
	function areaPath(key) {
		if (points.length < 2) return '';
		return `${linePath(key)}L${W},${height}L0,${height}Z`;
	}

	function onMove(e) {
		if (!el || points.length < 2) return;
		const rect = el.getBoundingClientRect();
		const frac = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width));
		const i = Math.round(frac * (points.length - 1));
		hover = { i, left: frac * 100 };
	}
</script>

<div
	class="chart"
	bind:this={el}
	onpointermove={onMove}
	onpointerleave={() => (hover = null)}
	role="img"
	aria-label={series.map((s) => s.label).join(' and ') + ' over time'}
>
	<svg viewBox="0 0 {W} {height}" preserveAspectRatio="none" style="height: {height}px">
		{#each [0.25, 0.5, 0.75] as g}
			<line class="grid" x1="0" x2={W} y1={height * g} y2={height * g} />
		{/each}
		{#each series as s (s.key)}
			<path d={areaPath(s.key)} fill={s.color} opacity="0.10" />
			<path d={linePath(s.key)} fill="none" stroke={s.color} stroke-width="2" vector-effect="non-scaling-stroke" stroke-linejoin="round" />
		{/each}
		{#if hover}
			<line class="cross" x1={x(hover.i)} x2={x(hover.i)} y1="0" y2={height} vector-effect="non-scaling-stroke" />
		{/if}
	</svg>
	{#if hover && points[hover.i]}
		{#each series as s (s.key)}
			<span class="dot" style="left: {hover.left}%; top: {y(points[hover.i][s.key])}px; background: {s.color}"></span>
		{/each}
		<div class="tip" style="left: {hover.left}%" class:flip={hover.left > 65}>
			<div class="tip-time">{formatClock(points[hover.i].at)}</div>
			{#each series as s (s.key)}
				<div class="tip-row"><span class="swatch" style="background: {s.color}"></span>{s.label}<b>{format(points[hover.i][s.key] ?? 0)}</b></div>
			{/each}
		</div>
	{/if}
	{#if points.length < 2}
		<div class="empty">Collecting data…</div>
	{/if}
</div>

<style>
	.chart {
		position: relative;
		width: 100%;
		cursor: crosshair;
	}
	svg {
		display: block;
		width: 100%;
		overflow: visible;
	}
	.grid {
		stroke: var(--border);
		stroke-width: 1;
		vector-effect: non-scaling-stroke;
		stroke-dasharray: 2 4;
	}
	.cross {
		stroke: var(--muted);
		stroke-width: 1;
	}
	.dot {
		position: absolute;
		width: 9px;
		height: 9px;
		border-radius: 50%;
		transform: translate(-50%, -50%);
		box-shadow: 0 0 0 2px var(--surface);
		pointer-events: none;
	}
	.tip {
		position: absolute;
		top: -6px;
		transform: translate(12px, -100%);
		background: var(--sur3);
		border: 1px solid var(--border2);
		border-radius: 10px;
		padding: 8px 10px;
		font-size: 11px;
		white-space: nowrap;
		pointer-events: none;
		z-index: 5;
		box-shadow: var(--card-shadow);
	}
	.tip.flip {
		transform: translate(calc(-100% - 12px), -100%);
	}
	.tip-time {
		color: var(--muted);
		margin-bottom: 4px;
	}
	.tip-row {
		display: flex;
		align-items: center;
		gap: 6px;
		color: var(--muted-strong);
	}
	.tip-row b {
		margin-left: auto;
		padding-left: 12px;
		color: var(--text);
		font-weight: 500;
	}
	.swatch {
		width: 8px;
		height: 8px;
		border-radius: 2px;
	}
	.empty {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--muted);
		font-size: 11px;
	}
</style>
