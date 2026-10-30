const dollarsPerCredit = 0.001;
const creditDigitsKept = 9;

export function creditsOfCost(costInDollars: number): number {
	if (!Number.isFinite(costInDollars) || costInDollars <= 0) return 0;
	return Math.ceil(Number((costInDollars / dollarsPerCredit).toFixed(creditDigitsKept)));
}

export function costOfAnswer(answer: unknown): number | null {
	if (typeof answer !== 'object' || answer === null) return null;
	const usage = (answer as { usage?: unknown }).usage;
	if (typeof usage !== 'object' || usage === null) return null;
	const cost = (usage as { cost?: unknown }).cost;
	return typeof cost === 'number' ? cost : null;
}

export type StreamCostReader = {
	read: (text: string) => void;
	cost: () => number | null;
};

export function streamCostReader(): StreamCostReader {
	let unfinishedLine = '';
	let lastCost: number | null = null;

	function readLine(line: string): void {
		if (!line.startsWith('data:')) return;
		const data = line.slice('data:'.length).trim();
		if (data === '' || data === '[DONE]') return;
		try {
			const cost = costOfAnswer(JSON.parse(data));
			if (cost !== null) lastCost = cost;
		} catch (thrown) {
			if (!(thrown instanceof SyntaxError)) throw thrown;
		}
	}

	return {
		read(text) {
			const lines = (unfinishedLine + text).split('\n');
			unfinishedLine = lines.pop() ?? '';
			lines.forEach((line) => readLine(line.trimEnd()));
		},
		cost() {
			readLine(unfinishedLine.trimEnd());
			unfinishedLine = '';
			return lastCost;
		}
	};
}
