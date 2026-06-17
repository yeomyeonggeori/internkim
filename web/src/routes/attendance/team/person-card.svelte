<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import type { AttendanceWorkSegment, PersonToday } from '../shared/attendance-aggregation';
	import { absenceLabelText } from '../shared/attendance-absence';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import { STATUS_TONE } from '../shared/color-tokens';
	import { attendanceText } from '../text';

	type Props = {
		person: PersonToday;
	};

	let { person }: Props = $props();

	const text = createPageText(attendanceText);
	let areLocationsExpanded = $state(false);
	const statusLabel = $derived(buildStatusLabel(person));
	const locationChips = $derived(buildLocationChips(person));
	const visibleLocationChips = $derived(areLocationsExpanded ? locationChips : locationChips.slice(0, 2));
	const hiddenLocationCount = $derived(Math.max(0, locationChips.length - visibleLocationChips.length));

	function buildStatusLabel(p: PersonToday): string {
		if (p.status === 'working' && p.activeSegment) return `● ${text.working} (${p.activeSegment.startTime}~)`;
		if (p.status === 'finished' && p.clockIn && p.clockOut) {
			return `○ ${text.finished} (${p.clockIn.localTime}–${p.clockOut.localTime})`;
		}
		if (p.status === 'upcoming') return `· ${text.upcoming}`;
		if (p.status === 'weekend') return '—';
		if (p.status === 'absence' && p.absence) return `— ${text.absence} · ${absenceLabelText(p.absence, text)}`;
		return `— ${text.absent}`;
	}

	type LocationChip = {
		id: string;
		label: string;
		isCurrent: boolean;
	};

	function buildLocationChips(p: PersonToday): LocationChip[] {
		const segments = p.segments.filter((segment) => segment.locationName || segment.locationID);
		const currentSegment = p.activeSegment;
		const closedSegments = segments.filter((segment) => segment.id !== currentSegment?.id);
		const orderedSegments = currentSegment ? [currentSegment, ...closedSegments.reverse()] : segments;
		return orderedSegments.map((segment) => segmentLocationChip(segment, segment.id === currentSegment?.id));
	}

	function segmentLocationChip(segment: AttendanceWorkSegment, isCurrent: boolean): LocationChip {
		return {
			id: segment.id,
			label: segment.locationName ?? segment.locationID ?? '',
			isCurrent,
		};
	}
</script>

<div class="flex flex-col items-start gap-1 rounded-md border bg-card p-3 text-left">
	<span class="text-sm font-medium">{person.displayName}</span>
	<span class={`text-xs ${STATUS_TONE[person.status]}`}>{statusLabel}</span>
	<div class="flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
		{#each visibleLocationChips as chip (chip.id)}
			<Badge
				variant="outline"
				class={`max-w-full gap-1 ${chip.isCurrent ? 'border-success/40 bg-success/10 text-success' : ''}`}
			>
				<MapPinIcon class="h-3 w-3" />
				<span class="truncate">{chip.label}</span>
			</Badge>
		{/each}
		{#if locationChips.length > 2}
			<button
				type="button"
				class="rounded-full border border-border px-2 py-0.5 text-[11px] transition-colors hover:bg-muted hover:text-foreground focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40"
				aria-expanded={areLocationsExpanded}
				onclick={() => (areLocationsExpanded = !areLocationsExpanded)}
			>
				{areLocationsExpanded
					? text.collapseLocations
					: text.moreLocationsTemplate.replace('{count}', String(hiddenLocationCount))}
			</button>
		{/if}
		{#if person.workedMinutes > 0}
			<span>{formatHoursMinutes(person.workedMinutes)}</span>
		{/if}
	</div>
</div>
