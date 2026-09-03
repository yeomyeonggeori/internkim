<script lang="ts">
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { getLeaveManagementState } from './leave-management-state.svelte';

	const text = createPageText(attendanceText);
	const management = getLeaveManagementState();

	let isOpen = $state(false);
	let leaveTypeID = $state('');
	let amountDays = $state('');

	const balanceTypes = $derived(
		(management.payload?.leaveTypes ?? []).filter(
			(leaveType) => leaveType.balanceMode !== 'none'
		)
	);

	function reset(): void {
		leaveTypeID = balanceTypes[0]?.id ?? '';
		amountDays = '';
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
			kind: 'adjustment'
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
