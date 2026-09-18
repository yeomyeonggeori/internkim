<script lang="ts">
	import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';
	import UsersIcon from '@lucide/svelte/icons/users';
	import { Button } from '$lib/components/ui/button';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	let {
		memberCount,
		openMembers,
		openDetails
	}: {
		memberCount: number;
		openMembers: () => void;
		openDetails: () => void;
	} = $props();

	const text = createPageText(channelText);
</script>

<div class="ml-auto flex shrink-0 items-center gap-1">
	<Button
		variant="ghost"
		size="sm"
		aria-label={text.channelMemberCount.replace('{count}', String(memberCount))}
		onclick={openMembers}
	>
		<UsersIcon />
		{memberCount}
	</Button>
	<Button variant="ghost" size="icon-sm" aria-label={text.channelDetails} onclick={openDetails}>
		<EllipsisVerticalIcon />
	</Button>
</div>
