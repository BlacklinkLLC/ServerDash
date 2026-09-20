<script>
	let { name, nickname, editable, onsave, ondelete } = $props();

	let editing = $state(false);
	let draft = $state('');

	function startEdit() {
		draft = nickname ?? '';
		editing = true;
	}

	async function save() {
		const trimmed = draft.trim();
		if (trimmed === '') {
			if (nickname) await ondelete?.();
		} else {
			await onsave?.(trimmed);
		}
		editing = false;
	}
</script>

{#if editing}
	<form class="edit" onsubmit={(e) => (e.preventDefault(), save())}>
		<input bind:value={draft} placeholder={name} autofocus onblur={save} />
	</form>
{:else}
	<span class="label">
		{#if nickname}
			<span class="nickname">{nickname}</span>
			<span class="real-name">{name}</span>
		{:else}
			<span class="real-name primary">{name}</span>
		{/if}
		{#if editable}
			<button class="edit-btn" onclick={startEdit} aria-label={`Rename ${name}`}>✎</button>
		{/if}
	</span>
{/if}

<style>
	.label {
		display: inline-flex;
		align-items: baseline;
		gap: 6px;
	}
	.nickname {
		font-weight: 600;
	}
	.real-name {
		color: var(--text-muted);
		font-size: 11px;
	}
	.real-name.primary {
		color: var(--text-primary);
		font-size: inherit;
		font-weight: 400;
	}
	.edit-btn {
		background: none;
		border: none;
		color: var(--text-muted);
		font-size: 11px;
		padding: 0 2px;
		opacity: 0;
	}
	.label:hover .edit-btn {
		opacity: 1;
	}
	.edit input {
		font: inherit;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 4px;
		padding: 2px 6px;
		color: var(--text-primary);
		width: 160px;
	}
</style>
