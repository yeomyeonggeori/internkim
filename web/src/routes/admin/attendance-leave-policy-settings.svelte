<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import AttendanceLeaveBalanceTracking from './attendance-leave-balance-tracking.svelte';
	import AttendanceLeavePolicyEditor from './attendance-leave-policy-editor.svelte';
	import AttendanceLeavePolicyList from './attendance-leave-policy-list.svelte';
	import AttendanceLeavePolicyRemoveDialog from './attendance-leave-policy-remove-dialog.svelte';
	import {
		copyLeaveType,
		createLeaveType,
		leaveTypeIsValid
	} from './attendance-leave-policy-model';
	import {
		apiErrorMessage,
		fetchAttendanceLeavePolicy,
		updateAttendanceLeavePolicy
	} from './admin-api';
	import type {
		AdminPageText,
		AttendanceLeavePolicy,
		LeaveBalanceTrackingMode,
		LeaveType
	} from './admin-types';

	type LeavePolicySettingsProps = {
		adminBaseURL: string;
		text: AdminPageText;
	};

	let { adminBaseURL, text }: LeavePolicySettingsProps = $props();
	let policy = $state<AttendanceLeavePolicy | null>(null);
	let draft = $state<LeaveType | null>(null);
	let loadedAdminBaseURL = $state('');
	let message = $state('');
	let isLoading = $state(false);
	let isSaving = $state(false);
	let validationAttempted = $state(false);
	let previousSelectedID = $state('');
	let expiryConfirmationOpen = $state(false);
	let removalConfirmationOpen = $state(false);
	let balanceTrackingMode = $state<LeaveBalanceTrackingMode>('managed');

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		void loadPolicy();
	});

	async function loadPolicy(): Promise<void> {
		isLoading = true;
		message = '';
		try {
			policy = await fetchAttendanceLeavePolicy(
				adminBaseURL,
				text.attendanceSettings.loadError
			);
			balanceTrackingMode = policy.balanceTrackingMode;
			draft = policy.leaveTypes[0] ? copyLeaveType(policy.leaveTypes[0]) : null;
			previousSelectedID = draft?.id ?? '';
		} catch (error) {
			message = apiErrorMessage(error, text.attendanceSettings.loadError);
		} finally {
			isLoading = false;
		}
	}

	async function saveBalanceTrackingMode(): Promise<void> {
		if (!policy || balanceTrackingMode === policy.balanceTrackingMode) return;
		isSaving = true;
		message = '';
		try {
			policy = await updateAttendanceLeavePolicy(
				adminBaseURL,
				{ ...policy, balanceTrackingMode },
				text.attendanceSettings.saveError
			);
			balanceTrackingMode = policy.balanceTrackingMode;
			message = text.attendanceSettings.saveSuccess;
		} catch (error) {
			message = apiErrorMessage(error, text.attendanceSettings.saveError);
		} finally {
			isSaving = false;
		}
	}

	function selectLeaveType(leaveType: LeaveType): void {
		draft = copyLeaveType(leaveType);
		previousSelectedID = leaveType.id;
		validationAttempted = false;
		message = '';
	}

	function addLeaveType(): void {
		previousSelectedID = draft?.id ?? previousSelectedID;
		draft = createLeaveType(policy?.leaveTypes.length ?? 0);
		validationAttempted = false;
		message = '';
	}

	function draftHasChanges(): boolean {
		if (!policy || !draft) return false;
		const currentDraft = draft;
		if (!currentDraft.id) return true;
		const savedLeaveType = policy.leaveTypes.find(
			(leaveType) => leaveType.id === currentDraft.id
		);
		return !savedLeaveType || JSON.stringify(savedLeaveType) !== JSON.stringify(currentDraft);
	}

	function cancelChanges(): void {
		if (!policy || !draft || !draftHasChanges()) return;
		const currentDraft = draft;
		const savedLeaveType = currentDraft.id
			? policy.leaveTypes.find((leaveType) => leaveType.id === currentDraft.id)
			: policy.leaveTypes.find((leaveType) => leaveType.id === previousSelectedID);
		draft = savedLeaveType ? copyLeaveType(savedLeaveType) : null;
		previousSelectedID = draft?.id ?? '';
		validationAttempted = false;
		message = '';
	}

	function expiryPolicyHasChanges(): boolean {
		if (!policy || !draft?.id) return false;
		const saved = policy.leaveTypes.find((leaveType) => leaveType.id === draft?.id);
		if (!saved) return false;
		return (
			saved.expiryMode !== draft.expiryMode ||
			saved.expiryMonths !== draft.expiryMonths ||
			saved.carryoverEnabled !== draft.carryoverEnabled ||
			saved.carryoverLimitMilliDays !== draft.carryoverLimitMilliDays
		);
	}

	function requestSave(): void {
		if (expiryPolicyHasChanges()) {
			expiryConfirmationOpen = true;
			return;
		}
		void savePolicy();
	}

	async function savePolicy(): Promise<void> {
		if (!policy || !draft) return;
		const draftToSave = copyLeaveType(draft);
		validationAttempted = true;
		if (!leaveTypeIsValid(draftToSave)) {
			message = !draftToSave.name.trim()
				? text.attendanceSettings.requiredName
				: draftToSave.allowedUnits.length === 0
					? text.attendanceSettings.atLeastOneUnit
					: text.attendanceSettings.invalidPolicy;
			return;
		}
		const leaveTypes = draftToSave.id
			? policy.leaveTypes.map((leaveType) =>
					leaveType.id === draftToSave.id ? draftToSave : leaveType
				)
			: [...policy.leaveTypes, draftToSave];
		isSaving = true;
		message = '';
		try {
			policy = await updateAttendanceLeavePolicy(
				adminBaseURL,
				{ ...policy, leaveTypes },
				text.attendanceSettings.saveError
			);
			const savedLeaveType = draftToSave.id
				? policy.leaveTypes.find((leaveType) => leaveType.id === draftToSave.id)
				: policy.leaveTypes.find((leaveType) => leaveType.name === draftToSave.name);
			draft = copyLeaveType(savedLeaveType ?? policy.leaveTypes[0]);
			previousSelectedID = draft.id;
			validationAttempted = false;
			message = text.attendanceSettings.saveSuccess;
		} catch (error) {
			message = apiErrorMessage(error, text.attendanceSettings.saveError);
		} finally {
			isSaving = false;
		}
	}

	async function removeLeaveType(): Promise<void> {
		if (!policy || !draft?.id) return;
		const removedID = draft.id;
		isSaving = true;
		message = '';
		try {
			policy = await updateAttendanceLeavePolicy(
				adminBaseURL,
				{
					...policy,
					leaveTypes: policy.leaveTypes.filter((leaveType) => leaveType.id !== removedID)
				},
				text.attendanceSettings.removeError
			);
			const nextLeaveType = policy.leaveTypes.find((leaveType) => leaveType.isActive);
			draft = nextLeaveType ? copyLeaveType(nextLeaveType) : null;
			previousSelectedID = draft?.id ?? '';
			validationAttempted = false;
			removalConfirmationOpen = false;
			message = text.attendanceSettings.removeSuccess;
		} catch (error) {
			message = apiErrorMessage(error, text.attendanceSettings.removeError);
		} finally {
			isSaving = false;
		}
	}
</script>

<div data-testid="attendance-leave-policy-settings" class="space-y-5">
	{#if policy}
		<AttendanceLeaveBalanceTracking
			mode={balanceTrackingMode}
			savedMode={policy.balanceTrackingMode}
			{isSaving}
			{text}
			onChange={(mode) => (balanceTrackingMode = mode)}
			onCancel={() => (balanceTrackingMode = policy?.balanceTrackingMode ?? 'managed')}
			onSave={() => void saveBalanceTrackingMode()}
		/>
	{/if}

	<div
		class="grid gap-5 lg:h-[clamp(36rem,calc(100dvh-26rem),52rem)] lg:grid-cols-[minmax(260px,0.8fr)_minmax(0,1.2fr)] lg:items-stretch"
	>
		<AttendanceLeavePolicyList
			{policy}
			pendingDraft={draft && !draft.id ? draft : null}
			selectedID={draft?.id ?? ''}
			{message}
			{isLoading}
			{isSaving}
			{text}
			onAdd={addLeaveType}
			onSelect={selectLeaveType}
		/>

		{#if draft}
			<AttendanceLeavePolicyEditor
				{draft}
				{text}
				{isSaving}
				{validationAttempted}
				hasChanges={draftHasChanges()}
				onChange={(nextDraft) => (draft = nextDraft)}
				onCancel={cancelChanges}
				onRemove={() => (removalConfirmationOpen = true)}
				onSave={requestSave}
			/>
		{/if}
	</div>
</div>

<AlertDialog.Root
	open={expiryConfirmationOpen}
	onOpenChange={(open) => (expiryConfirmationOpen = open)}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.attendanceSettings.expiryConfirmationTitle}</AlertDialog.Title>
			<AlertDialog.Description>
				{text.attendanceSettings.expiryConfirmationDescription}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>{text.attendanceSettings.expiryConfirmationCancel}</AlertDialog.Cancel>
			<AlertDialog.Action
				onclick={() => {
					expiryConfirmationOpen = false;
					void savePolicy();
				}}
			>
				{text.attendanceSettings.expiryConfirmationSave}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AttendanceLeavePolicyRemoveDialog
	open={removalConfirmationOpen}
	leaveTypeName={draft
		? localizedLeaveTypeName(draft.id, draft.name, currentLocale.value)
		: ''}
	{isSaving}
	{text}
	onOpenChange={(open) => (removalConfirmationOpen = open)}
	onConfirm={() => void removeLeaveType()}
/>
