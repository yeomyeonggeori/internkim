declare module 'bun:test' {
	type TestCallback = () => void | Promise<void>;

	type Expectation = {
		readonly not: Expectation;
		toBe(expected: unknown): void;
		toEqual(expected: unknown): void;
		toThrow(): void;
	};

	export function describe(name: string, callback: TestCallback): void;
	export function test(name: string, callback: TestCallback): void;
	export function expect(actual: unknown): Expectation;
}
