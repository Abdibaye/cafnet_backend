# Canonical Food Discovery Specification
**Document ID:** `CAFNET-BACKEND-20-FOOD-DISCOVERY`  
**Status:** Production Standard  
**Target Engine:** Neon Serverless PostgreSQL 16+ (PostgreSQL 18.6 Verified)  
**Author:** Cafnet Architecture Team  

---

## 1. Overview & Product Vision

Cafnet enables diners to discover where to eat specific dishes across Addis Ababa and beyond. In real-world restaurant operations, restaurants name their menu items with brand adjectives, portions, or signature distinctions:
- *"Gojo Special Tibs"*
- *"Siga Tibs"*
- *"Beef Tibs"*
- *"Special Beef Tibs"*
- *"Shekla Tibs"*

When a customer searches for **"tibs"**, **"tibse"**, or **"ጥብስ"**, Cafnet must discover all of these restaurant offerings, while strictly honoring the restaurant's authentic naming.

### The Invariant Principle:
> **The restaurant owns its menu naming; Cafnet owns the normalized discovery taxonomy.**  
> Cafnet NEVER replaces, overrides, or alters a restaurant's menu item name with the canonical dish name. The restaurant's original name and pricing remain immutable.

---

## 2. Product Dimensions

Cafnet explicitly separates six distinct concepts into independent, normalized entities. These concepts must **never** be collapsed into a single table:

| Dimension | Scope | Owner | Example |
|---|---|---|---|
| **Restaurant** | Physical venue / business | Business Owner / Admin | *Gojo Restaurant* |
| **Cuisine** | Culinary tradition | Cafnet / Business | *Ethiopian*, *Italian*, *Indian* |
| **Menu Section** | Venue-controlled menu category | Business Owner / Manager | *Main Dishes*, *Traditional Foods* |
| **Menu Item** | Physical offering sold by the venue | Business Owner / Manager | *Gojo Special Tibs* (280 ETB) |
| **Canonical Dish** | Normalized food concept | Cafnet Platform | *Tibs*, *Kitfo*, *Shiro* |
| **Dish Alias** | Synonyms, spellings, transliterations | Cafnet Platform | *tibse*, *tibbs*, *tibis*, *ጥብስ* |

### Why Cuisine is Not Dish:
Cuisine describes a culinary tradition (e.g. *Ethiopian* or *Italian*), not a specific dish. Kitfo, Tibs, and Shiro all share the Ethiopian cuisine, but represent distinct food concepts.

---

## 3. Database Architecture (Neon PostgreSQL)

Implemented in migration `000007_canonical_food_discovery.up.sql`.

### 3.1 `canonical_dishes`
Represents the normalized food concept.
```sql
CREATE TABLE canonical_dishes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(150) NOT NULL UNIQUE,
    slug VARCHAR(160) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    cuisine_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
- **Indexes:**
  - `idx_canonical_dishes_name_trgm`: GIN trigram index on `name` for fuzzy / partial search.
  - `idx_canonical_dishes_cuisine`: B-tree index on `cuisine_id`.
  - Trigger `trg_canonical_dishes_updated_at` automatically maintains `updated_at`.

### 3.2 `dish_aliases`
Represents alternative names, typos, and multilingual variations (including Ethiopic Amharic script).
```sql
CREATE TABLE dish_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_dish_id UUID NOT NULL REFERENCES canonical_dishes(id) ON DELETE CASCADE,
    alias VARCHAR(150) NOT NULL,
    normalized_alias VARCHAR(150) NOT NULL,
    language VARCHAR(10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_canonical_dish_alias UNIQUE (canonical_dish_id, normalized_alias)
);
```
- **Indexes:**
  - `idx_dish_aliases_dish_id`: B-tree on `canonical_dish_id`.
  - `idx_dish_aliases_normalized_trgm`: GIN trigram index on `normalized_alias`.
  - `idx_dish_aliases_norm`: B-tree on `normalized_alias`.

### 3.3 `menu_items.canonical_dish_id`
Optional reference linking an individual menu item to a canonical dish.
```sql
ALTER TABLE menu_items
ADD COLUMN canonical_dish_id UUID REFERENCES canonical_dishes(id) ON DELETE SET NULL;

CREATE INDEX idx_menu_items_canonical_dish_id 
ON menu_items(canonical_dish_id) 
WHERE canonical_dish_id IS NOT NULL;
```

### Core Invariants Enforced in Storage:
1. **Optional Classification:** `canonical_dish_id` is nullable. Restaurant owners can create menu items (e.g. *"Chef's Special Surprise"*) without being blocked by taxonomy classification.
2. **Safe Deletions (`ON DELETE SET NULL`):** Deleting a canonical dish or alias NEVER deletes any restaurant's menu item; it only clears the classification pointer.
3. **No Cascade to Pricing or Naming:** Price and name reside strictly on `menu_items`.

---

## 4. Deterministic Normalization Engine

Food search uses deterministic text normalization implemented in `internal/domain/dish.go`:

```go
func NormalizeAlias(alias string) string {
    // 1. Lowercase via Unicode mapping (handles non-Latin scripts cleanly)
    // 2. Trim whitespace and outer punctuation
    // 3. Collapse multiple internal whitespace runes into single space
    // 4. Preserves Ethiopic unicode runes (e.g. 'ጥ', 'ብ', 'ስ')
}
```

This ensures queries like `"  Tibse! "` and `"TIBSE"` normalize identically to `"tibse"`, matching the registered alias.

---

## 5. Multi-Tier Search & Relevance Ranking

Food discovery operates without external search engines, leveraging Neon PostgreSQL full-text search, trigrams, and deterministic scoring weights:

### Relevance Ranking Hierarchy:
1. **Exact Canonical Dish Name Match** (Score: `1000`)
2. **Exact Menu Item Name Match** (Score: `800`)
3. **Exact Canonical Alias Match** (Score: `700`)
4. **Partial / Trigram Canonical Dish or Alias Match** (Score: `500`)
5. **Partial / Trigram Menu Item Name Match** (Score: `400`)
6. **Menu Item Description Match** (Score: `200`)

### Restaurant-Oriented Result Model:
Food search does not return raw disconnected database records. It returns venue-contextualized results answering: **"Where can I eat this?"**
```json
{
  "status": "success",
  "data": {
    "canonical_dish": {
      "id": "...",
      "name": "Tibs",
      "slug": "tibs"
    },
    "items": [
      {
        "menu_item": {
          "id": "...",
          "name": "Gojo Special Tibs",
          "price_etb_minor": 28000,
          "description": "Tender beef sautéed with rosemary and onions"
        },
        "business": {
          "id": 12,
          "name": "Gojo Restaurant",
          "slug": "gojo-restaurant",
          "neighborhood": "Bole",
          "rating": 4.7
        },
        "canonical_dish": { "name": "Tibs" },
        "matched_by": "exact_dish"
      }
    ]
  }
}
```

---

## 6. Authorization & IDOR Defenses (ReBAC)

1. **Taxonomy Management:**
   - Platform Administrators (`role = 'ADMIN'`) hold exclusive permission `PermissionAdminManageTaxonomy`.
   - Customers and unverified users receive `403 Forbidden` (`code: "forbidden"`) when attempting to create canonical dishes or register aliases.
2. **Menu Item Classification:**
   - Platform Administrators can classify any menu item.
   - Business Managers can classify menu items **only** for businesses they are verified to manage.
   - Cross-tenant IDOR attempts (Manager of Business A attempting to classify a menu item of Business B) are halted with `403 Forbidden` (`code: "not_business_manager"`).

---

## 7. Curated Starter Taxonomy

Seed data in migration 000007 initializes essential dishes and aliases:

### Ethiopian Core:
- **Tibs** (*tibse, tibbs, tibis, beef tibs, lamb tibs, siga tibs, shekla tibs, ጥብስ*)
- **Kitfo** (*ketfo, kitfow, ክትፎ, special kitfo*)
- **Shiro** (*shuro, shiro tegabino, tegabino shiro, ሺሮ, ሽሮ*)
- **Doro Wot** (*doro wat, doro wet, chicken wot, ዶሮ ወጥ*)
- **Firfir** (*fitfit, fir fir, fit fit, ፍርፍር*)
- **Chechebsa** (*kita firfir, ጨጨብሳ*)
- **Beyaynetu** (*yeetsom beyaynetu, vegetarian combo, fasting platter, በያይነቱ*)
- **Gomen** (*collard greens, ye'gomen wot, ጎመን*)
- **Misir** (*misir wot, red lentil stew, ምስር ወጥ*)
- **Alicha Wot** (*alicha, mild beef stew, አልጫ ወጥ*)
- **Kik Wot** (*yellow split peas, ክክ ወጥ*)
- **Dulet** (*ዱለት*)
- **Bozena Shiro** (*bozena, shiro with beef, ቦዘና ሺሮ*)
- **Fuul** (*foul mudammas, foul, ፉል*)

### International:
- **Pizza** (*margherita, pepperoni pizza, pizza*)
- **Burger** (*cheeseburger, beef burger, hamburger*)
- **Pasta** (*spaghetti, pasta bolognese, macaroni*)
- **Sandwich** (*club sandwich, sub*)
- **Fried Chicken** (*crispy chicken, broasted chicken*)

---

## 8. Frontend Implementation

1. **Food Discovery Search (`/search`):**
   - Queries `searchFood({ q: query })` in parallel with `searchBusinesses({ q: query })`.
   - Displays a dedicated "Where to eat [Dish]" section displaying the restaurant's authentic menu item name, price in ETB, rating, and venue links.
2. **Admin Center (`/admin/businesses/[id]`):**
   - Inspects menu categories and items.
   - Displays canonical classification status for each menu item (`Tibs` badge or `Unclassified`).
   - Allows administrators to map or unmap menu items to canonical dishes with instant database persistence.
