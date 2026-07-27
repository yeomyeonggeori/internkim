<script lang="ts">
	import { attendanceClock } from '$lib/components/attendance-clock.svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../../routes/attendance/text';
	import { tick } from 'svelte';

	const text = createPageText(attendanceText);
	let clockOutElement = $state<HTMLElement | null>(null);
	let locationElements = $state<(HTMLElement | null)[]>([]);

	focusPrimaryAction();

	async function focusPrimaryAction() {
		await attendanceClock.load();
		await tick();
		const element = attendanceClock.isClockedIn ? clockOutElement : locationElements[0];
		element?.focus();
	}
</script>

{#snippet locationItems()}
	<DropdownMenu.RadioGroup
		value={attendanceClock.currentLocationID}
		onValueChange={(locationID) => attendanceClock.clock('clock_in', locationID)}
	>
		{#each attendanceClock.locations as location, index (location.id)}
			<DropdownMenu.RadioItem
				bind:ref={() => locationElements[index] ?? null, (element) => (locationElements[index] = element)}
				value={location.id}
				disabled={attendanceClock.isSubmitting}
			>
				{location.name}
			</DropdownMenu.RadioItem>
		{/each}
	</DropdownMenu.RadioGroup>
{/snippet}

{#if attendanceClock.summary}
	{#if attendanceClock.locations.length <= 1}
		<DropdownMenu.Item
			closeOnSelect={false}
			disabled={attendanceClock.isClockedIn || attendanceClock.isSubmitting}
			onclick={() => attendanceClock.clock('clock_in', attendanceClock.locations[0]?.id ?? '')}
		>
			{text.clockIn}
		</DropdownMenu.Item>
	{:else if attendanceClock.isClockedIn}
		<DropdownMenu.Sub>
			<DropdownMenu.SubTrigger>
				{text.clockIn}
			</DropdownMenu.SubTrigger>
			<DropdownMenu.SubContent>
				<DropdownMenu.Label>{text.location}</DropdownMenu.Label>
				{@render locationItems()}
			</DropdownMenu.SubContent>
		</DropdownMenu.Sub>
	{:else}
		<DropdownMenu.Label>{text.clockIn}</DropdownMenu.Label>
		{@render locationItems()}
	{/if}
	<DropdownMenu.Item
		bind:ref={clockOutElement}
		closeOnSelect={false}
		disabled={!attendanceClock.isClockedIn || attendanceClock.isSubmitting}
		onclick={() => attendanceClock.clock('clock_out', '')}
	>
		{text.clockOut}
	</DropdownMenu.Item>
{/if}
