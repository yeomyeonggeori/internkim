import type { OrganizationOrganizationSection } from './organization-model';

export type OrganizationSectionNode = {
	section: OrganizationOrganizationSection;
	children: OrganizationSectionNode[];
};

export function organizationSectionTree(sections: OrganizationOrganizationSection[]): OrganizationSectionNode[] {
	const roots: OrganizationSectionNode[] = [];
	const ancestors: OrganizationSectionNode[] = [];
	for (const section of sections) {
		const node: OrganizationSectionNode = { section, children: [] };
		while (ancestors.length > 0 && ancestors[ancestors.length - 1].section.depth >= section.depth) ancestors.pop();
		const parent = ancestors[ancestors.length - 1];
		if (parent) parent.children.push(node);
		else roots.push(node);
		ancestors.push(node);
	}
	return roots;
}

export function organizationSectionsWithRecords(nodes: OrganizationSectionNode[]): OrganizationSectionNode[] {
	return nodes
		.map((node) => ({ section: node.section, children: organizationSectionsWithRecords(node.children) }))
		.filter((node) => node.section.records.length > 0 || node.children.length > 0);
}

export function organizationSectionTrailingGap(node: OrganizationSectionNode, sectionPadding: number): number {
	const lastChild = node.children[node.children.length - 1];
	return sectionPadding + (lastChild ? organizationSectionTrailingGap(lastChild, sectionPadding) : 0);
}
