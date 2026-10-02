<script lang="ts">
	import { onMount } from 'svelte';
	import { z } from 'zod';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Spinner } from '$lib/components/ui/spinner';
	import { invokeTool } from '$lib/public-api-call';
	import { supabaseOrganizationDirectory } from '$lib/organization/supabase-directory';
	import { dataRoomGetResultSchema } from '$lib/data-room/schemas';
	import { dataRoomText } from '$lib/data-room/text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import DataRoomRoles from './data-room-roles.svelte';

	type Employee = { id: string; name: string; email: string };
	const text = createPageText(dataRoomText);
	let room = $state<z.infer<typeof dataRoomGetResultSchema> | null>(null);
	let employees = $state<Employee[]>([]);
	let selectedEmployee = $state<Employee | null>(null);
	let selectedRoles = $state<string[]>([]);
	let isLoading = $state(true);
	let isSaving = $state(false);
	let errorMessage = $state('');
	const roles = $derived(room?.roles ?? []);
	const fieldID = $props.id();

	function roleName(code: string): string {
		const role = roles.find((candidate) => candidate.code === code);
		return currentLocale.value === 'ko' ? role?.nameKO || role?.name || code : role?.name || code;
	}

	function rolesOf(employeeID: string): string[] {
		return [
			...new Set(
				room?.shares
					.filter(
						(share) =>
							share.memberID === employeeID &&
							!share.revokedAt &&
							(!share.expiresAt || Date.parse(share.expiresAt) > Date.now())
					)
					.map((share) => share.roleCode) ?? []
			)
		];
	}

	async function load() {
		isLoading = true;
		errorMessage = '';
		try {
			const [answer, directory] = await Promise.all([
				invokeTool('dataroom_get', {}),
				supabaseOrganizationDirectory()
			]);
			room = dataRoomGetResultSchema.parse(answer);
			employees = (directory.records ?? []).flatMap((record) =>
				record.memberID
					? [{ id: record.memberID, name: record.name || record.email, email: record.email }]
					: []
			);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.loadFailed;
		} finally {
			isLoading = false;
		}
	}

	function edit(employee: Employee) {
		selectedEmployee = employee;
		selectedRoles = rolesOf(employee.id);
		errorMessage = '';
	}

	function toggle(code: string, checked: boolean) {
		selectedRoles = checked
			? [...selectedRoles, code]
			: selectedRoles.filter((role) => role !== code);
	}

	async function save() {
		if (!selectedEmployee) return;
		isSaving = true;
		try {
			await invokeTool('dataroom_member_update', {
				memberID: selectedEmployee.id,
				roleCodes: selectedRoles
			});
			room = dataRoomGetResultSchema.parse(await invokeTool('dataroom_get', {}));
			selectedEmployee = null;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.loadFailed;
		} finally {
			isSaving = false;
		}
	}
	onMount(load);
</script>

<section class="grid gap-4">
	<header class="grid gap-1">
		<h2 class="text-xl font-semibold">{text.permissions}</h2>
		<p class="text-sm text-muted-foreground">{text.permissionsDescription}</p>
	</header>
	<Card.Root
		><Card.Content>
			{#if isLoading}<div role="status" class="flex justify-center py-6"><Spinner /></div>
			{:else}<Table.Root
					><Table.Header
						><Table.Row
							><Table.Head>{text.name}</Table.Head><Table.Head>{text.roles}</Table.Head><Table.Head
								><span class="sr-only">{text.chooseRoles}</span></Table.Head
							></Table.Row
						></Table.Header
					>
					<Table.Body
						>{#each employees as employee (employee.id)}<Table.Row
								><Table.Cell
									><p class="font-medium">{employee.name}</p>
									<p class="text-xs text-muted-foreground">{employee.email}</p></Table.Cell
								><Table.Cell
									>{rolesOf(employee.id).map(roleName).join(', ') || text.noRoles}</Table.Cell
								><Table.Cell class="text-right"
									><Button variant="outline" size="sm" onclick={() => edit(employee)}
										>{text.chooseRoles}</Button
									></Table.Cell
								></Table.Row
							>{/each}</Table.Body
					></Table.Root
				>{/if}
		</Card.Content></Card.Root
	>
	{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
	{#if room?.canManage}<DataRoomRoles {room} onSaved={load} />{/if}
</section>

<Dialog.Root
	open={selectedEmployee !== null}
	onOpenChange={(open) => {
		if (!open && !isSaving) selectedEmployee = null;
	}}
>
	<Dialog.Content
		><Dialog.Header
			><Dialog.Title>{text.chooseRoles}</Dialog.Title><Dialog.Description
				>{selectedEmployee?.name} · {text.directRolesDescription}</Dialog.Description
			></Dialog.Header
		>
		<div class="grid gap-3">
			{#each roles as role (role.code)}<div class="flex items-start gap-3">
					<Checkbox
						id="{fieldID}-{role.code}"
						checked={selectedRoles.includes(role.code)}
						onCheckedChange={(checked) => toggle(role.code, checked === true)}
						disabled={isSaving}
					/><label for="{fieldID}-{role.code}" class="grid gap-1 text-sm"
						><span>{roleName(role.code)}</span><span class="text-xs text-muted-foreground"
							>{role.readableCategories.join(', ')}</span
						></label
					>
				</div>{/each}
		</div>
		{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
		<Dialog.Footer
			><Button disabled={isSaving} onclick={save}
				>{#if isSaving}<Spinner />{/if}{text.save}</Button
			></Dialog.Footer
		>
	</Dialog.Content>
</Dialog.Root>
