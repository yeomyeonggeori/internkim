<script lang="ts">
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../../routes/attendance/text';
	import { tick } from 'svelte';

	const text = createPageText(attendanceText);
	let clockOutElement = $state<HTMLElement | null>(null);
	let locationElements = $state<(HTMLElement | null)[]>([]);

	focusPrimaryAction();

	async function focusPrimaryAction() {
		await myAttendanceToday.load();
		await tick();
		const element = myAttendanceToday.nextKind === 'clock_out' ? clockOutElement : locationElements[0];
		element?.focus();
	}
</script>

{#snippet locationItems()}
	<DropdownMenu.RadioGroup
		value={myAttendanceToday.currentLocationID}
		onValueChange={(locationID) => myAttendanceToday.clock('clock_in', locationID)}
	>
		{#each myAttendanceToday.locations as location, index (location.id)}
			<DropdownMenu.RadioItem
				bind:ref={() => locationElements[index] ?? null, (element) => (locationElements[index] = element)}
				value={location.id}
				disabled={myAttendanceToday.isSubmitting}
			>
				{location.name}
			</DropdownMenu.RadioItem>
		{/each}
	</DropdownMenu.RadioGroup>
{/snippet}

{#if myAttendanceToday.summary}
	{#if myAttendanceToday.locations.length <= 1}
		<DropdownMenu.Item
			closeOnSelect={false}
			disabled={myAttendanceToday.nextKind === 'clock_out' || myAttendanceToday.isSubmitting}
			onclick={() => myAttendanceToday.clock('clock_in', myAttendanceToday.locations[0]?.id ?? '')}
		>
			{text.clockIn}
		</DropdownMenu.Item>
	{:else if myAttendanceToday.nextKind === 'clock_out'}
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
		disabled={myAttendanceToday.nextKind === 'clock_in' || myAttendanceToday.isSubmitting}
		onclick={() => myAttendanceToday.clock('clock_out', '')}
	>
		{text.clockOut}
	</DropdownMenu.Item>
{:else if myAttendanceToday.loadFailure}
	<DropdownMenu.Item disabled>
		{text.clockUnavailable}
	</DropdownMenu.Item>
	<DropdownMenu.Item class="text-muted-foreground text-xs" disabled>
		{myAttendanceToday.loadFailure}
	</DropdownMenu.Item>
{/if}
