begin;
create extension if not exists pgtap with schema extensions;
select plan(1);

select is(
  (select count(*) from public.company
    where id::text !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  0::bigint,
  'every seeded company id is an RFC 4122 UUID of version 1 to 8, which the claim route parses'
);

select * from finish();
rollback;
