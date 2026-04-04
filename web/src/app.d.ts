declare global {
	namespace App {
		interface Platform {
			env: {
				KV: KVNamespace;
			};
		}
	}
}

export {};
