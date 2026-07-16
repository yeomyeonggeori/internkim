<script lang="ts">
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import type { AttendanceLocation } from '../attendance-context.svelte';
	import type { AttendanceText } from '../text';

	type Props = {
		startTime: string;
		endTime?: string;
		locationID: string;
		locations: AttendanceLocation[];
		isSaving: boolean;
		startMaximumTime?: string;
		endMaximumTime?: string;
		text: AttendanceText;
		onStartTimeChange: (value: string) => string;
		onEndTimeChange: (value: string) => string;
		onLocationChange: (value: string) => void;
	};

	let {
		startTime,
		endTime,
		locationID,
		locations,
		isSaving,
		startMaximumTime,
		endMaximumTime,
		text,
		onStartTimeChange,
		onEndTimeChange,
		onLocationChange
	}: Props = $props();

	function updateTimeInput(event: Event, onTimeChange: (value: string) => string): void {
		if (!(event.currentTarget instanceof HTMLInputElement)) return;
		event.currentTarget.value = onTimeChange(event.currentTarget.value);
	}

	function locationName(): string {
		return locations.find((location) => location.id === locationID)?.name ?? text.location;
	}
</script>

<div class="mt-3 grid gap-2 border-t pt-3" data-slot="work-segment-edit-fields">
	<div class={`grid gap-2 ${endTime === undefined ? '' : 'grid-cols-2'}`}>
		<label class="grid gap-1 text-[11px] font-medium text-muted-foreground">
			<span>{text.clockIn}</span>
			<Input
				type="time"
				value={startTime}
				max={startMaximumTime}
				disabled={isSaving}
				oninput={(event) => updateTimeInput(event, onStartTimeChange)}
				class="w-full"
			/>
		</label>
		{#if endTime !== undefined}
			<label class="grid gap-1 text-[11px] font-medium text-muted-foreground">
				<span>{text.clockOut}</span>
				<Input
					type="time"
					value={endTime}
					max={endMaximumTime}
					disabled={isSaving}
					oninput={(event) => updateTimeInput(event, onEndTimeChange)}
					class="w-full"
				/>
			</label>
		{/if}
	</div>
	<label class="grid gap-1 text-[11px] font-medium text-muted-foreground">
		<span>{text.location}</span>
		<Select.Root type="single" value={locationID} onValueChange={onLocationChange} disabled={isSaving}>
			<Select.Trigger class="w-full">{locationName()}</Select.Trigger>
			<Select.Content>
				<Select.Group>
					{#each locations as location (location.id)}
						<Select.Item value={location.id} label={location.name}>{location.name}</Select.Item>
					{/each}
				</Select.Group>
			</Select.Content>
		</Select.Root>
	</label>
</div>
