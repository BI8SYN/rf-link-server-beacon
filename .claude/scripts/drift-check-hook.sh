#!/usr/bin/env bash
# drift-check-hook.sh — SessionStart hook for doc drift detection
# Runs at the start of every Claude Code session.
# Silent exit if no issues; outputs JSON systemMessage if drift found.

set -euo pipefail

DOCS_DIR="docs"
MAIN_BRANCH="main"

# Detect main branch from CLAUDE.md if possible
if [ -f "CLAUDE.md" ]; then
    detected_branch=$(grep -oP '(?<=Main Branch\s*\|\s*)\S+' CLAUDE.md 2>/dev/null || true)
    if [ -n "$detected_branch" ]; then
        MAIN_BRANCH="$detected_branch"
    fi
fi

# --- Staleness check ---
stale_count=0
critical_count=0
today_epoch=$(date +%s)

if [ -d "$DOCS_DIR" ]; then
    while IFS= read -r line; do
        file=$(echo "$line" | cut -d: -f1)
        date_str=$(echo "$line" | grep -oP '(?<=last-reviewed:\s)[\d-]+' 2>/dev/null || true)
        if [ -z "$date_str" ]; then
            continue
        fi
        doc_epoch=$(date -d "$date_str" +%s 2>/dev/null || continue)
        days_old=$(( (today_epoch - doc_epoch) / 86400 ))
        if [ "$days_old" -gt 90 ]; then
            critical_count=$((critical_count + 1))
        elif [ "$days_old" -gt 30 ]; then
            stale_count=$((stale_count + 1))
        fi
    done < <(grep -r "last-reviewed:" "$DOCS_DIR" 2>/dev/null || true)
fi

# --- Branch change count ---
changed_files=0
on_feature_branch=false

current_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
if [ -n "$current_branch" ] && [ "$current_branch" != "$MAIN_BRANCH" ]; then
    on_feature_branch=true
    merge_base=$(git merge-base HEAD "$MAIN_BRANCH" 2>/dev/null || echo "")
    if [ -n "$merge_base" ]; then
        changed_files=$(git diff --name-only "$merge_base"..HEAD 2>/dev/null | wc -l)
    fi
fi

# --- Decision ---
# Silent exit if no issues
if [ "$stale_count" -eq 0 ] && [ "$critical_count" -eq 0 ] && [ "$changed_files" -lt 10 ]; then
    exit 0
fi

# Build message parts
parts=()
if [ "$critical_count" -gt 0 ]; then
    parts+=("$critical_count docs critically stale (>90 days)")
fi
if [ "$stale_count" -gt 0 ]; then
    parts+=("$stale_count docs stale (>30 days)")
fi
if [ "$changed_files" -ge 10 ]; then
    parts+=("$changed_files files changed on branch '$current_branch'")
fi

# Join parts
message="Doc drift detected: "
first=true
for part in "${parts[@]}"; do
    if [ "$first" = true ]; then
        message+="$part"
        first=false
    else
        message+=", $part"
    fi
done
message+=". Consider running the doc-drift-check skill for details."

# Output JSON for Claude Code hook system
echo "{\"systemMessage\": \"$message\"}"
