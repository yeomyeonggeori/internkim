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
	let amountDays = $state('');
	let kind = $state<'adjustment' | 'legalCorrection'>('adjustment');
	let reason = $state('');
	let effectiveOn = $state('');
	let expiresOn = $state('');

	const balanceTypes = $derived(
		(management.payload?.leaveTypes ?? []).filter(
			(leaveType) => leaveType.balanceMode !== 'none'
		)
	);

	function reset(): void {
		leaveTypeID = balanceTypes[0]?.id ?? '';
		amountDays = '';
		kind = 'adjustment';
		reason = '';
		effectiveOn = todayDateInTimeZone(attendance.summary?.timeZone);
		expiresOn = '';
	}

	function handleOpenChange(open: boolean): void {
		isOpen = open;
		if (open) reset();
	}

	function leaveTypeName(id: string, name: string): string {
		return localizedLeaveTypeName(id, name, currentLocale.value);
	}

	async function submit(): Promise<void> {
		const amountMilliDays = Math.round(Number(amountDays) * 1000);
		if (!management.selectedEmployeeEmail || !leaveTypeID || !amountMilliDays) {
			return;
		}
		await management.adjust({
			employeeEmail: management.selectedEmployeeEmail,
			leaveTypeID,
			amountMilliDays,
			kind,
			reason: reason.trim(),
			effectiveOn,
			expiresOn
		});
		isOpen = false;
	}
</script>

<Dialog.Root open={isOpen} onOpenChange={handleOpenChange}>
	<Dialog.Trigger
		class={buttonVariants({ variant: 'outline' })}
		disabled={!management.selectedEmployeeEmail}
	>
		{text.management.adjustAction}
	</Dialog.Trigger>
	<Dialog.Content class="sm:max-w-lg" data-testid="leave-adjustment-dialog">
		<Dialog.Header>
			<Dialog.Title>{text.management.adjustTitle}</Dialog.Title>
			<Dialog.Description>{text.management.adjustDescription}</Dialog.Description>
		</Dialog.Header>

		<div class="grid gap-4 py-2">
			<label class="grid gap-1.5 text-sm font-medium">
				{text.management.leaveType}
				<Select.Root type="single" bind:value={leaveTypeID}>
					<Select.Trigger class="w-full">
						{@const selectedLeaveType = balanceTypes.find(
							(leaveType) => leaveType.id === leaveTypeID
						)}
						{selectedLeaveType
							? leaveTypeName(selectedLeaveType.id, selectedLeaveType.name)
							: text.management.selectLeaveType}
					</Select.Trigger>
					<Select.Content>
						{#each balanceTypes as leaveType (leaveType.id)}
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
				{text.management.adjustKind}
				<Select.Root type="single" bind:value={kind}>
					<Select.Trigger class="w-full">
						{kind === 'legalCorrection'
							? text.management.legalCorrection
							: text.management.manualAdjustment}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value="adjustment" label={text.management.manualAdjustment}>
							{text.management.manualAdjustment}
						</Select.Item>
						<Select.Item value="legalCorrection" label={text.management.legalCorrection}>
							{text.management.legalCorrection}
						</Select.Item>
					</Select.Content>
				</Select.Root>
			</label>

			<label class="grid gap-1.5 text-sm font-medium">
				{text.management.amount}
				<Input
					type="number"
					step="0.25"
					bind:value={amountDays}
					placeholder={text.management.amountPlaceholder}
				/>
				<span class="text-xs font-normal text-muted-foreground">
					{text.management.amountHelp}
				</span>
			</label>

			<div class="grid grid-cols-2 gap-3">
				<label class="grid gap-1.5 text-sm font-medium">
					{text.management.effectiveOn}
					<Input type="date" bind:value={effectiveOn} />
				</label>
				<label class="grid gap-1.5 text-sm font-medium">
					{text.management.expiresOn}
					<Input type="date" bind:value={expiresOn} />
				</label>
			</div>

			<label class="grid gap-1.5 text-sm font-medium">
				{text.management.reasonOptional}
				<Input bind:value={reason} placeholder={text.management.reasonPlaceholder} />
			</label>
		</div>

		<Dialog.Footer>
			<Dialog.Close>
				<Button variant="outline">{text.cancel}</Button>
			</Dialog.Close>
			<Button
				onclick={submit}
				disabled={management.isMutating || !leaveTypeID || !amountDays}
			>
				{text.management.saveAdjustment}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
