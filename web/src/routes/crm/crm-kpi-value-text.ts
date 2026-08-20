const donutValueSizes = [
	{ maxLength: 6, className: 'text-xl leading-6' },
	{ maxLength: 8, className: 'text-base leading-5' },
	{ maxLength: 10, className: 'text-sm leading-4' }
];
const smallestDonutValueSize = 'text-xs leading-4';

export function donutValueTextClass(displayValue: string): string {
	const size = donutValueSizes.find((candidate) => displayValue.length <= candidate.maxLength);
	return size ? size.className : smallestDonutValueSize;
}
