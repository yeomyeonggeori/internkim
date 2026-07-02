<script lang="ts">
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import * as Dialog from '$lib/components/ui/dialog';
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

{#if isMobile.current}
	<Sheet.Root bind:open={isOpen}>
		<Sheet.Content
			side="bottom"
			class="max-h-[85vh] overflow-y-auto rounded-t-xl p-4"
			closeLabel={text.close}
			data-testid="team-status-day-detail-sheet"
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
{:else}
	<Dialog.Root bind:open={isOpen}>
		<Dialog.Content
			class="max-h-[calc(100vh-2rem)] max-w-md grid-rows-[auto_minmax(0,1fr)] overflow-hidden"
			data-testid="team-status-day-detail-dialog"
		>
			{#if detail}
				<Dialog.Header>
					<Dialog.Title>{detail.displayName} · {detail.day.date}</Dialog.Title>
					<Dialog.Description>{detail.email}</Dialog.Description>
				</Dialog.Header>
				<TeamStatusDayDetailContent {text} {detail} />
			{/if}
		</Dialog.Content>
	</Dialog.Root>
{/if}
