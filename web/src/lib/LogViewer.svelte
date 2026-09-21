<script>
	let { title, socketUrl, onclose } = $props();

	let lines = $state([]);
	let socket;

	$effect(() => {
		lines = [];
		socket = new WebSocket(socketUrl);
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
		aria-label={`Logs for ${title}`}
		tabindex="-1"
		onclick={(e) => e.stopPropagation()}
		onkeydown={(e) => e.stopPropagation()}
	>
		<header>
			<h3>{title}</h3>
			<button onclick={onclose} aria-label="Close">✕</button>
		</header>
		<pre class="log">{lines.join('')}</pre>
	</div>
</div>

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 10;
	}
	.panel {
		background: var(--surface);
		border: 1px solid var(--border2);
		border-radius: 18px;
		width: min(800px, 90vw);
		height: min(600px, 80vh);
		display: flex;
		flex-direction: column;
		overflow: hidden;
		box-shadow: var(--card-shadow);
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 14px 18px;
		border-bottom: 1px solid var(--border);
	}
	h3 {
		margin: 0;
		font-size: 15px;
	}
	header button {
		background: none;
		border: none;
		color: var(--muted);
		font-size: 14px;
	}
	header button:hover {
		color: var(--accent);
	}
	.log {
		flex: 1;
		margin: 0;
		padding: 14px 18px;
		overflow-y: auto;
		font-family: var(--font-mono);
		font-size: 12px;
		white-space: pre-wrap;
		color: var(--muted-strong);
	}
</style>
