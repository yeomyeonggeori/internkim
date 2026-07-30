<script lang="ts">
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';
	import { getLeaveManagementState } from './leave-management-state.svelte';

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const management = getLeaveManagementState();

	let isOpen = $state(false);
	let leaveTypeID = $state('');
	let unit = $state<'fullDay' | 'halfDay' | 'quarterDay'>('fullDay');
	let startDate = $state('');
	let endDate = $state('');
	let startTime = $state('');
	let reason = $state('');

	const leaveTypes = $derived(
		(management.payload?.leaveTypes ?? []).filter((leaveType) => leaveType.isActive)
	);
	const selectedLeaveType = $derived(
		leaveTypes.find((leaveType) => leaveType.id === leaveTypeID)
	);
	const allowedUnits = $derived(selectedLeaveType?.allowedUnits ?? ['fullDay']);
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));

	function reset(): void {
		leaveTypeID = leaveTypes[0]?.id ?? '';
		unit = 'fullDay';
		startDate = today;
		endDate = startDate;
		startTime = '';
		reason = '';
	}

	function handleOpenChange(open: boolean): void {
		isOpen = open;
		if (open) reset();
	}

	function leaveTypeName(id: string, name: string): string {
		return localizedLeaveTypeName(id, name, currentLocale.value);
	}

	$effect(() => {
		if (!allowedUnits.includes(unit)) {
			unit = (allowedUnits[0] ?? 'fullDay') as typeof unit;
		}
	});

	async function submit(): Promise<void> {
		if (
			!management.selectedEmployeeEmail ||
			!leaveTypeID ||
			!startDate ||
			(unit !== 'fullDay' && !startTime)
		) {
			return;
		}
		await management.addPastLeave({
			employeeEmail: management.selectedEmployeeEmail,
			leaveTypeID,
			unit,
			startDate,
			endDate: unit === 'fullDay' ? endDate || startDate : startDate,
			partialPeriod: unit === 'fullDay' ? '' : 'custom',
			startTime: unit === 'fullDay' ? '' : startTime,
			reason: reason.trim()
		});
		isOpen = false;
	}
</script>

<Dialog.Root open={isOpen} onOpenChange={handleOpenChange}>
	<Dialog.Trigger class={buttonVariants()} disabled={!management.selectedEmployeeEmail}>
		{text.management.addPastLeaveAction}
	</Dialog.Trigger>
	<Dialog.Content class="sm:max-w-lg" data-testid="past-leave-dialog">
		<Dialog.Header>
			<Dialog.Title>{text.management.addPastLeaveTitle}</Dialog.Title>
			<Dialog.Description>{text.management.addPastLeaveDescription}</Dialog.Description>
		</Dialog.Header>

		<div class="grid gap-4 py-2">
			<label class="grid gap-1.5 text-sm font-medium">
				{text.management.leaveType}
				<Select.Root type="single" bind:value={leaveTypeID}>
					<Select.Trigger class="w-full">
						{selectedLeaveType
							? leaveTypeName(selectedLeaveType.id, selectedLeaveType.name)
							: text.management.selectLeaveType}
					</Select.Trigger>
					<Select.Content>
						{#each leaveTypes as leaveType (leaveType.id)}
							<Select.Item
								value={leaveType.id}
								label={leaveTypeName(leaveType.id, leaveType.name)}
							>
								{leaveTypeName(leaveType.id, leaveType.name)}
							</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</label>

			<label class="grid gap-1.5 text-sm font-medium">
				{text.management.unit}
				<Select.Root type="single" bind:value={unit}>
					<Select.Trigger class="w-full">
						{unit === 'fullDay'
							? text.management.fullDay
							: unit === 'halfDay'
								? text.management.halfDay
								: text.management.quarterDay}
					</Select.Trigger>
					<Select.Content>
						{#if allowedUnits.includes('fullDay')}
							<Select.Item value="fullDay" label={text.management.fullDay}>
								{text.management.fullDay}
							</Select.Item>
						{/if}
						{#if allowedUnits.includes('halfDay')}
							<Select.Item value="halfDay" label={text.management.halfDay}>
								{text.management.halfDay}
							</Select.Item>
						{/if}
						{#if allowedUnits.includes('quarterDay')}
							<Select.Item value="quarterDay" label={text.management.quarterDay}>
								{text.management.quarterDay}
							</Select.Item>
						{/if}
					</Select.Content>
				</Select.Root>
			</label>

			<div class="grid grid-cols-2 gap-3">
				<label class="grid gap-1.5 text-sm font-medium">
					{text.management.startDate}
					<Input type="date" bind:value={startDate} max={today} />
				</label>
				{#if unit === 'fullDay'}
					<label class="grid gap-1.5 text-sm font-medium">
						{text.management.endDate}
						<Input type="date" bind:value={endDate} min={startDate} max={today} />
					</label>
				{:else}
					<label class="grid gap-1.5 text-sm font-medium">
						{text.management.startTime}
						<Input type="time" bind:value={startTime} />
					</label>
				{/if}
			</div>

			<label class="grid gap-1.5 text-sm font-medium">
				{text.management.reasonOptional}
				<Input bind:value={reason} placeholder={text.management.pastLeaveReasonPlaceholder} />
			</label>
		</div>

		<Dialog.Footer>
			<Dialog.Close>
				<Button variant="outline">{text.cancel}</Button>
			</Dialog.Close>
			<Button
				onclick={submit}
				disabled={management.isMutating ||
					!leaveTypeID ||
					!startDate ||
					(unit !== 'fullDay' && !startTime)}
			>
				{text.management.addPastLeaveSubmit}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
