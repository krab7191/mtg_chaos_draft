-- Weights were stored as multipliers (1.0 = normal, 0.5 = half, 2.0 = double).
-- The system now uses integer offsets (0 = normal, +1 = double, -1 = exclude).
-- Clear old values so all packs start at 0 (neutral).
DELETE FROM pack_weight_overrides;
ALTER TABLE pack_weight_overrides ALTER COLUMN multiplier SET DEFAULT 0;
