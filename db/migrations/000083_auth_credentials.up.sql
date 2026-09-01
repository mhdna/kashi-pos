-- Login credentials for the touch login screen. Single row (id = 1).
-- Replaces the values that used to be hardcoded in VerifyPIN: the admin now
-- signs in with a password (initial value "1963", stored as sha256 hex - see
-- hashPassword) and cashiers keep the shared 4-digit PIN (initial "1478").
-- Both are editable from Settings and, for the admin password, the login
-- screen's admin sign-in flow.
CREATE TABLE IF NOT EXISTS auth_credentials (
    id integer primary key check (id = 1),
    admin_password_hash text not null,
    cashier_pin text not null
);

INSERT OR IGNORE INTO auth_credentials (id, admin_password_hash, cashier_pin)
VALUES (
    1,
    '59279341ea59fbf34025024596b670b6df2c9f80e71b9ad19aea71ba43b083fc',
    '1478'
);
