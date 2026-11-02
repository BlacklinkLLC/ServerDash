<script>
	import { api } from './api.js';
	import BrandLogo from './BrandLogo.svelte';
	import Icon from './Icon.svelte';

	// First-run setup wizard: account → look → cluster role → done. The
	// account step creates the admin and signs in, so later steps use the
	// normal authenticated APIs and every step after the first is skippable.
	let { branding, onready, onbrandingchange } = $props();

	const steps = ['Account', 'Look', 'Cluster', 'Done'];
	let step = $state(0);
	let user = $state(null);
	let error = $state(null);
	let busy = $state(false);

	// --- Account ---
	let codeRequired = $state(false);
	let setupCode = $state('');
	let username = $state('admin');
	let password = $state('');
	let confirm = $state('');

	api.setupStatus()
		.then((s) => (codeRequired = !!s.codeRequired))
		.catch(() => {});

	// --- Look ---
	let appName = $state(branding?.appName ?? 'ServerDash');
	let accentColor = $state(branding?.accentColor ?? '#ffb020');
	let theme = $state(branding?.theme ?? 'dark');
	const presets = ['#ffb020', '#55b5ff', '#65db93', '#b18cff', '#ff6f9c', '#ff7a45', '#2dd4bf'];
	const themes = [
		{ id: 'dark', label: 'Dark', swatch: '#08080f' },
		{ id: 'midnight', label: 'Midnight', swatch: '#050b18' },
		{ id: 'terminal', label: 'Terminal', swatch: '#030503' },
		{ id: 'light', label: 'Light', swatch: '#f4f4f8' },
	];
	$effect(() => {
		document.documentElement.dataset.theme = theme;
		document.documentElement.style.setProperty('--accent', accentColor);
	});

	// --- Cluster ---
	let clusterChoice = $state('standalone'); // standalone | primary | join
	let joinToken = $state('');
	let nodeName = $state('');
	let selfUrl = $state(location.origin);
	let clusterResult = $state('');

	async function run(fn) {
		busy = true;
		error = null;
		try {
			await fn();
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}

	function submitAccount(e) {
		e.preventDefault();
		if (password !== confirm) {
			error = "Passwords don't match";
			return;
		}
		run(async () => {
			user = await api.setup(username, password, codeRequired ? setupCode : undefined);
			api.cluster()
				.then((c) => (nodeName = c.self?.hostname ?? ''))
				.catch(() => {});
			step = 1;
		});
	}

	function submitLook(e) {
		e.preventDefault();
		run(async () => {
			const updated = await api.updateSettings({ appName, accentColor, theme });
			onbrandingchange?.(updated);
			step = 2;
		});
	}

	function submitCluster(e) {
		e.preventDefault();
		run(async () => {
			if (clusterChoice === 'primary') {
				await api.setClusterMode('primary');
				clusterResult = 'This server is a primary. Add servers from the Cluster page.';
			} else if (clusterChoice === 'join') {
				const m = await api.joinCluster(joinToken, nodeName, selfUrl);
				clusterResult =
					m.role === 'replica'
						? `Joined ${m.primaryName} as a replica. Its users replace this server's within a few seconds, so sign in with your account there from now on.`
						: `Joined ${m.primaryName} as a node.`;
			} else {
				clusterResult = 'Running on its own. You can join or create a cluster later from the Cluster page.';
			}
			step = 3;
		});
	}

	function finish() {
		onready?.(user);
	}
</script>

<div class="wrap">
	<div class="center">
		<div class="auth-card">
			<div class="brand">
				<BrandLogo {branding} size={32} />
				<div>
					<h1>Set up {branding?.appName ?? 'ServerDash'}</h1>
					<p class="subtitle">Step {step + 1} of {steps.length} · {steps[step]}</p>
				</div>
			</div>

			<ol class="stepper" aria-label="Setup progress">
				{#each steps as s, i}
					<li class:done={i < step} class:current={i === step}><span>{i < step ? '✓' : i + 1}</span>{s}</li>
				{/each}
			</ol>

			{#if error}<p class="error">{error}</p>{/if}

			{#if step === 0}
				<form onsubmit={submitAccount}>
					<p class="hint">Create the first administrator account.</p>
					{#if codeRequired}
						<label>
							Setup code
							<input bind:value={setupCode} placeholder="XXXX-XXXX" autocomplete="off" required />
							<span class="help">Shown by the installer. Lost it? Run <code>sudo serverdash setup-code</code> on the server.</span>
						</label>
					{/if}
					<label>Username<input bind:value={username} autocomplete="username" required minlength="3" /></label>
					<label>Password<input type="password" bind:value={password} autocomplete="new-password" required minlength="8" /></label>
					<label>Confirm password<input type="password" bind:value={confirm} autocomplete="new-password" required minlength="8" /></label>
					<button class="btn primary" type="submit" disabled={busy}>{busy ? 'Creating…' : 'Create account & continue'}</button>
				</form>
			{:else if step === 1}
				<form onsubmit={submitLook}>
					<p class="hint">Make it yours. Everything here can be changed later under Admin → Appearance.</p>
					<label>Name<input bind:value={appName} maxlength="60" required /></label>
					<label>
						Accent color
						<div class="presets">
							{#each presets as c}
								<button type="button" class="preset" class:on={accentColor === c} style="background: {c}" aria-label="Use {c}" onclick={() => (accentColor = c)}></button>
							{/each}
							<input type="color" bind:value={accentColor} aria-label="Custom color" />
						</div>
					</label>
					<label>
						Theme
						<div class="themes">
							{#each themes as t (t.id)}
								<button type="button" class="theme" class:on={theme === t.id} onclick={() => (theme = t.id)}>
									<span class="swatch" style="background: {t.swatch}"><i style="background: {accentColor}"></i></span>
									{t.label}
								</button>
							{/each}
						</div>
					</label>
					<div class="nav">
						<button type="button" class="btn subtle" onclick={() => (step = 2)}>Skip</button>
						<button class="btn primary" type="submit" disabled={busy}>Continue</button>
					</div>
				</form>
			{:else if step === 2}
				<form onsubmit={submitCluster}>
					<p class="hint">Is this server part of a group? You can change this later on the Cluster page.</p>
					<div class="choices">
						<button type="button" class="choice" class:on={clusterChoice === 'standalone'} onclick={() => (clusterChoice = 'standalone')}>
							<Icon name="Desktop" size={18} /><b>Standalone</b><span>Just this server</span>
						</button>
						<button type="button" class="choice" class:on={clusterChoice === 'primary'} onclick={() => (clusterChoice = 'primary')}>
							<Icon name="Organization" size={18} /><b>Primary</b><span>Manage other servers from here</span>
						</button>
						<button type="button" class="choice" class:on={clusterChoice === 'join'} onclick={() => (clusterChoice = 'join')}>
							<Icon name="Connect" size={18} /><b>Join a cluster</b><span>Become a node or replica of a primary</span>
						</button>
					</div>
					{#if clusterChoice === 'join'}
						<label>
							Join token
							<input bind:value={joinToken} placeholder="sdj1.…" required />
							<span class="help">On the primary: Cluster → Add a server → Create token.</span>
						</label>
						<label>Name for this server<input bind:value={nodeName} maxlength="60" /></label>
						<label>
							This server's address
							<input bind:value={selfUrl} />
							<span class="help">How the primary reaches this server. Leave it blank if it can't; the server still reports its status.</span>
						</label>
					{/if}
					<div class="nav">
						<button type="button" class="btn subtle" onclick={() => (step = 1)}>Back</button>
						<button class="btn primary" type="submit" disabled={busy}>{busy ? 'Working…' : 'Continue'}</button>
					</div>
				</form>
			{:else}
				<div class="done">
					<Icon name="CheckCircle" size={40} />
					<h2>You're all set</h2>
					<p class="hint">{clusterResult}</p>
					<button class="btn primary" onclick={finish}>Open the dashboard</button>
				</div>
			{/if}
		</div>
	</div>
	<footer>ServerDash by Blacklink Enterprise. ©{new Date().getFullYear()} Blacklink, Inc. All Rights Reserved.</footer>
</div>

<style>
	.wrap {
		min-height: 100vh;
		display: flex;
		flex-direction: column;
		background:
			radial-gradient(90% 60% at 50% 0%, var(--accent-glow), transparent 60%),
			var(--bg);
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
		width: min(480px, 100%);
		background: var(--surface);
		border: 1px solid var(--border2);
		border-radius: 18px;
		padding: 30px 30px 26px;
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
		margin: 2px 0 0;
		color: var(--muted);
		font-size: 12px;
	}
	.stepper {
		display: flex;
		gap: 6px;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.stepper li {
		flex: 1;
		display: flex;
		align-items: center;
		gap: 6px;
		padding-top: 8px;
		border-top: 3px solid var(--sur3);
		font-size: 10px;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}
	.stepper li span {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		border-radius: 50%;
		background: var(--sur3);
		font-size: 10px;
	}
	.stepper li.done,
	.stepper li.current {
		border-top-color: var(--accent);
		color: var(--text);
	}
	.stepper li.done span,
	.stepper li.current span {
		background: var(--accent);
		color: #1f1400;
	}
	form {
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.hint {
		margin: 0;
		color: var(--muted-strong);
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
	.help {
		text-transform: none;
		letter-spacing: normal;
		font-size: 11px;
	}
	.error {
		margin: 0;
		color: var(--danger);
		font-size: 13px;
	}
	.nav {
		display: flex;
		justify-content: space-between;
		gap: 8px;
	}
	.presets {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.preset {
		width: 24px;
		height: 24px;
		border-radius: 50%;
		border: 2px solid var(--surface);
		box-shadow: 0 0 0 1px var(--border2);
		padding: 0;
	}
	.preset.on {
		box-shadow: 0 0 0 2px var(--text);
	}
	.presets input[type='color'] {
		width: 34px;
		height: 28px;
		padding: 2px;
	}
	.themes {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}
	.theme {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 6px;
		padding: 8px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--sur2);
		color: var(--text);
		font-size: 11px;
		text-transform: none;
		letter-spacing: normal;
	}
	.theme.on {
		border-color: var(--accent);
	}
	.swatch {
		position: relative;
		display: block;
		width: 72px;
		height: 40px;
		border-radius: 8px;
		border: 1px solid var(--border2);
	}
	.swatch i {
		position: absolute;
		left: 8px;
		bottom: 8px;
		width: 28px;
		height: 5px;
		border-radius: 3px;
	}
	.choices {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.choice {
		display: grid;
		grid-template-columns: 26px auto 1fr;
		align-items: center;
		gap: 4px 10px;
		text-align: left;
		padding: 12px 14px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--sur2);
		color: var(--text);
	}
	.choice span {
		color: var(--muted);
		font-size: 11px;
		justify-self: end;
	}
	.choice.on {
		border-color: var(--accent);
		background: var(--accent-soft);
	}
	.done {
		display: flex;
		flex-direction: column;
		align-items: center;
		text-align: center;
		gap: 10px;
		padding: 10px 0;
		color: var(--success);
	}
	.done h2 {
		margin: 0;
		color: var(--text);
	}
</style>
