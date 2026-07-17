import type { Event as DayFlowEvent } from '@dayflow/core';
import {
	monthEventSegments,
	type MonthEventSegment,
	type MonthEventWeek
} from './calendar-month-event-model';

export type MonthEventPlacement = MonthEventSegment & {
	left: number;
	top: number;
	width: number;
	height: number;
};

export type MonthMorePlacement = {
	id: string;
	weekID: string;
	dateKey: string;
	left: number;
	top: number;
	width: number;
	height: number;
	count: number;
	hiddenSegments: MonthEventSegment[];
};

export type MonthEventPlacementResult = {
	placements: MonthEventPlacement[];
	morePlacements: MonthMorePlacement[];
};

type MonthDateCellMeasure = {
	dateKey: string;
	left: number;
	right: number;
	top: number;
	bottom: number;
	width: number;
	height: number;
};

type MonthWeekMeasure = {
	week: MonthEventWeek;
	cellsByDateKey: Map<string, MonthDateCellMeasure>;
};

type MonthEventGeometry = {
	eventHeight: number;
	moreButtonHeight: number;
};

const compactMonthWidth = 768;
const compactMonthEventHeight = 24;
const desktopMonthEventHeight = 16;
const monthEventTopOffset = 34;
const monthEventLaneGap = 3;
const monthEventHorizontalInset = 4;
const monthMoreButtonBottomInset = 6;
const monthWeekGridSelector = '.df-month-week-grid';
const monthDateCellSelector = '.df-month-day-cell[data-date]';

export function measureMonthEventPlacements(stageElement: HTMLElement | null, events: DayFlowEvent[]): MonthEventPlacementResult {
	if (!stageElement) return { placements: [], morePlacements: [] };
	const stageRectangle = stageElement.getBoundingClientRect();
	const geometry = monthEventGeometry(stageRectangle.width);
	const weekMeasures = measuredMonthWeeks(stageElement, stageRectangle);
	const weeks = weekMeasures.map((weekMeasure) => weekMeasure.week);
	const weekMeasureByID = new Map(weekMeasures.map((weekMeasure) => [weekMeasure.week.id, weekMeasure]));
	const segments = monthEventSegments(events, weeks);
	const shouldReserveMoreByWeekID = new Map(
		weekMeasures.map((weekMeasure) => [
			weekMeasure.week.id,
			segments.some(
				(segment) =>
					segment.weekID === weekMeasure.week.id &&
					isSegmentOutsideVisibleLanes(segment, weekMeasure, false, geometry)
			)
		])
	);
	const placements: MonthEventPlacement[] = [];
	const hiddenSegmentsByMoreID = new Map<string, { cell: MonthDateCellMeasure; segments: MonthEventSegment[]; weekID: string }>();
	for (const segment of segments) {
		const weekMeasure = weekMeasureByID.get(segment.weekID);
		if (!weekMeasure) continue;
		const startCell = weekMeasure.cellsByDateKey.get(segment.startDateKey);
		const endCell = weekMeasure.cellsByDateKey.get(segment.endDateKey);
		if (!startCell || !endCell) continue;
		if (
			isSegmentOutsideVisibleLanes(
				segment,
				weekMeasure,
				shouldReserveMoreByWeekID.get(segment.weekID) ?? false,
				geometry
			)
		) {
			for (const cell of hiddenSegmentDateCells(segment, weekMeasure)) {
				const moreID = `${segment.weekID}::more::${cell.dateKey}`;
				const existingSummary = hiddenSegmentsByMoreID.get(moreID);
				if (existingSummary) {
					existingSummary.segments.push(segment);
				} else {
					hiddenSegmentsByMoreID.set(moreID, { cell, segments: [segment], weekID: segment.weekID });
				}
			}
			continue;
		}
		placements.push(placementFromSegment(segment, startCell, endCell, geometry));
	}
	return {
		placements,
		morePlacements: Array.from(hiddenSegmentsByMoreID.entries()).map(([id, summary]) =>
			morePlacementFromSummary(id, summary.weekID, summary.cell, summary.segments, geometry)
		)
	};
}

function monthEventGeometry(stageWidth: number): MonthEventGeometry {
	const eventHeight = stageWidth < compactMonthWidth ? compactMonthEventHeight : desktopMonthEventHeight;
	return { eventHeight, moreButtonHeight: eventHeight };
}

function hiddenSegmentDateCells(segment: MonthEventSegment, weekMeasure: MonthWeekMeasure): MonthDateCellMeasure[] {
	return weekMeasure.week.dateKeys
		.filter((dateKey) => dateKey >= segment.startDateKey && dateKey <= segment.endDateKey)
		.flatMap((dateKey) => {
			const cell = weekMeasure.cellsByDateKey.get(dateKey);
			return cell ? [cell] : [];
		});
}

function measuredMonthWeeks(stageElement: HTMLElement, stageRectangle: DOMRect): MonthWeekMeasure[] {
	return Array.from(stageElement.querySelectorAll<HTMLElement>(monthWeekGridSelector)).flatMap((weekElement) => {
		const cells = measuredMonthDateCells(weekElement, stageRectangle);
		const firstCell = cells[0];
		if (!firstCell) return [];
		return [
			{
				week: {
					id: firstCell.dateKey,
					dateKeys: cells.map((cell) => cell.dateKey)
				},
				cellsByDateKey: new Map(cells.map((cell) => [cell.dateKey, cell]))
			}
		];
	});
}

function measuredMonthDateCells(weekElement: HTMLElement, stageRectangle: DOMRect): MonthDateCellMeasure[] {
	return Array.from(weekElement.querySelectorAll<HTMLElement>(monthDateCellSelector))
		.flatMap((cellElement) => measuredMonthDateCell(cellElement, stageRectangle))
		.sort((firstCell, secondCell) => firstCell.left - secondCell.left);
}

function measuredMonthDateCell(cellElement: HTMLElement, stageRectangle: DOMRect): MonthDateCellMeasure[] {
	const dateKey = cellElement.dataset.date;
	if (!dateKey) return [];
	const rectangle = cellElement.getBoundingClientRect();
	if (rectangle.width <= 0 || rectangle.height <= 0) return [];
	return [
		{
			dateKey,
			left: rectangle.left - stageRectangle.left,
			right: rectangle.right - stageRectangle.left,
			top: rectangle.top - stageRectangle.top,
			bottom: rectangle.bottom - stageRectangle.top,
			width: rectangle.width,
			height: rectangle.height
		}
	];
}

function isSegmentOutsideVisibleLanes(
	segment: MonthEventSegment,
	weekMeasure: MonthWeekMeasure,
	shouldReserveMoreButton: boolean,
	geometry: MonthEventGeometry
): boolean {
	const spannedCells = weekMeasure.week.dateKeys
		.filter((dateKey) => dateKey >= segment.startDateKey && dateKey <= segment.endDateKey)
		.map((dateKey) => weekMeasure.cellsByDateKey.get(dateKey))
		.filter((cell): cell is MonthDateCellMeasure => Boolean(cell));
	if (spannedCells.length === 0) return false;
	const visibleLaneCount = Math.max(
		1,
		Math.min(...spannedCells.map((cell) => visibleLaneCountForCell(cell, shouldReserveMoreButton, geometry)))
	);
	return segment.lane >= visibleLaneCount;
}

function placementFromSegment(
	segment: MonthEventSegment,
	startCell: MonthDateCellMeasure,
	endCell: MonthDateCellMeasure,
	geometry: MonthEventGeometry
): MonthEventPlacement {
	const left = startCell.left + monthEventHorizontalInset;
	const width = Math.max(28, endCell.right - startCell.left - monthEventHorizontalInset * 2);
	const top = startCell.top + monthEventTopOffset + segment.lane * (geometry.eventHeight + monthEventLaneGap);
	return {
		...segment,
		left: Math.round(left),
		top: Math.round(top),
		width: Math.round(Math.min(width, endCell.right - left - monthEventHorizontalInset)),
		height: geometry.eventHeight
	};
}

function morePlacementFromSummary(
	id: string,
	weekID: string,
	cell: MonthDateCellMeasure,
	hiddenSegments: MonthEventSegment[],
	geometry: MonthEventGeometry
): MonthMorePlacement {
	const left = cell.left + monthEventHorizontalInset;
	return {
		id,
		weekID,
		dateKey: cell.dateKey,
		left: Math.round(left),
		top: Math.round(cell.bottom - monthMoreButtonBottomInset - geometry.moreButtonHeight),
		width: Math.round(Math.max(28, cell.width - monthEventHorizontalInset * 2)),
		height: geometry.moreButtonHeight,
		count: hiddenSegments.length,
		hiddenSegments
	};
}

function visibleLaneCountForCell(
	cell: MonthDateCellMeasure,
	shouldReserveMoreButton: boolean,
	geometry: MonthEventGeometry
): number {
	const reservedMoreHeight = shouldReserveMoreButton ? geometry.moreButtonHeight + monthEventLaneGap : 0;
	const availableHeight = cell.height - monthEventTopOffset - monthMoreButtonBottomInset - reservedMoreHeight;
	return Math.floor(availableHeight / (geometry.eventHeight + monthEventLaneGap));
}
