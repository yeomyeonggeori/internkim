<script lang="ts">
	import { z } from 'zod';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Field from '$lib/components/ui/field';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { invokeTool } from '$lib/public-api-call';
	import { dataRoomGetResultSchema } from '$lib/data-room/schemas';
	import { normalizeCategoryGrants, categoryName, type DataRoomRole } from '$lib/data-room/model';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { dataRoomText } from '$lib/data-room/text';
	let {
		room,
		onSaved
	}: { room: z.infer<typeof dataRoomGetResultSchema>; onSaved: () => Promise<void> } = $props();
	const text = createPageText(dataRoomText);
	const fieldID = $props.id();
	let isOpen = $state(false);
	let isSaving = $state(false);
	let code = $state('');
	let name = $state('');
	let nameKO = $state('');
	let readableCategories = $state<string[]>([]);
	let errorMessage = $state('');
	let isExisting = $state(false);
	const parents = $derived(room.categories.filter((category) => !category.parent));

	function edit(role?: DataRoomRole) {
		code = role?.code ?? '';
		name = role?.name ?? '';
		nameKO = role?.nameKO ?? '';
		readableCategories = role?.readableCategories ?? [];
		isExisting = !!role;
		errorMessage = '';
		isOpen = true;
	}

	function toggle(categoryCode: string, checked: boolean) {
		readableCategories =
			normalizeCategoryGrants(
				checked
					? [...readableCategories, categoryCode]
					: readableCategories.filter((value) => value !== categoryCode),
				room.categories
			) ?? [];
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		isSaving = true;
		errorMessage = '';
		try {
			await invokeTool('dataroom_role_update', {
				code,
				name,
				nameKO: nameKO || name,
				readableCategories
			});
			await onSaved();
			isOpen = false;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.loadFailed;
		} finally {
			isSaving = false;
		}
	}
</script>

<div class="flex flex-wrap items-center gap-2">
	<Button variant="outline" onclick={() => edit()}>{text.createRole}</Button
	>{#each room.roles as role (role.code)}<Button
			variant="ghost"
			size="sm"
			onclick={() => edit(role)}
			>{currentLocale.value === 'ko' ? role.nameKO || role.name : role.name}</Button
		>{/each}
</div>
<Dialog.Root bind:open={isOpen}
	><Dialog.Content class="max-h-[85vh] overflow-auto sm:max-w-2xl"
		><Dialog.Header
			><Dialog.Title>{text.readerRole}</Dialog.Title><Dialog.Description
				>{text.roleDescription}</Dialog.Description
			></Dialog.Header
		>
		<form onsubmit={save} class="grid gap-5">
			<Field.Group
				><Field.Field
					><Field.Label for="{fieldID}-code">{text.roleCode}</Field.Label><Input
						id="{fieldID}-code"
						required
						pattern="[a-z][a-z0-9-]*"
						bind:value={code}
						disabled={isExisting || isSaving}
					/></Field.Field
				><Field.Field
					><Field.Label for="{fieldID}-name">{text.roleName}</Field.Label><Input
						id="{fieldID}-name"
						required
						bind:value={name}
						disabled={isSaving}
					/></Field.Field
				><Field.Field
					><Field.Label for="{fieldID}-name-ko">{text.koreanRoleName}</Field.Label><Input
						id="{fieldID}-name-ko"
						bind:value={nameKO}
						disabled={isSaving}
					/></Field.Field
				></Field.Group
			>
			<div class="grid gap-4 sm:grid-cols-2">
				{#each parents as parent (parent.code)}<fieldset class="grid gap-2">
						<legend class="sr-only">{categoryName(parent, currentLocale.value)}</legend>
						<div class="flex items-center gap-2">
							<Checkbox
								id="{fieldID}-{parent.code}"
								checked={readableCategories.includes(parent.code)}
								onCheckedChange={(checked) => toggle(parent.code, checked === true)}
							/><label for="{fieldID}-{parent.code}" class="text-sm font-medium"
								>{parent.code} · {categoryName(parent, currentLocale.value)}</label
							>
						</div>
						{#each room.categories.filter((category) => category.parent === parent.code) as child (child.code)}<div
								class="ml-5 flex items-center gap-2"
							>
								<Checkbox
									id="{fieldID}-{child.code}"
									checked={readableCategories.includes(parent.code) ||
										readableCategories.includes(child.code)}
									disabled={readableCategories.includes(parent.code)}
									onCheckedChange={(checked) => toggle(child.code, checked === true)}
								/><label for="{fieldID}-{child.code}" class="text-sm"
									>{child.code} · {categoryName(child, currentLocale.value)}</label
								>
							</div>{/each}
					</fieldset>{/each}
			</div>
			{#if errorMessage}<p role="alert" class="text-sm text-destructive">
					{errorMessage}
				</p>{/if}<Dialog.Footer
				><Button type="submit" disabled={isSaving}>{text.save}</Button></Dialog.Footer
			>
		</form>
	</Dialog.Content></Dialog.Root
>
