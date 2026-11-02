<script>
	import { statusFor, statusLabel } from '../format.js';

	let { label, percent = 0, detail = '', warn = 75, crit = 90 } = $props();

	const R = 42;
	const C = 2 * Math.PI * R;
	// 270° arc, opening at the bottom.
	const ARC = C * 0.75;

	let pct = $derived(Math.min(100, Math.max(0, percent ?? 0)));
	let status = $derived(statusFor(percent, warn, crit));
	let color = $derived(status === 'critical' ? 'var(--danger)' : status === 'warning' ? 'var(--warning)' : 'var(--accent)');
</script>

<div class="gauge" role="meter" aria-label={label} aria-valuenow={Math.round(pct)} aria-valuemin="0" aria-valuemax="100">
	<svg viewBox="0 0 100 100">
		<circle class="track" cx="50" cy="50" r={R} stroke-dasharray="{ARC} {C}" transform="rotate(135 50 50)" />
		<circle
			class="fill"
			cx="50"
			cy="50"
			r={R}
			stroke={color}
			stroke-dasharray="{(ARC * pct) / 100} {C}"
			transform="rotate(135 50 50)"
		/>
	</svg>
	<div class="center">
		<span class="value">{percent === null || percent === undefined ? '—' : `${Math.round(pct)}%`}</span>
		<span class="label">{label}</span>
	</div>
	<div class="meta">
		<span class="status"><span class="status-dot {status}"></span>{statusLabel[status]}</span>
		{#if detail}<span class="detail">{detail}</span>{/if}
	</div>
</div>

<style>
	.gauge {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: center;
	}
	svg {
		width: 100%;
		max-width: 140px;
	}
	circle {
		fill: none;
		stroke-width: 7;
		stroke-linecap: round;
	}
	.track {
		stroke: var(--sur3);
	}
	.fill {
		transition: stroke-dasharray 0.6s ease;
	}
	.center {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		aspect-ratio: 1;
		max-width: 140px;
		margin: 0 auto;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
	}
	.value {
		font-family: var(--font-display);
		font-size: 24px;
		font-weight: 800;
	}
	.label {
		font-size: 10px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--muted);
	}
	.meta {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 2px;
		margin-top: -8px;
		font-size: 11px;
	}
	.status {
		display: flex;
		align-items: center;
		gap: 6px;
		color: var(--muted-strong);
	}
	.detail {
		color: var(--muted);
	}
</style>
