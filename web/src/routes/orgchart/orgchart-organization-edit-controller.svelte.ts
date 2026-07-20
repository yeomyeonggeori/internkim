import type { OrgGroup } from '$lib/orgchart/types';
import { moveOrgchartOrganization } from './orgchart-organization-tree-model';

export class OrgchartOrganizationEditController {
	isEditing = $state(false);
	draftGroups = $state<OrgGroup[]>([]);

	begin(groups: OrgGroup[]): void {
		this.draftGroups = groups.map((group) => ({ ...group }));
		this.isEditing = true;
	}

	move(draggedGroupID: string, insertionIndex: number, requestedDepth: number): void {
		if (!this.isEditing) return;
		this.draftGroups = moveOrgchartOrganization(this.draftGroups, draggedGroupID, insertionIndex, requestedDepth);
	}

	cancel(): void {
		this.draftGroups = [];
		this.isEditing = false;
	}
}
