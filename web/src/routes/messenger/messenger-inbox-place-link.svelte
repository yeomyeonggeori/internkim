<script lang="ts">
	import { messengerInboxText } from '$lib/i18n/messenger-inbox-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { InboxPlace } from './messenger-inbox-place';

	let { place, go }: { place: InboxPlace; go: () => void } = $props();

	const text = createPageText(messengerInboxText);
</script>

<button type="button" class="hover:bg-accent min-w-0 cursor-pointer truncate rounded-md px-1 py-0.5 font-semibold" onclick={go}>
	{place.kind === 'direct' ? `${text.directFrom} ${place.personName}` : `${text.threadIn} #${place.channelName}`}
</button>
