
2026/05/03 00:21:22 [32m/Users/pushya/Documents/pushya_projects/pateproject/backend/database/db.go:40 [33mSLOW SQL >= 200ms
[0m[31;1m[318.037ms] [33m[rows:0][35m SELECT description FROM pg_catalog.pg_description WHERE objsubid = (SELECT ordinal_position FROM information_schema.columns WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'ingredients' AND column_name = 'name') AND objoid = (SELECT oid FROM pg_catalog.pg_class WHERE relname = 'ingredients' AND relnamespace = (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname = CURRENT_SCHEMA()))[0m

2026/05/03 00:21:23 [32m/Users/pushya/Documents/pushya_projects/pateproject/backend/database/db.go:40 [33mSLOW SQL >= 200ms
[0m[31;1m[417.754ms] [33m[rows:-][35m SELECT constraint_name FROM information_schema.table_constraints tc JOIN information_schema.constraint_column_usage AS ccu USING (constraint_schema, constraint_catalog, table_name, constraint_name) JOIN information_schema.columns AS c ON c.table_schema = tc.constraint_schema AND tc.table_name = c.table_name AND ccu.column_name = c.column_name WHERE constraint_type IN ('PRIMARY KEY', 'UNIQUE') AND c.table_catalog = 'neondb' AND c.table_schema = CURRENT_SCHEMA() AND c.table_name = 'items' AND constraint_type = 'UNIQUE'[0m

2026/05/03 00:21:24 [32m/Users/pushya/Documents/pushya_projects/pateproject/backend/database/db.go:40 [33mSLOW SQL >= 200ms
[0m[31;1m[316.395ms] [33m[rows:0][35m SELECT description FROM pg_catalog.pg_description WHERE objsubid = (SELECT ordinal_position FROM information_schema.columns WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'items' AND column_name = 'protein') AND objoid = (SELECT oid FROM pg_catalog.pg_class WHERE relname = 'items' AND relnamespace = (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname = CURRENT_SCHEMA()))[0m

2026/05/03 00:21:28 [32m/Users/pushya/Documents/pushya_projects/pateproject/backend/database/db.go:40 [33mSLOW SQL >= 200ms
[0m[31;1m[306.814ms] [33m[rows:0][35m SELECT description FROM pg_catalog.pg_description WHERE objsubid = (SELECT ordinal_position FROM information_schema.columns WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'goals' AND column_name = 'target_date') AND objoid = (SELECT oid FROM pg_catalog.pg_class WHERE relname = 'goals' AND relnamespace = (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname = CURRENT_SCHEMA()))[0m

2026/05/03 00:21:29 [32m/Users/pushya/Documents/pushya_projects/pateproject/backend/database/db.go:40 [33mSLOW SQL >= 200ms
[0m[31;1m[223.764ms] [33m[rows:1][35m SELECT count(*) FROM pg_indexes WHERE tablename = 'goals' AND indexname = 'idx_goals_user_id' AND schemaname = CURRENT_SCHEMA()[0m

2026/05/03 00:21:34 [32m/Users/pushya/Documents/pushya_projects/pateproject/backend/database/db.go:40 [33mSLOW SQL >= 200ms
[0m[31;1m[376.652ms] [33m[rows:-][35m SELECT a.attname as column_name, format_type(a.atttypid, a.atttypmod) AS data_type
		FROM pg_attribute a JOIN pg_class b ON a.attrelid = b.oid AND relnamespace = (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname = CURRENT_SCHEMA())
		WHERE a.attnum > 0 -- hide internal columns
		AND NOT a.attisdropped -- hide deleted columns
		AND b.relname = 'goal_macro_profiles'[0m

2026/05/03 00:21:35 [32m/Users/pushya/Documents/pushya_projects/pateproject/backend/database/db.go:40 [33mSLOW SQL >= 200ms
[0m[31;1m[322.773ms] [33m[rows:1][35m SELECT CURRENT_DATABASE()[0m

2026/05/03 00:21:37 [32m/Users/pushya/Documents/pushya_projects/pateproject/backend/database/db.go:40 [33mSLOW SQL >= 200ms
[0m[31;1m[251.058ms] [33m[rows:1][35m SELECT CURRENT_DATABASE()[0m
Test User ID: 10
Meal Logs:
- [00:10:33] [Breakfast] Boiled Eggs: 140 kcal (2 large eggs)
- [00:10:35] [Breakfast] Black Coffee: 2 kcal (1 cup (8 oz) black coffee)
- [00:10:44] [Breakfast] Boiled Eggs: 140 kcal (2 large eggs)
- [00:10:47] [Breakfast] Black Coffee: 240 kcal (1 cup (8 oz))
- [00:10:50] [Breakfast] Whole Wheat Toast with Butter: 130 kcal (1 slice of whole wheat bread, 1 tsp butter)
- [00:19:24] [Lunch] Grilled Chicken Salad: 320 kcal (Grilled chicken breast (150g), mixed greens (100g), cherry tomatoes (50g), cucumbers (50g), olive oil dressing (1 tbsp))
