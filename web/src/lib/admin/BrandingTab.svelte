<script>
	import { api } from '../api.js';
	import BrandLogo from '../BrandLogo.svelte';

	let { branding, onbrandingchange } = $props();

	let appName = $state(branding?.appName ?? 'ServerDash');
	let error = $state(null);
	let saving = $state(false);
	let uploading = $state(false);
	let fileInput = $state(null);

	async function saveName(e) {
		e.preventDefault();
		saving = true;
		error = null;
		try {
			const updated = await api.updateSettings(appName);
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
		<h3>App name</h3>
		<p class="muted">Shown in the header, browser tab, and sign-in screen. Rename it to your own company/product name.</p>
		<form class="row" onsubmit={saveName}>
			<input bind:value={appName} maxlength="60" placeholder="ServerDash" />
			<button class="btn primary" type="submit" disabled={saving}>{saving ? 'Saving…' : 'Save'}</button>
		</form>
	</section>

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
		align-items: center;
		gap: 10px;
	}
	.row > input {
		flex: 1;
		max-width: 320px;
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
