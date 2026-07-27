declare module 'bun:test' {
	type TestCallback = () => void | Promise<void>;

	type Expectation<T> = {
		readonly not: Expectation<T>;
		readonly rejects: Expectation<Awaited<T>>;
		toBe(expected: unknown): void;
		toEqual(expected: unknown): void;
		toMatchObject(expected: unknown): void;
		toThrow(expected?: unknown): void;
		toContain(expected: unknown): void;
		toHaveLength(expected: number): void;
		toBeNull(): void;
		toBeInstanceOf(expected: unknown): void;
	};

	export function beforeEach(callback: () => void | Promise<void>): void;
	export function describe(name: string, callback: TestCallback): void;
	export function test(name: string, callback: TestCallback): void;

	export function mock<Arguments extends unknown[], ReturnValue>(
		implementation: (...parameters: Arguments) => ReturnValue
	): (...parameters: Arguments) => ReturnValue;

	export namespace mock {
		function clearAllMocks(): void;
		function module(specifier: string, factory: () => Record<string, unknown>): void;
	}
	export function expect<T>(actual: T): Expectation<T>;
}
