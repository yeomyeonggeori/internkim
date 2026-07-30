<script lang="ts">
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import * as Card from '$lib/components/ui/card';
	import { cn } from '$lib/utils';
	import { defaultCalendarEventColor } from '../grid/calendar-grid-events';
	import CalendarEventContent from './calendar-event-content.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { calendarParticipantKey, type CalendarParticipant } from './calendar-participants';

	type Props = {
		title: string;
		participants?: CalendarParticipant[];
		start: Date;
		isAllDay: boolean;
		color?: string;
		timeLabel?: string;
		location?: string;
		cardTestID?: string;
		buttonTestID?: string;
		buttonClass?: string;
		class?: string;
		openEvent: (originElement: HTMLElement) => void;
	};

	let {
		title,
		start,
		isAllDay,
		color = defaultCalendarEventColor,
		timeLabel = '',
		location = '',
		participants = [],
		cardTestID = 'calendar-event-card',
		buttonTestID = 'calendar-event-card-button',
		buttonClass = '',
		class: className = '',
		openEvent
	}: Props = $props();
</script>

<Card.Root
	size="sm"
	style={`--calendar-event-color: ${color}`}
	class={cn(
		'bg-(--calendar-event-color)/12 hover:bg-(--calendar-event-color)/20 text-foreground ring-(--calendar-event-color)/20 gap-0 rounded-md py-0 transition-colors data-[size=sm]:gap-0 data-[size=sm]:py-0',
		className
	)}
	data-testid={cardTestID}
>
	<Card.Content class="p-0 group-data-[size=sm]/card:px-0">
		<button
			type="button"
			class={cn(
				'focus-visible:ring-ring/50 flex min-h-11 w-full min-w-0 items-stretch gap-2 px-2 py-2 text-left focus-visible:ring-2 focus-visible:ring-inset focus-visible:outline-none',
				buttonClass
			)}
			aria-label={title}
			data-testid={buttonTestID}
			onclick={(event) => openEvent(event.currentTarget)}
		>
			<span class="w-1 shrink-0 self-stretch rounded-full bg-(--calendar-event-color)"></span>
			<span
				class="min-w-0 flex-1 [&_.calendar-event-content]:flex [&_.calendar-event-content]:min-w-0 [&_.calendar-event-content]:flex-col [&_.calendar-event-content]:gap-0.5 [&_.calendar-event-time]:!text-muted-foreground/70 [&_.calendar-event-time]:self-start [&_.calendar-event-time]:!text-[10px] [&_.calendar-event-time]:tabular-nums [&_.calendar-event-title]:block [&_.calendar-event-title]:min-w-0 [&_.calendar-event-title]:truncate [&_.calendar-event-title]:text-sm [&_.calendar-event-title]:font-medium"
			>
				<CalendarEventContent event={{ title, start, allDay: isAllDay }} {isAllDay} {timeLabel} />
				{#if participants.length > 0}
					<span class="mt-1 flex items-center -space-x-1">
						{#each participants.slice(0, 4) as participant (calendarParticipantKey(participant))}
							<PersonAvatar
								name={participant.name}
								email={participant.email ?? ''}
								seed={calendarParticipantKey(participant)}
								image={participant.image ?? ''}
								class="ring-background size-4 ring-2"
							/>
						{/each}
						{#if participants.length > 4}
							<span class="text-muted-foreground pl-2 text-[10px]">+{participants.length - 4}</span>
						{/if}
					</span>
				{/if}
				{#if location}
					<span class="text-muted-foreground mt-1 flex min-w-0 items-center gap-1 text-xs">
						<MapPinIcon class="size-3 shrink-0" aria-hidden="true" />
						<span class="truncate">{location}</span>
					</span>
				{/if}
			</span>
		</button>
	</Card.Content>
</Card.Root>
