<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
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

	function formatDate(date: string): string {
		return new Intl.DateTimeFormat(text.dateLocale, {
			year: 'numeric',
			month: 'long',
			day: 'numeric',
			weekday: 'long',
			timeZone: 'UTC'
		}).format(new Date(`${date}T00:00:00Z`));
	}

</script>

<Sheet.Root bind:open={isOpen}>
	<Sheet.Content
		side={isMobile.current ? 'bottom' : 'right'}
		class={isMobile.current
			? 'max-h-[92vh] gap-0 overflow-y-auto rounded-t-xl p-0'
			: 'w-full gap-0 overflow-y-auto p-0 sm:max-w-[34rem]'}
		closeLabel={text.close}
		data-testid={isMobile.current ? 'team-status-day-detail-sheet' : 'team-status-day-detail-dialog'}
	>
		{#if detail}
			<Sheet.Header class="sticky top-0 z-10 gap-4 border-b bg-popover/95 px-5 py-5 pr-14 text-left backdrop-blur-sm">
				<Sheet.Title class="min-w-0 text-lg font-semibold tracking-tight text-balance" data-testid="team-status-day-detail-date">
					{formatDate(detail.day.date)}
				</Sheet.Title>
				<div class="flex min-w-0 items-center gap-3">
					<PersonAvatar
						name={detail.displayName}
						email={detail.email}
						seed={detail.email || detail.displayName}
						image={detail.image ?? ''}
						class="size-9 ring-1 ring-border"
					/>
					<div class="min-w-0">
						<p class="truncate text-sm font-semibold" data-testid="team-status-day-detail-person">{detail.displayName}</p>
						<p class="truncate text-xs text-muted-foreground">
							{detail.mattermostUsername ? `@${detail.mattermostUsername}` : detail.email}
						</p>
					</div>
				</div>
				<Sheet.Description class="sr-only">{detail.email}</Sheet.Description>
			</Sheet.Header>
			<TeamStatusDayDetailContent {text} {detail} />
		{/if}
	</Sheet.Content>
</Sheet.Root>
