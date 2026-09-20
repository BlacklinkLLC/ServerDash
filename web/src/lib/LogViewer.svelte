<script>
	import { api } from './api.js';

	let { container, onclose } = $props();

	let lines = $state([]);
	let socket;

	$effect(() => {
		lines = [];
		socket = new WebSocket(api.logsSocketUrl(container.id));
		socket.onmessage = (event) => {
			lines = [...lines.slice(-500), event.data];
		};
		return () => socket?.close();
	});
</script>

<div
	class="overlay"
	role="button"
	tabindex="0"
	onclick={onclose}
	onkeydown={(e) => (e.key === 'Escape' || e.key === 'Enter') && onclose()}
>
	<div
		class="panel"
		role="dialog"
		aria-modal="true"
		aria-label={`Logs for ${container.name}`}
		tabindex="-1"
		onclick={(e) => e.stopPropagation()}
		onkeydown={(e) => e.stopPropagation()}
	>
		<header>
			<h3>{container.name}</h3>
			<button onclick={onclose} aria-label="Close">✕</button>
		</header>
		<pre class="log">{lines.join('')}</pre>
	</div>
</div>

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 10;
	}
	.panel {
		background: var(--surface-1);
		border: 1px solid var(--border);
		border-radius: 10px;
		width: min(800px, 90vw);
		height: min(600px, 80vh);
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 16px;
		border-bottom: 1px solid var(--gridline);
	}
	h3 {
		margin: 0;
		font-size: 15px;
	}
	header button {
		background: none;
		border: none;
		color: var(--text-secondary);
		font-size: 14px;
	}
	.log {
		flex: 1;
		margin: 0;
		padding: 12px 16px;
		overflow-y: auto;
		font-family: ui-monospace, monospace;
		font-size: 12px;
		white-space: pre-wrap;
		color: var(--text-secondary);
	}
</style>
