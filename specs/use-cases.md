# Stash: Use Cases

This document describes real-world scenarios for using Stash, serving as both validation for the spec and a basis for integration tests.

---

## Use Case 1: Company Research Pipeline

### Scenario

An AI agent needs to research 100 companies from a CSV file, verify each exists, and collect detailed information.

### Workflow

```bash
# 1. User provides CSV with company names
# companies.csv:
# CompanyName,Industry
# Microsoft,Technology
# Apple,Technology
# Acme Corp,Unknown
# ...

# 2. User: "Create a stash from this CSV and research each company"

# Agent initializes
stash init research --prefix re-
stash import companies.csv --confirm

# Agent adds research columns
stash column add Verified --desc "true if company confirmed to exist"
stash column add Website --desc "Official company website URL"
stash column add Overview --desc "Markdown file with company summary"
stash column add CEO --desc "Current CEO full name"
stash column add Founded --desc "Year company was founded"

# 3. Agent iterates through records
stash list --json | jq -c '.[]' | while read record; do
    id=$(echo "$record" | jq -r '.id')
    name=$(echo "$record" | jq -r '.CompanyName')

    # Agent researches company...
    # If found:
    stash set "$id" Verified true
    stash set "$id" Website "https://microsoft.com"
    stash set "$id" CEO "Satya Nadella"
    stash set "$id" Founded 1975
    stash file "$id" Overview --content "# $name\n\n..."

    # If not found:
    stash set "$id" Verified false
done

# 4. User: "Show me companies we couldn't verify"
stash list --where "Verified = false"

# 5. User: "Export verified companies"
stash export verified-companies.csv --where "Verified = true"
```

### Expected Outputs

```
# After import
Imported 100 records from companies.csv
Added columns: Industry (existed: CompanyName)

# After research
stash list --columns id,CompanyName,Verified
ID        CompanyName   Verified
────────────────────────────────
re-ex4j   Microsoft     true
re-8t5n   Apple         true
re-k2m9   Acme Corp     false
...

# Stats
stash column list
Column       Description                     Populated  Empty
─────────────────────────────────────────────────────────────
CompanyName  (imported)                      100        0
Industry     (imported)                       98        2
Verified     true if company confirmed        97        3
Website      Official company website URL     95        5
Overview     Markdown file with summary       92        8
CEO          Current CEO full name            88       12
Founded      Year company was founded         90       10
```

---

## Use Case 2: Hierarchical Product Catalog

### Scenario

Building a product catalog with categories, subcategories, and products.

### Workflow

```bash
# 1. Initialize catalog stash
stash init catalog --prefix cat-

# 2. Add schema
stash column add Name --desc "Display name"
stash column add Type --desc "category, subcategory, or product"
stash column add Price --desc "Price in USD (products only)"
stash column add Description --desc "Markdown description file"

# 3. Create category hierarchy
stash add "Electronics" --set Type category
# → cat-a1b2

stash add "Computers" --parent cat-a1b2 --set Type subcategory
# → cat-a1b2.1

stash add "Phones" --parent cat-a1b2 --set Type subcategory
# → cat-a1b2.2

stash add "MacBook Pro" --parent cat-a1b2.1 --set Type product --set Price 1999
# → cat-a1b2.1.1

stash add "iPhone 15" --parent cat-a1b2.2 --set Type product --set Price 999
# → cat-a1b2.2.1

# 4. View hierarchy
stash list --tree
```

### Expected Output

```
stash list --tree

cat-a1b2   Electronics
├─ cat-a1b2.1   Computers
│  └─ cat-a1b2.1.1   MacBook Pro
└─ cat-a1b2.2   Phones
   └─ cat-a1b2.2.1   iPhone 15

stash children cat-a1b2
ID           Name        Type
─────────────────────────────
cat-a1b2.1   Computers   subcategory
cat-a1b2.2   Phones      subcategory

stash query "SELECT Name, Price FROM catalog WHERE Type = 'product'"
Name          Price
─────────────────────
MacBook Pro   1999
iPhone 15     999
```

---

## Use Case 3: Competitor Analysis

### Scenario

Tracking competitors with multiple data points, updating over time.

### Workflow

```bash
# 1. Initialize
stash init competitors --prefix comp-

# 2. Define schema with descriptions
stash column add Name --desc "Competitor company name"
stash column add Website --desc "Primary website URL"
stash column add Employees --desc "Estimated employee count"
stash column add Funding --desc "Total funding raised (USD)"
stash column add LastUpdated --desc "Date of last research update (YYYY-MM-DD)"
stash column add Notes --desc "Markdown file with detailed analysis"
stash column add Threat --desc "Threat level: low, medium, high"

# 3. Add competitors
stash add "Competitor A" \
    --set Website "https://competitor-a.com" \
    --set Employees 500 \
    --set Funding 50000000 \
    --set Threat high

stash add "Competitor B" \
    --set Website "https://competitor-b.com" \
    --set Employees 100 \
    --set Threat medium

# 4. Update when new info available
stash set comp-ex4j Funding 75000000
stash set comp-ex4j LastUpdated "2025-01-08"
stash file comp-ex4j Notes --content "# Competitor A Analysis\n\n## Recent News\n..."

# 5. Query by threat level
stash list --where "Threat = 'high'" --json
```

### Expected Output

```json
[
  {
    "id": "comp-ex4j",
    "Name": "Competitor A",
    "Website": "https://competitor-a.com",
    "Employees": 500,
    "Funding": 75000000,
    "LastUpdated": "2025-01-08",
    "Notes": "comp-ex4j.md",
    "Threat": "high"
  }
]
```

---

## Use Case 4: Multi-Stash Workflow

### Scenario

Research project with companies AND contacts, cross-referenced.

### Workflow

```bash
# 1. Initialize both stashes
stash init companies --prefix co-
stash init contacts --prefix ct-

# 2. Define schemas
stash column add --stash companies Name Website Industry

stash column add --stash contacts \
    Name --desc "Full name" \
    Email --desc "Email address" \
    Company --desc "Company ID from companies stash (co-xxxx)" \
    Role --desc "Job title"

# 3. Add companies
stash add "Acme Inc" --stash companies
# → co-a1b2

stash add "Beta Corp" --stash companies
# → co-c3d4

# 4. Add contacts linked to companies
stash add "John Smith" --stash contacts \
    --set Email "john@acme.com" \
    --set Company co-a1b2 \
    --set Role "CEO"

stash add "Jane Doe" --stash contacts \
    --set Email "jane@beta.com" \
    --set Company co-c3d4 \
    --set Role "CTO"

# 5. Query contacts at a specific company
stash list --stash contacts --where "Company = 'co-a1b2'"

# 6. View all stashes
stash info
```

### Expected Output

```
stash info

Stashes:
  companies (co-)  2 records, 0 files, synced
  contacts (ct-)   2 records, 0 files, synced

Daemon: running (PID 12345), last sync 1s ago
Cache: .stash/cache.db (48 KB)
```

---

## Use Case 5: Resume Interrupted Work

### Scenario

Agent session ends mid-research. New session needs to continue.

### Workflow

```bash
# Session 1: Start research
stash init research --prefix re-
stash import companies.csv --confirm
stash column add Verified Overview

# Process 50 of 100 companies...
# Session ends

# Session 2: Resume
# Agent reads context
stash prime

# Output shows:
# Records: 100 total
#   - 50 with Verified = true
#   - 50 with Verified = false or empty

# Agent queries unfinished work
stash list --where "Verified IS NULL OR Verified = ''" --json

# Continue processing remaining 50...
```

### Key Points

- `stash prime` gives agent full context
- Querying empty/null fields identifies incomplete work
- JSONL append-only log preserves all history

---

## Use Case 6: Data Cleanup and Maintenance

### Scenario

Stash has accumulated issues: orphaned files, missing columns, sync problems.

### Workflow

```bash
# 1. Check health
stash doctor

# Output:
# ✓ Daemon running (PID 12345)
# ✓ Cache database valid
# ✓ research: 100 records
# ⚠ research: 5 file references point to missing files
# ⚠ research: Column 'OldColumn' in DB not in schema
# ✗ contacts: JSONL has 3 records not in cache

# 2. Fix automatically
stash doctor --fix --yes

# Output:
# Fixed: Removed 5 dangling file references
# Fixed: Added 'OldColumn' to schema
# Fixed: Synced 3 records from JSONL to cache

# 3. Force full rebuild if needed
stash sync --rebuild

# 4. Clean orphaned files
stash repair --clean-orphans --dry-run

# Output:
# Would remove 3 orphaned files:
#   .stash/research/files/re-deleted1.md
#   .stash/research/files/re-deleted2.md
#   .stash/research/files/old-backup.md

stash repair --clean-orphans --yes
```

---

## Use Case 7: Export for External Tools

### Scenario

Need to export stash data for spreadsheet analysis or reporting.

### Workflow

```bash
# 1. Full export to CSV
stash export full-export.csv

# 2. Filtered export
stash export high-priority.csv --where "Threat = 'high'"

# 3. Export specific columns
stash list --columns "id,Name,Website,Threat" > custom.csv

# 4. Export to JSON for programmatic use
stash export data.json --format json

# 5. Export with file contents embedded
stash list --json --with-files > full-data.json
```

---

## Use Case 8: Git Worktree Collaboration

### Scenario

Multiple agents working in different git worktrees on same project.

### Workflow

```bash
# Main worktree: Agent 1 working on companies A-M
cd ~/project
stash list --where "CompanyName < 'N'" --json
# Process companies...

# Feature worktree: Agent 2 working on companies N-Z
cd ~/worktrees/feature-branch
stash sync --from-main  # Get latest JSONL from main
stash list --where "CompanyName >= 'N'" --json
# Process companies...

# Before commit in worktree
stash sync --flush  # Ensure JSONL is up to date

# After merge to main
cd ~/project
git pull
stash sync --rebuild  # Rebuild cache from merged JSONL
```

---

## Use Case 9: Schema Evolution

### Scenario

Research needs evolve; add new columns mid-project.

### Workflow

```bash
# Initial schema
stash column list
Column       Description              Populated  Empty
──────────────────────────────────────────────────────
CompanyName  Company name             100        0
Verified     Verification status       50       50

# User: "Also track social media presence"
stash column add Twitter LinkedIn --desc "Social media handle/URL"

# New columns available immediately
stash set re-ex4j Twitter "@microsoft"
stash set re-ex4j LinkedIn "https://linkedin.com/company/microsoft"

# Column stats update
stash column list
Column       Description              Populated  Empty
──────────────────────────────────────────────────────
CompanyName  Company name             100        0
Verified     Verification status       50       50
Twitter      Social media handle/URL    1       99
LinkedIn     Social media handle/URL    1       99

# User: "Rename 'Twitter' to 'X'"
stash column rename Twitter X

# User: "We don't need LinkedIn anymore"
stash column drop LinkedIn --yes
```

---

## Use Case 10: Debugging and Recovery

### Scenario

Something went wrong; need to investigate and recover.

### Workflow

```bash
# 1. Check daemon status
stash daemon status

# If not running:
stash daemon start

# 2. View daemon logs
stash daemon logs

# 3. Check sync status
stash sync --status

# 4. View raw JSONL (source of truth)
cat .stash/research/records.jsonl | tail -20

# 5. View raw config
cat .stash/research/config.json | jq .

# 6. Force rebuild from JSONL
stash repair --source jsonl

# 7. If JSONL is corrupted, rebuild from DB
stash repair --source db

# 8. Nuclear option: start fresh
stash drop research --yes
stash init research --prefix re-
stash import backup.csv --confirm
```

---

## Error Scenarios

### E1: Stash Not Found

```bash
$ stash list --stash nonexistent
Error: stash 'nonexistent' not found
Available stashes: research, contacts
Exit code: 3
```

### E2: Record Not Found

```bash
$ stash show re-invalid
Error: record 're-invalid' not found
Exit code: 4
```

### E3: Parent Not Found (Hierarchy)

```bash
$ stash add "Child" --parent re-invalid
Error: parent record 're-invalid' not found
Exit code: 4
```

### E4: Column Already Exists

```bash
$ stash column add CompanyName
Error: column 'CompanyName' already exists
Exit code: 1
```

### E5: Import Conflict

```bash
$ stash import companies.csv
Error: Column 'Status' in CSV conflicts with existing column 'Status'
       (different case: 'status' vs 'Status')
Use --force to override or rename the CSV column.
Exit code: 1
```

### E6: Sync Conflict

```bash
$ stash sync
Error: Sync conflict detected
  Record re-ex4j modified in both JSONL and cache
  JSONL: Verified = true (2025-01-08T10:00:00Z)
  Cache: Verified = false (2025-01-08T10:05:00Z)

Use 'stash sync --source jsonl' to prefer JSONL
Use 'stash sync --source db' to prefer cache
Exit code: 5
```

---

## Performance Scenarios

### P1: Large Import (10,000 records)

```bash
$ time stash import large-dataset.csv --confirm

Importing 10,000 records...
  Progress: [████████████████████] 100%
  Time: 4.2s
  Records/sec: 2,380

Imported 10,000 records with 15 columns.
```

### P2: Filtered Query (10,000 records)

```bash
$ time stash list --where "Industry = 'Technology' AND Verified = true" --json | wc -l

847
real    0m0.12s
```

### P3: Tree View (1,000 hierarchical records)

```bash
$ time stash list --tree | head -50

real    0m0.35s
```

---

## Integration Points

### I1: Claude Code Session Start

```bash
# In .claude/hooks/session-start.sh
stash prime >> /tmp/claude-context.md
```

### I2: Beads Integration

```bash
# Link stash record to beads issue
bd create "Research company re-ex4j" --type task
stash set re-ex4j BeadID bd-xyz123

# Query by bead
stash list --where "BeadID = 'bd-xyz123'"
```

### I3: Git Pre-Commit Hook

```bash
# In .git/hooks/pre-commit
stash sync --flush
git add .stash/*/records.jsonl
```
