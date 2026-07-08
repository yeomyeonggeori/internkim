import type { UserRecord } from '../admin/admin-types';
import type { OrgchartTeamColumn } from './orgchart-directory-model';

export type OrgchartPositionedNode = {
	record: UserRecord;
	x: number;
	y: number;
	isAttachedToHeader: boolean;
};

export type OrgchartPositionedEdge = {
	fromX: number;
	fromY: number;
	toX: number;
	toY: number;
	middleY?: number;
};

export type OrgchartTeamLayout = {
	column: OrgchartTeamColumn;
	width: number;
	height: number;
	headerX: number;
	headerWidth: number;
	nodes: OrgchartPositionedNode[];
	edges: OrgchartPositionedEdge[];
};

export type OrgchartPositionedRoot = {
	record: UserRecord;
	x: number;
	centerX: number;
};

export type OrgchartRootBlockLayout = {
	width: number;
	columnLefts: number[];
	rootCentersByUserID: Map<string, number>;
};

export type OrgchartFitZoomInput = {
	boardWidth: number;
	boardHeight: number;
	viewportWidth: number;
	viewportHeight: number;
	minimumZoom?: number;
	maximumZoom?: number;
};

export type OrgchartRootTarget = {
	supervisorID: string;
	x: number;
	left: number;
	right: number;
};
