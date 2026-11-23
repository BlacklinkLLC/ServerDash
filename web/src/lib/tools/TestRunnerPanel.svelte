<script>
	import { onMount } from 'svelte';
	import { api } from '../api.js';
	import Icon from '../Icon.svelte';

	const presets = [
		{ name: 'Go tests', icon: 'Code', dir: '/home/Services/ServerDash/server', command: 'go test ./...' },
		{ name: 'Go plugins', icon: 'Stack', dir: '/home/Services/ServerDash/server', command: 'go test ./internal/plugins/...' },
		{ name: 'npm test', icon: 'Code', dir: '/home/Services/ServerDash/web', command: 'npm test' },
		{ name: 'npm build', icon: 'Code', dir: '/home/Services/ServerDash/web', command: 'npm run build' },
		{ name: 'Shell syntax', icon: 'Script', dir: '/home/Services/ServerDash', command: 'bash -n install.sh && bash -n fix.sh' },
		{ name: 'Plugin scripts', icon: 'Stack', dir: '/home/Services/ServerDash', command: 'for f in plugins/*/main.sh; do sh -n "$f" || exit 1; done' }
	];

	let scripts = $state([]);
	let name = $state('');
	let command = $state('');
	let dir = $state('/home/Services/ServerDash');
	let result = $state(null);
	let running = $state(false);
	let error = $state('');

	onMount(async () => {
		try {
			scripts = await api.scripts();
		} catch (e) {
			error = e.message;
		}
	});

	function choosePreset(preset) {
		name = preset.name;
		command = preset.command;
		dir = preset.dir;
		result = null;
	}

	async function run(test) {
		running = true;
		error = '';
		try {
			result = await api.runTest(test);
		} catch (e) {
			error = e.message;
		} finally {
			running = false;
		}
	}
</script>

{#if error}<p class="error">{error}</p>{/if}

<div class="panel">
	<div class="panel-title"><span><Icon name="Code" size={16} /> Presets</span><span class="muted">Change the working directory if your checkout lives elsewhere</span></div>
	<div class="preset-grid">
		{#each presets as preset (preset.name)}
			<button class="preset" onclick={() => choosePreset(preset)}>
				<Icon name={preset.icon} size={16} />
				<b>{preset.name}</b>
				<code>{preset.command}</code>
			</button>
		{/each}
	</div>
</div>

<div class="panel run-panel">
	<div class="panel-title"><span><Icon name="Play" size={16} /> Custom test</span></div>
	<div class="field"><label for="test-name">Name</label><input id="test-name" bind:value={name} placeholder="Smoke test" /></div>
	<div class="field"><label for="test-command">Command</label><textarea id="test-command" rows="4" bind:value={command} placeholder="go test ./..."></textarea></div>
	<div class="field"><label for="test-dir">Working directory</label><input id="test-dir" bind:value={dir} placeholder="/home/Services/ServerDash" /></div>
	<button class="btn primary" disabled={running || !command.trim()} onclick={() => run({ name: name || 'Custom test', command, dir })}>
		<Icon name="Play" size={14} /> {running ? 'Running…' : 'Run test'}
	</button>
</div>

{#if scripts.length}
	<div class="panel">
		<div class="panel-title"><span><Icon name="Script" size={16} /> Saved scripts</span><span class="muted">Runs are recorded in script history</span></div>
		<div class="saved-list">
			{#each scripts as script (script.id)}
				<div class="saved-row">
					<div><b>{script.name}</b>{#if script.schedule}<span class="muted"> · {script.schedule}</span>{/if}</div>
					<button class="btn subtle" disabled={running} onclick={() => run({ scriptId: script.id, name: script.name })}><Icon name="Play" size={14} /> Run</button>
				</div>
			{/each}
		</div>
	</div>
{/if}

{#if result}
	<div class="panel result-panel">
		<div class="panel-title">
			<span class:good={result.ok} class:bad={!result.ok}><Icon name={result.ok ? 'Shield' : 'Warning'} size={16} /> {result.name || 'Test'}</span>
			<span class="muted">exit {result.exitCode} · {result.durationMs} ms{result.recorded ? ' · saved to history' : ''}</span>
		</div>
		<pre>{result.output || '(no output)'}</pre>
	</div>
{/if}

<style>
	.panel {
		margin-bottom: 14px;
	}
	.panel-title {
		display: flex;
		justify-content: space-between;
		gap: 10px;
		align-items: center;
	}
	.panel-title > span:first-child {
		display: inline-flex;
		align-items: center;
		gap: 7px;
	}
	.preset-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
		gap: 10px;
		margin-top: 12px;
	}
	.preset {
		display: grid;
		grid-template-columns: 20px 1fr;
		gap: 4px 8px;
		align-items: center;
		text-align: left;
		padding: 10px;
	}
	.preset code {
		grid-column: 2;
		font-size: 10px;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.field {
		display: grid;
		grid-template-columns: 130px 1fr;
		gap: 10px;
		align-items: start;
		margin: 10px 0;
	}
	.field label {
		padding-top: 9px;
		font-size: 12px;
		color: var(--muted);
	}
	textarea,
	input {
		width: 100%;
	}
	.saved-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 10px;
		padding: 8px 0;
		border-bottom: 1px solid var(--border);
	}
	.saved-row:last-child {
		border-bottom: none;
	}
	.result-panel pre {
		max-height: 52vh;
		overflow: auto;
		white-space: pre-wrap;
		word-break: break-word;
		font-family: var(--font-mono);
		font-size: 12px;
	}
	.good {
		color: var(--success);
	}
	.bad {
		color: var(--danger);
	}
	.error {
		color: var(--danger);
	}
	.muted {
		color: var(--muted);
	}
</style>
