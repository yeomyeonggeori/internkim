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
	import { circleListResultSchema, dataRoomGetResultSchema } from '$lib/data-room/schemas';
	import { dataRoomText } from '$lib/data-room/text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import CircleEditor from './circle-editor.svelte';

	type Employee = { id: string; name: string; email: string };
	const text = createPageText(dataRoomText);
	let room = $state<z.infer<typeof dataRoomGetResultSchema> | null>(null);
	let employees = $state<Employee[]>([]);
	let selectedEmployee = $state<Employee | null>(null);
	let circles = $state<z.infer<typeof circleListResultSchema>['circles']>([]);
	let selectedCircles = $state<string[]>([]);
	let isLoading = $state(true);
	let isSaving = $state(false);
	let errorMessage = $state('');
	const fieldID = $props.id();

	function circleName(circle: (typeof circles)[number]): string {
		return currentLocale.value === 'ko' ? circle.nameKO || circle.name : circle.name;
	}

	function circlesOf(employeeID: string): typeof circles {
		return circles.filter((circle) => circle.memberIDs.includes(employeeID));
	}

	async function loadCircles() {
		circles = circleListResultSchema.parse(await invokeTool('circle_list', {})).circles;
	}

	async function load() {
		isLoading = true;
		errorMessage = '';
		try {
			const [answer, directory] = await Promise.all([
				invokeTool('dataroom_get', {}),
				supabaseOrganizationDirectory(),
				loadCircles()
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
		selectedCircles = circlesOf(employee.id).map((circle) => circle.id);
		errorMessage = '';
	}

	function toggle(circleID: string, checked: boolean) {
		selectedCircles = checked
			? [...selectedCircles, circleID]
			: selectedCircles.filter((selected) => selected !== circleID);
	}

	async function save() {
		if (!selectedEmployee) return;
		isSaving = true;
		try {
			await invokeTool('circle_member_update', {
				memberID: selectedEmployee.id,
				circleIDs: selectedCircles
			});
			await loadCircles();
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
		<h2 class="text-xl font-semibold">{text.circles}</h2>
		<p class="text-sm text-muted-foreground">{text.circlesDescription}</p>
	</header>
	<Card.Root
		><Card.Content>
			{#if isLoading}<div role="status" class="flex justify-center py-6"><Spinner /></div>
			{:else}<Table.Root
					><Table.Header
						><Table.Row
							><Table.Head>{text.name}</Table.Head><Table.Head>{text.circles}</Table.Head><Table.Head
								><span class="sr-only">{text.chooseCircles}</span></Table.Head
							></Table.Row
						></Table.Header
					>
					<Table.Body
						>{#each employees as employee (employee.id)}<Table.Row
								><Table.Cell
									><p class="font-medium">{employee.name}</p>
									<p class="text-xs text-muted-foreground">{employee.email}</p></Table.Cell
								><Table.Cell
									>{circlesOf(employee.id).map(circleName).join(', ') || text.noCircles}</Table.Cell
								><Table.Cell class="text-right"
									><Button variant="outline" size="sm" onclick={() => edit(employee)}
										>{text.chooseCircles}</Button
									></Table.Cell
								></Table.Row
							>{/each}</Table.Body
					></Table.Root
				>{/if}
		</Card.Content></Card.Root
	>
	{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
	{#if room?.canManage}<CircleEditor {circles} categories={room.categories} onSaved={loadCircles} />{/if}
</section>

<Dialog.Root
	open={selectedEmployee !== null}
	onOpenChange={(open) => {
		if (!open && !isSaving) selectedEmployee = null;
	}}
>
	<Dialog.Content
		><Dialog.Header
			><Dialog.Title>{text.chooseCircles}</Dialog.Title><Dialog.Description
				>{selectedEmployee?.name}</Dialog.Description
			></Dialog.Header
		>
		<div class="grid gap-3">
			{#each circles as circle (circle.id)}<div class="flex items-start gap-3">
					<Checkbox
						id="{fieldID}-{circle.id}"
						checked={selectedCircles.includes(circle.id)}
						onCheckedChange={(checked) => toggle(circle.id, checked === true)}
						disabled={isSaving}
					/><label for="{fieldID}-{circle.id}" class="grid gap-1 text-sm"
						><span>{circleName(circle)}</span><span class="text-xs text-muted-foreground"
							>{circle.readableCategories.join(', ')}</span
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
