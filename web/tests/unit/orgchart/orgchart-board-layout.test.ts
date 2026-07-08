import { describe, expect, test } from 'bun:test';
import {
	orgchartBoardMinimumWidth,
	orgchartColumnCenterPositions,
	orgchartColumnLeftPositions,
	orgchartDistributedColumnSpace,
	orgchartFitZoom
} from '../../../src/routes/orgchart/orgchart-layout';

describe('orgchart board layout', () => {
	test('distributes connector positions across the available board width', () => {
		const columnWidths = [220, 456, 220];
		const boardWidth = 1200;
		const boardMinimumWidth = orgchartBoardMinimumWidth(columnWidths);
		const columnSpace = orgchartDistributedColumnSpace(columnWidths, boardWidth);
		const columnLefts = orgchartColumnLeftPositions(columnWidths, columnSpace, boardWidth);
		const connectorPositions = orgchartColumnCenterPositions(columnWidths, columnSpace, boardWidth);

		expect(boardMinimumWidth).toBe(1008);
		expect(columnSpace).toBe(152);
		expect(columnLefts).toEqual([0, 372, 980]);
		expect(connectorPositions).toEqual([110, 600, 1090]);
	});

	test('centers a single column connector in the board', () => {
		expect(orgchartColumnLeftPositions([220], 0, 1200)).toEqual([490]);
		expect(orgchartColumnCenterPositions([220], 0, 1200)).toEqual([600]);
	});

	test('fits org chart zoom to the available viewport down to the minimum zoom', () => {
		expect(orgchartFitZoom({ boardWidth: 900, boardHeight: 500, viewportWidth: 1200, viewportHeight: 800 })).toBe(100);
		expect(orgchartFitZoom({ boardWidth: 1200, boardHeight: 500, viewportWidth: 960, viewportHeight: 800 })).toBe(80);
		expect(orgchartFitZoom({ boardWidth: 900, boardHeight: 900, viewportWidth: 1200, viewportHeight: 720 })).toBe(80);
		expect(orgchartFitZoom({ boardWidth: 2400, boardHeight: 1200, viewportWidth: 960, viewportHeight: 600 })).toBe(70);
	});
});
