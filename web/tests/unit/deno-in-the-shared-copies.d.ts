declare module 'npm:@supabase/supabase-js@2' {
	export * from '@supabase/supabase-js';
}

declare const Deno: { env: { get(name: string): string | undefined } };
