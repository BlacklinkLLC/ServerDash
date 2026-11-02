<script>
	import { api } from '../api.js';
	import BrandLogo from '../BrandLogo.svelte';

	let { branding, onbrandingchange } = $props();

	let appName = $state(branding?.appName ?? 'ServerDash');
	let accentColor = $state(branding?.accentColor ?? '#ffb020');
	let showSystem = $state(branding?.showSystem ?? true);
	let showContainers = $state(branding?.showContainers ?? true);
	let theme = $state(branding?.theme ?? 'dark');
	let density = $state(branding?.density ?? 'comfortable');
	let refreshSeconds = $state(branding?.refreshSeconds ?? 5);
	let warnThreshold = $state(branding?.warnThreshold ?? 75);
	let critThreshold = $state(branding?.critThreshold ?? 90);
	let footerText = $state(branding?.footerText ?? '');

	const widgetCatalog = {
		health: 'Health banner (hostname, status, uptime, load, network)',
		gauges: 'Resource gauges (CPU, memory, swap, fullest disk)',
		cpuChart: 'CPU history chart',
		memChart: 'Memory history chart',
		netChart: 'Network throughput chart',
		cores: 'Per-core CPU bars',
		disks: 'Storage volumes',
		containers: 'Container summary',
		processes: 'Top processes (operators & admins)',
	};
	const themes = [
		{ id: 'dark', label: 'Dark', swatch: '#08080f' },
		{ id: 'midnight', label: 'Midnight', swatch: '#050b18' },
		{ id: 'terminal', label: 'Terminal', swatch: '#030503' },
		{ id: 'light', label: 'Light', swatch: '#f4f4f8' },
	];
	const presets = ['#ffb020', '#55b5ff', '#65db93', '#b18cff', '#ff6f9c', '#ff7a45', '#2dd4bf'];

	// Enabled widgets in saved order, followed by disabled ones.
	let enabled = $state([...(branding?.widgets ?? Object.keys(widgetCatalog))]);
	let widgetOrder = $state([...enabled, ...Object.keys(widgetCatalog).filter((w) => !enabled.includes(w))]);

	function move(i, delta) {
		const j = i + delta;
		if (j < 0 || j >= widgetOrder.length) return;
		const next = [...widgetOrder];
		[next[i], next[j]] = [next[j], next[i]];
		widgetOrder = next;
	}
	function toggleWidget(w) {
		enabled = enabled.includes(w) ? enabled.filter((x) => x !== w) : [...enabled, w];
	}

	// Theme previews instantly; saving persists it for everyone.
	$effect(() => {
		document.documentElement.dataset.theme = theme;
		document.documentElement.dataset.density = density;
		// Leaving the tab without saving reverts to the saved look.
		return () => {
			document.documentElement.dataset.theme = branding?.theme ?? 'dark';
			document.documentElement.dataset.density = branding?.density ?? 'comfortable';
		};
	});
	let error = $state(null);
	let saving = $state(false);
	let uploading = $state(false);
	let fileInput = $state(null);

	async function save(e) {
		e.preventDefault();
		saving = true;
		error = null;
		try {
			const updated = await api.updateSettings({
				appName,
				accentColor,
				showSystem,
				showContainers,
				theme,
				density,
				refreshSeconds: Number(refreshSeconds),
				warnThreshold: Number(warnThreshold),
				critThreshold: Number(critThreshold),
				widgets: widgetOrder.filter((w) => enabled.includes(w)),
				footerText,
			});
			onbrandingchange?.(updated);
		} catch (e) {
			error = e.message;
		} finally {
			saving = false;
		}
	}

	async function onFileChosen(e) {
		const file = e.target.files?.[0];
		if (!file) return;
		uploading = true;
		error = null;
		try {
			const updated = await api.uploadLogo(file);
			onbrandingchange?.(updated);
		} catch (e) {
			error = e.message;
		} finally {
			uploading = false;
			if (fileInput) fileInput.value = '';
		}
	}

	async function removeLogo() {
		try {
			const updated = await api.deleteLogo();
			onbrandingchange?.(updated);
		} catch (e) {
			error = e.message;
		}
	}
</script>

<div class="tab">
	{#if error}
		<p class="error">{error}</p>
	{/if}

	<section class="block">
		<h3>Logo</h3>
		<p class="muted">PNG, JPEG, SVG, WebP, or ICO, up to 2MB. Replaces the default monogram everywhere the app name appears.</p>
		<div class="row">
			<div class="preview">
				<BrandLogo {branding} size={40} />
			</div>
			<label class="btn subtle file-btn">
				{uploading ? 'Uploading…' : 'Upload logo'}
				<input
					bind:this={fileInput}
					type="file"
					accept="image/png,image/jpeg,image/svg+xml,image/webp,image/x-icon"
					onchange={onFileChosen}
					disabled={uploading}
				/>
			</label>
			{#if branding?.logoUrl}
				<button class="btn danger" onclick={removeLogo}>Remove</button>
			{/if}
		</div>
	</section>

	<form class="block" onsubmit={save}>
		<h3>Name &amp; color</h3>
		<p class="muted">Shown in the header, browser tab, and sign-in screen. Rename it to your own company/product name.</p>
		<div class="row">
			<label class="field">
				App name
				<input bind:value={appName} maxlength="60" placeholder="ServerDash" />
			</label>
			<label class="field color-field">
				Accent color
				<div class="color-row">
					<input type="color" bind:value={accentColor} />
					<input class="mono" bind:value={accentColor} maxlength="7" placeholder="#ffb020" />
				</div>
			</label>
		</div>
		<div class="presets">
			{#each presets as c}
				<button type="button" class="preset" class:on={accentColor === c} style="background: {c}" aria-label="Use {c}" onclick={() => (accentColor = c)}></button>
			{/each}
		</div>

		<h3>Theme</h3>
		<div class="themes">
			{#each themes as t (t.id)}
				<button type="button" class="theme" class:on={theme === t.id} onclick={() => (theme = t.id)}>
					<span class="theme-swatch" style="background: {t.swatch}"><i style="background: {accentColor}"></i></span>
					{t.label}
				</button>
			{/each}
		</div>
		<div class="row">
			<label class="field">
				Density
				<select bind:value={density}>
					<option value="comfortable">Comfortable</option>
					<option value="compact">Compact</option>
				</select>
			</label>
			<label class="field">
				Refresh every (seconds)
				<input type="number" min="2" max="300" bind:value={refreshSeconds} class="short" />
			</label>
		</div>

		<h3>Alert thresholds</h3>
		<p class="muted">Gauges, bars, and the health banner turn yellow at the warning level and red at the critical level.</p>
		<div class="row">
			<label class="field">
				Warning at (%)
				<input type="number" min="1" max="99" bind:value={warnThreshold} class="short" />
			</label>
			<label class="field">
				Critical at (%)
				<input type="number" min="2" max="100" bind:value={critThreshold} class="short" />
			</label>
		</div>

		<h3>Footer</h3>
		<label class="field">
			Footer text (blank uses the default)
			<input bind:value={footerText} maxlength="200" placeholder="ServerDash by Blacklink Enterprise…" />
		</label>

		<h3>Dashboard</h3>
		<p class="muted">Choose which sections show on the main Dashboard page for everyone.</p>
		<label class="checkbox">
			<input type="checkbox" bind:checked={showSystem} />
			Server overview (CPU, memory, disks, uptime)
		</label>
		<label class="checkbox">
			<input type="checkbox" bind:checked={showContainers} />
			Services (container list)
		</label>

		<h3>Overview widgets</h3>
		<p class="muted">Toggle widgets and use the arrows to set the order they appear in.</p>
		<ul class="widgets">
			{#each widgetOrder as w, i (w)}
				<li class:off={!enabled.includes(w)}>
					<label class="checkbox">
						<input type="checkbox" checked={enabled.includes(w)} onchange={() => toggleWidget(w)} />
						{widgetCatalog[w]}
					</label>
					<span class="arrows">
						<button type="button" class="btn subtle" disabled={i === 0} onclick={() => move(i, -1)} aria-label="Move up">↑</button>
						<button type="button" class="btn subtle" disabled={i === widgetOrder.length - 1} onclick={() => move(i, 1)} aria-label="Move down">↓</button>
					</span>
				</li>
			{/each}
		</ul>

		<button class="btn primary" type="submit" disabled={saving}>{saving ? 'Saving…' : 'Save'}</button>
	</form>
</div>

<style>
	.tab {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}
	.block {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 14px;
		padding: 18px;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	h3 {
		margin: 0;
		font-size: 14px;
	}
	h3:not(:first-child) {
		margin-top: 6px;
	}
	.muted {
		margin: 0;
		color: var(--muted);
		font-size: 12px;
	}
	.error {
		color: var(--danger);
		font-size: 13px;
	}
	.row {
		display: flex;
		align-items: flex-end;
		gap: 16px;
		flex-wrap: wrap;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 11px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--muted);
	}
	.field input:not([type='color']) {
		min-width: 220px;
	}
	.color-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.color-row input[type='color'] {
		width: 40px;
		height: 34px;
		padding: 2px;
	}
	.color-row input.mono {
		width: 100px;
		font-family: var(--font-mono);
	}
	.checkbox {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--text);
	}
	.preview {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 56px;
		height: 56px;
		border-radius: 12px;
		background: var(--sur2);
		border: 1px solid var(--border);
	}
	.short {
		min-width: 0 !important;
		width: 120px;
	}
	.presets {
		display: flex;
		gap: 8px;
	}
	.preset {
		width: 24px;
		height: 24px;
		border-radius: 50%;
		border: 2px solid var(--surface);
		box-shadow: 0 0 0 1px var(--border2);
		padding: 0;
	}
	.preset.on {
		box-shadow: 0 0 0 2px var(--text);
	}
	.themes {
		display: flex;
		gap: 10px;
		flex-wrap: wrap;
	}
	.theme {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 6px;
		padding: 8px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--sur2);
		color: var(--text);
		font-size: 11px;
	}
	.theme.on {
		border-color: var(--accent);
	}
	.theme-swatch {
		position: relative;
		display: block;
		width: 72px;
		height: 44px;
		border-radius: 8px;
		border: 1px solid var(--border2);
	}
	.theme-swatch i {
		position: absolute;
		left: 8px;
		bottom: 8px;
		width: 28px;
		height: 5px;
		border-radius: 3px;
	}
	.widgets {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.widgets li {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 6px 10px;
		border-radius: 10px;
		background: var(--sur2);
	}
	.widgets li.off {
		opacity: 0.55;
	}
	.arrows {
		display: flex;
		gap: 4px;
	}
	.arrows .btn {
		padding: 4px 9px;
	}
	.file-btn {
		position: relative;
		cursor: pointer;
	}
	.file-btn input[type='file'] {
		position: absolute;
		inset: 0;
		opacity: 0;
		cursor: pointer;
		padding: 0;
	}
</style>
