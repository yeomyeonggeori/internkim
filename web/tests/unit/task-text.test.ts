import { describe, expect, test } from 'bun:test';
import { taskText } from '../../src/routes/task/text';

type TextTree = { readonly [key: string]: TextNode };
type TextNode = string | readonly string[] | TextTree;

describe('taskText', () => {
	test('keeps Korean and English key shapes aligned', () => {
		expect(collectTextShape(taskText.en).sort()).toEqual(collectTextShape(taskText.ko).sort());
	});

	test('uses business copy for user-facing business labels', () => {
		expect(taskText.ko.report.businessDistance).toBe('이번 주간 사업 거리 분포');
		expect(taskText.ko.definitions.business).toBe('사업');
		expect(taskText.ko.filters.business).toBe('사업');
		expect(taskText.ko.task.business).toBe('사업');
		expect(taskText.ko.table.business).toBe('사업');

		expect(taskText.en.report.businessDistance).toBe('Weekly business distance');
		expect(taskText.en.definitions.business).toBe('Business');
		expect(taskText.en.filters.business).toBe('Business');
		expect(taskText.en.task.business).toBe('Business');
		expect(taskText.en.table.business).toBe('Business');
	});

	test('does not keep user-facing category text keys', () => {
		expect(collectTextShape(taskText.ko).some((path) => path.includes('category'))).toBe(false);
		expect(collectTextShape(taskText.en).some((path) => path.includes('category'))).toBe(false);
	});

	test('localizes task accessibility labels', () => {
		expect(taskText.ko.task.quickAddClose).toBe('AI 업무 추가 닫기');
		expect(taskText.en.task.quickAddClose).toBe('Close add with AI');
		expect(taskText.ko.task.quickAddBackdropClose).toBe('AI 업무 추가 팝업 배경 닫기');
		expect(taskText.en.task.quickAddBackdropClose).toBe('Close add with AI backdrop');
		expect(taskText.ko.task.removeParticipantAction.replace('{name}', '김철수')).toBe('김철수 제거');
		expect(taskText.en.task.removeParticipantAction.replace('{name}', 'Alice')).toBe('Remove Alice');
	});

	test('localizes personal score dialog labels', () => {
		expect(taskText.ko.report.personalScoreClose).toBe('점수 상세 닫기');
		expect(taskText.en.report.personalScoreClose).toBe('Close score details');
		expect(taskText.ko.report.weekPeriodSingularLabel).toBe('1주 전');
		expect(taskText.en.report.weekPeriodSingularLabel).toBe('1 week ago');
		expect(taskText.ko.report.monthPeriodSingularLabel).toBe('1개월 전');
		expect(taskText.en.report.monthPeriodSingularLabel).toBe('1 month ago');
	});
});

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
