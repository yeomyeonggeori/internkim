-- An HR attribute, so it lives with the member rather than on the account, where
-- the person could rewrite it themselves.
alter table public.member add column phone_number text;
