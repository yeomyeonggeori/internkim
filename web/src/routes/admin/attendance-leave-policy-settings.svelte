<script lang="ts">
	import AttendanceLeavePolicyEditor from './attendance-leave-policy-editor.svelte';
	import AttendanceLeavePolicyList from './attendance-leave-policy-list.svelte';
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
	import type { AdminPageText, AttendanceLeavePolicy, LeaveType } from './admin-types';

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
			draft = policy.leaveTypes[0] ? copyLeaveType(policy.leaveTypes[0]) : null;
			previousSelectedID = draft?.id ?? '';
		} catch (error) {
			message = apiErrorMessage(error, text.attendanceSettings.loadError);
		} finally {
			isLoading = false;
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
</script>

<div
	data-testid="attendance-leave-policy-settings"
	class="grid gap-5 lg:grid-cols-[minmax(260px,0.8fr)_minmax(0,1.2fr)]"
>
	<AttendanceLeavePolicyList
		{policy}
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
			onSave={() => void savePolicy()}
		/>
	{/if}
</div>
