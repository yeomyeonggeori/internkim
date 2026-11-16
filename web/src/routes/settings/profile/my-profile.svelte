<script lang="ts">
	import { appNavigation } from '$lib/components/app-navigation.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { changeOwnPicture, ownPictureFormats, PictureFormatRefused } from '$lib/profile/change-own-picture';
	import { loadOwnProfile, saveOwnProfile, type OwnProfile } from '$lib/profile/own-profile';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { myProfileText } from './text';

	const text = createPageText(myProfileText);
	const shellText = createPageText(appShellText);
	const fieldID = $props.id();

	let profile = $state<OwnProfile | null>(null);
	let phoneNumber = $state('');
	let hireDate = $state('');
	let isSaving = $state(false);
	let isChangingPicture = $state(false);
	let chooser = $state<HTMLInputElement | null>(null);

	const position = $derived([profile?.jobTitle, profile?.teamName].filter(Boolean).join(' · '));
	const isChanged = $derived(
		profile !== null && (phoneNumber.trim() !== profile.phoneNumber || hireDate !== profile.hireDate)
	);

	onMount(async () => {
		try {
			show(await loadOwnProfile());
		} catch (failure) {
			toast.error(text.loadFailed, { description: reasonOf(failure) });
		}
	});

	function show(loaded: OwnProfile) {
		profile = loaded;
		phoneNumber = loaded.phoneNumber;
		hireDate = loaded.hireDate;
	}

	function cancel() {
		if (profile) show(profile);
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (!profile) return;
		isSaving = true;
		try {
			const saved = await saveOwnProfile(profile.memberID, { phoneNumber: phoneNumber.trim(), hireDate });
			show({ ...profile, ...saved });
			toast.success(text.saved);
		} catch (failure) {
			toast.error(text.saveFailed, { description: reasonOf(failure) });
		} finally {
			isSaving = false;
		}
	}

	async function choosePicture(event: Event & { currentTarget: HTMLInputElement }) {
		const chosen = event.currentTarget.files?.[0];
		event.currentTarget.value = '';
		if (!chosen) return;
		isChangingPicture = true;
		try {
			await changeOwnPicture(chosen);
			toast.success(text.pictureChanged);
		} catch (failure) {
			if (failure instanceof PictureFormatRefused) toast.error(text.pictureFormatRefused);
			else toast.error(text.pictureFailed, { description: reasonOf(failure) });
		} finally {
			isChangingPicture = false;
		}
	}

	function requestLogOut() {
		confirmDelete({
			title: shellText.logOutConfirmTitle,
			description: shellText.logOutConfirmDescription,
			confirm: { text: shellText.logOut },
			cancel: { text: shellText.cancel },
			onConfirm: appNavigation.logOut
		});
	}

	function reasonOf(failure: unknown): string {
		return failure instanceof Error ? failure.message : String(failure);
	}
</script>

<Card.Root>
	<Card.Content>
		<form onsubmit={save}>
			<Field.Group>
				<Item.Root class="p-0">
					<Item.Media>
						<PersonAvatar
							name={profile?.name ?? ''}
							email={profile?.email ?? ''}
							memberID={profile?.memberID ?? ''}
							class="size-16"
						/>
					</Item.Media>
					<Item.Content>
						<Item.Title>{displayPersonName(profile?.name ?? '')}</Item.Title>
						<Item.Description>{profile?.email ?? ''}</Item.Description>
						{#if position}
							<Item.Description>{position}</Item.Description>
						{/if}
					</Item.Content>
					<Item.Actions>
						<input
							bind:this={chooser}
							type="file"
							accept={ownPictureFormats.join(',')}
							class="hidden"
							aria-label={text.pictureChoose}
							onchange={choosePicture}
						/>
						<Button
							type="button"
							variant="outline"
							size="sm"
							disabled={!profile || isChangingPicture}
							onclick={() => chooser?.click()}
						>
							{text.pictureChoose}
						</Button>
					</Item.Actions>
				</Item.Root>
				<Field.Separator />
				<div class="grid gap-4 sm:grid-cols-2">
					<Field.Field>
						<Field.Label for="name-{fieldID}">{text.name}</Field.Label>
						<Input id="name-{fieldID}" value={displayPersonName(profile?.name ?? '')} disabled />
					</Field.Field>
					<Field.Field>
						<Field.Label for="phone-{fieldID}">{text.phoneNumber}</Field.Label>
						<Input
							id="phone-{fieldID}"
							type="tel"
							autocomplete="tel"
							bind:value={phoneNumber}
							disabled={!profile || isSaving}
						/>
					</Field.Field>
					<Field.Field class="sm:col-span-2">
						<Field.Label for="start-date-{fieldID}">{text.startDate}</Field.Label>
						<Input id="start-date-{fieldID}" type="date" bind:value={hireDate} disabled={!profile || isSaving} />
					</Field.Field>
				</div>
				<div class="flex justify-end gap-2">
					<Button type="submit" disabled={!isChanged || isSaving}>{text.save}</Button>
					<Button type="button" variant="outline" disabled={!isChanged || isSaving} onclick={cancel}>
						{text.cancel}
					</Button>
				</div>
			</Field.Group>
		</form>
	</Card.Content>
</Card.Root>

<div class="flex justify-end">
	<Button type="button" variant="destructive" onclick={requestLogOut}>{text.logOut}</Button>
</div>
