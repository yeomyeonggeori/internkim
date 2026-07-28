<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Empty from '$lib/components/ui/empty';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
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

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.wifiProfiles.title}</Card.Title>
		<Card.Description>{text.wifiProfiles.description}</Card.Description>
	</Card.Header>
	<Card.Content>
		{#if isLoading}
			<div class="text-muted-foreground flex items-center gap-2 text-sm">
				<LoaderIcon class="size-4 animate-spin" />
			</div>
		{:else if profiles.length === 0}
			<Empty.Root class="border border-dashed">
				<Empty.Header>
					<Empty.Media variant="icon">
						<WifiIcon />
					</Empty.Media>
					<Empty.Title>{text.wifiProfiles.empty}</Empty.Title>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<Item.Group class="gap-2">
				{#each profiles as profile (profile.name)}
					<Item.Root variant="outline">
						<Item.Media variant="icon">
							<WifiIcon />
						</Item.Media>
						<Item.Content>
							<Item.Title>
								{profile.ssid}
								{#if profile.isActive}
									<Badge variant="secondary">{text.wifiProfiles.active}</Badge>
								{/if}
							</Item.Title>
							{#if profile.ssid !== profile.name}
								<Item.Description>{profile.name}</Item.Description>
							{/if}
						</Item.Content>
						<Item.Actions>
							<Input
								class="w-48"
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
								<Trash2Icon />
							</Button>
						</Item.Actions>
					</Item.Root>
				{/each}
			</Item.Group>
		{/if}
		{#if message}
			<Field.Error class="mt-4">{message}</Field.Error>
		{/if}
	</Card.Content>
	<Card.Footer class="flex-wrap items-end gap-3">
		<Field.Field class="min-w-48 flex-1">
			<Field.Label for="wifi-ssid">{text.wifiProfiles.ssidLabel}</Field.Label>
			<Input id="wifi-ssid" bind:value={newSSID} placeholder={text.wifiProfiles.ssidPlaceholder} autocomplete="off" disabled={isAdding} />
		</Field.Field>
		<Field.Field class="min-w-48 flex-1">
			<Field.Label for="wifi-password">{text.wifiProfiles.passwordLabel}</Field.Label>
			<Input
				id="wifi-password"
				type="password"
				bind:value={newPassword}
				placeholder={text.wifiProfiles.passwordPlaceholder}
				autocomplete="off"
				disabled={isAdding}
			/>
		</Field.Field>
		<Button disabled={!isDeviceReachable || isAdding || !newSSID.trim()} onclick={handleAdd}>
			{#if isAdding}
				<LoaderIcon class="size-4 animate-spin" />
			{:else}
				<PlusIcon />
			{/if}
			{text.wifiProfiles.add}
		</Button>
	</Card.Footer>
</Card.Root>
