<script>
	import { onMount, onDestroy } from 'svelte';
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import '@xterm/xterm/css/xterm.css';

	let { title, socketUrl, onclose } = $props();

	let container;
	let term;
	let fitAddon;
	let socket;
	let resizeObserver;

	// Matches the server's framing (handlers_terminal.go): every
	// client->server message starts with a one-byte type — 0x00 for input,
	// 0x01 for a resize carrying two big-endian u16s (cols, rows). Server
	// output is unframed raw bytes.
	function sendInput(data) {
		const bytes = new TextEncoder().encode(data);
		const frame = new Uint8Array(bytes.length + 1);
		frame[0] = 0x00;
		frame.set(bytes, 1);
		socket?.readyState === WebSocket.OPEN && socket.send(frame);
	}

	function sendResize(cols, rows) {
		const frame = new Uint8Array(5);
		frame[0] = 0x01;
		new DataView(frame.buffer).setUint16(1, cols, false);
		new DataView(frame.buffer).setUint16(3, rows, false);
		socket?.readyState === WebSocket.OPEN && socket.send(frame);
	}

	onMount(() => {
		term = new Terminal({
			convertEol: true,
			fontFamily: 'DM Mono, ui-monospace, monospace',
			fontSize: 13,
			theme: {
				background: '#0f0f1a',
				foreground: '#e8e8f0',
				cursor: '#ffb020',
			},
		});
		fitAddon = new FitAddon();
		term.loadAddon(fitAddon);
		term.open(container);
		fitAddon.fit();

		socket = new WebSocket(socketUrl);
		socket.binaryType = 'arraybuffer';
		socket.onopen = () => sendResize(term.cols, term.rows);
		socket.onmessage = (event) => term.write(new Uint8Array(event.data));
		socket.onclose = () => term.write('\r\n\x1b[90m[disconnected]\x1b[0m\r\n');

		term.onData(sendInput);

		resizeObserver = new ResizeObserver(() => {
			fitAddon.fit();
			sendResize(term.cols, term.rows);
		});
		resizeObserver.observe(container);
	});

	onDestroy(() => {
		resizeObserver?.disconnect();
		socket?.close();
		term?.dispose();
	});
</script>

<div
	class="overlay"
	role="button"
	tabindex="0"
	onclick={onclose}
	onkeydown={(e) => e.key === 'Escape' && onclose()}
>
	<div class="panel" role="dialog" aria-modal="true" aria-label={`Terminal: ${title}`} tabindex="-1" onclick={(e) => e.stopPropagation()}>
		<header>
			<h3>{title}</h3>
			<button onclick={onclose} aria-label="Close">✕</button>
		</header>
		<div class="term" bind:this={container}></div>
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
		background: #0f0f1a;
		border: 1px solid var(--border2);
		border-radius: 18px;
		width: min(900px, 92vw);
		height: min(600px, 82vh);
		display: flex;
		flex-direction: column;
		overflow: hidden;
		box-shadow: var(--card-shadow);
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 16px;
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
	}
	h3 {
		margin: 0;
		font-size: 14px;
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
	.term {
		flex: 1;
		padding: 10px;
		overflow: hidden;
	}
</style>
