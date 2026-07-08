<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Textarea } from '$lib/components/ui/textarea';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import type { UpdateAttendanceEventRequest } from '../attendance-api';
	import type { AttendanceEvent, AttendanceLocation } from '../attendance-context.svelte';
	import { attendanceText } from '../text';

	type Props = {
		event: AttendanceEvent;
		locations: AttendanceLocation[];
		isExpanded: boolean;
		onToggle: (eventID: string) => void;
		onSaveOverride: (eventID: string, request: UpdateAttendanceEventRequest) => Promise<void>;
	};

	let {
		event,
		locations,
		isExpanded,
		onToggle,
		onSaveOverride
	}: Props = $props();

	const text = createPageText(attendanceText);
	const parsedMismatch = $derived(
		!!event.parsedAs?.locationID && event.parsedAs.locationID !== event.locationID
	);
	const firstLocationID = $derived(locations[0]?.id ?? '');
	const eventLocationID = $derived(event.locationID || firstLocationID);
	const sortedOverrideHistory = $derived(
		[...(event.overrideHistory ?? [])].sort((first, second) => second.editedAt.localeCompare(first.editedAt))
	);

	let isEditing = $state(false);
	let draftLocalTime = $state('');
	let draftLocationID = $state('');
	let draftReason = $state('');
	let isSaving = $state(false);
	let errorMessage = $state('');

	$effect(() => {
		if (isEditing) return;
		draftLocalTime = shortTime(event.localTime);
		draftLocationID = eventLocationID;
		draftReason = '';
		errorMessage = '';
	});

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

	function formatOverridePoint(localTime: string, locationName: string): string {
		if (!locationName) return localTime;
		return `${localTime} · ${locationName}`;
	}

	function formatCancelReason(reason: string): string {
		return text.cancelReasonTemplate.replace('{reason}', cancelReasonLabel(reason));
	}

	function shortTime(localTime: string): string {
		return localTime.slice(0, 5);
	}

	function openEditor() {
		isEditing = true;
		draftLocalTime = shortTime(event.localTime);
		draftLocationID = eventLocationID;
		draftReason = '';
		errorMessage = '';
	}

	function closeEditor() {
		if (isSaving) return;
		isEditing = false;
		errorMessage = '';
	}

	async function saveOverride() {
		if (isSaving) return;
		isSaving = true;
		errorMessage = '';
		try {
			await onSaveOverride(event.id, {
				localDate: event.localDate,
				localTime: draftLocalTime,
				locationID: draftLocationID,
				reason: draftReason.trim()
			});
			isEditing = false;
			draftReason = '';
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.processingFailed;
		} finally {
			isSaving = false;
		}
	}

	function handleDraftTimeInput(inputEvent: Event) {
		if (inputEvent.currentTarget instanceof HTMLInputElement) {
			draftLocalTime = inputEvent.currentTarget.value;
		}
	}

	function cancelReasonLabel(reason: string): string {
		if (reason === 'repeated_click_confirmed' || reason === 'repeated click confirmed') {
			return text.cancelReasonRepeatedClick;
		}
		if (reason === 'accidental_short_segment' || reason === 'accidental short segment') {
			return text.cancelReasonAccidentalShortSegment;
		}
		if (reason === 'same_location_resume' || reason === 'same location resume') {
			return text.cancelReasonSameLocationResume;
		}
		return reason;
	}
</script>

<div data-testid="personal-day-event-row" class={`rounded border border-border/40 ${event.canceledAt ? 'opacity-60' : ''}`}>
	<button
		type="button"
		class="flex w-full min-w-0 items-center justify-between gap-2 p-2 text-left hover:bg-accent/40"
		onclick={() => onToggle(event.id)}
	>
		<span data-testid="personal-day-event-label" class={`min-w-0 truncate ${event.canceledAt ? 'line-through' : ''}`}>
			{eventKindLabel(event.kind)} {event.localTime}
			{#if event.locationName}<span class="text-muted-foreground"> · {event.locationName}</span>{/if}
		</span>
		<span class="flex shrink-0 items-center">
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
					<div class="mt-0.5 break-words rounded bg-muted/40 px-2 py-1 italic">"{event.sourceMessage}"</div>
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

			{#if sortedOverrideHistory.length > 0}
				<div class="space-y-1">
					{#each sortedOverrideHistory as override (override.id)}
						<div class="space-y-0.5 rounded bg-muted/30 px-2 py-1.5 text-muted-foreground">
							<div>{formatEditedAt(override.editedBy, formatOverriddenAt(override.editedAt))}</div>
							<div>
								{text.beforeEdit}: <span class="text-foreground">{formatOverridePoint(override.originalLocalTime, override.originalLocationName)}</span>
							</div>
							<div>
								{text.afterEdit}: <span class="text-foreground">{formatOverridePoint(override.overrideLocalTime, override.overrideLocationName)}</span>
							</div>
							<div>
								{text.editReason}: <span class="text-foreground">{override.reason}</span>
							</div>
						</div>
					{/each}
				</div>
			{:else if event.overriddenBy && event.overriddenAt}
				<div class="space-y-0.5 text-muted-foreground">
					<div>{formatEditedAt(event.overriddenBy, formatOverriddenAt(event.overriddenAt))}</div>
					{#if event.originalLocalTime || event.originalLocationName}
						<div>
							{text.beforeEdit}: <span class="text-foreground">{formatOverridePoint(event.originalLocalTime ?? '', event.originalLocationName ?? '')}</span>
						</div>
					{/if}
					{#if event.overrideReason}
						<div>
							{text.editReason}: <span class="text-foreground">{event.overrideReason}</span>
						</div>
					{/if}
				</div>
			{/if}

			{#if event.cancelReason}
				<div class="text-muted-foreground">{formatCancelReason(event.cancelReason)}</div>
			{/if}

			{#if !event.canceledAt && locations.length > 0}
				{#if isEditing}
					<form
						class="grid gap-2 rounded border border-border/50 bg-muted/20 p-2"
						onsubmit={(submitEvent) => {
							submitEvent.preventDefault();
							saveOverride();
						}}
					>
						<label class="grid gap-1 text-[11px] font-medium text-muted-foreground">
							<span>{text.eventTime}</span>
							<Input
								type="time"
								value={draftLocalTime}
								disabled={isSaving}
								oninput={handleDraftTimeInput}
								class="w-full"
							/>
						</label>
						<label class="grid gap-1 text-[11px] font-medium text-muted-foreground">
							<span>{text.location}</span>
							<Select.Root type="single" bind:value={draftLocationID} disabled={isSaving}>
								<Select.Trigger class="w-full">
									{locationNameOf(draftLocationID) || text.location}
								</Select.Trigger>
								<Select.Content>
									{#each locations as location (location.id)}
										<Select.Item value={location.id} label={location.name}>{location.name}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</label>
						<label class="grid gap-1 text-[11px] font-medium text-muted-foreground">
							<span>{text.editReason}</span>
							<Textarea
								bind:value={draftReason}
								placeholder={text.editReasonPlaceholder}
								disabled={isSaving}
								class="min-h-16 text-sm"
							/>
						</label>
						{#if errorMessage}
							<p class="text-destructive">{errorMessage}</p>
						{/if}
						<div class="flex justify-end gap-1">
							<Button type="button" variant="outline" size="sm" disabled={isSaving} onclick={closeEditor}>
								{text.cancel}
							</Button>
							<Button type="submit" size="sm" disabled={isSaving || !draftReason.trim()}>
								{text.save}
							</Button>
						</div>
					</form>
				{:else}
					<div class="flex justify-end">
						<Button type="button" variant="outline" size="sm" onclick={openEditor}>
							{text.edit}
						</Button>
					</div>
				{/if}
			{/if}
		</div>
	{/if}
</div>
