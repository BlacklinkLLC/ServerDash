<script>
	import { onMount } from 'svelte';
	import { api } from './api.js';
	import { resolveWidgetLayout } from './preferences.js';

	let { branding, onpreferences, onreset } = $props();

	const themes = [
		{ id: 'dark', label: 'Dark' },
		{ id: 'midnight', label: 'Midnight' },
		{ id: 'terminal', label: 'Terminal' },
		{ id: 'light', label: 'Light' },
	];
	const fonts = [
		{ id: 'modern', label: 'Modern' },
		{ id: 'classic', label: 'Classic' },
		{ id: 'rounded', label: 'Rounded' },
		{ id: 'mono', label: 'Monospace' },
	];
	const radii = [
		{ id: 'sharp', label: 'Sharp' },
		{ id: 'soft', label: 'Soft' },
		{ id: 'default', label: 'Default' },
		{ id: 'round', label: 'Round' },
	];
	const patterns = [
		{ id: 'none', label: 'None' },
		{ id: 'grid', label: 'Grid' },
		{ id: 'dots', label: 'Dots' },
		{ id: 'radiant', label: 'Radiant' },
	];

	function initialItems() {
		const catalog = branding?.widgetCatalog ?? [];
		const base = resolveWidgetLayout(branding, []).map((item) => ({ ...item }));
		for (const item of catalog) {
			if (!base.some((existing) => existing.id === item.id)) {
				base.push({ id: item.id, enabled: false, width: item.width || 'half' });
			}
		}
		return base;
	}

	let theme = $state(branding?.theme ?? 'dark');
	let density = $state(branding?.density ?? 'comfortable');
	let fontFamily = $state(branding?.fontFamily ?? 'modern');
	let borderRadius = $state(branding?.borderRadius ?? 'default');
	let backgroundPattern = $state(branding?.backgroundPattern ?? 'none');
	let refreshSeconds = $state(branding?.refreshSeconds ?? 5);
	let warnThreshold = $state(branding?.warnThreshold ?? 75);
	let critThreshold = $state(branding?.critThreshold ?? 90);
	let showSystem = $state(branding?.showSystem ?? true);
	let showContainers = $state(branding?.showContainers ?? true);
	let widgetItems = $state(initialItems());
	let error = $state(null);
	let saving = $state(false);

	const catalogLabels = Object.fromEntries((branding?.widgetCatalog ?? []).map((item) => [item.id, item.label]));
	const fallbackLabels = {
		health: 'Health banner', gauges: 'Resource gauges', cpuChart: 'CPU history chart',
		memChart: 'Memory history chart', netChart: 'Network throughput chart', cores: 'Per-core CPU bars',
		disks: 'Storage volumes', containers: 'Container summary', processes: 'Top processes',
	};

	onMount(async () => {
		try {
			const saved = await api.preferences();
			theme = saved.theme ?? theme;
			density = saved.density ?? density;
			fontFamily = saved.fontFamily ?? fontFamily;
			borderRadius = saved.borderRadius ?? borderRadius;
			backgroundPattern = saved.backgroundPattern ?? backgroundPattern;
			refreshSeconds = saved.refreshSeconds ?? refreshSeconds;
			warnThreshold = saved.warnThreshold ?? warnThreshold;
			critThreshold = saved.critThreshold ?? critThreshold;
			showSystem = saved.showSystem ?? showSystem;
			showContainers = saved.showContainers ?? showContainers;
			if (Array.isArray(saved.widgetLayout)) {
				widgetItems = saved.widgetLayout.map((item) => ({ ...item }));
				for (const [id, width] of Object.entries(Object.fromEntries((branding?.widgetCatalog ?? []).map((item) => [item.id, item.width || 'half'])))) {
					if (!widgetItems.some((existing) => existing.id === id)) {
						widgetItems.push({ id, enabled: false, width });
					}
				}
			}
		} catch (e) {
			error = e.message;
		}
	});

	function move(i, delta) {
		const j = i + delta;
		if (j < 0 || j >= widgetItems.length) return;
		const next = [...widgetItems];
		[next[i], next[j]] = [next[j], next[i]];
		widgetItems = next;
	}

	async function save(e) {
		e.preventDefault();
		saving = true;
		error = null;
		try {
			const saved = await api.updatePreferences({
				theme,
				density,
				fontFamily,
				borderRadius,
				backgroundPattern,
				refreshSeconds: Number(refreshSeconds),
				warnThreshold: Number(warnThreshold),
				critThreshold: Number(critThreshold),
				showSystem,
				showContainers,
				widgetLayout: widgetItems.map((item) => ({ id: item.id, enabled: item.enabled, width: item.width || 'half' })),
			});
			onpreferences?.(saved);
		} catch (e) {
			error = e.message;
		} finally {
			saving = false;
		}
	}

	async function reset() {
		saving = true;
		error = null;
		try {
			await api.resetPreferences();
			onreset?.();
			theme = branding?.theme ?? 'dark';
			density = branding?.density ?? 'comfortable';
			fontFamily = branding?.fontFamily ?? 'modern';
			borderRadius = branding?.borderRadius ?? 'default';
			backgroundPattern = branding?.backgroundPattern ?? 'none';
			refreshSeconds = branding?.refreshSeconds ?? 5;
			warnThreshold = branding?.warnThreshold ?? 75;
			critThreshold = branding?.critThreshold ?? 90;
			showSystem = branding?.showSystem ?? true;
			showContainers = branding?.showContainers ?? true;
			widgetItems = initialItems();
		} catch (e) {
			error = e.message;
		} finally {
			saving = false;
		}
	}
</script>

<section class="panel prefs">
	<h2>Preferences</h2>
	<p class="muted">These settings apply only to your account and override the server defaults.</p>
	{#if error}<p class="error">{error}</p>{/if}

	<form onsubmit={save}>
		<h3>Look</h3>
		<div class="grid">
			<label class="field">
				Theme
				<select bind:value={theme}>
					{#each themes as item}<option value={item.id}>{item.label}</option>{/each}
				</select>
			</label>
			<label class="field">
				Density
				<select bind:value={density}>
					<option value="comfortable">Comfortable</option>
					<option value="compact">Compact</option>
				</select>
			</label>
			<label class="field">
				Font family
				<select bind:value={fontFamily}>
					{#each fonts as item}<option value={item.id}>{item.label}</option>{/each}
				</select>
			</label>
			<label class="field">
				Corner radius
				<select bind:value={borderRadius}>
					{#each radii as item}<option value={item.id}>{item.label}</option>{/each}
				</select>
			</label>
			<label class="field">
				Background
				<select bind:value={backgroundPattern}>
					{#each patterns as item}<option value={item.id}>{item.label}</option>{/each}
				</select>
			</label>
		</div>

		<h3>Dashboard</h3>
		<div class="grid">
			<label class="field">
				Refresh every (seconds)
				<input type="number" min="0.1" max="300" step="0.1" bind:value={refreshSeconds} />
			</label>
			<label class="field">
				Warning at (%)
				<input type="number" min="1" max="99" bind:value={warnThreshold} />
			</label>
			<label class="field">
				Critical at (%)
				<input type="number" min="2" max="100" bind:value={critThreshold} />
			</label>
		</div>
		<label class="checkbox"><input type="checkbox" bind:checked={showSystem} /> Server overview</label>
		<label class="checkbox"><input type="checkbox" bind:checked={showContainers} /> Services</label>

		<h3>Widget layout</h3>
		<ul class="widgets">
			{#each widgetItems as item, i (item.id)}
				<li>
					<label class="checkbox">
						<input type="checkbox" bind:checked={item.enabled} />
						{catalogLabels[item.id] ?? fallbackLabels[item.id] ?? item.id}
					</label>
					<select bind:value={item.width} aria-label="Width for {item.id}">
						<option value="full">Full width</option>
						<option value="half">Half width</option>
						<option value="third">Third width</option>
					</select>
					<span class="arrows">
						<button type="button" class="btn subtle" disabled={i === 0} onclick={() => move(i, -1)}>↑</button>
						<button type="button" class="btn subtle" disabled={i === widgetItems.length - 1} onclick={() => move(i, 1)}>↓</button>
					</span>
				</li>
			{/each}
		</ul>

		<div class="actions">
			<button class="btn primary" type="submit" disabled={saving}>{saving ? 'Saving…' : 'Save preferences'}</button>
			<button class="btn subtle" type="button" onclick={reset} disabled={saving}>Reset to server defaults</button>
		</div>
	</form>
</section>

<style>
	.prefs {
		max-width: 860px;
	}
	h2 {
		margin: 0 0 6px;
		font-size: 22px;
	}
	h3 {
		margin: 24px 0 10px;
		font-size: 14px;
	}
	.muted {
		margin: 0 0 6px;
		color: var(--muted);
		font-size: 12px;
	}
	.error {
		color: var(--danger);
		font-size: 13px;
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
		gap: 12px;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 5px;
		font-size: 11px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--muted);
	}
	.checkbox {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 10px;
		font-size: 12px;
	}
	.widgets {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.widgets li {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 8px 10px;
		border-radius: 10px;
		background: var(--sur2);
	}
	.widgets .checkbox {
		flex: 1;
		margin: 0;
	}
	.arrows {
		display: flex;
		gap: 4px;
	}
	.arrows .btn {
		padding: 4px 9px;
	}
	.actions {
		display: flex;
		gap: 10px;
		margin-top: 24px;
		flex-wrap: wrap;
	}
</style>
