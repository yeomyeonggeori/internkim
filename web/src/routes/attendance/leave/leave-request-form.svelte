<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Textarea } from '$lib/components/ui/textarea';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { getAttendanceViewState } from '../attendance-view-state.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';
	import type {
		EmployeeLeavePreview,
		EmployeeLeaveType,
		EmployeeLeaveUnit
	} from './employee-leave-types';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import LeaveEvidencePicker from './leave-evidence-picker.svelte';
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
	const attendanceView = getAttendanceViewState();
	const employeeLeave = getEmployeeLeaveState();
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	const selectableLeaveTypes = $derived(
		(employeeLeave.payload?.leaveTypes ?? []).filter(
			(leaveType) => leaveType.isActive || (draft.requestID && leaveType.id === draft.leaveTypeID)
		)
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

	function selectLeaveType(leaveType: EmployeeLeaveType): void {
		draft.setLeaveType(leaveType);
	}

	async function submit(): Promise<void> {
		const submission = draft.submission();
		if (!submission || !preview || employeeLeave.isMutating) return;
		try {
			switch (submission.mode) {
				case 'create':
					await employeeLeave.create(submission.request, draft.attachments);
					break;
				case 'edit':
					await employeeLeave.update(draft.requestID, submission.request, draft.attachments);
					break;
				case 'resubmit':
					await employeeLeave.resubmit(draft.requestID, submission.request, draft.attachments);
					break;
			}
			draft.reset(employeeLeave.payload?.leaveTypes ?? [], today);
			preview = null;
			previewRequestKey = '';
			onSubmitted();
		} catch {
			if (employeeLeave.requestMutationConflict) {
				attendanceView.select('leaveHistory');
				onClose();
			}
			return;
		}
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
		{#if draft.mode !== 'create'}
			<div class="rounded-lg bg-info/10 px-4 py-3 text-sm text-info">
				<p class="font-medium">
					{draft.mode === 'edit' ? text.leave.editTitle : text.leave.resubmitTitle}
				</p>
				<p class="mt-1 text-xs opacity-80">
					{draft.mode === 'edit' ? text.leave.editDescription : text.leave.resubmitDescription}
				</p>
			</div>
		{/if}

		<div class="grid gap-4 lg:grid-cols-2">
			<label class="grid gap-1.5 text-sm font-medium">
				<span>{text.leave.leaveTypeLabel}</span>
				<Select.Root type="single" bind:value={draft.leaveTypeID} disabled={employeeLeave.isMutating}>
					<Select.Trigger class="w-full">
						{selectedLeaveType?.name ?? text.leave.selectLeaveType}
					</Select.Trigger>
					<Select.Content>
						{#each selectableLeaveTypes as leaveType (leaveType.id)}
							<Select.Item
								value={leaveType.id}
								label={leaveType.name}
								onclick={() => selectLeaveType(leaveType)}
							>
								{leaveType.name}
							</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</label>

			<div class="grid gap-1.5">
				<p class="text-sm font-medium">{text.leave.unitLabel}</p>
				<div
					class="grid rounded-lg border p-0.5"
					style:grid-template-columns={`repeat(${Math.max(1, selectedLeaveType?.allowedUnits.length ?? 1)}, minmax(0, 1fr))`}
				>
					{#each selectedLeaveType?.allowedUnits ?? [] as unit (unit)}
						<Button
							type="button"
							variant={draft.unit === unit ? 'default' : 'ghost'}
							size="sm"
							class="w-full"
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

		{#if draft.mode === 'resubmit'}
			<label class="grid gap-1.5 text-sm font-medium">
				<span>{text.leave.responseLabel}</span>
				<Textarea
					class="min-h-20 resize-none"
					bind:value={draft.response}
					placeholder={text.leave.responsePlaceholder}
					disabled={employeeLeave.isMutating}
				/>
			</label>
		{/if}

		<LeaveEvidencePicker
			files={draft.attachments}
			onFilesChange={(files) => draft.setAttachments(files)}
			existingAttachments={draft.existingAttachments}
			onExistingAttachmentRemove={
				draft.mode === 'edit'
					? (attachmentID) => draft.removeExistingAttachment(attachmentID)
					: undefined
			}
			disabled={employeeLeave.isMutating}
			text={text.leave}
		/>

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
		class="sticky bottom-0 mt-auto flex flex-col-reverse gap-2 border-t bg-popover/95 px-4 py-4 backdrop-blur-sm sm:flex-row sm:justify-end sm:px-6"
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
			{employeeLeave.isMutating
				? text.leave.submitting
				: draft.mode === 'edit'
					? text.leave.editAction
					: draft.mode === 'resubmit'
						? text.leave.resubmitAction
						: text.leave.submitAction}
		</Button>
	</div>
</form>
