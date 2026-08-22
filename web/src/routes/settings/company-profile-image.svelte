<script lang="ts">
	import CompanyAvatar from '$lib/components/company-avatar.svelte';
	import * as Card from '$lib/components/ui/card';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import {
		companyPictureFormats,
		companyPictureMegabytes,
		forgetCompanyProfileImage,
		loadCompanyProfileImage,
		refusalOfCompanyPicture,
		saveCompanyProfileImage
	} from '$lib/company/profile-image';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';

	const text = createPageText(companySettingsText);
	const pressable =
		'rounded-md ring-offset-background transition-opacity hover:opacity-80 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none disabled:opacity-50';

	let name = $state('');
	let readableURL = $state('');
	let isBusy = $state(true);
	let chooser = $state<HTMLInputElement | null>(null);

	onMount(async () => {
		try {
			const picture = await loadCompanyProfileImage();
			name = picture.name;
			readableURL = picture.readableURL;
		} catch (failure) {
			toast.error(text.companyPictureLoadFailed, { description: (failure as Error).message });
		} finally {
			isBusy = false;
		}
	});

	async function choose(event: Event) {
		const chosen = (event.target as HTMLInputElement).files?.[0];
		if (chooser) chooser.value = '';
		if (!chosen) return;
		const tooBig = refusalOfCompanyPicture(chosen);
		if (tooBig) {
			toast.error(
				text.companyPictureTooBigTemplate
					.replace('{megabytes}', String(companyPictureMegabytes))
					.replace('{size}', tooBig)
			);
			return;
		}
		isBusy = true;
		try {
			readableURL = (await saveCompanyProfileImage(chosen)).readableURL;
			toast.success(text.companyPictureSaved);
		} catch (failure) {
			toast.error(text.companyPictureFailed, { description: (failure as Error).message });
		} finally {
			isBusy = false;
		}
	}

	async function forget() {
		isBusy = true;
		try {
			await forgetCompanyProfileImage();
			readableURL = '';
			toast.success(text.companyPictureRemoved);
		} catch (failure) {
			toast.error(text.companyPictureFailed, { description: (failure as Error).message });
		} finally {
			isBusy = false;
		}
	}
</script>

<Card.Root>
	<Card.Content class="flex items-center gap-4">
		<input
			bind:this={chooser}
			type="file"
			accept={companyPictureFormats.join(',')}
			class="hidden"
			onchange={choose}
		/>
		{#if readableURL}
			<DropdownMenu.Root>
				<DropdownMenu.Trigger
					disabled={isBusy}
					class={pressable}
					aria-label={text.companyPicture}
				>
					<CompanyAvatar {name} image={readableURL} class="size-14" />
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="start">
					<DropdownMenu.Item onSelect={() => chooser?.click()}>{text.companyPictureChoose}</DropdownMenu.Item>
					<DropdownMenu.Item variant="destructive" onSelect={forget}>{text.companyPictureRemove}</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		{:else}
			<button
				type="button"
				disabled={isBusy}
				onclick={() => chooser?.click()}
				class={pressable}
				aria-label={text.companyPicture}
			>
				<CompanyAvatar {name} class="size-14" />
			</button>
		{/if}
		<div class="grid gap-0.5">
			<p class="text-sm font-medium">{name}</p>
			<p class="text-xs text-muted-foreground">{text.companyPictureHintTemplate.replace('{megabytes}', String(companyPictureMegabytes))}</p>
		</div>
	</Card.Content>
</Card.Root>
