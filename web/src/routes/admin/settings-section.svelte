<script lang="ts">
	import ColorPicker from '$lib/components/color-picker.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
	import * as Select from '$lib/components/ui/select';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import {
		apiErrorMessage,
		fetchAttendanceLocations,
		fetchWorkspaceSettings,
		updateAttendanceLocations,
		updateWorkspaceSettings
	} from './admin-api';
	import type { AdminPageText, AttendanceLocation, WorkspaceLanguage, WorkspaceSettings } from './admin-types';

	type SettingsSectionProps = {
		adminBaseURL: string;
		isDeviceReachable: boolean;
		text: AdminPageText;
	};

	let { adminBaseURL, isDeviceReachable, text }: SettingsSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let workspaceSettings = $state<WorkspaceSettings>({ timeZone: 'system', language: 'ko', callingCode: '82' });
	let workspaceSettingsDraft = $state<WorkspaceSettings>({ timeZone: 'system', language: 'ko', callingCode: '82' });
	let workspaceSettingsMessage = $state('');
	let isLoadingWorkspaceSettings = $state(false);
	let isSavingWorkspaceSettings = $state(false);
	let attendanceLocations = $state<AttendanceLocation[]>([]);
	let attendanceLocationsMessage = $state('');
	let isLoadingAttendanceLocations = $state(false);
	let isSavingAttendanceLocations = $state(false);

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadWorkspaceSettings();
		loadAttendanceLocations();
	});

	function workspaceLanguageOptions(): { value: WorkspaceLanguage; label: string }[] {
		return [
			{ value: 'ko', label: text.settings.workspaceLanguageKorean },
			{ value: 'en', label: text.settings.workspaceLanguageEnglish }
		];
	}

	function normalizeWorkspaceSettings(settings: WorkspaceSettings): WorkspaceSettings {
		return {
			timeZone: settings.timeZone?.trim() || 'system',
			language: settings.language === 'en' ? 'en' : 'ko',
			callingCode: settings.callingCode?.replace(/[^0-9]/g, '') || '82',
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

	async function loadAttendanceLocations() {
		if (!adminBaseURL) return;

		isLoadingAttendanceLocations = true;
		attendanceLocationsMessage = '';
		try {
			const response = await fetchAttendanceLocations(adminBaseURL, text.attendanceLocations.loadError);
			attendanceLocations = response.locations ?? [];
		} catch {
			attendanceLocationsMessage = text.attendanceLocations.loadError;
		} finally {
			isLoadingAttendanceLocations = false;
		}
	}

	async function saveAttendanceLocations() {
		if (!adminBaseURL) return;

		isSavingAttendanceLocations = true;
		attendanceLocationsMessage = '';
		try {
			const response = await updateAttendanceLocations(adminBaseURL, attendanceLocations, text.attendanceLocations.saveError);
			attendanceLocations = response.locations ?? [];
			attendanceLocationsMessage = text.attendanceLocations.saveSuccess;
		} catch (error) {
			attendanceLocationsMessage = apiErrorMessage(error, text.attendanceLocations.saveError);
		} finally {
			isSavingAttendanceLocations = false;
		}
	}

	function addAttendanceLocation() {
		attendanceLocations = [
			...attendanceLocations,
			{
				id: '',
				name: '',
				color: '#0ea5e9',
				isDefault: attendanceLocations.length === 0
			}
		];
	}

	function removeAttendanceLocation(index: number) {
		if (attendanceLocations.length <= 1) return;
		const removedLocation = attendanceLocations[index];
		const nextLocations = attendanceLocations.filter((_, locationIndex) => locationIndex !== index);
		if (removedLocation.isDefault && nextLocations[0]) {
			nextLocations[0] = { ...nextLocations[0], isDefault: true };
		}
		attendanceLocations = nextLocations;
	}

	function updateAttendanceLocation(index: number, field: keyof AttendanceLocation, value: string | boolean) {
		attendanceLocations = attendanceLocations.map((location, locationIndex) => {
			if (locationIndex !== index) return field === 'isDefault' ? { ...location, isDefault: false } : location;
			return { ...location, [field]: value };
		});
	}
</script>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.settings.title}</Card.Title>
		<Card.Description>{text.settings.description}</Card.Description>
	</Card.Header>
	<Card.Content>
		<Field.Group class="@container/field-group">
			<div class="grid gap-5 md:grid-cols-3">
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
					<Field.Label for="workspace-calling-code">{text.settings.callingCode}</Field.Label>
					<Input
						id="workspace-calling-code"
						bind:value={workspaceSettingsDraft.callingCode}
						placeholder={text.settings.callingCodePlaceholder}
						disabled={isLoadingWorkspaceSettings}
						autocomplete="off"
						inputmode="numeric"
					/>
					<Field.Description>{text.settings.callingCodeHint}</Field.Description>
				</Field.Field>
				<Field.Field>
					<Field.Label for="workspace-language">{text.settings.workspaceLanguageTitle}</Field.Label>
					<Select.Root type="single" bind:value={workspaceSettingsDraft.language} disabled={isLoadingWorkspaceSettings || isSavingWorkspaceSettings}>
						<Select.Trigger id="workspace-language" class="w-full">
							{workspaceLanguageOptions().find((option) => option.value === workspaceSettingsDraft.language)?.label}
						</Select.Trigger>
						<Select.Content>
							{#each workspaceLanguageOptions() as option (option.value)}
								<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
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
		<Button disabled={!isDeviceReachable || isLoadingWorkspaceSettings || isSavingWorkspaceSettings} onclick={saveWorkspaceSettings}>
			{#if isSavingWorkspaceSettings}
				<LoaderIcon class="size-4 animate-spin" />
			{/if}
			{text.settings.save}
		</Button>
	</Card.Footer>
</Card.Root>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.attendanceLocations.title}</Card.Title>
		<Card.Description>{text.attendanceLocations.description}</Card.Description>
		<Card.Action>
			<Button variant="outline" size="sm" onclick={addAttendanceLocation} disabled={isLoadingAttendanceLocations}>
				<PlusIcon />
				{text.attendanceLocations.add}
			</Button>
		</Card.Action>
	</Card.Header>
	<Card.Content>
		<Item.Group class="gap-2">
			{#each attendanceLocations as location, index (index)}
				<Item.Root variant="outline">
					<Item.Media>
						<ColorPicker
							value={location.color}
							label={text.attendanceLocations.color}
							onChange={(color) => updateAttendanceLocation(index, 'color', color)}
						/>
					</Item.Media>
					<Item.Content>
						<Input
							value={location.name}
							placeholder={text.attendanceLocations.placeholder}
							autocomplete="off"
							oninput={(event) => updateAttendanceLocation(index, 'name', event.currentTarget.value)}
						/>
					</Item.Content>
					<Item.Actions>
						<Button
							variant={location.isDefault ? 'secondary' : 'ghost'}
							size="sm"
							onclick={() => updateAttendanceLocation(index, 'isDefault', true)}
						>
							{text.attendanceLocations.default}
						</Button>
						{#if attendanceLocations.length > 1}
							<Button variant="ghost" size="icon-sm" aria-label={text.attendanceLocations.remove} onclick={() => removeAttendanceLocation(index)}>
								<Trash2Icon />
							</Button>
						{/if}
					</Item.Actions>
				</Item.Root>
			{/each}
		</Item.Group>
		{#if attendanceLocationsMessage}
			<Field.Description class="mt-4">{attendanceLocationsMessage}</Field.Description>
		{/if}
	</Card.Content>
	<Card.Footer class="justify-end">
		<Button disabled={!isDeviceReachable || isSavingAttendanceLocations || attendanceLocations.length === 0} onclick={saveAttendanceLocations}>
			{#if isSavingAttendanceLocations}
				<LoaderIcon class="size-4 animate-spin" />
			{/if}
			{text.attendanceLocations.save}
		</Button>
	</Card.Footer>
</Card.Root>
