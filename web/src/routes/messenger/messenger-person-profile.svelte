<script lang="ts">
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Sheet from '$lib/components/ui/sheet';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { MentionPerson } from '$lib/messenger/mention-candidates';

	let {
		open = $bindable(false),
		person,
		onMessage
	}: {
		open?: boolean;
		person: MentionPerson | undefined;
		onMessage: (person: MentionPerson) => void;
	} = $props();

	const text = createPageText(channelText);
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="gap-0 p-0 sm:max-w-sm" closeLabel={text.closePersonProfile}>
		<Sheet.Header class="border-b">
			<Sheet.Title>{text.personProfile}</Sheet.Title>
		</Sheet.Header>
		{#if person}
			<div class="flex flex-col items-center gap-3 px-6 py-8 text-center">
				<PersonAvatar class="size-24" name={person.name} externalID={person.externalID} image={person.avatarURL ?? ''} />
				<p class="text-xl font-semibold break-all">{person.name}</p>
			</div>
			<div class="px-4">
				<Button variant="secondary" class="h-auto w-full flex-col gap-1 py-3" onclick={() => onMessage(person)}>
					<MessageSquareIcon class="size-5" />
					{text.sendDirectMessage}
				</Button>
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>
