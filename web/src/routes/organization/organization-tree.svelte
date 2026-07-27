<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import Building2Icon from '@lucide/svelte/icons/building-2';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import GripVerticalIcon from '@lucide/svelte/icons/grip-vertical';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';
	import OrganizationAvatarStack from './organization-avatar-stack.svelte';
	import { organizationTreeDrag } from './organization-tree-drag-action';
	import {
		organizationOrganizationMovePreview,
		organizationOrganizationSubtreeIDs,
		organizationOrganizationTreeIndex,
		type OrganizationOrganizationMovePreview,
		type OrganizationOrganizationTree,
		type OrganizationOrganizationTreeNode
	} from './organization-tree-model';
	import type { organizationDirectoryText } from './text';

	let {
		tree,
		canManage,
		isEditing,
		isSaving,
		text,
		onSelect,
		onAddOrganization,
		onBeginEdit,
		onCancelEdit,
		onSaveEdit,
		onMove
	}: {
		tree: OrganizationOrganizationTree;
		canManage: boolean;
		isEditing: boolean;
		isSaving: boolean;
		text: typeof organizationDirectoryText.ko;
		onSelect: (organizationID: string) => void;
		onAddOrganization: () => void;
		onBeginEdit: () => void;
		onCancelEdit: () => void;
		onSaveEdit: () => void | Promise<void>;
		onMove: (groupID: string, insertionIndex: number, depth: number) => void;
	} = $props();

	let treeElement: HTMLDivElement;
	let expandedGroupIDs = $state<Record<string, boolean>>({});
	let draggedGroupID = $state('');
	let dragClientX = $state(0);
	let dragClientY = $state(0);
	let movePreview = $state<OrganizationOrganizationMovePreview>();
	const treeIndex = $derived(organizationOrganizationTreeIndex(tree.nodes));
	const draggedSubtreeGroupIDs = $derived(organizationOrganizationSubtreeIDs(tree.nodes, draggedGroupID));
	const remainingGroupIDs = $derived(tree.nodes.filter((node) => !draggedSubtreeGroupIDs.has(node.id)).map((node) => node.id));
	const visibleNodes = $derived(isEditing ? tree.nodes : tree.nodes.filter(isNodeVisible));
	const draggedNode = $derived(treeIndex.nodeByID.get(draggedGroupID));
	const previewBeforeGroupID = $derived(movePreview ? remainingGroupIDs[movePreview.insertionIndex] ?? '' : '');
	const isPreviewAtEnd = $derived(Boolean(movePreview && movePreview.insertionIndex === remainingGroupIDs.length));

	function isExpanded(groupID: string): boolean {
		return expandedGroupIDs[groupID] !== false;
	}

	function isNodeVisible(node: OrganizationOrganizationTreeNode): boolean {
		let parentID = node.parentID;
		while (parentID) {
			if (!isExpanded(parentID)) return false;
			parentID = treeIndex.nodeByID.get(parentID)?.parentID ?? '';
		}
		return true;
	}

	function hasChildren(groupID: string): boolean {
		return treeIndex.groupIDsWithChildren.has(groupID);
	}

	function childCount(groupID: string): number {
		return tree.nodes.filter((node) => node.parentID === groupID).length;
	}

	function selectNode(groupID: string): void {
		if (isEditing) return;
		onSelect(groupID);
		if (hasChildren(groupID)) toggleExpanded(groupID);
	}

	function toggleExpanded(groupID: string): void {
		expandedGroupIDs = { ...expandedGroupIDs, [groupID]: !isExpanded(groupID) };
	}

	function startDrag(groupID: string, clientX: number, clientY: number): void {
		draggedGroupID = groupID;
		updateDrag(clientX, clientY);
	}

	function updateDrag(clientX: number, clientY: number): void {
		if (!draggedGroupID || !treeElement) return;
		dragClientX = clientX;
		dragClientY = clientY;
		const remainingRows = Array.from(treeElement.querySelectorAll<HTMLElement>('[data-organization-row]'))
			.filter((element) => !draggedSubtreeGroupIDs.has(element.dataset.organizationRow ?? ''));
		const insertionIndex = remainingRows.filter((element) => clientY >= element.getBoundingClientRect().top + element.getBoundingClientRect().height / 2).length;
		const treeBox = treeElement.getBoundingClientRect();
		const requestedDepth = Math.max(0, Math.round((clientX - treeBox.left - 28) / 24));
		movePreview = organizationOrganizationMovePreview(tree.nodes, draggedGroupID, insertionIndex, requestedDepth);
	}

	function finishDrag(): void {
		if (draggedGroupID && movePreview) onMove(draggedGroupID, movePreview.insertionIndex, movePreview.depth);
		clearDrag();
	}

	function clearDrag(): void {
		draggedGroupID = '';
		movePreview = undefined;
	}

	function previewLabel(): string {
		if (!movePreview?.parentID) return text.dragTopLevel;
		const parentName = treeIndex.nodeByID.get(movePreview.parentID)?.name ?? text.allOrganizations;
		return `${parentName} ${text.dragUnderPrefix}`;
	}

</script>

<div class="flex h-full min-h-0 flex-col border-r bg-background" data-testid="organization-sidebar">
	{#if canManage && isEditing}
		<div class="flex h-14 shrink-0 items-center justify-end gap-1 px-4">
			<Button type="button" variant="ghost" size="sm" disabled={isSaving} onclick={onCancelEdit}>{text.cancelOrganizationEdit}</Button>
			<Button type="button" size="sm" disabled={isSaving} onclick={() => void onSaveEdit()}>{text.saveOrganizations}</Button>
		</div>
	{/if}

	<div class="min-h-0 flex-1 overflow-y-auto px-2 pt-3 pb-4" bind:this={treeElement} data-testid="organization-tree">
		<div class="hover:bg-accent/50 flex h-9 items-center gap-1 rounded-md pr-1 pl-2">
			<button
				type="button"
				class="flex min-w-0 flex-1 items-center gap-1 text-left text-sm"
				onclick={() => !isEditing && onSelect('')}
				data-testid="organization-root"
			>
				{#if !isEditing}
					<span class="text-muted-foreground grid size-6 shrink-0 place-items-center">
						<Building2Icon class="size-4" />
					</span>
				{/if}
				<span class="min-w-0 truncate">{tree.root.name}</span>
				{#if childCount('') > 0}
					<Badge variant="outline" class="h-5 min-w-5 rounded-full px-1 font-mono tabular-nums">{childCount('')}</Badge>
				{/if}
			</button>
			{#if canManage && !isEditing}
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button {...props} type="button" variant="ghost" size="icon-sm" aria-label={text.organizationActions}>
								<EllipsisVerticalIcon class="size-4" />
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end">
						<DropdownMenu.Item onSelect={onAddOrganization}>{text.addOrganization}</DropdownMenu.Item>
						<DropdownMenu.Item onSelect={onBeginEdit}>{text.editOrganizations}</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/if}
		</div>

		{#each visibleNodes as node (node.id)}
			{#if movePreview && previewBeforeGroupID === node.id}
				<div class="relative h-8" data-testid="organization-drop-preview" style={`margin-left: ${movePreview.depth * 24 + 12}px`}>
					<div class="absolute left-0 right-0 top-4 h-0.5 rounded-full bg-primary"><span class="absolute -left-1 -top-[3px] size-2 rounded-full bg-primary"></span></div>
					<span class="absolute right-0 -top-1 rounded-md bg-primary px-2 py-1 text-[10px] font-semibold text-primary-foreground">{previewLabel()}</span>
				</div>
			{/if}
			<div
				class={['relative flex h-9 items-center gap-1 rounded-md pr-2 hover:bg-accent/50', draggedSubtreeGroupIDs.has(node.id) && 'opacity-40']}
				style={`padding-left: ${(node.depth + 1) * 16 + 8}px`}
				data-organization-row={node.id}
				data-testid={`organization-row-${node.id}`}
			>
				{#each { length: node.depth + 1 } as _, level (level)}
					<span class="bg-border absolute inset-y-0 w-px" style={`left: ${level * 16 + 20}px`} aria-hidden="true"></span>
				{/each}
				{#if isEditing}
					<span
						class="text-muted-foreground hover:bg-accent grid size-6 shrink-0 touch-none place-items-center rounded-md"
						aria-hidden="true"
						data-testid={`organization-drag-handle-${node.id}`}
						use:organizationTreeDrag={{ groupID: node.id, onStart: startDrag, onMove: updateDrag, onEnd: finishDrag, onCancel: clearDrag }}
					>
						<GripVerticalIcon class="size-4" />
					</span>
				{/if}
				<button
					type="button"
					class="flex min-w-0 flex-1 items-center gap-1 text-left text-sm"
					aria-expanded={hasChildren(node.id) ? isExpanded(node.id) : undefined}
					aria-label={hasChildren(node.id)
						? `${node.name} ${isExpanded(node.id) ? text.collapseOrganization : text.expandOrganization}`
						: node.name}
					onclick={() => selectNode(node.id)}
				>
					{#if !isEditing}
						<span class="text-muted-foreground grid size-6 shrink-0 place-items-center">
							{#if !hasChildren(node.id)}
								<UsersRoundIcon class="size-4" />
							{:else if isExpanded(node.id)}
								<ChevronDownIcon class="size-4" />
							{:else}
								<ChevronRightIcon class="size-4" />
							{/if}
						</span>
					{/if}
					<span class="min-w-0 truncate">{node.name}</span>
					{#if hasChildren(node.id)}
						<Badge variant="outline" class="h-5 min-w-5 rounded-full px-1 font-mono tabular-nums">{childCount(node.id)}</Badge>
					{/if}
				</button>
				{#if !isEditing && !hasChildren(node.id)}
					<OrganizationAvatarStack records={node.aggregateRecords} memberCountUnit={text.memberCountUnit} />
				{/if}
			</div>
		{/each}

		{#if movePreview && isPreviewAtEnd}
			<div class="relative h-8" data-testid="organization-drop-preview" style={`margin-left: ${movePreview.depth * 24 + 12}px`}>
				<div class="absolute left-0 right-0 top-4 h-0.5 rounded-full bg-primary"><span class="absolute -left-1 -top-[3px] size-2 rounded-full bg-primary"></span></div>
				<span class="absolute right-0 -top-1 rounded-md bg-primary px-2 py-1 text-[10px] font-semibold text-primary-foreground">{previewLabel()}</span>
			</div>
		{/if}
	</div>
</div>

{#if draggedNode}
	<div
		class="pointer-events-none fixed z-[60] flex h-12 w-52 items-center gap-2 rounded-xl border border-primary/40 bg-background px-3 text-sm font-semibold shadow-xl"
		style={`left: ${dragClientX + 12}px; top: ${dragClientY + 12}px`}
		data-testid="organization-drag-card"
	>
		<GripVerticalIcon class="size-4 text-muted-foreground" />
		<span class="truncate">{draggedNode.name}</span>
	</div>
{/if}
