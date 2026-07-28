<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
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

<div class="rounded-lg border p-4">
	<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
		<div>
			<h3 class="text-sm font-semibold">{text.settings.title}</h3>
			<p class="text-muted-foreground mt-1 text-sm">{text.settings.description}</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<Badge variant="outline">{workspaceSettings.timeZone || 'system'}</Badge>
			<Badge variant="outline">{workspaceLanguageOptions().find((option) => option.value === workspaceSettings.language)?.label}</Badge>
		</div>
	</div>
	<div class="grid gap-3 md:grid-cols-[1fr_160px_220px_auto] md:items-end">
		<div>
			<label class="text-xs font-medium text-muted-foreground" for="workspace-time-zone">{text.settings.timeZone}</label>
			<Input
				id="workspace-time-zone"
				bind:value={workspaceSettingsDraft.timeZone}
				placeholder={text.settings.timeZonePlaceholder}
				disabled={isLoadingWorkspaceSettings}
				autocomplete="off"
				class="mt-1"
			/>
			<p class="mt-2 text-xs text-muted-foreground">{text.settings.timeZoneHint}</p>
		</div>
		<div>
			<label class="text-xs font-medium text-muted-foreground" for="workspace-calling-code">{text.settings.callingCode}</label>
			<Input
				id="workspace-calling-code"
				bind:value={workspaceSettingsDraft.callingCode}
				placeholder={text.settings.callingCodePlaceholder}
				disabled={isLoadingWorkspaceSettings}
				autocomplete="off"
				inputmode="numeric"
				class="mt-1"
			/>
			<p class="mt-2 text-xs text-muted-foreground">{text.settings.callingCodeHint}</p>
		</div>
		<label class="grid gap-1.5">
			<span class="text-xs font-medium text-muted-foreground">{text.settings.workspaceLanguageTitle}</span>
			<Select.Root type="single" bind:value={workspaceSettingsDraft.language} disabled={isLoadingWorkspaceSettings || isSavingWorkspaceSettings}>
				<Select.Trigger class="w-full">
					{workspaceLanguageOptions().find((option) => option.value === workspaceSettingsDraft.language)?.label}
				</Select.Trigger>
				<Select.Content>
					{#each workspaceLanguageOptions() as option (option.value)}
						<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
			<span class="text-xs text-muted-foreground">{text.settings.workspaceLanguageDescription}</span>
		</label>
		<Button disabled={!isDeviceReachable || isLoadingWorkspaceSettings || isSavingWorkspaceSettings} onclick={saveWorkspaceSettings}>
			{#if isSavingWorkspaceSettings}
				<LoaderIcon class="size-4 animate-spin" />
			{/if}
			{text.settings.save}
		</Button>
	</div>
	{#if workspaceSettingsMessage}
		<p class="mt-3 rounded-md border bg-muted/30 px-3 py-2 text-sm">{workspaceSettingsMessage}</p>
	{/if}
</div>

<div class="rounded-lg border p-4">
	<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
		<div>
			<h3 class="flex items-center gap-2 text-sm font-semibold">
				<MapPinIcon class="size-4 text-emerald-600" />
				{text.attendanceLocations.title}
			</h3>
			<p class="mt-1 text-sm text-muted-foreground">{text.attendanceLocations.description}</p>
		</div>
		<Button variant="outline" size="sm" class="gap-2" onclick={addAttendanceLocation} disabled={isLoadingAttendanceLocations}>
			<PlusIcon class="size-4" />
			{text.attendanceLocations.add}
		</Button>
	</div>
	<div class="grid gap-2">
		{#each attendanceLocations as location, index (index)}
			<div class="grid gap-2 rounded-md border p-3 md:grid-cols-[auto_1fr_9rem_auto_auto] md:items-center">
				<input
					type="color"
					value={location.color}
					aria-label={text.attendanceLocations.color}
					class="size-9 rounded-md border bg-background"
					oninput={(event) => updateAttendanceLocation(index, 'color', event.currentTarget.value)}
				/>
				<Input
					value={location.name}
					placeholder={text.attendanceLocations.placeholder}
					autocomplete="off"
					oninput={(event) => updateAttendanceLocation(index, 'name', event.currentTarget.value)}
				/>
				<Button
					variant={location.isDefault ? 'secondary' : 'ghost'}
					size="sm"
					onclick={() => updateAttendanceLocation(index, 'isDefault', true)}
				>
					{text.attendanceLocations.default}
				</Button>
				{#if attendanceLocations.length > 1}
					<Button variant="ghost" size="icon-sm" aria-label={text.attendanceLocations.remove} onclick={() => removeAttendanceLocation(index)}>
						<Trash2Icon class="size-4" />
					</Button>
				{/if}
			</div>
		{/each}
	</div>
	<div class="mt-4 flex flex-wrap items-center gap-2">
		<Button disabled={!isDeviceReachable || isSavingAttendanceLocations || attendanceLocations.length === 0} onclick={saveAttendanceLocations}>
			{#if isSavingAttendanceLocations}
				<LoaderIcon class="size-4 animate-spin" />
			{/if}
			{text.attendanceLocations.save}
		</Button>
		{#if attendanceLocationsMessage}
			<p class="rounded-md border bg-muted/30 px-3 py-2 text-sm">{attendanceLocationsMessage}</p>
		{/if}
	</div>
</div>
