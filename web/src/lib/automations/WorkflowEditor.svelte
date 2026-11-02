<script>
	import { untrack } from 'svelte';
	import Icon from '../Icon.svelte';
	import { api } from '../api.js';
	import { TRIGGERS, BLOCKS, CHECKS, VARIABLES, blankTrigger, blankBlock, blockMeta } from './catalog.js';

	let { workflow, containers = [], scripts = [], plugins = [], onsave, oncancel } = $props();

	const TIMEZONES = [
		'UTC', 'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles', 'America/Anchorage',
		'Europe/London', 'Europe/Berlin', 'Asia/Tokyo', 'Asia/Shanghai', 'Asia/Kolkata', 'Australia/Sydney',
	];
	const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

	// Edited copy; the parent's object is never mutated until save.
	let wf = $state(untrack(() => JSON.parse(JSON.stringify(workflow))));
	let error = $state(null);
	let saving = $state(false);
	let showPalette = $state(false);
	let showVars = $state(false);
	let expanded = $state(wf.blocks.length <= 3 ? wf.blocks.map((_, i) => i) : []);

	let groups = $derived([...new Set(BLOCKS.map((b) => b.group))]);
	let tzs = $derived(TIMEZONES.includes(wf.trigger.tz) || !wf.trigger.tz ? TIMEZONES : [wf.trigger.tz, ...TIMEZONES]);

	function setTrigger(type) {
		if (wf.trigger.type === type) return;
		// Keep the webhook token if switching away and back.
		const token = wf.trigger.token;
		wf.trigger = { ...blankTrigger(type), ...(type === 'webhook' && token ? { token } : {}) };
	}

	function toggleDay(d) {
		const days = new Set(wf.trigger.days ?? []);
		days.has(d) ? days.delete(d) : days.add(d);
		wf.trigger.days = [...days].sort();
	}

	function addBlock(type) {
		wf.blocks = [...wf.blocks, blankBlock(type)];
		expanded = [...expanded, wf.blocks.length - 1];
		showPalette = false;
	}
	function removeBlock(i) {
		wf.blocks = wf.blocks.filter((_, j) => j !== i);
		expanded = expanded.filter((j) => j !== i).map((j) => (j > i ? j - 1 : j));
	}
	function duplicateBlock(i) {
		const copy = JSON.parse(JSON.stringify(wf.blocks[i]));
		wf.blocks = [...wf.blocks.slice(0, i + 1), copy, ...wf.blocks.slice(i + 1)];
		expanded = [...expanded.map((j) => (j > i ? j + 1 : j)), i + 1];
	}
	function move(i, d) {
		const j = i + d;
		if (j < 0 || j >= wf.blocks.length) return;
		const b = [...wf.blocks];
		[b[i], b[j]] = [b[j], b[i]];
		wf.blocks = b;
		expanded = expanded.map((k) => (k === i ? j : k === j ? i : k));
	}
	function toggle(i) {
		expanded = expanded.includes(i) ? expanded.filter((j) => j !== i) : [...expanded, i];
	}

	function summary(b) {
		switch (b.type) {
			case 'check': {
				const c = CHECKS.find(([k]) => k === b.check);
				const v = c?.[2] === 'threshold' ? `${b.threshold ?? ''}%` : (b[c?.[2]] ?? '');
				return `${c?.[1] ?? b.check} ${v}`.trim() + (b.onFalse === 'fail' ? ' · else fail' : ' · else stop');
			}
			case 'wait':
				return `${b.seconds}s`;
			case 'notify':
				return `${b.format ?? 'json'} · ${b.message ?? ''}`;
			case 'http_request':
				return `${b.method ?? 'GET'} ${b.url ?? ''}`;
			case 'shell':
				return b.command ?? '';
			case 'run_script':
				return scripts.find((s) => s.id === b.scriptId)?.name ?? 'choose a script';
			case 'service_action':
				return `${b.action} ${b.service ?? ''}`;
			case 'git_pull':
			case 'compose_up':
				return b.dir ?? '';
			default:
				return b.container ?? '';
		}
	}

	let webhookUrl = $derived(wf.trigger.type === 'webhook' && wf.trigger.token ? api.workflowHookUrl(wf.trigger.token) : null);
	let copied = $state(false);
	async function copyHook() {
		await navigator.clipboard.writeText(webhookUrl);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	async function submit(e) {
		e.preventDefault();
		saving = true;
		error = null;
		try {
			const payload = {
				name: wf.name,
				enabled: wf.enabled,
				trigger: wf.trigger,
				onFailure: wf.onFailure?.url ? wf.onFailure : {},
				blocks: wf.blocks,
			};
			await onsave?.(payload);
		} catch (e) {
			error = e.message;
		} finally {
			saving = false;
		}
	}
</script>

<form class="editor" onsubmit={submit}>
	<div class="top">
		<input class="name" bind:value={wf.name} placeholder="Name this workflow" required maxlength="100" />
		<label class="switch"><input type="checkbox" bind:checked={wf.enabled} /> Enabled</label>
	</div>

	{#if error}<p class="error">{error}</p>{/if}

	<!-- Trigger -->
	<section>
		<h4><span class="step">1</span> When</h4>
		<div class="triggers">
			{#each TRIGGERS as t (t.type)}
				<button type="button" class="trig" class:on={wf.trigger.type === t.type} onclick={() => setTrigger(t.type)}>
					<Icon name={t.icon} size={16} />
					<b>{t.label}</b>
					<span>{t.hint}</span>
				</button>
			{/each}
		</div>

		<div class="trig-config">
			{#if wf.trigger.type === 'daily' || wf.trigger.type === 'weekly'}
				{#if wf.trigger.type === 'weekly'}
					<div class="days">
						{#each DAYS as d, i}
							<button type="button" class="day" class:on={wf.trigger.days?.includes(i)} onclick={() => toggleDay(i)}>{d}</button>
						{/each}
					</div>
				{/if}
				<label class="f">At <input type="time" bind:value={wf.trigger.time} required /></label>
				<label class="f">Timezone
					<select bind:value={wf.trigger.tz}>
						{#each tzs as tz}<option value={tz}>{tz}</option>{/each}
					</select>
				</label>
			{:else if wf.trigger.type === 'interval'}
				<label class="f">Every <input type="number" min="1" max="10080" bind:value={wf.trigger.minutes} class="short" /> minutes</label>
				<div class="quick">
					{#each [[5, '5m'], [15, '15m'], [60, '1h'], [360, '6h'], [1440, '1d']] as [m, l]}
						<button type="button" class="chip" class:on={wf.trigger.minutes === m} onclick={() => (wf.trigger.minutes = m)}>{l}</button>
					{/each}
				</div>
			{:else if wf.trigger.type === 'cron'}
				<label class="f grow">Cron expression <input class="mono" bind:value={wf.trigger.cron} placeholder="*/30 * * * *" required /></label>
				<span class="hint">minute hour day month weekday · prefix with <code>CRON_TZ=Europe/London</code> for a timezone</span>
			{:else if wf.trigger.type === 'metric'}
				<label class="f">When
					<select bind:value={wf.trigger.metric}>
						<option value="cpu">CPU</option>
						<option value="mem">Memory</option>
						<option value="disk">Fullest disk</option>
						<option value="load">Load average</option>
					</select>
				</label>
				<label class="f">is above <input type="number" step="any" min="0" bind:value={wf.trigger.threshold} class="short" />{wf.trigger.metric === 'load' ? '' : '%'}</label>
				<label class="f">for <input type="number" min="0" max="1440" bind:value={wf.trigger.forMinutes} class="short" /> min</label>
			{:else if wf.trigger.type === 'container'}
				<label class="f">When a container named <input bind:value={wf.trigger.pattern} list="ctr-names" placeholder="* or web-*" class="mid" /></label>
				<label class="f">
					<select bind:value={wf.trigger.event}>
						<option value="either">stops or turns unhealthy</option>
						<option value="stopped">stops</option>
						<option value="unhealthy">turns unhealthy</option>
					</select>
				</label>
			{:else if wf.trigger.type === 'service'}
				<label class="f">When a systemd service named <input bind:value={wf.trigger.pattern} placeholder="nginx or *" class="mid" /> fails</label>
				<span class="hint">Needs a native install.</span>
			{:else if wf.trigger.type === 'webhook'}
				{#if webhookUrl}
					<div class="hook">
						<code>POST {webhookUrl}</code>
						<button type="button" class="btn subtle" onclick={copyHook}><Icon name="Copy" size={13} /> {copied ? 'Copied' : 'Copy'}</button>
					</div>
					<span class="hint">Anyone with this URL can run the workflow. The request body is available as <code>{'{{payload}}'}</code>.</span>
				{:else}
					<span class="hint">A secret webhook URL is created when you save.</span>
				{/if}
			{:else if wf.trigger.type === 'manual'}
				<span class="hint">Runs only when you press Run now (or from another workflow's webhook).</span>
			{/if}

			{#if ['metric', 'container', 'service'].includes(wf.trigger.type)}
				<label class="f">Then wait at least <input type="number" min="0" max="10080" bind:value={wf.trigger.cooldownMinutes} class="short" /> min before firing again</label>
			{/if}
		</div>
	</section>

	<!-- Blocks -->
	<section>
		<h4>
			<span class="step">2</span> Do
			<button type="button" class="link" onclick={() => (showVars = !showVars)}>{showVars ? 'Hide' : 'Show'} variables</button>
		</h4>
		{#if showVars}
			<div class="vars">
				{#each VARIABLES as [v, d]}<div><code>{v}</code><span>{d}</span></div>{/each}
				<p class="hint">In Run command blocks, variables are safely quoted, and also available as <code>$SERVERDASH_EVENT</code>, <code>$SERVERDASH_PAYLOAD</code>, <code>$SERVERDASH_OUTPUT</code>.</p>
			</div>
		{/if}

		<ol class="blocks">
			{#each wf.blocks as b, i (i)}
				{@const meta = blockMeta(b.type)}
				<li class="block" class:open={expanded.includes(i)} class:cond={b.type === 'check'}>
					<div class="bhead">
						<span class="num">{i + 1}</span>
						<button type="button" class="btitle" onclick={() => toggle(i)}>
							<Icon name={meta.icon} size={15} />
							<b>{b.label || meta.label}</b>
							<span class="bsum">{summary(b)}</span>
						</button>
						<span class="btools">
							<button type="button" title="Move up" disabled={i === 0} onclick={() => move(i, -1)}>↑</button>
							<button type="button" title="Move down" disabled={i === wf.blocks.length - 1} onclick={() => move(i, 1)}>↓</button>
							<button type="button" title="Duplicate" onclick={() => duplicateBlock(i)}><Icon name="Copy" size={12} /></button>
							<button type="button" title="Remove" onclick={() => removeBlock(i)}><Icon name="X" size={12} /></button>
						</span>
					</div>
					{#if expanded.includes(i)}
						<div class="bbody">
							{#if b.type === 'check'}
								<div class="row">
									<label class="f">Only continue if
										<select bind:value={b.check}>
											{#each CHECKS as [k, l]}<option value={k}>{l}</option>{/each}
										</select>
									</label>
									{#if CHECKS.find(([k]) => k === b.check)?.[2] === 'threshold'}
										<label class="f"><input type="number" min="0" max="100" bind:value={b.threshold} class="short" />%</label>
									{:else if b.check?.startsWith('container')}
										<label class="f grow"><input bind:value={b.container} list="ctr-names" placeholder="container name" required /></label>
									{:else if b.check === 'http_ok'}
										<label class="f grow"><input bind:value={b.url} placeholder="https://example.com/health" required /></label>
									{:else if b.check === 'command_ok'}
										<label class="f grow"><input class="mono" bind:value={b.command} placeholder="test -f /tmp/flag" required /></label>
									{:else if b.check === 'output_contains'}
										<label class="f grow"><input bind:value={b.text} placeholder="text to look for" required /></label>
									{/if}
								</div>
								<label class="f">Otherwise
									<select bind:value={b.onFalse}>
										<option value="stop">stop here (counts as success)</option>
										<option value="fail">fail the run (sends the failure alert)</option>
									</select>
								</label>
							{:else}
								{#each meta.fields ?? [] as f (f.key)}
									{#if f.kind === 'checkbox'}
										<label class="cb"><input type="checkbox" bind:checked={b[f.key]} /> {f.label}</label>
									{:else}
										<label class="f full">{f.label}
											{#if f.kind === 'select'}
												<select bind:value={b[f.key]}>{#each f.options as [v, l]}<option value={v}>{l}</option>{/each}</select>
											{:else if f.kind === 'number'}
												<input type="number" min={f.min} max={f.max} bind:value={b[f.key]} class="short" />
											{:else if f.kind === 'textarea'}
												<textarea rows="3" bind:value={b[f.key]} placeholder={f.placeholder}></textarea>
											{:else if f.kind === 'code'}
												<textarea rows="3" class="mono" bind:value={b[f.key]} placeholder={f.placeholder} spellcheck="false"></textarea>
											{:else if f.kind === 'container'}
												<input bind:value={b[f.key]} list="ctr-names" placeholder="name or ID" />
											{:else if f.kind === 'plugin'}
												<select bind:value={b[f.key]} onchange={() => (b.pluginAction = undefined)}>
													<option value={undefined} disabled>Choose a plugin…</option>
													{#each plugins.filter((p) => p.actions?.length) as p (p.id)}<option value={p.id}>{p.name}{p.enabled ? '' : ' (disabled)'}</option>{/each}
												</select>
											{:else if f.kind === 'pluginAction'}
												<select bind:value={b[f.key]} disabled={!b.plugin}>
													<option value={undefined} disabled>Choose an action…</option>
													{#each plugins.find((p) => p.id === b.plugin)?.actions ?? [] as a (a.id)}<option value={a.id}>{a.label}</option>{/each}
												</select>
											{:else if f.kind === 'script'}
												<select bind:value={b[f.key]}>
													<option value={undefined} disabled>Choose a script…</option>
													{#each scripts as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
												</select>
											{:else}
												<input bind:value={b[f.key]} placeholder={f.placeholder} />
											{/if}
										</label>
									{/if}
								{/each}
							{/if}
							<div class="bopts">
								<label class="cb"><input type="checkbox" bind:checked={b.continueOnError} /> Keep going if this step fails</label>
								<input class="lbl" bind:value={b.label} placeholder="Custom step name (optional)" />
							</div>
						</div>
					{/if}
				</li>
			{:else}
				<li class="empty">No steps yet. Add one below.</li>
			{/each}
		</ol>

		{#if showPalette}
			<div class="palette">
				{#each groups as g}
					<div class="pgroup">
						<span class="pg">{g}</span>
						{#each BLOCKS.filter((b) => b.group === g) as b (b.type)}
							<button type="button" class="pitem" onclick={() => addBlock(b.type)}><Icon name={b.icon} size={14} /> {b.label}</button>
						{/each}
					</div>
				{/each}
			</div>
		{/if}
		<button type="button" class="btn subtle add" onclick={() => (showPalette = !showPalette)}>{showPalette ? 'Close' : '+ Add a step'}</button>
	</section>

	<!-- On failure -->
	<section>
		<h4><span class="step">3</span> If it fails</h4>
		<div class="row">
			<label class="f">Alert via
				<select bind:value={wf.onFailure.format}>
					<option value="discord">Discord</option>
					<option value="slack">Slack</option>
					<option value="ntfy">ntfy</option>
					<option value="json">Generic JSON</option>
				</select>
			</label>
			<label class="f grow">Webhook URL <span class="muted">(blank = no alert)</span>
				<input bind:value={wf.onFailure.url} placeholder="https://discord.com/api/webhooks/…" />
			</label>
		</div>
	</section>

	<datalist id="ctr-names">
		{#each containers as c}<option value={c}></option>{/each}
	</datalist>

	<div class="actions">
		<button type="button" class="btn subtle" onclick={() => oncancel?.()}>Cancel</button>
		<button type="submit" class="btn primary" disabled={saving || !wf.blocks.length}>{saving ? 'Saving…' : 'Save workflow'}</button>
	</div>
</form>

<style>
	.editor {
		display: flex;
		flex-direction: column;
		gap: 18px;
	}
	.top {
		display: flex;
		gap: 12px;
		align-items: center;
	}
	.name {
		flex: 1;
		font-size: 16px;
		font-family: var(--font-display);
		font-weight: 700;
	}
	.switch,
	.cb {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
	}
	.error {
		color: var(--danger);
		margin: 0;
	}
	section {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	h4 {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 0;
		font-size: 13px;
	}
	.step {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		background: var(--accent);
		color: #1f1400;
		font-size: 11px;
	}
	.link {
		margin-left: auto;
		background: none;
		border: none;
		color: var(--accent);
		font-size: 11px;
		padding: 0;
	}
	.triggers {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
		gap: 8px;
	}
	.trig {
		display: grid;
		grid-template-columns: 18px 1fr;
		gap: 2px 8px;
		align-items: center;
		text-align: left;
		padding: 10px 12px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--sur2);
		color: var(--text);
		font-size: 12px;
	}
	.trig span {
		grid-column: 2;
		color: var(--muted);
		font-size: 10.5px;
	}
	.trig.on {
		border-color: var(--accent);
		background: var(--accent-soft);
	}
	.trig-config {
		display: flex;
		flex-wrap: wrap;
		gap: 10px 16px;
		align-items: center;
		padding: 12px 14px;
		border-radius: 12px;
		background: var(--sur2);
		border: 1px solid var(--border);
	}
	.f {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--muted-strong);
		flex-wrap: wrap;
	}
	.f.full {
		display: flex;
		flex-direction: column;
		align-items: stretch;
		gap: 4px;
		color: var(--muted);
		font-size: 11px;
	}
	.grow {
		flex: 1;
		min-width: 220px;
	}
	.grow input {
		flex: 1;
	}
	.short {
		width: 90px;
	}
	.mid {
		width: 160px;
	}
	.mono {
		font-family: var(--font-mono);
	}
	.hint,
	.muted {
		color: var(--muted);
		font-size: 11px;
	}
	.days,
	.quick {
		display: flex;
		gap: 4px;
	}
	.day,
	.chip {
		padding: 6px 10px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: var(--surface);
		color: var(--muted-strong);
		font-size: 11px;
	}
	.day.on,
	.chip.on {
		border-color: var(--accent);
		color: var(--accent);
		background: var(--accent-soft);
	}
	.hook {
		display: flex;
		gap: 8px;
		align-items: center;
		width: 100%;
	}
	.hook code {
		flex: 1;
		padding: 8px 10px;
		border-radius: 8px;
		background: var(--bg);
		border: 1px solid var(--border-strong);
		color: var(--accent);
		font-size: 11px;
		word-break: break-all;
	}
	.hook .btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}
	.vars {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
		gap: 6px 16px;
		padding: 12px 14px;
		border-radius: 12px;
		background: var(--sur2);
		font-size: 11px;
	}
	.vars div {
		display: flex;
		gap: 8px;
	}
	.vars code {
		color: var(--accent);
	}
	.vars span {
		color: var(--muted);
	}
	.vars .hint {
		grid-column: 1 / -1;
		margin: 4px 0 0;
	}
	.blocks {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0;
	}
	.block {
		position: relative;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--surface);
		margin-bottom: 14px;
	}
	/* connector line between steps */
	.block:not(:last-child)::after {
		content: '';
		position: absolute;
		left: 23px;
		bottom: -15px;
		width: 2px;
		height: 14px;
		background: var(--border2);
	}
	.block.cond {
		border-style: dashed;
		border-color: var(--border-strong);
	}
	.block.open {
		border-color: var(--border2);
	}
	.bhead {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
	}
	.num {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		border-radius: 8px;
		background: var(--sur3);
		font-size: 11px;
		flex-shrink: 0;
	}
	.btitle {
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: 8px;
		background: none;
		border: none;
		color: var(--text);
		text-align: left;
		font-size: 12.5px;
		padding: 4px 0;
	}
	.bsum {
		color: var(--muted);
		font-size: 11px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.btools {
		display: flex;
		gap: 2px;
	}
	.btools button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		border-radius: 7px;
		border: 1px solid transparent;
		background: none;
		color: var(--muted);
	}
	.btools button:hover:not(:disabled) {
		border-color: var(--border);
		color: var(--text);
	}
	.btools button:disabled {
		opacity: 0.3;
	}
	.bbody {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 4px 14px 14px 46px;
	}
	.row {
		display: flex;
		gap: 10px 14px;
		flex-wrap: wrap;
		align-items: center;
	}
	.bopts {
		display: flex;
		gap: 12px;
		align-items: center;
		flex-wrap: wrap;
		padding-top: 8px;
		border-top: 1px solid var(--border);
	}
	.lbl {
		flex: 1;
		min-width: 180px;
		padding: 7px 10px;
		font-size: 11px;
	}
	.empty {
		padding: 16px;
		text-align: center;
		color: var(--muted);
		border: 1px dashed var(--border2);
		border-radius: 12px;
		margin-bottom: 12px;
	}
	.palette {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
		gap: 12px;
		padding: 14px;
		border-radius: 12px;
		border: 1px solid var(--border-strong);
		background: var(--sur2);
	}
	.pgroup {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.pg {
		font-size: 10px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--muted);
		margin-bottom: 2px;
	}
	.pitem {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 7px 9px;
		border-radius: 8px;
		border: 1px solid transparent;
		background: none;
		color: var(--text);
		font-size: 12px;
		text-align: left;
	}
	.pitem:hover {
		border-color: var(--border-strong);
		background: var(--accent-soft);
	}
	.add {
		align-self: flex-start;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}
</style>
