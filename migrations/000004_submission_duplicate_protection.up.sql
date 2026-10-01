-- Migration 000004: Duplicate Pending Business Submission Protection
-- Clean up any duplicate pending test submissions, keeping the most recent
DELETE FROM business_submissions a USING business_submissions b
WHERE a.status = 'PENDING' AND b.status = 'PENDING'
  AND LOWER(TRIM(a.business_name)) = LOWER(TRIM(b.business_name))
  AND LOWER(TRIM(a.area)) = LOWER(TRIM(b.area))
  AND a.created_at < b.created_at;

-- Enforces uniqueness on (LOWER(business_name), LOWER(area)) while submission is PENDING
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_pending_business_submission 
ON business_submissions(LOWER(TRIM(business_name)), LOWER(TRIM(area))) 
WHERE status = 'PENDING';
