// bun:test 모듈 타입을 Svelte 타입체크에 제공한다.
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
