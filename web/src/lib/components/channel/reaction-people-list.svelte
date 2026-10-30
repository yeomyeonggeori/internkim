<script lang="ts">
	import * as Tabs from '$lib/components/ui/tabs';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ReactionGlyph from './reaction-glyph.svelte';
	import type { ChannelMessageReaction, ChannelParticipant } from './channel-api';

	let {
		reactions,
		selected = $bindable('')
	}: {
		reactions: ChannelMessageReaction[];
		selected?: string;
	} = $props();

	const everyReaction = ':all:';
	const text = createPageText(channelText);
	const totalCount = $derived(reactions.reduce((sum, reaction) => sum + reaction.count, 0));
</script>

{#snippet personRow(person: ChannelParticipant, reaction: ChannelMessageReaction)}
	<li class="flex items-center gap-3 px-1 py-2">
		<PersonAvatar
			name={person.name}
			email={person.email ?? ''}
			seed={person.email || person.id || person.name}
			image={person.avatarURL ?? ''}
			memberID={person.memberID ?? ''}
			externalID={person.externalID ?? ''}
		/>
		<span class="min-w-0 flex-1 truncate">{person.name}</span>
		<ReactionGlyph {reaction} class="text-base" />
	</li>
{/snippet}

<Tabs.Root bind:value={() => selected || everyReaction, (value) => (selected = value)}>
	<Tabs.List variant="line" class="h-auto w-auto flex-wrap justify-start">
		<Tabs.Trigger value={everyReaction} class="flex-none">{text.reactionSheetAll} {totalCount}</Tabs.Trigger>
		{#each reactions as reaction (reaction.value)}
			<Tabs.Trigger value={reaction.value} class="flex-none tabular-nums">
				<ReactionGlyph {reaction} />
				{reaction.count}
			</Tabs.Trigger>
		{/each}
	</Tabs.List>
	<Tabs.Content value={everyReaction}>
		<ul class="max-h-[50vh] overflow-y-auto py-1">
			{#each reactions as reaction (reaction.value)}
				{#each reaction.people ?? [] as person (person.id)}
					{@render personRow(person, reaction)}
				{/each}
			{/each}
		</ul>
	</Tabs.Content>
	{#each reactions as reaction (reaction.value)}
		<Tabs.Content value={reaction.value}>
			<ul class="max-h-[50vh] overflow-y-auto py-1">
				{#each reaction.people ?? [] as person (person.id)}
					{@render personRow(person, reaction)}
				{/each}
			</ul>
		</Tabs.Content>
	{/each}
</Tabs.Root>
