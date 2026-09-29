-- CAFNET INITIAL SCHEMA MIGRATION (DOWN)
-- Version: 000001
-- Description: Reverses initial schema, dropping all tables, triggers, and functions

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS admin_actions CASCADE;
DROP TABLE IF EXISTS followed_businesses CASCADE;
DROP TABLE IF EXISTS saved_businesses CASCADE;
DROP TABLE IF EXISTS business_reports CASCADE;
DROP TABLE IF EXISTS review_reports CASCADE;
DROP TABLE IF EXISTS review_helpful_votes CASCADE;
DROP TABLE IF EXISTS reviews CASCADE;
DROP TABLE IF EXISTS reservations CASCADE;
DROP TABLE IF EXISTS business_updates CASCADE;
DROP TABLE IF EXISTS menu_items CASCADE;
DROP TABLE IF EXISTS menu_categories CASCADE;
DROP TABLE IF EXISTS business_submissions CASCADE;
DROP TABLE IF EXISTS business_claims CASCADE;
DROP TABLE IF EXISTS business_managers CASCADE;
DROP TABLE IF EXISTS business_images CASCADE;
DROP TABLE IF EXISTS business_hours CASCADE;
DROP TABLE IF EXISTS businesses CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Drop procedural trigger functions
DROP FUNCTION IF EXISTS check_review_reservation_completed() CASCADE;
DROP FUNCTION IF EXISTS check_reservation_status_transition() CASCADE;
DROP FUNCTION IF EXISTS set_updated_at() CASCADE;
