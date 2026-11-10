begin;

select plan(7);

select is('(1500.25,USD)'::public.currency_amount, row(1500.25, 'USD')::public.currency_amount,
  'an amount is in the currency''s own unit and names its code');
select is(('(,)'::public.currency_amount) is null, true,
  'an amount with neither part is no amount');
select throws_ok($$select '(1500,krw)'::public.currency_amount$$, '23514', null,
  'a currency is an ISO 4217 code');
select throws_ok($$select '(1500,)'::public.currency_amount$$, '23514', null,
  'an amount names its currency');
select throws_ok($$select '(,KRW)'::public.currency_amount$$, '23514', null,
  'a currency names an amount');
select is(public.is_money('{"amount": 1500, "currency": "krw"}'), false,
  'money written in a profile follows the same currency rule');
select is(public.is_money('{"amount": 1500, "currency": "KRW"}'), true,
  'money written in a profile is an amount and a currency');

select * from finish();
rollback;
