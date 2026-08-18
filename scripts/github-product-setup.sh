#!/usr/bin/env bash
# One-shot GitHub product setup for TerraDrift (labels, milestones, topics,
# branch protection, Discussions, private vulnerability reporting).
# Requires: gh auth login with repo + admin:repo_hook (or fine-grained admin) scopes.
set -euo pipefail

REPO="${REPO:-niravraychura/terradrift}"
API="repos/${REPO}"

echo "==> Labels"
create_label() {
  local name="$1" color="$2" desc="$3"
  if gh label list --repo "$REPO" --limit 200 --json name -q '.[].name' | grep -Fxq "$name"; then
    gh label edit "$name" --repo "$REPO" --color "$color" --description "$desc" >/dev/null || true
  else
    gh label create "$name" --repo "$REPO" --color "$color" --description "$desc" || true
  fi
}

create_label "bug" "d73a4a" "Something isn't working"
create_label "enhancement" "a2eeef" "New feature or request"
create_label "security" "ee0701" "Security-sensitive"
create_label "docs" "0075ca" "Documentation only"
create_label "good first issue" "7057ff" "Good for newcomers"
create_label "dependencies" "0366d6" "Dependency updates"
create_label "release" "c2e0c6" "Release or versioning work"

echo "==> Milestones"
ensure_milestone() {
  local title="$1" desc="$2"
  if gh api "$API/milestones" --jq '.[].title' 2>/dev/null | grep -Fxq "$title"; then
    echo "    exists: $title"
  else
    gh api --method POST "$API/milestones" -f title="$title" -f description="$desc" -f state=open >/dev/null
    echo "    created: $title"
  fi
}

ensure_milestone "v0.1.0" "First public tagged CLI release"
ensure_milestone "v0.2.0" "Next product slice after 0.1"
ensure_milestone "v1.0.0" "CLI/JSON stability promise"

echo "==> Repository topics + Discussions + private vuln reporting"
gh api --method PATCH "$API" \
  -f description='Self-hosted Terraform/OpenTofu drift detection CLI' \
  -f homepage='https://github.com/niravraychura/terradrift' \
  -F has_discussions=true \
  -F has_issues=true \
  -f "topics[]=terraform" \
  -f "topics[]=drift" \
  -f "topics[]=golang" \
  -f "topics[]=cli" \
  -f "topics[]=opentofu" \
  -f "topics[]=infrastructure-as-code" >/dev/null || \
gh repo edit "$REPO" \
  --description "Self-hosted Terraform/OpenTofu drift detection CLI" \
  --add-topic terraform --add-topic drift --add-topic golang --add-topic cli --add-topic opentofu \
  --enable-discussions 2>/dev/null || true

# Topics via dedicated endpoint (more reliable)
gh api --method PUT "$API/topics" \
  -H "Accept: application/vnd.github.mercy-preview+json" \
  -f "names[]=terraform" \
  -f "names[]=drift" \
  -f "names[]=golang" \
  -f "names[]=cli" \
  -f "names[]=opentofu" \
  -f "names[]=infrastructure-as-code" >/dev/null || true

# Private vulnerability reporting
gh api --method PUT "$API/private-vulnerability-reporting" >/dev/null 2>&1 || \
gh api --method PUT "$API/private-vulnerability-reporting" -F enabled=true >/dev/null 2>&1 || \
  echo "    (enable private vulnerability reporting in Settings → Code security if this failed)"

echo "==> Branch protection (main, dev)"
# Solo-maintainer friendly: require green CI and a PR; approvals optional until a second reviewer exists.
# enforce_admins=false so the owner can bypass in emergencies. Re-run with reviews=1 when ready.
protect_branch() {
  local branch="$1"
  gh api --method PUT "$API/branches/${branch}/protection" \
    -H "Accept: application/vnd.github+json" \
    --input - <<EOF
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["test"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "required_approving_review_count": 0,
    "require_code_owner_reviews": false,
    "dismiss_stale_reviews": true
  },
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": false
}
EOF
  echo "    protected: $branch"
}

protect_branch main || echo "    WARN: could not protect main (check token permissions / status check name)"
protect_branch dev || echo "    WARN: could not protect dev"

echo "==> Retarget open Dependabot PRs to dev (best effort)"
gh pr list --repo "$REPO" --author "app/dependabot" --state open --json number,baseRefName \
  --jq '.[] | select(.baseRefName != "dev") | .number' | while read -r num; do
  [ -z "$num" ] && continue
  echo "    retargeting PR #$num → dev"
  gh pr edit "$num" --repo "$REPO" --base dev || true
done

echo "==> Done"
echo "Create the Project board manually or via: gh project create --owner niravraychura --title 'TerraDrift' --format json"
echo "Then add columns: Backlog, Ready, In progress, In review, Done"
echo "After promoting to main, cut the first release: see docs/RELEASE.md"
