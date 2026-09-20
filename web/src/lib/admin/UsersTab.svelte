<script>
	import { api } from '../api.js';

	let { currentUserId } = $props();

	let users = $state([]);
	let error = $state(null);
	let newUsername = $state('');
	let newPassword = $state('');
	let newRole = $state('viewer');
	let resettingFor = $state(null);
	let resetPassword = $state('');

	async function load() {
		try {
			users = await api.users();
			error = null;
		} catch (e) {
			error = e.message;
		}
	}
	load();

	async function createUser(e) {
		e.preventDefault();
		try {
			await api.createUser(newUsername, newPassword, newRole);
			newUsername = '';
			newPassword = '';
			newRole = 'viewer';
			await load();
		} catch (e) {
			error = e.message;
		}
	}

	async function setRole(u, role) {
		try {
			await api.setUserRole(u.id, role);
			await load();
		} catch (e) {
			error = e.message;
		}
	}

	async function submitReset(e) {
		e.preventDefault();
		try {
			await api.setUserPassword(resettingFor.id, resetPassword);
			resettingFor = null;
			resetPassword = '';
		} catch (e) {
			error = e.message;
		}
	}

	async function remove(u) {
		if (!confirm(`Delete user "${u.username}"? This can't be undone.`)) return;
		try {
			await api.deleteUser(u.id);
			await load();
		} catch (e) {
			error = e.message;
		}
	}
</script>

<div class="tab">
	{#if error}
		<p class="error">{error}</p>
	{/if}

	<table>
		<thead>
			<tr>
				<th>Username</th>
				<th>Role</th>
				<th>Created</th>
				<th></th>
			</tr>
		</thead>
		<tbody>
			{#each users as u (u.id)}
				<tr>
					<td>{u.username}</td>
					<td>
						{#if u.id === currentUserId}
							<span class="muted">{u.role} (you)</span>
						{:else}
							<select value={u.role} onchange={(e) => setRole(u, e.target.value)}>
								<option value="admin">admin</option>
								<option value="operator">operator</option>
								<option value="viewer">viewer</option>
							</select>
						{/if}
					</td>
					<td class="muted">{new Date(u.createdAt).toLocaleDateString()}</td>
					<td class="actions">
						<button onclick={() => (resettingFor = u)}>Reset password</button>
						{#if u.id !== currentUserId}
							<button class="danger" onclick={() => remove(u)}>Delete</button>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>

	{#if resettingFor}
		<form class="inline-form" onsubmit={submitReset}>
			<span>New password for <strong>{resettingFor.username}</strong>:</span>
			<input type="password" bind:value={resetPassword} minlength="8" required autofocus />
			<button type="submit">Set</button>
			<button type="button" onclick={() => (resettingFor = null)}>Cancel</button>
		</form>
	{/if}

	<h3>Add user</h3>
	<form class="inline-form" onsubmit={createUser}>
		<input placeholder="Username" bind:value={newUsername} minlength="3" required />
		<input type="password" placeholder="Password" bind:value={newPassword} minlength="8" required />
		<select bind:value={newRole}>
			<option value="admin">admin</option>
			<option value="operator">operator</option>
			<option value="viewer">viewer</option>
		</select>
		<button type="submit">Create</button>
	</form>
</div>

<style>
	.tab {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	h3 {
		margin: 0;
		font-size: 14px;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		background: var(--surface-1);
		border: 1px solid var(--border);
		border-radius: 10px;
		overflow: hidden;
	}
	th,
	td {
		text-align: left;
		padding: 10px 14px;
		font-size: 13px;
		border-bottom: 1px solid var(--gridline);
	}
	th {
		color: var(--text-muted);
		font-weight: 600;
		text-transform: uppercase;
		font-size: 11px;
	}
	tr:last-child td {
		border-bottom: none;
	}
	.muted {
		color: var(--text-secondary);
	}
	.error {
		color: var(--status-critical);
		font-size: 13px;
	}
	select,
	input {
		font: inherit;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 6px 8px;
		color: var(--text-primary);
	}
	.actions {
		display: flex;
		gap: 6px;
	}
	button {
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 6px 10px;
		font-size: 12px;
		color: var(--text-primary);
	}
	button.danger {
		color: var(--status-critical);
		border-color: var(--status-critical);
	}
	.inline-form {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
	}
</style>
