<script lang="ts">
	import * as Drawer from '$lib/components/ui/drawer';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ReactionPeopleList from './reaction-people-list.svelte';
	import type { ChannelMessageReaction } from './channel-api';

	let {
		reactions,
		open = $bindable(false),
		selected = $bindable('')
	}: {
		reactions: ChannelMessageReaction[];
		open?: boolean;
		selected?: string;
	} = $props();

	const text = createPageText(channelText);
	const totalCount = $derived(reactions.reduce((sum, reaction) => sum + reaction.count, 0));
</script>

<Drawer.Root bind:open>
	<Drawer.Content class="pb-[max(1rem,env(safe-area-inset-bottom))] sm:mx-auto sm:max-w-md">
		<Drawer.Header>
			<Drawer.Title>{text.reactionSheetTitle.replace('{count}', String(totalCount))}</Drawer.Title>
		</Drawer.Header>
		<div class="px-3">
			<ReactionPeopleList {reactions} bind:selected />
		</div>
	</Drawer.Content>
</Drawer.Root>
