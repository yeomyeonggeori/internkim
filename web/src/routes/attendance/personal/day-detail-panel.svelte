<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { attendanceText } from '../text';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const dayEvents = $derived(
		attendance.summary && attendance.selectedDate
			? attendance.summary.events.filter(
					(e) =>
						e.email === (attendance.selectedEmail || attendance.summary?.currentUserEmail) &&
						e.localDate === attendance.selectedDate
				)
			: []
	);

	const locations = $derived(attendance.summary?.locations ?? []);

	let expanded = $state<Record<string, boolean>>({});

	function toggle(id: string) {
		expanded[id] = !expanded[id];
	}

	function formatOverriddenAt(iso: string): string {
		try {
			return new Date(iso).toLocaleString(text.dateLocale, { dateStyle: 'short', timeStyle: 'short' });
		} catch {
			return iso;
		}
	}

	function locationNameOf(id: string | undefined): string {
		if (!id) return '';
		return locations.find((l) => l.id === id)?.name ?? id;
	}

	function pickLocation(eventID: string, currentLocationID: string | undefined, newLocationID: string) {
		if (newLocationID === currentLocationID) {
			attendance.confirmClassification(eventID);
		} else {
			attendance.overrideLocation(eventID, newLocationID);
		}
	}

	function skipEvent(eventID: string) {
		attendance.dismissEvent(eventID, 'classification_dismissed');
	}

	function eventKindLabel(kind: string): string {
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

{#if attendance.selectedDate}
	<Card.Root>
		<Card.Header>
			<Card.Title class="text-sm">{attendance.selectedDate}</Card.Title>
		</Card.Header>
		<Card.Content class="space-y-2 text-xs">
			{#each dayEvents as event (event.id)}
				{@const isExpanded = !!expanded[event.id]}
				{@const parsedMismatch = !!event.parsedAs?.locationID && event.parsedAs.locationID !== event.locationID}
				<div class={`rounded border border-border/40 ${event.canceledAt ? 'opacity-60' : ''}`}>
					<button
						type="button"
						class="flex w-full items-center justify-between gap-2 p-2 text-left hover:bg-accent/40"
						onclick={() => toggle(event.id)}
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
													onclick={() => pickLocation(event.id, event.locationID, location.id)}
												>
													{location.name}
												</button>
											{/each}
										{:else}
											<button
												type="button"
												class="rounded border border-foreground/60 bg-foreground/5 px-2 py-1 font-medium"
												onclick={() => attendance.confirmClassification(event.id)}
											>
												{text.confirmClockOut}
											</button>
										{/if}
										<button
											type="button"
											class="ml-1 rounded border border-border/40 bg-background px-2 py-1 text-muted-foreground hover:bg-muted/40"
											onclick={() => skipEvent(event.id)}
										>
											{text.skip}
										</button>
									</div>
								</div>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
			{#if dayEvents.length === 0}
				<p class="text-muted-foreground">{text.eventNone}</p>
			{/if}
		</Card.Content>
	</Card.Root>
{/if}
