<script lang="ts">
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import * as Card from '$lib/components/ui/card';
	import { cn } from '$lib/utils';
	import CalendarEventContent from './calendar-event-content.svelte';

	type Props = {
		title: string;
		cardTestID?: string;
		buttonTestID?: string;
		buttonClass?: string;
		start: Date;
		isAllDay: boolean;
		timeLabel?: string;
		location?: string;
		openEvent: (originElement: HTMLElement) => void;
	};

	let {
		title,
		start,
		isAllDay,
		timeLabel = '',
		location = '',
		cardTestID = 'calendar-event-card',
		buttonTestID = 'calendar-event-card-button',
		buttonClass = '',
		openEvent
	}: Props = $props();
</script>

<Card.Root
	size="sm"
	class="bg-info/10 text-info ring-info/20 hover:bg-info/15 gap-0 rounded-md py-0 transition-colors data-[size=sm]:gap-0 data-[size=sm]:py-0"
	data-testid={cardTestID}
>
	<Card.Content class="p-0 group-data-[size=sm]/card:px-0">
		<button
			type="button"
			class={cn(
				'focus-visible:ring-info/40 min-h-11 w-full min-w-0 px-3 py-2 text-left focus-visible:ring-2 focus-visible:ring-inset focus-visible:outline-none',
				buttonClass
			)}
			aria-label={title}
			data-testid={buttonTestID}
			onclick={(event) => openEvent(event.currentTarget)}
		>
			<CalendarEventContent event={{ title, start, allDay: isAllDay }} {isAllDay} {timeLabel} />
			{#if location}
				<span class="text-info/70 mt-1 flex min-w-0 items-center gap-1 text-xs">
					<MapPinIcon class="size-3 shrink-0" aria-hidden="true" />
					<span class="truncate">{location}</span>
				</span>
			{/if}
		</button>
	</Card.Content>
</Card.Root>
