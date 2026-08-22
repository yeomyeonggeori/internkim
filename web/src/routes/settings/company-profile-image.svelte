<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Label } from '$lib/components/ui/label';
	import {
		forgetCompanyProfileImage,
		loadCompanyProfileImage,
		saveCompanyProfileImage
	} from '$lib/company/profile-image';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';

	const text = createPageText(companySettingsText);
	const fieldID = $props.id();

	let readableURL = $state('');
	let hasPicture = $state(false);
	let isLoading = $state(true);
	let isSaving = $state(false);
	let chooser = $state<HTMLInputElement | null>(null);

	onMount(async () => {
		try {
			const picture = await loadCompanyProfileImage();
			readableURL = picture.readableURL;
			hasPicture = picture.path !== '';
		} catch (failure) {
			toast.error(text.companyPictureLoadFailed, { description: (failure as Error).message });
		} finally {
			isLoading = false;
		}
	});

	async function choose(event: Event) {
		const chosen = (event.target as HTMLInputElement).files?.[0];
		if (!chosen) return;
		isSaving = true;
		try {
			const picture = await saveCompanyProfileImage(chosen);
			readableURL = picture.readableURL;
			hasPicture = true;
			toast.success(text.companyPictureSaved);
		} catch (failure) {
			toast.error(text.companyPictureFailed, { description: (failure as Error).message });
		} finally {
			isSaving = false;
			if (chooser) chooser.value = '';
		}
	}

	async function forget() {
		isSaving = true;
		try {
			await forgetCompanyProfileImage();
			readableURL = '';
			hasPicture = false;
			toast.success(text.companyPictureRemoved);
		} catch (failure) {
			toast.error(text.companyPictureFailed, { description: (failure as Error).message });
		} finally {
			isSaving = false;
		}
	}
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.companyPicture}</Card.Title>
		<Card.Description>{text.companyPictureDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-4">
		<div class="flex items-center gap-4">
			{#if readableURL}
				<img src={readableURL} alt={text.companyPicture} class="size-16 rounded-md object-cover" />
			{:else}
				<div class="grid size-16 place-items-center rounded-md bg-muted text-xs text-muted-foreground">
					{isLoading ? '' : text.companyPictureNone}
				</div>
			{/if}
			<div class="grid gap-2">
				<Label for={fieldID} class="sr-only">{text.companyPicture}</Label>
				<input
					bind:this={chooser}
					id={fieldID}
					type="file"
					accept="image/*"
					class="hidden"
					onchange={choose}
				/>
				<div class="flex gap-2">
					<Button variant="outline" disabled={isLoading || isSaving} onclick={() => chooser?.click()}>
						{text.companyPictureChoose}
					</Button>
					{#if hasPicture}
						<Button variant="ghost" disabled={isSaving} onclick={forget}>
							{text.companyPictureRemove}
						</Button>
					{/if}
				</div>
			</div>
		</div>
	</Card.Content>
</Card.Root>
