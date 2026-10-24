UPDATE settings SET return_policy = 'Exchange within 3 days with tags and this receipt.'
WHERE return_policy = 'Exchange within 3 days if items are unworn, unwashed and with tags.' || char(10) || 'Please present this receipt.';
