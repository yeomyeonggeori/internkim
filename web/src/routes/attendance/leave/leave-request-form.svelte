<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Textarea } from '$lib/components/ui/textarea';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';
	import type {
		EmployeeLeavePreview,
		EmployeeLeaveType,
		EmployeeLeaveUnit
	} from './employee-leave-types';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import { milliDaysValue } from './leave-history-model';
	import LeavePartialTimeFields from './leave-partial-time-fields.svelte';
	import type { LeaveRequestDraft } from './leave-request-draft.svelte';
	import LeaveRequestPreview from './leave-request-preview.svelte';

	type Props = {
		draft: LeaveRequestDraft;
		onSubmitted: () => void;
		onClose: () => void;
	};

	let { draft, onSubmitted, onClose }: Props = $props();

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const employeeLeave = getEmployeeLeaveState();
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	const selectableLeaveTypes = $derived(
		(employeeLeave.payload?.leaveTypes ?? []).filter((leaveType) => leaveType.isActive)
	);
	const selectedLeaveType = $derived(
		selectableLeaveTypes.find((leaveType) => leaveType.id === draft.leaveTypeID)
	);
	let preview = $state<EmployeeLeavePreview | null>(null);
	let isPreviewLoading = $state(false);
	let previewErrorMessage = $state('');
	let previewRequestKey = '';
	let previewSequence = 0;

	$effect(() => {
		draft.synchronize(employeeLeave.payload?.leaveTypes ?? [], today);
	});

	$effect(() => {
		const request = draft.previewRequest();
		const requestKey = request ? JSON.stringify(request) : '';
		if (!requestKey) {
			previewSequence++;
			preview = null;
			previewErrorMessage = '';
			isPreviewLoading = false;
			previewRequestKey = '';
			return;
		}
		if (requestKey === previewRequestKey) return;
		if (!request) return;
		previewRequestKey = requestKey;
		const sequence = ++previewSequence;
		preview = null;
		previewErrorMessage = '';
		isPreviewLoading = true;
		void employeeLeave
			.preview(request)
			.then((result) => {
				if (sequence !== previewSequence) return;
				preview = result;
			})
			.catch((error) => {
				if (sequence !== previewSequence) return;
				previewErrorMessage = employeeLeave.localizedErrorMessage(
					error,
					text.leave.previewFailed
				);
			})
			.finally(() => {
				if (sequence === previewSequence) isPreviewLoading = false;
			});
	});

	function unitLabel(unit: EmployeeLeaveUnit): string {
		if (unit === 'halfDay') return text.leave.unitHalfDay;
		if (unit === 'quarterDay') return text.leave.unitQuarterDay;
		return text.leave.unitFullDay;
	}

	function leaveTypeName(leaveType: EmployeeLeaveType): string {
		return localizedLeaveTypeName(leaveType.id, leaveType.name, currentLocale.value);
	}

	function selectLeaveType(leaveType: EmployeeLeaveType): void {
		draft.setLeaveType(leaveType);
	}

	function selectedLeaveBalanceText(leaveType: EmployeeLeaveType): string {
		if (employeeLeave.payload?.balanceTrackingMode === 'unlimited') {
			return text.leave.selectedBalanceUnlimited;
		}
		if (leaveType.balanceMode === 'none') return text.leave.selectedBalanceUntracked;
		const available = `${milliDaysValue(leaveType.balance?.availableMilliDays ?? 0)}${text.leave.dayUnit}`;
		const reserved = `${milliDaysValue(leaveType.balance?.reservedMilliDays ?? 0)}${text.leave.dayUnit}`;
		if (leaveType.balanceMode === 'annual' && leaveType.id !== 'annual') {
			return text.leave.selectedAnnualBalance.replace('{available}', available);
		}
		return text.leave.selectedBalance
			.replace('{available}', available)
			.replace('{reserved}', reserved);
	}

	async function submit(): Promise<void> {
		const submission = draft.submission();
		if (!submission || !preview || employeeLeave.isMutating) return;
		try {
			await employeeLeave.create(submission);
		} catch {
			return;
		}
		draft.reset(employeeLeave.payload?.leaveTypes ?? [], today);
		preview = null;
		previewRequestKey = '';
		onSubmitted();
	}
</script>

<form
	class="flex min-h-full flex-col"
	data-testid="leave-request-form"
	onsubmit={(event) => {
		event.preventDefault();
		void submit();
	}}
>
	<div class="grid gap-5 px-4 py-5 sm:px-6">
		<div class="grid gap-4 lg:grid-cols-2">
			<label class="grid gap-1.5 text-sm font-medium">
				<span>{text.leave.leaveTypeLabel}</span>
				<Select.Root type="single" bind:value={draft.leaveTypeID} disabled={employeeLeave.isMutating}>
					<Select.Trigger class="w-full" data-testid="leave-request-type-trigger">
						{selectedLeaveType ? leaveTypeName(selectedLeaveType) : text.leave.selectLeaveType}
					</Select.Trigger>
					<Select.Content>
						{#each selectableLeaveTypes as leaveType (leaveType.id)}
							<Select.Item
								value={leaveType.id}
								label={leaveTypeName(leaveType)}
								onclick={() => selectLeaveType(leaveType)}
							>
								{leaveTypeName(leaveType)}
							</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
				{#if selectedLeaveType}
					<span class="text-xs font-normal text-muted-foreground">
						{selectedLeaveBalanceText(selectedLeaveType)}
					</span>
				{/if}
			</label>

			<div class="grid content-start gap-1.5">
				<p class="text-sm font-medium">{text.leave.unitLabel}</p>
				<div
					class="grid min-h-12 rounded-lg border p-0.5 sm:min-h-8"
					data-testid="leave-request-unit-control"
					style:grid-template-columns={`repeat(${Math.max(1, selectedLeaveType?.allowedUnits.length ?? 1)}, minmax(0, 1fr))`}
				>
					{#each selectedLeaveType?.allowedUnits ?? [] as unit (unit)}
						<Button
							type="button"
							variant={draft.unit === unit ? 'default' : 'ghost'}
							size="sm"
							class="h-full w-full"
							onclick={() => draft.setUnit(unit)}
							disabled={employeeLeave.isMutating}
						>
							{unitLabel(unit)}
						</Button>
					{/each}
				</div>
			</div>
		</div>

		<div class="grid gap-4 sm:grid-cols-2">
			<label class="grid gap-1.5 text-sm font-medium" data-testid="leave-request-date-field">
				<span>{draft.unit === 'fullDay' ? text.leave.startDateLabel : text.leave.dateLabel}</span>
				<Input
					type="date"
					value={draft.startDate}
					min={today}
					required
					disabled={employeeLeave.isMutating}
					oninput={(event) => {
						if (event.currentTarget instanceof HTMLInputElement) {
							draft.setStartDate(event.currentTarget.value);
						}
					}}
				/>
			</label>

			{#if draft.unit === 'fullDay'}
				<label class="grid gap-1.5 text-sm font-medium">
					<span>{text.leave.endDateLabel}</span>
					<Input
						type="date"
						bind:value={draft.endDate}
						min={draft.startDate || today}
						required
						disabled={employeeLeave.isMutating}
					/>
				</label>
			{:else if draft.unit === 'halfDay'}
				<LeavePartialTimeFields
					{draft}
					disabled={employeeLeave.isMutating}
					field="period"
					text={text.leave}
				/>
			{:else}
				<LeavePartialTimeFields
					{draft}
					disabled={employeeLeave.isMutating}
					field="startTime"
					text={text.leave}
				/>
			{/if}

			{#if draft.unit === 'halfDay' && draft.partialPeriod === 'custom'}
				<LeavePartialTimeFields
					{draft}
					disabled={employeeLeave.isMutating}
					field="startTime"
					text={text.leave}
				/>
			{/if}
		</div>

		<label class="grid gap-1.5 text-sm font-medium">
			<span>{text.leave.reasonLabel}</span>
			<Textarea
				class="min-h-24 resize-none"
				bind:value={draft.reason}
				placeholder={text.leave.reasonPlaceholder}
				maxlength={500}
				disabled={employeeLeave.isMutating}
			/>
			<span class="text-right text-xs font-normal tabular-nums text-muted-foreground">
				{draft.reason.length} / 500
			</span>
		</label>

		<LeaveRequestPreview
			{preview}
			isLoading={isPreviewLoading}
			errorMessage={previewErrorMessage}
			text={text.leave}
		/>

		{#if employeeLeave.mutationErrorMessage}
			<p class="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive">
				{employeeLeave.mutationErrorMessage}
			</p>
		{/if}
	</div>

	<div
		class="sticky bottom-0 mt-auto flex flex-col-reverse gap-2 border-t bg-popover/95 px-4 pt-4 pb-[max(1rem,env(safe-area-inset-bottom))] backdrop-blur-sm sm:flex-row sm:justify-end sm:px-6 sm:pb-4"
	>
		<Button type="button" variant="outline" class="sm:min-w-32" onclick={onClose}>
			{text.cancel}
		</Button>
		<Button
			type="submit"
			class="sm:min-w-36"
			disabled={
				employeeLeave.isMutating ||
				isPreviewLoading ||
				!preview ||
				preview.totalDeductionMilliDays <= 0
			}
		>
			{employeeLeave.isMutating ? text.leave.submitting : text.leave.submitAction}
		</Button>
	</div>
</form>
