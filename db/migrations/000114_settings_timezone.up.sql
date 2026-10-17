-- The branch's IANA timezone as kashi last reported it (/branch/health). The
-- till runs its clock in this zone instead of the PC's own, so its days and
-- times match kashi's. NULL until kashi has answered once: the PC's zone.
ALTER TABLE settings ADD COLUMN timezone text;
