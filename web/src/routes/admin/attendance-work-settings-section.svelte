<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { apiErrorMessage } from './admin-api';
	import { fetchAttendanceWorkPolicy, updateAttendanceWorkPolicy } from './attendance-settings-api';
	import type {
		AdminPageText,
		AttendanceWorkPolicy,
		AttendanceWorkPolicyRevision
	} from './admin-types';
	import AttendanceWorkModeSelector from './attendance-work-mode-selector.svelte';
	import {
		currentAttendanceWorkPolicyRevision,
		sameAttendanceWorkPolicyRevision,
		setAttendanceWorkMode
	} from './attendance-work-policy-model';
	import AttendanceWorkPolicyForm from './attendance-work-policy-form.svelte';
	import AttendanceWorkPolicyPreview from './attendance-work-policy-preview.svelte';
	import CompanyHolidaySettings from './company-holiday-settings.svelte';

	type Props = {
		adminBaseURL: string;
		text: AdminPageText;
	};

	let { adminBaseURL, text }: Props = $props();
	let policy = $state<AttendanceWorkPolicy | null>(null);
	let draft = $state<AttendanceWorkPolicyRevision | null>(null);
	let hasLoaded = $state(false);
	let message = $state('');
	let isLoading = $state(false);
	let isSaving = $state(false);
	let currentMonth = $state('');
	let holidayDates = $state<string[]>([]);
	const hasChanges = $derived(
		Boolean(
			draft && policy && !sameAttendanceWorkPolicyRevision(draft, currentAttendanceWorkPolicyRevision(policy))
		)
	);

	$effect(() => {
		if (hasLoaded) return;
		hasLoaded = true;
		void loadPolicy();
	});

	async function loadPolicy(): Promise<void> {
		isLoading = true;
		message = '';
		try {
			const response = await fetchAttendanceWorkPolicy();
			policy = response.policy;
			currentMonth = response.currentMonth;
			holidayDates = response.holidayDates;
			draft = currentAttendanceWorkPolicyRevision(policy);
		} catch (error) {
			message = apiErrorMessage(error, text.workSettings.loadError);
		} finally {
			isLoading = false;
		}
	}

	function validationMessage(): string {
		if (!draft) return text.workSettings.invalidTarget;
		if (draft.workMode !== 'autonomous' && draft.workingWeekdays.length === 0) {
			return text.workSettings.invalidWeekdays;
		}
		if (
			draft.workMode !== 'autonomous' &&
			(draft.dailyTargetMinutes <= 0 ||
				draft.weeklyTargetMinutes !== draft.dailyTargetMinutes * draft.workingWeekdays.length)
		) {
			return text.workSettings.invalidTarget;
		}
		return '';
	}

	async function savePolicy(): Promise<void> {
		if (!draft) return;
		const invalidMessage = validationMessage();
		if (invalidMessage) {
			message = invalidMessage;
			return;
		}
		isSaving = true;
		message = '';
		try {
			const response = await updateAttendanceWorkPolicy(draft);
			policy = response.policy;
			currentMonth = response.currentMonth;
			holidayDates = response.holidayDates;
			draft = currentAttendanceWorkPolicyRevision(policy);
			message = text.workSettings.saveSuccess;
		} catch (error) {
			message = apiErrorMessage(error, text.workSettings.saveError);
		} finally {
			isSaving = false;
		}
	}

	async function refreshPreviewContext(): Promise<void> {
		try {
			const response = await fetchAttendanceWorkPolicy();
			currentMonth = response.currentMonth;
			holidayDates = response.holidayDates;
		} catch (error) {
			message = apiErrorMessage(error, text.workSettings.loadError);
		}
	}
</script>

<div class="@container space-y-5" data-testid="attendance-work-settings">
	<div>
		<h2 class="text-lg font-semibold">{text.workSettings.title}</h2>
		<p class="mt-1 text-sm text-muted-foreground">{text.workSettings.description}</p>
	</div>

	{#if isLoading && !draft}
		<p class="text-sm text-muted-foreground">{text.workSettings.loading}</p>
	{:else if draft}
		<div class="grid min-w-0 gap-5 @5xl:grid-cols-[minmax(0,1fr)_20rem]">
			<div class="space-y-5">
				<Card.Root>
					<Card.Header>
						<Card.Title>{text.workSettings.workMode}</Card.Title>
						<Card.Description>{text.workSettings.workModeDescription}</Card.Description>
						<Card.Action>
							<Button
								disabled={!hasChanges || isLoading || isSaving}
								onclick={() => void savePolicy()}
							>
								{text.workSettings.save}
							</Button>
						</Card.Action>
					</Card.Header>
					<Card.Content>
						<AttendanceWorkModeSelector
							value={draft.workMode}
							{text}
							disabled={isSaving}
							onChange={(mode) => (draft = setAttendanceWorkMode(draft!, mode))}
						/>
					</Card.Content>
				</Card.Root>
				<Card.Root>
					<Card.Header>
						<Card.Title>{text.workSettings[draft.workMode]}</Card.Title>
						<Card.Description>{text.workSettings[`${draft.workMode}Description`]}</Card.Description>
					</Card.Header>
					<Card.Content>
						<AttendanceWorkPolicyForm
							revision={draft}
							{text}
							disabled={isSaving}
							onChange={(revision) => (draft = revision)}
						/>
					</Card.Content>
				</Card.Root>
			</div>
			<AttendanceWorkPolicyPreview revision={draft} {text} {currentMonth} {holidayDates} />
		</div>
	{/if}

	{#if message}
		<p
			class="text-sm"
			class:text-destructive={message !== text.workSettings.saveSuccess}
			class:text-emerald-700={message === text.workSettings.saveSuccess}
			role="status"
		>
			{message}
		</p>
	{/if}

	<CompanyHolidaySettings
		{adminBaseURL}
		{text}
		onChanged={() => void refreshPreviewContext()}
	/>
</div>
