-- Not everyone on the roster sells: someone who only punches the attendance
-- device (a guard, a cleaner) stays a person here, but is_salesperson = 0
-- keeps them out of every checkout salesperson picker. Set from kashi's
-- branch roster; everyone already here sells, as before.
ALTER TABLE salespersons ADD COLUMN is_salesperson integer NOT NULL DEFAULT 1;
