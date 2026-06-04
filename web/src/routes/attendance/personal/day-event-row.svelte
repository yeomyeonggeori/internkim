<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import type { AttendanceEvent, AttendanceLocation } from '../attendance-context.svelte';
	import { attendanceText } from '../text';

	type Props = {
		event: AttendanceEvent;
		locations: AttendanceLocation[];
		isExpanded: boolean;
		onToggle: (eventID: string) => void;
		onPickLocation: (
			eventID: string,
			currentLocationID: string | undefined,
			newLocationID: string
		) => void;
		onConfirmClockOut: (eventID: string) => void;
		onSkip: (eventID: string) => void;
	};

	let {
		event,
		locations,
		isExpanded,
		onToggle,
		onPickLocation,
		onConfirmClockOut,
		onSkip
	}: Props = $props();

	const text = createPageText(attendanceText);
	const parsedMismatch = $derived(
		!!event.parsedAs?.locationID && event.parsedAs.locationID !== event.locationID
	);

	function formatOverriddenAt(isoTimestamp: string): string {
		return new Date(isoTimestamp).toLocaleString(text.dateLocale, { dateStyle: 'short', timeStyle: 'short' });
	}

	function locationNameOf(locationID: string | undefined): string {
		if (!locationID) return '';
		return locations.find((location) => location.id === locationID)?.name ?? locationID;
	}

	function eventKindLabel(kind: AttendanceEvent['kind']): string {
		return kind === 'clock_in' ? `▶ ${text.clockIn}` : `◀ ${text.clockOut}`;
	}

	function formatAgentConfidence(confidence: number): string {
		return text.agentConfidenceTemplate.replace('{percent}', String(Math.round(confidence * 100)));
	}

	function formatEditedAt(user: string, time: string): string {
		return text.editedAtTemplate.replace('{user}', user).replace('{time}', time);
	}

	function formatCancelReason(reason: string): string {
		return text.cancelReasonTemplate.replace('{reason}', reason);
	}
</script>

<div class={`rounded border border-border/40 ${event.canceledAt ? 'opacity-60' : ''}`}>
	<button
		type="button"
		class="flex w-full items-center justify-between gap-2 p-2 text-left hover:bg-accent/40"
		onclick={() => onToggle(event.id)}
	>
		<span class={event.canceledAt ? 'line-through' : ''}>
			{eventKindLabel(event.kind)} {event.localTime}
			{#if event.locationName}<span class="text-muted-foreground"> · {event.locationName}</span>{/if}
		</span>
		<span class="flex items-center">
			{#if isExpanded}
				<ChevronDownIcon class="h-3 w-3" />
			{:else}
				<ChevronRightIcon class="h-3 w-3" />
			{/if}
		</span>
	</button>
	{#if isExpanded}
		<div class="space-y-2 border-t border-border/40 p-2">
			{#if event.sourceMessage}
				<div>
					<div class="text-[10px] uppercase tracking-wide text-muted-foreground">{text.originalMessage}</div>
					<div class="mt-0.5 rounded bg-muted/40 px-2 py-1 italic">"{event.sourceMessage}"</div>
				</div>
			{:else if event.manualEntry}
				<div class="text-muted-foreground">{text.manualEntry}</div>
			{/if}

			{#if event.confidence !== undefined}
				<div class="text-muted-foreground">{formatAgentConfidence(event.confidence)}</div>
			{/if}

			{#if parsedMismatch}
				<div class="text-muted-foreground">
					{text.agentInitialClassification}: <span class="text-foreground">{locationNameOf(event.parsedAs?.locationID)}</span>
					→ {text.current} <span class="text-foreground">{event.locationName ?? '?'}</span>
				</div>
			{/if}

			{#if event.overriddenBy && event.overriddenAt}
				<div class="text-muted-foreground">
					{formatEditedAt(event.overriddenBy, formatOverriddenAt(event.overriddenAt))}
				</div>
			{/if}

			{#if event.cancelReason}
				<div class="text-muted-foreground">{formatCancelReason(event.cancelReason)}</div>
			{/if}

			{#if !event.canceledAt && locations.length > 0}
				<div>
					<div class="text-[10px] uppercase tracking-wide text-muted-foreground">{text.classification}</div>
					<div class="mt-1 flex flex-wrap items-center gap-1">
						{#if event.kind === 'clock_in'}
							{#each locations as location (location.id)}
								<button
									type="button"
									class={`rounded border px-2 py-1 transition ${
										event.locationID === location.id
											? 'border-foreground/60 bg-foreground/5 font-medium'
											: 'border-border/40 hover:bg-accent/40'
									}`}
									onclick={() => onPickLocation(event.id, event.locationID, location.id)}
								>
									{location.name}
								</button>
							{/each}
						{:else}
							<button
								type="button"
								class="rounded border border-foreground/60 bg-foreground/5 px-2 py-1 font-medium"
								onclick={() => onConfirmClockOut(event.id)}
							>
								{text.confirmClockOut}
							</button>
						{/if}
						<button
							type="button"
							class="ml-1 rounded border border-border/40 bg-background px-2 py-1 text-muted-foreground hover:bg-muted/40"
							onclick={() => onSkip(event.id)}
						>
							{text.skip}
						</button>
					</div>
				</div>
			{/if}
		</div>
	{/if}
</div>
