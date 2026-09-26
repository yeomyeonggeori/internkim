begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

select is(
  (select count(*)::int from vault.secrets where name = 'scheduled_job_secret'),
  1,
  'the plane holds a scheduled job secret without anyone founding a company first'
);

select hasnt_function(
  'public',
  'digest_agent_key_keep',
  'no company is asked to lend the scheduled jobs an agent key'
);

select ok(
  not has_function_privilege('anon', 'public.is_a_scheduled_job(text)', 'execute')
    and not has_function_privilege('authenticated', 'public.is_a_scheduled_job(text)', 'execute'),
  'no signed-in caller can probe the secret'
);

set local role service_role;

select ok(
  public.is_a_scheduled_job((select decrypted_secret from vault.decrypted_secrets where name = 'scheduled_job_secret')),
  'the secret the jobs send is recognized'
);

select ok(
  not public.is_a_scheduled_job('not-the-secret'),
  'another value is refused'
);

select ok(
  not public.is_a_scheduled_job(''),
  'an empty value is refused'
);

select ok(
  not public.is_a_scheduled_job(null),
  'no value is refused'
);

reset role;

delete from net.http_request_queue;
delete from vault.secrets where name in ('day_digest_app_url', 'scheduled_job_secret');
select vault.create_secret('https://ours.example.test', 'day_digest_app_url');
select vault.create_secret('the-job-secret', 'scheduled_job_secret');

select public.ask_about_shifts_nobody_closed();

select is(
  (select url from net.http_request_queue order by id desc limit 1),
  'https://ours.example.test/api/agent/unclosed-shifts',
  'the unclosed shift is asked about at the app'
);

select is(
  (select headers->>'Authorization' from net.http_request_queue order by id desc limit 1),
  'Bearer the-job-secret',
  'and the ask carries the scheduled job secret'
);

select * from finish();
rollback;
