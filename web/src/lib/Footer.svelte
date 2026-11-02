<script>
	import Icon from './Icon.svelte';

	let { branding } = $props();

	const year = new Date().getFullYear();
	const links = [
		{ href: 'https://blacklink.net', label: 'Blacklink', icon: 'Globe' },
		{ href: 'https://nova.blacklink.net', label: 'NOVA', icon: 'Brush' },
		{ href: 'https://cdn.blacklink.net', label: 'CDN', icon: 'Cloud' },
	];
</script>

<footer>
	<div class="glow" aria-hidden="true"></div>
	<div class="inner">
		<div class="brand">
			<a class="mark" href="https://blacklink.net" target="_blank" rel="noopener noreferrer" aria-label="Blacklink">
				<Icon name="BlacklinkLogo" variant="color" size={26} />
				<span class="wordmark"><Icon name="BlacklinkWordmark" size={110} /></span>
			</a>
			<p class="tagline">
				<span class="product">{branding?.appName ?? 'ServerDash'}</span>
				<span class="dot">·</span>
				Built on <b>Blacklink Enterprise</b>
			</p>
		</div>

		<nav class="links" aria-label="Blacklink">
			{#each links as l (l.href)}
				<a href={l.href} target="_blank" rel="noopener noreferrer">
					<Icon name={l.icon} size={13} />
					{l.label}
				</a>
			{/each}
		</nav>

		<div class="badge">
			<Icon name="Shield" size={13} />
			Enterprise
		</div>
	</div>
	<p class="legal">
		{branding?.footerText || `ServerDash by Blacklink Enterprise. ©${year} Blacklink, Inc. All Rights Reserved.`}
	</p>
</footer>

<style>
	footer {
		position: relative;
		margin-top: 32px;
		padding: 28px 24px 22px;
		border-top: 1px solid var(--border);
		background: linear-gradient(180deg, var(--surface), var(--bg));
		overflow: hidden;
	}
	.glow {
		position: absolute;
		top: -1px;
		left: 50%;
		width: min(560px, 80%);
		height: 1px;
		transform: translateX(-50%);
		background: linear-gradient(90deg, transparent, var(--accent), transparent);
		box-shadow: 0 0 24px 2px var(--accent-glow);
	}
	.inner {
		max-width: 1180px;
		margin: 0 auto;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 20px;
		flex-wrap: wrap;
	}
	.brand {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.mark {
		display: inline-flex;
		align-items: center;
		gap: 12px;
		color: var(--text);
		text-decoration: none;
	}
	.wordmark {
		display: inline-flex;
		height: 8px;
		align-items: center;
	}
	.wordmark :global(.icon) {
		height: 8px !important;
	}
	.tagline {
		margin: 0;
		font-size: 11px;
		color: var(--muted);
	}
	.tagline b {
		color: var(--muted-strong);
		font-weight: 500;
	}
	.product {
		color: var(--accent);
	}
	.dot {
		margin: 0 4px;
	}
	.links {
		display: flex;
		gap: 6px;
		flex-wrap: wrap;
	}
	.links a {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 7px 12px;
		border-radius: 999px;
		border: 1px solid var(--border);
		background: var(--sur2);
		color: var(--muted-strong);
		font-size: 11px;
		text-decoration: none;
		transition:
			border-color 0.15s ease,
			color 0.15s ease;
	}
	.links a:hover {
		border-color: var(--border-strong);
		color: var(--accent);
	}
	.badge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 12px;
		border-radius: 999px;
		background: var(--accent-soft);
		border: 1px solid var(--border-strong);
		color: var(--accent);
		font-size: 10px;
		letter-spacing: 0.1em;
		text-transform: uppercase;
	}
	.legal {
		max-width: 1180px;
		margin: 20px auto 0;
		padding-top: 14px;
		border-top: 1px solid var(--border);
		text-align: center;
		font-size: 10px;
		letter-spacing: 0.04em;
		color: var(--muted);
	}
	@media (max-width: 640px) {
		.inner {
			flex-direction: column;
			align-items: flex-start;
		}
	}
</style>
