<script lang="ts">
	import { messengerInboxText } from '$lib/i18n/messenger-inbox-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { InboxPlace } from './messenger-inbox-place';

	let { place, go }: { place: InboxPlace; go: () => void } = $props();

	const text = createPageText(messengerInboxText);

	const label = $derived.by(() => {
		if (place.kind === 'direct') return `${text.directFrom} ${place.personName}`;
		if (place.kind === 'thread') return `${text.threadIn} #${place.channelName}`;
		return `${text.channelIn} #${place.channelName}`;
	});
</script>

<button type="button" class="hover:bg-accent min-w-0 cursor-pointer truncate rounded-md px-1 py-0.5 font-semibold" onclick={go}>
	{label}
</button>
