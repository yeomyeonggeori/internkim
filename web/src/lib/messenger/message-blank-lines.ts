const blankLinePlaceholder = ' ';
const codeFencePattern = /^ {0,3}(`{3,}|~{3,})/;

type CodeFence = { marker: string; length: number };

function fenceOpenedBy(line: string): CodeFence | undefined {
	const fence = codeFencePattern.exec(line)?.[1];
	return fence ? { marker: fence[0], length: fence.length } : undefined;
}

function closesFence(line: string, open: CodeFence): boolean {
	const fence = fenceOpenedBy(line);
	return fence !== undefined && fence.marker === open.marker && fence.length >= open.length;
}

function isBlank(line: string): boolean {
	return line.trim() === '';
}

function placeholderParagraphs(count: number): string[] {
	return Array.from({ length: count }, () => [blankLinePlaceholder, '']).flat();
}

export function markdownKeepingBlankLines(source: string): string {
	const lines = source.split('\n');
	const kept: string[] = [];
	let openFence: CodeFence | undefined;
	let blankRun = 0;

	for (const line of lines) {
		if (openFence) {
			kept.push(line);
			if (closesFence(line, openFence)) openFence = undefined;
			continue;
		}
		if (isBlank(line)) {
			blankRun += 1;
			continue;
		}
		if (blankRun > 0 && kept.length > 0) kept.push('', ...placeholderParagraphs(blankRun - 1));
		blankRun = 0;
		kept.push(line);
		openFence = fenceOpenedBy(line);
	}
	return kept.join('\n');
}
