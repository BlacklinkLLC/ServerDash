<script>
	import { api } from '../api.js';
	import { applyAppearance } from '../preferences.js';
	import BrandLogo from '../BrandLogo.svelte';

	let { branding, onbrandingchange } = $props();

	const themes = [
		{ id: 'dark', label: 'Dark', swatch: '#08080f' },
		{ id: 'midnight', label: 'Midnight', swatch: '#050b18' },
		{ id: 'terminal', label: 'Terminal', swatch: '#030503' },
		{ id: 'light', label: 'Light', swatch: '#f4f4f8' },
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
	const presets = ['#ffb020', '#55b5ff', '#65db93', '#b18cff', '#ff6f9c', '#ff7a45', '#2dd4bf'];

	function initialWidgetItems() {
		const catalog = branding?.widgetCatalog ?? [];
		const source = branding?.widgetLayout?.length
			? branding.widgetLayout
			: (branding?.widgets ?? []).map((id) => ({ id, enabled: true, width: catalog.find((item) => item.id === id)?.width ?? 'half' }));
		const items = source.map((item) => ({ ...item }));
		for (const item of catalog) {
			if (!items.some((existing) => existing.id === item.id)) {
				items.push({ id: item.id, enabled: false, width: item.width || 'half' });
			}
		}
		return items;
	}

	let appName = $state(branding?.appName ?? 'ServerDash');
	let accentColor = $state(branding?.accentColor ?? '#ffb020');
	let showSystem = $state(branding?.showSystem ?? true);
	let showContainers = $state(branding?.showContainers ?? true);
	let theme = $state(branding?.theme ?? 'dark');
	let density = $state(branding?.density ?? 'comfortable');
	let fontFamily = $state(branding?.fontFamily ?? 'modern');
	let borderRadius = $state(branding?.borderRadius ?? 'default');
	let backgroundPattern = $state(branding?.backgroundPattern ?? 'none');
	let refreshSeconds = $state(branding?.refreshSeconds ?? 5);
	let warnThreshold = $state(branding?.warnThreshold ?? 75);
	let critThreshold = $state(branding?.critThreshold ?? 90);
	let logoHeight = $state(branding?.logoHeight ?? 22);
	let loginMessage = $state(branding?.loginMessage ?? 'Sign in to continue.');
	let faviconFromLogo = $state(branding?.faviconFromLogo ?? true);
	let widgetItems = $state(initialWidgetItems());

	const catalog = $derived(Object.fromEntries((branding?.widgetCatalog ?? []).map((item) => [item.id, item.label])));

	function move(i, delta) {
		const j = i + delta;
		if (j < 0 || j >= widgetItems.length) return;
		const next = [...widgetItems];
		[next[i], next[j]] = [next[j], next[i]];
		widgetItems = next;
	}

	$effect(() => {
		applyAppearance({
			...branding,
			accentColor,
			theme,
			density,
			fontFamily,
			borderRadius,
			backgroundPattern,
			logoUrl: branding?.logoUrl,
			faviconFromLogo: branding?.faviconFromLogo ?? true,
		});
		return () => applyAppearance(branding ?? {});
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
				fontFamily,
				borderRadius,
				backgroundPattern,
				refreshSeconds: Number(refreshSeconds),
				warnThreshold: Number(warnThreshold),
				critThreshold: Number(critThreshold),
				logoHeight: Number(logoHeight),
				loginMessage,
				faviconFromLogo,
				widgetLayout: widgetItems.map((item) => ({ id: item.id, enabled: item.enabled, width: item.width || 'half' })),
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
			<label class="field">
				Header logo height (px)
				<input type="number" min="16" max="72" bind:value={logoHeight} class="short" />
			</label>
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
				Font family
				<select bind:value={fontFamily}>
					{#each fonts as f (f.id)}<option value={f.id}>{f.label}</option>{/each}
				</select>
			</label>
			<label class="field">
				Corner radius
				<select bind:value={borderRadius}>
					{#each radii as r (r.id)}<option value={r.id}>{r.label}</option>{/each}
				</select>
			</label>
			<label class="field">
				Background
				<select bind:value={backgroundPattern}>
					{#each patterns as p (p.id)}<option value={p.id}>{p.label}</option>{/each}
				</select>
			</label>
			<label class="field">
				Refresh every (seconds)
				<input type="number" min="0.1" max="300" step="0.1" bind:value={refreshSeconds} class="short" />
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

		<h3>Sign-in</h3>
		<label class="field">
			Sign-in message
			<input bind:value={loginMessage} maxlength="200" placeholder="Sign in to continue." />
		</label>
		<label class="checkbox">
			<input type="checkbox" bind:checked={faviconFromLogo} />
			Use the uploaded logo as the favicon
		</label>
		<p class="muted">The ServerDash and Blacklink Enterprise footer credits are required and cannot be changed.</p>

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

		<h3>Dashboard widget layout</h3>
		<p class="muted">Toggle widgets, choose their width, and use the arrows to set the order for everyone.</p>
		<ul class="widgets">
			{#each widgetItems as item, i (item.id)}
				<li class:off={!item.enabled}>
					<label class="checkbox">
						<input type="checkbox" bind:checked={item.enabled} />
						{catalog[item.id] ?? item.id}
					</label>
					<select bind:value={item.width} aria-label="Width for {item.id}">
						<option value="full">Full width</option>
						<option value="half">Half width</option>
						<option value="third">Third width</option>
					</select>
					<span class="arrows">
						<button type="button" class="btn subtle" disabled={i === 0} onclick={() => move(i, -1)} aria-label="Move up">↑</button>
						<button type="button" class="btn subtle" disabled={i === widgetItems.length - 1} onclick={() => move(i, 1)} aria-label="Move down">↓</button>
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
	.widgets li select {
		font-size: 11px;
		padding: 5px 8px;
	}
</style>
