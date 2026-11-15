-- Who pays for a company's model calls is one rule: the company's own
-- OpenRouter key pays when the company's computer holds one and this setting
-- is on, and credits pay through the model-gateway edge function otherwise.
-- The two never mix. Holding a key and using it are separate: turning the
-- setting off keeps the key where it is.
--
-- A key typed into a computer at install never reaches the plane, so the
-- computer says whether it holds one each time its services start, through
-- the model-gateway's payer report. Until it has said, holding one is unknown.
--
-- A key entered from now on turns the setting on, because entering a key is
-- the intent to use it. Existing companies keep who pays them today: a
-- company with a paying subscription spent its credits first, so it starts
-- with the setting off and keeps spending them; every other company paid
-- with its key whenever it held one, so it starts with the setting on.

alter table public.company
  add column uses_own_model_key boolean not null default true,
  add column host_holds_model_key boolean;

comment on column public.company.uses_own_model_key is
  'whether the company''s own OpenRouter key pays for its model calls when its computer holds one; credits pay through the model gateway otherwise';
comment on column public.company.host_holds_model_key is
  'whether the computer serving the company said, when its services last started, that it holds an OpenRouter key; null until one has said';

update public.company
set uses_own_model_key = false
where subscription_status in ('active', 'trialing');
