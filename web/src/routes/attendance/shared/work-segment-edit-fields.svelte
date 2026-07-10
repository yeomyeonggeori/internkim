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
		text: AttendanceText;
		onStartTimeChange: (value: string) => void;
		onEndTimeChange: (value: string) => void;
		onLocationChange: (value: string) => void;
	};

	let {
		startTime,
		endTime,
		locationID,
		locations,
		isSaving,
		text,
		onStartTimeChange,
		onEndTimeChange,
		onLocationChange
	}: Props = $props();

	function inputValue(event: Event): string {
		return event.currentTarget instanceof HTMLInputElement ? event.currentTarget.value : '';
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
				disabled={isSaving}
				oninput={(event) => onStartTimeChange(inputValue(event))}
				class="w-full"
			/>
		</label>
		{#if endTime !== undefined}
			<label class="grid gap-1 text-[11px] font-medium text-muted-foreground">
				<span>{text.clockOut}</span>
				<Input
					type="time"
					value={endTime}
					disabled={isSaving}
					oninput={(event) => onEndTimeChange(inputValue(event))}
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
