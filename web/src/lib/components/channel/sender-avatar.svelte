<script lang="ts">
	import { getContext, type Snippet } from 'svelte';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { MentionPerson } from '$lib/messenger/mention-candidates';
	import { personProfileContext, type PersonProfileActions } from '$lib/messenger/person-profile';

	let { person, children }: { person?: MentionPerson; children: Snippet } = $props();

	const profile = getContext<PersonProfileActions | undefined>(personProfileContext);
	const text = createPageText(channelText);
	const avatarClass =
		'bg-muted mt-[5px] flex w-fit min-w-8 shrink-0 items-center justify-center self-start overflow-hidden rounded-full group-has-data-[slot=message-header]/message:mt-[23px]';
</script>

{#if profile && person}
	{@const shown = person}
	<button
		type="button"
		data-slot="sender-avatar"
		class="{avatarClass} cursor-pointer"
		aria-label={`${text.personProfile}: ${shown.name}`}
		onclick={() => profile.show(shown)}
	>
		{@render children()}
	</button>
{:else}
	<div data-slot="sender-avatar" class={avatarClass}>
		{@render children()}
	</div>
{/if}
