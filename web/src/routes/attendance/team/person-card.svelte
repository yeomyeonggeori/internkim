<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import type { PersonToday } from '../shared/attendance-aggregation';
	import { absenceLabelText } from '../shared/attendance-absence';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import { STATUS_TONE } from '../shared/color-tokens';
	import { attendanceText } from '../text';

	type Props = {
		person: PersonToday;
		onSelect: (email: string) => void;
	};

	let { person, onSelect }: Props = $props();

	const text = createPageText(attendanceText);
	const statusLabel = $derived(buildStatusLabel(person));

	function buildStatusLabel(p: PersonToday): string {
		if (p.status === 'working' && p.clockIn) return `● ${text.working} (${p.clockIn.localTime}~)`;
		if (p.status === 'finished' && p.clockIn && p.clockOut) {
			return `○ ${text.finished} (${p.clockIn.localTime}–${p.clockOut.localTime})`;
		}
		if (p.status === 'upcoming') return `· ${text.upcoming}`;
		if (p.status === 'weekend') return '—';
		if (p.status === 'absence' && p.absence) return `— ${text.absence} · ${absenceLabelText(p.absence, text)}`;
		return `— ${text.absent}`;
	}
</script>

<button
	type="button"
	class="flex flex-col items-start gap-1 rounded-md border bg-card p-3 text-left transition hover:bg-accent"
	onclick={() => onSelect(person.email)}
>
	<span class="text-sm font-medium">{person.displayName}</span>
	<span class={`text-xs ${STATUS_TONE[person.status]}`}>{statusLabel}</span>
	<div class="flex items-center gap-2 text-xs text-muted-foreground">
		{#if person.locationName}
			<Badge variant="outline" class="gap-1">
				<MapPinIcon class="h-3 w-3" />
				{person.locationName}
			</Badge>
		{/if}
		{#if person.workedMinutes > 0}
			<span>{formatHoursMinutes(person.workedMinutes)}</span>
		{/if}
	</div>
</button>
