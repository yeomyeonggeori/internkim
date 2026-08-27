-- A key is used far from the person who made it, so what it may do belongs on
-- the key rather than on them. Read, write and delete are a ladder: delete
-- implies write implies read, because a caller that cannot read cannot know
-- what to delete.
--
-- Every key that exists was made when a key meant all three, so the default is
-- what they take. Nothing is migrated and nothing is counted: no key is
-- narrower today.
alter table public.credential
  add column permission text not null default 'delete',
  add constraint credential_permission_is_a_rung
    check (permission in ('read', 'write', 'delete'));

comment on column public.credential.permission is
  'how far up the read-write-delete ladder a key reaches; the kinds that are not keys carry the default and nothing reads it';
