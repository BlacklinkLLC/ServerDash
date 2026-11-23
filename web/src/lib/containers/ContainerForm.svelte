<script>
	let {
		initial = null,
		submitLabel = 'Create',
		managed = false,
		busy = false,
		onsubmit,
		oncancel,
	} = $props();

	let name = $state(initial?.name ?? '');
	let image = $state(initial?.image ?? '');
	let envText = $state((initial?.env ?? []).join('\n'));
	let volumesText = $state((initial?.volumes ?? []).join('\n'));
	let commandText = $state((initial?.command ?? []).join(' '));
	let restartPolicy = $state(initial?.restartPolicy ?? 'unless-stopped');
	let ports = $state(
		(initial?.ports ?? []).length
			? (initial.ports ?? []).map((p) => ({ ...p }))
			: [{ hostPort: '', containerPort: '', protocol: 'tcp' }],
	);
	let error = $state(null);

	function addPort() {
		ports = [...ports, { hostPort: '', containerPort: '', protocol: 'tcp' }];
	}

	function removePort(i) {
		ports = ports.filter((_, index) => index !== i);
	}

	async function submit(e) {
		e.preventDefault();
		error = null;
		const spec = {
			name: name.trim(),
			image: image.trim(),
			env: envText.split('\n').map((v) => v.trim()).filter(Boolean),
			volumes: volumesText.split('\n').map((v) => v.trim()).filter(Boolean),
			command: commandText.trim() ? commandText.trim().split(/\s+/) : [],
			restartPolicy,
			ports: ports
				.filter((p) => p.hostPort && p.containerPort)
				.map((p) => ({
					hostPort: String(p.hostPort).trim(),
					containerPort: String(p.containerPort).trim(),
					protocol: p.protocol || 'tcp',
				})),
		};
		try {
			await onsubmit?.(spec);
		} catch (e) {
			error = e.message;
		}
	}
</script>

<form onsubmit={submit}>
	{#if !managed}
		<p class="hint">Docker/Podman will create and start this container immediately.</p>
	{:else}
		<p class="hint">ServerDash stores this definition and deploys it on demand.</p>
	{/if}
	<div class="grid">
		<label class="field">
			Name
			<input bind:value={name} maxlength="64" required placeholder={managed ? 'my-service' : 'web'} />
		</label>
		<label class="field">
			Image
			<input bind:value={image} maxlength="255" required placeholder="nginx:latest" />
		</label>
		<label class="field">
			Restart policy
			<select bind:value={restartPolicy}>
				<option value="unless-stopped">Unless stopped</option>
				<option value="always">Always</option>
				<option value="on-failure">On failure</option>
				<option value="no">Never</option>
			</select>
		</label>
		<label class="field">
			Command (optional)
			<input bind:value={commandText} placeholder="--config /etc/app.conf" />
		</label>
	</div>

	<h4>Ports</h4>
	{#each ports as port, i (i)}
		<div class="row">
			<label class="field">
				Host
				<input type="number" min="1" max="65535" bind:value={port.hostPort} />
			</label>
			<label class="field">
				Container
				<input type="number" min="1" max="65535" bind:value={port.containerPort} />
			</label>
			<label class="field">
				Protocol
				<select bind:value={port.protocol}>
					<option value="tcp">TCP</option>
					<option value="udp">UDP</option>
				</select>
			</label>
			<button type="button" class="btn subtle" onclick={() => removePort(i)}>Remove</button>
		</div>
	{/each}
	<button type="button" class="btn subtle" onclick={addPort}>Add port</button>

	<div class="grid">
		<label class="field">
			Environment (one KEY=value per line)
			<textarea bind:value={envText} rows="3" placeholder="PORT=8080"></textarea>
		</label>
		<label class="field">
			Volumes (one /host:/container[:ro] per line)
			<textarea bind:value={volumesText} rows="3" placeholder="/srv/data:/data"></textarea>
		</label>
	</div>

	{#if error}
		<p class="error">{error}</p>
	{/if}
	<div class="form-actions">
		<button class="btn primary" disabled={busy} type="submit">{busy ? 'Working…' : submitLabel}</button>
		{#if oncancel}
			<button class="btn subtle" type="button" onclick={oncancel}>Cancel</button>
		{/if}
	</div>
</form>

<style>
	h4 {
		margin: 18px 0 8px;
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: var(--muted);
	}
	.hint,
	.error {
		font-size: 12px;
	}
	.hint {
		color: var(--muted);
	}
	.error {
		color: var(--danger);
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
		gap: 12px;
	}
	.row {
		display: grid;
		grid-template-columns: repeat(3, minmax(100px, 1fr)) auto;
		gap: 10px;
		align-items: end;
		margin-bottom: 8px;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 5px;
		font-size: 12px;
	}
	.field input,
	.field select,
	.field textarea {
		width: 100%;
		padding: 8px 10px;
		font-size: 12px;
		font-family: var(--font-sans);
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--sur2);
		color: var(--text);
	}
	.field textarea {
		resize: vertical;
	}
	.form-actions {
		display: flex;
		gap: 8px;
		margin-top: 16px;
	}
	@media (max-width: 640px) {
		.row {
			grid-template-columns: 1fr 1fr;
		}
	}
</style>
