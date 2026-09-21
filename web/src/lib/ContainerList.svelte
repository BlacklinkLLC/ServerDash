<script>
	import StatusBadge from './StatusBadge.svelte';
	import LogViewer from './LogViewer.svelte';
	import NicknameLabel from './NicknameLabel.svelte';
	import { api } from './api.js';

	let { containers, canOperate, onaction } = $props();

	let logsFor = $state(null);
	let pending = $state(new Set());

	async function run(action, id) {
		pending = new Set([...pending, id]);
		try {
			if (action === 'start') await api.startContainer(id);
			if (action === 'stop') await api.stopContainer(id);
			if (action === 'restart') await api.restartContainer(id);
			onaction?.();
		} finally {
			pending = new Set([...pending].filter((p) => p !== id));
		}
	}
</script>

<section>
	<h2>Services</h2>
	<table class="token-table">
		<thead>
			<tr>
				<th>Name</th>
				<th>Status</th>
				<th>Image</th>
				<th>Ports</th>
				<th></th>
			</tr>
		</thead>
		<tbody>
			{#each containers as c (c.id)}
				<tr>
					<td>
						<NicknameLabel
							name={c.name}
							nickname={c.nickname}
							editable={canOperate}
							onsave={async (nick) => {
								await api.setContainerNickname(c.id, nick);
								onaction?.();
							}}
							ondelete={async () => {
								await api.deleteContainerNickname(c.id);
								onaction?.();
							}}
						/>
					</td>
					<td><StatusBadge state={c.state} /></td>
					<td class="muted">{c.image}</td>
					<td class="muted">{c.ports.map((p) => `${p.publicPort || ''}${p.publicPort ? ':' : ''}${p.privatePort}`).join(', ') || '—'}</td>
					<td class="actions">
						{#if canOperate}
							<button class="btn subtle" disabled={pending.has(c.id)} onclick={() => run(c.state === 'running' ? 'stop' : 'start', c.id)}>
								{c.state === 'running' ? 'Stop' : 'Start'}
							</button>
							<button class="btn subtle" disabled={pending.has(c.id)} onclick={() => run('restart', c.id)}>Restart</button>
						{/if}
						<button class="btn subtle" onclick={() => (logsFor = c)}>Logs</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</section>

{#if logsFor}
	<LogViewer title={logsFor.name} socketUrl={api.containerLogsSocketUrl(logsFor.id)} onclose={() => (logsFor = null)} />
{/if}

<style>
	h2 {
		margin: 0 0 12px;
		font-size: 18px;
	}
	.muted {
		color: var(--muted-strong);
	}
	.actions {
		display: flex;
		gap: 6px;
	}
</style>
