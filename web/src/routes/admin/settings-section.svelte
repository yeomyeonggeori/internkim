<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import { apiErrorMessage, fetchWorkspaceSettings, updateWorkspaceSettings } from './admin-api';
	import type { AdminPageText, WorkspaceLanguage, WorkspaceSettings } from './admin-types';

	type SettingsSectionProps = {
		adminBaseURL: string;
		text: AdminPageText;
	};

	let { adminBaseURL, text }: SettingsSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let workspaceSettings = $state<WorkspaceSettings>({ timeZone: '', language: 'ko' });
	let workspaceSettingsDraft = $state<WorkspaceSettings>({ timeZone: '', language: 'ko' });
	let workspaceSettingsMessage = $state('');
	let isLoadingWorkspaceSettings = $state(false);
	let isSavingWorkspaceSettings = $state(false);

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadWorkspaceSettings();
	});

	function workspaceLanguageOptions(): { value: WorkspaceLanguage; label: string }[] {
		return [
			{ value: 'ko', label: text.settings.workspaceLanguageKorean },
			{ value: 'en', label: text.settings.workspaceLanguageEnglish }
		];
	}

	function normalizeWorkspaceSettings(settings: WorkspaceSettings): WorkspaceSettings {
		return {
			timeZone: settings.timeZone?.trim() ?? '',
			language: settings.language === 'en' ? 'en' : 'ko',
			updatedAt: settings.updatedAt
		};
	}

	async function loadWorkspaceSettings() {
		if (!adminBaseURL) return;

		isLoadingWorkspaceSettings = true;
		workspaceSettingsMessage = '';
		try {
			workspaceSettings = normalizeWorkspaceSettings(await fetchWorkspaceSettings(adminBaseURL, text.settings.loadError));
			workspaceSettingsDraft = { ...workspaceSettings };
		} catch {
			workspaceSettingsMessage = text.settings.loadError;
		} finally {
			isLoadingWorkspaceSettings = false;
		}
	}

	async function saveWorkspaceSettings() {
		if (!adminBaseURL) return;

		isSavingWorkspaceSettings = true;
		workspaceSettingsMessage = '';
		try {
			workspaceSettings = normalizeWorkspaceSettings(await updateWorkspaceSettings(adminBaseURL, workspaceSettingsDraft, text.settings.saveError));
			workspaceSettingsDraft = { ...workspaceSettings };
			workspaceSettingsMessage = text.settings.saveSuccess;
		} catch (error) {
			workspaceSettingsMessage = apiErrorMessage(error, text.settings.saveError);
		} finally {
			isSavingWorkspaceSettings = false;
		}
	}

</script>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.settings.title}</Card.Title>
		<Card.Description>{text.settings.description}</Card.Description>
	</Card.Header>
	<Card.Content>
		<Field.Group class="@container/field-group">
			<div class="grid gap-5 md:grid-cols-2">
				<Field.Field>
					<Field.Label for="workspace-time-zone">{text.settings.timeZone}</Field.Label>
					<Input
						id="workspace-time-zone"
						bind:value={workspaceSettingsDraft.timeZone}
						placeholder={text.settings.timeZonePlaceholder}
						disabled={isLoadingWorkspaceSettings}
						autocomplete="off"
					/>
					<Field.Description>{text.settings.timeZoneHint}</Field.Description>
				</Field.Field>
				<Field.Field>
					<Field.Label for="workspace-language">{text.settings.workspaceLanguageTitle}</Field.Label>
					<Select.Root type="single" bind:value={workspaceSettingsDraft.language} disabled={isLoadingWorkspaceSettings || isSavingWorkspaceSettings}>
						<Select.Trigger id="workspace-language" class="w-full">
							{workspaceLanguageOptions().find((option) => option.value === workspaceSettingsDraft.language)?.label}
						</Select.Trigger>
						<Select.Content><Select.Group>
							{#each workspaceLanguageOptions() as option (option.value)}
								<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
							{/each}
						</Select.Group></Select.Content>
					</Select.Root>
					<Field.Description>{text.settings.workspaceLanguageDescription}</Field.Description>
				</Field.Field>
			</div>
			{#if workspaceSettingsMessage}
				<Field.Description>{workspaceSettingsMessage}</Field.Description>
			{/if}
		</Field.Group>
	</Card.Content>
	<Card.Footer class="justify-end">
		<Button disabled={isLoadingWorkspaceSettings || isSavingWorkspaceSettings} onclick={saveWorkspaceSettings}>
			{#if isSavingWorkspaceSettings}
				<LoaderIcon class="size-4 animate-spin" />
			{/if}
			{text.settings.save}
		</Button>
	</Card.Footer>
</Card.Root>
