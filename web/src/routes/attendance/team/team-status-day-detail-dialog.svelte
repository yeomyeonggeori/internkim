<script lang="ts">
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import * as Sheet from '$lib/components/ui/sheet';
	import type { AttendanceText } from '../text';
	import type { TeamStatusDayDetail } from './team-status-day-detail';
	import TeamStatusDayDetailContent from './team-status-day-detail-content.svelte';

	type Props = {
		text: AttendanceText;
		isOpen: boolean;
		detail: TeamStatusDayDetail | null;
	};

	let { text, isOpen = $bindable(), detail }: Props = $props();
	const isMobile = new IsMobile();
</script>

<Sheet.Root bind:open={isOpen}>
	<Sheet.Content
		side={isMobile.current ? 'bottom' : 'right'}
		class={isMobile.current
			? 'max-h-[85vh] overflow-y-auto rounded-t-xl p-4'
			: 'w-full overflow-y-auto p-4 sm:max-w-md'}
		closeLabel={text.close}
		data-testid={isMobile.current ? 'team-status-day-detail-sheet' : 'team-status-day-detail-dialog'}
	>
		{#if detail}
			<Sheet.Header class="pr-8 text-left">
				<Sheet.Title>{detail.displayName} · {detail.day.date}</Sheet.Title>
				<Sheet.Description>{detail.email}</Sheet.Description>
			</Sheet.Header>
			<TeamStatusDayDetailContent {text} {detail} />
		{/if}
	</Sheet.Content>
</Sheet.Root>
