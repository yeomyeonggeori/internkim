import type { OrgchartCanvasModel } from './orgchart-directory-model';
import type {
	OrgchartPositionedEdge,
	OrgchartPositionedRoot,
	OrgchartRootBlockLayout,
	OrgchartTeamLayout
} from './orgchart-layout-types';
import { orgchartRootConnectorEdges } from './orgchart-connector-layout';
import { orgchartRootBlockLayout } from './orgchart-root-block-layout';
import { orgchartRootLayouts } from './orgchart-root-center-layout';
import { orgchartOrderedTeamLayouts, orgchartTeamLayout } from './orgchart-team-layout';

export type OrgchartCanvasLayoutModel = {
	teamLayouts: OrgchartTeamLayout[];
	rootBlockLayout: OrgchartRootBlockLayout;
	boardWidth: number;
	columnLefts: number[];
	rootLayouts: OrgchartPositionedRoot[];
	rootConnectorEdges: OrgchartPositionedEdge[];
	teamGridHeight: number;
};

export function orgchartCanvasLayoutModel(model: OrgchartCanvasModel): OrgchartCanvasLayoutModel {
	const teamLayouts = orgchartOrderedTeamLayouts(model.roots, model.columns.map((column) => orgchartTeamLayout(column, model.roots)));
	const rootBlockLayout = orgchartRootBlockLayout(model.roots, teamLayouts);
	const boardWidth = rootBlockLayout.width;
	const columnLefts = rootBlockLayout.columnLefts;
	const rootLayouts = orgchartRootLayouts(model.roots, teamLayouts, columnLefts, boardWidth, rootBlockLayout.rootCentersByUserID);
	const rootConnectorEdges = orgchartRootConnectorEdges(model.roots, teamLayouts, columnLefts, boardWidth, rootBlockLayout.rootCentersByUserID);
	const teamGridHeight = Math.max(0, ...teamLayouts.map((layout) => layout.height));
	return {
		teamLayouts,
		rootBlockLayout,
		boardWidth,
		columnLefts,
		rootLayouts,
		rootConnectorEdges,
		teamGridHeight
	};
}
