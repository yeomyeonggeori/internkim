<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import WifiIcon from '@lucide/svelte/icons/wifi';
	import {
		addWifiProfile,
		apiErrorMessage,
		fetchWifiProfiles,
		removeWifiProfile,
		updateWifiPassword
	} from './admin-api';
	import type { AdminPageText, WifiProfile } from './admin-types';

	type NetworkSectionProps = {
		adminBaseURL: string;
		isDeviceReachable: boolean;
		text: AdminPageText;
	};

	let { adminBaseURL, isDeviceReachable, text }: NetworkSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let profiles = $state<WifiProfile[]>([]);
	let message = $state('');
	let isLoading = $state(false);

	let newSSID = $state('');
	let newPassword = $state('');
	let isAdding = $state(false);

	let editingPasswords = $state<Record<string, string>>({});
	let savingProfile = $state<string | null>(null);

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadProfiles();
	});

	async function loadProfiles() {
		if (!adminBaseURL) return;
		isLoading = true;
		message = '';
		try {
			const response = await fetchWifiProfiles(adminBaseURL, text.wifiProfiles.loadError);
			profiles = response.profiles ?? [];
		} catch {
			message = text.wifiProfiles.loadError;
		} finally {
			isLoading = false;
		}
	}

	async function handleAdd() {
		if (!newSSID.trim()) return;
		isAdding = true;
		message = '';
		try {
			const response = await addWifiProfile(adminBaseURL, newSSID.trim(), newPassword, text.wifiProfiles.addError);
			profiles = response.profiles ?? [];
			newSSID = '';
			newPassword = '';
		} catch (error) {
			message = apiErrorMessage(error, text.wifiProfiles.addError);
		} finally {
			isAdding = false;
		}
	}

	async function handleUpdatePassword(profile: WifiProfile) {
		const password = editingPasswords[profile.name] ?? '';
		if (!password.trim()) return;
		savingProfile = profile.name;
		message = '';
		try {
			const response = await updateWifiPassword(adminBaseURL, profile.name, password.trim(), text.wifiProfiles.updateError);
			profiles = response.profiles ?? [];
			editingPasswords = { ...editingPasswords, [profile.name]: '' };
		} catch (error) {
			message = apiErrorMessage(error, text.wifiProfiles.updateError);
		} finally {
			savingProfile = null;
		}
	}

	async function handleRemove(profile: WifiProfile) {
		if (!confirm(text.wifiProfiles.removeConfirm)) return;
		savingProfile = profile.name;
		message = '';
		try {
			const response = await removeWifiProfile(adminBaseURL, profile.name, text.wifiProfiles.removeError);
			profiles = response.profiles ?? [];
		} catch (error) {
			message = apiErrorMessage(error, text.wifiProfiles.removeError);
		} finally {
			savingProfile = null;
		}
	}
</script>

<div class="rounded-lg border p-4">
	<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
		<div>
			<h3 class="flex items-center gap-2 text-sm font-semibold">
				<WifiIcon class="size-4 text-blue-500" />
				{text.wifiProfiles.title}
			</h3>
			<p class="mt-1 text-sm text-muted-foreground">{text.wifiProfiles.description}</p>
		</div>
	</div>

	{#if isLoading}
		<div class="flex items-center gap-2 text-sm text-muted-foreground">
			<LoaderIcon class="size-4 animate-spin" />
		</div>
	{:else if profiles.length === 0}
		<p class="text-sm text-muted-foreground">{text.wifiProfiles.empty}</p>
	{:else}
		<div class="grid gap-2">
			{#each profiles as profile (profile.name)}
				<div class="grid gap-2 rounded-md border p-3 md:grid-cols-[1fr_auto_1fr_auto_auto] md:items-center">
					<div class="min-w-0">
						<p class="truncate text-sm font-medium">{profile.ssid}</p>
						{#if profile.ssid !== profile.name}
							<p class="truncate text-xs text-muted-foreground">{profile.name}</p>
						{/if}
					</div>
					<div>
						{#if profile.isActive}
							<Badge variant="secondary" class="text-xs text-green-600">{text.wifiProfiles.active}</Badge>
						{/if}
					</div>
					<Input
						type="password"
						value={editingPasswords[profile.name] ?? ''}
						placeholder={text.wifiProfiles.passwordPlaceholder}
						autocomplete="off"
						disabled={savingProfile === profile.name}
						oninput={(event) => {
							editingPasswords = { ...editingPasswords, [profile.name]: event.currentTarget.value };
						}}
					/>
					<Button
						variant="outline"
						size="sm"
						disabled={!isDeviceReachable || savingProfile === profile.name || !(editingPasswords[profile.name] ?? '').trim()}
						onclick={() => handleUpdatePassword(profile)}
					>
						{#if savingProfile === profile.name}
							<LoaderIcon class="size-4 animate-spin" />
						{/if}
						{text.wifiProfiles.save}
					</Button>
					<Button
						variant="ghost"
						size="icon-sm"
						aria-label={text.wifiProfiles.remove}
						disabled={!isDeviceReachable || savingProfile === profile.name}
						onclick={() => handleRemove(profile)}
					>
						<Trash2Icon class="size-4" />
					</Button>
				</div>
			{/each}
		</div>
	{/if}

	{#if message}
		<p class="mt-3 rounded-md border bg-muted/30 px-3 py-2 text-sm">{message}</p>
	{/if}
</div>

<div class="rounded-lg border p-4">
	<h3 class="mb-3 flex items-center gap-2 text-sm font-semibold">
		<PlusIcon class="size-4" />
		{text.wifiProfiles.addTitle}
	</h3>
	<div class="grid gap-2 md:grid-cols-[1fr_1fr_auto] md:items-end">
		<div>
			<label class="text-xs font-medium text-muted-foreground" for="wifi-ssid">{text.wifiProfiles.ssidLabel}</label>
			<Input
				id="wifi-ssid"
				bind:value={newSSID}
				placeholder={text.wifiProfiles.ssidPlaceholder}
				autocomplete="off"
				disabled={isAdding}
				class="mt-1"
			/>
		</div>
		<div>
			<label class="text-xs font-medium text-muted-foreground" for="wifi-password">{text.wifiProfiles.passwordLabel}</label>
			<Input
				id="wifi-password"
				type="password"
				bind:value={newPassword}
				placeholder={text.wifiProfiles.passwordPlaceholder}
				autocomplete="off"
				disabled={isAdding}
				class="mt-1"
			/>
		</div>
		<Button disabled={!isDeviceReachable || isAdding || !newSSID.trim()} onclick={handleAdd}>
			{#if isAdding}
				<LoaderIcon class="size-4 animate-spin" />
			{/if}
			{text.wifiProfiles.add}
		</Button>
	</div>
</div>
