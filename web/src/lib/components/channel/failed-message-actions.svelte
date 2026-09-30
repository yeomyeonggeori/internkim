<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';

	let { onRetry, onDiscard }: { onRetry: () => void; onDiscard: () => void } = $props();

	const text = createPageText(channelText);
	const labelID = $props.id();
</script>

<div
	role="group"
	aria-labelledby={labelID}
	class="group-data-[align=end]/message:self-end flex w-fit items-center gap-1 text-xs"
>
	<CircleAlertIcon class="text-destructive size-4 shrink-0" aria-hidden="true" />
	<span id={labelID} class="text-destructive">{text.sendFailed}</span>
	<Button variant="ghost" size="sm" onclick={onRetry}>{text.sendAgain}</Button>
	<Button variant="ghost" size="sm" onclick={onDiscard}>{text.discardFailedMessage}</Button>
</div>
