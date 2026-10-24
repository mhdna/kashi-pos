-- The default exchange policy was a long paragraph that took a third of the
-- receipt. Tills still on the old default move to the short one; a policy the
-- shop wrote itself is left alone.
UPDATE settings SET return_policy = 'Exchange within 3 days with tags and this receipt.'
WHERE return_policy = 'Exchanges accepted within 3 days. Items must be unworn, unused, unwashed, undamaged, with tags. Present this receipt.';
