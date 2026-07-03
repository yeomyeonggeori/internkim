<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button';
	import XIcon from '@lucide/svelte/icons/x';
	import type { OrgGroup, UserRecord } from '../admin/admin-types';
	import type { orgchartDirectoryText } from './text';

	type PopupAnchor = {
		top: number;
		right: number;
		bottom: number;
		left: number;
		width: number;
		height: number;
	};

	type OrgchartPersonDetailPanelProps = {
		record: UserRecord | undefined;
		anchor: PopupAnchor | undefined;
		groups: OrgGroup[];
		text: typeof orgchartDirectoryText.ko;
		clearSelection: () => void;
	};

	let { record, anchor, groups, text, clearSelection }: OrgchartPersonDetailPanelProps = $props();

	let popupElement = $state<HTMLElement>();
	let innerWidth = $state(0);
	let innerHeight = $state(0);

	const popupStyle = $derived(createPopupStyle(anchor, innerWidth, innerHeight));

	function groupName(groupID: string | undefined): string {
		const normalizedGroupID = groupID?.trim() ?? '';
		if (!normalizedGroupID) return text.unassignedTeam;
		return groups.find((group) => group.id === normalizedGroupID)?.name ?? normalizedGroupID;
	}

	function personLabel(person: UserRecord): string {
		return person.name || person.email;
	}

	function createPopupStyle(nextAnchor: PopupAnchor | undefined, viewportWidth: number, viewportHeight: number): string {
		if (!nextAnchor) return '';
		if (viewportWidth < 768) return 'left: 12px; right: 12px; bottom: 12px; max-height: min(560px, calc(100vh - 24px));';

		const gap = 12;
		const margin = 16;
		const width = 320;
		const estimatedHeight = 520;
		const rightSideLeft = nextAnchor.right + gap;
		const leftSideLeft = nextAnchor.left - width - gap;
		const hasRightSpace = rightSideLeft + width + margin <= viewportWidth;
		const left = hasRightSpace ? rightSideLeft : Math.max(margin, leftSideLeft);
		const top = Math.min(Math.max(margin, nextAnchor.top), Math.max(margin, viewportHeight - estimatedHeight));

		return `left: ${left}px; top: ${top}px; width: ${width}px; max-height: calc(100vh - ${top + margin}px);`;
	}

	function handleKeydown(event: KeyboardEvent): void {
		if (event.key === 'Escape') clearSelection();
	}

	function handleWindowClick(event: MouseEvent): void {
		const target = event.target;
		if (!(target instanceof Node)) return;
		if (popupElement?.contains(target)) return;
		clearSelection();
	}
</script>

<svelte:window bind:innerWidth bind:innerHeight onkeydown={handleKeydown} onclick={handleWindowClick} />

{#if record && anchor}
	<aside
		bind:this={popupElement}
		class="fixed z-50 grid max-w-[calc(100vw-1.5rem)] overflow-hidden rounded-lg border bg-card shadow-xl"
		style={popupStyle}
		data-testid="orgchart-person-detail-panel"
	>
		<div class="grid max-h-[inherit] gap-5 overflow-auto p-5">
			<div class="flex items-center justify-between gap-3">
				<h2 class="text-base font-semibold">{text.personDetail}</h2>
				<Button variant="ghost" size="icon" aria-label={text.closeDetail} onclick={clearSelection}>
					<XIcon class="size-4" />
				</Button>
			</div>

			<div class="flex min-w-0 items-center gap-4">
				<PersonAvatar name={record.name} email={record.email} seed={record.userID} class="size-14" />
				<div class="min-w-0">
					<h3 class="truncate text-xl font-semibold">{personLabel(record)}</h3>
					<p class="truncate text-sm text-muted-foreground">{record.jobTitle || text.noTitle}</p>
				</div>
			</div>

			<section class="grid gap-4 border-t pt-5">
				<h3 class="text-sm font-semibold">{text.basicInformation}</h3>
				<dl class="grid gap-0 text-sm">
					<div class="grid grid-cols-[86px_minmax(0,1fr)] border-b py-3">
						<dt class="text-muted-foreground">{text.email}</dt>
						<dd class="truncate">{record.email}</dd>
					</div>
					<div class="grid grid-cols-[86px_minmax(0,1fr)] border-b py-3">
						<dt class="text-muted-foreground">{text.primaryOrganization}</dt>
						<dd>{groupName(record.primaryGroupID ?? record.group)}</dd>
					</div>
					<div class="grid grid-cols-[86px_minmax(0,1fr)] border-b py-3">
						<dt class="text-muted-foreground">{text.position}</dt>
						<dd>{record.jobTitle || text.noTitle}</dd>
					</div>
					<div class="grid grid-cols-[86px_minmax(0,1fr)] py-3">
						<dt class="text-muted-foreground">{text.hireDate}</dt>
						<dd>{record.hireDate || text.notProvided}</dd>
					</div>
					</dl>
				</section>
		</div>
	</aside>
{/if}
