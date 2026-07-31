<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Switch } from '$lib/components/ui/switch';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { CompanyHolidaySettingsState } from './company-holiday-settings-state.svelte';
	import type { AdminPageText, CompanyHoliday } from './admin-types';

	type Props = {
		adminBaseURL: string;
		text: AdminPageText;
		onChanged?: () => void;
	};

	let { adminBaseURL, text, onChanged = () => {} }: Props = $props();
	const settingsState = new CompanyHolidaySettingsState();
	let removalConfirmationOpen = $state(false);

	$effect(() => {
		settingsState.sync(adminBaseURL, text.companyHolidays);
	});

	function holidayDateLabel(holiday: CompanyHoliday): string {
		const [year, month, day] = holiday.date.split('-').map(Number);
		const locale = currentLocale.value === 'ko' ? 'ko-KR' : 'en-US';
		const options: Intl.DateTimeFormatOptions = holiday.recursAnnually
			? { month: 'long', day: 'numeric', timeZone: 'UTC' }
			: { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'UTC' };
		const formatted = new Intl.DateTimeFormat(locale, options).format(
			new Date(Date.UTC(year, month - 1, day))
		);
		const recurrence = holiday.recursAnnually
			? text.companyHolidays.annual
			: text.companyHolidays.oneTime;
		return `${formatted} · ${recurrence}`;
	}

	async function saveHoliday(): Promise<void> {
		if (await settingsState.save()) onChanged();
	}

	async function removeHoliday(): Promise<void> {
		if (await settingsState.remove()) onChanged();
	}
</script>

<Card.Root data-testid="company-holiday-settings">
	<Card.Header>
		<div class="flex items-start justify-between gap-4">
			<div class="space-y-1">
				<Card.Title>{text.companyHolidays.title}</Card.Title>
				<Card.Description>{text.companyHolidays.description}</Card.Description>
			</div>
			{#if !settingsState.draft}
				<Button onclick={() => settingsState.startCreate()}>{text.companyHolidays.add}</Button>
			{/if}
		</div>
	</Card.Header>
	<Card.Content class="grid gap-4">
		{#if settingsState.draft}
			<div class="grid gap-5 rounded-lg border p-4" data-testid="company-holiday-editor">
				<h3 class="font-semibold">
					{settingsState.draft.id ? text.companyHolidays.editTitle : text.companyHolidays.createTitle}
				</h3>
				<div class="grid gap-4 md:grid-cols-[minmax(0,3fr)_minmax(12rem,2fr)]">
					<Field.Field data-invalid={settingsState.validationAttempted && !settingsState.draft.title.trim()}>
						<Field.Label for="company-holiday-title">{text.companyHolidays.name}</Field.Label>
						<Input
							id="company-holiday-title"
							value={settingsState.draft.title}
							oninput={(event) => settingsState.updateDraft({ title: event.currentTarget.value })}
							placeholder={text.companyHolidays.namePlaceholder}
							disabled={settingsState.isSaving}
							aria-invalid={settingsState.validationAttempted && !settingsState.draft.title.trim()}
						/>
					</Field.Field>
					<Field.Field data-invalid={settingsState.validationAttempted && !settingsState.draft.date}>
						<Field.Label for="company-holiday-date">{text.companyHolidays.date}</Field.Label>
						<Input
							id="company-holiday-date"
							type="date"
							value={settingsState.draft.date}
							oninput={(event) => settingsState.updateDraft({ date: event.currentTarget.value })}
							disabled={settingsState.isSaving}
							aria-invalid={settingsState.validationAttempted && !settingsState.draft.date}
						/>
					</Field.Field>
				</div>
				<Field.Field orientation="horizontal">
					<Field.Content>
						<Field.Label for="company-holiday-annual">
							{text.companyHolidays.recursAnnually}
						</Field.Label>
						<Field.Description>{text.companyHolidays.recurrenceDescription}</Field.Description>
					</Field.Content>
					<Switch
						id="company-holiday-annual"
						checked={settingsState.draft.recursAnnually}
						onCheckedChange={(checked) => settingsState.updateDraft({ recursAnnually: checked })}
						disabled={settingsState.isSaving}
					/>
				</Field.Field>
				<div class="flex items-center justify-between gap-3">
					<div>
						{#if settingsState.draft.id}
							<Button
								variant="destructive"
								onclick={() => (removalConfirmationOpen = true)}
								disabled={settingsState.isSaving}
							>
								{text.companyHolidays.remove}
							</Button>
						{/if}
					</div>
					<div class="flex gap-2">
						<Button variant="outline" onclick={() => settingsState.cancel()} disabled={settingsState.isSaving}>
							{text.companyHolidays.cancel}
						</Button>
						<Button onclick={() => void saveHoliday()} disabled={settingsState.isSaving}>
							{text.companyHolidays.save}
						</Button>
					</div>
				</div>
			</div>
		{/if}

		{#if settingsState.isLoading}
			<p class="text-sm text-muted-foreground">{text.companyHolidays.loading}</p>
		{:else if settingsState.holidays.length === 0 && !settingsState.draft}
			<p class="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
				{text.companyHolidays.empty}
			</p>
		{:else}
			<div class="grid gap-2">
				{#each settingsState.holidays as holiday (holiday.id)}
					<div class="flex items-center justify-between gap-4 rounded-lg border p-3">
						<div class="flex min-w-0 items-center gap-3">
							<span class="size-2 shrink-0 rounded-full bg-red-500"></span>
							<div class="min-w-0">
								<p class="truncate font-medium">{holiday.title}</p>
								<p class="text-sm text-muted-foreground">{holidayDateLabel(holiday)}</p>
							</div>
						</div>
						<Button
							variant="outline"
							size="sm"
							onclick={() => settingsState.startEdit(holiday)}
							disabled={settingsState.isSaving}
						>
							{text.companyHolidays.edit}
						</Button>
					</div>
				{/each}
			</div>
		{/if}

		{#if settingsState.message}
			<p class="text-sm text-muted-foreground" role="status">{settingsState.message}</p>
		{/if}
	</Card.Content>
</Card.Root>

<AlertDialog.Root
	open={removalConfirmationOpen}
	onOpenChange={(open) => (removalConfirmationOpen = open)}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.companyHolidays.removeConfirmationTitle}</AlertDialog.Title>
			<AlertDialog.Description>
				{text.companyHolidays.removeConfirmationDescription}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>{text.companyHolidays.cancel}</AlertDialog.Cancel>
			<AlertDialog.Action
				onclick={() => {
					removalConfirmationOpen = false;
					void removeHoliday();
				}}
			>
				{text.companyHolidays.remove}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
