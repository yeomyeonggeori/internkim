<script lang="ts">
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as HoverCard from '$lib/components/ui/hover-card';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { MentionPerson } from '$lib/messenger/mention-candidates';
	import type { PersonProfileActions } from '$lib/messenger/person-profile';

	let { text, person, actions }: { text: string; person: MentionPerson; actions: PersonProfileActions } = $props();

	const words = createPageText(channelText);
</script>

<HoverCard.Root openDelay={300} closeDelay={100}>
	<HoverCard.Trigger>
		{#snippet child({ props })}
			<button
				{...props}
				type="button"
				class="bg-primary/15 text-primary hover:bg-primary/25 cursor-pointer rounded px-1 font-medium"
				onclick={() => actions.show(person)}>{text}</button
			>
		{/snippet}
	</HoverCard.Trigger>
	<HoverCard.Content class="w-72 p-4" align="start">
		<div class="flex items-center gap-3">
			<PersonAvatar class="size-12" name={person.name} externalID={person.externalID} image={person.avatarURL ?? ''} />
			<p class="truncate font-semibold">{person.name}</p>
		</div>
		<Button variant="outline" size="sm" class="mt-3 w-full" onclick={() => actions.message(person)}>
			<MessageSquareIcon />
			{words.sendDirectMessage}
		</Button>
	</HoverCard.Content>
</HoverCard.Root>
