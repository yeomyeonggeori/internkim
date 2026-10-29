<script lang="ts">
	import * as Drawer from '$lib/components/ui/drawer';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as Avatar from '$lib/components/ui/avatar';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ReactionGlyph from './reaction-glyph.svelte';
	import type { ChannelMessageReaction, ChannelParticipant } from './channel-api';

	let {
		reactions,
		open = $bindable(false),
		selected = $bindable('')
	}: {
		reactions: ChannelMessageReaction[];
		open?: boolean;
		selected?: string;
	} = $props();

	const everyReaction = ':all:';
	const text = createPageText(channelText);
	const totalCount = $derived(reactions.reduce((sum, reaction) => sum + reaction.count, 0));
</script>

{#snippet personRow(person: ChannelParticipant, reaction: ChannelMessageReaction)}
	<li class="flex items-center gap-3 px-4 py-2">
		<Avatar.Root class="size-8">
			{#if person.avatarURL}
				<Avatar.Image src={person.avatarURL} alt={person.name} />
			{/if}
			<Avatar.Fallback>{person.name.slice(0, 1)}</Avatar.Fallback>
		</Avatar.Root>
		<span class="min-w-0 flex-1 truncate">{person.name}</span>
		<ReactionGlyph {reaction} class="text-base" />
	</li>
{/snippet}

<Drawer.Root bind:open>
	<Drawer.Content class="pb-[max(1rem,env(safe-area-inset-bottom))] sm:mx-auto sm:max-w-md">
		<Drawer.Header>
			<Drawer.Title>{text.reactionSheetTitle.replace('{count}', String(totalCount))}</Drawer.Title>
		</Drawer.Header>
		<Tabs.Root bind:value={() => selected || everyReaction, (value) => (selected = value)}>
			<Tabs.List variant="line" class="mx-4 h-auto w-auto flex-wrap justify-start">
				<Tabs.Trigger value={everyReaction} class="flex-none">{text.reactionSheetAll} {totalCount}</Tabs.Trigger>
				{#each reactions as reaction (reaction.value)}
					<Tabs.Trigger value={reaction.value} class="flex-none tabular-nums">
						<ReactionGlyph {reaction} />
						{reaction.count}
					</Tabs.Trigger>
				{/each}
			</Tabs.List>
			<Tabs.Content value={everyReaction}>
				<ul class="max-h-[50vh] overflow-y-auto py-2">
					{#each reactions as reaction (reaction.value)}
						{#each reaction.people ?? [] as person (person.id)}
							{@render personRow(person, reaction)}
						{/each}
					{/each}
				</ul>
			</Tabs.Content>
			{#each reactions as reaction (reaction.value)}
				<Tabs.Content value={reaction.value}>
					<ul class="max-h-[50vh] overflow-y-auto py-2">
						{#each reaction.people ?? [] as person (person.id)}
							{@render personRow(person, reaction)}
						{/each}
					</ul>
				</Tabs.Content>
			{/each}
		</Tabs.Root>
	</Drawer.Content>
</Drawer.Root>
