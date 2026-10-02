<script lang="ts">
	import { z } from 'zod';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Select from '$lib/components/ui/select';
	import * as Field from '$lib/components/ui/field';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Spinner } from '$lib/components/ui/spinner';
	import { invokeTool } from '$lib/public-api-call';
	import { dataRoomGetResultSchema } from '$lib/data-room/schemas';
	import {
		dataRoomLinkLifetimeHours,
		dataRoomLinksSchema,
		dataRoomLinkCreatedSchema
	} from '$lib/data-room/links';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { dataRoomSharingText } from '$lib/data-room/sharing-text';
	import { dataRoomText } from '$lib/data-room/text';
	let { room }: { room: z.infer<typeof dataRoomGetResultSchema> } = $props();
	const text = createPageText(dataRoomSharingText);
	const browserText = createPageText(dataRoomText);
	const fieldID = $props.id();
	let isOpen = $state(false);
	let isBusy = $state(false);
	let label = $state('');
	let roleCode = $state('investor');
	let lifetimeHours = $state('72');
	let canDownload = $state(false);
	let shareableRoleCodes = $state<string[]>([]);
	let downloadableRoleCodes = $state<string[]>([]);
	let links = $state<z.infer<typeof dataRoomLinksSchema>['links']>([]);
	let createdLink = $state<z.infer<typeof dataRoomLinkCreatedSchema> | null>(null);
	let errorMessage = $state('');
	let isCopied = $state(false);
	const roleItems = $derived(
		room.roles
			.filter((role) => shareableRoleCodes.includes(role.code))
			.map((role) => ({
				value: role.code,
				label: currentLocale.value === 'ko' ? role.nameKO || role.name : role.name
			}))
	);
	const lifetimeItems = $derived(
		dataRoomLinkLifetimeHours.map((hours) => ({
			value: String(hours),
			label: hours < 24 ? `${hours} ${text.hours}` : `${hours / 24} ${text.days}`
		}))
	);
	const shareURL = $derived(
		createdLink && typeof window !== 'undefined'
			? `${window.location.origin}/share/links/${createdLink.linkID}`
			: ''
	);

	async function refresh() {
		const answer = dataRoomLinksSchema.parse(await invokeTool('dataroom_links_get', {}));
		links = answer.links;
		shareableRoleCodes = answer.shareableRoleCodes;
		downloadableRoleCodes = answer.downloadableRoleCodes;
	}

	async function open() {
		isOpen = true;
		errorMessage = '';
		isBusy = true;
		try {
			await refresh();
			if (!shareableRoleCodes.includes(roleCode)) roleCode = shareableRoleCodes[0] ?? '';
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		} finally {
			isBusy = false;
		}
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		isBusy = true;
		errorMessage = '';
		isCopied = false;
		createdLink = null;
		try {
			createdLink = dataRoomLinkCreatedSchema.parse(
				await invokeTool('dataroom_link_add', {
					label,
					roleCode,
					lifetimeHours: Number(lifetimeHours),
					canDownload: canDownload && downloadableRoleCodes.includes(roleCode)
				})
			);
			await refresh();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		} finally {
			isBusy = false;
		}
	}

	async function revoke(linkID: string) {
		isBusy = true;
		errorMessage = '';
		try {
			await invokeTool('dataroom_link_delete', { linkID });
			if (createdLink?.linkID === linkID) createdLink = null;
			await refresh();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		} finally {
			isBusy = false;
		}
	}

	async function copy() {
		try {
			await navigator.clipboard.writeText(shareURL);
			isCopied = true;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		}
	}
</script>

<Button size="sm" onclick={open}>{browserText.share}</Button>
<Dialog.Root
	bind:open={isOpen}
	onOpenChange={(open) => {
		if (!open) {
			createdLink = null;
			isCopied = false;
		}
	}}
>
	<Dialog.Content class="max-h-[85vh] overflow-auto sm:max-w-xl"
		><Dialog.Header
			><Dialog.Title>{text.title}</Dialog.Title><Dialog.Description
				>{text.description}</Dialog.Description
			></Dialog.Header
		>
		{#if roleItems.length}<form onsubmit={create} class="grid gap-4">
				<Field.Group>
					<Field.Field
						><Field.Label for="{fieldID}-label">{text.label}</Field.Label><Input
							id="{fieldID}-label"
							required
							maxlength={120}
							bind:value={label}
							disabled={isBusy}
						/></Field.Field
					>
					<Field.Field
						><Field.Label for="{fieldID}-role">{text.role}</Field.Label><Select.Root
							type="single"
							items={roleItems}
							bind:value={roleCode}
							disabled={isBusy}
							><Select.Trigger id="{fieldID}-role"
								>{roleItems.find((item) => item.value === roleCode)?.label}</Select.Trigger
							><Select.Content
								><Select.Group
									>{#each roleItems as item (item.value)}<Select.Item
											value={item.value}
											label={item.label}>{item.label}</Select.Item
										>{/each}</Select.Group
								></Select.Content
							></Select.Root
						><Field.Description
							>{room.roles
								.find((role) => role.code === roleCode)
								?.readableCategories.join(', ')}</Field.Description
						></Field.Field
					>
					<Field.Field
						><Field.Label for="{fieldID}-lifetime">{text.lifetime}</Field.Label><Select.Root
							type="single"
							items={lifetimeItems}
							bind:value={lifetimeHours}
							disabled={isBusy}
							><Select.Trigger id="{fieldID}-lifetime"
								>{lifetimeItems.find((item) => item.value === lifetimeHours)?.label}</Select.Trigger
							><Select.Content
								><Select.Group
									>{#each lifetimeItems as item (item.value)}<Select.Item
											value={item.value}
											label={item.label}>{item.label}</Select.Item
										>{/each}</Select.Group
								></Select.Content
							></Select.Root
						></Field.Field
					>
				</Field.Group>
				<div class="flex items-center gap-2">
					<Checkbox id="{fieldID}-download" checked={canDownload && downloadableRoleCodes.includes(roleCode)} onCheckedChange={(checked) => canDownload = checked === true} disabled={isBusy || !downloadableRoleCodes.includes(roleCode)} /><label
						for="{fieldID}-download"
						class="text-sm">{text.download}</label
					>
				</div>
				<p class="text-xs text-muted-foreground">{text.live}</p>
				<Button type="submit" disabled={isBusy}
					>{#if isBusy}<Spinner />{/if}{text.create}</Button
				>
			</form>{:else}<p class="text-sm text-muted-foreground">{text.noRoles}</p>{/if}
		{#if createdLink}<section class="grid gap-3 rounded-lg border p-4" aria-label={text.code}>
				<p class="text-sm text-muted-foreground">{text.codeOnce}</p>
				<div class="flex gap-2">
					<Input aria-label={text.copy} readonly value={shareURL} /><Button
						variant="outline"
						onclick={copy}>{isCopied ? text.copied : text.copy}</Button
					>
				</div>
				<p class="font-mono text-2xl tracking-[0.25em] select-all">{createdLink.accessCode}</p>
			</section>{/if}
		{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
		<section class="grid gap-3 border-t pt-4">
			<h2 class="text-sm font-semibold">{text.links}</h2>
			<p class="text-xs text-muted-foreground">{text.revokedDescription}</p>
			{#each links as link (link.id)}<div class="flex items-center justify-between gap-3 text-sm">
					<div class="min-w-0">
						<p class="truncate font-medium">{link.label}</p>
						<p class="text-xs text-muted-foreground">
							{link.roleCode} · {link.revokedAt
								? text.revoked
								: Date.parse(link.expiresAt) <= Date.now()
									? text.expired
									: `${text.expires}: ${new Date(link.expiresAt).toLocaleString(currentLocale.value)}`}
						</p>
					</div>
					{#if !link.revokedAt && Date.parse(link.expiresAt) > Date.now()}<Button
							variant="outline"
							size="sm"
							disabled={isBusy}
							onclick={() => revoke(link.id)}>{text.revoke}</Button
						>{/if}
				</div>{:else}<p class="text-sm text-muted-foreground">{text.noLinks}</p>{/each}
		</section>
	</Dialog.Content>
</Dialog.Root>
