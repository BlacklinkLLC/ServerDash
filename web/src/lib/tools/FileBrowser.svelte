<script>
	import { api } from '../api.js';
	import { formatBytes } from '../format.js';
	import Icon from '../Icon.svelte';

	let path = $state('/');
	let listing = $state(null);
	let error = $state('');
	let loading = $state(false);
	let preview = $state(null);
	let previewText = $state('');

	function ext(entry) {
		const name = entry.name.toLowerCase();
		const dot = name.lastIndexOf('.');
		return dot === -1 ? '' : name.slice(dot);
	}

	function kind(entry) {
		if (entry.isDir) return 'dir';
		const mime = (entry.mime || '').split(';')[0];
		if (mime.startsWith('image/')) return 'image';
		if (mime.startsWith('video/')) return 'video';
		if (mime.startsWith('audio/')) return 'audio';
		if (mime === 'application/pdf') return 'pdf';
		if (mime.startsWith('text/') || ['json', 'js', 'ts', 'svelte', 'go', 'sh', 'py', 'yml', 'yaml', 'toml', 'env', 'log', 'md', 'css', 'html'].includes(ext(entry).replace('.', ''))) return 'text';
		return 'other';
	}

	function iconFor(entry) {
		const k = kind(entry);
		if (k === 'dir') return 'Folder';
		if (k === 'video' || k === 'audio') return 'Play';
		if (k === 'text') return 'Code';
		return 'Document';
	}

	async function load(next = path) {
		loading = true;
		try {
			listing = await api.files(next);
			path = listing.path;
			error = '';
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load(path);
	});

	function goUp() {
		if (listing?.parent) {
			preview = null;
			path = listing.parent;
		}
	}

	async function openFile(entry) {
		preview = entry;
		previewText = '';
		if (kind(entry) !== 'text') return;
		try {
			const res = await fetch(api.fileContentUrl(entry.path), { credentials: 'include' });
			if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
			previewText = await res.text();
		} catch (e) {
			previewText = `Could not load file: ${e.message}`;
		}
	}
</script>

<div class="toolbar">
	<button class="btn subtle" onclick={goUp} disabled={!listing || listing.path === '/'}><Icon name="Home" size={14} /> Up</button>
	<input aria-label="Path" bind:value={path} onkeydown={(e) => e.key === 'Enter' && load()} />
	<button class="btn subtle" onclick={() => load()}><Icon name="Refresh" size={14} /> Go</button>
	{#if loading}<span class="muted">Loading…</span>{/if}
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if listing}
	<p class="muted crumb">{listing.path}</p>
	<table class="token-table">
		<thead>
			<tr><th>Name</th><th class="num">Size</th><th>Type</th><th>Modified</th><th></th></tr>
		</thead>
		<tbody>
			{#each listing.entries as entry (entry.path)}
				<tr>
					<td>
						<button class="file-link" onclick={() => (entry.isDir ? (path = entry.path) : openFile(entry))}>
							<Icon name={iconFor(entry)} size={14} /> {entry.name}
						</button>
					</td>
					<td class="num muted">{entry.isDir ? '—' : formatBytes(entry.size)}</td>
					<td class="muted">{entry.mime.split(';')[0]}</td>
					<td class="muted">{new Date(entry.modified).toLocaleString()}</td>
					<td class="actions">
						{#if !entry.isDir}
							<a class="btn subtle" href={api.fileContentUrl(entry.path, true)}><Icon name="Download" size={14} /> Download</a>
						{/if}
					</td>
				</tr>
			{:else}
				<tr><td colspan="5" class="muted">This directory is empty.</td></tr>
			{/each}
		</tbody>
	</table>
{/if}

{#if preview}
	{@const src = api.fileContentUrl(preview.path)}
	<div class="modal-backdrop" role="presentation" onclick={() => (preview = null)}>
		<div class="modal panel preview" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && (preview = null)}>
			<div class="preview-head">
				<div>
					<h3><Icon name={iconFor(preview)} size={16} /> {preview.name}</h3>
					<p class="muted">{preview.path} · {formatBytes(preview.size)}</p>
				</div>
				<div class="preview-actions">
					<a class="btn subtle" href={api.fileContentUrl(preview.path, true)}><Icon name="Download" size={14} /> Download</a>
					<button class="btn subtle" onclick={() => (preview = null)}>Close</button>
				</div>
			</div>
			{#if kind(preview) === 'image'}
				<img {src} alt={preview.name} />
			{:else if kind(preview) === 'video'}
				<video {src} controls><track kind="captions" /></video>
			{:else if kind(preview) === 'audio'}
				<audio {src} controls></audio>
			{:else if kind(preview) === 'pdf'}
				<iframe {src} title={preview.name}></iframe>
			{:else if kind(preview) === 'text'}
				<pre class="text">{previewText}</pre>
			{:else}
				<div class="panel note"><Icon name="Information" size={18} /> No inline preview for this file type. Download it to open locally.</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.toolbar input {
		flex: 1;
		min-width: 260px;
	}
	.crumb {
		font-family: var(--font-mono);
		font-size: 11px;
	}
	.file-link {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		background: none;
		border: none;
		color: var(--text);
		cursor: pointer;
		font: inherit;
		text-align: left;
	}
	.file-link:hover {
		color: var(--accent);
	}
	.actions {
		display: flex;
		gap: 6px;
		justify-content: flex-end;
	}
	.actions .btn {
		padding: 5px 9px;
		font-size: 11px;
	}
	.error {
		color: var(--danger);
	}
	.muted {
		color: var(--muted);
	}
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 60;
	}
	.modal.preview {
		width: min(1000px, 92vw);
		max-height: 88vh;
		overflow: auto;
	}
	.preview-head {
		display: flex;
		justify-content: space-between;
		gap: 12px;
		align-items: flex-start;
		margin-bottom: 12px;
	}
	.preview-head h3 {
		display: flex;
		align-items: center;
		gap: 7px;
		margin: 0;
	}
	.preview-head p {
		margin: 4px 0 0;
		font-size: 11px;
	}
	.preview-actions {
		display: flex;
		gap: 8px;
	}
	.modal img,
	.modal video,
	.modal iframe {
		display: block;
		width: 100%;
		max-height: 70vh;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: #000;
	}
	.modal iframe {
		min-height: 65vh;
	}
	.modal audio {
		width: 100%;
	}
	.modal .text {
		max-height: 68vh;
		overflow: auto;
		white-space: pre-wrap;
		word-break: break-word;
		font-family: var(--font-mono);
		font-size: 12px;
	}
	.note {
		display: flex;
		gap: 10px;
		align-items: center;
	}
</style>
