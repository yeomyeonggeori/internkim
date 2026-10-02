<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import AtSignIcon from '@lucide/svelte/icons/at-sign';
	import type { MentionCandidate } from '$lib/messenger/mention-candidates';

	let {
		name,
		rows,
		active,
		listLabel,
		everyoneLabel,
		onPick
	}: {
		name: string;
		rows: MentionCandidate[];
		active: number;
		listLabel: string;
		everyoneLabel: string;
		onPick: (candidate: MentionCandidate) => void;
	} = $props();

	$effect(() => {
		document.getElementById(`mention-row-${name}-${active}`)?.scrollIntoView({ block: 'nearest' });
	});
</script>

<ul
	id={`mention-list-${name}`}
	role="listbox"
	class="bg-popover text-popover-foreground absolute bottom-full left-3 z-30 mb-2 max-h-56 w-64 overflow-y-auto rounded-md border p-1 shadow-md"
	aria-label={listLabel}
>
	{#each rows as row, index (row.key)}
		<li
			id={`mention-row-${name}-${index}`}
			role="option"
			aria-selected={index === active}
			class={`flex items-center gap-2 rounded-sm px-2 py-1.5 text-sm max-sm:min-h-11 ${
				index === active ? 'bg-accent text-accent-foreground' : ''
			}`}
			onmousedown={(event) => {
				event.preventDefault();
				onPick(row);
			}}
		>
			{#if row.isEveryone}
				<span class="bg-muted text-muted-foreground flex size-6 items-center justify-center rounded-full">
					<AtSignIcon class="size-3.5" />
				</span>
				<span class="truncate">{row.label}</span>
				<span class="text-muted-foreground ms-auto truncate text-xs">{everyoneLabel}</span>
			{:else}
				<PersonAvatar
					class="size-6"
					name={row.label}
					externalID={row.person?.externalID ?? ''}
					image={row.person?.avatarURL ?? ''}
				/>
				<span class="truncate">{row.label}</span>
			{/if}
		</li>
	{/each}
</ul>
