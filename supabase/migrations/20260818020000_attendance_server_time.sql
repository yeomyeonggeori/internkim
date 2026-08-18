create function public.attendance_server_time()
returns timestamptz
language sql
volatile
security invoker
set search_path = ''
as $$
	select statement_timestamp();
$$;

revoke execute on function public.attendance_server_time() from public, anon;
grant execute on function public.attendance_server_time() to authenticated;
