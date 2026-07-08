<script lang="ts">
	import type { UserRecord } from '../admin/admin-types';
	import {
		orgchartEdgePath,
		orgchartPersonNodeWidth,
		type OrgchartTeamLayout
	} from './orgchart-layout';
	import OrgchartPersonNode from './orgchart-person-node.svelte';
	import type { orgchartDirectoryText } from './text';

	type OrgchartTeamColumnProps = {
		layout: OrgchartTeamLayout;
		left: number;
		selectedUserID: string;
		text: typeof orgchartDirectoryText.ko;
		selectRecord: (record: UserRecord, anchor: DOMRect) => void;
	};

	let { layout, left, selectedUserID, text, selectRecord }: OrgchartTeamColumnProps = $props();

	function teamTone(): string {
		return 'border-slate-200 bg-slate-50 text-slate-800 dark:border-slate-800 dark:bg-slate-950/30 dark:text-slate-200';
	}
</script>

<div class="absolute top-0 min-w-0" style={`left: ${left}px; width: ${layout.width}px; height: ${layout.height}px`} data-testid={`orgchart-team-column-${layout.column.id}`}>
	<svg class="absolute inset-0 size-full text-border" viewBox={`0 0 ${layout.width} ${layout.height}`} preserveAspectRatio="none" aria-hidden="true">
		{#each layout.edges as edge}
			<path d={orgchartEdgePath(edge)} fill="none" stroke="currentColor" stroke-width="1" vector-effect="non-scaling-stroke" />
		{/each}
	</svg>
	<header class={['absolute grid h-14 content-center rounded-t-md border px-3 text-center', layout.column.treeRoots.length === 1 && 'border-b-0', teamTone()]} style={`left: ${layout.headerX}px; top: 0; width: ${layout.headerWidth}px`}>
		<h2 class="truncate text-base font-semibold">{layout.column.name}</h2>
	</header>
	{#each layout.nodes as node (node.record.userID)}
		<div class="absolute" style={`left: ${node.x}px; top: ${node.y}px; width: ${orgchartPersonNodeWidth}px`} data-testid={`orgchart-tree-node-${node.record.userID}`}>
			<OrgchartPersonNode
				record={node.record}
				isAttachedToHeader={node.isAttachedToHeader}
				isSelected={selectedUserID === node.record.userID}
				{text}
				{selectRecord}
			/>
		</div>
	{/each}
</div>
