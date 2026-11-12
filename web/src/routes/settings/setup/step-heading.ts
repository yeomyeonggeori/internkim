export function stepHeading(title: string, step?: number): string {
	if (step === undefined) return title;
	return `${step}. ${title}`;
}
