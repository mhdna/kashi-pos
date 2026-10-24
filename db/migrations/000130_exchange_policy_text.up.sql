-- The short default exchange policy (000124) gets a second line saying to
-- present the receipt. A policy the shop wrote itself is left alone.
UPDATE settings SET return_policy = 'Exchange within 3 days if items are unworn, unwashed and with tags.' || char(10) || 'Please present this receipt.'
WHERE return_policy = 'Exchange within 3 days with tags and this receipt.';
