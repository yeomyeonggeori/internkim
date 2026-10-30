<script lang="ts">
	import * as Popover from '$lib/components/ui/popover';
	import type { Popover as PopoverPrimitive } from 'bits-ui';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ReactionPeopleList from './reaction-people-list.svelte';
	import type { ChannelMessageReaction } from './channel-api';

	let {
		reactions,
		selected = $bindable(''),
		onInteractOutside
	}: {
		reactions: ChannelMessageReaction[];
		selected?: string;
		onInteractOutside: PopoverPrimitive.ContentProps['onInteractOutside'];
	} = $props();

	const text = createPageText(channelText);
	const title = $derived(
		text.reactionSheetTitle.replace('{count}', String(reactions.reduce((sum, reaction) => sum + reaction.count, 0)))
	);
</script>

<Popover.Content side="right" role="dialog" aria-label={title} {onInteractOutside}>
	<Popover.Header>
		<Popover.Title>{title}</Popover.Title>
	</Popover.Header>
	<ReactionPeopleList {reactions} bind:selected />
</Popover.Content>
