<script>
	import { untrack } from 'svelte';
	import { api } from '../api.js';

	// Admin form for one plugin's settings.
	let { plugin, onsaved } = $props();
	let values = $state(
		untrack(() => Object.fromEntries((plugin.settings ?? []).map((s) => [s.key, plugin.values?.[s.key] ?? s.default ?? '']))),
	);
	let saving = $state(false);
	let error = $state(null);
	let saved = $state(false);

	async function save(e) {
		e.preventDefault();
		saving = true;
		error = null;
		try {
			await api.updatePlugin(plugin.id, { enabled: plugin.enabled, dashboard: plugin.dashboard, settings: values });
			saved = true;
			setTimeout(() => (saved = false), 2000);
			onsaved?.();
		} catch (err) {
			error = err.message;
		} finally {
			saving = false;
		}
	}
</script>

<form onsubmit={save}>
	{#each plugin.settings as s (s.key)}
		<label class:check={s.type === 'checkbox'}>
			{#if s.type === 'checkbox'}
				<input type="checkbox" checked={values[s.key] === 'true'} onchange={(e) => (values[s.key] = e.currentTarget.checked ? 'true' : 'false')} />
				<span>{s.label}</span>
			{:else}
				<span>{s.label}</span>
				{#if s.type === 'textarea'}
					<textarea rows="4" bind:value={values[s.key]} placeholder={s.placeholder}></textarea>
				{:else if s.type === 'select'}
					<select bind:value={values[s.key]}>{#each s.options ?? [] as o (o)}<option value={o}>{o}</option>{/each}</select>
				{:else}
					<input type={s.type === 'number' ? 'number' : s.type === 'password' ? 'password' : 'text'} bind:value={values[s.key]} placeholder={s.placeholder} />
				{/if}
			{/if}
			{#if s.help}<small>{s.help}</small>{/if}
		</label>
	{/each}
	{#if error}<p class="err">{error}</p>{/if}
	<div class="row">
		<button class="btn primary" disabled={saving}>{saving ? 'Saving…' : 'Save settings'}</button>
		{#if saved}<span class="ok">Saved</span>{/if}
	</div>
</form>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12px;
	}
	label.check {
		flex-direction: row;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
	}
	label > span {
		color: var(--muted-strong);
	}
	small {
		color: var(--muted);
		font-size: 11px;
		width: 100%;
	}
	textarea {
		font-family: var(--font-mono);
		font-size: 12px;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.ok {
		color: var(--success);
		font-size: 12px;
	}
	.err {
		color: var(--danger);
		margin: 0;
	}
</style>
