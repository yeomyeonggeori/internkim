<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import GripVerticalIcon from '@lucide/svelte/icons/grip-vertical';
	import LockKeyholeIcon from '@lucide/svelte/icons/lock-keyhole';
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
		selectedOrganizationID,
		canManage,
		isEditing,
		isSaving,
		text,
		onSelect,
		onBeginEdit,
		onCancelEdit,
		onSaveEdit,
		onMove
	}: {
		tree: OrganizationOrganizationTree;
		selectedOrganizationID: string;
		canManage: boolean;
		isEditing: boolean;
		isSaving: boolean;
		text: typeof organizationDirectoryText.ko;
		onSelect: (organizationID: string) => void;
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
	<div class="flex h-14 shrink-0 items-center justify-between px-4">
		<span class="text-muted-foreground text-xs font-medium">{text.organizationNavigation}</span>
		{#if canManage}
			{#if isEditing}
				<div class="flex items-center gap-1">
					<Button type="button" variant="ghost" size="sm" disabled={isSaving} onclick={onCancelEdit}>{text.cancelOrganizationEdit}</Button>
					<Button type="button" size="sm" disabled={isSaving} onclick={() => void onSaveEdit()}>{text.saveOrganizations}</Button>
				</div>
			{:else}
				<Button type="button" variant="ghost" size="sm" onclick={onBeginEdit}>{text.editOrganizations}</Button>
			{/if}
		{/if}
	</div>

	<div class="min-h-0 flex-1 overflow-y-auto px-2 pb-4" bind:this={treeElement} data-testid="organization-tree">
		<button
			type="button"
			class={['flex h-9 w-full items-center gap-2 rounded-md px-2 text-left text-sm', selectedOrganizationID === '' ? 'bg-accent text-accent-foreground font-medium' : 'hover:bg-accent/50']}
			onclick={() => !isEditing && onSelect('')}
			data-testid="organization-root"
		>
			{#if isEditing}<LockKeyholeIcon class="text-muted-foreground size-4" />{:else}<ChevronDownIcon class="text-muted-foreground size-4" />{/if}
			<span class="min-w-0 flex-1 truncate">{tree.root.name}</span>
			{#if !isEditing}<OrganizationAvatarStack records={tree.root.aggregateRecords} memberCountUnit={text.memberCountUnit} />{/if}
		</button>

		{#each visibleNodes as node (node.id)}
			{#if movePreview && previewBeforeGroupID === node.id}
				<div class="relative h-8" data-testid="organization-drop-preview" style={`margin-left: ${movePreview.depth * 24 + 12}px`}>
					<div class="absolute left-0 right-0 top-4 h-0.5 rounded-full bg-primary"><span class="absolute -left-1 -top-[3px] size-2 rounded-full bg-primary"></span></div>
					<span class="absolute right-0 -top-1 rounded-md bg-primary px-2 py-1 text-[10px] font-semibold text-primary-foreground">{previewLabel()}</span>
				</div>
			{/if}
			<div
				class={['flex h-9 items-center gap-1 rounded-md px-2', selectedOrganizationID === node.id && !isEditing ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50', draggedSubtreeGroupIDs.has(node.id) && 'opacity-40']}
				style={`margin-left: ${node.depth * 16 + 8}px`}
				data-organization-row={node.id}
				data-testid={`organization-row-${node.id}`}
			>
				{#if isEditing}
					<span
						class="text-muted-foreground hover:bg-accent grid size-6 shrink-0 touch-none place-items-center rounded-md"
						aria-hidden="true"
						data-testid={`organization-drag-handle-${node.id}`}
						use:organizationTreeDrag={{ groupID: node.id, onStart: startDrag, onMove: updateDrag, onEnd: finishDrag, onCancel: clearDrag }}
					>
						<GripVerticalIcon class="size-4" />
					</span>
				{:else if hasChildren(node.id)}
					<button type="button" class="text-muted-foreground grid size-6 shrink-0 place-items-center rounded-md" aria-label={`${node.name} ${isExpanded(node.id) ? text.collapseOrganization : text.expandOrganization}`} onclick={() => toggleExpanded(node.id)}>
						{#if isExpanded(node.id)}<ChevronDownIcon class="size-4" />{:else}<ChevronRightIcon class="size-4" />{/if}
					</button>
				{:else}
					<span class="size-6 shrink-0"></span>
				{/if}
				<button type="button" class="min-w-0 flex-1 truncate text-left text-sm" onclick={() => !isEditing && onSelect(node.id)}>{node.name}</button>
				{#if !isEditing}<OrganizationAvatarStack records={node.aggregateRecords} memberCountUnit={text.memberCountUnit} />{/if}
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
