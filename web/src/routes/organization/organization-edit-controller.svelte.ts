import type { OrgGroup } from '$lib/organization/types';
import { moveOrganizationOrganization } from './organization-tree-model';

export class OrganizationOrganizationEditController {
	isEditing = $state(false);
	draftGroups = $state<OrgGroup[]>([]);

	begin(groups: OrgGroup[]): void {
		this.draftGroups = groups.map((group) => ({ ...group }));
		this.isEditing = true;
	}

	move(draggedGroupID: string, insertionIndex: number, requestedDepth: number): void {
		if (!this.isEditing) return;
		this.draftGroups = moveOrganizationOrganization(this.draftGroups, draggedGroupID, insertionIndex, requestedDepth);
	}

	cancel(): void {
		this.draftGroups = [];
		this.isEditing = false;
	}
}
