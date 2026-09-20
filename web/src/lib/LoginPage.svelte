<script>
	import { api } from './api.js';

	let { onready } = $props();

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
	<form class="card" onsubmit={submit}>
		<h1>ServerDash</h1>
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

		<button type="submit" disabled={submitting}>{submitting ? 'Signing in…' : 'Sign in'}</button>
	</form>
</div>

<style>
	.wrap {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 16px;
	}
	.card {
		width: min(320px, 100%);
		background: var(--surface-1);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 28px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	h1 {
		margin: 0;
		font-size: 20px;
	}
	.subtitle {
		margin: 0;
		color: var(--text-secondary);
		font-size: 13px;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12px;
		color: var(--text-muted);
	}
	input {
		font: inherit;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 8px 10px;
		color: var(--text-primary);
	}
	button {
		margin-top: 8px;
		background: var(--sequential-500);
		border: none;
		border-radius: 6px;
		padding: 10px;
		color: white;
		font-weight: 600;
		font-size: 13px;
	}
	button:disabled {
		opacity: 0.6;
	}
	.error {
		margin: 0;
		color: var(--status-critical);
		font-size: 13px;
	}
</style>
