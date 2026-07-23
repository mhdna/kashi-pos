create table if not exists currencies (
  code text not null primary key,
  name text not null unique,
  symbol text unique not null,
  is_default boolean not null default false,
  is_active boolean not null default true,
  value_in_default_currency bigint not null default 1,

  check (value_in_default_currency > 0),
  check (
    (is_default = true and value_in_default_currency = 1)
    or is_default = false
  )
);

create unique index one_default_currency on currencies (is_default) where is_default = true;

insert into currencies (code, name, symbol, is_default) values ('USC', 'US Cent', '¢', true);
insert into currencies (code, name, symbol, is_default, value_in_default_currency) values ('USD', 'US Dollar', '$', false, 100);
insert into currencies (code, name, symbol, is_default, value_in_default_currency) values ('LBP', 'Lebanese Pound', 'LL', false, 9000000);
