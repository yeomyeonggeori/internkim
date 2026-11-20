<script lang="ts">
	import MonitorOffIcon from '@lucide/svelte/icons/monitor-off';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Spinner } from '$lib/components/ui/spinner/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	let {
		isShowingSavedConversations,
		isReconnecting,
		reconnect
	}: {
		isShowingSavedConversations: boolean;
		isReconnecting: boolean;
		reconnect: () => void;
	} = $props();

	const text = createPageText(channelText);
</script>

<div
	role="status"
	class="flex shrink-0 items-center gap-2.5 border-b border-warning/30 bg-warning-subtle px-4 py-1.5 text-sm text-warning-subtle-foreground"
>
	<MonitorOffIcon class="size-4 shrink-0" aria-hidden="true" />
	<p class="min-w-0 flex-1 break-keep">
		<span class="font-medium">{text.companyComputerOffline}</span>
		<span class="opacity-80">{isShowingSavedConversations ? text.showingSavedConversations : text.conversationsNeedCompanyComputer}</span>
	</p>
	<Button variant="outline" size="xs" class="shrink-0" disabled={isReconnecting} onclick={reconnect}>
		{#if isReconnecting}<Spinner aria-hidden="true" />{:else}<RefreshCwIcon />{/if}
		{text.reconnect}
	</Button>
</div>
