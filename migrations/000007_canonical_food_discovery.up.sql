-- ============================================================================
-- CAFNET POSTGRESQL MIGRATION 000007: CANONICAL FOOD DISCOVERY TAXONOMY
-- ============================================================================

-- 1. Create canonical_dishes table
CREATE TABLE IF NOT EXISTS canonical_dishes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    cuisine VARCHAR(80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT unique_canonical_dish_slug UNIQUE (slug),
    CONSTRAINT unique_canonical_dish_name UNIQUE (name)
);

CREATE TRIGGER trg_canonical_dishes_updated_at
BEFORE UPDATE ON canonical_dishes
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_canonical_dishes_slug ON canonical_dishes(slug);
CREATE INDEX IF NOT EXISTS idx_canonical_dishes_name_trgm ON canonical_dishes USING GIN (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_canonical_dishes_cuisine ON canonical_dishes(cuisine) WHERE deleted_at IS NULL;

-- 2. Create dish_aliases table
CREATE TABLE IF NOT EXISTS dish_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_dish_id UUID NOT NULL REFERENCES canonical_dishes(id) ON DELETE CASCADE,
    alias VARCHAR(100) NOT NULL,
    normalized_alias VARCHAR(100) NOT NULL,
    language VARCHAR(10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_dish_alias UNIQUE (canonical_dish_id, normalized_alias)
);

CREATE INDEX IF NOT EXISTS idx_dish_aliases_dish_id ON dish_aliases(canonical_dish_id);
CREATE INDEX IF NOT EXISTS idx_dish_aliases_normalized ON dish_aliases(normalized_alias);
CREATE INDEX IF NOT EXISTS idx_dish_aliases_trgm ON dish_aliases USING GIN (normalized_alias gin_trgm_ops);

-- 3. Add nullable canonical_dish_id to menu_items
ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS canonical_dish_id UUID REFERENCES canonical_dishes(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_menu_items_canonical_dish ON menu_items(canonical_dish_id) WHERE deleted_at IS NULL;

-- 4. Seed Canonical Dishes
INSERT INTO canonical_dishes (name, slug, description, cuisine) VALUES
    ('Tibs', 'tibs', 'Sautéed or grilled meat seasoned with onions, garlic, rosemary, and spices.', 'Ethiopian'),
    ('Kitfo', 'kitfo', 'Minced raw or lightly warmed beef seasoned with mitmita and niter kibbeh.', 'Ethiopian'),
    ('Shiro', 'shiro', 'Flavorful spiced chickpea and legume flour stew simmered with garlic and onions.', 'Ethiopian'),
    ('Doro Wot', 'doro-wot', 'Slow-cooked spicy chicken stew simmered in rich berbere sauce with hard-boiled eggs.', 'Ethiopian'),
    ('Firfir', 'firfir', 'Shredded injera sautéed with rich berbere sauce, spices, and clarified butter.', 'Ethiopian'),
    ('Chechebsa', 'chechebsa', 'Torn pieces of unleavened flatbread fried with niter kibbeh and berbere.', 'Ethiopian'),
    ('Beyaynetu', 'beyaynetu', 'A colorful platter of assorted vegetarian and lentil stews served over injera.', 'Ethiopian'),
    ('Gomen', 'gomen', 'Slow-cooked collard greens sautéed with onions, garlic, ginger, and spices.', 'Ethiopian'),
    ('Misir', 'misir', 'Red split lentils simmered in a spiced berbere stew.', 'Ethiopian'),
    ('Alicha Wot', 'alicha-wot', 'Mild, fragrant stew made with turmeric, ginger, garlic, and meat or vegetables.', 'Ethiopian'),
    ('Kik Wot', 'kik-wot', 'Yellow split pea stew gently seasoned with turmeric, ginger, and garlic.', 'Ethiopian'),
    ('Dulet', 'dulet', 'Finely minced tripe, liver, and lean beef seasoned with mitmita, onions, and jalapeños.', 'Ethiopian'),
    ('Bozena Shiro', 'bozena-shiro', 'Hearty spiced chickpea stew cooked with tender cubes of dried or sautéed beef.', 'Ethiopian'),
    ('Fuul', 'fuul', 'Mashed fava beans cooked with spices, topped with eggs, cheese, tomatoes, and yogurt.', 'Ethiopian'),
    ('Pizza', 'pizza', 'Baked flatbread crust topped with tomato sauce, cheese, and various toppings.', 'Italian'),
    ('Burger', 'burger', 'Juicy patty served inside a toasted bun with lettuce, tomatoes, and condiments.', 'American'),
    ('Pasta', 'pasta', 'Traditional Italian noodles served with tomato sauce, meat sauce, or cream.', 'Italian'),
    ('Sandwich', 'sandwich', 'Freshly sliced meats, vegetables, and spreads served between toasted bread.', 'International'),
    ('Fried Chicken', 'fried-chicken', 'Crispy golden fried chicken seasoned with herbs and spices.', 'American')
ON CONFLICT (slug) DO NOTHING;

-- 5. Seed Dish Aliases
INSERT INTO dish_aliases (canonical_dish_id, alias, normalized_alias, language)
SELECT id, 'tibs', 'tibs', 'en' FROM canonical_dishes WHERE slug = 'tibs'
UNION ALL SELECT id, 'tibse', 'tibse', 'en' FROM canonical_dishes WHERE slug = 'tibs'
UNION ALL SELECT id, 'tibbs', 'tibbs', 'en' FROM canonical_dishes WHERE slug = 'tibs'
UNION ALL SELECT id, 'tibis', 'tibis', 'en' FROM canonical_dishes WHERE slug = 'tibs'
UNION ALL SELECT id, 'beef tibs', 'beef tibs', 'en' FROM canonical_dishes WHERE slug = 'tibs'
UNION ALL SELECT id, 'siga tibs', 'siga tibs', 'en' FROM canonical_dishes WHERE slug = 'tibs'
UNION ALL SELECT id, 'ጥብስ', 'ጥብስ', 'am' FROM canonical_dishes WHERE slug = 'tibs'
UNION ALL SELECT id, 'kitfo', 'kitfo', 'en' FROM canonical_dishes WHERE slug = 'kitfo'
UNION ALL SELECT id, 'ketfo', 'ketfo', 'en' FROM canonical_dishes WHERE slug = 'kitfo'
UNION ALL SELECT id, 'ክትፎ', 'ክትፎ', 'am' FROM canonical_dishes WHERE slug = 'kitfo'
UNION ALL SELECT id, 'shiro', 'shiro', 'en' FROM canonical_dishes WHERE slug = 'shiro'
UNION ALL SELECT id, 'shiro tegabino', 'shiro tegabino', 'en' FROM canonical_dishes WHERE slug = 'shiro'
UNION ALL SELECT id, 'shiro wot', 'shiro wot', 'en' FROM canonical_dishes WHERE slug = 'shiro'
UNION ALL SELECT id, 'ሺሮ', 'ሺሮ', 'am' FROM canonical_dishes WHERE slug = 'shiro'
UNION ALL SELECT id, 'doro wot', 'doro wot', 'en' FROM canonical_dishes WHERE slug = 'doro-wot'
UNION ALL SELECT id, 'doro wat', 'doro wat', 'en' FROM canonical_dishes WHERE slug = 'doro-wot'
UNION ALL SELECT id, 'doro wet', 'doro wet', 'en' FROM canonical_dishes WHERE slug = 'doro-wot'
UNION ALL SELECT id, 'ዶሮ ወጥ', 'ዶሮ ወጥ', 'am' FROM canonical_dishes WHERE slug = 'doro-wot'
UNION ALL SELECT id, 'firfir', 'firfir', 'en' FROM canonical_dishes WHERE slug = 'firfir'
UNION ALL SELECT id, 'fitfit', 'fitfit', 'en' FROM canonical_dishes WHERE slug = 'firfir'
UNION ALL SELECT id, 'fir fir', 'fir fir', 'en' FROM canonical_dishes WHERE slug = 'firfir'
UNION ALL SELECT id, 'ፍርፍር', 'ፍርፍር', 'am' FROM canonical_dishes WHERE slug = 'firfir'
UNION ALL SELECT id, 'chechebsa', 'chechebsa', 'en' FROM canonical_dishes WHERE slug = 'chechebsa'
UNION ALL SELECT id, 'kita firfir', 'kita firfir', 'en' FROM canonical_dishes WHERE slug = 'chechebsa'
UNION ALL SELECT id, 'ጨጨብሳ', 'ጨጨብሳ', 'am' FROM canonical_dishes WHERE slug = 'chechebsa'
UNION ALL SELECT id, 'beyaynetu', 'beyaynetu', 'en' FROM canonical_dishes WHERE slug = 'beyaynetu'
UNION ALL SELECT id, 'yetsom beyaynetu', 'yetsom beyaynetu', 'en' FROM canonical_dishes WHERE slug = 'beyaynetu'
UNION ALL SELECT id, 'fast beyaynetu', 'fast beyaynetu', 'en' FROM canonical_dishes WHERE slug = 'beyaynetu'
UNION ALL SELECT id, 'በያይነቱ', 'በያይነቱ', 'am' FROM canonical_dishes WHERE slug = 'beyaynetu'
UNION ALL SELECT id, 'gomen', 'gomen', 'en' FROM canonical_dishes WHERE slug = 'gomen'
UNION ALL SELECT id, 'gomen besiga', 'gomen besiga', 'en' FROM canonical_dishes WHERE slug = 'gomen'
UNION ALL SELECT id, 'ጎመን', 'ጎመን', 'am' FROM canonical_dishes WHERE slug = 'gomen'
UNION ALL SELECT id, 'misir', 'misir', 'en' FROM canonical_dishes WHERE slug = 'misir'
UNION ALL SELECT id, 'misir wot', 'misir wot', 'en' FROM canonical_dishes WHERE slug = 'misir'
UNION ALL SELECT id, 'misir wat', 'misir wat', 'en' FROM canonical_dishes WHERE slug = 'misir'
UNION ALL SELECT id, 'ምስር ወጥ', 'ምስር ወጥ', 'am' FROM canonical_dishes WHERE slug = 'misir'
UNION ALL SELECT id, 'alicha wot', 'alicha wot', 'en' FROM canonical_dishes WHERE slug = 'alicha-wot'
UNION ALL SELECT id, 'alicha wat', 'alicha wat', 'en' FROM canonical_dishes WHERE slug = 'alicha-wot'
UNION ALL SELECT id, 'alicha', 'alicha', 'en' FROM canonical_dishes WHERE slug = 'alicha-wot'
UNION ALL SELECT id, 'አሊጫ ወጥ', 'አሊጫ ወጥ', 'am' FROM canonical_dishes WHERE slug = 'alicha-wot'
UNION ALL SELECT id, 'kik wot', 'kik wot', 'en' FROM canonical_dishes WHERE slug = 'kik-wot'
UNION ALL SELECT id, 'kik wat', 'kik wat', 'en' FROM canonical_dishes WHERE slug = 'kik-wot'
UNION ALL SELECT id, 'yater kik', 'yater kik', 'en' FROM canonical_dishes WHERE slug = 'kik-wot'
UNION ALL SELECT id, 'ክክ ወጥ', 'ክክ ወጥ', 'am' FROM canonical_dishes WHERE slug = 'kik-wot'
UNION ALL SELECT id, 'dulet', 'dulet', 'en' FROM canonical_dishes WHERE slug = 'dulet'
UNION ALL SELECT id, 'ዱለት', 'ዱለት', 'am' FROM canonical_dishes WHERE slug = 'dulet'
UNION ALL SELECT id, 'bozena shiro', 'bozena shiro', 'en' FROM canonical_dishes WHERE slug = 'bozena-shiro'
UNION ALL SELECT id, 'bojena shiro', 'bojena shiro', 'en' FROM canonical_dishes WHERE slug = 'bozena-shiro'
UNION ALL SELECT id, 'ቦዘና ሺሮ', 'ቦዘና ሺሮ', 'am' FROM canonical_dishes WHERE slug = 'bozena-shiro'
UNION ALL SELECT id, 'fuul', 'fuul', 'en' FROM canonical_dishes WHERE slug = 'fuul'
UNION ALL SELECT id, 'foul', 'foul', 'en' FROM canonical_dishes WHERE slug = 'fuul'
UNION ALL SELECT id, 'ful', 'ful', 'en' FROM canonical_dishes WHERE slug = 'fuul'
UNION ALL SELECT id, 'ፉል', 'ፉል', 'am' FROM canonical_dishes WHERE slug = 'fuul'
UNION ALL SELECT id, 'pizza', 'pizza', 'en' FROM canonical_dishes WHERE slug = 'pizza'
UNION ALL SELECT id, 'ፒዛ', 'ፒዛ', 'am' FROM canonical_dishes WHERE slug = 'pizza'
UNION ALL SELECT id, 'burger', 'burger', 'en' FROM canonical_dishes WHERE slug = 'burger'
UNION ALL SELECT id, 'hamburger', 'hamburger', 'en' FROM canonical_dishes WHERE slug = 'burger'
UNION ALL SELECT id, 'cheeseburger', 'cheeseburger', 'en' FROM canonical_dishes WHERE slug = 'burger'
UNION ALL SELECT id, 'በርገር', 'በርገር', 'am' FROM canonical_dishes WHERE slug = 'burger'
UNION ALL SELECT id, 'pasta', 'pasta', 'en' FROM canonical_dishes WHERE slug = 'pasta'
UNION ALL SELECT id, 'spaghetti', 'spaghetti', 'en' FROM canonical_dishes WHERE slug = 'pasta'
UNION ALL SELECT id, 'ፓስታ', 'ፓስታ', 'am' FROM canonical_dishes WHERE slug = 'pasta'
UNION ALL SELECT id, 'sandwich', 'sandwich', 'en' FROM canonical_dishes WHERE slug = 'sandwich'
UNION ALL SELECT id, 'ሳንድዊች', 'ሳንድዊች', 'am' FROM canonical_dishes WHERE slug = 'sandwich'
UNION ALL SELECT id, 'fried chicken', 'fried chicken', 'en' FROM canonical_dishes WHERE slug = 'fried-chicken'
UNION ALL SELECT id, 'crispy chicken', 'crispy chicken', 'en' FROM canonical_dishes WHERE slug = 'fried-chicken'
UNION ALL SELECT id, 'ዶሮ ጥብስ', 'ዶሮ ጥብስ', 'am' FROM canonical_dishes WHERE slug = 'fried-chicken'
ON CONFLICT (canonical_dish_id, normalized_alias) DO NOTHING;
