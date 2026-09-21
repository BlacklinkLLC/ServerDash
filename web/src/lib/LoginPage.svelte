<script>
	import { api } from './api.js';
	import BrandLogo from './BrandLogo.svelte';

	let { branding, onready } = $props();

	let username = $state('');
	let password = $state('');
	let error = $state(null);
	let submitting = $state(false);

	async function submit(e) {
		e.preventDefault();
		submitting = true;
		error = null;
		try {
			const user = await api.login(username, password);
			onready?.(user);
		} catch (e) {
			error = e.message;
		} finally {
			submitting = false;
		}
	}
</script>

<div class="wrap">
	<div class="center">
		<form class="auth-card" onsubmit={submit}>
			<div class="brand">
				<BrandLogo {branding} size={32} />
				<h1>{branding?.appName ?? 'ServerDash'}</h1>
			</div>
			<p class="subtitle">Sign in to continue.</p>

			{#if error}
				<p class="error">{error}</p>
			{/if}

			<label>
				Username
				<input bind:value={username} autocomplete="username" required autofocus />
			</label>
			<label>
				Password
				<input type="password" bind:value={password} autocomplete="current-password" required />
			</label>

			<button class="btn primary" type="submit" disabled={submitting}>{submitting ? 'Signing in…' : 'Sign in'}</button>
		</form>
	</div>
	<footer>ServerDash by Blacklink Enterprise. ©2026 Blacklink, Inc. All Rights Reserved.</footer>
</div>

<style>
	.wrap {
		min-height: 100vh;
		display: flex;
		flex-direction: column;
		background: var(--bg);
	}
	.center {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 16px;
	}
	footer {
		text-align: center;
		padding: 0 16px 20px;
		color: var(--muted);
		font-size: 11px;
	}
	.auth-card {
		width: min(340px, 100%);
		background: var(--surface);
		border: 1px solid var(--border2);
		border-radius: 18px;
		padding: 32px 30px 28px;
		display: flex;
		flex-direction: column;
		gap: 16px;
		box-shadow: var(--card-shadow);
	}
	.brand {
		display: flex;
		align-items: center;
		gap: 12px;
	}
	h1 {
		margin: 0;
		font-size: 19px;
	}
	.subtitle {
		margin: -8px 0 0;
		color: var(--muted);
		font-size: 12px;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 11px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--muted);
	}
	button.primary {
		margin-top: 6px;
	}
	.error {
		margin: 0;
		color: var(--danger);
		font-size: 13px;
	}
</style>
